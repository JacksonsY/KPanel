// @vitest-environment jsdom
import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { desktopWindowCloseGuardKey } from '@/lib/desktopRouteKeys'
import ShortcutCommandTerminal from './ShortcutCommandTerminal.vue'
import type { TerminalSession } from '@/types/api'
import { ApiError } from '@/lib/api'

const mocks = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn(), closeOnUnload: vi.fn(), execute: vi.fn(), childClose: vi.fn() }))
vi.mock('@/lib/api', () => ({
  ApiError: class extends Error { code = '' },
  api: { terminals: { open: mocks.open, close: mocks.close, closeOnUnload: mocks.closeOnUnload } },
}))
const HostTerminal = defineComponent({
  name: 'HostTerminal', props: ['sessionId', 'hostName', 'initialOffset', 'retained'], emits: ['stateChange'],
  setup(_, { expose }) { expose({ executeCommand: mocks.execute, closeSession: mocks.childClose }); return () => null },
})
const opened: TerminalSession = { sessionId: 'session-1', hostId: 'remote-1', offset: 0, createdAt: '' }
const wrappers: VueWrapper[] = []
let closeGuard: () => boolean | Promise<boolean>
function create(hostId?: string, hostName = 'Local server', retainedProps: { shortcutId?: string; shortcutVersion?: string; existingSession?: TerminalSession } = {}) {
  const wrapper = mount(ShortcutCommandTerminal, {
    props: { name: 'Load', command: 'uptime\ndf -h', hostName, hostId, ...retainedProps },
    global: { provide: { [desktopWindowCloseGuardKey as symbol]: { register: (guard: () => boolean | Promise<boolean>) => { closeGuard = guard; return () => undefined } } }, stubs: { HostTerminal, ModalDialog: { name: 'ModalDialog', props: ['closeDisabled'], emits: ['close'], template: '<div><slot /></div>' } } },
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
  it('uses the saved shortcut on an explicit temporary retry without client-side command submission', async () => {
    mocks.open.mockRejectedValueOnce(Object.assign(new ApiError('old node', 502, 'terminal_open_failed'), { code: 'terminal_open_failed' }))
    const wrapper = create('remote-1', 'Old node', { shortcutId: 'a'.repeat(32), shortcutVersion: 'v1' })
    await flushPromises()
    expect(mocks.open).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('刷新后不保留')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(mocks.open).toHaveBeenLastCalledWith('remote-1', 30, 120, { retained: false, shortcutId: 'a'.repeat(32), shortcutVersion: 'v1' })
    wrapper.findComponent(HostTerminal).vm.$emit('stateChange', 'connected')
    await flushPromises()
    expect(mocks.execute).not.toHaveBeenCalled()
    expect(await closeGuard()).toBe(true)
  })
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
    const closing = closeGuard()
    await flushPromises()
    expect(wrapper.emitted('close')).toBeUndefined()
    resolveOpen(opened)
    await closing
    await flushPromises()
    expect(mocks.childClose).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
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
    expect(await closeGuard()).toBe(false)
    await flushPromises()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.text()).toContain('终端关闭失败')
    expect(await closeGuard()).toBe(true)
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

it('snapshots the selected host and never retargets a pending session when props change', async () => {
  let resolveOpen!: (value: TerminalSession) => void
  mocks.open.mockReturnValue(new Promise(resolve => { resolveOpen = resolve }))
  const remote = 'b'.repeat(32)
  const w = create(remote, 'Remote A')
  await w.setProps({ hostId: 'local', hostName: 'Changed local target' })
  resolveOpen(opened); await flushPromises()
  expect(mocks.open).toHaveBeenCalledOnce()
  expect(mocks.open).toHaveBeenCalledWith(remote, 30, 120)
  expect(w.findComponent({ name: 'HostTerminal' }).props('hostName')).toBe('Remote A')
  expect(w.find('[data-testid="command-target-host"]').exists()).toBe(false)
})

it('starts a saved shortcut once and stops it through the existing window close control', async () => {
  const shortcutId = 'a'.repeat(32)
  mocks.open.mockResolvedValue({ ...opened, retained: true, shortcutId })
  const wrapper = create('local', 'Local', { shortcutId, shortcutVersion: 'v1' })
  await flushPromises()
  expect(mocks.open).toHaveBeenCalledWith('local', 30, 120, { retained: true, shortcutId, shortcutVersion: 'v1' })
  const child = wrapper.findComponent(HostTerminal)
  child.vm.$emit('stateChange', 'connected')
  child.vm.$emit('stateChange', 'connected')
  await flushPromises()
  expect(mocks.execute).not.toHaveBeenCalled()
  expect(wrapper.find('button').exists()).toBe(false)
  expect(await closeGuard()).toBe(true)
  wrapper.unmount()
  expect(mocks.close).not.toHaveBeenCalled()
  expect(mocks.childClose).toHaveBeenCalledOnce()
})
it('reattaches an existing shortcut without sending an open request or command', async () => {
  const shortcutId = 'a'.repeat(32)
  const wrapper = create('local', 'Local', { shortcutId, existingSession: { ...opened, retained: true, offset: 50, launchUncertain: true } })
  await flushPromises()
  expect(mocks.open).not.toHaveBeenCalled()
  expect(wrapper.findComponent(HostTerminal).props('initialOffset')).toBe(0)
  expect(wrapper.text()).toContain('启动结果未确认')
  mocks.childClose.mockRejectedValueOnce(new Error('offline'))
  expect(await closeGuard()).toBe(false)
  await flushPromises()
  expect(wrapper.findComponent(HostTerminal).exists()).toBe(true)
  expect(wrapper.emitted('close')).toBeUndefined()
  expect(await closeGuard()).toBe(true)
  await flushPromises()
  expect(wrapper.emitted('close')).toHaveLength(1)
  expect(mocks.open).not.toHaveBeenCalled()
})

it('detaches a retained shortcut on page reload without closing or replaying it', async () => {
  const shortcutId = 'a'.repeat(32)
  mocks.open.mockResolvedValue({ ...opened, retained: true, shortcutId })
  const wrapper = create('local', 'Local', { shortcutId, shortcutVersion: 'v1' })
  await flushPromises()
  wrapper.unmount()
  expect(mocks.childClose).not.toHaveBeenCalled()
  expect(mocks.close).not.toHaveBeenCalled()
  expect(mocks.closeOnUnload).not.toHaveBeenCalled()
  const restored = create('local', 'Local', { shortcutId, existingSession: { ...opened, retained: true, shortcutId } })
  await flushPromises()
  expect(mocks.open).toHaveBeenCalledOnce()
  expect(restored.findComponent(HostTerminal).props('sessionId')).toBe(opened.sessionId)
  expect(mocks.execute).not.toHaveBeenCalled()
})
