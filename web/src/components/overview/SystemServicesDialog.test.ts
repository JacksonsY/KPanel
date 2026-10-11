// @vitest-environment jsdom
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import SystemServicesDialog from './SystemServicesDialog.vue'
import { registerPhraseCatalog, resetPhraseLocalizationForTest } from '@/i18n/phrase'
import servicePhrases from '@/i18n/pages/shared/en-US'
import overviewPhrases from '@/i18n/pages/OverviewView/en-US'

const mocks = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn(), logs: vi.fn(), action: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: { system: { services: mocks.list, service: mocks.detail, serviceLogs: mocks.logs, serviceAction: mocks.action } } }))
const service = { name: 'nginx.service', description: 'Web server', loadState: 'loaded', activeState: 'active', subState: 'running', unitFileState: 'enabled', resourceVersion: 'a'.repeat(64), fragmentPath: '/usr/lib/systemd/system/nginx.service', mainPID: 100, canReload: true, canStart: true, canStop: true }
const snapshot = { supported: true, services: [service, { ...service, name: 'backup.service', activeState: 'inactive', description: 'Backup' }], observedAt: '2026-10-11T00:00:00Z', truncated: false }
enableAutoUnmount(afterEach)
afterEach(() => { vi.unstubAllGlobals(); resetPhraseLocalizationForTest() })
beforeEach(() => {
  vi.resetAllMocks()
  mocks.list.mockResolvedValue(snapshot)
  mocks.detail.mockImplementation((name: string) => Promise.resolve({ ...service, name }))
  mocks.logs.mockResolvedValue({ name: service.name, content: 'service log output', truncated: false, observedAt: snapshot.observedAt })
  mocks.action.mockResolvedValue({ name: service.name, action: 'restart', accepted: true, message: 'queued' })
  vi.stubGlobal('confirm', vi.fn(() => true))
})
function dialog(props = {}) { return mount(SystemServicesDialog, { props: { open: true, readable: true, writable: true, ...props }, global: { stubs: { teleport: true } } }) }
async function select(wrapper: ReturnType<typeof dialog>, index = 0) { await flushPromises(); await wrapper.findAll('.services-row')[index]!.trigger('click'); await flushPromises() }
function deferred() { let resolve!: (value: unknown) => void; const promise = new Promise((yes) => { resolve = yes }); return { promise, resolve } }

describe('SystemServicesDialog', () => {
  it('translates teleported content explicitly in English, including log controls', async () => {
    registerPhraseCatalog(servicePhrases)
    registerPhraseCatalog(overviewPhrases)
    const wrapper = dialog()
    await select(wrapper)
    expect(wrapper.find('.services-manager').text()).not.toMatch(/[\u3400-\u9fff]/)
    expect(wrapper.findComponent({ name: 'ModalDialog' }).props('title')).toBe('systemd services')
    expect(wrapper.find('.services-search input').attributes('placeholder')).toBe('Search service names or descriptions')
    expect(wrapper.find('.services-log-header button').text()).toBe('Read logs')
  })
  it('loads only when opened and reads details and logs only on demand', async () => {
    const wrapper = dialog({ open: false })
    expect(mocks.list).not.toHaveBeenCalled()
    await wrapper.setProps({ open: true })
    await flushPromises()
    expect(wrapper.findAll('.services-row')).toHaveLength(2)
    expect(mocks.detail).not.toHaveBeenCalled()
    expect(mocks.logs).not.toHaveBeenCalled()
    await select(wrapper)
    expect(mocks.detail).toHaveBeenCalledWith('nginx.service', expect.any(AbortSignal))
    await wrapper.findAll('button').find((button) => button.text() === '读取日志')!.trigger('click')
    await flushPromises()
    expect(wrapper.find('pre').text()).toContain('service log output')
  })
  it('filters names, descriptions and status without additional host reads', async () => {
    const wrapper = dialog()
    await flushPromises()
    await wrapper.find('input').setValue('backup')
    expect(wrapper.findAll('.services-row')).toHaveLength(1)
    await wrapper.find('select').setValue('active')
    expect(wrapper.text()).toContain('没有匹配的服务。')
    expect(mocks.list).toHaveBeenCalledTimes(1)
  })
  it('uses the selected resource version, confirms, and distinguishes queued from completed', async () => {
    const wrapper = dialog()
    await select(wrapper)
    await wrapper.find('[data-action="restart"]').trigger('click')
    await flushPromises()
    expect(mocks.action).toHaveBeenCalledExactlyOnceWith({ name: service.name, action: 'restart', expectedResourceVersion: service.resourceVersion })
    expect(wrapper.text()).toContain('请求已提交给 systemd；请刷新确认最终状态。')
    expect(mocks.list).toHaveBeenCalledTimes(2)
    expect(mocks.detail).toHaveBeenCalledTimes(2)
    mocks.action.mockResolvedValue({ accepted: false })
    await wrapper.find('[data-action="disable"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('开机自启设置已更新，并已重新读取服务状态。')
  })
  it('does not write when confirmation is cancelled or write capability is disabled', async () => {
    const wrapper = dialog()
    await select(wrapper)
    vi.stubGlobal('confirm', vi.fn(() => false))
    await wrapper.find('[data-action="restart"]').trigger('click')
    expect(mocks.action).not.toHaveBeenCalled()
    await wrapper.setProps({ writable: false })
    expect(wrapper.find('[data-action="restart"]').attributes('disabled')).toBeDefined()
  })
  it('retains failure feedback and refreshes rather than automatically retrying a write', async () => {
    mocks.action.mockRejectedValue(new Error('resource version changed'))
    const wrapper = dialog()
    await select(wrapper)
    await wrapper.find('[data-action="stop"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('resource version changed')
    expect(wrapper.text()).toContain('请勿重复提交')
    expect(mocks.action).toHaveBeenCalledTimes(1)
    expect(mocks.detail).toHaveBeenCalledTimes(2)
  })
  it('rejects stale detail and log results after selecting another service', async () => {
    const old = deferred()
    mocks.detail.mockReturnValueOnce(old.promise)
    const wrapper = dialog()
    await select(wrapper)
    await select(wrapper, 1)
    old.resolve({ ...service, description: 'STALE' })
    await flushPromises()
    expect(wrapper.find('.services-detail').text()).not.toContain('STALE')
    expect(wrapper.find('.services-detail h3').text()).toBe('backup.service')
  })
  it('aborts reads on close and ignores delayed results after reopening', async () => {
    const old = deferred()
    mocks.list.mockReturnValueOnce(old.promise)
    const wrapper = dialog()
    const signal = mocks.list.mock.calls[0]![0] as AbortSignal
    await wrapper.setProps({ open: false })
    expect(signal.aborted).toBe(true)
    await wrapper.setProps({ open: true })
    await flushPromises()
    old.resolve({ ...snapshot, services: [] })
    await flushPromises()
    expect(wrapper.findAll('.services-row')).toHaveLength(2)
  })
  it('shows read failures and unsupported hosts without offering writes', async () => {
    mocks.list.mockRejectedValueOnce(new Error('system bus unavailable'))
    const wrapper = dialog()
    await flushPromises()
    expect(wrapper.text()).toContain('system bus unavailable')
    mocks.list.mockResolvedValue({ supported: false, services: [], reason: 'OpenRC host' })
    await wrapper.find('.services-toolbar button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('OpenRC host')
    expect(wrapper.findAll('[data-action]')).toHaveLength(0)
  })
})
