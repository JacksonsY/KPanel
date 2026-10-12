<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { loadGalleryPoster, readGalleryPoster } from '@/lib/galleryPosters'
import { galleryIntersectionRoot } from '@/lib/galleryMedia'

const props = defineProps<{ src: string; posterKey: string; preview?: boolean; suspended?: boolean }>()
const emit = defineEmits<{ load: []; error: [] }>()
const root = ref<HTMLElement>()
const poster = computed(() => readGalleryPoster(props.posterKey)?.poster)
let observer: IntersectionObserver | undefined
let controller: AbortController | undefined
let near = false
let stopped = false

async function load(): Promise<void> {
  controller?.abort()
  if (!near || stopped || props.suspended || poster.value) return
  const current = new AbortController()
  controller = current
  try {
    for (let attempt = 0; ; attempt += 1) {
      try { await loadGalleryPoster(props.posterKey, props.src, current.signal); break }
      catch (error) {
        if (current.signal.aborted || attempt >= 2) throw error
        // Decoder/network failures are bounded and release the shared slot between attempts.
      }
    }
    if (!current.signal.aborted && !stopped) emit('load')
  } catch {
    if (!current.signal.aborted && !stopped) emit('error')
  }
}

watch(() => [props.src, props.posterKey, props.suspended], () => { void load() })
onMounted(() => {
  if (typeof IntersectionObserver === 'undefined') { near = true; void load(); return }
  observer = new IntersectionObserver((entries) => {
    near = entries.some((entry) => entry.isIntersecting)
    if (near) void load()
    else controller?.abort()
  }, { root: galleryIntersectionRoot(root.value), rootMargin: '160px 0px' })
  if (root.value) observer.observe(root.value)
})
onBeforeUnmount(() => { stopped = true; observer?.disconnect(); controller?.abort() })
</script>

<template>
  <span ref="root" class="gallery-video">
    <img v-if="poster" :src="poster" alt="" decoding="async" draggable="false" />
    <video v-if="preview && !suspended" :src="src" :poster="poster" muted loop playsinline autoplay preload="metadata" aria-hidden="true" tabindex="-1" />
  </span>
</template>

<style scoped>
.gallery-video { position: absolute; inset: 0; display: block; }
.gallery-video img, .gallery-video video { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
</style>
