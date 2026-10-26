import { describe, expect, it } from 'vitest'
import { browserCoordinates, browserEvent, browserInput, browserTextChunks } from './webBrowser'

describe('remote browser boundary', () => {
  it('splits Chinese and emoji input using the Go protocol UTF-8 byte budget', () => {
    const value = '中'.repeat(3000) + '🙂'.repeat(1000)
    const chunks = browserTextChunks(value)
    expect(chunks.join('')).toBe(value)
    expect(chunks.length).toBeGreaterThan(1)
    expect(chunks.every(chunk => new TextEncoder().encode(chunk).length <= 8192)).toBe(true)
  })
  it('also bounds JSON escaping in complete text commands', () => {
    for (const char of ['"', '\t', '\u0001']) {
      const value = char.repeat(8192), chunks = browserTextChunks(value)
      expect(chunks.join('')).toBe(value)
      expect(chunks.every(text => new TextEncoder().encode(JSON.stringify({ type: 'text', text })).length <= 16 * 1024)).toBe(true)
    }
  })
  it('accepts web addresses and searches, rejecting credentials and native schemes', () => {
    expect(browserInput('example.com/docs')).toBe('https://example.com/docs')
    expect(browserInput('http://192.168.1.5:8080')).toBe('http://192.168.1.5:8080/')
    expect(browserInput('服务器监控')).toContain('bing.com/search?q=')
    for (const value of ['javascript:alert(1)', 'file:///etc/passwd', 'https://name:password@example.com', '', 'x'.repeat(4097)]) expect(browserInput(value)).toBeUndefined()
  })
  it('maps scaled display coordinates to the actual remote viewport', () => {
    expect(browserCoordinates(350, 200, { left: 100, top: 50, width: 500, height: 300 }, 1000, 600)).toEqual({ x: 500, y: 300 })
    expect(browserCoordinates(0, 0, { left: 100, top: 50, width: 0, height: 300 }, 1000, 600)).toBeUndefined()
  })
  it('rejects malformed and oversized worker events before rendering', () => {
    for (const value of ['{', '{}', JSON.stringify({ type: 'state', active: '', tabs: Array(9).fill({ id: 'x', title: '', url: '' }) }), JSON.stringify({ type: 'frame', tab: 'x', image: '<svg>', width: 640, height: 480 })]) expect(browserEvent(value)).toBeUndefined()
    expect(browserEvent(JSON.stringify({ type: 'state', active: '', tabs: [] }))?.tabs).toEqual([])
    expect(browserEvent(JSON.stringify({ type: 'state', active: '0', tabs: Array.from({ length: 8 }, (_, i) => ({ id: String(i), title: '', url: '' })) }))?.tabs).toHaveLength(8)
  })
})
