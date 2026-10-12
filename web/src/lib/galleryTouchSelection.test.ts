// @vitest-environment jsdom
import { expect, it, vi } from 'vitest'
import { createGalleryTouchSelection } from './galleryTouchSelection'

it('updates each hit path once and releases the pending animation on cancellation', () => {
  vi.useFakeTimers()
  const root = document.createElement('div')
  root.innerHTML = '<div data-gallery-path="/a.jpg"></div><div data-gallery-path="/b.jpg"></div>'
  const [a, b] = [...root.children]
  const original = document.elementFromPoint
  document.elementFromPoint = vi.fn(() => b!)
  const begin = vi.fn()
  const update = vi.fn()
  const gesture = createGalleryTouchSelection({ root: () => root, selecting: () => true, begin, update })
  try {
    gesture.start({ target: a, touches: [{ identifier: 1, clientX: 100, clientY: 200 }] } as unknown as TouchEvent)
    vi.advanceTimersByTime(401)
    expect(begin).toHaveBeenCalledWith('/a.jpg')
    for (let index = 0; index < 3; index++) {
      gesture.move({ touches: [{ identifier: 1, clientX: 130 + index, clientY: 200 }], cancelable: true, preventDefault: vi.fn() } as unknown as TouchEvent)
    }
    expect(update.mock.calls).toEqual([['/b.jpg']])
    gesture.cancel()
    expect(vi.getTimerCount()).toBe(0)
  } finally {
    gesture.cancel()
    document.elementFromPoint = original
    vi.useRealTimers()
  }
})
