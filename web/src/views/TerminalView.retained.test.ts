// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TerminalView from './TerminalView.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), open: vi.fn(), hosts: vi.fn() }))
vi.mock('vue-router', async (original) => ({ ...await original<typeof import('vue-router')>(), useRoute: () => ({ path: '/terminal', query: {} }) }))
vi.mock('@/lib/api', () => ({ api: { cluster: { hosts: mocks.hosts }, terminals: { list: mocks.list, open: mocks.open } }, ApiError: class extends Error {} }))
vi.mock('@/components/terminal/HostTerminal.vue', () => ({ default: defineComponent({
  name: 'HostTerminalStub', props: ['sessionId', 'retained', 'initialOffset'],
  setup(_, { expose }) { expose({ closeSession: async () => {}, focusTerminal() {}, executeCommand() { return true }, scheduleResize() {} }); return () => null },
}) }))

beforeEach(() => {
  vi.clearAllMocks()
  mocks.hosts.mockResolvedValue({ items: [{ id: 'local', name: 'Local', isLocal: true, terminalAvailable: true }] })
  mocks.open.mockResolvedValue({ sessionId: 'new', hostId: 'local', retained: true, offset: 0 })
})

describe('retained terminal restoration', () => {
  it('restores the same local and remote shortcut sessions without replay or new opens', async () => {
    mocks.list.mockResolvedValue({ items: [
      { sessionId: 'original', hostId: 'local', offset: 123, retained: true },
      { sessionId: 'remote', hostId: 'offline-node', offset: 456, retained: true, title: 'Saved command', shortcutId: 'a'.repeat(32), closeRequested: true },
    ] })
    const wrapper = mount(TerminalView)
    await flushPromises()
    expect(mocks.open).not.toHaveBeenCalled()
    expect(wrapper.findAll('.terminal-tab')).toHaveLength(2)
    expect(wrapper.text()).toContain('Saved command')
    expect(wrapper.findComponent({ name: 'HostTerminalStub' }).props()).toMatchObject({ sessionId: 'original', retained: true, initialOffset: 0 })
    expect(wrapper.text()).toContain('会话已保留')
    wrapper.unmount()
  })
  it('blocks automatic and manual opens after lookup failure until restoration succeeds', async () => {
    mocks.list.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount(TerminalView)
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    await wrapper.get('.terminal-host').trigger('click')
    expect(mocks.open).not.toHaveBeenCalled()
    mocks.list.mockResolvedValue({ items: [{ sessionId: 'original', hostId: 'local', offset: 0, retained: true }] })
    await wrapper.get('[aria-label="刷新连接"]').trigger('click')
    await flushPromises()
    expect(mocks.open).not.toHaveBeenCalled()
    expect(wrapper.findAll('.terminal-tab')).toHaveLength(1)
    wrapper.unmount()
  })
  it('does not open after unmount while the session lookup is pending', async () => {
    let resolve!: (value: unknown) => void
    mocks.list.mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = mount(TerminalView)
    await flushPromises()
    wrapper.unmount()
    resolve({ items: [] })
    await flushPromises()
    expect(mocks.open).not.toHaveBeenCalled()
  })
})
