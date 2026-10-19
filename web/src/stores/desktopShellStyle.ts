import { computed, ref } from 'vue'
import { useDesktopWallpaper } from '@/lib/desktopWallpapers'

export type DesktopShellStyle = 'modern' | 'xp' | 'win98'

const STORAGE_KEY = 'kpanel:desktop-shell-style:v1'
const preference = ref<DesktopShellStyle>('modern')
const desktopModeActive = ref(false)
let storageListenerAttached = false

function isDesktopShellStyle(value: string | null): value is DesktopShellStyle {
  return value === 'modern' || value === 'xp' || value === 'win98'
}

function readPreference(): string | null {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

function persistPreference(value: DesktopShellStyle): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, value)
  } catch {
    // The selected appearance remains usable in this tab when storage is unavailable.
  }
}

function handleStorageChange(event: StorageEvent): void {
  if (event.key === null) {
    preference.value = 'modern'
  } else if (event.key === STORAGE_KEY) {
    preference.value = isDesktopShellStyle(event.newValue) ? event.newValue : 'modern'
  }
}

export function initializeDesktopShellStyle(): void {
  const stored = readPreference()
  preference.value = isDesktopShellStyle(stored) ? stored : 'modern'

  if (typeof window !== 'undefined' && !storageListenerAttached) {
    window.addEventListener('storage', handleStorageChange)
    storageListenerAttached = true
  }
}

export function resetDesktopShellStyleForTest(): void {
  if (typeof window !== 'undefined' && storageListenerAttached) {
    window.removeEventListener('storage', handleStorageChange)
  }
  storageListenerAttached = false
  preference.value = 'modern'
  desktopModeActive.value = false
}

export function useDesktopShellStyle() {
  function setDesktopModeActive(active: boolean): void {
    desktopModeActive.value = active
  }

  function setStyle(value: DesktopShellStyle): void {
    const enteringWindowsStyle = preference.value !== value && value !== 'modern'
    preference.value = value
    persistPreference(value)
    if (enteringWindowsStyle) useDesktopWallpaper().select(value === 'win98' ? 'win98-teal' : value)
  }

  return {
    preference: computed(() => preference.value),
    desktopModeActive: computed(() => desktopModeActive.value),
    setDesktopModeActive,
    setStyle,
  }
}
