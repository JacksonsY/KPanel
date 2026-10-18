import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import { describe, expect, it, vi } from 'vitest'

const script = readFileSync(new URL('../../public/appearance-init.js', import.meta.url), 'utf8')

function boot(values: Record<string, string> = {}, pathname = '/overview', blocked = false) {
  const properties = new Map<string, string>()
  const classes = new Set<string>()
  const listeners = new Map<string, (event: { detail: { id: string } }) => void>()
  const root = { dataset: {} as Record<string, string>, style: { colorScheme: '', setProperty: (key: string, value: string) => properties.set(key, value) }, classList: { add: (name: string) => classes.add(name), remove: (name: string) => classes.delete(name) } }
  const fetch = vi.fn()
  const Image = vi.fn(function () { return { decode: () => Promise.resolve() } })
  runInNewContext(script, {
    document: { documentElement: root }, location: { pathname }, matchMedia: () => ({ matches: true }),
    localStorage: { getItem: (key: string) => { if (blocked) throw new Error('blocked'); return values[key] ?? null } },
    sessionStorage: { getItem: (key: string) => values[key] ?? null, setItem: (key: string, value: string) => { values[key] = value }, removeItem: (key: string) => { delete values[key] } }, fetch, Image,
    window: { addEventListener: (name: string, listener: (event: { detail: { id: string } }) => void) => listeners.set(name, listener) },
  })
  return { root, properties, classes, fetch, Image, cache: (id: string) => listeners.get('kpanel:cache-classic-wallpaper')?.({ detail: { id } }) }
}

describe('appearance before server bootstrap', () => {
  it('restores the last public built-in bitmap on a classic refresh without fetching or writing settings', () => {
    const result = boot({ 'kejilion-panel-desktop-mode': 'classic', 'kpanel:classic-wallpaper:v1': 'clear', 'kpanel:desktop-wallpaper:v1': 'prism', 'kpanel:desktop-wallpaper-cache:v1:prism': 'data:image/webp;base64,UklGRg==' })
    expect(result.classes.has('classic-wallpaper-boot')).toBe(true)
    expect(result.properties.get('--classic-wallpaper-image')).toBe('url("data:image/webp;base64,UklGRg==")')
    expect(result.Image).toHaveBeenCalledOnce()
    expect(result.fetch).not.toHaveBeenCalled()
  })
  it.each(['/login', '/setup', '/share/token'])('does not carry a classic refresh preview onto %s', pathname => {
    const result = boot({ 'kpanel:classic-wallpaper:v1': 'clear', 'kpanel:desktop-wallpaper:v1': 'prism', 'kpanel:desktop-wallpaper-cache:v1:prism': 'data:image/webp;base64,UklGRg==' }, pathname)
    expect(result.classes.has('classic-wallpaper-boot')).toBe(false)
    expect(result.properties.has('--classic-wallpaper-image')).toBe(false)
    expect(result.Image).not.toHaveBeenCalled()
  })
  it('ignores cache URLs and never requests private assets through the bitmap cache event', () => {
    const result = boot({ 'kpanel:classic-wallpaper:v1': 'clear', 'kpanel:desktop-wallpaper:v1': 'prism', 'kpanel:desktop-wallpaper-cache:v1:prism': 'https://example.com/x.webp' })
    expect(result.classes.has('classic-wallpaper-boot')).toBe(false)
    expect(result.properties.get('--classic-wallpaper-image')).toBe('url("/wallpapers/kpanel-desktop-prism.webp")')
    for (const id of ['pack:orbital-station', `custom:${'a'.repeat(32)}`, '../../logout']) result.cache(id)
    expect(result.fetch).not.toHaveBeenCalled()
    result.cache('orbit')
    expect(result.fetch).toHaveBeenCalledWith('/wallpapers/kpanel-desktop-orbit.webp', { cache: 'force-cache' })
  })
  it('uses the XP wallpaper for early classic-mode paint without treating SVG as a cached WebP', () => {
    const result = boot({ 'kpanel:classic-wallpaper:v1': 'clear', 'kpanel:desktop-wallpaper:v1': 'xp' })
    expect(result.properties.get('--classic-wallpaper-image')).toBe('url("/wallpapers/kpanel-desktop-xp.svg")')
    expect(result.Image).toHaveBeenCalledOnce()
    result.cache('xp')
    expect(result.fetch).toHaveBeenCalledWith('/wallpapers/kpanel-desktop-xp.svg', { cache: 'force-cache' })
  })
  it.each(['light', 'dark'])('restores %s tokens without fetching a cached or default wallpaper', (theme) => {
    const result = boot({ 'kejilion-panel-theme': theme, 'kejilion-panel-desktop-mode': 'desktop', 'kpanel:desktop-wallpaper:v1': 'prism' })
    expect(result.root.dataset.theme).toBe(theme)
    expect(result.root.style.colorScheme).toBe(theme)
    expect(result.classes.has('desktop-boot')).toBe(true)
    expect(result.properties.get('--desktop-wallpaper-image')).toBe('none')
    expect(result.fetch).not.toHaveBeenCalled()
    expect(result.Image).not.toHaveBeenCalled()
  })
  it.each(['/login', '/setup', '/share/token', '/overview'])('does not request stale/private/default images on %s', (path) => {
    const result = boot({ 'kpanel:desktop-wallpaper:v1': 'custom:' + 'a'.repeat(32), 'kpanel:auth-wallpaper:v1': '{"image":"data:image/webp;base64,UklGRg=="}' }, path)
    expect(result.properties.get('--auth-wallpaper-image')).toBe('none')
    expect(result.properties.has('--classic-wallpaper-image')).toBe(false)
    expect(result.fetch).not.toHaveBeenCalled()
    expect(result.Image).not.toHaveBeenCalled()
  })
  it.each(['/login', '/setup', '/share/token'])('does not show the desktop boot layer on %s', (path) => {
    expect(boot({ 'kejilion-panel-desktop-mode': 'desktop' }, path).classes.has('desktop-boot')).toBe(false)
  })
  it('keeps startup available when browser storage is blocked', () => {
    expect(boot({}, '/overview', true).root.dataset.theme).toBe('dark')
  })
  it('accepts matching color tokens but rejects CSS URL injection', () => {
    const cache = JSON.stringify({ theme: 'dark', colors: null, tokens: { '--bg': '#243447', '--brand-soft': 'rgb(40 103 178 / 12%)', '--desktop-wallpaper-veil-dark': 'linear-gradient(145deg, rgb(0 0 0 / 26%), rgb(0 0 0 / 48%))', '--desktop-aurora-one': 'url(https://example.com/x)' } })
    const result = boot({ 'kpanel:desktop-backdrop:v1': cache })
    expect(result.properties.get('--bg')).toBe('#243447')
    expect(result.properties.get('--brand-soft')).toBe('rgb(40 103 178 / 12%)')
    expect(result.properties.get('--desktop-wallpaper-veil-dark')).toContain('26%')
    expect(result.properties.has('--desktop-aurora-one')).toBe(false)
    expect(boot({ 'kpanel:desktop-backdrop:v1': cache, 'kejilion-panel-colors': 'changed' }).properties.has('--desktop-wallpaper-veil-dark')).toBe(false)
    expect(boot({ 'kpanel:desktop-backdrop:v1': cache, 'kejilion-panel-colors': 'changed' }).properties.has('--bg')).toBe(false)
  })
})
