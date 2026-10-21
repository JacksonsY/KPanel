// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import GalleryTimeline from './GalleryTimeline.vue'
import type { GalleryItem } from '@/lib/gallery'

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.useRealTimers(); document.body.innerHTML = '' })

it('mounts a bounded range, reveals a distant photo and updates geometry on density change', async () => {
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
  let width = 1280
  let minimum = 168
  const scroller = document.createElement('div')
  document.body.append(scroller)
  Object.defineProperty(scroller, 'clientHeight', { value: 800 })
  scroller.getBoundingClientRect = () => ({ top: 0, bottom: 800 }) as DOMRect
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(() => width)
  const rectangle = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    return { top: this === scroller ? 0 : 100 - scroller.scrollTop, bottom: 800 } as DOMRect
  })
  vi.stubGlobal('getComputedStyle', () => ({ getPropertyValue: (name: string) => name === '--gallery-tile' ? String(minimum) : '6' }))
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => window.setTimeout(() => callback(0), 0))
  vi.stubGlobal('cancelAnimationFrame', (id: number) => window.clearTimeout(id))
  const items = Array.from({ length: 5000 }, (_, index) => ({ entry: { path: `/p${index}` } }) as GalleryItem)
  const view = mount(GalleryTimeline, {
    props: { groups: [{ key: 'all', items }], featured: true, density: 'comfortable', resetKey: 'all', scrollRoot: scroller },
    slots: { default: '<button :data-gallery-path="params.item.entry.path">{{ params.item.entry.path }}</button>' },
    attachTo: scroller,
  })
  await flushPromises()
  await new Promise(resolve => setTimeout(resolve, 10))
  expect(view.findAll('button').length).toBeLessThan(240)
  expect(view.find('[data-gallery-path="/p0"]').exists()).toBe(true)
  // New thumbnails mount during scrolling, with recent nodes retained for backtracking.
  vi.useFakeTimers()
  const styleReads = vi.fn(() => ({ getPropertyValue: () => '' }))
  vi.stubGlobal('getComputedStyle', styleReads)
  const initial = view.findAll('button').map(button => button.attributes('data-gallery-path'))
  const first = view.get('[data-gallery-path="/p0"]').element
  scroller.scrollTop = 600
  scroller.dispatchEvent(new Event('scroll'))
  await vi.advanceTimersByTimeAsync(80)
  expect(view.get('[data-gallery-path="/p0"]').element).toBe(first)
  expect(view.findAll('button').length).toBeGreaterThan(initial.length)
  scroller.scrollTop = 0
  scroller.dispatchEvent(new Event('scroll'))
  await vi.advanceTimersByTimeAsync(80)
  expect(view.get('[data-gallery-path="/p0"]').element).toBe(first)
  scroller.scrollTop = 10000
  scroller.dispatchEvent(new Event('scroll'))
  await vi.advanceTimersByTimeAsync(80)
  expect(view.findAll('button').map(button => button.attributes('data-gallery-path'))).not.toEqual(initial)
  expect(view.findAll('.gallery-timeline__placeholder')).toHaveLength(0)
  expect(view.findAll('button').length).toBeLessThanOrEqual(240)
  scroller.scrollTop = 12000
  scroller.dispatchEvent(new Event('scroll'))
  await vi.advanceTimersByTimeAsync(159)
  expect(view.findAll('button').length).toBeLessThanOrEqual(240)
  expect(styleReads).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(1)
  expect(view.findAll('button').map(button => button.attributes('data-gallery-path'))).not.toEqual(initial)
  expect(view.findAll('.gallery-timeline__placeholder')).toHaveLength(0)
  expect(view.emitted('scrolling')).toEqual([[true], [false]])
  // An inactive desktop window neither mounts new cards nor starts another scroll cycle.
  await view.setProps({ active: false })
  const inactive = view.findAll('button').map(button => button.attributes('data-gallery-path'))
  scroller.scrollTop = 18000
  scroller.dispatchEvent(new Event('scroll'))
  await vi.advanceTimersByTimeAsync(500)
  expect(view.findAll('button').map(button => button.attributes('data-gallery-path'))).toEqual(inactive)
  await view.setProps({ active: true })
  await vi.advanceTimersByTimeAsync(20)
  expect(view.findAll('button').map(button => button.attributes('data-gallery-path'))).not.toEqual(inactive)
  vi.useRealTimers()
  vi.stubGlobal('getComputedStyle', () => ({ getPropertyValue: (name: string) => name === '--gallery-tile' ? String(minimum) : '6' }))
  view.vm.reveal('/p4500')
  await flushPromises()
  expect(view.find('[data-gallery-path="/p4500"]').exists()).toBe(true)
  expect(view.find('[data-gallery-path="/p0"]').exists()).toBe(false)
  expect(view.findAll('button').length).toBeLessThanOrEqual(240)
  width = 390
  minimum = 104
  await view.setProps({ density: 'compact', featured: false, resetKey: 'filtered', groups: [{ key: 'all', items: items.slice(0, 7) }] })
  await new Promise((resolve) => setTimeout(resolve, 10))
  await flushPromises()
  expect(view.find('[data-gallery-path="/p0"]').exists()).toBe(true)
  expect(view.findAll('button')).toHaveLength(7)
  const cell = view.find('.gallery-timeline__cell').element as HTMLElement
  expect(parseFloat(cell.style.width)).toBeCloseTo(126)
  view.unmount()
  rectangle.mockRestore()
  vi.restoreAllMocks()
})
