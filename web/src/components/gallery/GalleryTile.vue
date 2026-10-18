<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Check, Film, ImageOff, Play } from '@lucide/vue'
import { formatGalleryDuration, galleryFileExtension, type GalleryItem } from '@/lib/gallery'
import {
  galleryPosterKey,
  readGalleryPoster,
} from '@/lib/galleryPosters'
import GalleryImage from './GalleryImage.vue'
import GalleryVideo from './GalleryVideo.vue'

const props = defineProps<{
  item: GalleryItem
  /** Image URL for the tile, or undefined when the browser cannot render one. */
  imageUrl?: string
  /** Second choice when `imageUrl` fails, typically the original after a thumbnail error. */
  fallbackUrl?: string
  /** Inline video source for the poster frame and hover preview. */
  videoUrl?: string
  suspended?: boolean
  featured?: boolean
  selecting?: boolean
  selected?: boolean
  /** Lets the tile be dragged onto an album; the page decides what the drag carries. */
  draggable?: boolean
}>()

const emit = defineEmits<{
  open: [item: GalleryItem, element: HTMLElement]
  toggle: [item: GalleryItem, range: boolean]
  dragstart: [item: GalleryItem, event: DragEvent]
  dragend: [item: GalleryItem, event: DragEvent]
}>()

const source = ref(props.imageUrl)
const failed = ref(false)
const loaded = ref(false)
const previewing = ref(false)
const posterKey = computed(() => galleryPosterKey(props.item.entry, props.videoUrl))
let hoverTimer: number | undefined
let pointerType = ''

watch([() => props.imageUrl, () => props.videoUrl, () => props.item.entry.resourceVersion], () => {
  source.value = props.imageUrl
  failed.value = false
  loaded.value = false
})
watch(() => props.item.entry, () => {
  // An explicit directory refresh retries failed media without reloading healthy tiles.
  if (failed.value) { failed.value = false; loaded.value = false }
})

const label = computed(() => props.item.entry.name)
const openLabel = computed(() => {
  if (!props.selecting) return `查看 ${label.value}`
  return props.selected ? `取消选择 ${label.value}` : `选择 ${label.value}`
})
const toggleLabel = computed(() => props.selected ? `取消选择 ${label.value}` : `选择 ${label.value}`)
const badge = computed(() => galleryFileExtension(props.item.entry.name).toUpperCase() || 'FILE')
const cached = computed(() => readGalleryPoster(posterKey.value))
const poster = computed(() => cached.value?.poster)
const duration = computed(() => cached.value?.duration === undefined ? '' : formatGalleryDuration(cached.value.duration))
const playable = computed(() => props.item.kind === 'video' && Boolean(props.videoUrl) && !failed.value)
const showsImage = computed(() => props.item.kind === 'image' && Boolean(source.value) && !failed.value)
const showsPlaceholder = computed(() => !showsImage.value && !poster.value)

function onImageError(): void {
  failed.value = true
}

function startPreview(event: PointerEvent): void {
  if (event.pointerType !== 'mouse' || props.selecting || props.suspended || !playable.value) return
  window.clearTimeout(hoverTimer)
  hoverTimer = window.setTimeout(() => {
    previewing.value = true
  }, 380)
}

function stopPreview(): void {
  window.clearTimeout(hoverTimer)
  hoverTimer = undefined
  previewing.value = false
}

function onClick(event: MouseEvent): void {
  if (props.selecting || event.ctrlKey || event.metaKey || event.shiftKey) {
    emit('toggle', props.item, event.shiftKey)
    return
  }
  emit('open', props.item, event.currentTarget as HTMLElement)
}

function onDragStart(event: DragEvent): void {
  if (pointerType === 'touch') { event.preventDefault(); return }
  // A hover preview must not keep playing under the drag image.
  stopPreview()
  emit('dragstart', props.item, event)
}

onBeforeUnmount(() => {
  window.clearTimeout(hoverTimer)
})
</script>

