// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import { setLocale } from '@/i18n'
import { registerPhraseCatalog } from '@/i18n/phrase'
import enUS from '@/i18n/pages/shared/en-US'
import zhTW from '@/i18n/pages/shared/zh-TW'

vi.mock('@/lib/clipboard', () => ({ copyText: vi.fn(async () => true) }))
vi.mock('@/lib/api', () => ({ ApiError: Error, api: { apps: {} } }))

const { default: AppOnboardingGuide } = await import('./AppOnboardingGuide.vue')

const wrappers: VueWrapper[] = []

afterEach(async () => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  await nextTick()
  document.body.innerHTML = ''
  await setLocale('zh-CN', false)
})

describe('AppOnboardingGuide', () => {
  it.each([
    ['en-US', enUS, 'Submit an app', 'Add the config', 'Local apps on this server'],
    ['zh-TW', zhTW, '應用入駐', '新增設定', '本機的本地應用'],
  ] as const)('localizes teleported dialog content and attributes in %s', async (locale, catalog, title, step, localLabel) => {
    await setLocale(locale, false)
    const dispose = registerPhraseCatalog(catalog)
    try {
      wrappers.push(mount(AppOnboardingGuide, { attachTo: document.body, props: { open: true, localApps: [] } }))
      // The dialog is teleported outside #app, so its strings must not rely on
      // the App-root DOM observer to receive translations.
      expect(document.querySelector('[role="dialog"] h2')?.textContent).toBe(title)
      expect(document.querySelector('.onboarding-flow strong')?.textContent).toBe(step)
      expect(document.querySelector('.local-apps')?.getAttribute('aria-label')).toBe(localLabel)
    } finally {
      dispose()
    }
  })

  it('opens straight into local app management with the add, verify, submit flow', () => {
    wrappers.push(mount(AppOnboardingGuide, { attachTo: document.body, props: { open: true, localApps: [] } }))
    const text = document.body.textContent || ''
    expect([...document.querySelectorAll('.onboarding-flow strong')].map((item) => item.textContent))
      .toEqual(['添加配置', '安装验证', '提交官方'])
    expect(text).toContain('本机的本地应用')
    expect(text).toContain('会以 root 身份执行')
    expect(document.querySelector('[aria-pressed]')).toBeNull()
    expect(document.querySelector('a[href="https://dev.kejilion.sh/"]')).not.toBeNull()
  })
})
