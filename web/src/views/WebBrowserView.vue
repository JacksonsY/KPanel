<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, ArrowRight, ExternalLink, Globe2, Home, LoaderCircle, Network, Plus, RefreshCw, Search, X } from '@lucide/vue'
import { api } from '@/lib/api'
import { BROWSER_MAX_TABS, browserCoordinates, browserEvent, browserInput, browserTextChunks, browserLinksKey, browserLaunchKey, type BrowserEvent, type BrowserTab } from '@/lib/webBrowser'
import { desktopWindowTitleKey, desktopWindowTitlebarTargetKey, desktopWindowVisibleKey } from '@/lib/desktopRouteKeys'
import { useBrowserText } from '@/i18n/webBrowser'
import { usePhraseCatalog } from '@/i18n/phrase'
import ModalDialog from '@/components/common/ModalDialog.vue'

const t = useBrowserText()
usePhraseCatalog(locale => locale === 'en-US' ? import('@/i18n/pages/WebBrowserView/en-US').then(m => m.default) : import('@/i18n/pages/WebBrowserView/zh-TW').then(m => m.default))
const route = useRoute()
const windowVisible = inject(desktopWindowVisibleKey, ref(true))
const setTitle = inject(desktopWindowTitleKey, undefined)
const titlebarTarget = inject(desktopWindowTitlebarTargetKey, ref<HTMLElement>())
const links = inject(browserLinksKey, ref([]))
const launch = inject(browserLaunchKey, ref(undefined))
const tabs = ref<BrowserTab[]>([])
const activeID = ref('')
const address = ref('')
const editingAddress = ref(false)
const startPage = ref(true)
const newTabPending = ref(true)
const status = ref<'connecting' | 'connected' | 'disconnected' | 'unconfigured' | 'unavailable'>('connecting')
const notice = ref('')
const frame = ref<HTMLCanvasElement>()
const tabStrip = ref<HTMLElement>()
const screen = ref<HTMLElement>()
const addressInput = ref<HTMLInputElement>()
const typing = ref<HTMLTextAreaElement>()
const text = ref('')
const dialog = ref<BrowserEvent>()
const imageReady = ref(false)
const width = ref(1280)
const height = ref(720)
const activeTab = computed(() => tabs.value.find(tab => tab.id === activeID.value))
const workspace = Array.from(crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, '0')).join('')
const visible = computed(() => windowVisible.value && !startPage.value && documentVisible.value)
const documentVisible = ref(!document.hidden)
let socket: WebSocket | undefined
let epoch = 0
let bitmapSequence = 0
let painting = false
let latestFrame: BrowserEvent | undefined
let controller: AbortController | undefined
let observer: ResizeObserver | undefined
let resizeTimer: ReturnType<typeof setTimeout> | undefined
let connectTimer: ReturnType<typeof setTimeout> | undefined
let lastPointer = 0
const composing = ref(false)
let pendingURL: string | undefined

