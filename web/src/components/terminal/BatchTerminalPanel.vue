<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type ComponentPublicInstance } from 'vue'
import { CheckCircle2, ChevronDown, LoaderCircle, Play, TerminalSquare, XCircle } from '@lucide/vue'
import StatusBadge from '@/components/feedback/StatusBadge.vue'
import OperatingSystemIcon from '@/components/overview/OperatingSystemIcon.vue'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import { ApiError, api } from '@/lib/api'
import { detectOperatingSystemIdentity } from '@/lib/operatingSystem'
import { useTerminalHistory } from '@/stores/terminalHistory'
import type { ClusterHost } from '@/types/api'
import type { BatchState, TerminalBatch } from '@/types/terminalBatch'

function phrase(value: string): string { phraseCatalogVersion.value; return translatePhrase(value) }
const props = defineProps<{ hosts: ClusterHost[]; hostInventory?: ClusterHost[]; sessionCapacity: number }>()
const emit = defineEmits<{ runningChange: [running: boolean]; restoreSelection: [ids: string[]] }>()
const command = ref('')
const panelElement = ref<HTMLElement>()
const jobs = ref<TerminalBatch[]>([])
const selectedID = ref('')
const loading = ref(true)
const submitting = ref(false)
const stopping = ref(false)
const available = ref(false)
const error = ref('')
const storageWarning = ref('')
const syncError = ref('')
const selectedJob = computed(() => jobs.value.find(job => job.id === selectedID.value))
const activeJob = computed(() => jobs.value.find(job => job.state === 'running'))
const executing = computed(() => Boolean(activeJob.value))
const cleanupJobs = computed(() => jobs.value.filter(job => job.cleanupPending || job.cleanupRunning))
const needsCleanup = computed(() => cleanupJobs.value.length > 0)
const cleanupHosts = computed(() => [...new Set(cleanupJobs.value.flatMap(job => job.results.filter(result => result.error === 'terminal_close_failed').map(result => result.host.name)))].join('、'))
const cleaningUp = computed(() => jobs.value.some(job => job.cleanupRunning))
const results = computed(() => (submitting.value ? [] : selectedJob.value?.results || []).map(result => ({ ...result, output: result.output || '', error: errorLabel(result.error || '') })))
const expandedHosts = ref<Set<string>>(new Set())
const terminalHistory = useTerminalHistory()
const commandInput = ref<HTMLTextAreaElement>()
const commandBytes = computed(() => new TextEncoder().encode(command.value).byteLength)
const commandError = computed(() => commandBytes.value > 16 * 1024 ? phrase('命令不能超过 16 KiB。') : '')
const canExecute = computed(() => !loading.value && available.value && !executing.value && !cleaningUp.value && !stopping.value && !submitting.value && !syncError.value && Boolean(command.value.trim()) && !commandError.value && props.hosts.length > 0)
const completedCount = computed(() => results.value.filter(item => ['succeeded', 'failed', 'timed_out', 'stopped', 'interrupted'].includes(item.state)).length)
const succeededCount = computed(() => results.value.filter(item => item.state === 'succeeded').length)
const failedCount = computed(() => results.value.filter(item => ['failed', 'timed_out', 'interrupted'].includes(item.state)).length)
const allExpanded = computed(() => results.value.length > 0 && results.value.every(item => expandedHosts.value.has(item.host.id)))
watch(executing, value => emit('runningChange', value), { immediate: true })

