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

it('admits visible images during a fling ahead of offscreen preloading', () => {
  vi.useFakeTimers()
  const policy = createGalleryRenderPolicy()
  policy.reset(0, 0); policy.interact(100, 20)
  const order: string[] = []
  policy.enqueue(() => order.push('offscreen'))
  policy.enqueue(() => order.push('visible'), true)
  vi.advanceTimersByTime(80)
  expect(order).toEqual(['visible'])
  policy.reset()
  vi.advanceTimersByTime(32)
  expect(order).toEqual(['visible', 'offscreen'])
  policy.dispose()
})

it('does not starve visible images when scroll speed keeps alternating across thresholds', () => {
  vi.useFakeTimers()
  const policy = createGalleryRenderPolicy()
  policy.reset(0, 0); policy.interact(100, 20)
  const visible = vi.fn()
  policy.enqueue(visible, true)
  let top = 100
  for (let i = 1; i <= 12; i++) {
    vi.advanceTimersByTime(20)
    top += i % 2 ? 4 : 24
    policy.interact(top, 20 + i * 20)
  }
  expect(visible).toHaveBeenCalledOnce()
  policy.dispose()
})

it('bounds native requests, reserves capacity for visible photos and reuses slots after completion', () => {
  vi.useFakeTimers()
  const policy = createGalleryRenderPolicy()
  const started: string[] = []
  const releaseBackground: (() => void)[] = []
  for (let i = 0; i < 20; i++) releaseBackground.push(policy.requestImage(() => started.push(`bg${i}`)))
  vi.advanceTimersByTime(200)
  expect(started).toEqual(['bg0'])
  const releaseVisible: (() => void)[] = []
  for (let i = 0; i < 4; i++) releaseVisible.push(policy.requestImage(() => started.push(`visible${i}`), true))
  vi.advanceTimersByTime(64)
  expect(started).toEqual(['bg0', 'visible0', 'visible1', 'visible2'])
  releaseBackground[0]!()
  vi.advanceTimersByTime(32)
  expect(started.at(-1)).toBe('visible3')
  releaseVisible[0]!()
  vi.advanceTimersByTime(32)
  expect(started.at(-1)).toBe('bg1')
  releaseBackground.forEach(release => release())
  releaseVisible.forEach(release => release())
  policy.dispose()
  expect(vi.getTimerCount()).toBe(0)
})
