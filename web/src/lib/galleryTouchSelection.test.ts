// @vitest-environment jsdom
import { expect, it, vi } from 'vitest'
import { createGalleryTouchSelection } from './galleryTouchSelection'

it('never activates selection from a browsing touch, even after a long press', () => {
  vi.useFakeTimers()
  const root = document.createElement('div')
  root.innerHTML = '<div data-gallery-path="/a.jpg"></div>'
  const begin = vi.fn()
  const gesture = createGalleryTouchSelection({ root: () => root, selecting: () => false, begin, update: vi.fn() })
  try {
    gesture.start({ target: root.firstElementChild, touches: [{ identifier: 1, clientX: 10, clientY: 10 }] } as unknown as TouchEvent)
    vi.advanceTimersByTime(1000)
    gesture.move({ touches: [{ identifier: 1, clientX: 100, clientY: 10 }] } as unknown as TouchEvent)
    expect(begin).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  } finally { gesture.cancel(); vi.useRealTimers() }
})

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
