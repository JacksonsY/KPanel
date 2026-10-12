// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'
import SettingsGuidesView from './SettingsGuidesView.vue'
import { resetLocaleForTest, setLocale } from '@/i18n'

async function mountGuides() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/settings', component: { template: '<div />' } },
      { path: '/settings/guides', component: SettingsGuidesView },
    ],
  })
  await router.push('/settings/guides')
  await router.isReady()
  return mount(SettingsGuidesView, { global: { plugins: [router] } })
}

describe('SettingsGuidesView', () => {
  beforeEach(() => resetLocaleForTest())

  it('uses the Bilibili video and its local cover for Chinese', async () => {
    const wrapper = await mountGuides()
    const link = wrapper.get('[data-testid="tutorial-video-link"]')

    expect(link.attributes('href')).toBe('https://www.bilibili.com/video/BV1e7ar69EfD/')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toContain('noreferrer')
    expect(link.get('img').attributes('src')).toBe('/tutorials/kpanel-overview-zh.webp')
    expect(link.get('img').attributes('loading')).toBe('lazy')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('video').exists()).toBe(false)
  })

  it('uses the English YouTube video, cover, and audio guidance', async () => {
    await setLocale('en-US')
    const wrapper = await mountGuides()
    const link = wrapper.get('[data-testid="tutorial-video-link"]')

    expect(link.attributes('href')).toBe('https://www.youtube.com/watch?v=HLTtsALHlpw&hl=en')
    expect(link.get('img').attributes('src')).toBe('/tutorials/kpanel-overview-en.webp')
    expect(wrapper.text()).toContain('Audio track')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('video').exists()).toBe(false)
  })
})
