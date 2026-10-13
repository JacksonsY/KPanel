// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { desktopWindowLaunchKey, desktopWindowCloseGuardKey } from '@/lib/desktopRouteKeys'
import ShortcutCommandWorkspace from './ShortcutCommandWorkspace.vue'

const mocks = vi.hoisted(() => ({ workspace: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: { desktop: { workspace: mocks.workspace } } }))
const id = 'a'.repeat(32)
const shortcut = { id, name: 'Load', targetType: 'command', command: 'uptime' }
let closeGuard: () => boolean | Promise<boolean>
const wrappers: ReturnType<typeof mount>[] = []
async function create(launch = true) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/command-shortcut/:shortcutId', component: {} }] })
  await router.push('/command-shortcut/' + id)
  const wrapper = mount(ShortcutCommandWorkspace, { global: { plugins: [router], provide: {
    [desktopWindowLaunchKey as symbol]: () => launch,
    [desktopWindowCloseGuardKey as symbol]: { register: (guard: () => boolean | Promise<boolean>) => { closeGuard = guard; return () => undefined } },
  }, stubs: { ShortcutCommandTerminal: { props: ['name', 'command', 'hostName'], template: '<div data-testid="terminal">{{ command }}</div>' } } } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}
beforeEach(() => { vi.clearAllMocks(); mocks.workspace.mockResolvedValue({ available: true, shortcuts: [shortcut] }) })
afterEach(() => { for (const w of wrappers.splice(0)) w.unmount() })
describe('command shortcut desktop workspace', () => {
  it('loads the saved command by id and launches only for a new explicit window', async () => {
    const w = await create()
    expect(w.get('[data-testid="terminal"]').text()).toBe('uptime')
    expect(mocks.workspace).toHaveBeenCalledWith(expect.any(AbortSignal))
  })
  it('restores idle and requires an explicit run with freshly read command data', async () => {
    const w = await create(false)
    expect(w.find('[data-testid="terminal"]').exists()).toBe(false)
    expect(w.text()).toContain('不会自动重复执行')
    expect(w.get('button').text()).toBe('执行“Load”')
    mocks.workspace.mockResolvedValue({ available: true, shortcuts: [{ ...shortcut, command: 'df -h' }] })
    await w.get('button').trigger('click'); await flushPromises()
    expect(w.get('[data-testid="terminal"]').text()).toBe('df -h')
  })
  it('rejects deleted or unavailable shortcuts without opening a terminal', async () => {
    mocks.workspace.mockResolvedValue({ available: true, shortcuts: [] })
    const w = await create()
    expect(w.get('[role="alert"]').text()).toContain('已删除')
    expect(w.find('[data-testid="terminal"]').exists()).toBe(false)
  })
  it('recovers a failed workspace load through retry without auto-running', async () => {
    mocks.workspace.mockRejectedValueOnce(new Error('offline'))
    const w = await create()
    expect(w.get('[role="alert"]').text()).toContain('无法读取')
    await w.get('button').trigger('click'); await flushPromises()
    expect(w.find('[role="alert"]').exists()).toBe(false)
    expect(w.find('[data-testid="terminal"]').exists()).toBe(false)
  })
  it('does not launch after close while the workspace lookup is pending', async () => {
    let resolve!: (value: unknown) => void
    mocks.workspace.mockReturnValue(new Promise(done => { resolve = done }))
    const w = await create()
    expect(await closeGuard()).toBe(true)
    resolve({ available: true, shortcuts: [shortcut] }); await flushPromises()
    expect(w.find('[data-testid="terminal"]').exists()).toBe(false)
  })
})
