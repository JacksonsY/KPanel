// @vitest-environment jsdom
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GalleryViewer from './GalleryViewer.vue'
import type { GalleryItem } from '@/lib/gallery'
import type { FileEntry } from '@/types/api'

function item(name: string): GalleryItem {
  const entry: FileEntry = {
    name, path: `/home/gallery/${name}`, kind: 'file', mime: 'image/jpeg', sizeBytes: 2048, mode: '-rw-r--r--',
    owner: 'root', group: 'root', modifiedAt: '2026-09-10T08:00:00Z', resourceVersion: `sha256:${'d'.repeat(64)}`,
    editable: false, previewable: true,
  }
  return { entry, kind: 'image', folder: '/home/gallery', time: Date.parse(entry.modifiedAt) }
}

let wrapper: VueWrapper | undefined

function mountViewer(props: Record<string, unknown> = {}): VueWrapper {
  wrapper = mount(GalleryViewer, {
    props: {
      items: [item('a.jpg'), item('b.jpg'), item('c.jpg')],
      index: 1,
      sources: (value: GalleryItem) => ({ original: `/content${value.entry.path}` }),
      filmstrip: true,
      ...props,
    },
    attachTo: document.body,
  })
  return wrapper
}

beforeEach(() => vi.useFakeTimers())

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

function mockOriginals() {
  let sequence = 0
  const NativeURL = URL
  vi.stubGlobal('URL', class extends NativeURL {
    static createObjectURL = vi.fn(() => `blob:original-${++sequence}`)
    static revokeObjectURL = vi.fn()
  })
  const fetch = vi.fn().mockImplementation(() => Promise.resolve(new Response('jpeg')))
  vi.stubGlobal('fetch', fetch)
  return fetch
}

describe('viewer recent original cache', () => {
  it('reuses a recent original on back navigation and releases all URLs on close', async () => {
    const fetch = mockOriginals()
    const view = mountViewer({ filmstrip: false })
    await flushPromises()
    const first = view.get('img.gallery-viewer__media').attributes('src')
    await view.setProps({ index: 2 }); await flushPromises()
    await view.setProps({ index: 1 }); await flushPromises()
    expect(view.get('img.gallery-viewer__media').attributes('src')).toBe(first)
    expect(fetch).toHaveBeenCalledTimes(2)
    view.unmount(); wrapper = undefined
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(2)
  })

  it('reloads changed versions and retries a failed decode instead of retaining it', async () => {
    const fetch = mockOriginals()
    const view = mountViewer({ filmstrip: false })
    await flushPromises()
    const changed = item('b.jpg'); changed.entry.resourceVersion = 'v2'
    await view.setProps({ items: [item('a.jpg'), changed, item('c.jpg')] }); await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(2)
    await view.get('img.gallery-viewer__media').trigger('error')
    await view.get('.gallery-viewer__notice-actions button').trigger('click'); await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(3)
    expect(view.find('.gallery-viewer__notice').exists()).toBe(false)
    expect(document.activeElement).toBe(view.element)
  })

  it('bypasses unknown and oversized originals and falls back to native loading after fetch failure', async () => {
    const fetch = mockOriginals()
    const huge = item('huge.jpg'); huge.entry.sizeBytes = 20 * 1024 * 1024
    const unknown = item('unknown.jpg'); unknown.entry.sizeBytes = 0
    const view = mountViewer({ items: [huge, unknown, item('c.jpg')], index: 0, filmstrip: false })
    expect(view.get('img.gallery-viewer__media').attributes('src')).toBe('/content/home/gallery/huge.jpg')
    await view.setProps({ index: 1 }); await flushPromises()
    expect(fetch).not.toHaveBeenCalled()
    fetch.mockRejectedValueOnce(new Error('offline'))
    await view.setProps({ index: 2 }); await flushPromises()
    expect(view.get('img.gallery-viewer__media').attributes('src')).toBe('/content/home/gallery/c.jpg')
    expect(view.find('.gallery-viewer__notice').exists()).toBe(false)
  })

  it('aborts obsolete requests, ignores late completion and cancels on close', async () => {
    const fetch = mockOriginals()
    let complete!: (response: Response) => void
    fetch.mockImplementationOnce(() => new Promise<Response>(resolve => { complete = resolve }))
    const view = mountViewer({ filmstrip: false }); await flushPromises()
    const signal = fetch.mock.calls[0]![1].signal as AbortSignal
    await view.setProps({ index: 2 }); await flushPromises()
    expect(signal.aborted).toBe(true)
    const current = view.get('img.gallery-viewer__media').attributes('src')
    complete(new Response('late')); await flushPromises()
    expect(view.get('img.gallery-viewer__media').attributes('src')).toBe(current)
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1)
    view.unmount(); wrapper = undefined
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(1)
  })

  it('evicts old originals after three retained entries and bypasses excessive decoded dimensions', async () => {
    const fetch = mockOriginals()
    const view = mountViewer({ items: ['a','b','c','d'].map(n => item(`${n}.jpg`)), index: 0, filmstrip: false })
    await flushPromises()
    for (const index of [1,2,3,0]) { await view.setProps({ index }); await flushPromises() }
    expect(fetch).toHaveBeenCalledTimes(5)
    const image = view.get('img.gallery-viewer__media')
    Object.defineProperties(image.element, { naturalWidth: { value: 6000 }, naturalHeight: { value: 4000 } })
    await image.trigger('load')
    await view.setProps({ index: 1 }); await flushPromises()
    await view.setProps({ index: 0 }); await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(7)
  })
})

