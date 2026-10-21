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
const emit = defineEmits<{ scrolling: [value: boolean]; fastScrolling: [value: boolean] }>()
const SETTLE_MS = 160
const root = ref<HTMLElement>()
const width = ref(0)
const minimum = ref(168)
const gap = ref(6)
const cells = shallowRef<GalleryLayoutCell[]>([])
// Keep recent image elements and their decoded pixels alive across short backtracks.
const retained = new Map<string, GalleryLayoutCell>()
const RETAINED_CELLS = 240
const moving = ref(false)
let fast = false
let sampledTop = 0
let sampledAt = 0
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
  // Already mounted, prefetched photos remain available during a fling.
  // Do not create new media subtrees until the scroll slows down.
  if (fast) return
  if (geometryDirty) measureGeometry()
  const top = viewportTop()
  // Warm two screens ahead, with a pixel cap for large desktop windows.
  const buffer = Math.min(1600, viewportHeight * 2)
  const near = galleryViewport(layout.value.cells, top - buffer, top + viewportHeight + buffer, layout.value.maxCellHeight)
  const pinned = focused && layout.value.cells.find((cell) => cell.key === focused)
  if (pinned && !near.includes(pinned)) near.push(pinned)
  const required = new Set(near.map(cell => cell.key))
  for (const cell of near) {
    retained.delete(cell.key)
    retained.set(cell.key, cell)
  }
  for (const key of retained.keys()) {
    if (retained.size <= Math.max(RETAINED_CELLS, near.length)) break
    if (!required.has(key)) retained.delete(key)
  }
  const next = [...retained.values()].sort((a, b) => a.top - b.top || a.left - b.left)
  if (!same(cells.value, next)) cells.value = next
}
function schedule(): void {
  if (frame === undefined && enabled()) frame = requestAnimationFrame(updateViewport)
}
function setMoving(value: boolean): void {
  if (moving.value === value) return
  moving.value = value
  emit('scrolling', value)
}
function setFast(value: boolean): void {
  if (fast === value) return
  fast = value
  emit('fastScrolling', value)
}
function resetSpeed(): void {
  setFast(false)
  sampledTop = scrollPosition()
  sampledAt = performance.now()
}
function settle(): void {
  clearTimeout(settleTimer)
  settleTimer = undefined
  setMoving(false)
  resetSpeed()
  updateViewport()
}
function interact(): void {
  if (!enabled()) return
  const top = scrollPosition()
  const now = performance.now()
  if (top !== sampledTop) {
    // Clamp idle gaps so a scrollbar jump is treated like fast movement too.
    const speed = Math.abs(top - sampledTop) / Math.max(16, Math.min(80, now - sampledAt))
    // Hysteresis prevents repeated pause/resume around a single threshold.
    setFast(speed >= (fast ? 0.45 : 1))
    sampledTop = top
    sampledAt = now
  }
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
  resetSpeed()
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
  resetSpeed()
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
  resetSpeed()
  geometryDirty = true
  schedule()
}
defineExpose({ reveal })
watch(() => props.scrollRoot, (next, previous) => {
  if (previous) observer?.unobserve(previous)
  if (next) observer?.observe(next)
  bindScroll()
}, { flush: 'post' })
watch(layout, () => {
  // Reconcile geometry and remove deleted/filtered/version-changed entries.
  const current = new Map(layout.value.cells.map(cell => [cell.key, cell]))
  for (const key of retained.keys()) {
    const cell = current.get(key)
    if (cell) retained.set(key, cell)
    else retained.delete(key)
  }
  geometryDirty = true
  updateViewport()
}, { flush: 'post' })
watch(() => props.active, activityChanged, { flush: 'post' })
watch(() => props.resetKey, async () => {
  focused = ''
  retained.clear()
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
  setMoving(false)
  resetSpeed()
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
  </div>
</template>

<style scoped>
.gallery-timeline { position: relative; min-width: 0; overflow-anchor: none; }
.gallery-timeline__cell { position: absolute; }
.gallery-timeline__cell :deep(.gallery-tile) { width: 100%; height: 100%; }
.gallery-timeline--scrolling :deep(.gallery-tile__media) { transition: none; }
</style>
