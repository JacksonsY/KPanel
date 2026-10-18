import type { GalleryItem, GalleryMonthGroup } from './gallery'

export interface GalleryLayoutCell {
  key: string
  top: number
  left: number
  width: number
  height: number
  item?: GalleryItem
  group?: GalleryMonthGroup
  featured: boolean
}

/** One geometry model for rendering, viewport lookup and returning from the viewer. */
export function layoutGallery(groups: GalleryMonthGroup[], width: number, minimum: number, gap: number, featured: boolean) {
  if (width <= 0) return { cells: [] as GalleryLayoutCell[], height: 0, maxCellHeight: 40 }
  const columns = Math.max(1, Math.floor((width + gap) / (minimum + gap)))
  const size = Math.max(1, (width - (columns - 1) * gap) / columns)
  const cells: GalleryLayoutCell[] = []
  let top = 0
  for (const group of groups) {
    if (group.key !== 'all') {
      cells.push({ key: `month:${group.key}`, group, top, left: 0, width, height: 40, featured: false })
      top += 40
    }
    const large = featured && columns >= 2 && group.items.length >= 5
    const beside = 2 * (columns - 2)
    let bottom = top
    group.items.forEach((item, index) => {
      let row = Math.floor(index / columns)
      let column = index % columns
      const span = large && index === 0 ? 2 : 1
      if (large && index > 0) {
        const rest = index - 1
        if (rest < beside) {
          row = Math.floor(rest / (columns - 2))
          column = 2 + rest % (columns - 2)
        } else {
          row = 2 + Math.floor((rest - beside) / columns)
          column = (rest - beside) % columns
        }
      }
      const height = span * size + (span - 1) * gap
      const y = top + row * (size + gap)
      cells.push({ key: item.entry.path, item, top: y, left: column * (size + gap), width: height, height, featured: span === 2 })
      bottom = Math.max(bottom, y + height)
    })
    top = bottom + 24
  }
  return { cells, height: Math.max(0, top - 24), maxCellHeight: size * 2 + gap }
}

/** Cells are ordered by their top edge; scrolling only inspects the nearby range. */
export function galleryViewport(cells: GalleryLayoutCell[], top: number, bottom: number, maxCellHeight: number) {
  let low = 0
  let high = cells.length
  while (low < high) {
    const middle = (low + high) >>> 1
    if (cells[middle]!.top < top - maxCellHeight) low = middle + 1
    else high = middle
  }
  const visible: GalleryLayoutCell[] = []
  for (let index = low; index < cells.length; index += 1) {
    const cell = cells[index]!
    if (cell.top > bottom) break
    if (cell.top + cell.height >= top) visible.push(cell)
  }
  return visible
}
