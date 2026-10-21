// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GalleryImage from './GalleryImage.vue'
import { galleryMediaPausedKey } from '@/lib/galleryMedia'
import { createGalleryRenderPolicy, galleryRenderPolicyKey } from '@/lib/galleryRenderPolicy'

const NativeURL = URL
beforeEach(() => {
  vi.stubGlobal('IntersectionObserver', undefined)
  vi.stubGlobal('URL', class extends NativeURL {
    static createObjectURL = vi.fn(() => 'blob:recovered')
    static revokeObjectURL = vi.fn()
  })
})
afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers() })

describe('native gallery image loading', () => {
  it('keeps an in-flight native URL during a fling and defers completion without restarting the request', async () => {
    vi.useFakeTimers()
    const policy = createGalleryRenderPolicy()
    policy.reset(0, 0)
    const view = mount(GalleryImage, { props: { src: '/thumbnail', eager: true }, global: { provide: { [galleryRenderPolicyKey as symbol]: policy } } })
    await vi.advanceTimersByTimeAsync(32)
    expect(view.attributes('src')).toBe('/thumbnail')
    policy.interact(100, 40)
    await view.trigger('load')
    await vi.advanceTimersByTimeAsync(40)
    expect(view.attributes('src')).toBe('/thumbnail')
    expect(view.emitted('load')).toBeUndefined()
    policy.interact(102, 100)
    await vi.advanceTimersByTimeAsync(32)
    expect(view.attributes('src')).toBe('/thumbnail')
    expect(view.emitted('load')).toHaveLength(1)
    view.unmount(); policy.dispose()
  })

  it('does not start a queued stale image after its version changes or it unmounts', async () => {
    vi.useFakeTimers()
    const policy = createGalleryRenderPolicy()
    policy.reset(0, 0); policy.interact(100, 20)
    const options = { props: { src: '/thumbnail?v=1', version: '1' }, global: { provide: { [galleryRenderPolicyKey as symbol]: policy } } }
    const stale = mount(GalleryImage, options)
    const current = mount(GalleryImage, options)
    await current.setProps({ src: '/thumbnail?v=2', version: '2' })
    stale.unmount()
    policy.reset()
    await vi.advanceTimersByTimeAsync(32)
    expect(current.attributes('src')).toBe('/thumbnail?v=2')
    expect(stale.attributes('src')).toBeUndefined()
    current.unmount(); policy.dispose()
  })
  it('lets the browser load and reuse versioned URLs without a JS body reader or Blob cache', async () => {
    const fetch = vi.fn(); vi.stubGlobal('fetch', fetch)
    const options = { props: { src: '/thumbnail?version=v1', version: 'v1' } }
    const first = mount(GalleryImage, options)
    await flushPromises()
    expect(first.attributes('src')).toBe('/thumbnail?version=v1')
    first.unmount()
    const second = mount(GalleryImage, options)
    await second.setProps({ src: '/thumbnail?version=v2', version: 'v2' })
    expect(second.attributes('src')).toBe('/thumbnail?version=v2')
    expect(fetch).not.toHaveBeenCalled()
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    second.unmount()
  })
  it('defers pending loads while scrolling or inactive but preserves already displayed photos', async () => {
    const paused = ref(true)
    const view = mount(GalleryImage, { props: { src: '/thumbnail', eager: true }, global: { provide: { [galleryMediaPausedKey as symbol]: paused } } })
    expect(view.attributes('src')).toBeUndefined()
    paused.value = false; await flushPromises()
    expect(view.attributes('src')).toBe('/thumbnail')
    paused.value = true; await flushPromises()
    expect(view.attributes('src')).toBeUndefined()
    paused.value = false; await flushPromises()
    await view.trigger('load')
    paused.value = true; await flushPromises()
    expect(view.attributes('src')).toBe('/thumbnail')
    expect(view.emitted('load')).toHaveLength(1)
    view.unmount()
  })
  it('only falls back on explicitly unsupported thumbnails, not authorization or network errors', async () => {
    for (const status of [401, 403, 413, 422]) {
      const fetch = vi.fn().mockResolvedValue(new Response('', { status })); vi.stubGlobal('fetch', fetch)
      const view = mount(GalleryImage, { props: { src: '/thumbnail', fallback: '/original' } })
      await view.trigger('error'); await flushPromises()
      expect(fetch.mock.calls.map(call => call[0])).toEqual(['/thumbnail'])
      expect(view.attributes('src')).toBe(status === 413 || status === 422 ? '/original' : '/thumbnail')
      expect(Boolean(view.emitted('error'))).toBe(status === 401 || status === 403)
      view.unmount()
    }
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    const view = mount(GalleryImage, { props: { src: '/thumbnail', fallback: '/original' } })
    await view.trigger('error'); await flushPromises()
    expect(view.attributes('src')).toBe('/thumbnail')
    expect(view.emitted('error')).toHaveLength(1)
    view.unmount()
  })
  it('uses bounded error recovery and releases its temporary URL after decode failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('jpeg')))
    const view = mount(GalleryImage, { props: { src: '/thumbnail', fallback: '/original' } })
    await view.trigger('error'); await flushPromises()
    expect(view.attributes('src')).toBe('blob:recovered')
    await view.trigger('error')
    expect(view.attributes('src')).toBe('/original')
    expect(URL.revokeObjectURL).toHaveBeenCalledExactlyOnceWith('blob:recovered')
    await view.trigger('error')
    expect(view.emitted('error')).toHaveLength(1)
    view.unmount()
  })
  it('cancels error recovery on pause and ignores its late result', async () => {
    const paused = ref(false)
    let complete!: (response: Response) => void
    let signal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn((_url, options) => new Promise<Response>(resolve => { signal = options.signal; complete = resolve })))
    const view = mount(GalleryImage, { props: { src: '/thumbnail' }, global: { provide: { [galleryMediaPausedKey as symbol]: computed(() => paused.value) } } })
    await view.trigger('error'); await flushPromises()
    paused.value = true; await flushPromises()
    expect(signal?.aborted).toBe(true)
    complete(new Response('late')); await flushPromises()
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    expect(view.emitted('error')).toBeUndefined()
    paused.value = false; await flushPromises()
    expect(view.attributes('src')).toBe('/thumbnail')
    view.unmount()
  })
})
