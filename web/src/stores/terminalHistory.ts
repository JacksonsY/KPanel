import { readonly, ref } from 'vue'
import { api } from '@/lib/api'
import type { TerminalHistory, TerminalHistoryEntry } from '@/types/api'

// The panel keeps one shared, newest-first history. Writes go through one
// serial queue so an append that is still in flight can never resurrect a
// command after it was removed or cleared; appends recorded while a request
// is running are coalesced into the next request.
const MAX_ENTRIES = 200
const MAX_APPEND_ITEMS = 32
const MAX_PENDING = 128
const MAX_COMMAND_BYTES = 8192
const encoder = new TextEncoder()

type PendingAppend = Pick<TerminalHistoryEntry, 'command' | 'hostId'>

function emptyHistory(): TerminalHistory {
  return { schemaVersion: 1, available: true, items: [] }
}

const history = ref<TerminalHistory>(emptyHistory())
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const syncFailed = ref(false)
let pending: PendingAppend[] = []
let inFlight: PendingAppend[] = []
let flushQueued = false
let tail: Promise<unknown> = Promise.resolve()
let loadSequence = 0
let mutationSequence = 0

/** Returns the stored form of a command, or undefined when it cannot be kept. */
export function normalizeHistoryCommand(value: string): string | undefined {
  // Preserve the shell's ignorespace privacy convention for every recording path.
  if (/^\s/.test(value)) return undefined
  const command = value.replace(/\r\n?/g, '\n').trimEnd()
  if (!command || encoder.encode(command).byteLength > MAX_COMMAND_BYTES) return undefined
  if (/[\u0000-\u0008\u000b-\u001f\u007f-\u009f]/.test(command)) return undefined
  return command
}

function withPending(snapshot: TerminalHistory): TerminalHistory {
  const unsaved = [...inFlight, ...pending]
  if (!snapshot.available || !unsaved.length) return snapshot
  const now = Date.now()
  let items = snapshot.items
  for (const item of unsaved) {
    items = [{ ...item, at: now }, ...items.filter((entry) => entry.command !== item.command)]
  }
  return { ...snapshot, items: items.slice(0, MAX_ENTRIES) }
}

function enqueue<T>(operation: () => Promise<T>): Promise<T> {
  const run = tail.then(operation)
  tail = run.catch(() => undefined)
  return run
}

async function load(signal?: AbortSignal): Promise<TerminalHistory> {
  const sequence = ++loadSequence
  const mutation = mutationSequence
  loading.value = true
  try {
    const value = await api.terminals.history(signal)
    // A write that finished meanwhile already returned a newer snapshot.
    if (sequence === loadSequence && mutation === mutationSequence) {
      history.value = withPending(value)
      loaded.value = true
    }
    return value
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

function ensureLoaded(): Promise<void> {
  if (loaded.value || loading.value) return Promise.resolve()
  return load().then(() => undefined, () => undefined)
}

function applySnapshot(snapshot: TerminalHistory): void {
  mutationSequence += 1
  history.value = withPending(snapshot)
  loaded.value = true
}

async function flushPending(): Promise<void> {
  flushQueued = false
  while (pending.length) {
    inFlight = pending.slice(0, MAX_APPEND_ITEMS)
    pending = pending.slice(inFlight.length)
    try {
      const snapshot = await api.terminals.appendHistory({ items: inFlight })
      inFlight = []
      applySnapshot(snapshot)
      syncFailed.value = false
    } catch {
      // History is a convenience: a failed write is reported once and dropped
      // instead of retried forever against an unavailable or damaged store.
      inFlight = []
      history.value = withPending(history.value)
      syncFailed.value = true
    }
  }
}

/** Records a command that just ran on a host ('' for a batch run). */
function record(command: string, hostId: string): void {
  const normalized = normalizeHistoryCommand(command)
  if (!normalized || !history.value.available) return
  pending = [...pending.filter((item) => item.command !== normalized), { command: normalized, hostId }]
  if (pending.length > MAX_PENDING) pending = pending.slice(-MAX_PENDING)
  history.value = withPending({ ...history.value, items: history.value.items })
  if (flushQueued) return
  flushQueued = true
  void enqueue(flushPending)
}

function remove(command: string): Promise<TerminalHistory> {
  pending = pending.filter((item) => item.command !== command)
  history.value = { ...history.value, items: history.value.items.filter((item) => item.command !== command) }
  return enqueue(async () => {
    saving.value = true
    try {
      const snapshot = await api.terminals.removeHistory(command)
      applySnapshot(snapshot)
      return snapshot
    } catch (error) {
      await load().catch(() => undefined)
      throw error
    } finally {
      saving.value = false
    }
  })
}

function clear(): Promise<TerminalHistory> {
  pending = []
  return enqueue(async () => {
    saving.value = true
    try {
      pending = []
      const snapshot = await api.terminals.clearHistory()
      applySnapshot(snapshot)
      syncFailed.value = false
      return snapshot
    } catch (error) {
      await load().catch(() => undefined)
      throw error
    } finally {
      saving.value = false
    }
  })
}

export function useTerminalHistory() {
  return {
    history: readonly(history),
    loaded: readonly(loaded),
    loading: readonly(loading),
    saving: readonly(saving),
    syncFailed: readonly(syncFailed),
    load,
    ensureLoaded,
    record,
    remove,
    clear,
  }
}

export function resetTerminalHistoryForTest(): void {
  history.value = emptyHistory()
  loaded.value = false
  loading.value = false
  saving.value = false
  syncFailed.value = false
  pending = []
  inFlight = []
  flushQueued = false
  tail = Promise.resolve()
  loadSequence += 1
  mutationSequence = 0
}