<template>
  <div
    class="gallery-tile"
    :class="{
      'gallery-tile--featured': featured,
      'gallery-tile--selected': selected,
      'gallery-tile--selecting': selecting,
      'gallery-tile--loaded': loaded || Boolean(poster),
      'gallery-tile--video': item.kind === 'video',
    }"
    :draggable="draggable ? 'true' : undefined"
    @pointerenter="startPreview"
    @pointerdown="pointerType = $event.pointerType"
    @pointerleave="stopPreview"
    @dragstart="onDragStart"
    @dragend="emit('dragend', item, $event)"
  >
    <button
      class="gallery-tile__open"
      type="button"
      :aria-label="openLabel"
      :aria-pressed="selecting ? selected : undefined"
      @click="onClick"
    >
      <GalleryImage
        v-if="showsImage"
        :key="item.entry.resourceVersion"
        :version="item.entry.resourceVersion"
        eager
        class="gallery-tile__media"
        :src="source!"
        :fallback="fallbackUrl"
        alt=""
        decoding="async"
        draggable="false"
        @load="loaded = true"
        @error="onImageError"
      />
      <GalleryVideo
        v-if="playable && videoUrl"
        class="gallery-tile__media"
        :src="videoUrl"
        :poster-key="posterKey"
        :preview="previewing"
        :suspended="suspended"
        @load="loaded = true"
        @error="failed = true"
      />
      <span v-if="showsPlaceholder" class="gallery-tile__placeholder" aria-hidden="true">
        <Film v-if="item.kind === 'video'" :size="featured ? 34 : 24" />
        <ImageOff v-else :size="featured ? 34 : 24" />
        <span>{{ badge }}</span>
      </span>
      <span v-if="item.kind === 'video'" class="gallery-tile__duration">
        <Play :size="12" fill="currentColor" aria-hidden="true" />
        <span v-if="duration">{{ duration }}</span>
      </span>
    </button>
    <button
      class="gallery-tile__check"
      type="button"
      :aria-label="toggleLabel"
      :aria-pressed="selected"
      @click.stop="emit('toggle', item, $event.shiftKey)"
    >
      <Check :size="14" :stroke-width="3" />
    </button>
  </div>
</template>

<style scoped>
.gallery-tile {
  position: relative;
  min-width: 0;
  aspect-ratio: 1;
  overflow: hidden;
  border-radius: var(--gallery-tile-radius, var(--radius-sm));
  background: var(--surface-subtle);
  isolation: isolate;
}

.gallery-tile__open {
  display: block;
  width: 100%;
  height: 100%;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  cursor: zoom-in;
}

.gallery-tile--selecting .gallery-tile__open {
  cursor: pointer;
}

.gallery-tile[draggable='true'] {
  -webkit-user-drag: element;
}

.gallery-tile__open:focus-visible {
  outline: 3px solid var(--brand);
  outline-offset: -3px;
}

.gallery-tile__media {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0;
  transform: scale(1.001);
  transition: opacity .32s ease, transform .5s cubic-bezier(.2, .7, .2, 1);
}

.gallery-tile--loaded .gallery-tile__media {
  opacity: 1;
}

.gallery-tile:hover .gallery-tile__media {
  transform: scale(1.035);
}

.gallery-tile--selected .gallery-tile__media {
  transform: scale(.9);
  border-radius: var(--radius-sm);
}

.gallery-tile--selected {
  background: var(--brand-soft);
}

.gallery-tile__placeholder {
  position: absolute;
  inset: 0;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}

.gallery-tile__duration {
  position: absolute;
  z-index: 1;
  right: 8px;
  bottom: 8px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px 2px 6px;
  border-radius: 999px;
  background: rgb(8 12 18 / 62%);
  color: #fff;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  line-height: 1.5;
  pointer-events: none;
}

.gallery-tile__check {
  position: absolute;
  z-index: 2;
  top: 8px;
  left: 8px;
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: 2px solid rgb(255 255 255 / 92%);
  border-radius: 50%;
  background: rgb(8 12 18 / 28%);
  color: transparent;
  cursor: pointer;
  opacity: 0;
  transition: opacity .16s ease, background-color .16s ease;
}

.gallery-tile::before {
  position: absolute;
  z-index: 1;
  inset: 0 0 auto;
  height: 46px;
  background: linear-gradient(rgb(8 12 18 / 34%), transparent);
  content: '';
  opacity: 0;
  pointer-events: none;
  transition: opacity .16s ease;
}

.gallery-tile:hover::before,
.gallery-tile:hover .gallery-tile__check,
.gallery-tile:focus-within .gallery-tile__check,
.gallery-tile--selecting .gallery-tile__check {
  opacity: 1;
}

.gallery-tile--selected::before {
  opacity: 0;
}

.gallery-tile--selected .gallery-tile__check {
  border-color: var(--brand);
  background: var(--brand);
  color: #fff;
}

.gallery-tile__check:focus-visible {
  outline: 3px solid var(--brand);
  outline-offset: 2px;
  opacity: 1;
}

@media (hover: none) {
  .gallery-tile {
    -webkit-touch-callout: none;
    user-select: none;
  }

  .gallery-tile:hover .gallery-tile__media {
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .gallery-tile__media,
  .gallery-tile__check {
    transition: none;
  }

  .gallery-tile:hover .gallery-tile__media {
    transform: none;
  }
}
</style>
