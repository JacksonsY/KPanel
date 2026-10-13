// @vitest-environment jsdom
import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ShortcutCommandTerminal from './ShortcutCommandTerminal.vue'
import type { TerminalSession } from '@/types/api'

const mocks = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn(), closeOnUnload: vi.fn(), execute: vi.fn(), childClose: vi.fn() }))
vi.mock('@/lib/api', () => ({
  ApiError: class extends Error { code = '' },
  api: { terminals: { open: mocks.open, close: mocks.close, closeOnUnload: mocks.closeOnUnload } },
}))
const HostTerminal = defineComponent({
  name: 'HostTerminal', props: ['sessionId', 'hostName', 'initialOffset'], emits: ['stateChange'],
  setup(_, { expose }) { expose({ executeCommand: mocks.execute, closeSession: mocks.childClose }); return () => null },
})
const opened: TerminalSession = { sessionId: 'session-1', hostId: 'remote-1', offset: 0, createdAt: '' }
const wrappers: VueWrapper[] = []
function create() {
  const wrapper = mount(ShortcutCommandTerminal, {
    props: { name: 'Load', command: 'uptime\ndf -h', hostName: 'Local server' },
    global: { stubs: { HostTerminal, ModalDialog: { name: 'ModalDialog', props: ['closeDisabled'], emits: ['close'], template: '<div><slot /></div>' } } },
  })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  mocks.open.mockResolvedValue(opened)
  mocks.close.mockResolvedValue({ closed: true })
  mocks.execute.mockReturnValue(true)
  mocks.childClose.mockResolvedValue(undefined)
})
afterEach(() => { for (const wrapper of wrappers.splice(0)) wrapper.unmount() })

describe('shortcut command terminal lifecycle', () => {
  it('opens the local host, submits once after connection, and never replays on reconnect', async () => {
    const wrapper = create()
    await flushPromises()
    expect(mocks.open).toHaveBeenCalledWith('local', 30, 120)
    expect(mocks.execute).not.toHaveBeenCalled()
    const terminal = wrapper.findComponent(HostTerminal)
    terminal.vm.$emit('stateChange', 'connected')
    terminal.vm.$emit('stateChange', 'connected')
    await flushPromises()
    terminal.vm.$emit('stateChange', 'reconnecting')
    terminal.vm.$emit('stateChange', 'connected')
    await flushPromises()
    expect(mocks.execute).toHaveBeenCalledTimes(1)
    expect(mocks.execute).toHaveBeenCalledWith('uptime\ndf -h')
  })

  it('reclaims a late session after the user closes during connection', async () => {
    let resolveOpen!: (value: TerminalSession) => void
    mocks.open.mockReturnValue(new Promise((resolve) => { resolveOpen = resolve }))
    const wrapper = create()
    await flushPromises()
    wrapper.findComponent({ name: 'ModalDialog' }).vm.$emit('close')
    await flushPromises()
    expect(wrapper.emitted('close')).toHaveLength(1)
    resolveOpen(opened)
    await flushPromises()
    expect(mocks.close).toHaveBeenCalledWith(opened.sessionId)
    expect(mocks.execute).not.toHaveBeenCalled()
  })

  it('reports open failure without submitting a command', async () => {
    mocks.open.mockRejectedValue(new Error('offline'))
    const wrapper = create()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('终端启动失败')
    expect(mocks.execute).not.toHaveBeenCalled()
  })

  it('keeps a failed close visible and allows retry through the existing terminal', async () => {
    const wrapper = create()
    await flushPromises()
    mocks.childClose.mockRejectedValueOnce(new Error('offline'))
    const modal = wrapper.findComponent({ name: 'ModalDialog' })
    modal.vm.$emit('close')
    await flushPromises()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.text()).toContain('终端关闭失败')
    modal.vm.$emit('close')
    await flushPromises()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})

it('reports a rejected submission and does not retry on reconnect', async () => {
  mocks.execute.mockReturnValue(false)
  const wrapper = create()
  await flushPromises()
  wrapper.findComponent(HostTerminal).vm.$emit('stateChange', 'connected')
  await flushPromises()
  expect(wrapper.find('[role="alert"]').text()).toContain('命令未发送')
  wrapper.findComponent(HostTerminal).vm.$emit('stateChange', 'connected')
  await flushPromises()
  expect(mocks.execute).toHaveBeenCalledTimes(1)
})
it('reclaims a late open after unmount and falls back if close fails', async () => {
  let resolveOpen!: (value: TerminalSession) => void
  mocks.open.mockReturnValue(new Promise(resolve => { resolveOpen = resolve }))
  mocks.close.mockRejectedValue(new Error('offline'))
  const wrapper = create()
  wrapper.unmount()
  resolveOpen(opened)
  await flushPromises()
  expect(mocks.closeOnUnload).toHaveBeenCalledWith(opened.sessionId)
  expect(mocks.execute).not.toHaveBeenCalled()
})
