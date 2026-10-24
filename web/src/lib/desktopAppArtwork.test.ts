// @vitest-environment jsdom
import { readFileSync } from 'node:fs'
import { URL as FileURL } from 'node:url'
import { describe, expect, it } from 'vitest'
import { computed, nextTick } from 'vue'
import { desktopApps, findDesktopApp } from './desktopApps'
import { desktopAppIconURL } from './desktopAppArtwork'
import { resetDesktopShellStyleForTest, useDesktopShellStyle } from '@/stores/desktopShellStyle'

describe('Windows desktop artwork', () => {
  it.each(['win98', 'xp'] as const)('covers every fixed launcher with a bounded self-contained %s image', (style) => {
    for (const app of desktopApps) {
      const url = desktopAppIconURL(app, style)!
      const source = readFileSync(new FileURL(`../../public${url}`, import.meta.url), 'utf8')
      const svg = new DOMParser().parseFromString(source, 'image/svg+xml')
      expect(svg.querySelector('parsererror'), url).toBeNull()
      expect(source.length, url).toBeLessThan(30 * 1024)
      expect(source, url).not.toMatch(/<(?:script|image|foreignObject)\b|\b(?:href|onload)\s*=/i)
      const size = style === 'win98' ? '32' : '64'
      expect(svg.documentElement.getAttribute('viewBox')).toBe(`0 0 ${size} ${size}`)
      expect(svg.documentElement.getAttribute('width')).toBe(size)
      expect(svg.documentElement.getAttribute('height')).toBe(size)
    }
  })

  it('switches live and restores native artwork without changing the app catalogue', async () => {
    resetDesktopShellStyleForTest()
    const shell = useDesktopShellStyle()
    const app = desktopApps[0]!
    const original = app.desktopIconURL
    const url = computed(() => desktopAppIconURL(app, shell.preference.value))
    for (const style of ['win98', 'xp', 'modern'] as const) {
      shell.setStyle(style)
      await nextTick()
      expect(url.value).toBe(style === 'modern' ? original : `/desktop-icons/${style}/overview-v1.svg`)
      expect(app.desktopIconURL).toBe(original)
    }
    resetDesktopShellStyleForTest()
  })

  it('preserves dynamic command artwork and missing-app fallback', () => {
    expect(desktopAppIconURL(undefined, 'win98')).toBeUndefined()
    const command = findDesktopApp('/command-shortcut/' + 'a'.repeat(32))!
    expect(desktopAppIconURL(command, 'xp')).toBe(command.desktopIconURL)
  })
})