function send(command: Record<string, unknown>): boolean {
  if (socket?.readyState !== WebSocket.OPEN || status.value !== 'connected') return false
  if (socket.bufferedAmount > 128 * 1024) { notice.value = t('failed'); return false }
  socket.send(JSON.stringify(command)); return true
}
function errorMessage(code = ''): string {
  if (code === 'tab_limit') return t('limit')
  if (code === 'invalid_url') return t('invalid')
  if (code === 'browser_start_failed') return t('startFailed')
  if (code === 'upload_unavailable') return t('upload')
  return t('failed')
}
async function paint(event: BrowserEvent): Promise<void> {
  if (event.tab !== activeID.value || !event.image || !visible.value) return
  if (painting) { latestFrame = event; return }
  painting = true
  const current = ++bitmapSequence
  const binary = atob(event.image)
  const bytes = Uint8Array.from(binary, char => char.charCodeAt(0))
  let bitmap: ImageBitmap | undefined
  try {
    bitmap = await createImageBitmap(new Blob([bytes], { type: 'image/jpeg' }))
    if (current !== bitmapSequence || !frame.value || event.tab !== activeID.value || !visible.value) return
    width.value = event.width!; height.value = event.height!
    frame.value.width = width.value; frame.value.height = height.value
    frame.value.getContext('2d')?.drawImage(bitmap, 0, 0, width.value, height.value)
    imageReady.value = true
  } catch { if (current === bitmapSequence) notice.value = t('failed') }
  finally {
    bitmap?.close(); painting = false
    const next = latestFrame; latestFrame = undefined
    if (next) void paint(next)
  }
}
function receive(raw: unknown): void {
  const event = browserEvent(raw)
  if (!event) return
  if (event.type === 'frame') { void paint(event); return }
  if (event.type === 'error') { notice.value = errorMessage(event.code); return }
  if (event.type === 'dialog') { dialog.value = event; return }
  if (event.type !== 'state') return
  const changed = activeID.value !== event.active
  tabs.value = event.tabs!; activeID.value = event.active!
  if (tabs.value.length >= BROWSER_MAX_TABS && newTabPending.value) { newTabPending.value = false; startPage.value = false }
  if (changed) { imageReady.value = false; bitmapSequence++ }
  if (!editingAddress.value) address.value = activeTab.value?.url === 'about:blank' ? '' : activeTab.value?.url || ''
  setTitle?.(activeTab.value?.title || t('title'))
  if (pendingURL) { const target = pendingURL; pendingURL = undefined; openURL(target) }
}
function disconnect(end = false): void {
  if (end) send({ type: 'end' })
  epoch++; clearTimeout(connectTimer); controller?.abort(); socket?.close(); socket = undefined; bitmapSequence++
}
async function connect(): Promise<void> {
  disconnect(); const current = epoch; status.value = 'connecting'; notice.value = ''; controller = new AbortController()
  try {
    const health = await api.browser.status(controller.signal)
    if (current !== epoch) return
    if (!health.ready) { status.value = health.reason === 'browser_unconfigured' ? 'unconfigured' : 'unavailable'; return }
    const url = new URL(`/api/v1/browser/stream/${workspace}`, location.href); url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const channel = new WebSocket(url); socket = channel
    connectTimer = setTimeout(() => { if (current === epoch) { disconnect(); status.value = 'unavailable' } }, 15000)
    channel.onopen = () => { if (current !== epoch) return; channel.send(JSON.stringify({ csrf: api.browser.csrf() })) }
    channel.onmessage = event => {
      if (current !== epoch) return
      const parsed = browserEvent(event.data)
      if (!parsed) return
      if (status.value !== 'connected') { clearTimeout(connectTimer); status.value = 'connected'; send({ type: 'visible', visible: visible.value }); resize() }
      receive(event.data)
    }
    channel.onerror = () => { if (current === epoch) status.value = 'disconnected' }
    channel.onclose = () => { if (current === epoch) { clearTimeout(connectTimer); status.value = 'disconnected'; imageReady.value = false } }
  } catch { if (current === epoch) status.value = 'unavailable' }
}
function openURL(url: string): void {
  if (status.value !== 'connected') { address.value = url; pendingURL = url; startPage.value = false; return }
  if (newTabPending.value || !activeTab.value) {
    if (tabs.value.length >= BROWSER_MAX_TABS) { notice.value = t('limit'); return }
    send({ type: 'new', url })
  } else send({ type: 'navigate', tab: activeID.value, url })
  startPage.value = false; newTabPending.value = false; imageReady.value = false; notice.value = ''; address.value = url
  void nextTick(() => { resize(); typing.value?.focus() })
}
function submit(): void { const url = browserInput(address.value); if (!url) { notice.value = t('invalid'); return }; openURL(url) }
function select(tab: BrowserTab): void { startPage.value = false; newTabPending.value = false; imageReady.value = false; send({ type: 'select', tab: tab.id }); void nextTick(resize) }
function close(tab: BrowserTab): void { send({ type: 'close', tab: tab.id }); if (tabs.value.length === 1) { startPage.value = true; newTabPending.value = true } }
function home(): void { startPage.value = true; newTabPending.value = !activeTab.value; address.value = ''; void nextTick(() => addressInput.value?.focus()) }
function closeStartTab(): void { const previous = tabs.value[tabs.value.length - 1]; if (previous) select(previous); else { pendingURL = undefined; notice.value = ''; home() } }
function newTab(): void { if (tabs.value.length >= BROWSER_MAX_TABS) { notice.value = t('limit'); return }; home(); newTabPending.value = true }
function external(): void { const url = browserInput(address.value || activeTab.value?.url || pendingURL || ''); if (url) window.open(url, '_blank', 'noopener,noreferrer') }
function resize(): void {
  if (!screen.value || !visible.value) return
  const box = screen.value.getBoundingClientRect()
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => send({ type: 'resize', width: Math.max(320, Math.min(1600, Math.round(box.width))), height: Math.max(240, Math.min(1000, Math.round(box.height))) }), 140)
}
function pointer(event: PointerEvent, action: string): void {
  if (!imageReady.value || !frame.value || startPage.value) return
  if (action === 'mouseMoved' && performance.now() - lastPointer < 40) return
  lastPointer = performance.now()
  const point = browserCoordinates(event.clientX, event.clientY, frame.value.getBoundingClientRect(), width.value, height.value)
  if (!point) return
  if (action === 'mousePressed') { typing.value?.focus({ preventScroll: true }); frame.value.setPointerCapture?.(event.pointerId) }
  send({ type: 'mouse', action, ...point, button: ['left', 'middle', 'right'][event.button] || 'none', modifiers: modifiers(event) })
}
function wheel(event: WheelEvent): void {
  if (!frame.value || !imageReady.value) return
  const point = browserCoordinates(event.clientX, event.clientY, frame.value.getBoundingClientRect(), width.value, height.value)
  if (point) send({ type: 'mouse', action: 'mouseWheel', ...point, deltaX: Math.max(-3000, Math.min(3000, event.deltaX)), deltaY: Math.max(-3000, Math.min(3000, event.deltaY)), button: 'none' })
}
function modifiers(event: { altKey: boolean; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean }): number { return Number(event.altKey) + Number(event.ctrlKey) * 2 + Number(event.metaKey) * 4 + Number(event.shiftKey) * 8 }
function key(event: KeyboardEvent, action: string): void {
  if (composing.value || event.isComposing || event.key === 'Process') return
  if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) return
  if (event.key === 'Tab' && event.shiftKey) return
  event.preventDefault(); send({ type: 'key', action, key: event.key, code: event.code, modifiers: modifiers(event) })
}
function insertText(value: string): void { for (const text of browserTextChunks(value)) { if (!send({ type: 'text', text })) break } }
function textInput(event: Event): void { if (composing.value || (event as InputEvent).isComposing) return; const value = typing.value?.value; if (value) insertText(value); if (typing.value) typing.value.value = '' }
function insert(): void { if (text.value) { insertText(text.value); text.value = '' } }
function answer(accept: boolean): void { if (dialog.value) send({ type: 'dialog', tab: dialog.value.tab, accept }); dialog.value = undefined }
function documentVisibility(): void { documentVisible.value = !document.hidden }
watch(visible, value => send({ type: 'visible', visible: value }))
watch([activeID, startPage, newTabPending], () => void nextTick(() => tabStrip.value?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })))
watch(launch, value => { if (!value) return; const url = browserInput(value.url); if (!url) return; const tab = tabs.value.find(tab => tab.url === url); if (tab) select(tab); else { newTabPending.value = true; openURL(url) } }, { immediate: true })
watch(() => route.query.url, raw => { const value = Array.isArray(raw) ? raw[0] : raw; if (value) { const url = browserInput(value); if (url) openURL(url) } }, { immediate: true })
watch(() => route.query.request, () => { const value = route.query.url; if (typeof value === 'string') { const url = browserInput(value); if (url) openURL(url) } })
onMounted(() => { observer = new ResizeObserver(resize); if (screen.value) observer.observe(screen.value); document.addEventListener('visibilitychange', documentVisibility); void connect() })
onBeforeUnmount(() => { disconnect(true); observer?.disconnect(); clearTimeout(resizeTimer); document.removeEventListener('visibilitychange', documentVisibility) })
</script>

