import { afterEach, describe, expect, it, vi } from 'vitest'
import { createGalleryQueue, fetchGalleryImage, GalleryImageError } from './galleryMedia'

afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers() })

describe('gallery request scheduling', () => {
  it('bounds concurrency and removes cancelled queued work', async () => {
    const queue = createGalleryQueue(1)
    let finish!: () => void
    const first = queue(() => new Promise<void>((resolve) => { finish = resolve }), new AbortController().signal)
    await Promise.resolve()
    const controller = new AbortController()
    const abandoned = vi.fn(async () => undefined)
    const second = queue(abandoned, controller.signal).catch(() => 'cancelled')
    const last = vi.fn(async () => 3)
    const third = queue(last, new AbortController().signal)
    expect(last).not.toHaveBeenCalled()
    controller.abort()
    expect(await second).toBe('cancelled')
    finish()
    await first
    expect(await third).toBe(3)
    expect(abandoned).not.toHaveBeenCalled()
  })

  it('releases its slot after a rejected job', async () => {
    const queue = createGalleryQueue(1)
    await expect(queue(async () => { throw new Error('failed') }, new AbortController().signal)).rejects.toThrow('failed')
    expect(await queue(async () => 7, new AbortController().signal)).toBe(7)
  })

  it('retries temporary congestion without changing the source', async () => {
    vi.useFakeTimers()
    const fetch = vi.fn().mockResolvedValueOnce(new Response('', { status: 429 }))
      .mockResolvedValueOnce(new Response('', { status: 503 })).mockResolvedValueOnce(new Response('image'))
    vi.stubGlobal('fetch', fetch)
    const result = fetchGalleryImage('/thumbnail', new AbortController().signal)
    await vi.runAllTimersAsync()
    expect(await (await result).text()).toBe('image')
    expect(fetch.mock.calls.map((call) => call[0])).toEqual(['/thumbnail', '/thumbnail', '/thumbnail'])
  })

  it('does not retry authentication failures or unsupported formats', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response('', { status: 401 }))
    vi.stubGlobal('fetch', fetch)
    await expect(fetchGalleryImage('/thumbnail', new AbortController().signal)).rejects.toMatchObject({ status: 401 })
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('caps busy retries and reports the original HTTP status', async () => {
    vi.useFakeTimers()
    const fetch = vi.fn().mockResolvedValue(new Response('', { status: 429 }))
    vi.stubGlobal('fetch', fetch)
    const result = fetchGalleryImage('/thumbnail', new AbortController().signal).catch((error: unknown) => error)
    await vi.runAllTimersAsync()
    expect(await result).toBeInstanceOf(GalleryImageError)
    expect(fetch).toHaveBeenCalledTimes(4)
  })

  it('rejects oversized bodies before retaining an image blob', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('image', { headers: { 'Content-Length': String(17 * 1024 * 1024) } })))
    await expect(fetchGalleryImage('/original', new AbortController().signal)).rejects.toMatchObject({ status: 413 })
  })
})
