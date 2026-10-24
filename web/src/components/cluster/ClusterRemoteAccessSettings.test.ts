// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import Settings from './ClusterRemoteAccessSettings.vue'
import { clusterRemoteAccessChanged } from '@/lib/clusterSettings'
import { ApiError } from '@/lib/api'
import type { ClusterHost } from '@/types/api'
const mocks = vi.hoisted(() => ({ read: vi.fn(), save: vi.fn(), hostRead: vi.fn(), hostSave: vi.fn() }))
vi.mock('@/lib/api', () => ({ ApiError: class extends Error { constructor(message: string, public status: number) { super(message) } }, api: { cluster: { remoteAccess: mocks.read, updateRemoteAccess: mocks.save, hostRemoteAccess: mocks.hostRead, updateHostRemoteAccess: mocks.hostSave } } }))
vi.mock('@/i18n/phrase', () => ({ usePhraseCatalog: vi.fn(), phraseCatalogVersion: { value: 1 }, translatePhrase: (value: string) => value }))
const snapshot = { terminalEnabled: true, filesEnabled: true, available: true, resourceVersion: 'v1' }
const host = (id: string, isLocal = false) => ({ id, name: id, origin: 'https://' + id + '.example', isLocal }) as ClusterHost
const wrappers: ReturnType<typeof mount>[] = []
function open(hosts: ClusterHost[] = []) {
  const wrapper = mount(Settings, { props: { open: true, active: true, formId: 'remote-form', hosts } })
  wrappers.push(wrapper); return wrapper
}
function input(wrapper: ReturnType<typeof mount>, name: string) {
  return wrapper.find('input[aria-label="' + name + '"]')
}
async function submit(wrapper: ReturnType<typeof mount>) {
  await wrapper.find('form').trigger('submit'); await flushPromises()
}
beforeEach(() => {
  vi.clearAllMocks()
  mocks.read.mockResolvedValue({ ...snapshot })
  let revision = 1
  mocks.save.mockImplementation(async body => ({ ...snapshot, ...body, resourceVersion: 'v' + ++revision }))
  mocks.hostRead.mockImplementation(async id => ({ ...snapshot, hostId: id }))
  mocks.hostSave.mockImplementation(async (id, body) => ({ ...snapshot, ...body, hostId: id, resourceVersion: 'v' + ++revision }))
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

describe('central remote operation settings', () => {
  it('lists remote hosts together, excludes local, and saves only the edited host', async () => {
    const wrapper = open([host('local', true), host('host-a'), host('host-b')]); await flushPromises()
    expect(wrapper.findAll('[data-host-id]').length).toBe(2)
    expect(mocks.hostRead).not.toHaveBeenCalledWith('local', expect.anything())
    await input(wrapper, 'host-a · 允许远程文件管理').setValue(false)
    await submit(wrapper)
    expect(mocks.hostSave).toHaveBeenCalledWith('host-a', { terminalEnabled: true, filesEnabled: false, expectedResourceVersion: 'v1' })
    expect(mocks.hostSave).toHaveBeenCalledTimes(1)
    expect(mocks.save).not.toHaveBeenCalled()
    expect((input(wrapper, 'host-b · 允许远程文件管理').element as HTMLInputElement).checked).toBe(true)
  })
  it('defaults on, saves the server version, and refreshes host capabilities', async () => {
    const changed = vi.fn(); window.addEventListener(clusterRemoteAccessChanged, changed)
    const wrapper = open(); await flushPromises()
    expect(wrapper.findAll('input[role="switch"]').map(input => (input.element as HTMLInputElement).checked)).toEqual([true, true])
    await input(wrapper, '允许远程终端').setValue(false)
    await submit(wrapper)
    expect(mocks.save).toHaveBeenCalledWith({ terminalEnabled: false, filesEnabled: true, expectedResourceVersion: 'v1' })
    expect(changed).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('远程操作设置已保存')
    window.removeEventListener(clusterRemoteAccessChanged, changed)
  })
  it('tightens globals first, saves hosts with successive CAS versions, then enables globals', async () => {
    mocks.read.mockResolvedValue({ ...snapshot, filesEnabled: false })
    const calls: string[] = []
    mocks.save.mockImplementation(async body => { calls.push('global:' + body.terminalEnabled + '/' + body.filesEnabled + ':' + body.expectedResourceVersion); return { ...snapshot, ...body, resourceVersion: 'v' + (calls.length + 1) } })
    mocks.hostSave.mockImplementation(async (id, body) => { calls.push(id + ':' + body.expectedResourceVersion); return { ...snapshot, ...body, hostId: id, resourceVersion: 'v' + (calls.length + 1) } })
    const wrapper = open([host('a'), host('b')]); await flushPromises()
    await input(wrapper, '允许远程终端').setValue(false)
    await input(wrapper, '允许远程文件管理').setValue(true)
    await input(wrapper, 'a · 允许远程终端').setValue(false)
    await input(wrapper, 'b · 允许远程文件管理').setValue(false)
    await submit(wrapper)
    expect(calls).toEqual(['global:false/false:v1', 'a:v2', 'b:v3', 'global:false/true:v4'])
    expect(wrapper.emitted('ready')?.at(-1)).toEqual([false])
  })
  it('retains only unsaved drafts after a partial failure and does not enable a global early', async () => {
    mocks.read.mockResolvedValue({ ...snapshot, terminalEnabled: false })
    mocks.hostSave.mockImplementation(async (id, body) => {
      if (id === 'b') throw new ApiError('unavailable', 503)
      return { ...snapshot, ...body, hostId: id, resourceVersion: 'v2' }
    })
    const wrapper = open([host('a'), host('b')]); await flushPromises()
    await input(wrapper, '允许远程终端').setValue(true)
    await input(wrapper, 'a · 允许远程文件管理').setValue(false)
    await input(wrapper, 'b · 允许远程文件管理').setValue(false)
    await submit(wrapper)
    expect(mocks.save).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('部分设置已保存')
    expect((input(wrapper, 'b · 允许远程文件管理').element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.emitted('saved')).toHaveLength(1)
    mocks.hostSave.mockResolvedValue({ ...snapshot, filesEnabled: false, resourceVersion: 'v3' })
    await submit(wrapper)
    expect(mocks.hostSave.mock.calls.map(call => call[0])).toEqual(['a', 'b', 'b'])
    expect(mocks.hostSave.mock.calls.at(-1)?.[1].expectedResourceVersion).toBe('v2')
    expect(mocks.save).toHaveBeenCalledWith({ terminalEnabled: true, filesEnabled: true, expectedResourceVersion: 'v3' })
  })
  it('supports MCP-style selection, inversion and batch editing without saving unselected hosts', async () => {
    const wrapper = open([host('a'), host('b'), host('c')]); await flushPromises()
    const button = (text: string) => wrapper.findAll('button').find(button => button.text() === text)!
    await button('全选').trigger('click')
    await input(wrapper, '选择主机 · c').setValue(false)
    await wrapper.find('select[aria-label="批量设置终端"]').setValue('deny')
    await button('应用到所选主机').trigger('click')
    expect((input(wrapper, 'a · 允许远程终端').element as HTMLInputElement).checked).toBe(false)
    expect((input(wrapper, 'b · 允许远程终端').element as HTMLInputElement).checked).toBe(false)
    expect((input(wrapper, 'c · 允许远程终端').element as HTMLInputElement).checked).toBe(true)
    expect(mocks.hostSave).not.toHaveBeenCalled()
    await button('反选').trigger('click')
    expect((input(wrapper, '选择主机 · c').element as HTMLInputElement).checked).toBe(true)
    expect(wrapper.text()).toContain('已选 1 台')
    await submit(wrapper)
    expect(mocks.hostSave.mock.calls.map(call => call[0])).toEqual(['a', 'b'])
  })
  it('loads at most four policies concurrently, supports cross-page selection and preserves hidden drafts', async () => {
    const resolvers: Array<() => void> = []
    let active = 0; let maxActive = 0
    mocks.hostRead.mockImplementation(id => new Promise(resolve => {
      active++; maxActive = Math.max(maxActive, active)
      resolvers.push(() => { active--; resolve({ ...snapshot, hostId: id }) })
    }))
    const wrapper = open(Array.from({ length: 12 }, (_, i) => host('node-' + i))); await flushPromises()
    expect(mocks.hostRead).toHaveBeenCalledTimes(4)
    await wrapper.findAll('button').find(button => button.text() === '全选')!.trigger('click')
    while (resolvers.length) { resolvers.shift()!(); await flushPromises() }
    expect(maxActive).toBe(4)
    expect(mocks.hostRead).toHaveBeenCalledTimes(12)
    await input(wrapper, 'node-0 · 允许远程文件管理').setValue(false)
    await wrapper.find('button[aria-label="下一页"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('已选 12 台')
    await input(wrapper, '搜索主机').setValue('node-11'); await flushPromises()
    expect(wrapper.findAll('[data-host-id]').map(row => row.attributes('data-host-id'))).toEqual(['node-11'])
    await submit(wrapper)
    expect(mocks.hostSave).toHaveBeenCalledWith('node-0', { terminalEnabled: true, filesEnabled: false, expectedResourceVersion: 'v1' })
  })
  it('does not pretend failed host reads are enabled, and retries without losing other drafts', async () => {
    mocks.hostRead.mockImplementation(async id => { if (id === 'a') throw new Error('offline'); return { ...snapshot, hostId: id } })
    const wrapper = open([host('a'), host('b')]); await flushPromises()
    expect(wrapper.find('[data-host-id="a"] [role="switch"]').exists()).toBe(false)
    expect(wrapper.find('[data-host-id="a"]').text()).toContain('读取失败')
    await input(wrapper, 'b · 允许远程终端').setValue(false)
    mocks.hostRead.mockResolvedValue({ ...snapshot, filesEnabled: false })
    await wrapper.find('[data-host-id="a"] button').trigger('click'); await flushPromises()
    expect((input(wrapper, 'a · 允许远程文件管理').element as HTMLInputElement).checked).toBe(false)
    expect((input(wrapper, 'b · 允许远程终端').element as HTMLInputElement).checked).toBe(false)
  })
  it('keeps drafts on tab switches and reloads on the next opening', async () => {
    const wrapper = open([host('a')]); await flushPromises()
    await input(wrapper, 'a · 允许远程文件管理').setValue(false)
    await wrapper.setProps({ active: false }); await wrapper.setProps({ active: true }); await flushPromises()
    expect(mocks.read).toHaveBeenCalledTimes(1)
    expect((input(wrapper, 'a · 允许远程文件管理').element as HTMLInputElement).checked).toBe(false)
    await wrapper.setProps({ open: false }); await wrapper.setProps({ open: true }); await flushPromises()
    expect(mocks.read).toHaveBeenCalledTimes(2)
    expect((input(wrapper, 'a · 允许远程文件管理').element as HTMLInputElement).checked).toBe(true)
  })
  it('retains selections on conflict and reloads on explicit request', async () => {
    mocks.hostSave.mockRejectedValue(new ApiError('changed', 409))
    const wrapper = open([host('a')]); await flushPromises()
    await input(wrapper, 'a · 允许远程终端').setValue(false); await submit(wrapper)
    expect(wrapper.find('[role="alert"]').text()).toContain('其他页面更新')
    expect((input(wrapper, 'a · 允许远程终端').element as HTMLInputElement).checked).toBe(false)
    await wrapper.findAll('button').find(button => button.text().includes('重新读取设置'))!.trigger('click'); await flushPromises()
    expect((input(wrapper, 'a · 允许远程终端').element as HTMLInputElement).checked).toBe(true)
  })
  it('allows unchanged policies to retry pending cleanup and blocks unreadable global config', async () => {
    mocks.read.mockResolvedValue({ ...snapshot, terminalEnabled: false, pendingTerminalCleanup: 1 })
    const wrapper = open(); await flushPromises(); await submit(wrapper)
    expect(mocks.save).toHaveBeenCalled()
    await wrapper.setProps({ open: false })
    mocks.read.mockResolvedValue({ terminalEnabled: false, filesEnabled: false, resourceVersion: '', available: false })
    await wrapper.setProps({ open: true }); await flushPromises()
    expect(wrapper.find('fieldset').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[role="alert"]').text()).toContain('远程操作已停用')
    expect(wrapper.emitted('ready')?.at(-1)).toEqual([false])
  })
})
