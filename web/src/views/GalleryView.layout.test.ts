import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./GalleryView.vue', import.meta.url), 'utf8')

/** The standalone rule for a selector, without grouped or responsive variants. */
function rule(selector: string): string {
  const start = source.lastIndexOf(`\n${selector} {`)
  expect(start, `${selector} should exist`).toBeGreaterThan(-1)
  return source.slice(start, source.indexOf('\n}', start))
}

describe('GalleryView layout contract', () => {
  it('keeps full section heights when the desktop scroll frame is shorter than its content', () => {
    expect(rule('.gallery-scroll')).toMatch(/grid-auto-rows:\s*max-content;/)
    expect(rule('.gallery-page--windowed .gallery-scroll')).toMatch(/overflow:\s*auto;/)
  })

  it('lets the cover actions menu drop below the cover instead of being clipped by it', () => {
    // The menu hangs under the "更多操作" button, outside the cover's box, so the
    // cover itself must not clip; the photo layer rounds and clips on its own.
    expect(rule('.gallery-hero')).not.toMatch(/overflow:\s*hidden/)
    expect(rule('.gallery-hero')).toMatch(/z-index:\s*2;/)
    expect(rule('.gallery-hero__backdrop')).toMatch(/overflow:\s*hidden;/)
    expect(rule('.gallery-hero__backdrop')).toMatch(/border-radius:\s*inherit;/)
  })

  it('leaves the filter bar in place on scroll while the selection bar stays reachable', () => {
    // The filter bar used to float over the photos; it now scrolls away with the page.
    expect(rule('.gallery-toolbar,\n.gallery-selection')).not.toMatch(/position:\s*sticky/)
    // Batch actions are needed while scrolling a long selection, so that bar keeps following.
    expect(source).toMatch(/\n\.gallery-selection \{\n\s+position: sticky;/)
  })

  it('keeps the hero actions inside the cover in a narrow desktop window', () => {
    const narrowWindow = source.slice(source.lastIndexOf('@container desktop-window (max-width: 640px)'))
    expect(narrowWindow).toMatch(/\.gallery-page--windowed \.gallery-hero\s*\{[^}]*display:\s*grid;[^}]*grid-template-columns:\s*minmax\(0, 1fr\);[^}]*min-height:\s*max-content;/)
    expect(narrowWindow).not.toMatch(/\.gallery-page--windowed \.gallery-hero--photo\s*\{[^}]*min-height:\s*0;/)
    expect(narrowWindow).toMatch(/\.gallery-page--windowed \.gallery-hero__content\s*\{[^}]*width:\s*100%;/)
    expect(narrowWindow).toMatch(/\.gallery-page--windowed \.gallery-hero__actions\s*\{[^}]*width:\s*100%;[^}]*justify-content:\s*flex-start;/)
  })
})