async function dispatchPointer(element: Element, type: string, init: MouseEventInit & { pointerId: number; pointerType: string }) {
  const event = new MouseEvent(type, { bubbles: true, ...init })
  Object.defineProperties(event, { pointerId: { value: init.pointerId }, pointerType: { value: init.pointerType } })
  element.dispatchEvent(event)
  await nextTick()
}
describe('GalleryViewer controls', () => {
  it('tracks a swipe under the finger, rejects diagonal drags and resets after cancellation', async () => {
    const view = mountViewer()
    const stage = view.get('.gallery-viewer__stage')
    const event = (id: number, x: number, y: number) => ({ pointerId: id, pointerType: 'touch', button: 0, clientX: x, clientY: y })
    await dispatchPointer(stage.element, 'pointerdown', event(1, 200, 200))
    await dispatchPointer(stage.element, 'pointermove', event(1, 100, 200))
    expect(view.get('img.gallery-viewer__media').attributes('style')).toContain('-100px')
    await dispatchPointer(stage.element, 'pointerup', event(1, 100, 200))
    expect(view.emitted('navigate')).toEqual([[2]])
    await dispatchPointer(stage.element, 'pointerdown', event(2, 200, 200))
    await dispatchPointer(stage.element, 'pointermove', event(2, 100, 125))
    await dispatchPointer(stage.element, 'pointerup', event(2, 100, 125))
    expect(view.emitted('navigate')).toHaveLength(1)
    await dispatchPointer(stage.element, 'pointerdown', event(3, 200, 200))
    await dispatchPointer(stage.element, 'pointermove', event(3, 100, 200))
    await dispatchPointer(stage.element, 'pointercancel', event(3, 100, 200))
    await dispatchPointer(stage.element, 'pointerup', event(3, 100, 200))
    expect(view.emitted('navigate')).toHaveLength(1)
    expect(stage.classes()).not.toContain('is-interacting')
  })

  it('pinches to zoom, pans with the remaining finger and never turns a pinch into navigation', async () => {
    const view = mountViewer()
    const stage = view.get('.gallery-viewer__stage')
    vi.spyOn(stage.element, 'getBoundingClientRect').mockReturnValue({ left: 0, top: 0, width: 400, height: 600 } as DOMRect)
    await view.get('img.gallery-viewer__media').trigger('load')
    const event = (id: number, x: number) => ({ pointerId: id, pointerType: 'touch', button: 0, clientX: x, clientY: 300 })
    await dispatchPointer(stage.element, 'pointerdown', event(1, 150))
    await dispatchPointer(stage.element, 'pointerdown', event(2, 250))
    await dispatchPointer(stage.element, 'pointermove', event(2, 350))
    expect(view.get('img.gallery-viewer__media').attributes('style')).toContain('scale(2)')
    await dispatchPointer(stage.element, 'pointerup', event(2, 350))
    await dispatchPointer(stage.element, 'pointermove', event(1, 50))
    await dispatchPointer(stage.element, 'pointerup', event(1, 50))
    expect(view.emitted('navigate')).toBeUndefined()
    expect(stage.classes()).not.toContain('is-interacting')
    await dispatchPointer(stage.element, 'pointerdown', event(3, 150))
    await dispatchPointer(stage.element, 'pointerdown', event(4, 250))
    await dispatchPointer(stage.element, 'pointermove', event(4, 151))
    expect(view.get('img.gallery-viewer__media').attributes('style')).toContain('scale(1)')
    await dispatchPointer(stage.element, 'pointerup', event(4, 151))
    await dispatchPointer(stage.element, 'pointermove', event(3, 40))
    await dispatchPointer(stage.element, 'pointerup', event(3, 40))
    expect(view.emitted('navigate')).toBeUndefined()
  })

  it('fades the bar, arrows and strip when idle and brings them back on movement', async () => {
    const view = mountViewer()
    expect(view.classes()).not.toContain('gallery-viewer--idle')

    vi.advanceTimersByTime(2900)
    await view.vm.$nextTick()
    expect(view.classes()).toContain('gallery-viewer--idle')

    await view.trigger('pointermove', { pointerType: 'mouse' })
    expect(view.classes()).not.toContain('gallery-viewer--idle')
  })

  it('keeps the controls while the pointer rests on them', async () => {
    const view = mountViewer()
    await view.get('.gallery-viewer__bar').trigger('pointerenter')
    vi.advanceTimersByTime(6000)
    await view.vm.$nextTick()
    expect(view.classes()).not.toContain('gallery-viewer--idle')

    await view.get('.gallery-viewer__bar').trigger('pointerleave')
    vi.advanceTimersByTime(2900)
    await view.vm.$nextTick()
    expect(view.classes()).toContain('gallery-viewer--idle')
  })

  it('pages with the arrow keys without waking the controls, and wakes them for other keys', async () => {
    const view = mountViewer()
    vi.advanceTimersByTime(2900)
    await view.vm.$nextTick()
    await view.trigger('keydown', { key: 'ArrowRight' })
    expect(view.emitted('navigate')?.[0]).toEqual([2])
    expect(view.classes()).toContain('gallery-viewer--idle')

    await view.trigger('keydown', { key: 'i' })
    expect(view.classes()).not.toContain('gallery-viewer--idle')
  })

  it('toggles the thumbnail strip through its own button', async () => {
    const view = mountViewer()
    expect(view.find('.gallery-viewer__strip').exists()).toBe(true)
    await view.get('button[aria-label="隐藏缩略图条"]').trigger('click')
    expect(view.emitted('update:filmstrip')?.[0]).toEqual([false])

    await view.setProps({ filmstrip: false })
    expect(view.find('.gallery-viewer__strip').exists()).toBe(false)
    expect(view.get('button[aria-label="显示缩略图条"]').attributes('aria-pressed')).toBe('false')
  })

  it('centres the current thumbnail by scrolling the strip only, never its ancestors', async () => {
    const intoView = vi.fn()
    const scrollTo = vi.fn()
    Element.prototype.scrollIntoView = intoView
    Element.prototype.scrollTo = scrollTo as unknown as typeof Element.prototype.scrollTo
    const view = mountViewer({ items: Array.from({ length: 30 }, (_, index) => item(`p${index}.jpg`)), index: 0 })
    await view.setProps({ index: 25 })
    await view.vm.$nextTick()
    // scrollIntoView would also scroll the overflow-hidden viewer sideways.
    expect(intoView).not.toHaveBeenCalled()
    expect(scrollTo).toHaveBeenCalled()
    expect(view.element.scrollLeft).toBe(0)
  })

  it('keeps the controls while the cover menu is open and closes it before the viewer', async () => {
    const view = mountViewer({ coverOptions: [{ id: 'own', label: '设为「图库」封面', active: false }] })
    await view.get('button[aria-label="设为封面"]').trigger('click')
    expect(view.find('.gallery-viewer__cover-menu').exists()).toBe(true)
    vi.advanceTimersByTime(6000)
    await view.vm.$nextTick()
    expect(view.classes()).not.toContain('gallery-viewer--idle')

    await view.trigger('keydown', { key: 'Escape' })
    expect(view.find('.gallery-viewer__cover-menu').exists()).toBe(false)
    expect(view.emitted('close')).toBeUndefined()
    await view.get('button[aria-label="设为封面"]').trigger('click')
    await view.get('.gallery-viewer__cover-menu button').trigger('click')
    expect(view.emitted('cover')?.[0]).toEqual([{ item: expect.objectContaining({ kind: 'image' }), option: 'own' }])
    expect(view.find('.gallery-viewer__cover-menu').exists()).toBe(false)
  })

  it('shows no cover button without options', () => {
    expect(mountViewer().find('button[aria-label="设为封面"]').exists()).toBe(false)
  })

  it('asks the page to move the open photo, and only offers it when the page can', async () => {
    expect(mountViewer().find('button[aria-label="移动到相册"]').exists()).toBe(false)
    wrapper?.unmount()
    const view = mountViewer({ canMove: true })
    await view.get('button[aria-label="移动到相册"]').trigger('click')
    expect(view.emitted('move')?.[0]).toEqual([expect.objectContaining({ entry: expect.objectContaining({ name: 'b.jpg' }) })])
  })

  it('has no strip toggle for a single photo', () => {
    const view = mountViewer({ items: [item('only.jpg')], index: 0 })
    expect(view.find('button[aria-label="隐藏缩略图条"]').exists()).toBe(false)
    expect(view.find('.gallery-viewer__strip').exists()).toBe(false)
  })
})
