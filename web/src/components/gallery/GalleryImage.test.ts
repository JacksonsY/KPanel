// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GalleryImage from './GalleryImage.vue'

beforeEach(() => {
  vi.stubGlobal('IntersectionObserver', undefined)
  vi.stubGlobal('URL', { createObjectURL: vi.fn(() => 'blob:test'), revokeObjectURL: vi.fn() })
})
afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers() })

describe('gallery image lifecycle', () => {
  it('loads a blob and revokes it when the tile leaves the page', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('jpeg')))
    const view = mount(GalleryImage, { props: { src: '/thumbnail' } })
    await flushPromises()
    expect(view.attributes('src')).toBe('blob:test')
    view.unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:test')
  })

  it('uses the original only when thumbnail generation is unsupported', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(new Response('', { status: 422 })).mockResolvedValueOnce(new Response('jpeg'))
    vi.stubGlobal('fetch', fetch)
    const view = mount(GalleryImage, { props: { src: '/thumbnail', fallback: '/original' } })
    await flushPromises()
    expect(fetch.mock.calls.map((call) => call[0])).toEqual(['/thumbnail', '/original'])
    view.unmount()
  })

  it('never falls back to the original on authorization or network errors', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response('', { status: 403 }))
    vi.stubGlobal('fetch', fetch)
    const view = mount(GalleryImage, { props: { src: '/thumbnail', fallback: '/original' } })
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(view.emitted('error')).toHaveLength(1)
    view.unmount()
  })

  it('cancels a pending load on unmount without emitting a stale error', async () => {
    let signal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn((_url, options) => new Promise((_resolve, reject) => {
      signal = options.signal
      signal!.addEventListener('abort', () => reject(signal!.reason), { once: true })
    })))
    const view = mount(GalleryImage, { props: { src: '/thumbnail' } })
    await flushPromises()
    view.unmount()
    await flushPromises()
    expect(signal?.aborted).toBe(true)
    expect(view.emitted('error')).toBeUndefined()
  })
})
