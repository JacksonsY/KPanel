import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { normalizeClassicWallpaperLevel } from './classicWallpaper'

describe('classic wallpaper level', () => {
  it('accepts only the known levels and defaults to off', () => {
    expect(normalizeClassicWallpaperLevel('ambient')).toBe('ambient')
    expect(normalizeClassicWallpaperLevel('clear')).toBe('clear')
    expect(normalizeClassicWallpaperLevel('off')).toBe('off')
    expect(normalizeClassicWallpaperLevel(null)).toBe('off')
    expect(normalizeClassicWallpaperLevel('glass')).toBe('off')
  })

  it('matches the classic topbar fill to the sidebar when reducing transparency', () => {
    const css = readFileSync(resolve(__dirname, '../styles/classicWallpaper.css'), 'utf8')
    const reduced = css.match(/@media \(prefers-reduced-transparency: reduce\) \{([\s\S]*?)^\}/m)?.[1]
    expect(reduced).toBeDefined()
    expect(reduced).toContain(':root:has(.classic-backdrop) .topbar')
    expect(reduced).toContain('background: color-mix(in srgb, var(--surface) var(--classic-wallpaper-sidebar), transparent);')
    expect(reduced).toContain('backdrop-filter: none;')
    const contrast = css.match(/@media \(prefers-contrast: more\) \{([\s\S]*?)^\}/m)?.[1]
    expect(contrast).toContain('background: var(--material-chrome-fill);')
    expect(css).toMatch(/@media\s*\(forced-colors:\s*active\)/)
  })
})
