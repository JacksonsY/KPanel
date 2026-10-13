// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
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
  const w = mount(TerminalHostPicker, { props: { active: true, modelValue } })
  wrappers.push(w); return w
}
beforeEach(() => { vi.clearAllMocks(); mocks.hosts.mockResolvedValue(inventory) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()) })
describe('terminal shortcut host picker', () => {
  it('defaults to local and shows names and system icons from the cluster', async () => {
    const w = create(); await flushPromises()
    expect(w.get('.shortcut-host-picker__selected').text()).toContain('My Debian')
    expect(w.find('.os-identity__mark').exists()).toBe(true)
    expect(w.emitted('validity')?.at(-1)).toEqual([true])
    await w.get('.shortcut-host-picker__selected').trigger('click')
    expect(w.text()).toContain('Remote Ubuntu')
    expect(w.get('input[value="' + 'c'.repeat(32) + '"]').attributes('disabled')).toBeDefined()
    await w.get('input[value="' + remote + '"]').setValue()
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
    await w.get('.shortcut-host-picker__selected').trigger('click')
    await w.get('input[value="local"]').setValue()
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