<template>
  <section class="web-browser" :aria-label="t('title')">
    <Teleport :to="titlebarTarget || 'body'" :disabled="!titlebarTarget">
    <div ref="tabStrip" class="web-browser__tabs" :class="{ 'web-browser__tabs--titlebar': titlebarTarget }" role="tablist" :aria-label="t('title')" @dblclick.stop>
      <div class="web-browser__tab-track">
      <div v-for="tab in tabs" :key="tab.id" class="web-browser__tab" :class="{ 'is-active': (!startPage || !newTabPending) && tab.id === activeID }">
        <button role="tab" :aria-selected="(!startPage || !newTabPending) && tab.id === activeID" :title="tab.title || t('untitled')" @click="select(tab)"><LoaderCircle v-if="tab.loading" :size="15" class="is-spinning" /><Globe2 v-else :size="15" /><span>{{ startPage && !newTabPending && tab.id === activeID ? t('untitled') : tab.title || t('untitled') }}</span></button>
        <button :aria-label="`${t('closeTab')} · ${tab.title || t('untitled')}`" @click="close(tab)"><X :size="14" /></button>
      </div>
      <div v-if="startPage && newTabPending" class="web-browser__tab is-active"><button role="tab" aria-selected="true" @click="addressInput?.focus()"><Globe2 :size="15" /><span>{{ t('untitled') }}</span></button><button :aria-label="t('closeTab')" @click="closeStartTab"><X :size="14" /></button></div>
      </div>
      <button class="web-browser__icon" :disabled="tabs.length >= BROWSER_MAX_TABS" :title="t('newTab')" :aria-label="t('newTab')" @click="newTab"><Plus :size="18" /></button>
      <span class="web-browser__tab-count">{{ tabs.length + Number(startPage && newTabPending) }}/{{ BROWSER_MAX_TABS }}</span>
    </div>
    </Teleport>
    <form class="web-browser__toolbar" @submit.prevent="submit">
      <button type="button" class="web-browser__icon" :disabled="!activeTab?.back || startPage || status !== 'connected'" :title="t('back')" :aria-label="t('back')" @click="send({ type: 'back', tab: activeID })"><ArrowLeft :size="18" /></button>
      <button type="button" class="web-browser__icon" :disabled="!activeTab?.forward || startPage || status !== 'connected'" :title="t('forward')" :aria-label="t('forward')" @click="send({ type: 'forward', tab: activeID })"><ArrowRight :size="18" /></button>
      <button type="button" class="web-browser__icon" :disabled="!activeTab || startPage || status !== 'connected'" :title="t(activeTab?.loading ? 'stop' : 'reload')" :aria-label="t(activeTab?.loading ? 'stop' : 'reload')" @click="send({ type: activeTab?.loading ? 'stop' : 'reload', tab: activeID })"><X v-if="activeTab?.loading" :size="18" /><RefreshCw v-else :size="17" /></button>
      <label class="web-browser__address"><Search :size="16" aria-hidden="true" /><input ref="addressInput" v-model="address" :aria-label="t('address')" :placeholder="t('address')" maxlength="4096" autocomplete="off" spellcheck="false" @focus="editingAddress = true" @blur="editingAddress = false" /><button type="submit" :aria-label="t('go')"><ArrowRight :size="17" /></button></label>
      <button type="button" class="web-browser__icon" :title="t('home')" :aria-label="t('home')" @click="home"><Home :size="17" /></button>
      <button type="button" class="web-browser__icon" :disabled="!address && !activeTab" :title="t('external')" :aria-label="t('external')" @click="external"><ExternalLink :size="17" /></button>
    </form>
    <div v-if="notice" class="web-browser__notice" role="alert"><span>{{ notice }}</span><button class="web-browser__icon" :aria-label="t('clear')" @click="notice = ''"><X :size="15" /></button></div>
    <main ref="screen" class="web-browser__content">
      <div v-if="status !== 'connected'" class="web-browser__state">
        <LoaderCircle v-if="status === 'connecting'" :size="32" class="is-spinning" /><Globe2 v-else :size="40" />
        <h2>{{ t(status === 'connecting' ? 'connecting' : status === 'unconfigured' ? 'unconfigured' : status === 'disconnected' ? 'disconnected' : 'unavailable') }}</h2>
        <p v-if="status !== 'connecting'">{{ t(status === 'unconfigured' ? 'unconfiguredHint' : 'unavailableHint') }}</p>
        <div v-if="status !== 'connecting'" class="web-browser__actions"><button class="button button--primary" @click="connect">{{ t('retry') }}</button><button v-if="address || activeTab" class="button button--secondary" @click="external">{{ t('external') }}</button></div>
      </div>
      <div v-else-if="startPage || !activeTab" class="web-browser__start">
        <span class="web-browser__emblem"><Globe2 :size="40" :stroke-width="1.5" /></span>
        <h1>{{ t('startTitle') }}</h1><p>{{ t('startHint') }}</p>
        <form class="web-browser__search" @submit.prevent="submit"><Search :size="20" /><input v-model="address" :aria-label="t('address')" :placeholder="t('address')" maxlength="4096" /><button class="button button--primary" type="submit">{{ t('go') }}<ArrowRight :size="16" /></button></form>
        <div v-if="links.length" class="web-browser__quick"><h2>{{ t('quick') }}</h2><div><button v-for="link in links.slice(0, 12)" :key="link.id" @click="openURL(link.url)"><img v-if="link.iconURL" :src="link.iconURL" alt="" /><Globe2 v-else :size="24" /><span>{{ link.name }}</span></button></div></div>
      </div>
      <div v-else class="web-browser__viewport">
        <div v-if="!imageReady" class="web-browser__loading" role="status"><LoaderCircle :size="24" class="is-spinning" /><span>{{ t('loading') }}</span></div>
        <canvas ref="frame" :class="{ 'is-ready': imageReady }" :aria-label="t('screen')" @pointerdown.prevent="pointer($event, 'mousePressed')" @pointerup.prevent="pointer($event, 'mouseReleased')" @pointermove.prevent="pointer($event, 'mouseMoved')" @wheel.prevent="wheel" @contextmenu.prevent />
        <textarea ref="typing" class="web-browser__keyboard" :aria-label="t('input')" autocapitalize="off" autocomplete="off" spellcheck="false" @keydown="key($event, 'keyDown')" @keyup="key($event, 'keyUp')" @compositionstart="composing = true" @compositionend="composing = false; textInput($event)" @input="textInput" />
      </div>
    </main>
    <footer class="web-browser__footer"><span><span class="web-browser__dot" :class="{ 'is-connected': status === 'connected' }" />{{ t(status === 'connected' ? 'connected' : status === 'connecting' ? 'connecting' : 'disconnected') }}</span><span class="web-browser__network"><Network :size="13" />{{ t('server') }}</span><span class="web-browser__session-hint">{{ t('sessionHint') }}</span><form v-if="activeTab && !startPage && status === 'connected'" @submit.prevent="insert"><input v-model="text" :aria-label="t('input')" :placeholder="t('input')" maxlength="8192" /><button type="submit">{{ t('send') }}</button></form></footer>
    <ModalDialog :open="Boolean(dialog)" :title="t('dialog')" size="compact" @close="answer(false)"><p class="web-browser__dialog">{{ dialog?.text }}</p><template #footer><button class="button button--secondary" @click="answer(false)">{{ t('cancel') }}</button><button class="button button--primary" @click="answer(true)">{{ t('accept') }}</button></template></ModalDialog>
  </section>