function operatingSystemIdentity(host: { id: string }) { return detectOperatingSystemIdentity((props.hostInventory || props.hosts).find(item => item.id === host.id)?.lastSnapshot?.telemetry) }
const outputPreElements = new Map<string, HTMLPreElement>()
function setOutputPre(id: string, element: Element | ComponentPublicInstance | null) { if (element instanceof HTMLPreElement) outputPreElements.set(id, element); else outputPreElements.delete(id) }
function scrollOutputToBottom(id: string) { const pre = outputPreElements.get(id); if (pre) pre.scrollTop = pre.scrollHeight }
function toggleExpanded(id: string) { const next = new Set(expandedHosts.value); if (next.has(id)) next.delete(id); else { next.add(id); void nextTick(() => scrollOutputToBottom(id)) }; expandedHosts.value = next }
function expandAll() { expandedHosts.value = new Set(results.value.map(item => item.host.id)); void nextTick(() => results.value.forEach(item => scrollOutputToBottom(item.host.id))) }
function collapseAll() { expandedHosts.value = new Set() }
function setCommandInput(element: Element | ComponentPublicInstance | null) { commandInput.value = element instanceof HTMLTextAreaElement ? element : undefined }
function applyQuickCommand(value: string) { if (executing.value || submitting.value) return; command.value = value; void nextTick(() => commandInput.value?.focus()) }
function executionLabel(state: BatchState): string {
  return phrase(({ queued: '等待执行', connecting: '正在连接', running: '命令执行中', closing: '正在释放会话', succeeded: '执行成功', failed: '执行失败', timed_out: '执行超时', stopped: '已手动终止', interrupted: '执行已中断' })[state])
}
function badgeStatus(state: BatchState): string { if (state === 'succeeded' || state === 'failed') return state; if (['timed_out', 'stopped', 'interrupted'].includes(state)) return 'expired'; return state === 'running' || state === 'closing' ? 'running_job' : 'queued' }
function errorLabel(code: string): string {
  const messages: Record<string, string> = {
    waiting_for_capacity: '等待终端名额，释放后自动继续。',
    terminal_close_failed: '会话释放结果未确认，请检查目标主机上的实际状态。',
    terminal_open_failed: '连接失败，请检查目标主机后单独重试。',
    terminal_input_uncertain: '命令发送结果不确定，请检查目标主机，避免重复执行。',
    terminal_output_failed: '无法继续读取输出，请检查目标主机上的实际执行结果。',
    terminal_execution_failed: '命令执行未完成。',
    terminal_exit_failed: '终端异常退出，请检查输出。',
    terminal_completion_unknown: '无法确认命令完成，请检查输出。',
    terminal_command_too_large: '命令包装后超过终端输入上限。',
    terminal_marker_failed: '无法准备终端完成标记。',
    batch_timeout: '超过 4 小时，任务已停止。',
    panel_restarted: 'Panel 已重启，未完成命令未自动重放。',
    panel_stopped: 'Panel 已停止，未完成命令未自动重放。',
  }
  return messages[code] ? phrase(messages[code]) : code
}
let disposed = false
let timer: ReturnType<typeof setTimeout> | undefined
const controller = new AbortController()
let restoredID = ''
let revision = 0
let refreshSequence = 0
let pending: { id: string; command: string; hostIds: string[] } | undefined
async function refresh(): Promise<void> {
  const sequence = ++refreshSequence
  const before = revision
  const current = () => !disposed && revision === before && sequence === refreshSequence
  try {
    const response = await api.terminals.batches(controller.signal)
    if (!current()) return
    const following = [...outputPreElements].filter(([, pre]) => pre.scrollHeight - pre.scrollTop - pre.clientHeight < 8).map(([id]) => id)
    const selection = response.items.find(job => job.state === 'running')?.id || response.items.find(job => job.id === selectedID.value)?.id || response.items[0]?.id
    if (selection) {
      const detail = await api.terminals.getBatch(selection, controller.signal)
      if (!current()) return
      response.items = response.items.map(job => job.id === detail.id ? detail : job)
    }
    syncError.value = ''
    jobs.value = response.items
    available.value = response.available
    storageWarning.value = !response.available ? phrase('批量记录不可用，请检查 Panel 数据目录。') : response.warning ? phrase('操作记录保存失败，当前进度可能无法在重启后保留。') : ''
    if (activeJob.value) selectedID.value = activeJob.value.id
    else if (!jobs.value.some(job => job.id === selectedID.value)) selectedID.value = jobs.value[0]?.id || ''
    const restored = activeJob.value || (loading.value ? selectedJob.value : undefined)
    if (restored && restored.id !== restoredID) { restoredID = restored.id; emit('restoreSelection', restored.results.map(result => result.host.id)) }
    if (pending && jobs.value.some(job => job.id === pending?.id)) pending = undefined
    await nextTick()
    following.forEach(scrollOutputToBottom)
  } catch {
    if (current()) syncError.value = phrase('无法同步批量进度，后台任务可能仍在执行，请刷新重连。')
  } finally { if (current()) loading.value = false }
}
async function poll(): Promise<void> {
  await refresh()
  if (!disposed) timer = setTimeout(() => void poll(), executing.value || cleaningUp.value ? 1000 : 4000)
}
async function execute(): Promise<void> {
  if (!canExecute.value) return
  submitting.value = true; error.value = ''
  if (panelElement.value) panelElement.value.scrollTop = 0
  const hostIds = props.hosts.map(host => host.id)
  if (!pending || pending.command !== command.value || JSON.stringify(pending.hostIds) !== JSON.stringify(hostIds)) {
    const nonce = crypto.getRandomValues(new Uint8Array(16))
    pending = { id: Array.from(nonce, byte => byte.toString(16).padStart(2, '0')).join(''), command: command.value, hostIds }
  }
  try {
    const job = await api.terminals.startBatch(pending)
    if (disposed) return
    revision++
    jobs.value = [job, ...jobs.value.filter(item => item.id !== job.id)]
    selectedID.value = job.id; expandedHosts.value = new Set()
    terminalHistory.record(command.value, '')
    pending = undefined
  } catch (reason) {
    if (disposed) return
    error.value = reason instanceof ApiError && reason.code === 'batch_busy' ? phrase('已有批量任务正在后台执行，请先查看当前进度。') : phrase('提交结果未确认，请同步进度后再重试，避免重复执行。')
    await refresh()
  } finally { if (!disposed) submitting.value = false }
}
async function stopExecution(): Promise<void> {
  const ids = activeJob.value ? [activeJob.value.id] : cleanupJobs.value.map(job => job.id)
  if (!ids.length || stopping.value || (!executing.value && cleaningUp.value) || submitting.value) return
  stopping.value = true; error.value = ''
  revision++
  try {
    // Retry every retained job so one offline host cannot hide older cleanup.
    const responses = await Promise.allSettled(ids.map(id => api.terminals.stopBatch(id)))
    if (disposed) return
    if (responses.some(response => response.status === 'rejected')) error.value = phrase('停止请求未确认，任务可能仍在执行，请重试。')
    revision++
    await refresh()
  } finally { if (!disposed) stopping.value = false }
}
defineExpose({ applyQuickCommand })
onMounted(() => void poll())
onBeforeUnmount(() => { disposed = true; controller.abort(); clearTimeout(timer); outputPreElements.clear(); emit('runningChange', false) })
</script>

