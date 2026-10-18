<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import type { GalleryMonthGroup } from '@/lib/gallery'
import { galleryViewport, layoutGallery, type GalleryLayoutCell } from '@/lib/galleryLayout'

const props = withDefaults(defineProps<{
  groups: GalleryMonthGroup[]
  featured: boolean
  density: string
  scrollRoot?: HTMLElement
  resetKey: string
  active?: boolean
}>(), { active: true })
const emit = defineEmits<{ scrolling: [value: boolean] }>()
const SETTLE_MS = 160
const root = ref<HTMLElement>()
const width = ref(0)
const minimum = ref(168)
const gap = ref(6)
const cells = shallowRef<GalleryLayoutCell[]>([])
const placeholders = shallowRef<GalleryLayoutCell[]>([])
const moving = ref(false)
let observer: ResizeObserver | undefined
let frame: number | undefined
let settleTimer: ReturnType<typeof setTimeout> | undefined
let scrollTarget: HTMLElement | Window | undefined
let geometryDirty = true
let contentTop = 0
let viewportHeight = 800
let scale = 1
let focused = ''
const layout = computed(() => layoutGallery(props.groups, width.value, minimum.value, gap.value, props.featured))
const scrollPosition = () => props.scrollRoot?.scrollTop ?? window.scrollY / scale
const viewportTop = () => scrollPosition() - contentTop
const enabled = () => props.active !== false && document.visibilityState !== 'hidden'
const same = (left: GalleryLayoutCell[], right: GalleryLayoutCell[]) => left.length === right.length && left.every((cell, index) => cell === right[index])

