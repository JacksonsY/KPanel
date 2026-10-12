<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchGalleryImage, GalleryImageError, galleryIntersectionRoot } from '@/lib/galleryMedia'

const props = defineProps<{ src: string; fallback?: string; eager?: boolean }>()
const emit = defineEmits<{ load: [event: Event]; error: [] }>()
const element = ref<HTMLImageElement>()
const objectUrl = ref<string>()
let controller: AbortController | undefined
let observer: IntersectionObserver | undefined
let visible = false
let stopped = false
let fallbackUsed = false

function release(): void {
  controller?.abort()
  controller = undefined
  if (objectUrl.value) URL.revokeObjectURL(objectUrl.value)
  objectUrl.value = undefined
}

async function load(fallback = false): Promise<void> {
  if (!fallback) fallbackUsed = false
  release()
  const current = new AbortController()
  controller = current
  try {
    const blob = await fetchGalleryImage(fallback ? props.fallback! : props.src, current.signal)
    if (current.signal.aborted || stopped || controller !== current) return
    objectUrl.value = URL.createObjectURL(blob)
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
  if (props.fallback && !fallbackUsed) {
    fallbackUsed = true
    void load(true)
  } else emit('error')
}

watch(() => props.src, () => {
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
    // Keep blobs and queued work proportional to the visible area, not scroll history.
    else release()
  }, { root: galleryIntersectionRoot(element.value), rootMargin: '160px 0px' })
  if (element.value) observer.observe(element.value)
})

onBeforeUnmount(() => { stopped = true; observer?.disconnect(); release() })
</script>

<template>
  <img ref="element" :src="objectUrl" decoding="async" @load="emit('load', $event)" @error="onDecodeError" />
</template>