<template>
  <section ref="panelElement" class="batch-terminal-panel" aria-label="批量执行">
    <div class="batch-command" :class="{ 'is-executing': executing }">
      <header>
        <div><TerminalSquare :size="19" aria-hidden="true" /><span><strong>{{ phrase('自定义命令') }}</strong><small>{{ phrase(`已选择 ${hosts.length} 台主机`) }}</small></span></div>
      </header>
      <label v-show="!executing">
        <span class="sr-only">{{ phrase('输入自定义命令') }}</span>
        <textarea v-model="command" :ref="setCommandInput" :disabled="executing" rows="4" spellcheck="false" :placeholder="phrase('例如：uname -a')" />
      </label>
      <footer>
        <p v-if="commandError" class="batch-command__error" role="alert">{{ commandError }}</p>
        <button v-if="executing || needsCleanup" class="button button--danger" type="button" :disabled="stopping || (!executing && cleaningUp) || submitting" @click="stopExecution">
          <XCircle :size="18" aria-hidden="true" />
          {{ phrase(executing ? '终止执行' : stopping || cleaningUp ? '正在释放会话' : '重试释放会话') }}
        </button>
        <button v-if="!executing" class="button button--primary" type="button" :disabled="!canExecute" @click="execute">
          <Play :size="18" aria-hidden="true" />
          {{ phrase('执行命令') }}
        </button>
      </footer>
      <p v-if="error" class="batch-result__error" role="alert">{{ error }}</p>
      <p v-if="syncError" class="batch-result__error" role="alert">{{ syncError }}</p>
      <p v-if="storageWarning" class="batch-result__error" role="alert">{{ storageWarning }}</p>
      <p v-if="needsCleanup && !executing" class="batch-result__error" role="status">{{ phrase('会话释放未确认，名额仍保留，请检查目标主机连接。') }} <span data-i18n-ignore>{{ cleanupHosts }}</span></p>
    </div>

    <div class="batch-status" :aria-busy="submitting">
      <header>
        <div><strong>{{ phrase('执行情况') }}</strong><small v-if="results.length">{{ phrase(`已完成 ${completedCount} / ${results.length} 台`) }}</small></div>
        <div v-if="results.length" class="batch-status__actions">
          <div class="batch-status__counts">
            <span><CheckCircle2 :size="15" aria-hidden="true" /> {{ phrase(`成功 ${succeededCount}`) }}</span>
            <span><XCircle :size="15" aria-hidden="true" /> {{ phrase(`失败 ${failedCount}`) }}</span>
          </div>
          <button type="button" class="batch-status__toggle" @click="allExpanded ? collapseAll() : expandAll()">
            {{ phrase(allExpanded ? '全部收起' : '全部展开') }}
          </button>
        </div>
      </header>
      <Transition name="batch-results" mode="out-in">
      <div v-if="submitting || !results.length" :key="submitting ? 'submitting' : 'empty'" class="batch-status__empty">
        <span><LoaderCircle v-if="submitting" class="spin" :size="28" aria-hidden="true" /><TerminalSquare v-else :size="28" aria-hidden="true" /></span>
        <strong>{{ phrase(submitting ? '正在提交…' : '等待执行') }}</strong>
        <p v-if="!submitting">{{ phrase('在左侧选择主机，输入命令后开始执行。') }}</p>
      </div>
      <div v-else :key="selectedID" class="batch-result-list">
        <article v-for="result in results" :key="result.host.id" class="batch-result" :class="{ 'is-expanded': expandedHosts.has(result.host.id) }">
          <button
            type="button"
            class="batch-result__summary"
            :aria-expanded="expandedHosts.has(result.host.id)"
            :aria-controls="`batch-result-detail-${result.host.id}`"
            :aria-label="phrase('展开输出')"
            @click="toggleExpanded(result.host.id)"
          >
            <OperatingSystemIcon class="batch-result__os" :distro="operatingSystemIdentity(result.host).key" :label="operatingSystemIdentity(result.host).label" :show-tooltip="false" />
            <strong data-i18n-ignore>{{ result.host.name }}</strong>
            <StatusBadge :status="badgeStatus(result.state)" :label="executionLabel(result.state)" />
            <ChevronDown :size="16" class="batch-result__chevron" aria-hidden="true" />
          </button>
          <div :id="`batch-result-detail-${result.host.id}`" v-show="expandedHosts.has(result.host.id)" class="batch-result__detail">
            <pre v-if="result.output" :ref="(element) => setOutputPre(result.host.id, element)" data-i18n-ignore>{{ result.output }}</pre>
            <div v-else-if="result.state === 'queued' || result.state === 'connecting' || result.state === 'running'" class="batch-result__pending"><LoaderCircle class="spin" :size="16" aria-hidden="true" /> {{ executionLabel(result.state) }}</div>
            <p v-if="!result.outputAvailable" class="batch-result__notice">{{ phrase('操作记录已保留，原终端输出已释放。') }}</p>
            <p v-if="result.truncated" class="batch-result__notice">{{ phrase('输出已截断，仅保留最后一部分。') }}</p>
            <p v-if="result.error" class="batch-result__error" role="alert"><span data-i18n-ignore>{{ result.error }}</span></p>
          </div>
        </article>
      </div>
      </Transition>
    </div>
  </section>
