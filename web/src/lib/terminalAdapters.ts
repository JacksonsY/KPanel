import { api, terminalStream } from '@/lib/api'
import type { TerminalDuplexOptions } from '@/lib/terminalDuplexInput'
import type { TerminalStreamSubscription } from '@/lib/terminalStream'
import type { AppTerminalChunk, JobTerminalKind, TerminalOutput } from '@/types/api'

export interface TerminalChunk {
  data: string
  nextOffset: number
  truncated?: boolean
  finished: boolean
  inputOpen: boolean
  exitError?: string
}

export interface TerminalOutputHandlers {
  chunk: (chunk: TerminalChunk) => void
  connected: () => void
  error: (code: string) => void
  unavailable: () => void
}

// Adapters attach to an existing identity. Creation, cancellation and process
// termination remain with the owning business flow, never with the viewer.
export interface TerminalAdapter {
  key: string
  mode: 'session' | 'task'
  input: Omit<TerminalDuplexOptions, 'error' | 'recovered'>
  read: (offset: number, inputOpen: boolean, signal: AbortSignal) => Promise<TerminalChunk>
  subscribe: (offset: number, inputOpen: boolean, handlers: TerminalOutputHandlers) => TerminalStreamSubscription | null
  resize?: (rows: number, columns: number) => Promise<void>
  missing: (code: string) => boolean
}

const hostChunk = (chunk: TerminalOutput): TerminalChunk => ({
  data: chunk.data, nextOffset: chunk.nextOffset, truncated: chunk.truncated,
  finished: Boolean(chunk.closed || chunk.exitedAt), inputOpen: !chunk.closed && !chunk.exitedAt,
  exitError: chunk.exitError,
})
const jobChunk = (chunk: AppTerminalChunk): TerminalChunk => ({
  data: chunk.dataBase64, nextOffset: chunk.nextOffset, truncated: chunk.truncated,
  finished: chunk.finished, inputOpen: chunk.inputOpen && !chunk.finished,
})

export function hostTerminalAdapter(sessionId: string): TerminalAdapter {
  return {
    key: `terminal:${sessionId}`, mode: 'session',
    input: {
      negotiate: signal => api.terminals.inputTransport(sessionId, signal),
      credentials: () => api.terminals.inputSocket(sessionId),
      legacy: data => api.terminals.input(sessionId, data),
      batch: (frames, signal) => api.terminals.inputBatch(sessionId, frames, signal),
    },
    read: async (offset, _inputOpen, signal) => hostChunk(await api.terminals.output(sessionId, offset, signal)),
    subscribe: (offset, _inputOpen, handlers) => terminalStream.subscribe({ kind: 'terminal', id: sessionId, offset }, {
      output: chunk => handlers.chunk(hostChunk(chunk)), connected: handlers.connected,
      error: handlers.error, unavailable: handlers.unavailable,
    }),
    resize: async (rows, columns) => {
      const result = await api.terminals.resize(sessionId, rows, columns)
      if (!result.accepted) throw new Error('Terminal resize was not accepted')
    },
    missing: code => code === 'terminal_not_found',
  }
}

export function jobTerminalAdapter(kind: JobTerminalKind, jobId: string): TerminalAdapter {
  const backend = { app: api.apps, site: api.sites, diagnostic: api.diagnostics, environment: api.webEnvironment }[kind]
  return {
    key: `job:${kind}:${jobId}`, mode: 'task',
    input: {
      negotiate: signal => api.jobTerminals.inputTransport(kind, jobId, signal),
      credentials: () => api.jobTerminals.inputSocket(kind, jobId),
      legacy: data => backend.terminalInput(jobId, data), legacyText: true,
      batch: (frames, signal) => api.jobTerminals.inputBatch(kind, jobId, frames, signal),
    },
    read: async (offset, inputOpen, signal) => jobChunk(await backend.terminal(jobId, offset, inputOpen, signal)),
    subscribe: (offset, inputOpen, handlers) => terminalStream.subscribe({ kind: 'job', job: kind, id: jobId, offset, inputOpen }, {
      job: chunk => handlers.chunk(jobChunk(chunk)), connected: handlers.connected,
      error: handlers.error, unavailable: handlers.unavailable,
    }),
    resize: kind === 'app' ? async (rows, columns) => {
      const result = await api.apps.terminalResize(jobId, rows, columns)
      if (!result.accepted) throw new Error('Task terminal resize was not accepted')
    } : undefined,
    missing: code => ({ app: ['app_job_not_found', 'app_terminal_not_found'], site: ['site_terminal_not_found'], diagnostic: ['diagnostic_job_not_found', 'diagnostic_terminal_not_found'], environment: ['environment_terminal_not_found'] }[kind]).includes(code),
  }
}
