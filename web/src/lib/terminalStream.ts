// One Server-Sent Events connection per tab carries output for every open
// terminal (host, batch and task terminals). Components subscribe with their
// current offset; when the stream is unavailable (no EventSource, a proxy that
// buffers or blocks it, or repeated failures) subscribe() returns null or the
// subscription reports `unavailable`, and callers keep their polling path.
import type { AppTerminalChunk, TerminalOutput } from '@/types/api'
import { terminalRequest } from './terminalRequest'

export type TerminalStreamTarget =
  | { kind: 'terminal'; id: string; offset: number }
  | { kind: 'job'; job: 'app' | 'site' | 'diagnostic' | 'environment'; id: string; offset: number; inputOpen: boolean }

export interface TerminalStreamHandlers {
  connected?: () => void
  output?: (output: TerminalOutput) => void
  job?: (chunk: AppTerminalChunk) => void
  error?: (code: string) => void
  unavailable?: () => void
}

export interface TerminalStreamSubscription {
  close(): void
}

interface StreamEvent {
  key: string
  output?: TerminalOutput
  job?: AppTerminalChunk
  error?: string
}

export interface TerminalStreamTransport {
  url(): string
  subscribe(body: { streamId: string; add: unknown[]; remove: string[] }, signal: AbortSignal): Promise<unknown>
  createSource?: (url: string) => EventSource
}

interface Entry {
  target: TerminalStreamTarget
  handlers: TerminalStreamHandlers
  finished?: boolean
  inputOpen?: boolean
  exitError?: string
}

const MAX_FAILURES_BEFORE_READY = 3
const READY_TIMEOUT_MS = 5000
const SUBSCRIBE_DEBOUNCE_MS = 10
// Terminals briefly unsubscribe when minimized or while xterm drains a burst;
// keep the idle connection so resuming does not reopen the stream each time.
const IDLE_CLOSE_MS = 5000

export function terminalStreamKey(target: TerminalStreamTarget): string {
  return target.kind === 'job' ? `job:${target.job}:${target.id}` : `terminal:${target.id}`
}

export class TerminalStreamClient {
  private source?: EventSource
  private streamId = ''
  private failures = 0
  private disabled = false
  private readyTimer?: ReturnType<typeof setTimeout>
  private flushTimer?: ReturnType<typeof setTimeout>
  private idleTimer?: ReturnType<typeof setTimeout>
  private readonly entries = new Map<string, Set<Entry>>()
  private pendingAdd = new Set<string>()
  private pendingRemove = new Set<string>()
  private subscriptionRequest?: AbortController

  constructor(
    private readonly transport: TerminalStreamTransport,
    private readonly idleCloseMs = IDLE_CLOSE_MS,
  ) {}

  get available(): boolean {
    return !this.disabled
  }

  subscribe(target: TerminalStreamTarget, handlers: TerminalStreamHandlers): TerminalStreamSubscription | null {
    if (this.disabled || !this.canStream()) return null
    this.clearIdleTimer()
    this.ensureSource()
    if (this.disabled) return null
    const key = terminalStreamKey(target)
    const viewers = this.entries.get(key) ?? new Set<Entry>()
    // Match the server's per-tab target budget and bound local fan-out.
    if (viewers.size >= 16 || !this.entries.has(key) && this.entries.size >= 16) return null
    const previousOffset = this.targetFor(key)?.offset
    const entry: Entry = { target: { ...target }, handlers }
    viewers.add(entry)
    this.entries.set(key, viewers)
    this.pendingRemove.delete(key)
    if (previousOffset === undefined || target.offset < previousOffset) this.pendingAdd.add(key)
    this.scheduleFlush()
    let closed = false
    return {
      close: () => {
        if (closed) return
        closed = true
        this.removeViewer(key, entry)
      },
    }
  }

  private removeViewer(key: string, entry: Entry): void {
    const current = this.entries.get(key)
    if (!current?.delete(entry) || current.size) return
    this.entries.delete(key)
    this.pendingAdd.delete(key)
    if (this.streamId) this.pendingRemove.add(key)
    this.scheduleFlush()
    if (!this.entries.size) this.scheduleIdleClose()
  }

  private targetFor(key: string): TerminalStreamTarget | undefined {
    const viewers = this.entries.get(key)
    if (!viewers?.size) return undefined
    let selected: TerminalStreamTarget | undefined
    for (const entry of viewers) {
      if (!selected || entry.target.offset < selected.offset) selected = entry.target
    }
    return selected ? { ...selected } : undefined
  }

