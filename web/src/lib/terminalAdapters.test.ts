import { beforeEach, describe, expect, it, vi } from 'vitest'
import { hostTerminalAdapter, jobTerminalAdapter } from './terminalAdapters'

const mocks = vi.hoisted(() => {
  const backend = () => ({ terminal: vi.fn(), terminalInput: vi.fn(), terminalResize: vi.fn() })
  return { host: { output: vi.fn(), resize: vi.fn() }, app: backend(), site: backend(), diagnostic: backend(), environment: backend(), subscribe: vi.fn() }
})
vi.mock('@/lib/api', () => ({ api: {
  terminals: mocks.host, apps: mocks.app, sites: mocks.site, diagnostics: mocks.diagnostic,
  webEnvironment: mocks.environment,
}, terminalStream: { subscribe: mocks.subscribe } }))

beforeEach(() => vi.resetAllMocks())
describe('terminal adapters', () => {
  it('maps host lifecycle and forwards cancellation with its captured identity', async () => {
    mocks.host.output.mockResolvedValue({ data: 'eA==', nextOffset: 1, closed: false, exitedAt: 'now' })
    const adapter = hostTerminalAdapter('session-a'), signal = new AbortController().signal
    expect(await adapter.read(0, true, signal)).toMatchObject({ data: 'eA==', finished: true, inputOpen: false })
    expect(mocks.host.output).toHaveBeenCalledWith('session-a', 0, signal)
    mocks.host.resize.mockResolvedValue({ accepted: false })
    await expect(adapter.resize!(24, 80)).rejects.toThrow('not accepted')
    expect(mocks.host.resize).toHaveBeenCalledWith('session-a', 24, 80)
    expect(adapter.missing('terminal_not_found')).toBe(true)
    expect(adapter.missing('forbidden')).toBe(false)
  })
  it.each(['app', 'site', 'diagnostic', 'environment'] as const)('routes %s jobs without exposing process lifecycle actions', async kind => {
    const backend = mocks[kind], adapter = jobTerminalAdapter(kind, 'same-id')
    const signal = new AbortController().signal
    backend.terminal.mockResolvedValue({ dataBase64: 'eA==', nextOffset: 4, finished: true, inputOpen: true })
    expect(await adapter.read(3, true, signal)).toMatchObject({ data: 'eA==', nextOffset: 4, finished: true, inputOpen: false })
    expect(backend.terminal).toHaveBeenCalledWith('same-id', 3, true, signal)
    expect(adapter.key).toBe(`job:${kind}:same-id`)
    expect(adapter.input.legacyText).toBe(true)
    await adapter.input.legacy('x')
    expect(backend.terminalInput).toHaveBeenCalledWith('same-id', 'x')
    expect('close' in adapter || 'create' in adapter || 'cancel' in adapter).toBe(false)
    if (kind === 'app') {
      backend.terminalResize.mockResolvedValue({ accepted: false })
      await expect(adapter.resize!(24, 80)).rejects.toThrow('not accepted')
    } else expect(adapter.resize).toBeUndefined()
  })
})
