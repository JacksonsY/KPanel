<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchGalleryImage, GalleryImageError, galleryIntersectionRoot } from '@/lib/galleryMedia'
import { galleryImageCacheKey, type GalleryImageLease } from '@/lib/galleryImageCache'

const props = defineProps<{ src: string; fallback?: string; eager?: boolean; version?: string }>()
const emit = defineEmits<{ load: [event: Event]; error: [] }>()
const element = ref<HTMLImageElement>()
const objectUrl = ref<string>()
let controller: AbortController | undefined
let observer: IntersectionObserver | undefined
let visible = false
let stopped = false
let fallbackUsed = false
const cache = inject(galleryImageCacheKey, undefined)
let lease: GalleryImageLease | undefined

function release(): void {
  controller?.abort()
  controller = undefined
  if (lease) { lease.release(); lease = undefined }
  else if (objectUrl.value) URL.revokeObjectURL(objectUrl.value)
  objectUrl.value = undefined
}

async function load(fallback = false): Promise<void> {
  if (!fallback) fallbackUsed = false
  release()
  const current = new AbortController()
  controller = current
  const src = fallback ? props.fallback! : props.src
  const version = cache && (props.version || new URL(src, location.href).searchParams.get('version'))
  const key = version ? `${src}\0${version}` : undefined
  const generation = cache?.generation
  try {
    const cached = key ? cache?.acquire(key) : undefined
    if (cached) { lease = cached; objectUrl.value = cached.url; return }
    const blob = await fetchGalleryImage(src, current.signal)
    if (current.signal.aborted || stopped || controller !== current) return
    lease = key ? cache?.store(key, blob, generation) : undefined
    objectUrl.value = lease?.url ?? URL.createObjectURL(blob)
  } catch (error) {
    if (current.signal.aborted || stopped || controller !== current) return
    if (!fallback && props.fallback && error instanceof GalleryImageError && [413, 422].includes(error.status)) {
      fallbackUsed = true
      void load(true)
    } else emit('error')
  }
}

function onDecodeError(): void {
  if (!visible || stopped || !objectUrl.value) return
  lease?.invalidate()
  if (props.fallback && !fallbackUsed) {
    fallbackUsed = true
    void load(true)
  } else emit('error')
}

function onLoad(event: Event): void {
  const image = event.currentTarget as HTMLImageElement
  lease?.decoded(image.naturalWidth, image.naturalHeight)
  emit('load', event)
}

watch([() => props.src, () => props.version], () => {
  release()
  fallbackUsed = false
  if (visible) void load()
})

onMounted(() => {
  if (props.eager || typeof IntersectionObserver === 'undefined') {
    visible = true
    void load()
    return
  }
  observer = new IntersectionObserver((entries) => {
    const next = entries.some((entry) => entry.isIntersecting)
    if (next === visible) return
    visible = next
    if (visible) void load()
    // Release the visible lease; recent small images remain in the bounded page cache.
    else release()
  }, { root: galleryIntersectionRoot(element.value), rootMargin: '160px 0px' })
  if (element.value) observer.observe(element.value)
})

onBeforeUnmount(() => { stopped = true; observer?.disconnect(); release() })
</script>

<template>
  <img ref="element" :src="objectUrl" decoding="async" @load="onLoad" @error="onDecodeError" />
</template>
