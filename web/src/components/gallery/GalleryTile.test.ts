// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { GalleryItem } from '@/lib/gallery'
import GalleryImage from './GalleryImage.vue'
import GalleryTile from './GalleryTile.vue'

const item: GalleryItem = {
  folder: '/photos', time: 1, kind: 'image',
  entry: { path: '/photos/a.jpg', name: 'a.jpg', kind: 'file', sizeBytes: 100,
    mode: '-rw-r--r--', owner: 'root', group: 'root', modifiedAt: '2026-10-12T01:00:00Z',
    resourceVersion: 'sha256:1', editable: false, previewable: true },
}

describe('gallery tile refresh', () => {
  it('keeps decoded images visible when revalidation returns an unchanged file', async () => {
    const view = mount(GalleryTile, {
      props: { item, imageUrl: '/thumbnail?v=1' },
      global: { stubs: { GalleryImage: { template: '<img />' } } },
    })
    view.findComponent(GalleryImage).vm.$emit('load', new Event('load'))
    await view.vm.$nextTick()
    expect(view.classes()).toContain('gallery-tile--loaded')
    await view.setProps({ item: { ...item, entry: { ...item.entry } } })
    expect(view.classes()).toContain('gallery-tile--loaded')
    await view.setProps({ item: { ...item, entry: { ...item.entry, resourceVersion: 'sha256:2' } }, imageUrl: '/thumbnail?v=2' })
    expect(view.classes()).not.toContain('gallery-tile--loaded')
    view.unmount()
  })
})
