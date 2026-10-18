import type { InjectionKey, Ref } from 'vue'

/** A gallery window's interaction state; the viewer overrides it for its own media. */
export const galleryMediaPausedKey: InjectionKey<Readonly<Ref<boolean>>> = Symbol('gallery-media-paused')

/** Shared across gallery windows: leave file-service capacity for the active viewer. */
export function createGalleryQueue(limit: number) {
  let active = 0
  const waiting: Array<() => void> = []
  return async function run<T>(job: () => Promise<T>, signal: AbortSignal, priority = false): Promise<T> {
    signal.throwIfAborted()
    await new Promise<void>((resolve, reject) => {
      const start = () => {
        signal.removeEventListener('abort', cancel)
        active += 1
        resolve()
      }
      const cancel = () => {
        const index = waiting.indexOf(start)
        if (index >= 0) waiting.splice(index, 1)
        reject(signal.reason)
      }
      signal.addEventListener('abort', cancel, { once: true })
      if (active < limit) start()
      else if (priority) waiting.unshift(start)
      else waiting.push(start)
    })
    try {
      signal.throwIfAborted()
      return await job()
    } finally {
      active -= 1
      waiting.shift()?.()
    }
  }
}

export const queueGalleryImage = createGalleryQueue(2)
export const queueGalleryVideo = createGalleryQueue(1)

export function galleryIntersectionRoot(element?: Element): Element | null {
  // Classic layout scrolls the document; only desktop windows own a scroll frame.
  return element?.closest('.gallery-page--windowed') ? element.closest('.gallery-scroll') : null
}

export class GalleryImageError extends Error {
  constructor(public readonly status: number) { super(`Gallery image HTTP ${status}`) }
}

const MAX_IMAGE_BYTES = 16 * 1024 * 1024
async function imageBlob(response: Response): Promise<Blob> {
  const reader = response.body?.getReader()
  if (!reader) return response.blob()
  const chunks: Uint8Array<ArrayBuffer>[] = []
  let bytes = 0
  try {
    if (Number(response.headers.get('Content-Length')) > MAX_IMAGE_BYTES) throw new GalleryImageError(413)
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      bytes += value.byteLength
      if (bytes > MAX_IMAGE_BYTES) throw new GalleryImageError(413)
      chunks.push(new Uint8Array(value))
    }
    return new Blob(chunks, { type: response.headers.get('Content-Type') ?? '' })
  } catch (error) {
    await reader.cancel().catch(() => undefined)
    throw error
  } finally { reader.releaseLock() }
}

function pause(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const cancel = () => { clearTimeout(timer); signal.removeEventListener('abort', cancel); reject(signal.reason) }
    const timer = setTimeout(() => { signal.removeEventListener('abort', cancel); resolve() }, ms)
    signal.addEventListener('abort', cancel, { once: true })
    if (signal.aborted) cancel()
  })
}

/** Retry congestion, never mistake a busy file service for an unsupported decoder. */
export async function fetchGalleryImage(url: string, signal: AbortSignal, priority = false): Promise<Blob> {
  return queueGalleryImage(async () => {
    const controller = new AbortController()
    const cancel = () => controller.abort(signal.reason)
    signal.addEventListener('abort', cancel, { once: true })
    const timer = setTimeout(() => controller.abort(new Error('Image request timed out')), 20_000)
    try {
      for (let attempt = 0; ; attempt += 1) {
        signal.throwIfAborted()
        const response = await fetch(url, { signal: controller.signal, credentials: 'same-origin' })
        if (response.ok) return await imageBlob(response)
        await response.body?.cancel()
        if ((response.status === 429 || response.status === 503) && attempt < 3) {
          await pause(250 * 2 ** attempt, controller.signal)
          continue
        }
        throw new GalleryImageError(response.status)
      }
    } finally {
      clearTimeout(timer)
      signal.removeEventListener('abort', cancel)
    }
  }, signal, priority)
}

export function galleryAllowsPrefetch(bytes: number): boolean {
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  return bytes > 0 && bytes <= 2 * 1024 * 1024 && !connection?.saveData
    && !['slow-2g', '2g', '3g'].includes(connection?.effectiveType ?? '')
}
