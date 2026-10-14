// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import ClusterShareThemes from './ClusterShareThemes.vue'
import { ApiError } from '@/lib/api'

const mocks = vi.hoisted(() => ({ shareThemes: vi.fn(), uploadShareTheme: vi.fn(), installShareTheme: vi.fn(), selectShareTheme: vi.fn(), deleteShareTheme: vi.fn() }))
vi.mock('@/lib/api', async importOriginal => ({ ...await importOriginal<typeof import('@/lib/api')>(), api: { cluster: mocks } }))
const wrappers: ReturnType<typeof mount>[] = []
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.resetAllMocks() })
const pack = { id: 'minimal', name: { 'zh-CN': '<script>主题</script>' }, description: { 'zh-CN': '一句话介绍' }, author: { name: 'KPanel' }, theme: { brand: '#112233', neutral: '#ffffff', signature: '#00aa66' }, sizeBytes: 1024, version: '1.0.0', installed: false, resourceVersion: 'pack-version' }
describe('share theme settings', () => {
  it('uploads then applies explicitly, identifies local content and permits retry', async () => {
    let uploaded = false
    mocks.shareThemes.mockImplementation(async () => ({ resourceVersion: 'list-version', selected: '', packs: uploaded ? [{ ...pack, uploaded: true, installed: true, installedVersion: pack.version }] : [] }))
    mocks.uploadShareTheme.mockRejectedValueOnce(new ApiError('conflict', 409, 'scene_pack_changed')).mockImplementationOnce(async () => { uploaded = true })
    const wrapper = mount(ClusterShareThemes); wrappers.push(wrapper); await flushPromises()
    const file = new File(['zip'], 'my-theme.zip', { type: 'application/zip' })
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('主题 ID 已存在')
    await input.trigger('change'); await flushPromises()
    expect(mocks.uploadShareTheme).toHaveBeenCalledWith(file, 'list-version', expect.any(AbortSignal))
    expect(mocks.selectShareTheme).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('本地上传')
    expect(wrapper.text()).toContain('主题已上传，请点击应用')
    await wrapper.findAll('button').find(button => button.text() === '应用')!.trigger('click'); await flushPromises()
    expect(mocks.selectShareTheme).toHaveBeenCalledWith(pack.id, 'list-version')
  })
  it('rejects oversized packages before sending and disables uploads while in progress', async () => {
    mocks.shareThemes.mockResolvedValue({ resourceVersion: 'v', packs: [] })
    const wrapper = mount(ClusterShareThemes); wrappers.push(wrapper); await flushPromises()
    const input = wrapper.get('input[type="file"]')
    const large = new File(['x'], 'large.zip'); Object.defineProperty(large, 'size', { value: 6 * 1024 * 1024 + 1 })
    Object.defineProperty(input.element, 'files', { value: [large], configurable: true })
    await input.trigger('change'); await flushPromises()
    expect(mocks.uploadShareTheme).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('6 MiB')
    Object.defineProperty(input.element, 'files', { value: [new File(['zip'], 'valid.zip')], configurable: true })
    let finish: () => void = () => {}
    mocks.uploadShareTheme.mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    await input.trigger('change')
    expect(wrapper.findAll('button').find(button => button.text() === '正在上传…')!.attributes('disabled')).toBeDefined()
    finish(); await flushPromises()
  })
  it('downloads without selecting, applies explicitly, and deletes active theme', async () => {
    let installed = false, selected = ''
    mocks.shareThemes.mockImplementation(async () => ({ resourceVersion: 'list-version', selected, packs: [{ ...pack, installed, installedVersion: installed ? '1.0.0' : null }] }))
    mocks.installShareTheme.mockImplementation(async () => { installed = true })
    mocks.selectShareTheme.mockImplementation(async id => { selected = id })
    mocks.deleteShareTheme.mockImplementation(async () => { installed = false; selected = '' })
    const wrapper = mount(ClusterShareThemes); wrappers.push(wrapper); await flushPromises()
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain('一句话介绍'); expect(wrapper.text()).toContain('官方')
    expect(wrapper.get('.theme-card__swatch[style]').attributes('style')).toContain('--swatch-brand: #112233')
    const click = async (text: string) => { await wrapper.findAll('button').find(button => button.text() === text)!.trigger('click'); await flushPromises() }
    await click('下载'); expect(mocks.installShareTheme).toHaveBeenCalledWith('minimal', 'pack-version'); expect(mocks.selectShareTheme).not.toHaveBeenCalled()
    await click('应用'); expect(mocks.selectShareTheme).toHaveBeenCalledWith('minimal', 'list-version')
    await click('删除'); expect(mocks.deleteShareTheme).toHaveBeenCalledWith('minimal', 'pack-version'); expect(wrapper.text()).toMatch(/默认样式\s*内置/); expect(wrapper.text()).toContain('使用中')
  })
  it('keeps a retry after a failed catalog request', async () => {
    mocks.shareThemes.mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ selected: '', resourceVersion: 'v', packs: [] })
    const wrapper = mount(ClusterShareThemes); wrappers.push(wrapper); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('读取失败')
    await wrapper.findAll('button').find(button => button.text() === '重试')!.trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('暂无可下载主题')
  })
})
