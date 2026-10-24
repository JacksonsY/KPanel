// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import Settings from './ClusterRemoteAccessSettings.vue'
import { clusterRemoteAccessChanged } from '@/lib/clusterSettings'
import { ApiError } from '@/lib/api'
const mocks = vi.hoisted(() => ({ read: vi.fn(), save: vi.fn(), hostRead: vi.fn(), hostSave: vi.fn() }))
vi.mock('@/lib/api', () => ({ ApiError: class extends Error { constructor(message: string, public status: number) { super(message) } }, api: { cluster: { remoteAccess: mocks.read, updateRemoteAccess: mocks.save, hostRemoteAccess: mocks.hostRead, updateHostRemoteAccess: mocks.hostSave } } }))
vi.mock('@/i18n/phrase', () => ({ usePhraseCatalog: vi.fn(), phraseCatalogVersion: { value: 1 }, translatePhrase: (value: string) => value }))
const snapshot = { terminalEnabled: true, filesEnabled: true, available: true, resourceVersion: 'v1' }
const wrappers: ReturnType<typeof mount>[] = []
function open() { const wrapper = mount(Settings, { props: { open: true, active: true, formId: 'remote-form' } }); wrappers.push(wrapper); return wrapper }
beforeEach(() => { vi.clearAllMocks(); mocks.read.mockResolvedValue({ ...snapshot }); mocks.save.mockResolvedValue({ ...snapshot, terminalEnabled: false, resourceVersion: 'v2' }) })
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

describe('center remote operation settings', () => {
  it('edits one host while explaining the global override', async () => {
    mocks.hostRead.mockResolvedValue({ ...snapshot, hostId: 'host-a', globalTerminalEnabled: false, globalFilesEnabled: true })
    mocks.hostSave.mockResolvedValue({ ...snapshot, hostId: 'host-a', terminalEnabled: false, globalTerminalEnabled: false, globalFilesEnabled: true })
    const wrapper = mount(Settings, { props: { open: true, active: true, formId: 'host-form', hostId: 'host-a' } }); wrappers.push(wrapper)
    await flushPromises()
    expect(mocks.hostRead).toHaveBeenCalledWith('host-a', expect.any(AbortSignal))
    expect(mocks.read).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('集群总开关已关闭，此项暂不生效')
    await wrapper.findAll('input')[0]!.setValue(false)
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(mocks.hostSave).toHaveBeenCalledWith('host-a', { terminalEnabled: false, filesEnabled: true, expectedResourceVersion: 'v1' })
    expect(mocks.save).not.toHaveBeenCalled()
  })
  it('defaults on, saves the server version, and refreshes host capabilities', async () => {
    const changed = vi.fn(); window.addEventListener(clusterRemoteAccessChanged, changed)
    const wrapper = open(); await flushPromises()
    expect(wrapper.findAll('input').map(input => (input.element as HTMLInputElement).checked)).toEqual([true, true])
    await wrapper.findAll('input')[0]!.setValue(false)
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith({ terminalEnabled: false, filesEnabled: true, expectedResourceVersion: 'v1' })
    expect(changed).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('远程操作设置已保存')
    window.removeEventListener(clusterRemoteAccessChanged, changed)
  })
  it('keeps drafts on tab switches and reloads on the next opening', async () => {
    const wrapper = open(); await flushPromises()
    await wrapper.findAll('input')[1]!.setValue(false)
    await wrapper.setProps({ active: false }); await wrapper.setProps({ active: true }); await flushPromises()
    expect(mocks.read).toHaveBeenCalledTimes(1)
    expect((wrapper.findAll('input')[1]!.element as HTMLInputElement).checked).toBe(false)
    await wrapper.setProps({ open: false }); await wrapper.setProps({ open: true }); await flushPromises()
    expect(mocks.read).toHaveBeenCalledTimes(2)
    expect((wrapper.findAll('input')[1]!.element as HTMLInputElement).checked).toBe(true)
  })
  it('retains an unsaved selection after a conflicting save and offers reload', async () => {
    mocks.save.mockRejectedValue(new ApiError('changed', 409))
    const wrapper = open(); await flushPromises(); await wrapper.findAll('input')[0]!.setValue(false)
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('其他页面更新')
    expect((wrapper.findAll('input')[0]!.element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.find('button').text()).toContain('重新读取设置')
  })
  it('allows an unchanged policy to retry unconfirmed session cleanup', async () => {
    mocks.save.mockResolvedValue({ ...snapshot, terminalEnabled: false, resourceVersion: 'v2', pendingTerminalCleanup: 1 })
    const wrapper = open(); await flushPromises(); await wrapper.findAll('input')[0]!.setValue(false)
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(wrapper.text()).toContain('部分远端会话尚待关闭')
    expect(wrapper.emitted('ready')?.at(-1)).toEqual([true])
  })
  it('never enables edits when saved settings cannot be read', async () => {
    mocks.read.mockResolvedValue({ terminalEnabled: false, filesEnabled: false, resourceVersion: '', available: false })
    const wrapper = open(); await flushPromises()
    expect(wrapper.find('fieldset').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[role="alert"]').text()).toContain('远程操作已停用')
    expect(wrapper.emitted('ready')?.at(-1)).toEqual([false])
  })
})
