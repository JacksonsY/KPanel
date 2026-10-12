// @vitest-environment jsdom
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import HostTerminal from './HostTerminal.vue'
import type { TerminalStreamHandlers } from '@/lib/terminalStream'

const mocks = vi.hoisted(() => ({ input: vi.fn(), subscribe: vi.fn(), writes: [] as Array<string | Uint8Array>, callbacks: [] as Array<() => void>, defer: false }))
vi.mock('@/lib/api', async original => ({ ...await original<typeof import('@/lib/api')>(),
  terminalStream: { subscribe: mocks.subscribe },
  api: { terminals: {
    inputTransport: () => Promise.resolve({ protocol: '' }), input: mocks.input,
    close: () => Promise.resolve({ closed: true }), resize: () => Promise.resolve({ accepted: true }),
  } },
}))
vi.mock('@xterm/xterm', () => ({ Terminal: class {
  options = {}; parser = { registerOscHandler() {}, registerCsiHandler() { return { dispose() {} } } }
  rows = 24; cols = 80; buffer = { active: { viewportY: 0, baseY: 0 } }
  loadAddon() {} attachCustomKeyEventHandler() {} onData() {} open() {} dispose() {} focus() {} scrollToBottom() {}
  write(data: string | Uint8Array, callback?: () => void) {
    mocks.writes.push(data)
    if (mocks.defer && callback) mocks.callbacks.push(callback)
    else callback?.()
  }
} }))
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit() {} } }))
vi.mock('@xterm/addon-web-links', () => ({ WebLinksAddon: class {} }))
let wrapper: VueWrapper
let handlers: TerminalStreamHandlers
let frames: FrameRequestCallback[]
let offset: number
function output(value: string, closed = false) {
  const data = Buffer.from(value)
  handlers.output?.({ data: data.toString('base64'), offset, nextOffset: offset += data.length, truncated: false, closed })
}
function view() { return mocks.writes.map(data => typeof data === 'string' ? data : new TextDecoder().decode(data)).join('') }
type Exposed = { executePresentedCommand(command: string, display: string): Promise<boolean> }
async function paint() {
  for (let i = 0; i < 2; i++) {
    for (const callback of frames.splice(0)) callback(0)
    await flushPromises()
  }
}
beforeEach(async () => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  mocks.writes.length = 0; mocks.callbacks.length = 0; mocks.defer = false
  frames = []; offset = 0
  mocks.input.mockResolvedValue({ accepted: true })
  mocks.subscribe.mockImplementation((_target, value) => { handlers = value; return { close() {} } })
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} })
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { frames.push(callback); return frames.length })
  wrapper = mount(HostTerminal, { props: { sessionId: 'launch', hostName: 'Local', initialOffset: 0 } })
  await flushPromises()
  output('root@host:~# ')
  await flushPromises()
  frames.length = 0
})
afterEach(() => { wrapper.unmount(); vi.useRealTimers(); vi.unstubAllGlobals() })

describe('HostTerminal presented execution', () => {
  it('paints the concise command before sending, hides only bootstrap echo and streams subsequent output', async () => {
    mocks.defer = true
    const result = (wrapper.vm as unknown as Exposed).executePresentedCommand('internal-check && bash ./demo.sh', 'cd -- /root && bash ./demo.sh')
    expect(view()).toContain('\r\x1b[2K$ cd -- /root && bash ./demo.sh\r\n')
    expect(mocks.input).not.toHaveBeenCalled()
    output('late partial prompt:~# ')
    expect(view()).not.toContain('late partial prompt')
    mocks.callbacks.shift()!()
    mocks.defer = false
    await flushPromises()
    expect(mocks.input).not.toHaveBeenCalled()
    await paint()
    expect(await result).toBe(true)
    expect(mocks.input).toHaveBeenCalledTimes(1)
    const sent = Buffer.from(mocks.input.mock.calls[0]![1] as string, 'base64').toString('utf8')
    const token = sent.match(/kpanel-start;([a-zA-Z0-9-]+)/)![1]
    output(`${sent}\r\n`)
    expect(view()).not.toContain('internal-check')
    output(`\x1b]kpanel-start;${token}`)
    output('\x07\x1b[32m中文日志\x1b[0m\r\n')
    expect(view()).toContain('\x1b[32m中文日志\x1b[0m\r\n')
    expect(view()).not.toContain('kpanel-start;')
    output('second log\r\nroot@host:~# ')
    expect(view()).toContain('second log\r\nroot@host:~# ')
    await flushPromises()
    expect(wrapper.find('input').attributes('disabled')).toBeUndefined()
    await wrapper.find('input').setValue('echo next')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(mocks.input).toHaveBeenCalledTimes(2)
  })

  it('does not launch if the window closes before the command is painted', async () => {
    const result = (wrapper.vm as unknown as Exposed).executePresentedCommand('internal', 'bash ./demo.sh')
    await flushPromises()
    wrapper.unmount()
    await paint()
    expect(await result).toBe(false)
    expect(mocks.input).not.toHaveBeenCalled()
  })

  it.each(['timeout', 'closed'] as const)('restores complete diagnostic output on %s without re-executing', async reason => {
    const result = (wrapper.vm as unknown as Exposed).executePresentedCommand('internal', 'bash ./demo.sh')
    await flushPromises(); await paint(); await result
    output('shell startup failed\r\n', reason === 'closed')
    if (reason === 'timeout') await vi.advanceTimersByTimeAsync(15000)
    expect(view()).toContain('shell startup failed\r\n')
    expect(mocks.input).toHaveBeenCalledTimes(1)
  })
})
