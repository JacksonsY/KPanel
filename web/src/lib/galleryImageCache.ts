import type { InjectionKey } from 'vue'

interface Entry {
  key: string
  blob: Blob
  url?: string
  at: number
  refs: number
}
export interface GalleryImageLease {
  url: string
  release: () => void
  decoded: (width: number, height: number) => void
  invalidate: () => void
}

/** Retain compressed bytes, not a library of decoded images. URLs exist only while mounted. */
export class GalleryImageCache {
  private entries = new Map<string, Entry>()
  private timer?: ReturnType<typeof setTimeout>
  private disposed = false
  private bytes = 0
  generation = 0
  constructor(private limits = { count: 2048, bytes: 24 * 1024 * 1024, decoded: 32 * 1024 * 1024, itemBytes: 512 * 1024, ttl: 10 * 60_000 }) {}

  private remove(entry: Entry): void {
    if (this.entries.get(entry.key) === entry) {
      this.entries.delete(entry.key)
      this.bytes -= entry.blob.size
    }
  }

  private prune(): void {
    clearTimeout(this.timer)
    this.timer = undefined
    for (const entry of this.entries.values()) {
      if (Date.now() - entry.at >= this.limits.ttl) this.remove(entry)
    }
    while (this.entries.size > this.limits.count || this.bytes > this.limits.bytes) {
      const idle = [...this.entries.values()].find((entry) => !entry.refs)
      // Evict active entries from retention as well; their owners still hold safe leases.
      const entry = idle ?? this.entries.values().next().value
      if (!entry) break
      this.remove(entry)
    }
    const deadline = Math.min(...[...this.entries.values()].map((entry) => entry.at + this.limits.ttl))
    if (Number.isFinite(deadline)) this.timer = setTimeout(() => this.prune(), Math.max(1, deadline - Date.now()))
  }

  private lease(entry: Entry): GalleryImageLease {
    entry.url ??= URL.createObjectURL(entry.blob)
    entry.refs += 1
    let released = false
    return {
      url: entry.url,
      release: () => {
        if (released) return
        released = true
        entry.refs -= 1
        if (!entry.refs && entry.url) {
          URL.revokeObjectURL(entry.url)
          entry.url = undefined
        }
      },
      decoded: (width, height) => {
        if (released) return
        if (width * height * 4 > this.limits.decoded && this.entries.get(entry.key) === entry) this.remove(entry)
      },
      invalidate: () => { if (this.entries.get(entry.key) === entry) this.remove(entry) },
    }
  }

  acquire(key: string): GalleryImageLease | undefined {
    const entry = this.entries.get(key)
    if (!entry || this.disposed) return undefined
    if (Date.now() - entry.at >= this.limits.ttl) { this.remove(entry); return undefined }
    this.entries.delete(key)
    this.entries.set(key, entry)
    return this.lease(entry)
  }

  store(key: string, blob: Blob, generation = this.generation): GalleryImageLease {
    if (!this.disposed && generation === this.generation) {
      const existing = this.acquire(key)
      if (existing) return existing
    }
    const entry: Entry = { key, blob, at: Date.now(), refs: 0 }
    const lease = this.lease(entry)
    if (!this.disposed && generation === this.generation && blob.size <= Math.min(this.limits.itemBytes, this.limits.bytes)) {
      this.entries.set(key, entry)
      this.bytes += blob.size
      this.prune()
    }
    return lease
  }

  clear(): void {
    this.generation += 1
    for (const entry of this.entries.values()) this.remove(entry)
    clearTimeout(this.timer)
    this.timer = undefined
  }

  dispose(): void { this.disposed = true; this.clear() }
}

export const galleryImageCacheKey: InjectionKey<GalleryImageCache> = Symbol('gallery-image-cache')
