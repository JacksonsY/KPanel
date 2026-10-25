// @vitest-environment jsdom
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import WorldTrafficMap from './WorldTrafficMap.vue'
import type { TrafficCountry } from '@/types/siteTraffic'

const country = (code: string, requests: number): TrafficCountry => ({ code, requests, bytes: 0, errors: 0, uniqueIPs: 0 })
const countries = [country('US', 100), country('DE', 50), country('SG', 10), country('ZZ', 40)]
let wrapper: VueWrapper | undefined
function setup(selectedCountry = '') {
 wrapper = mount(WorldTrafficMap, { props: { countries, selectedCountry, mode: 'traffic' }, global: { stubs: { CountryFlagIcon: true } } })
 return wrapper
}
function camera(view: VueWrapper) { return view.get('.map-stage>svg').attributes('viewBox')!.split(' ').map(Number) }
async function pointer(target: Element, type: string, init: PointerEventInit = {}) {
 target.dispatchEvent(new PointerEvent(type, { bubbles: true, ...init }))
 await nextTick()
}
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks() })

describe('world traffic map interactions', () => {
 it('shows attributed counts on hover without selecting or changing scope', async () => {
  const view = setup()
  await pointer(view.get('[data-country="US"]').element, 'pointerenter', { pointerType: 'mouse', clientX: 150, clientY: 80 })
  expect(view.get('[role="tooltip"]').text()).toContain('100')
  expect(view.get('[role="tooltip"]').text()).toContain('50%')
  expect(view.emitted('update:selectedCountry')).toBeUndefined()
  await view.setProps({ countries: [country('US', 80), country('DE', 80)] })
  expect(view.get('[role="tooltip"]').text()).toContain('80')
  expect(view.get('[data-country="US"]').attributes('aria-label')).toContain('80')
  await pointer(view.get('[data-country="CA"]').element, 'pointerenter', { pointerType: 'mouse' })
  expect(view.get('[role="tooltip"]').text()).toContain('暂无地区数据')
  expect(view.get('[data-country="CA"]').attributes('tabindex')).toBe('-1')
 })
 it('supports keyboard selection, pressed state and deselection with Escape', async () => {
  const view = setup(), path = view.get('[data-country="US"]')
  await path.trigger('keydown', { key: 'Enter' })
  expect(view.emitted('update:selectedCountry')?.at(-1)).toEqual(['US'])
  await view.setProps({ selectedCountry: 'US' })
  expect(path.attributes('aria-pressed')).toBe('true')
  expect(view.get('.country-detail').text()).toContain('50%')
  await path.trigger('focus')
  expect(view.find('[role="tooltip"]').exists()).toBe(true)
  await path.trigger('keydown', { key: 'Escape' })
  expect(view.emitted('update:selectedCountry')?.at(-1)).toEqual([''])
  expect(view.find('[role="tooltip"]').exists()).toBe(false)
 })
 it('bounds zoom and keyboard panning, resets the camera, and resets after mode changes', async () => {
  const view = setup(), map = view.get('.map-stage>svg')
  for (let i = 0; i < 12; i++) await map.trigger('keydown', { key: '+' })
  expect(view.get('.map-zoom').text()).toBe('400%')
  for (let i = 0; i < 30; i++) await map.trigger('keydown', { key: 'ArrowRight' })
  const box = camera(view)
  expect(box[0]! + box[2]!).toBeLessThanOrEqual(900)
  await map.trigger('keydown', { key: '0' })
  expect(camera(view)).toEqual([0, 0, 900, 363])
  await map.trigger('keydown', { key: '+' })
  await view.setProps({ mode: 'security' })
  expect(view.get('.map-zoom').text()).toBe('100%')
 })
 it('pans a zoomed map without turning a completed drag into a country click', async () => {
  const view = setup(), map = view.get('.map-stage>svg'), element = map.element as SVGSVGElement
  Object.assign(element, { getScreenCTM: () => ({ inverse: () => ({ a: 1, b: 0, c: 0, d: 1 }) }), setPointerCapture: vi.fn(), hasPointerCapture: () => true, releasePointerCapture: vi.fn() })
  await map.trigger('keydown', { key: '+' })
  const before = camera(view)[0]!
  await pointer(element, 'pointerdown', { pointerId: 1, isPrimary: true, button: 0, clientX: 100, clientY: 100 })
  await pointer(element, 'pointermove', { pointerId: 1, clientX: 130, clientY: 100 })
  await pointer(element, 'pointerup', { pointerId: 1 })
  expect(camera(view)[0]).toBe(before - 30)
  await view.get('[data-country="US"]').trigger('click')
  expect(view.emitted('update:selectedCountry')).toBeUndefined()
  await view.get('[data-country="US"]').trigger('click')
  expect(view.emitted('update:selectedCountry')?.at(-1)).toEqual(['US'])
 })
 it('keeps small-region statistics when a boundary is absent and removes obsolete selection', async () => {
  const view = setup('SG')
  expect(view.get('.country-detail').text()).toContain('10')
  expect(view.get('.boundary-note').text()).toContain('统计仍已计入')
  expect(view.get('.country-actions button').attributes('disabled')).toBeDefined()
  await view.setProps({ countries: [] })
  expect(view.find('.country-detail').exists()).toBe(false)
  expect(view.emitted('update:selectedCountry')?.at(-1)).toEqual([''])
 })
})
