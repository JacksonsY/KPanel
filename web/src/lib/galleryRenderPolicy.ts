import { computed, onBeforeUnmount, provide, readonly, ref, watch, type InjectionKey, type Ref } from 'vue'
import { galleryMediaPausedKey } from './galleryMedia'

export type GalleryScrollPhase = 'idle' | 'slow' | 'fast'
export const GALLERY_RENDER_LIMITS = {
  sampleMs: 16, settleMs: 160, fastSpeed: 1, slowSpeed: 0.45,
  batchMs: 32, imageBatch: 4, mountBatch: 16, fastBatchMs: 80, fastImageBatch: 2,
  retainedCells: 240, overscanScreens: 2, overscanPx: 1600,
} as const

/** Per gallery surface: scroll classification and admission of new image work.
 * Native requests already in flight belong to GalleryImage, not this scheduler.
 */
export function createGalleryRenderPolicy(active: Readonly<Ref<boolean>> = ref(true)) {
  const phase = ref<GalleryScrollPhase>('idle')
  const inactive = computed(() => !active.value)
  const deferImages = computed(() => !active.value || phase.value === 'fast')
  const pauseVideos = computed(() => !active.value || phase.value !== 'idle')
  const pending = new Set<() => void>()
  const visible = new Set<() => void>()
  let batchTimer: ReturnType<typeof setTimeout> | undefined
  let settleTimer: ReturnType<typeof setTimeout> | undefined
  let lastTop = 0
  let sampleAt = 0
  let distance = 0
  let disposed = false

  function drain(): void {
    batchTimer = undefined
    if (disposed || inactive.value) return
    // Visible work always progresses; a fling defers only offscreen preloading.
    const fast = phase.value === 'fast'
    const batch = [...visible, ...(fast ? [] : pending)]
      .slice(0, fast ? GALLERY_RENDER_LIMITS.fastImageBatch : GALLERY_RENDER_LIMITS.imageBatch)
    for (const job of batch) {
      if (disposed || inactive.value) break
      if (visible.delete(job) || (!deferImages.value && pending.delete(job))) job()
    }
    scheduleBatch()
  }
  function scheduleBatch(): void {
    if (!disposed && !inactive.value && (visible.size || (!deferImages.value && pending.size)) && batchTimer === undefined) {
      batchTimer = setTimeout(drain, phase.value === 'fast' ? GALLERY_RENDER_LIMITS.fastBatchMs : GALLERY_RENDER_LIMITS.batchMs)
    }
  }
  function enqueue(job: () => void, onScreen = false): () => void {
    if (!disposed) {
      pending.delete(job); visible.delete(job)
      const queue = onScreen ? visible : pending
      queue.add(job)
      scheduleBatch()
    }
    return () => { pending.delete(job); visible.delete(job) }
  }
  function reset(top = lastTop, now = performance.now()): void {
    if (disposed) return
    clearTimeout(settleTimer)
    settleTimer = undefined
    lastTop = top
    sampleAt = now
    distance = 0
    phase.value = 'idle'
  }
  function interact(top: number, now = performance.now()): void {
    if (disposed || !active.value) return
    if (phase.value === 'idle') phase.value = 'slow'
    const delta = Math.abs(top - lastTop)
    distance += delta
    lastTop = top
    const elapsed = now - sampleAt
    // Accumulate high-frequency events over a real sampling window. Clamping each
    // individual event to 16 ms would underestimate the speed on 120 Hz devices.
    if (delta > 0 && elapsed >= GALLERY_RENDER_LIMITS.sampleMs) {
      const speed = distance / Math.min(GALLERY_RENDER_LIMITS.settleMs / 2, elapsed)
      phase.value = speed >= (phase.value === 'fast' ? GALLERY_RENDER_LIMITS.slowSpeed : GALLERY_RENDER_LIMITS.fastSpeed) ? 'fast' : 'slow'
      distance = 0
      sampleAt = now
    }
    clearTimeout(settleTimer)
    settleTimer = setTimeout(() => reset(), GALLERY_RENDER_LIMITS.settleMs)
  }
  const stopAdmission = watch([inactive, phase], () => {
    // Speed changes must not keep pushing an already scheduled batch into the future.
    if (inactive.value) { clearTimeout(batchTimer); batchTimer = undefined }
    else scheduleBatch()
  }, { flush: 'sync' })
  const stopActivity = watch(active, () => reset(), { flush: 'sync' })
  function dispose(): void {
    disposed = true
    stopAdmission()
    stopActivity()
    clearTimeout(batchTimer)
    clearTimeout(settleTimer)
    pending.clear()
    visible.clear()
  }
  return { phase: readonly(phase), inactive, deferImages, pauseVideos, interact, reset, enqueue, dispose }
}

export type GalleryRenderPolicy = ReturnType<typeof createGalleryRenderPolicy>
export const galleryRenderPolicyKey: InjectionKey<GalleryRenderPolicy> = Symbol('gallery-render-policy')

/** Page and viewer each own a surface; closing a viewer disposes only its own work. */
export function provideGalleryRenderPolicy(active: Readonly<Ref<boolean>>): GalleryRenderPolicy {
  const policy = createGalleryRenderPolicy(active)
  provide(galleryRenderPolicyKey, policy)
  // Hard inactivity may cancel pending native media. Fast scrolling only defers admission.
  provide(galleryMediaPausedKey, computed(() => !active.value))
  onBeforeUnmount(policy.dispose)
  return policy
}