  private deliver(key: string, payload: StreamEvent): void {
    const viewers = this.entries.get(key)
    if (!viewers) return
    const chunk = payload.output ?? payload.job
    if (!chunk) {
      if (payload.error) for (const entry of [...viewers]) if (viewers.has(entry)) entry.handlers.error?.(payload.error)
      return
    }
    let bytes: Uint8Array
    try {
      const data = payload.output?.data ?? payload.job?.dataBase64 ?? ''
      bytes = Uint8Array.from(atob(data), character => character.charCodeAt(0))
      if (!Number.isSafeInteger(chunk.nextOffset) || chunk.nextOffset < bytes.length) throw new Error('Invalid output offset')
    } catch {
      for (const entry of [...viewers]) if (viewers.has(entry)) entry.handlers.error?.('invalid_terminal_output')
      return
    }
    const start = chunk.nextOffset - bytes.length
    for (const entry of [...viewers]) {
      if (!viewers.has(entry) || Boolean(payload.output) !== (entry.target.kind === 'terminal')) continue
      const cursor = entry.target.offset
      // An old live pump can emit before the catch-up subscription takes
      // effect. A gap without a server truncation marker is not data loss:
      // let this viewer read from its own cursor over polling instead.
      if (start > cursor && !chunk.truncated) {
        this.removeViewer(key, entry)
        entry.handlers.unavailable?.()
        continue
      }
      const finished = payload.output ? Boolean(payload.output.closed || payload.output.exitedAt) : Boolean(payload.job?.finished)
      const inputOpen = payload.job?.inputOpen
      const exitError = payload.output?.exitError
      if (chunk.nextOffset <= cursor && entry.finished === finished && entry.inputOpen === inputOpen && entry.exitError === exitError) continue
      let data = payload.output?.data ?? payload.job?.dataBase64 ?? ''
      if (cursor >= chunk.nextOffset) data = ''
      else if (cursor > start) {
        let text = ''
        for (const byte of bytes.subarray(cursor - start)) text += String.fromCharCode(byte)
        data = btoa(text)
      }
      entry.target.offset = Math.max(cursor, chunk.nextOffset)
      entry.finished = finished; entry.inputOpen = inputOpen; entry.exitError = exitError
      if (entry.target.kind === 'job' && inputOpen !== undefined) entry.target.inputOpen = inputOpen
      // Restarting the shared pump for a new viewer may replay history. Each
      // existing viewer gets only bytes after its own cursor, never a rewind.
      const common = { nextOffset: entry.target.offset, truncated: cursor < start }
      if (payload.output) entry.handlers.output?.({ ...payload.output, ...common, offset: Math.max(cursor, start), data })
      else if (payload.job) entry.handlers.job?.({ ...payload.job, ...common, dataBase64: data })
    }
  }

  private scheduleIdleClose(): void {
    this.clearIdleTimer()
    this.idleTimer = setTimeout(() => {
      this.idleTimer = undefined
      if (!this.entries.size) this.closeSource()
    }, this.idleCloseMs)
  }

  private clearIdleTimer(): void {
    if (this.idleTimer) clearTimeout(this.idleTimer)
    this.idleTimer = undefined
  }

  private canStream(): boolean {
    return Boolean(this.transport.createSource) || typeof EventSource !== 'undefined'
  }

  private ensureSource(): void {
    if (this.source || this.disabled) return
    const create = this.transport.createSource || ((url: string) => new EventSource(url, { withCredentials: true }))
    let source: EventSource
    try {
      source = create(this.transport.url())
    } catch {
      this.disable()
      return
    }
    this.source = source
    this.armReadyTimer()
    source.addEventListener('ready', (event) => {
      if (this.source !== source) return
      this.cancelSubscriptionRequest()
      try {
        const payload = JSON.parse((event as MessageEvent<string>).data) as { streamId?: string }
        if (!payload.streamId) throw new Error('missing stream id')
        this.streamId = payload.streamId
      } catch {
        this.disable()
        return
      }
      this.failures = 0
      this.clearReadyTimer()
      // A reconnect gets a new stream: resubscribe everything at its latest offset.
      this.pendingAdd = new Set(this.entries.keys())
      this.pendingRemove.clear()
      this.scheduleFlush()
      for (const viewers of this.entries.values()) for (const entry of [...viewers]) if (viewers.has(entry)) entry.handlers.connected?.()
    })
    source.addEventListener('output', (event) => {
      if (this.source !== source) return
      let payload: StreamEvent
      try {
        payload = JSON.parse((event as MessageEvent<string>).data) as StreamEvent
      } catch {
        return
      }
      this.deliver(payload.key, payload)
    })
    source.addEventListener('auth.expired', () => { if (this.source === source) this.disable() })
    source.addEventListener('error', () => {
      if (this.source !== source) return
      this.streamId = ''
      this.cancelSubscriptionRequest()
      if (source.readyState === 2 /* CLOSED */) {
        this.failures = MAX_FAILURES_BEFORE_READY
      } else {
        this.failures += 1
      }
      if (this.failures >= MAX_FAILURES_BEFORE_READY) this.disable()
      else this.armReadyTimer()
    })
  }

