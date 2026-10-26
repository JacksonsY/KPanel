// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { desktopApps } from './desktopApps'
import {
  createWindowRouter,
  resolveWindowComponent,
  synchronizeWindowRoute,
  windowRouteRecords,
  windowRouterCanGoBack,
} from './desktopWindowRoute'

vi.mock('@/components/terminal/ShortcutCommandWorkspace.vue', () => ({ default: { name: 'ShortcutCommandWorkspace' } }))
vi.mock('@/views/AppScriptView.vue', () => ({
  default: { name: 'AppScriptView' },
}))

describe('desktop window route', () => {
  it.each(desktopApps)('opens $path without falling back to another app', async ({ path }) => {
    const router = createWindowRouter(path)
    await router.isReady()
    expect(router.currentRoute.value.path).toBe(path)
    expect(router.currentRoute.value.redirectedFrom).toBeUndefined()
  })

  it('restores the virtual machine route with its query and canonical name', async () => {
    const router = createWindowRouter('/virtual-machines?node=local')
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('virtual-machines')
    expect(router.currentRoute.value.fullPath).toBe('/virtual-machines?node=local')
  })

  it('keeps site monitoring navigation and its filters in the Sites window', async () => {
    const handoff = vi.fn(() => true)
    const router = createWindowRouter('/sites', vi.fn(), handoff)
    await router.isReady()
    await router.push('/sites/monitor?site=example.com')
    expect(router.currentRoute.value.path).toBe('/sites/monitor')
    expect(router.currentRoute.value.name).toBe('sites-monitor')
    expect(router.currentRoute.value.query.site).toBe('example.com')
    expect(handoff).not.toHaveBeenCalled()
    await synchronizeWindowRoute(router, '/sites/monitor?site=other.example')
    expect(router.currentRoute.value.fullPath).toBe('/sites/monitor?site=other.example')
    await router.push('/sites/environment')
    expect(router.currentRoute.value.path).toBe('/sites/environment')
    expect(handoff).not.toHaveBeenCalled()
  })

  beforeEach(() => {
    document.head.innerHTML = ''
    window.scrollTo = vi.fn()
  })

  it('registers every application route with the canonical names', () => {
    const records = windowRouteRecords()
    const paths = records.map((record) => record.path)
    const names = records.map((record) => record.name)

    expect(paths).toEqual(expect.arrayContaining([
      '/overview',
      '/system',
      '/monitoring',
      '/processes',
      '/cluster',
      '/ai',
      '/ai/s/:sessionId',
      '/sites',
      '/sites/environment',
      '/apps',
      '/app-script/:appId',
      '/files',
      '/terminal',
      '/diagnostics',
      '/docker',
      '/activity',
      '/jobs',
      '/audit',
      '/settings',
      '/guides',
      '/settings/guides',
    ]))
    expect(names).toEqual(expect.arrayContaining([
      'overview',
      'system',
      'monitoring',
      'processes',
      'ai',
      'ai-session',
      'sites-environment',
      'files',
      'app-script',
      'activity',
      'settings',
      'guides',
    ]))
  })

  it('resolves dynamic ai session paths to the ai workspace component', async () => {
    expect(await resolveWindowComponent('/ai/s/42')).toBe(await resolveWindowComponent('/ai'))
  })

  it('resolves dynamic app script paths to the dedicated terminal workspace', async () => {
    const component = await resolveWindowComponent('/app-script/openclaw')
    expect((component as { name?: string }).name).toBe('AppScriptView')
  })

  it('uses synchronous route placeholders so router navigation never owns page chunk loading', () => {
    const components = windowRouteRecords()
      .map((record) => record.component)
      .filter(Boolean)
    expect(components.length).toBeGreaterThan(0)
    expect(components.every((component) => typeof component !== 'function')).toBe(true)
  })

  it('navigates within the window router without touching the app router', async () => {
    const router = createWindowRouter('/ai')
    await router.push('/ai')
    expect(router.currentRoute.value.path).toBe('/ai')

    await router.push('/ai/s/42')
    expect(router.currentRoute.value.path).toBe('/ai/s/42')
    expect(router.currentRoute.value.params.sessionId).toBe('42')

    await router.replace('/ai')
    expect(router.currentRoute.value.path).toBe('/ai')
    expect(router.currentRoute.value.params.sessionId).toBeUndefined()
  })

  it('starts without a synthetic back entry and records only real page navigation', async () => {
    const router = createWindowRouter('/overview')
    await router.isReady()

    expect(windowRouterCanGoBack(router)).toBe(false)
    await router.push('/monitoring')
    expect(windowRouterCanGoBack(router)).toBe(true)

    router.back()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/overview'))
    expect(windowRouterCanGoBack(router)).toBe(false)
  })

  it('returns through file directory query history one level at a time', async () => {
    const router = createWindowRouter('/files?path=/')
    await router.isReady()
    await router.push({ name: 'files', query: { path: '/home' } })
    await router.push({ name: 'files', query: { path: '/home/web' } })

    router.back()
    await vi.waitFor(() => expect(router.currentRoute.value.query.path).toBe('/home'))
    expect(windowRouterCanGoBack(router)).toBe(true)

    router.back()
    await vi.waitFor(() => expect(router.currentRoute.value.query.path).toBe('/'))
    expect(windowRouterCanGoBack(router)).toBe(false)
  })

  it('preserves monitoring zoom state inside window history entries', async () => {
    const router = createWindowRouter('/monitoring?range=6h')
    await router.isReady()
    await router.push({
      path: '/monitoring',
      query: { range: '6h', start: '2026-08-08T00:00:00Z', end: '2026-08-08T01:00:00Z' },
      state: { monitoringZoomDepth: 1 },
    })

    expect(router.options.history.state.monitoringZoomDepth).toBe(1)
    router.back()
    await vi.waitFor(() => expect(router.currentRoute.value.query.start).toBeUndefined())
    expect(router.options.history.state.monitoringZoomDepth).toBeUndefined()
  })

  it('keeps the process manager route inside its desktop window', async () => {
    const router = createWindowRouter('/processes')
    await router.push('/processes')

    expect(router.currentRoute.value.path).toBe('/processes')
  })

  it('keeps independent navigation state across window routers', async () => {
    const first = createWindowRouter('/ai')
    const second = createWindowRouter('/ai')
    await first.push('/ai/s/7')
    await second.push('/ai/s/9')
    expect(first.currentRoute.value.params.sessionId).toBe('7')
    expect(second.currentRoute.value.params.sessionId).toBe('9')
  })

  it('resolves window-relative links against the window router', () => {
    const router = createWindowRouter('/ai')
    const resolved = router.resolve('/ai/s/3')
    expect(resolved.path).toBe('/ai/s/3')
    expect(resolved.params.sessionId).toBe('3')
  })

  it('resolves canonical named routes used by window page components', () => {
    const router = createWindowRouter('/overview')
    const files = router.resolve({ name: 'files', query: { path: '/home' } })
    const environment = router.resolve({ name: 'sites-environment' })
    const guides = router.resolve({ name: 'guides' })

    expect(files.fullPath).toBe('/files?path=/home')
    expect(environment.path).toBe('/sites/environment')
    expect(guides.path).toBe('/guides')
  })

  it('redirects legacy activity paths inside the window', async () => {
    const router = createWindowRouter('/activity')
    await router.push('/jobs')
    expect(router.currentRoute.value.fullPath).toBe('/activity?tab=jobs')

    await router.push('/audit')
    expect(router.currentRoute.value.fullPath).toBe('/activity?tab=audit')
  })

  it('hands authentication routes back to the global application', async () => {
    const globalNavigation = vi.fn()
    const router = createWindowRouter('/settings', globalNavigation)
    await router.push('/settings')

    await router.replace({ name: 'login', query: { redirect: '/settings' } })

    expect(globalNavigation).toHaveBeenCalledWith('/login?redirect=/settings')
    expect(router.currentRoute.value.path).toBe('/settings')
  })

  it('hands cross-app links to the desktop while keeping same-app routes local', async () => {
    const desktopNavigation = vi.fn(() => true)
    const router = createWindowRouter('/sites', vi.fn(), desktopNavigation)
    await router.push('/sites')

    await router.push('/sites/environment')
    expect(router.currentRoute.value.path).toBe('/sites/environment')
    expect(desktopNavigation).not.toHaveBeenCalled()

    await router.push({ name: 'files', query: { path: '/home/web/site' } })
    expect(desktopNavigation).toHaveBeenCalledWith('/files?path=/home/web/site')
    expect(router.currentRoute.value.path).toBe('/sites/environment')
  })

  it('restores a native-history target in the same window without app handoff', async () => {
    const desktopNavigation = vi.fn(() => true)
    const router = createWindowRouter('/processes', vi.fn(), desktopNavigation)
    await router.isReady()

    await synchronizeWindowRoute(router, '/overview', { monitoringZoomDepth: 2 })

    expect(router.currentRoute.value.path).toBe('/overview')
    expect(router.options.history.state.monitoringZoomDepth).toBe(2)
    expect(desktopNavigation).not.toHaveBeenCalled()
  })

  it('delegates view-level back and go actions to native document history', async () => {
    const historyNavigation = vi.fn()
    const router = createWindowRouter('/monitoring', vi.fn(), undefined, historyNavigation)
    await router.isReady()

    router.back()
    router.go(-3)
    router.forward()

    expect(historyNavigation.mock.calls).toEqual([[-1], [-3], [1]])
    expect(router.currentRoute.value.path).toBe('/monitoring')
  })
})

it('routes command shortcut ids to an independent terminal workspace', async () => {
  const path = '/command-shortcut/' + 'a'.repeat(32)
  expect((await resolveWindowComponent(path) as { name: string }).name).toBe('ShortcutCommandWorkspace')
  const router = createWindowRouter(path)
  await router.push(path)
  expect(router.currentRoute.value.params.shortcutId).toBe('a'.repeat(32))
})
