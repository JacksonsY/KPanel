import type { Ref } from 'vue'
import { createSSRApp, ssrContextKey, nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import SiteMonitoringView from './SiteMonitoringView.vue'
import { api } from '@/lib/api'
import type { SiteTrafficSnapshot } from '@/types/siteTraffic'
vi.mock('@/lib/api', () => ({ api: { siteTraffic: { summary: vi.fn(), action: vi.fn() } } }))
interface Bindings { data: Ref<SiteTrafficSnapshot | undefined>; error: Ref<string>; loading: Ref<boolean>; host: Ref<string>; busy: Ref<boolean>; load: () => Promise<void>; perform: (action: 'enable' | 'pause' | 'clear') => Promise<void> }
function setup(): Bindings {
 const app = createSSRApp({ render: () => null }); app.provide(ssrContextKey, { modules: new Set<string>() })
 const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
 try { return app.runWithContext(() => (SiteMonitoringView as unknown as { setup: (props: object, ctx: object) => Bindings }).setup({}, { expose: () => undefined })) } finally { warn.mockRestore() }
}
function snapshot(requests = 1): SiteTrafficSnapshot { return { requests, enabled: true } as SiteTrafficSnapshot }
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(r => { resolve = r }); return { promise, resolve } }
afterEach(() => vi.resetAllMocks())
describe('website monitoring request lifecycle', () => {
 it('discards an older response after a newer request has completed', async () => {
  const old = deferred<SiteTrafficSnapshot>(), latest = deferred<SiteTrafficSnapshot>()
  vi.mocked(api.siteTraffic.summary).mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
  const view = setup(); const first = view.load(), second = view.load()
  latest.resolve(snapshot(22)); await second; old.resolve(snapshot(11)); await first
  expect(view.data.value?.requests).toBe(22); expect(view.loading.value).toBe(false)
 })
 it('keeps same-filter history available when a refresh fails', async () => {
  vi.mocked(api.siteTraffic.summary).mockResolvedValueOnce(snapshot(4)).mockRejectedValueOnce(new Error('source unavailable'))
  const view = setup(); await view.load(); await view.load()
  expect(view.data.value?.requests).toBe(4); expect(view.error.value).toBe('source unavailable')
 })
 it('clears old site data before loading a new filter and does not show it after failure', async () => {
  vi.mocked(api.siteTraffic.summary).mockResolvedValueOnce(snapshot(9)).mockRejectedValueOnce(new Error('new site unavailable'))
  const view = setup(); await view.load(); view.host.value = 'other.example'; await nextTick(); await nextTick()
  expect(view.data.value).toBeUndefined(); expect(view.error.value).toBe('new site unavailable')
 })
 it('serializes controls while an action is pending and reloads the result', async () => {
  const action = deferred<{ ok: boolean }>(); vi.mocked(api.siteTraffic.action).mockReturnValue(action.promise)
  vi.mocked(api.siteTraffic.summary).mockResolvedValue(snapshot(0))
  const view = setup(); const pending = view.perform('clear'); await view.perform('enable')
  expect(api.siteTraffic.action).toHaveBeenCalledTimes(1); expect(view.busy.value).toBe(true)
  action.resolve({ ok: true }); await pending
  expect(view.data.value?.requests).toBe(0); expect(view.busy.value).toBe(false)
 })
})
