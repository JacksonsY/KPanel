import { api, ApiError } from '@/lib/api'
import { getLocale, setLocale, t } from '@/i18n'
import { applyDesktopPreferences, desktopPreferencesSnapshot } from '@/stores/desktopMode'
import { useToast } from '@/stores/toast'
import type { UserPreferences, UserPreferencesPatch } from '@/types/api'

interface Sync {
  owner: string
  controller: AbortController
  initial: UserPreferencesPatch
  pending: UserPreferencesPatch
  server?: UserPreferences
  loaded: boolean
  loading?: Promise<void>
  saving?: Promise<void>
  timer?: number
  retries: number
  notified: boolean
  hydrating: boolean
}
let current: Sync | undefined
const pendingKey = 'kpanel:preferences-pending:v1'

function readPending(owner: string): UserPreferencesPatch {
  try {
    const raw = sessionStorage.getItem(pendingKey)
    if (!raw || raw.length > 64000) return {}
    const value = JSON.parse(raw)
    if (value.owner !== owner) return {}
    const patch: UserPreferencesPatch = {}
    if (['zh-CN', 'zh-TW', 'en-US'].includes(value.patch?.locale)) patch.locale = value.patch.locale
    const d = value.patch?.desktop
    if (d && ['classic', 'desktop'].includes(d.mode) && Array.isArray(d.windows) && d.windows.length <= 8
      && Array.isArray(d.windowSizes) && d.windowSizes.length <= 32 && typeof d.sideSplitRatio === 'number'
      && typeof d.focusedId === 'number'
      && d.windows.every((w: unknown) => w && typeof w === 'object' && typeof (w as { path?: unknown }).path === 'string')
      && d.windowSizes.every((w: unknown) => w && typeof w === 'object')) patch.desktop = d
    return patch
  } catch { return {} }
}

function persist(sync: Sync): void {
  try {
    if (Object.keys(sync.pending).length) sessionStorage.setItem(pendingKey, JSON.stringify({ owner: sync.owner, patch: sync.pending }))
    else sessionStorage.removeItem(pendingKey)
  } catch { /* In-memory edits still retry when tab storage is disabled. */ }
}

function requestSignal(sync: Sync): AbortSignal {
  return AbortSignal.any([sync.controller.signal, AbortSignal.timeout(8000)])
}

function failed(sync: Sync): void {
  if (current !== sync) return
  if (!sync.notified) {
    useToast().danger(t('settings.preferencesSyncFailed'), t('settings.preferencesSyncRetry'))
    sync.notified = true
  }
  if (sync.retries < 4) schedule(sync, 1000 * 2 ** sync.retries++)
}

function schedule(sync: Sync, delay = 450): void {
  window.clearTimeout(sync.timer)
  sync.timer = window.setTimeout(() => {
    if (current !== sync) return
    if (!sync.loaded) void load(sync)
    else void save(sync)
  }, delay)
}

async function load(sync: Sync): Promise<void> {
  if (sync.loading) return sync.loading
  sync.loading = (async () => {
    try {
      const server = await api.preferences.get(requestSignal(sync))
      if (current !== sync) return
      sync.server = server
      // Migrate each unconfigured section once. A competing configured value
      // cancels implicit migration; only explicit edits are retried on conflict.
      const migration: UserPreferencesPatch = {}
      if (!server.locale) migration.locale = sync.initial.locale
      if (!server.desktop) migration.desktop = sync.initial.desktop
      sync.loaded = true
      sync.retries = 0
      sync.notified = false
      await save(sync, migration)
      if (current !== sync) return
      const locale = sync.pending.locale ?? sync.server?.locale ?? migration.locale
      if (locale) await setLocale(locale, false)
      if (current !== sync) return
      const desktop = sync.pending.desktop ?? sync.server?.desktop
      if (sync.hydrating && desktop) applyDesktopPreferences(desktop)
    } catch {
      failed(sync)
    } finally { sync.loading = undefined }
  })()
  return sync.loading
}

function save(sync: Sync, migration: UserPreferencesPatch = {}): Promise<void> {
  if (sync.saving) return sync.saving
  if (!sync.server || (!Object.keys(migration).length && !Object.keys(sync.pending).length)) return Promise.resolve()
  window.clearTimeout(sync.timer)
  sync.saving = (async () => {
    let conflicts = 0
    try {
      while (current === sync && sync.server && (Object.keys(migration).length || Object.keys(sync.pending).length)) {
        const patch = { ...migration, ...sync.pending }
        try {
          const result = await api.preferences.update({ ...patch, expectedResourceVersion: sync.server.resourceVersion }, requestSignal(sync))
          if (current !== sync) return
          sync.server = result
          migration = {}
          for (const key of Object.keys(patch) as (keyof UserPreferencesPatch)[]) {
            if (JSON.stringify(sync.pending[key]) === JSON.stringify(patch[key])) delete sync.pending[key]
          }
          persist(sync)
        } catch (error) {
          if (current !== sync) return
          if (!(error instanceof ApiError && error.status === 409) || ++conflicts > 3) throw error
          const server = await api.preferences.get(requestSignal(sync))
          if (current !== sync) return
          sync.server = server
          if (server.locale) delete migration.locale
          if (server.desktop) delete migration.desktop
          // Do not remount live windows on a conflict; the next login/refresh
          // restores server state. Only dirty sections are written back.
        }
      }
      sync.retries = 0
      sync.notified = false
    } catch {
      // Retry implicit migration from a fresh GET, never a stale full snapshot.
      if (Object.keys(migration).length) sync.loaded = false
      failed(sync)
    }
    finally { sync.saving = undefined }
  })()
  return sync.saving
}

function changed(event: Event): void {
  const sync = current
  if (!sync) return
  Object.assign(sync.pending, (event as CustomEvent<UserPreferencesPatch>).detail)
  persist(sync)
  sync.retries = 0
  schedule(sync)
}

function retry(): void {
  if (!current) return
  current.retries = 0
  schedule(current, 0)
}

/** Called before the authenticated shell mounts, so cached windows cannot launch first. */
export function startPreferencesSync(owner: string): Promise<void> {
  if (current?.owner === owner) return current.loading ?? Promise.resolve()
  stopPreferencesSync()
  const sync: Sync = {
    owner, controller: new AbortController(), initial: { locale: getLocale(), desktop: desktopPreferencesSnapshot() },
    pending: readPending(owner), loaded: false, retries: 0, notified: false,
    hydrating: true,
  }
  current = sync
  window.addEventListener('kpanel:preferences-changed', changed)
  window.addEventListener('online', retry)
  window.addEventListener('focus', retry)
  return load(sync).finally(() => { sync.hydrating = false })
}

export function stopPreferencesSync(): void {
  if (current) {
    window.clearTimeout(current.timer)
    current.controller.abort()
  }
  current = undefined
  window.removeEventListener('kpanel:preferences-changed', changed)
  window.removeEventListener('online', retry)
  window.removeEventListener('focus', retry)
}
