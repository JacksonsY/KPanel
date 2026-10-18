// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { GalleryImageCache } from './galleryImageCache'

let cache: GalleryImageCache
let sequence = 0
beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('URL', { createObjectURL: vi.fn(() => `blob:${++sequence}`), revokeObjectURL: vi.fn() })
  cache = new GalleryImageCache({ count: 2, bytes: 8, decoded: 64, itemBytes: 4, ttl: 100 })
})
afterEach(() => { cache.dispose(); vi.unstubAllGlobals(); vi.useRealTimers() })

it('shares the active URL and revokes only after the last active owner, even after expiry', () => {
  const first = cache.store('a:v1', new Blob(['aa']))
  const second = cache.acquire('a:v1')!
  expect(second.url).toBe(first.url)
  first.release()
  vi.advanceTimersByTime(101)
  expect(cache.acquire('a:v1')).toBeUndefined()
  expect(URL.revokeObjectURL).not.toHaveBeenCalled()
  second.release()
  second.release()
  expect(URL.revokeObjectURL).toHaveBeenCalledExactlyOnceWith(first.url)
})

it('evicts the least recently used idle image under count pressure', () => {
  cache.store('a', new Blob(['aa'])).release()
  const b = cache.store('b', new Blob(['bb'])); b.release()
  cache.acquire('a')!.release()
  cache.store('c', new Blob(['cc'])).release()
  expect(cache.acquire('b')).toBeUndefined()
  expect(URL.revokeObjectURL).toHaveBeenCalledWith(b.url)
  const a = cache.acquire('a'); expect(a).toBeDefined(); a!.release()
})

it('bounds encoded and estimated decoded sizes without flushing small images for an oversized one', () => {
  cache.store('small', new Blob(['aa'])).release()
  const huge = cache.store('huge', new Blob(['12345'])); huge.release()
  expect(cache.acquire('huge')).toBeUndefined()
  const decoded = cache.store('decoded', new Blob(['bb']))
  decoded.decoded(100, 100)
  decoded.release()
  expect(cache.acquire('decoded')).toBeUndefined()
  const small = cache.acquire('small'); expect(small).toBeDefined(); small!.release()
  const tight = new GalleryImageCache({ count: 8, bytes: 3, decoded: 64, itemBytes: 4, ttl: 100 })
  tight.store('a', new Blob(['aa'])).release()
  tight.store('b', new Blob(['bb'])).release()
  expect(tight.acquire('a')).toBeUndefined()
  tight.dispose()
})

it('invalidates bad decodes and isolates old requests across clear/dispose', () => {
  const staleGeneration = cache.generation
  const image = cache.store('bad', new Blob(['aa']))
  image.invalidate()
  expect(cache.acquire('bad')).toBeUndefined()
  image.release()
  cache.clear()
  cache.store('late', new Blob(['aa']), staleGeneration).release()
  expect(cache.acquire('late')).toBeUndefined()
  cache.store('fresh', new Blob(['aa'])).release()
  const pinned = cache.acquire('fresh')!
  cache.dispose()
  expect(URL.revokeObjectURL).not.toHaveBeenCalledWith(pinned.url)
  pinned.release()
  expect(URL.revokeObjectURL).toHaveBeenCalledWith(pinned.url)
  cache.store('closed', new Blob(['aa'])).release()
  expect(cache.acquire('closed')).toBeUndefined()
})

it('keeps different resource versions separate and never extends the freshness deadline on a hit', () => {
  const one = cache.store('a:v1', new Blob(['aa'])); one.release()
  vi.advanceTimersByTime(90)
  const hit = cache.acquire('a:v1')!; hit.release()
  cache.store('a:v2', new Blob(['bb'])).release()
  vi.advanceTimersByTime(11)
  expect(cache.acquire('a:v1')).toBeUndefined()
  const two = cache.acquire('a:v2'); expect(two).toBeDefined(); two!.release()
})

it('retains a thousand compressed thumbnails without retaining their display URLs', () => {
  const library = new GalleryImageCache()
  for (let index = 0; index < 1000; index += 1) {
    const image = library.store(`photo:${index}`, new Blob([new Uint8Array(8192)]))
    image.decoded(320, 210)
    image.release()
  }
  expect(URL.revokeObjectURL).toHaveBeenCalledTimes(1000)
  const returned = library.acquire('photo:0')!
  expect(returned).toBeDefined()
  returned.release()
  library.dispose()
})
