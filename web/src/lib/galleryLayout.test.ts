import { describe, expect, it } from 'vitest'
import { galleryViewport, layoutGallery } from './galleryLayout'
import type { GalleryItem } from './gallery'

const items = (count: number) => Array.from({ length: count }, (_, index) => ({ entry: { path: `/photo-${index}.jpg` } }) as GalleryItem)
describe('gallery geometry and viewport', () => {
  it('does not mount a library inside a hidden or not-yet-measured window', () => {
    expect(layoutGallery([{ key: 'all', items: items(5000) }], 0, 168, 6, true).cells).toEqual([])
  })
  it.each([112, 236, 390, 768, 1280])('places featured tiles without overlap at width %i', (width) => {
    const list = items(31)
    const result = layoutGallery([{ key: 'month', items: list }], width, 112, 6, true)
    expect(result.cells.filter((cell) => cell.item)).toHaveLength(list.length)
    for (const [index, cell] of result.cells.entries()) {
      expect(cell.left).toBeGreaterThanOrEqual(0)
      expect(cell.left + cell.width).toBeLessThanOrEqual(width + 0.001)
      expect(cell.top + cell.height).toBeLessThanOrEqual(result.height + 0.001)
      if (index) expect(cell.top).toBeGreaterThanOrEqual(result.cells[index - 1]!.top)
      for (const other of result.cells.slice(index + 1)) {
        const overlaps = cell.left < other.left + other.width - 0.001 && other.left < cell.left + cell.width - 0.001
          && cell.top < other.top + other.height - 0.001 && other.top < cell.top + cell.height - 0.001
        expect(overlaps).toBe(false)
      }
    }
  })
  it('bounds mounted work independently of library size, including return to the first viewport', () => {
    const result = layoutGallery([{ key: 'all', items: items(5000) }], 1280, 168, 6, true)
    for (const top of [0, 10000, result.height - 800, 0]) {
      const visible = galleryViewport(result.cells, top - 800, top + 1600, result.maxCellHeight)
      expect(visible.length).toBeLessThan(120)
      expect(visible.length).toBeGreaterThan(0)
      expect(visible.every((cell) => cell.top + cell.height >= top - 800 && cell.top <= top + 1600)).toBe(true)
    }
  })
  it('keeps a tall featured tile while its lower half remains visible and preserves month counts', () => {
    const list = items(20)
    const result = layoutGallery([{ key: 'a', items: list }, { key: 'b', items: items(7) }], 768, 168, 6, true)
    const featured = result.cells.find((cell) => cell.featured)!
    expect(galleryViewport(result.cells, featured.top + featured.height - 1, featured.top + featured.height + 200, result.maxCellHeight)).toContain(featured)
    expect(result.cells.filter((cell) => cell.group).map((cell) => cell.group!.items.length)).toEqual([20, 7])
    expect(galleryViewport([], 0, 800, 100)).toEqual([])
  })
})
