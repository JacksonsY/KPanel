import { describe, expect, it } from 'vitest'
import { browserCoordinates, browserEvent, browserInput } from './webBrowser'

describe('remote browser boundary', () => {
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
    for (const value of ['{', '{}', JSON.stringify({ type: 'state', active: '', tabs: Array(5).fill({ id: 'x', title: '', url: '' }) }), JSON.stringify({ type: 'frame', tab: 'x', image: '<svg>', width: 640, height: 480 })]) expect(browserEvent(value)).toBeUndefined()
    expect(browserEvent(JSON.stringify({ type: 'state', active: '', tabs: [] }))?.tabs).toEqual([])
  })
})
