// @vitest-environment jsdom
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import SettingsGuidesView from './SettingsGuidesView.vue'
import { resetLocaleForTest, setLocale } from '@/i18n'

enableAutoUnmount(afterEach)

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

function feed(prefix = 'New tutorial', count = 9) {
  return { stale: false, videos: Array.from({ length: count }, (_, i) => ({
    url: 'https://www.bilibili.com/video/BV13xpb6tEq7/?item=' + i,
    title: prefix + ' ' + i,
    thumbnailUrl: '/api/v1/tutorials/thumbnail?url=' + i,
  })) }
}

describe('SettingsGuidesView', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    resetLocaleForTest()
    vi.spyOn(api.tutorials, 'videos').mockResolvedValue(feed())
  })

  it('loads dynamic videos without individual metadata requests or embedded players', async () => {
    const metadata = vi.spyOn(api.tutorials, 'metadata')
    const wrapper = await mountGuides()
    await flushPromises()
    const links = wrapper.findAll('[data-testid="tutorial-video-link"]')
    expect(api.tutorials.videos).toHaveBeenCalledWith('bilibili', expect.any(AbortSignal))
    expect(links).toHaveLength(9)
    expect(links[0]?.get('strong').text()).toBe('New tutorial 0')
    expect(links[0]?.get('img').attributes('src')).toBe('/api/v1/tutorials/thumbnail?url=0')
    expect(links.every(link => link.attributes('target') === '_blank' && link.attributes('rel') === 'noopener noreferrer')).toBe(true)
    expect(links.every(link => link.get('img').attributes('loading') === 'lazy')).toBe(true)
    expect(metadata).not.toHaveBeenCalled()
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('video').exists()).toBe(false)
    expect(wrapper.find('[data-testid="tutorial-feed-status"]').exists()).toBe(false)
  })

  it('switches to the YouTube feed and back without claiming English audio', async () => {
    const wrapper = await mountGuides()
    await flushPromises()
    const youtube = feed('Playlist', 2)
    youtube.videos[0]!.url = 'https://www.youtube.com/watch?v=4bdTRvOmIXc&hl=en'
    vi.mocked(api.tutorials.videos).mockResolvedValue(youtube)
    await setLocale('en-US')
    await flushPromises()
    expect(api.tutorials.videos).toHaveBeenLastCalledWith('youtube', expect.any(AbortSignal))
    expect(wrapper.findAll('[data-testid="tutorial-video-link"]')).toHaveLength(2)
    expect(wrapper.get('[data-testid="tutorial-video-link"]').attributes('href')).toBe(youtube.videos[0]!.url)
    expect(wrapper.find('.guide-video__audio').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="tutorial-more-link"]')[0]?.attributes('href')).toContain('playlist?list=PLH8p8Llp_SZM')
    await setLocale('zh-TW')
    await flushPromises()
    expect(api.tutorials.videos).toHaveBeenLastCalledWith('bilibili', expect.any(AbortSignal))
  })

  it('keeps loading visible and ignores a late response after changing language', async () => {
    let resolveOld!: (value: ReturnType<typeof feed>) => void
    vi.mocked(api.tutorials.videos).mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const wrapper = await mountGuides()
    expect(wrapper.get('[data-testid="tutorial-feed-status"]').text()).toContain('正在更新')
    expect(wrapper.findAll('[data-testid="tutorial-video-link"]')).toHaveLength(9)
    expect(wrapper.get('img').attributes('src')).toBe('/tutorials/kpanel-release-125-zh.webp')
    const oldSignal = vi.mocked(api.tutorials.videos).mock.calls[0]![1]!
    await setLocale('en-US')
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    resolveOld(feed('Outdated'))
    await flushPromises()
    expect(wrapper.get('strong').text()).toBe('New tutorial 0')
    wrapper.unmount()
    expect(vi.mocked(api.tutorials.videos).mock.calls.at(-1)![1]!.aborted).toBe(true)
  })

  it('shows a recoverable failure and retains both platform destinations', async () => {
    vi.mocked(api.tutorials.videos).mockRejectedValueOnce(new Error('offline'))
    const wrapper = await mountGuides()
    await flushPromises()
    expect(wrapper.get('[data-testid="tutorial-feed-status"]').text()).toContain('内置教程')
    expect(wrapper.findAll('[data-testid="tutorial-video-link"]')).toHaveLength(9)
    expect(wrapper.get('[data-testid="tutorial-video-link"]').attributes('href')).toBe('https://www.bilibili.com/video/BV13xpb6tEq7/')
    expect(wrapper.findAll('[data-testid="tutorial-more-link"]')).toHaveLength(2)
    await wrapper.get('[data-testid="tutorial-feed-status"] button').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="tutorial-video-link"]')).toHaveLength(9)
  })

  it('distinguishes an empty source from a stale source and limits displayed cards', async () => {
    vi.mocked(api.tutorials.videos).mockResolvedValueOnce(feed('Empty', 0))
    const wrapper = await mountGuides()
    await flushPromises()
    expect(wrapper.get('[data-testid="tutorial-feed-status"]').text()).toContain('暂未获取到新视频')
    expect(wrapper.findAll('[data-testid="tutorial-video-link"]')).toHaveLength(9)
    vi.mocked(api.tutorials.videos).mockResolvedValueOnce({ ...feed('Cached', 11), stale: true })
    await wrapper.get('[data-testid="tutorial-feed-status"] button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="tutorial-feed-status"]').text()).toContain('上次成功')
    expect(wrapper.findAll('[data-testid="tutorial-video-link"]')).toHaveLength(9)
    const image = wrapper.get('img')
    await image.trigger('error')
    expect(image.attributes('src')).toBe('/tutorials/kpanel-video-placeholder.svg')
  })

  it('retains a live list through retries, empty replies and platform changes', async () => {
    vi.mocked(api.tutorials.videos).mockResolvedValueOnce({ ...feed('Last good'), stale: true })
    const wrapper = await mountGuides()
    await flushPromises()
    let rejectRefresh!: (reason: Error) => void
    vi.mocked(api.tutorials.videos).mockReturnValueOnce(new Promise((_resolve, reject) => { rejectRefresh = reject }))
    await wrapper.get('[data-testid="tutorial-feed-status"] button').trigger('click')
    expect(wrapper.get('strong').text()).toBe('Last good 0')
    rejectRefresh(new Error('upstream unavailable'))
    await flushPromises()
    expect(wrapper.get('strong').text()).toBe('Last good 0')
    expect(wrapper.get('[data-testid="tutorial-feed-status"]').text()).toContain('上次成功')
    vi.mocked(api.tutorials.videos).mockResolvedValueOnce(feed('Empty', 0))
    await wrapper.get('[data-testid="tutorial-feed-status"] button').trigger('click')
    await flushPromises()
    expect(wrapper.get('strong').text()).toBe('Last good 0')
    vi.mocked(api.tutorials.videos).mockRejectedValue(new Error('offline'))
    await setLocale('en-US')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="tutorial-video-link"]')).toHaveLength(5)
    expect(wrapper.get('[data-testid="tutorial-video-link"]').attributes('href')).toContain('youtube.com/watch?')
    expect(wrapper.get('img').attributes('src')).toBe('/tutorials/kpanel-release-125-en.webp')
    expect(wrapper.get('[data-testid="tutorial-feed-status"]').text()).toContain('built-in tutorials')
    await setLocale('zh-TW')
    await flushPromises()
    expect(wrapper.get('strong').text()).toBe('Last good 0')
  })
})
