// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import GalleryTimeline from './GalleryTimeline.vue'
import type { GalleryItem } from '@/lib/gallery'

afterEach(() => { vi.unstubAllGlobals(); document.body.innerHTML = '' })

it('mounts a bounded range, reveals a distant photo and updates geometry on density change', async () => {
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
  expect(view.findAll('button').length).toBeLessThan(120)
  expect(view.find('[data-gallery-path="/p0"]').exists()).toBe(true)
  view.vm.reveal('/p4500')
  await flushPromises()
  expect(view.find('[data-gallery-path="/p4500"]').exists()).toBe(true)
  expect(view.find('[data-gallery-path="/p0"]').exists()).toBe(false)
  expect(view.findAll('button').length).toBeLessThan(120)
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
