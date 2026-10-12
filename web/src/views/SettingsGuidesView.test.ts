// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import SettingsGuidesView from './SettingsGuidesView.vue'
import { resetLocaleForTest, setLocale } from '@/i18n'

async function mountGuides() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/settings', component: { template: '<div />' } },
      { path: '/guides', component: SettingsGuidesView },
    ],
  })
  await router.push('/guides')
  await router.isReady()
  return mount(SettingsGuidesView, { global: { plugins: [router] } })
}

describe('SettingsGuidesView', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    resetLocaleForTest()
    vi.spyOn(api.tutorials, 'metadata').mockRejectedValue(new Error('metadata unavailable'))
  })

  it('shows the official Chinese video matrix with Bilibili links and lazy covers', async () => {
    const wrapper = await mountGuides()
    const links = wrapper.findAll('[data-testid="tutorial-video-link"]')

    expect(links).toHaveLength(9)
    expect(links.map((link) => link.attributes('href'))).toEqual([
      'https://www.bilibili.com/video/BV13xpb6tEq7/',
      'https://www.bilibili.com/video/BV1J2HX6ME7p/',
      'https://www.bilibili.com/video/BV1jNe46WEjf/',
      'https://www.bilibili.com/video/BV1rvpe6dE9k/',
      'https://www.bilibili.com/video/BV1YKaY6FEzk/',
      'https://www.bilibili.com/video/BV145by6kEgG/',
      'https://www.bilibili.com/video/BV14UH66EEZ9/',
      'https://www.bilibili.com/video/BV1e7ar69EfD/',
      'https://www.bilibili.com/video/BV1c5GG6vExw/',
    ])
    expect(links.every((link) => link.attributes('target') === '_blank')).toBe(true)
    expect(links.every((link) => link.attributes('rel')?.includes('noreferrer'))).toBe(true)
    expect(links.every((link) => link.get('img').attributes('loading') === 'lazy')).toBe(true)
    expect(links.every((link) => link.get('img').attributes('src')?.startsWith('/tutorials/'))).toBe(true)
    expect(links[7]?.get('img').attributes('src')).toBe('/tutorials/kpanel-overview-zh.webp')
    expect(api.tutorials.metadata).toHaveBeenCalledTimes(9)
    expect(api.tutorials.metadata).toHaveBeenCalledWith('https://www.bilibili.com/video/BV13xpb6tEq7/', expect.any(AbortSignal))
    expect(wrapper.get('[data-testid="tutorial-video-grid"]').classes()).toContain('guide-video-grid')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('video').exists()).toBe(false)
  })

  it('uses YouTube videos with English audio for the English matrix', async () => {
    await setLocale('en-US')
    const wrapper = await mountGuides()
    const links = wrapper.findAll('[data-testid="tutorial-video-link"]')

    expect(links).toHaveLength(5)
    expect(links.map((link) => link.attributes('href'))).toEqual([
      'https://www.youtube.com/watch?v=3HwmSTCtozg&hl=en',
      'https://www.youtube.com/watch?v=kNBMTGF4G1k&hl=en',
      'https://www.youtube.com/watch?v=6OFR8TxFguo&hl=en',
      'https://www.youtube.com/watch?v=vl7ulAKvah4&hl=en',
      'https://www.youtube.com/watch?v=HLTtsALHlpw&hl=en',
    ])
    expect(links[0]?.get('img').attributes('src')).toBe('/tutorials/kpanel-release-125-en.webp')
    expect(links[4]?.get('img').attributes('src')).toBe('/tutorials/kpanel-overview-en.webp')
    expect(links.every((link) => link.attributes('target') === '_blank')).toBe(true)
    expect(links.every((link) => link.get('img').attributes('loading') === 'lazy')).toBe(true)
    expect(links.every((link) => link.get('img').attributes('src')?.startsWith('/tutorials/'))).toBe(true)
    expect(wrapper.findAll('.guide-video__audio')).toHaveLength(5)
    expect(wrapper.text()).toContain('Audio track')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('video').exists()).toBe(false)
  })

  it('uses the title and cover resolved from each video link', async () => {
    vi.mocked(api.tutorials.metadata).mockResolvedValue({
      title: 'KPanel video title from YouTube',
      thumbnailUrl: '/api/v1/tutorials/thumbnail?url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3D3HwmSTCtozg%26hl%3Den',
    })
    await setLocale('en-US')
    const wrapper = await mountGuides()
    await flushPromises()

    expect(wrapper.find('strong').text()).toBe('KPanel video title from YouTube')
    expect(wrapper.find('img').attributes('src')).toBe('/api/v1/tutorials/thumbnail?url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3D3HwmSTCtozg%26hl%3Den')
    expect(wrapper.find('img').attributes('referrerpolicy')).toBe('no-referrer')
  })
})
