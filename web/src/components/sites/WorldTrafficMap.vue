<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue'
import { Globe2, LocateFixed, MousePointer2, RotateCcw, X, ZoomIn, ZoomOut } from '@lucide/vue'
import { useI18n } from '@/i18n'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import { worldCountries } from '@/assets/worldCountries'
import type { TrafficCountry } from '@/types/siteTraffic'
import CountryFlagIcon from '@/components/overview/CountryFlagIcon.vue'

const props = defineProps<{ countries: TrafficCountry[]; selectedCountry: string; mode: 'traffic' | 'security' }>()
const emit = defineEmits<{ 'update:selectedCountry': [code: string] }>()
const i18n = useI18n()
const id = useId()
const stage = ref<HTMLElement>(), svg = ref<SVGSVGElement>()
const zoom = ref(1), x = ref(0), y = ref(0)
const hovered = ref(''), focused = ref(''), dragging = ref(false)
const tooltipX = ref(12), tooltipY = ref(12)
const metrics = computed(() => new Map(props.countries.filter(row => row.code !== 'ZZ' && row.requests > 0).map(row => [row.code, row])))
const total = computed(() => props.countries.reduce((sum, row) => sum + row.requests, 0))
const maximum = computed(() => Math.max(1, ...[...metrics.value.values()].map(row => row.requests)))
const heatShades = [18, 35, 52, 69, 86] as const
const shapes = new Set<string>(worldCountries.map(country => country.code))
const inspected = computed(() => hovered.value || focused.value)
const highlighted = computed(() => inspected.value || (shapes.has(props.selectedCountry) ? props.selectedCountry : ''))
const chosen = computed(() => metrics.value.get(props.selectedCountry))
const canLocate = computed(() => !!chosen.value && shapes.has(props.selectedCountry))
const viewBox = computed(() => `${x.value} ${y.value} ${900 / zoom.value} ${363 / zoom.value}`)
const unit = computed(() => phrase(props.mode === 'traffic' ? '请求' : '事件'))
const formatter = computed(() => new Intl.NumberFormat(i18n.locale.value, { maximumFractionDigits: 1 }))
const number = (value: number) => formatter.value.format(value)
// Pointer movement only updates the tooltip position; names and SVG nodes are reused.
const names = computed(() => {
 const display = new Intl.DisplayNames([i18n.locale.value], { type: 'region' })
 return new Map([...new Set([...shapes, ...props.countries.map(row => row.code)])].map(code => {
  try { return [code, display.of(code) || code] } catch { return [code, code] }
 }))
})
const share = (value: number) => total.value ? number(value / total.value * 100) + '%' : '—'
function phrase(value: string): string { phraseCatalogVersion.value; return translatePhrase(value) }
function countryName(code: string): string { return names.value.get(code) || code }
function fill(code: string): string {
 const value = metrics.value.get(code)?.requests
 if (!value) return 'var(--map-land)'
 // Equal bands of the regional peak keep low and high traffic visibly distinct.
 const level = Math.min(heatShades.length - 1, Math.max(0, Math.ceil(value / maximum.value * heatShades.length) - 1))
 const intensity = heatShades[level]
 return `color-mix(in srgb, var(--monitor-accent) ${intensity}%, var(--surface))`
}
function clampCamera(): void {
 x.value = Math.max(0, Math.min(900 - 900 / zoom.value, x.value))
 y.value = Math.max(0, Math.min(363 - 363 / zoom.value, y.value))
}
function changeZoom(value: number): void {
 stopPan()
 const centerX = x.value + 450 / zoom.value, centerY = y.value + 181.5 / zoom.value
 zoom.value = Math.max(1, Math.min(4, value))
 x.value = centerX - 450 / zoom.value; y.value = centerY - 181.5 / zoom.value
 clampCamera(); hovered.value = ''; focused.value = ''
}
function reset(): void { stopPan(); zoom.value = 1; x.value = 0; y.value = 0; hovered.value = ''; focused.value = '' }
function locate(): void {
 if (!canLocate.value) return
 const path = [...(svg.value?.querySelectorAll<SVGPathElement>('[data-country]') || [])].find(node => node.dataset.country === props.selectedCountry)
 if (!path) return
 stopPan()
 const box = path.getBBox()
 zoom.value = Math.max(1, Math.min(4, 650 / Math.max(1, box.width), 250 / Math.max(1, box.height)))
 x.value = box.x + box.width / 2 - 450 / zoom.value
 y.value = box.y + box.height / 2 - 181.5 / zoom.value
 clampCamera(); hovered.value = ''; focused.value = ''
}
function positionTooltip(clientX: number, clientY: number): void {
 const box = stage.value?.getBoundingClientRect()
 if (!box) return
 tooltipX.value = Math.max(12, Math.min(box.width - 236, clientX - box.left + 14))
 tooltipY.value = Math.max(12, Math.min(box.height - 118, clientY - box.top + 14))
}
function inspect(code: string, event: PointerEvent): void {
 if (event.pointerType === 'touch' || dragging.value) return
 hovered.value = code; positionTooltip(event.clientX, event.clientY)
}
function focusCountry(code: string, event: FocusEvent): void {
 hovered.value = ''; focused.value = code
 const path = event.currentTarget as SVGPathElement
 const box = path.getBoundingClientRect(), viewport = stage.value?.getBoundingClientRect()
 if (zoom.value > 1 && viewport?.width && (box.right < viewport.left || box.left > viewport.right || box.bottom < viewport.top || box.top > viewport.bottom)) {
  const bounds = path.getBBox()
  x.value = bounds.x + bounds.width / 2 - 450 / zoom.value
  y.value = bounds.y + bounds.height / 2 - 181.5 / zoom.value
  clampCamera()
 }
 void nextTick(() => {
  const current = path.getBoundingClientRect()
  positionTooltip(current.left + current.width / 2, current.top + current.height / 2)
 })
}
function select(code: string): void {
 if (metrics.value.has(code)) emit('update:selectedCountry', code === props.selectedCountry ? '' : code)
}
function keyboard(event: KeyboardEvent): void {
 if (event.altKey || event.ctrlKey || event.metaKey) return
 if (event.key === '+' || event.key === '=') { event.preventDefault(); changeZoom(zoom.value + .5) }
 else if (event.key === '-') { event.preventDefault(); changeZoom(zoom.value - .5) }
 else if (event.key === '0') { event.preventDefault(); reset() }
 else if (event.key === 'Escape') { event.preventDefault(); hovered.value = ''; focused.value = ''; emit('update:selectedCountry', '') }
 else if (zoom.value > 1 && ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) {
  event.preventDefault()
  x.value += (event.key === 'ArrowLeft' ? -1 : event.key === 'ArrowRight' ? 1 : 0) * 90 / zoom.value
  y.value += (event.key === 'ArrowUp' ? -1 : event.key === 'ArrowDown' ? 1 : 0) * 36.3 / zoom.value
  clampCamera(); hovered.value = ''; focused.value = ''
 }
}
let pointer: { id: number; clientX: number; clientY: number; x: number; y: number; inverse: DOMMatrix } | undefined
let moved = false
function stopPan(): void {
 const active = pointer?.id
 pointer = undefined; dragging.value = false
 if (active !== undefined && svg.value?.hasPointerCapture(active)) svg.value.releasePointerCapture(active)
}
function startPan(event: PointerEvent): void {
 moved = false
 if (zoom.value <= 1 || !event.isPrimary || event.button !== 0) return
 const inverse = svg.value?.getScreenCTM()?.inverse()
 if (inverse) pointer = { id: event.pointerId, clientX: event.clientX, clientY: event.clientY, x: x.value, y: y.value, inverse }
}
function pan(event: PointerEvent): void {
 if (!pointer || pointer.id !== event.pointerId) return
 const dx = event.clientX - pointer.clientX, dy = event.clientY - pointer.clientY
 if (!moved && Math.hypot(dx, dy) < 5) return
 moved = true; dragging.value = true; hovered.value = ''; focused.value = ''
 svg.value?.setPointerCapture(event.pointerId)
 x.value = pointer.x - (dx * pointer.inverse.a + dy * pointer.inverse.c)
 y.value = pointer.y - (dx * pointer.inverse.b + dy * pointer.inverse.d)
 clampCamera()
}
function endPan(event: PointerEvent): void {
 if (pointer?.id !== event.pointerId) return
 stopPan()
}
function captureClick(event: MouseEvent): void { if (moved) { event.preventDefault(); event.stopPropagation(); moved = false } }
watch(() => props.mode, reset)
watch(metrics, value => {
 if (props.selectedCountry && !value.has(props.selectedCountry)) emit('update:selectedCountry', '')
 if (hovered.value && !shapes.has(hovered.value)) hovered.value = ''
})
</script>

