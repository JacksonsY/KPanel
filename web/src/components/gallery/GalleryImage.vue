<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchGalleryImage, GalleryImageError, galleryIntersectionRoot, galleryMediaPausedKey } from '@/lib/galleryMedia'
import { galleryRenderPolicyKey } from '@/lib/galleryRenderPolicy'

const props = defineProps<{ src: string; fallback?: string; eager?: boolean; version?: string; priority?: boolean }>()
const emit = defineEmits<{ load: [event: Event]; error: [] }>()
const paused = inject(galleryMediaPausedKey, computed(() => false))
const policy = inject(galleryRenderPolicyKey, undefined)
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
let cancelWork: (() => void) | undefined
let pendingWork: (() => void) | undefined
let pendingNative = false
let releaseImage: (() => void) | undefined

function releaseRequest(): void { releaseImage?.(); releaseImage = undefined }
function schedule(work: () => void, native = false): void {
  cancelWork?.()
  pendingWork = work
  pendingNative = native
  const run = () => {
    cancelWork = undefined
    pendingWork = undefined
    if (!paused.value && !stopped) work()
  }
  if (policy && native) {
    releaseRequest()
    releaseImage = policy.requestImage(run, props.priority)
    cancelWork = releaseImage
  } else if (policy) cancelWork = policy.enqueue(run, props.priority)
  else run()
}

function releaseRecovery(): void {
  recovery?.abort()
  recovery = undefined
  if (recoveryUrl) URL.revokeObjectURL(recoveryUrl)
  recoveryUrl = undefined
}
function start(): void {
  if (visible && !paused.value && !stopped && !source.value) {
    schedule(() => { source.value = fallbackUsed ? props.fallback : props.src }, true)
  }
}
function fallback(): void {
  releaseRecovery()
  recovering = false
  if (props.fallback && !fallbackUsed) {
    fallbackUsed = true
    schedule(() => { source.value = props.fallback }, true)
  } else emit('error')
}
async function recover(): Promise<void> {
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
    schedule(() => { source.value = recoveryUrl })
  } catch (error) {
    if (current.signal.aborted || paused.value || stopped || recovery !== current) return
    if (error instanceof GalleryImageError && [413, 422].includes(error.status)) fallback()
    else emit('error')
  } finally {
    if (recovery === current) recovering = false
  }
}
function onError(): void {
  if (!source.value || stopped || paused.value) return
  releaseRequest()
  schedule(() => { void recover() })
}
function notifyLoad(): void {
  // Decoded pixels must never wait behind admission of new requests.
  if (pendingLoad) {
    emit('load', pendingLoad)
    pendingLoad = undefined
  }
}
function onLoad(event: Event): void {
  if (!source.value || stopped) return
  releaseRequest()
  loaded = true
  pendingLoad = event
  if (!paused.value) notifyLoad()
}
watch([() => props.src, () => props.version], () => {
  releaseRecovery()
  releaseRequest()
  cancelWork?.()
  pendingWork = undefined
  source.value = undefined
  loaded = false
  recovering = false
  fallbackUsed = false
  pendingLoad = undefined
  start()
})
watch(() => props.priority, () => {
  if (!props.priority && releaseImage && source.value && !loaded) {
    // Release an unfinished request that has left the viewport. Retain decoded
    // photos; stale offscreen transfers must not occupy all foreground slots.
    releaseRequest()
    source.value = undefined
    start()
  } else if (pendingWork) schedule(pendingWork, pendingNative)
})
watch(paused, (value) => {
  if (value) {
    // Only hard inactivity cancels work; fast scrolling is handled by admission.
    cancelWork?.()
    releaseRequest()
    if (!loaded) { releaseRecovery(); recovering = false; source.value = undefined }
  } else {
    notifyLoad()
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
onBeforeUnmount(() => { stopped = true; cancelWork?.(); releaseRequest(); observer?.disconnect(); releaseRecovery() })
</script>

<template>
  <img ref="element" :src="source" decoding="async" @load="onLoad" @error="onError" />
</template>
