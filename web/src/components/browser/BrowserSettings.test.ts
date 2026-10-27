// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BrowserSettings from './BrowserSettings.vue'
import { api } from '@/lib/api'
import type { BrowserComponent } from '@/lib/webBrowser'

vi.mock('@/lib/api', () => ({ api: { browser: { component: vi.fn(), componentAction: vi.fn() }, preferences: { get: vi.fn(), update: vi.fn() } } }))
const version = `sha256:${'a'.repeat(64)}`
const snapshot = (patch: Partial<BrowserComponent> = {}): BrowserComponent => ({ supported: true, installed: false, running: false, healthy: false, resourceVersion: version, image: 'fixed-image', ...patch })
function render() { return mount(BrowserSettings, { props: { open: true }, global: { stubs: { ModalDialog: { props: ['open'], template: '<div v-if="open"><slot/><slot name="footer"/></div>' } } } }) }
function button(wrapper: ReturnType<typeof render>, text: string) { const found = wrapper.findAll('button').find(b => b.text() === text); if (!found) throw new Error(text); return found }
describe('browser settings lifecycle', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.browser.component).mockResolvedValue(snapshot())
    vi.mocked(api.preferences.get).mockResolvedValue({ resourceVersion: version, locale: 'zh-CN' })
  })
  afterEach(() => vi.useRealTimers())
  it('submits a version-bound installation and follows the persisted task', async () => {
    vi.useFakeTimers()
    const wrapper = render(); await flushPromises()
    const job = { id: 'job-one', action: 'install' as const, state: 'queued', stage: 'queued' }
    vi.mocked(api.browser.componentAction).mockResolvedValue(job)
    vi.mocked(api.browser.component).mockResolvedValue(snapshot({ job }))
    await button(wrapper, '安装并启用').trigger('click'); await flushPromises()
    expect(api.browser.componentAction).toHaveBeenCalledWith('install', version)
    expect(wrapper.text()).toContain('后台正在处理组件')
    vi.mocked(api.browser.component).mockResolvedValue(snapshot({ installed: true, running: true, healthy: true, job: { ...job, state: 'succeeded' } }))
    await vi.advanceTimersByTimeAsync(2000); await flushPromises()
    expect(wrapper.text()).toContain('服务已就绪')
    expect(wrapper.emitted('changed')).toEqual([['install']])
    wrapper.unmount()
  })
  it('requires one confirmation before uninstalling and retains failure recovery', async () => {
    vi.mocked(api.browser.component).mockResolvedValue(snapshot({ installed: true, running: true, healthy: true }))
    const wrapper = render(); await flushPromises()
    await button(wrapper, '卸载组件').trigger('click')
    expect(api.browser.componentAction).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('起始页设置和共享镜像保留')
    vi.mocked(api.browser.componentAction).mockRejectedValue(new Error('conflict'))
    await button(wrapper, '确定').trigger('click'); await flushPromises()
    expect(api.browser.componentAction).toHaveBeenCalledWith('uninstall', version)
    expect(wrapper.text()).toContain('操作提交未确认或状态已变化')
    expect(api.browser.component).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('saves only the browser preference and rejects credential-bearing URLs', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('input').setValue('https://user:secret@example.com')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.preferences.update).not.toHaveBeenCalled()
    await wrapper.get('input').setValue('https://example.com/start')
    vi.mocked(api.preferences.update).mockResolvedValue({ resourceVersion: 'new-version', browser: { startURL: 'https://example.com/start' } })
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.preferences.update).toHaveBeenCalledWith({ browser: { startURL: 'https://example.com/start' }, expectedResourceVersion: version }, expect.any(AbortSignal))
    expect(wrapper.emitted('saved')).toEqual([['https://example.com/start']])
    wrapper.unmount()
  })
  it('offers upgrade guidance when the old Compose network is absent', async () => {
    vi.mocked(api.browser.component).mockResolvedValue(snapshot({ supported: false, reason: 'panel_compose_upgrade_required' }))
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('先使用此版本的 Compose 配置更新面板')
    expect(wrapper.findAll('button').some(b => b.text() === '安装并启用')).toBe(false)
    wrapper.unmount()
  })
  it('reads existing completed jobs without replaying their action', async () => {
    vi.mocked(api.browser.component).mockResolvedValue(snapshot({ job: { id: 'old', action: 'stop', state: 'succeeded', stage: 'complete' } }))
    const wrapper = render(); await flushPromises()
    expect(wrapper.emitted('changed')).toBeUndefined()
    wrapper.unmount()
  })
})
