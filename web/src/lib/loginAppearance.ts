import type { LoginAppearance } from '@/types/api'
import { parseStoredThemeColors } from '@/theme/colors'
import { applyLoginTheme } from '@/stores/theme'

let generation = 0

export function cancelLoginWallpaper(): void {
  generation++
  document.documentElement.style.setProperty('--auth-wallpaper-image', 'none')
  delete document.documentElement.dataset.authWallpaper
}

/** The session guard calls this before displaying the login form. */
export function applyLoginAppearance(value: LoginAppearance): void {
  const run = ++generation
  if (!value || !['system', 'light', 'dark'].includes(value.theme)) return
  const colors = value.colors === null ? null : parseStoredThemeColors(JSON.stringify({ version: 1, ...value.colors }))
  if (value.colors !== null && !colors) return
  applyLoginTheme({ theme: value.theme, colors })
  const wallpaper = value.wallpaper
  const inRange = (coordinate: number) => Number.isInteger(coordinate) && coordinate >= 0 && coordinate <= 1000
  if (!wallpaper || typeof wallpaper.url !== 'string' || typeof wallpaper.bright !== 'boolean'
    || !inRange(wallpaper.focusX) || !inRange(wallpaper.focusY)
    || (!/^\/wallpapers\/kpanel-desktop(?:-(?:orbit|horizon|rift|prism))?\.webp$/.test(wallpaper.url)
      && wallpaper.url !== '/wallpapers/kpanel-desktop-xp.svg'
      && !/^\/api\/v1\/auth\/wallpaper\/[0-9a-f]{64}$/.test(wallpaper.url))) return
  const root = document.documentElement
  root.dataset.authWallpaper = 'server'
  root.style.setProperty('--auth-wallpaper-image', 'none')
  root.style.setProperty('--auth-wallpaper-position', `${wallpaper.focusX / 10}% ${wallpaper.focusY / 10}%`)
  if (wallpaper.bright) root.dataset.authWallpaperBright = 'true'
  else delete root.dataset.authWallpaperBright
  const image = new Image()
  image.fetchPriority = 'high'
  image.src = wallpaper.url
  void image.decode().then(() => {
    if (run === generation) root.style.setProperty('--auth-wallpaper-image', `url("${wallpaper.url}")`)
  }).catch(() => { /* The saved palette and login form remain usable without the bitmap. */ })
}
