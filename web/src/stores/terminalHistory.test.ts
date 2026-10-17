import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from '@/lib/api'
import type { TerminalHistory, TerminalHistoryAppend } from '@/types/api'
import { normalizeHistoryCommand, resetTerminalHistoryForTest, useTerminalHistory } from './terminalHistory'

function snapshot(commands: Array<[string, string]>, available = true): TerminalHistory {
  return { schemaVersion: 1, available, items: commands.map(([command, hostId], index) => ({ command, hostId, at: 1000 - index })) }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail })
  return { promise, resolve, reject }
}

const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0))

describe('terminal history store', () => {
  beforeEach(() => {
    resetTerminalHistoryForTest()
    vi.restoreAllMocks()
  })
  afterEach(() => resetTerminalHistoryForTest())

  it('normalizes commands and refuses ones the panel cannot store', () => {
    expect(normalizeHistoryCommand('  docker ps \r\n')).toBeUndefined()
    expect(normalizeHistoryCommand('docker ps \r\n')).toBe('docker ps')
    expect(normalizeHistoryCommand('for i in 1 2\r\ndo\techo $i\r\ndone')).toBe('for i in 1 2\ndo\techo $i\ndone')
    expect(normalizeHistoryCommand('   ')).toBeUndefined()
    expect(normalizeHistoryCommand('echo \x1b[2J')).toBeUndefined()
    expect(normalizeHistoryCommand('x'.repeat(8193))).toBeUndefined()
  })

  it('never persists space-prefixed batch or direct commands', async () => {
    const append = vi.spyOn(api.terminals, 'appendHistory')
    const store = useTerminalHistory()
    store.record(' secret --token private', '')
    store.record('\tsecret --token private', 'local')
    await flush()
    expect(append).not.toHaveBeenCalled()
    expect(store.history.value.items).toEqual([])
  })

  it('shows a command at once and coalesces commands recorded while a write is in flight', async () => {
    const first = deferred<TerminalHistory>()
    const bodies: TerminalHistoryAppend[] = []
    vi.spyOn(api.terminals, 'appendHistory').mockImplementation((body) => {
      bodies.push(body)
      return bodies.length === 1 ? first.promise : Promise.resolve(snapshot([['df -h', 'b'], ['free -m', 'a'], ['uptime', 'a']]))
    })
    const store = useTerminalHistory()
    store.record('uptime', 'a')
    await flush()
    store.record('free -m', 'a')
    store.record('df -h', 'b')
    expect(store.history.value.items.map((item) => item.command)).toEqual(['df -h', 'free -m', 'uptime'])
    first.resolve(snapshot([['uptime', 'a']]))
    await flush()
    await flush()
    expect(bodies).toEqual([
      { items: [{ command: 'uptime', hostId: 'a' }] },
      { items: [{ command: 'free -m', hostId: 'a' }, { command: 'df -h', hostId: 'b' }] },
    ])
    expect(store.history.value.items.map((item) => item.command)).toEqual(['df -h', 'free -m', 'uptime'])
  })

  it('removes after an in-flight append so the command cannot come back', async () => {
    const append = deferred<TerminalHistory>()
    const calls: string[] = []
    vi.spyOn(api.terminals, 'appendHistory').mockImplementation(() => { calls.push('append'); return append.promise })
    vi.spyOn(api.terminals, 'removeHistory').mockImplementation(async () => { calls.push('remove'); return snapshot([]) })
    const store = useTerminalHistory()
    store.record('rm -rf /tmp/cache', 'a')
    await flush()
    const removal = store.remove('rm -rf /tmp/cache')
    expect(store.history.value.items).toEqual([])
    append.resolve(snapshot([['rm -rf /tmp/cache', 'a']]))
    await removal
    expect(calls).toEqual(['append', 'remove'])
    expect(store.history.value.items).toEqual([])
  })

  it('reports a failed write once and drops it instead of retrying forever', async () => {
    const append = vi.spyOn(api.terminals, 'appendHistory')
      .mockRejectedValueOnce(new ApiError('unavailable', 503, 'terminal_history_unavailable'))
      .mockResolvedValueOnce(snapshot([['ls', 'a']]))
    const store = useTerminalHistory()
    store.record('uptime', 'a')
    await flush()
    await flush()
    expect(store.syncFailed.value).toBe(true)
    store.record('ls', 'a')
    await flush()
    await flush()
    expect(append).toHaveBeenCalledTimes(2)
    expect(append.mock.calls[1]![0]).toEqual({ items: [{ command: 'ls', hostId: 'a' }] })
    expect(store.syncFailed.value).toBe(false)
  })

  it('keeps unsent commands on top of a reloaded snapshot', async () => {
    const append = deferred<TerminalHistory>()
    vi.spyOn(api.terminals, 'appendHistory').mockReturnValue(append.promise)
    vi.spyOn(api.terminals, 'history').mockResolvedValueOnce(snapshot([['ls', 'b']]))
    const store = useTerminalHistory()
    store.record('uptime', 'a')
    await store.load()
    expect(store.history.value.items.map((item) => item.command)).toEqual(['uptime', 'ls'])
    append.resolve(snapshot([['uptime', 'a'], ['ls', 'b']]))
    await flush()
  })

  it('stops recording while the stored history is damaged', async () => {
    const recordSpy = vi.spyOn(api.terminals, 'appendHistory')
    vi.spyOn(api.terminals, 'history').mockResolvedValueOnce(snapshot([], false))
    const store = useTerminalHistory()
    await store.load()
    store.record('uptime', 'a')
    await flush()
    expect(store.history.value.available).toBe(false)
    expect(recordSpy).not.toHaveBeenCalled()
  })

  it('drops commands that were not sent yet when the list is cleared', async () => {
    const append = vi.spyOn(api.terminals, 'appendHistory').mockResolvedValue(snapshot([['ls', 'a']]))
    const clear = vi.spyOn(api.terminals, 'clearHistory').mockResolvedValue(snapshot([]))
    const store = useTerminalHistory()
    store.record('ls', 'a')
    await store.clear()
    expect(append).not.toHaveBeenCalled()
    expect(clear).toHaveBeenCalledTimes(1)
    expect(store.history.value.items).toEqual([])
  })
})