  private armReadyTimer(): void {
    this.clearReadyTimer()
    // A buffering proxy delivers nothing; fall back instead of waiting forever.
    this.readyTimer = setTimeout(() => {
      if (!this.streamId) this.disable()
    }, READY_TIMEOUT_MS)
  }

  private clearReadyTimer(): void {
    if (this.readyTimer) clearTimeout(this.readyTimer)
    this.readyTimer = undefined
  }

  private scheduleFlush(): void {
    if (this.flushTimer || !this.streamId) return
    this.flushTimer = setTimeout(() => {
      this.flushTimer = undefined
      void this.flush()
    }, SUBSCRIBE_DEBOUNCE_MS)
  }

  private async flush(): Promise<void> {
    if (this.subscriptionRequest || !this.streamId || (!this.pendingAdd.size && !this.pendingRemove.size)) return
    const streamId = this.streamId
    const addKeys = [...this.pendingAdd]
    const entries = new Map(addKeys.map(key => [key, new Set(this.entries.get(key))]))
    const remove = [...this.pendingRemove]
    this.pendingAdd.clear()
    this.pendingRemove.clear()
    const add = addKeys
      .map((key) => this.targetFor(key))
      .filter((target): target is TerminalStreamTarget => Boolean(target))
    const controller = new AbortController()
    this.subscriptionRequest = controller
    try {
      await terminalRequest(controller, signal => this.transport.subscribe({ streamId, add, remove }, signal))
    } catch {
      if (this.subscriptionRequest !== controller || streamId !== this.streamId) return
      if (controller.signal.aborted) {
        // A timed-out add may still reach the server. Closing this stream
        // cancels every pump; a racing remove alone cannot guarantee cleanup.
        this.disable()
        return
      }
      // The server could not take these subscriptions (limit or stale
      // stream): those terminals keep working over polling.
      for (const key of addKeys) {
        const viewers = this.entries.get(key)
        if (!viewers) continue
        const failed = [...viewers].filter(entry => entries.get(key)?.has(entry))
        for (const entry of failed) viewers.delete(entry)
        if (viewers.size) this.pendingAdd.add(key)
        else this.entries.delete(key)
        // The server may have started some of this batch before rejecting
        // the rest; release those pumps instead of leaving them orphaned.
        if (!viewers.size) this.pendingRemove.add(key)
        for (const entry of failed) entry.handlers.unavailable?.()
      }
      this.scheduleFlush()
    } finally {
      if (this.subscriptionRequest === controller) {
        this.subscriptionRequest = undefined
        if (this.pendingAdd.size || this.pendingRemove.size) this.scheduleFlush()
      }
    }
  }

  private cancelSubscriptionRequest(): void {
    const controller = this.subscriptionRequest
    this.subscriptionRequest = undefined
    controller?.abort()
  }

  private disable(): void {
    if (this.disabled) return
    this.disabled = true
    this.closeSource()
    const entries = [...this.entries.values()]
    this.entries.clear()
    for (const viewers of entries) for (const entry of viewers) entry.handlers.unavailable?.()
  }

  private closeSource(): void {
    this.cancelSubscriptionRequest()
    this.clearReadyTimer()
    this.clearIdleTimer()
    if (this.flushTimer) clearTimeout(this.flushTimer)
    this.flushTimer = undefined
    this.source?.close()
    this.source = undefined
    this.streamId = ''
    this.failures = 0
    this.pendingAdd.clear()
    this.pendingRemove.clear()
  }

  /** Allows a later subscription to retry streaming, e.g. after re-login. */
  reset(): void {
    this.closeSource()
    this.disabled = false
  }
}
