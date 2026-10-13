import type { InjectionKey } from 'vue'

interface Entry {
  key: string
  url: string
  bytes: number
  decoded: number
  at: number
  refs: number
}
export interface GalleryImageLease {
  url: string
  release: () => void
  decoded: (width: number, height: number) => void
  invalidate: () => void
}

/** Per-gallery memory only. Active leases survive eviction; idle URLs are revoked immediately. */
export class GalleryImageCache {
  private entries = new Map<string, Entry>()
  private timer?: ReturnType<typeof setTimeout>
  private disposed = false
  generation = 0
  constructor(private limits = { count: 96, bytes: 16 * 1024 * 1024, decoded: 64 * 1024 * 1024, itemBytes: 4 * 1024 * 1024, ttl: 60_000 }) {}

  private remove(entry: Entry): void {
    if (this.entries.get(entry.key) === entry) this.entries.delete(entry.key)
    if (!entry.refs) URL.revokeObjectURL(entry.url)
  }

  private prune(): void {
    clearTimeout(this.timer)
    this.timer = undefined
    for (const entry of this.entries.values()) {
      if (Date.now() - entry.at >= this.limits.ttl) this.remove(entry)
    }
    const total = (field: 'bytes' | 'decoded') => [...this.entries.values()].reduce((sum, entry) => sum + entry[field], 0)
    while (this.entries.size > this.limits.count || total('bytes') > this.limits.bytes || total('decoded') > this.limits.decoded) {
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
    entry.refs += 1
    let released = false
    return {
      url: entry.url,
      release: () => {
        if (released) return
        released = true
        entry.refs -= 1
        if (this.entries.get(entry.key) !== entry && !entry.refs) URL.revokeObjectURL(entry.url)
        else this.prune()
      },
      decoded: (width, height) => {
        if (released) return
        entry.decoded = Math.max(0, width * height * 4)
        if (entry.decoded > this.limits.decoded && this.entries.get(entry.key) === entry) this.remove(entry)
        this.prune()
      },
      invalidate: () => { if (this.entries.get(entry.key) === entry) this.remove(entry) },
    }
  }

  acquire(key: string): GalleryImageLease | undefined {
    this.prune()
    const entry = this.entries.get(key)
    if (!entry) return undefined
    this.entries.delete(key)
    this.entries.set(key, entry)
    return this.lease(entry)
  }

  store(key: string, blob: Blob, generation = this.generation): GalleryImageLease {
    if (!this.disposed && generation === this.generation) {
      const existing = this.acquire(key)
      if (existing) return existing
    }
    const entry: Entry = { key, url: URL.createObjectURL(blob), bytes: blob.size, decoded: 0, at: Date.now(), refs: 0 }
    const lease = this.lease(entry)
    if (!this.disposed && generation === this.generation && blob.size <= Math.min(this.limits.itemBytes, this.limits.bytes)) this.entries.set(key, entry)
    this.prune()
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