<template>
 <div class="world-map" :class="{ 'is-zoomed': zoom > 1, 'is-dragging': dragging, 'is-inspecting': !!highlighted }">
  <div class="map-toolbar">
   <span class="map-instruction"><MousePointer2 :size="15" aria-hidden="true" />{{ phrase('选择地区查看详情') }}</span>
   <div class="map-controls" role="group" :aria-label="phrase('地图操作')">
    <button type="button" :disabled="zoom <= 1" :aria-label="phrase('缩小地图')" :title="phrase('缩小地图')" @click="changeZoom(zoom - .5)"><ZoomOut :size="17" /></button>
    <span class="map-zoom" data-i18n-ignore>{{ Math.round(zoom * 100) }}%</span>
    <button type="button" :disabled="zoom >= 4" :aria-label="phrase('放大地图')" :title="phrase('放大地图')" @click="changeZoom(zoom + .5)"><ZoomIn :size="17" /></button>
    <button type="button" :disabled="zoom === 1" :aria-label="phrase('恢复全球视角')" :title="phrase('恢复全球视角')" @click="reset"><RotateCcw :size="16" /></button>
   </div>
  </div>
  <div ref="stage" class="map-stage" @pointerleave="hovered = ''">
   <svg ref="svg" :viewBox="viewBox" role="group" tabindex="0" :aria-label="phrase('世界来源分布地图')" :aria-describedby="id + '-help'" @keydown="keyboard" @pointerdown="startPan" @pointermove="pan" @pointerup="endPan" @pointercancel="endPan" @lostpointercapture="endPan" @click.capture="captureClick">
    <g class="map-graticule" aria-hidden="true"><path v-for="column in [150, 300, 450, 600, 750]" :key="'x'+column" :d="`M${column},0 V363`" /><path v-for="row in [60, 120, 180, 240, 300]" :key="'y'+row" :d="`M0,${row} H900`" /></g>
    <g class="map-countries"><path v-for="country in worldCountries" :key="country.code" v-memo="[metrics.get(country.code)?.requests, maximum, selectedCountry === country.code, highlighted === country.code, !!highlighted, inspected === country.code, i18n.locale.value, phraseCatalogVersion, mode]" :data-country="country.code" :d="country.path" :fill="fill(country.code)" :class="{ 'has-data': metrics.has(country.code), selected: selectedCountry === country.code, inspected: highlighted === country.code, subdued: highlighted && highlighted !== country.code }" :tabindex="metrics.has(country.code) ? 0 : -1" :role="metrics.has(country.code) ? 'button' : undefined" :aria-pressed="metrics.has(country.code) ? selectedCountry === country.code : undefined" :aria-label="countryName(country.code) + ': ' + number(metrics.get(country.code)?.requests || 0) + ' ' + unit" :aria-describedby="inspected === country.code ? id + '-tooltip' : undefined" @pointerenter="inspect(country.code, $event)" @pointermove="inspect(country.code, $event)" @pointerleave="hovered = ''" @focus="focusCountry(country.code, $event)" @blur="focused = ''" @click="select(country.code)" @keydown.enter.prevent="select(country.code)" @keydown.space.prevent="select(country.code)" /></g>
   </svg>
   <div v-if="inspected && !dragging" :id="id + '-tooltip'" class="map-tooltip" role="tooltip" :style="{ left: tooltipX + 'px', top: tooltipY + 'px' }">
    <div><CountryFlagIcon :country-code="inspected" :label="countryName(inspected)" aria-hidden="true" /><strong>{{ countryName(inspected) }}</strong></div>
    <p v-if="metrics.has(inspected)"><strong>{{ number(metrics.get(inspected)!.requests) }}</strong> {{ unit }}<span>{{ share(metrics.get(inspected)!.requests) }}</span></p><p v-else>{{ phrase('暂无地区数据') }}</p>
   </div>
  </div>
  <p :id="id + '-help'" class="map-help"><span>{{ phrase('放大后可拖动地图') }}</span><span>{{ phrase('键盘：加减缩放，方向键平移，0 复位') }}</span></p>
  <div v-if="chosen" class="country-detail" aria-live="polite">
   <div class="country-identity"><CountryFlagIcon :country-code="chosen.code" :label="countryName(chosen.code)" aria-hidden="true" /><div><strong>{{ countryName(chosen.code) }}</strong><span>{{ phrase('已选地区') }} · {{ chosen.code }}</span></div></div>
   <div class="country-metric"><strong>{{ number(chosen.requests) }}</strong><span>{{ unit }} · {{ share(chosen.requests) }} {{ phrase('占比') }}</span></div>
   <div class="country-actions"><button type="button" :disabled="!canLocate" :aria-label="phrase('定位所选地区')" :title="phrase('定位所选地区')" @click="locate"><LocateFixed :size="17" /></button><button type="button" :aria-label="phrase('取消选择')" :title="phrase('取消选择')" @click="emit('update:selectedCountry', '')"><X :size="17" /></button></div>
   <p v-if="!canLocate" class="boundary-note">{{ phrase('此地图未收录该地区边界，统计仍已计入。') }}</p>
  </div>
  <div v-else class="map-selection-hint"><Globe2 :size="17" aria-hidden="true" /><span>{{ phrase('点击地图或右侧排行，查看地区请求与占比。') }}</span></div>
  <div class="map-legend"><span class="map-no-data"><i aria-hidden="true" />{{ phrase('无数据') }}</span><span class="map-heat-scale"><span>{{ phrase('较少') }}</span><i v-for="(shade, index) in heatShades" :key="shade" :style="{ background: `color-mix(in srgb, var(--monitor-accent) ${shade}%, var(--surface))` }" :title="`${index * 20}–${(index + 1) * 20}% · ${phrase('相对当前最高地区')}`" /><span>{{ phrase('更多') }}</span></span><span class="map-scale-note">{{ phrase('相对当前最高地区') }}</span></div>
 </div>
