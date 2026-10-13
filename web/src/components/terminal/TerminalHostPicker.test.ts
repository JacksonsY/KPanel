// @vitest-environment jsdom
import { DOMWrapper, mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import TerminalHostPicker from './TerminalHostPicker.vue'

const mocks = vi.hoisted(() => ({ hosts: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: { cluster: { hosts: mocks.hosts } } }))
const remote = 'b'.repeat(32)
const inventory = { items: [
  { id: 'local', name: 'My Debian', isLocal: true, terminalAvailable: true },
  { id: remote, name: 'Remote Ubuntu', isLocal: false, terminalAvailable: true },
  { id: 'c'.repeat(32), name: 'Monitoring node', isLocal: false, terminalAvailable: false },
] }
const wrappers: ReturnType<typeof mount>[] = []
function create(modelValue = 'local') {
  const w = mount(TerminalHostPicker, { attachTo: document.body, props: { active: true, modelValue } })
  wrappers.push(w); return w
}
beforeEach(() => { vi.clearAllMocks(); mocks.hosts.mockResolvedValue(inventory) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = '' })
const body = () => new DOMWrapper(document.body)
describe('terminal shortcut host picker', () => {
  it('defaults to local and shows names and system icons from the cluster', async () => {
    const w = create(); await flushPromises()
    expect(w.get('.host-switcher__trigger').text()).toContain('本机')
    expect(w.emitted('validity')?.at(-1)).toEqual([true])
    await w.get('.host-switcher__trigger').trigger('click')
    expect(body().get('.host-switcher__menu').text()).toContain('My Debian')
    expect(body().get('.host-switcher__menu').text()).toContain('Remote Ubuntu')
    expect(body().findAll('.os-identity__mark')).toHaveLength(3)
    expect(w.find('.host-switcher__menu').exists()).toBe(false)
    expect(body().get('[data-host-id="' + 'c'.repeat(32) + '"]').attributes('disabled')).toBeDefined()
    await body().get('[data-host-id="' + remote + '"]').trigger('click')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([remote])
  })
  it('keeps local usable when loading cluster hosts fails', async () => {
    mocks.hosts.mockRejectedValueOnce(new Error('offline'))
    const w = create(); await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('无法读取')
    expect(w.emitted('validity')?.at(-1)).toEqual([true])
    expect(w.emitted('update:modelValue')).toBeUndefined()
  })
  it('keeps a missing remote selection invalid rather than switching to local', async () => {
    const w = create('d'.repeat(32)); await flushPromises()
    expect(w.emitted('validity')?.at(-1)).toEqual([false])
    expect(w.emitted('update:modelValue')).toBeUndefined()
    expect(w.get('[role="alert"]').text()).toContain('目标主机')
  })
  it('can explicitly change an unavailable remote selection back to local', async () => {
    const w = create('d'.repeat(32)); await flushPromises()
    await w.get('.host-switcher__trigger').trigger('click')
    await body().get('[data-host-id="local"]').trigger('click')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['local'])
  })
  it('ignores a late inventory after deactivation', async () => {
    let resolve!: (value: unknown) => void
    mocks.hosts.mockReturnValue(new Promise(done => { resolve = done }))
    const w = create(remote)
    await w.setProps({ active: false })
    resolve(inventory); await flushPromises()
    expect(w.text()).not.toContain('Remote Ubuntu')
    expect(w.emitted('validity')?.at(-1)).toEqual([false])
  })
})