function measureGeometry(): void {
  const element = root.value
  if (!element) return
  const style = getComputedStyle(element)
  width.value = element.clientWidth
  minimum.value = parseFloat(style.getPropertyValue('--gallery-tile')) || 168
  gap.value = parseFloat(style.getPropertyValue('--gallery-gap')) || 6
  const bounds = element.getBoundingClientRect()
  scale = element.clientWidth > 0 && bounds.width > 0 ? bounds.width / element.clientWidth : 1
  const container = props.scrollRoot?.getBoundingClientRect()
  contentTop = (bounds.top - (container?.top ?? 0)) / scale + scrollPosition()
  viewportHeight = props.scrollRoot?.clientHeight ?? window.innerHeight / scale
  geometryDirty = false
}
function updateViewport(): void {
  frame = undefined
  if (!enabled()) return
  if (geometryDirty) measureGeometry()
  const top = viewportTop()
  // Pixel-bounded overscan avoids multiplying a maximized screen by three.
  const buffer = Math.min(320, viewportHeight * 0.35)
  const near = galleryViewport(layout.value.cells, top - buffer, top + viewportHeight + buffer, layout.value.maxCellHeight)
  if (moving.value) {
    const retained = new Set(cells.value.map((cell) => cell.key))
    const pending = near.filter((cell) => cell.item && !retained.has(cell.key))
    if (!same(placeholders.value, pending)) placeholders.value = pending
    return
  }
  const pinned = focused && layout.value.cells.find((cell) => cell.key === focused)
  if (pinned && !near.includes(pinned)) near.push(pinned)
  if (!same(cells.value, near)) cells.value = near
  if (placeholders.value.length) placeholders.value = []
}
function schedule(): void {
  if (frame === undefined && enabled()) frame = requestAnimationFrame(updateViewport)
}
function setMoving(value: boolean): void {
  if (moving.value === value) return
  moving.value = value
  emit('scrolling', value)
}
function settle(): void {
  clearTimeout(settleTimer)
  settleTimer = undefined
  setMoving(false)
  updateViewport()
}
function interact(): void {
  if (!enabled()) return
  setMoving(true)
  clearTimeout(settleTimer)
  settleTimer = setTimeout(settle, SETTLE_MS)
  schedule()
}
function resize(): void {
  geometryDirty = true
  interact()
}
function bindScroll(): void {
  scrollTarget?.removeEventListener('scroll', interact)
  scrollTarget?.removeEventListener('wheel', interact)
  scrollTarget = props.scrollRoot ?? window
  scrollTarget.addEventListener('scroll', interact, { passive: true })
  // Pause pending media before the browser dispatches the following scroll event.
  scrollTarget.addEventListener('wheel', interact, { passive: true })
  geometryDirty = true
  schedule()
}
function reveal(path: string): void {
  if (geometryDirty) measureGeometry()
  const cell = layout.value.cells.find((entry) => entry.key === path)
  if (!cell || !root.value) return
  const top = viewportTop()
  if (cell.top < top || cell.top + cell.height > top + viewportHeight) {
    const delta = cell.top - top - Math.max(0, (viewportHeight - cell.height) / 2)
    if (props.scrollRoot) props.scrollRoot.scrollTop += delta
    else window.scrollBy({ top: delta * scale, behavior: 'instant' })
  }
  // Returning from the viewer needs the destination mounted before focus restoration.
  clearTimeout(settleTimer)
  setMoving(false)
  measureGeometry()
  updateViewport()
}
function focusChanged(): void {
  const tile = document.activeElement?.closest<HTMLElement>('[data-gallery-path]')
  focused = tile && root.value?.contains(tile) ? tile.dataset.galleryPath ?? '' : ''
}
function activityChanged(): void {
  clearTimeout(settleTimer)
  if (frame !== undefined) cancelAnimationFrame(frame)
  frame = undefined
  setMoving(false)
  geometryDirty = true
  schedule()
}
defineExpose({ reveal })
watch(() => props.scrollRoot, (next, previous) => {
  if (previous) observer?.unobserve(previous)
  if (next) observer?.observe(next)
  bindScroll()
}, { flush: 'post' })
watch(() => [props.featured, props.density, props.groups], () => { geometryDirty = true; updateViewport() }, { flush: 'post' })
watch(() => props.active, activityChanged, { flush: 'post' })
watch(() => props.resetKey, async () => {
  focused = ''
  await nextTick()
  if (!root.value) return
  measureGeometry()
  const top = viewportTop()
  if (top > 0) {
    if (props.scrollRoot) props.scrollRoot.scrollTop -= top
    else window.scrollBy({ top: -top * scale, behavior: 'instant' })
  }
  settle()
}, { flush: 'post' })
onMounted(() => {
  measureGeometry()
  updateViewport()
  bindScroll()
  observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver((entries) => {
    // The spacer height is our own output, not a resize input.
    if (entries.some((entry) => entry.target === root.value
      ? Math.abs(entry.contentRect.width - width.value) > 1
      : entry.target === props.scrollRoot && props.scrollRoot.clientHeight !== viewportHeight)) resize()
    else if (entries.some((entry) => entry.target === root.value?.parentElement)) { geometryDirty = true; schedule() }
  })
  if (root.value) observer?.observe(root.value)
  if (root.value?.parentElement) observer?.observe(root.value.parentElement)
  if (props.scrollRoot) observer?.observe(props.scrollRoot)
  window.addEventListener('resize', resize, { passive: true })
  document.addEventListener('visibilitychange', activityChanged)
})
onBeforeUnmount(() => {
  clearTimeout(settleTimer)
  observer?.disconnect()
  scrollTarget?.removeEventListener('scroll', interact)
  scrollTarget?.removeEventListener('wheel', interact)
  window.removeEventListener('resize', resize)
  document.removeEventListener('visibilitychange', activityChanged)
  if (frame !== undefined) cancelAnimationFrame(frame)
})
</script>

<template>
  <div ref="root" class="gallery-timeline" :class="{ 'gallery-timeline--scrolling': moving }" :style="{ height: `${layout.height}px` }" @focusin="focusChanged" @focusout="focusChanged">
    <div
      v-for="cell in cells"
      :key="cell.key"
      class="gallery-timeline__cell"
      :style="{ top: `${cell.top}px`, left: `${cell.left}px`, width: `${cell.width}px`, height: `${cell.height}px` }"
    >
      <slot v-if="cell.item" :item="cell.item" :featured="cell.featured" />
      <slot v-else-if="cell.group" name="month" :group="cell.group" />
    </div>
    <div
      v-for="cell in placeholders"
      :key="`pending:${cell.key}`"
      class="gallery-timeline__placeholder"
      aria-hidden="true"
      :style="{ top: `${cell.top}px`, left: `${cell.left}px`, width: `${cell.width}px`, height: `${cell.height}px` }"
    />
  </div>
</template>

<style scoped>
.gallery-timeline { position: relative; min-width: 0; overflow-anchor: none; }
.gallery-timeline__cell, .gallery-timeline__placeholder { position: absolute; }
.gallery-timeline__cell :deep(.gallery-tile) { width: 100%; height: 100%; }
.gallery-timeline__placeholder { border-radius: var(--gallery-tile-radius, var(--radius-sm)); background: var(--surface-subtle); }
.gallery-timeline--scrolling :deep(.gallery-tile__media) { transition: none; }
</style>
