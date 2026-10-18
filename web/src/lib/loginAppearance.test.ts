// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLoginAppearance, cancelLoginWallpaper } from './loginAppearance'
import { initializeTheme, resetThemeForTest, useTheme } from '@/stores/theme'
import { deriveThemeTokens } from '@/theme/colors'
import type { LoginAppearance } from '@/types/api'

const saved: LoginAppearance = {
  theme: 'dark', colors: { brand: '#356fc0', neutral: '#34465c', signature: '#23a6bd', signatureLinked: false },
  wallpaper: { url: `/api/v1/auth/wallpaper/${'a'.repeat(64)}`, focusX: 700, focusY: 320, bright: false },
}
const images: { src: string; resolve: () => void; reject: () => void }[] = []

describe('server appearance before the login form paints', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('style')
    document.documentElement.removeAttribute('data-auth-wallpaper')
    initializeTheme()
    images.length = 0
    vi.stubGlobal('Image', class {
      src = ''
      fetchPriority = ''
      decode() {
        return new Promise<void>((resolve, reject) => images.push({ src: this.src, resolve, reject: () => reject(new Error('offline')) }))
      }
    })
  })
  afterEach(() => { cancelLoginWallpaper(); resetThemeForTest(); vi.unstubAllGlobals() })

  it('shares the saved palette immediately and holds back the default image during decode', async () => {
    const changed = vi.fn()
    window.addEventListener('kpanel:appearance-changed', changed)
    applyLoginAppearance(saved)
    expect(useTheme().preference.value).toBe('dark')
    expect(useTheme().colors.value).toEqual(saved.colors)
    expect(document.documentElement.style.getPropertyValue('--brand')).toBe(deriveThemeTokens(saved.colors!, 'dark')['--brand'])
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(document.documentElement.style.getPropertyValue('--auth-wallpaper-image')).toBe('none')
    expect(document.documentElement.style.getPropertyValue('--auth-wallpaper-position')).toBe('70% 32%')
    expect(changed).not.toHaveBeenCalled()
    expect(localStorage.length).toBe(0)
    images[0]!.resolve()
    await Promise.resolve()
    expect(document.documentElement.style.getPropertyValue('--auth-wallpaper-image')).toBe(`url("${saved.wallpaper.url}")`)
    window.removeEventListener('kpanel:appearance-changed', changed)
  })

  it('keeps the saved palette usable when the thumbnail fails', async () => {
    applyLoginAppearance(saved)
    images[0]!.reject()
    await Promise.resolve(); await Promise.resolve()
    expect(useTheme().preference.value).toBe('dark')
    expect(document.documentElement.style.getPropertyValue('--auth-wallpaper-image')).toBe('none')
  })

  it('accepts the built-in Windows XP SVG wallpaper', async () => {
    const wallpaper = { url: '/wallpapers/kpanel-desktop-xp.svg', focusX: 500, focusY: 500, bright: false }
    applyLoginAppearance({ ...saved, wallpaper })
    expect(images[0]?.src).toBe(wallpaper.url)
    images[0]!.resolve()
    await Promise.resolve()
    expect(document.documentElement.style.getPropertyValue('--auth-wallpaper-image')).toBe(`url("${wallpaper.url}")`)
  })

  it('does not let an old decode overwrite a newer scheme or the authenticated page', async () => {
    applyLoginAppearance(saved)
    applyLoginAppearance({ ...saved, theme: 'light', wallpaper: { ...saved.wallpaper, url: '/wallpapers/kpanel-desktop-horizon.webp', bright: true } })
    images[1]!.resolve(); await Promise.resolve()
    images[0]!.resolve(); await Promise.resolve()
    expect(document.documentElement.style.getPropertyValue('--auth-wallpaper-image')).toContain('horizon')
    expect(useTheme().preference.value).toBe('light')
    applyLoginAppearance(saved)
    cancelLoginWallpaper()
    images[2]!.resolve(); await Promise.resolve()
    expect(document.documentElement.style.getPropertyValue('--auth-wallpaper-image')).toBe('none')
  })

  it.each(['https://external.test/image.webp', '/api/v1/desktop/wallpapers/private/image', 'data:image/svg+xml,attack', '/api/v1/auth/wallpaper/../file'])('rejects an unexpected wallpaper URL: %s', (url) => {
    applyLoginAppearance({ ...saved, wallpaper: { ...saved.wallpaper, url } })
    expect(images).toHaveLength(0)
  })
})