</template>

<style scoped>
.web-browser{display:flex;flex-direction:column;width:100%;height:100%;min-height:24rem;overflow:hidden;background:var(--surface);color:var(--text);font-size:.875rem}
.web-browser__tabs{display:flex;align-items:center;gap:.25rem;padding:.4rem .65rem 0;min-width:0;min-height:2.6rem;background:var(--surface-subtle);font-size:.875rem}
.web-browser__tabs--titlebar{height:42px;min-height:42px;padding:0 .25rem;background:transparent}.web-browser__tab-track{display:flex;align-items:center;flex:0 1 auto;min-width:0;gap:.25rem;overflow-x:auto;scrollbar-width:thin}.web-browser__tab-count{margin-left:auto;flex-shrink:0;font-size:.8125rem;color:var(--muted);padding:0 .25rem}.web-browser__tabs--titlebar .web-browser__tab{border-radius:var(--radius-sm);height:32px}.web-browser__tabs--titlebar .web-browser__tab button{padding:.4rem}.web-browser__tabs--titlebar .web-browser__tab.is-active{border-color:var(--border)}
.web-browser__tab{display:flex;align-items:center;min-width:7rem;max-width:13rem;border:1px solid transparent;border-radius:var(--radius-sm) var(--radius-sm) 0 0;flex-shrink:0}
.web-browser__tab.is-active{background:var(--surface);border-color:var(--border);border-bottom-color:var(--surface)}
.web-browser__tab button{display:flex;align-items:center;gap:.5rem;border:0;background:none;color:inherit;padding:.65rem .5rem;font:inherit;min-width:0;cursor:pointer}.web-browser__tab button:first-child{flex:1}.web-browser__tab span{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.web-browser__network{display:flex;align-items:center;gap:.4rem;margin-left:auto;white-space:nowrap;font-size:.8125rem;color:var(--muted);padding:0 .5rem}
.web-browser__toolbar{display:flex;align-items:center;gap:.3rem;padding:.65rem;border-bottom:1px solid var(--border)}
.web-browser__icon{display:inline-flex;align-items:center;justify-content:center;flex-shrink:0;width:2.1rem;min-height:2.1rem;border:0;border-radius:var(--radius-sm);background:none;color:var(--text);cursor:pointer}.web-browser__icon:hover{background:var(--surface-subtle)}.web-browser__icon:disabled{opacity:.4;cursor:default}
.web-browser__address{display:flex;align-items:center;gap:.5rem;min-width:0;flex:1;background:var(--surface-subtle);border:1px solid var(--border);border-radius:999px;padding:.25rem .65rem;color:var(--muted)}
.web-browser__address input{min-width:0;width:100%;border:0;background:none;color:var(--text);font:inherit;outline:none;padding:.3rem 0}.web-browser__address:focus-within{border-color:var(--brand);outline:2px solid var(--brand-soft)}.web-browser__address button{display:flex;border:0;background:none;color:var(--muted);padding:.25rem;cursor:pointer}
.web-browser__content{position:relative;min-height:0;flex:1;overflow:auto}.web-browser__state,.web-browser__start{display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:100%;padding:2rem;text-align:center;box-sizing:border-box}.web-browser__state>svg{color:var(--muted)}.web-browser__state h2{font-size:1.125rem;margin:1rem 0 .5rem}.web-browser__state p,.web-browser__start>p{max-width:34rem;line-height:1.7;color:var(--muted);margin:0 0 1.5rem;font-size:.875rem}.web-browser__actions{display:flex;flex-wrap:wrap;gap:.65rem;justify-content:center}
.web-browser__emblem{display:grid;place-items:center;width:5rem;height:5rem;border-radius:var(--radius-lg);background:var(--brand-soft);color:var(--brand);margin-bottom:1rem}.web-browser__start h1{font-size:clamp(1.25rem,2vw,1.65rem);line-height:1.4;font-weight:600;margin:0 0 .6rem}
.web-browser__search{display:flex;align-items:center;gap:.75rem;width:min(100%,36rem);padding:.55rem .6rem .55rem 1rem;border:1px solid var(--border);border-radius:var(--radius-lg);background:var(--surface-raised);box-shadow:var(--shadow-sm)}.web-browser__search>svg{flex-shrink:0;color:var(--muted)}.web-browser__search input{min-width:0;flex:1;border:0;background:none;color:var(--text);font:inherit;outline:none}.web-browser__search:focus-within{border-color:var(--brand)}.web-browser__search .button{display:flex;align-items:center;gap:.4rem;white-space:nowrap;font-size:.875rem}
.web-browser__quick{width:min(100%,36rem);margin-top:2rem;text-align:left}.web-browser__quick h2{font-size:.8125rem;font-weight:500;color:var(--muted);margin:0 0 .75rem}.web-browser__quick>div{display:grid;grid-template-columns:repeat(auto-fit,minmax(5rem,1fr));gap:.5rem}.web-browser__quick button{display:flex;flex-direction:column;align-items:center;gap:.6rem;min-width:0;padding:.8rem .4rem;border:0;border-radius:var(--radius-sm);background:none;color:var(--text);font:inherit;cursor:pointer}.web-browser__quick button:hover{background:var(--surface-subtle)}.web-browser__quick img{width:2rem;height:2rem;object-fit:contain}.web-browser__quick span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:100%}
.web-browser__viewport{position:relative;min-height:100%;display:flex;align-items:flex-start;justify-content:center;background:var(--surface-subtle)}.web-browser__viewport canvas{display:block;max-width:100%;height:auto;visibility:hidden;touch-action:none}.web-browser__viewport canvas.is-ready{visibility:visible}.web-browser__loading{position:absolute;inset:0;display:flex;gap:.75rem;align-items:center;justify-content:center;color:var(--muted)}.web-browser__keyboard{position:absolute;left:0;top:0;width:1px;height:1px;opacity:0;resize:none;padding:0;border:0}
.web-browser__footer{display:flex;align-items:center;flex-wrap:wrap;gap:.75rem;border-top:1px solid var(--border);padding:.5rem .85rem;color:var(--muted);font-size:.8125rem}.web-browser__footer>span:first-child{display:flex;gap:.4rem;align-items:center;white-space:nowrap}.web-browser__dot{width:.4rem;height:.4rem;border-radius:50%;background:var(--muted)}.web-browser__dot.is-connected{background:var(--brand)}.web-browser__session-hint{flex:1;min-width:10rem;font-size:.8125rem;line-height:1.5}.web-browser__footer form{display:flex;gap:.4rem;flex:1;min-width:10rem}.web-browser__footer input{min-width:0;width:100%;padding:.35rem .5rem;border:1px solid var(--border);border-radius:var(--radius-sm);background:var(--surface);color:var(--text);font:inherit;font-size:.875rem}.web-browser__footer button{border:0;background:none;color:var(--brand);font-size:.875rem;white-space:nowrap;cursor:pointer}.web-browser__notice{display:flex;gap:.75rem;align-items:center;justify-content:space-between;padding:.4rem .85rem;border-bottom:1px solid var(--border);background:var(--brand-soft);font-size:.875rem;line-height:1.5}.web-browser__dialog{white-space:pre-wrap;overflow-wrap:anywhere;font-size:.875rem;line-height:1.6}
.web-browser button:focus-visible,.web-browser__tabs button:focus-visible{outline:2px solid var(--brand);outline-offset:2px}.is-spinning{animation:browser-spin calc(var(--motion-duration-ambient) * 2) linear infinite}@keyframes browser-spin{to{transform:rotate(360deg)}}@media(prefers-reduced-motion:reduce){.is-spinning{animation:none}}@media(max-width:600px){.web-browser__toolbar{flex-wrap:wrap}.web-browser__address{order:-1;flex-basis:100%}.web-browser__network{font-size:.8125rem}.web-browser__start,.web-browser__state{padding:1.5rem 1rem}.web-browser__session-hint{display:none}}
</style>