</template>

<style scoped>
.batch-terminal-panel { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow-y: auto; overscroll-behavior: contain; scrollbar-color: var(--terminal-shell-scrollbar, #35474a) var(--terminal-shell-panel, #111a1d); scrollbar-width: thin; color: var(--terminal-shell-text, #d8dddc); background: var(--terminal-shell-background, #0b1214); }
.batch-command { flex: 0 0 auto; padding: 18px; border-bottom: 1px solid var(--terminal-shell-border, #29383a); background: var(--terminal-shell-panel, #111a1d); }
.batch-command.is-executing { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 8px 16px; }
.batch-command.is-executing > header { margin-bottom: 0; }
.batch-command.is-executing > footer { margin-top: 0; }
.batch-command.is-executing > .batch-result__error { grid-column: 1 / -1; }
.batch-command > header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.batch-command > header > div { display: flex; align-items: center; gap: 9px; }
.batch-command > header svg { color: var(--brand); }
.batch-command > header span { display: grid; gap: 2px; }
.batch-command strong { font-size: 15px; }
.batch-command small { color: var(--terminal-shell-muted, #8a9695); font-size: 13px; }
.batch-command label { display: block; }
.batch-command textarea { width: 100%; min-height: 104px; resize: vertical; padding: 12px 14px; color: var(--terminal-shell-text, #d8dddc); background: var(--terminal-shell-background, #0b1214); border: 1px solid var(--terminal-shell-border, #29383a); border-radius: var(--radius-sm); outline: none; font: 14px/1.55 ui-monospace, SFMono-Regular, Consolas, monospace; tab-size: 2; scrollbar-color: var(--terminal-shell-scrollbar, #35474a) var(--terminal-shell-background, #0b1214); scrollbar-width: thin; }
.batch-command textarea:focus { border-color: var(--brand); box-shadow: 0 0 0 2px color-mix(in srgb, var(--brand) 18%, transparent); }
.batch-command textarea:disabled { cursor: wait; opacity: .72; }
.batch-command textarea::placeholder { color: var(--terminal-shell-muted, #8a9695); }
.batch-command footer { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 8px; }
.batch-command footer p { margin: 0; color: var(--terminal-shell-muted, #8a9695); font-size: 13px; line-height: 1.45; }
.batch-command footer p.batch-command__error { flex: 1; color: var(--danger); }
.batch-results-enter-active, .batch-results-leave-active { transition: opacity var(--motion-duration-fast) var(--motion-ease-fade); }
.batch-results-enter-from, .batch-results-leave-to { opacity: 0; }
@media (prefers-reduced-motion: reduce) {
  .batch-results-enter-active, .batch-results-leave-active { transition: none; }
}
.batch-status { display: flex; flex: 1 0 auto; flex-direction: column; }
.batch-status > header { display: flex; min-height: 58px; align-items: center; justify-content: space-between; gap: 14px; padding: 10px 18px; border-bottom: 1px solid var(--terminal-shell-border, #29383a); }
.batch-status > header > div:first-child { display: grid; gap: 2px; }
.batch-status > header strong { font-size: 14px; }
.batch-status > header small { color: var(--terminal-shell-muted, #8a9695); font-size: 13px; }
.batch-status__counts { display: flex; flex-wrap: wrap; gap: 12px; }
.batch-status__counts span { display: inline-flex; align-items: center; gap: 5px; color: var(--terminal-shell-muted, #8a9695); font-size: 13px; }
.batch-status__counts span:first-child svg { color: var(--success); }
.batch-status__counts span:last-child svg { color: var(--danger); }
.batch-status__actions { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
.batch-status__toggle { padding: 5px 12px; border: 1px solid var(--terminal-shell-border, #29383a); border-radius: var(--radius-sm); color: var(--terminal-shell-text, #d8dddc); background: transparent; font: inherit; font-size: 13px; cursor: pointer; transition: border-color .16s ease, color .16s ease; }
.batch-status__toggle:hover, .batch-status__toggle:focus-visible { border-color: var(--brand); color: var(--brand); outline: none; }
.batch-status__empty { display: grid; flex: 1; place-content: center; justify-items: center; gap: 7px; padding: 24px; text-align: center; }
.batch-status__empty > span { display: grid; width: 52px; height: 52px; place-items: center; color: var(--brand); background: color-mix(in srgb, var(--brand) 12%, transparent); border-radius: var(--radius); }
.batch-status__empty strong { font-size: 16px; }
.batch-status__empty p { margin: 0; color: var(--terminal-shell-muted, #8a9695); font-size: 14px; }
.batch-result-list { padding: 12px 16px 18px; scrollbar-color: var(--terminal-shell-scrollbar, #35474a) var(--terminal-shell-panel, #111a1d); scrollbar-width: thin; }
.batch-result { min-width: 0; margin-bottom: 10px; overflow: hidden; border: 1px solid var(--terminal-shell-border, #29383a); border-radius: var(--radius-sm); background: var(--terminal-shell-panel, #111a1d); }
.batch-result:last-child { margin-bottom: 0; }
.batch-result > header { display: flex; min-height: 58px; align-items: center; gap: 10px; padding: 10px 12px; border-bottom: 1px solid var(--terminal-shell-border, #29383a); }
.batch-result__summary { display: flex; width: 100%; min-height: 58px; align-items: center; gap: 10px; padding: 10px 12px; border: 0; color: inherit; background: transparent; font: inherit; text-align: left; cursor: pointer; }
.batch-result__summary:focus-visible { outline: 2px solid var(--brand); outline-offset: -2px; }
.batch-result__summary strong { min-width: 0; flex: 1; overflow-wrap: anywhere; font-size: 14px; }
.batch-result__chevron { flex: 0 0 auto; color: var(--terminal-shell-muted, #8a9695); transition: transform .16s ease; }
.batch-result.is-expanded > .batch-result__summary { border-bottom: 1px solid var(--terminal-shell-border, #29383a); }
.batch-result.is-expanded > .batch-result__summary .batch-result__chevron { transform: rotate(180deg); color: var(--brand); }
.batch-result__detail { display: grid; min-width: 0; }
.batch-result :deep(.batch-result__os) { width: 34px; height: 34px; flex: 0 0 auto; border-radius: var(--radius-sm); box-shadow: none; }
.batch-result pre { max-height: 190px; margin: 0; overflow: auto; padding: 12px 14px; color: var(--terminal-shell-text, #d8dddc); background: var(--terminal-wallpaper-background, var(--terminal-shell-background, #0b1214)); font: 13px/1.55 ui-monospace, SFMono-Regular, Consolas, monospace; white-space: pre-wrap; overflow-wrap: anywhere; scrollbar-color: var(--terminal-shell-scrollbar, #35474a) var(--terminal-shell-background, #0b1214); scrollbar-width: thin; }
.batch-result__pending { display: flex; min-height: 64px; align-items: center; justify-content: center; gap: 7px; color: var(--terminal-shell-muted, #8a9695); font-size: 14px; }
.batch-result__notice, .batch-result__error { margin: 0; padding: 9px 13px; font-size: 13px; line-height: 1.45; }
.batch-result__notice { color: var(--warning); }
.batch-result__error { color: var(--danger); font-size:14px; }

@media (max-width: 720px) {
  .batch-command { padding: 14px; }
  .batch-command footer, .batch-status > header { align-items: stretch; flex-direction: column; }
  .batch-command footer .button { width: 100%; justify-content: center; }
}
</style>
