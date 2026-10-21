import { ref } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { createGalleryRenderPolicy } from './galleryRenderPolicy'

afterEach(() => vi.useRealTimers())

it.each([60, 120, 240])('classifies the same physical scroll speed at %i events per second', hz => {
  vi.useFakeTimers()
  const policy = createGalleryRenderPolicy()
  policy.reset(0, 0)
  for (let i = 1; i <= hz / 4; i++) policy.interact(i * 1200 / hz, i * 1000 / hz)
  expect(policy.phase.value).toBe('fast')
  // Slow motion resumes admission before the inactivity deadline.
  for (let i = 1; i <= hz / 4; i++) policy.interact(300 + i * 200 / hz, 250 + i * 1000 / hz)
  expect(policy.phase.value).toBe('slow')
  vi.advanceTimersByTime(160)
  expect(policy.phase.value).toBe('idle')
  policy.dispose()
})

it('spreads resumed work across batches, supports cancellation and leaves no timers after disposal', () => {
  vi.useFakeTimers()
  const policy = createGalleryRenderPolicy()
  policy.reset(0, 0)
  policy.interact(100, 20)
  policy.interact(100, 40) // A wheel notification without displacement must not unpause a fling.
  expect(policy.phase.value).toBe('fast')
  const calls: number[] = []
  for (let i = 0; i < 10; i++) policy.enqueue(() => calls.push(i))
  const cancel = policy.enqueue(() => calls.push(99)); cancel()
  vi.advanceTimersByTime(80)
  expect(calls).toEqual([])
  policy.interact(102, 100)
  vi.advanceTimersByTime(32)
  expect(calls).toEqual([0, 1, 2, 3])
  vi.advanceTimersByTime(32)
  expect(calls).toEqual([0, 1, 2, 3, 4, 5, 6, 7])
  policy.dispose()
  vi.advanceTimersByTime(1000)
  expect(calls).toHaveLength(8)
  expect(vi.getTimerCount()).toBe(0)
})

it('keeps surfaces independent and prevents queued work while inactive', () => {
  vi.useFakeTimers()
  const active = ref(true)
  const page = createGalleryRenderPolicy(active), viewer = createGalleryRenderPolicy()
  const hidden = vi.fn(), visible = vi.fn()
  page.reset(0, 0); page.interact(100, 20)
  page.enqueue(hidden); viewer.enqueue(visible)
  active.value = false
  vi.advanceTimersByTime(200)
  expect(hidden).not.toHaveBeenCalled()
  expect(visible).toHaveBeenCalledOnce()
  expect(page.phase.value).toBe('idle')
  active.value = true
  vi.advanceTimersByTime(32)
  expect(hidden).toHaveBeenCalledOnce()
  page.dispose(); viewer.dispose()
})
