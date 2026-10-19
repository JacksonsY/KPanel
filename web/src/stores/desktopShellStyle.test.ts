// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  initializeDesktopShellStyle,
  resetDesktopShellStyleForTest,
  useDesktopShellStyle,
} from './desktopShellStyle'

const wallpaper = vi.hoisted(() => ({ select: vi.fn() }))
vi.mock('@/lib/desktopWallpapers', () => ({ useDesktopWallpaper: () => ({ select: wallpaper.select }) }))

const STORAGE_KEY = 'kpanel:desktop-shell-style:v1'

beforeEach(() => {
  resetDesktopShellStyleForTest()
  window.localStorage.clear()
  wallpaper.select.mockClear()
})

afterEach(() => {
  resetDesktopShellStyleForTest()
  vi.restoreAllMocks()
})

describe('desktop shell style preference', () => {
  it('defaults to the native shell and restores Windows shell choices', () => {
    initializeDesktopShellStyle()
    expect(useDesktopShellStyle().preference.value).toBe('modern')

    window.localStorage.setItem(STORAGE_KEY, 'xp')
    initializeDesktopShellStyle()

    expect(useDesktopShellStyle().preference.value).toBe('xp')

    window.localStorage.setItem(STORAGE_KEY, 'win98')
    initializeDesktopShellStyle()
    expect(useDesktopShellStyle().preference.value).toBe('win98')
  })

  it('falls back to the native shell for an unknown stored value', () => {
    window.localStorage.setItem(STORAGE_KEY, 'unexpected')

    initializeDesktopShellStyle()

    expect(useDesktopShellStyle().preference.value).toBe('modern')
  })

  it('persists selections and synchronizes changes from another tab', () => {
    initializeDesktopShellStyle()
    const style = useDesktopShellStyle()

    style.setStyle('xp')
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('xp')
    expect(style.preference.value).toBe('xp')
    expect(wallpaper.select).toHaveBeenCalledOnce()
    expect(wallpaper.select).toHaveBeenCalledWith('xp')

    window.dispatchEvent(new StorageEvent('storage', { key: STORAGE_KEY, newValue: 'modern' }))
    expect(style.preference.value).toBe('modern')

    window.dispatchEvent(new StorageEvent('storage', { key: null, newValue: null }))
    expect(style.preference.value).toBe('modern')
  })

  it('selects the matching classic wallpaper when entering Win98 and keeps it when returning to native', () => {
    initializeDesktopShellStyle()
    const style = useDesktopShellStyle()

    style.setStyle('win98')
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('win98')
    expect(style.preference.value).toBe('win98')
    expect(wallpaper.select).toHaveBeenCalledOnce()
    expect(wallpaper.select).toHaveBeenCalledWith('win98-teal')

    style.setStyle('modern')
    expect(style.preference.value).toBe('modern')
    expect(wallpaper.select).toHaveBeenCalledOnce()
  })

  it('does not reset a manually selected wallpaper when XP is already active', () => {
    initializeDesktopShellStyle()
    const style = useDesktopShellStyle()

    style.setStyle('xp')
    style.setStyle('xp')

    expect(wallpaper.select).toHaveBeenCalledOnce()
  })

  it('keeps the in-memory choice when browser storage is unavailable', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('blocked') })

    expect(() => initializeDesktopShellStyle()).not.toThrow()
    expect(() => useDesktopShellStyle().setStyle('xp')).not.toThrow()
    expect(useDesktopShellStyle().preference.value).toBe('xp')
  })

  it('does not attach duplicate storage listeners when initialized more than once', () => {
    const addEventListener = vi.spyOn(window, 'addEventListener')

    initializeDesktopShellStyle()
    initializeDesktopShellStyle()

    expect(addEventListener.mock.calls.filter(([type]) => type === 'storage')).toHaveLength(1)
  })
})