</template>

<style scoped>
.world-map{--map-land:color-mix(in srgb,var(--text-soft) 10%,var(--surface));display:grid;gap:12px;padding:4px 20px 12px;min-width:0}
.map-toolbar{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap}.map-instruction{display:flex;align-items:center;gap:7px;font-size:13px;color:var(--muted)}.map-controls{display:flex;align-items:center;gap:2px;padding:3px;border:1px solid var(--border);border-radius:var(--radius-sm);background:var(--surface)}
button{display:grid;place-items:center;flex-shrink:0;min-width:32px;min-height:32px;padding:6px;border:0;border-radius:var(--radius-sm);background:transparent;color:var(--text-soft);cursor:pointer;transition:background var(--motion-duration-fast) var(--motion-ease-fade)}button:hover:not(:disabled){background:var(--brand-soft);color:var(--brand)}button:disabled{color:var(--muted);opacity:.5;cursor:default}button:focus-visible,svg:focus-visible{outline:2px solid var(--brand);outline-offset:2px}.map-zoom{min-width:42px;text-align:center;font-size:12px;font-variant-numeric:tabular-nums;color:var(--text-soft)}
.map-stage{position:relative;min-width:0;overflow:hidden;border:1px solid var(--border);border-radius:var(--radius);background:color-mix(in srgb,var(--surface-subtle) 55%,var(--surface))}.map-stage>svg{display:block;width:100%;height:auto;aspect-ratio:900/420;min-height:220px;touch-action:pan-y pinch-zoom;outline-offset:-3px}.is-zoomed .map-stage>svg{cursor:grab;touch-action:none}.is-dragging .map-stage>svg{cursor:grabbing}.map-graticule{pointer-events:none}.map-graticule path{stroke:var(--border);stroke-dasharray:2 6;stroke-width:.6;vector-effect:non-scaling-stroke;fill:none}
.map-countries>path{stroke:var(--surface);stroke-width:.7;vector-effect:non-scaling-stroke;outline:none;transition:fill var(--motion-duration-fast) var(--motion-ease-fade),opacity var(--motion-duration-fast) var(--motion-ease-fade)}.map-countries>path.has-data{cursor:pointer}.is-zoomed .map-countries>path.has-data{cursor:grab}.is-dragging .map-countries>path{cursor:grabbing}.map-countries>path.subdued{opacity:.48}.map-countries>path.inspected,.map-countries>path.selected,.map-countries>path:focus-visible{opacity:1;stroke:var(--monitor-accent);stroke-width:1.8}.map-countries>path:focus-visible{stroke:var(--text);stroke-width:2.3}
.map-tooltip{position:absolute;z-index:2;width:224px;max-width:calc(100% - 24px);padding:12px;border:1px solid var(--border-strong);border-radius:var(--radius);background:var(--surface-raised);box-shadow:var(--shadow-md);pointer-events:none;overflow-wrap:anywhere}.map-tooltip>div{display:flex;align-items:center;gap:8px}.map-tooltip strong{font-size:14px;font-weight:600}.map-tooltip p{display:flex;align-items:baseline;gap:6px;flex-wrap:wrap;margin:9px 0 0;font-size:13px;color:var(--muted)}.map-tooltip p>strong{color:var(--text)}.map-tooltip p>span{margin-left:auto;color:var(--text-soft);font-variant-numeric:tabular-nums}.map-tooltip :deep(.country-flag){width:22px;height:22px;flex-shrink:0}
.map-help{display:flex;justify-content:space-between;gap:8px 16px;flex-wrap:wrap;margin:0;font-size:12px;line-height:1.6;color:var(--muted)}.country-detail{display:flex;align-items:center;gap:14px;flex-wrap:wrap;padding:14px;border:1px solid var(--border);border-radius:var(--radius);background:var(--surface-raised);border-left:3px solid var(--monitor-accent)}.country-identity{display:flex;align-items:center;gap:10px;flex:1;min-width:120px}.country-identity>div,.country-metric{display:grid;gap:4px}.country-identity strong{font-size:14px;font-weight:600;overflow-wrap:anywhere}.country-identity span,.country-metric span{font-size:13px;color:var(--muted)}.country-identity :deep(.country-flag){width:32px;height:32px;flex-shrink:0}.country-metric strong{font-size:22px;font-weight:600;line-height:1.3;font-variant-numeric:tabular-nums;overflow-wrap:anywhere}.country-actions{display:flex;gap:4px}.boundary-note{flex-basis:100%;margin:0;font-size:13px;line-height:1.6;color:var(--muted)}.map-selection-hint{display:flex;align-items:center;gap:8px;min-height:70px;padding:12px;color:var(--muted);font-size:13px;line-height:1.6}.map-selection-hint>svg{flex-shrink:0;color:var(--monitor-accent)}
.map-legend{display:flex;align-items:center;justify-content:center;gap:10px 18px;flex-wrap:wrap;padding:4px 0;font-size:12px;line-height:1.5;color:var(--muted)}.map-no-data,.map-heat-scale{display:flex;align-items:center;gap:5px}.map-legend i{display:block;width:23px;height:9px;border:1px solid var(--border);border-radius:var(--radius-sm)}.map-no-data i{background:var(--map-land)}.map-scale-note{flex-basis:100%;text-align:center}
@media(max-width:760px){.world-map{padding:4px 12px 12px}.map-toolbar{gap:8px}.map-instruction{flex:1;min-width:110px}.map-controls button{min-width:36px;min-height:36px}.map-stage>svg{min-height:220px}.map-help>span:last-child{flex-basis:100%}.country-detail{gap:12px}.country-metric{margin-left:auto}.country-actions{margin-left:auto}.country-metric strong{font-size:20px}}
@media(prefers-reduced-motion:reduce){button,.map-countries>path{transition:none}}
</style>
