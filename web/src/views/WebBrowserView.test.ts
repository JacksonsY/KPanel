// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import WebBrowserView from './WebBrowserView.vue'
import { api } from '@/lib/api'
import { browserLinksKey } from '@/lib/webBrowser'
import { desktopWindowVisibleKey } from '@/lib/desktopRouteKeys'

vi.mock('@/lib/api', () => ({ api: { browser: { status: vi.fn(), csrf: () => 'test-csrf' } } }))
class Socket {
  static OPEN = 1
  static instances: Socket[] = []
  readyState = 1
  bufferedAmount = 0
  sent: Record<string, unknown>[] = []
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null
  constructor(public url: string) { Socket.instances.push(this) }
  send(data: string) { this.sent.push(JSON.parse(data)) }
  close() { this.readyState = 3 }
  state(tabs: unknown[] = [], active = '') { this.onmessage?.({ data: JSON.stringify({ type: 'state', tabs, active }) }) }
}
describe('desktop remote browser', () => {
  beforeEach(() => {
    Socket.instances = []
    vi.stubGlobal('WebSocket', Socket)
    vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} })
    vi.mocked(api.browser.status).mockResolvedValue({ ready: true, maxTabs: 4 })
  })
  afterEach(() => { vi.unstubAllGlobals() })
  async function render(visible = ref(true)) {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/browser', component: { template: '<div />' } }] })
    await router.push('/browser')
    const wrapper = mount(WebBrowserView, { global: { plugins: [router], provide: { [desktopWindowVisibleKey as symbol]: visible, [browserLinksKey as symbol]: ref([{ id: 'app', name: 'App', url: 'https://example.com' }]) } } })
    await flushPromises()
    return wrapper
  }
  it('explains a missing optional component and keeps external opening available', async () => {
    vi.mocked(api.browser.status).mockResolvedValue({ ready: false, reason: 'browser_unconfigured', maxTabs: 4 })
    const wrapper = await render()
    expect(wrapper.text()).toContain('浏览器组件尚未启用')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(Socket.instances).toHaveLength(0)
    wrapper.unmount()
  })
  it('authenticates the socket and opens addresses through bounded commands', async () => {
    const wrapper = await render(), socket = Socket.instances[0]!
    socket.onopen?.()
    expect(socket.sent[0]).toEqual({ csrf: 'test-csrf' })
    socket.state(); await nextTick()
    await wrapper.get('.web-browser__address input').setValue('example.com')
    await wrapper.get('.web-browser__toolbar').trigger('submit')
    expect(socket.sent).toContainEqual({ type: 'new', url: 'https://example.com/' })
    expect(wrapper.text()).toContain('通过服务器访问')
    wrapper.unmount()
    expect(socket.sent).toContainEqual({ type: 'end' })
  })
  it('keeps page titles as text and pauses transmission when the desktop window is hidden', async () => {
    const visible = ref(true), wrapper = await render(visible), socket = Socket.instances[0]!
    socket.state([{ id: 'tab-1', url: 'https://example.com', title: '<img src=x>', loading: false, back: true, forward: false }], 'tab-1')
    await nextTick()
    await wrapper.get('[role="tab"]').trigger('click')
    expect(wrapper.find('[role="tab"] img').exists()).toBe(false)
    expect(wrapper.get('[role="tab"]').text()).toContain('<img src=x>')
    visible.value = false; await nextTick()
    expect(socket.sent).toContainEqual({ type: 'visible', visible: false })
    expect(socket.readyState).toBe(1)
    wrapper.unmount()
  })
})
