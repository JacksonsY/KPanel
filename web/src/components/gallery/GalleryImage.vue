<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchGalleryImage, GalleryImageError, galleryIntersectionRoot, galleryMediaPausedKey } from '@/lib/galleryMedia'

const props = defineProps<{ src: string; fallback?: string; eager?: boolean; version?: string }>()
const emit = defineEmits<{ load: [event: Event]; error: [] }>()
const paused = inject(galleryMediaPausedKey, computed(() => false))
const element = ref<HTMLImageElement>()
const source = ref<string>()
let observer: IntersectionObserver | undefined
let recovery: AbortController | undefined
let recoveryUrl: string | undefined
let visible = false
let stopped = false
let loaded = false
let recovering = false
let fallbackUsed = false
let pendingLoad: Event | undefined

function releaseRecovery(): void {
  recovery?.abort()
  recovery = undefined
  if (recoveryUrl) URL.revokeObjectURL(recoveryUrl)
  recoveryUrl = undefined
}
function start(): void {
  if (visible && !paused.value && !stopped && !source.value) source.value = fallbackUsed ? props.fallback : props.src
}
function fallback(): void {
  releaseRecovery()
  recovering = false
  if (props.fallback && !fallbackUsed) {
    fallbackUsed = true
    source.value = props.fallback
  } else emit('error')
}
async function onError(): Promise<void> {
  if (paused.value || stopped || !source.value) return
  if (fallbackUsed) { emit('error'); return }
  if (recoveryUrl) { fallback(); return }
  if (recovering) return
  // Native img errors do not expose HTTP status. Only this exceptional path probes
  // through the bounded reader, preserving retries and never falling back on 401/403.
  recovering = true
  const current = new AbortController()
  recovery = current
  try {
    const blob = await fetchGalleryImage(props.src, current.signal)
    if (current.signal.aborted || paused.value || stopped || recovery !== current) return
    recoveryUrl = URL.createObjectURL(blob)
    source.value = recoveryUrl
  } catch (error) {
    if (current.signal.aborted || paused.value || stopped || recovery !== current) return
    if (error instanceof GalleryImageError && [413, 422].includes(error.status)) fallback()
    else emit('error')
  } finally {
    if (recovery === current) recovering = false
  }
}
function onLoad(event: Event): void {
  if (!source.value || stopped) return
  loaded = true
  if (paused.value) pendingLoad = event
  else emit('load', event)
}
watch([() => props.src, () => props.version], () => {
  releaseRecovery()
  source.value = undefined
  loaded = false
  recovering = false
  fallbackUsed = false
  pendingLoad = undefined
  start()
})
watch(paused, (value) => {
  if (value) {
    // Keep already displayed pixels; cancel only work that has not produced a photo.
    if (!loaded) { releaseRecovery(); recovering = false; source.value = undefined }
  } else {
    if (pendingLoad) { emit('load', pendingLoad); pendingLoad = undefined }
    start()
  }
}, { flush: 'sync' })
onMounted(() => {
  if (props.eager || typeof IntersectionObserver === 'undefined') { visible = true; start(); return }
  observer = new IntersectionObserver((entries) => {
    if (!entries.some((entry) => entry.isIntersecting)) return
    visible = true
    observer?.disconnect()
    observer = undefined
    start()
  }, { root: galleryIntersectionRoot(element.value), rootMargin: '160px 0px' })
  if (element.value) observer.observe(element.value)
})
onBeforeUnmount(() => { stopped = true; observer?.disconnect(); releaseRecovery() })
</script>

<template>
  <img ref="element" :src="source" decoding="async" @load="onLoad" @error="onError" />
</template>
