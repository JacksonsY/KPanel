<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { GalleryMonthGroup } from '@/lib/gallery'
import { galleryViewport, layoutGallery } from '@/lib/galleryLayout'

const props = defineProps<{
  groups: GalleryMonthGroup[]
  featured: boolean
  density: string
  scrollRoot?: HTMLElement
  resetKey: string
}>()
const root = ref<HTMLElement>()
const width = ref(0)
const minimum = ref(168)
const gap = ref(6)
const viewport = ref({ top: 0, height: 800 })
const focused = ref('')
let observer: ResizeObserver | undefined
let frame: number | undefined
let scrollTarget: HTMLElement | Window | undefined
let scale = 1
const layout = computed(() => layoutGallery(props.groups, width.value, minimum.value, gap.value, props.featured))
const cells = computed(() => {
  // One screen ahead/behind absorbs normal wheel and touch scrolling without mounting the library.
  const { top, height } = viewport.value
  const result = galleryViewport(layout.value.cells, top - height, top + 2 * height, layout.value.maxCellHeight)
  const pinned = focused.value && layout.value.cells.find((cell) => cell.key === focused.value)
  if (pinned && !result.includes(pinned)) result.push(pinned)
  return result
})

function measure(): void {
  frame = undefined
  const element = root.value
  if (!element) return
  const style = getComputedStyle(element)
  width.value = element.clientWidth
  minimum.value = parseFloat(style.getPropertyValue('--gallery-tile')) || 168
  gap.value = parseFloat(style.getPropertyValue('--gallery-gap')) || 6
  const bounds = element.getBoundingClientRect()
  scale = element.clientWidth > 0 && bounds.width > 0 ? bounds.width / element.clientWidth : 1
  const container = props.scrollRoot?.getBoundingClientRect()
  viewport.value = { top: ((container?.top ?? 0) - bounds.top) / scale, height: props.scrollRoot?.clientHeight ?? window.innerHeight / scale }
}
function schedule(): void {
  if (frame === undefined) frame = requestAnimationFrame(measure)
}
function bindScroll(): void {
  scrollTarget?.removeEventListener('scroll', schedule)
  scrollTarget = props.scrollRoot ?? window
  scrollTarget.addEventListener('scroll', schedule, { passive: true })
  measure()
}
function reveal(path: string): void {
  const cell = layout.value.cells.find((entry) => entry.key === path)
  if (!cell || !root.value) return
  measure()
  const { top, height } = viewport.value
  if (cell.top >= top && cell.top + cell.height <= top + height) return
  const delta = cell.top - top - Math.max(0, (height - cell.height) / 2)
  if (props.scrollRoot) props.scrollRoot.scrollTop += delta
  else window.scrollBy({ top: delta * scale, behavior: 'instant' })
  measure()
}
function focusChanged(): void {
  const tile = document.activeElement?.closest<HTMLElement>('[data-gallery-path]')
  focused.value = tile && root.value?.contains(tile) ? tile.dataset.galleryPath ?? '' : ''
}
defineExpose({ reveal })
watch(() => props.scrollRoot, (next, previous) => {
  if (previous) observer?.unobserve(previous)
  if (next) observer?.observe(next)
  bindScroll()
}, { flush: 'post' })
watch(() => [props.featured, props.density, props.groups], schedule, { flush: 'post' })
watch(() => props.resetKey, async () => {
  focused.value = ''
  await nextTick()
  if (!root.value) return
  measure()
  // Filtering a long list must not leave the viewport below its new end.
  if (viewport.value.top > 0) {
    if (props.scrollRoot) props.scrollRoot.scrollTop -= viewport.value.top
    else window.scrollBy({ top: -viewport.value.top * scale, behavior: 'instant' })
  }
  measure()
}, { flush: 'post' })
onMounted(() => {
  bindScroll()
  observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(schedule)
  if (root.value) observer?.observe(root.value)
  if (props.scrollRoot) observer?.observe(props.scrollRoot)
  window.addEventListener('resize', schedule, { passive: true })
})
onBeforeUnmount(() => {
  observer?.disconnect()
  scrollTarget?.removeEventListener('scroll', schedule)
  window.removeEventListener('resize', schedule)
  if (frame !== undefined) cancelAnimationFrame(frame)
})
</script>

<template>
  <div ref="root" class="gallery-timeline" :style="{ height: `${layout.height}px` }" @focusin="focusChanged" @focusout="focusChanged">
    <div
      v-for="cell in cells"
      :key="cell.key"
      class="gallery-timeline__cell"
      :style="{ top: `${cell.top}px`, left: `${cell.left}px`, width: `${cell.width}px`, height: `${cell.height}px` }"
    >
      <slot v-if="cell.item" :item="cell.item" :featured="cell.featured" />
      <slot v-else-if="cell.group" name="month" :group="cell.group" />
    </div>
  </div>
</template>

<style scoped>
.gallery-timeline { position: relative; min-width: 0; overflow-anchor: none; }
.gallery-timeline__cell { position: absolute; }
.gallery-timeline__cell :deep(.gallery-tile) { width: 100%; height: 100%; }
</style>
