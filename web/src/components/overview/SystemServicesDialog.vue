<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RefreshCw, Search, Server } from '@lucide/vue'
import ModalDialog from '@/components/common/ModalDialog.vue'
import LoadingState from '@/components/feedback/LoadingState.vue'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import { api } from '@/lib/api'
import type { SystemServiceAction, SystemServiceDetail, SystemServiceLogs, SystemServicesSnapshot } from '@/types/systemServices'

const props = withDefaults(defineProps<{ open: boolean; readable: boolean; writable: boolean; unavailableReason?: string }>(), { unavailableReason: '' })
const emit = defineEmits<{ close: [] }>()
const snapshot = ref<SystemServicesSnapshot>()
const selected = ref('')
const detail = ref<SystemServiceDetail>()
const logs = ref<SystemServiceLogs>()
const search = ref('')
const filter = ref('all')
const page = ref(1)
const pageSize = 25
const loading = ref(false)
const detailLoading = ref(false)
const logsLoading = ref(false)
const running = ref(false)
const error = ref('')
const detailError = ref('')
const logsError = ref('')
const actionMessage = ref('')
const actionError = ref('')
let epoch = 0
let listController: AbortController | undefined
let detailController: AbortController | undefined
let logsController: AbortController | undefined

function phrase(value: string): string {
  phraseCatalogVersion.value
  return translatePhrase(value)
}

const actions: { id: SystemServiceAction; label: string }[] = [
  { id: 'start', label: '启动' }, { id: 'stop', label: '停止' },
  { id: 'restart', label: '重启' }, { id: 'reload', label: '重载配置' },
  { id: 'enable', label: '启用开机自启' }, { id: 'disable', label: '禁用开机自启' },
]
const filtered = computed(() => {
  const query = search.value.trim().toLowerCase()
  return (snapshot.value?.services || []).filter((service) =>
    (filter.value === 'all' || service.activeState === filter.value)
    && (!query || `${service.name} ${service.description}`.toLowerCase().includes(query)))
})
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const visible = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch([search, filter], () => { page.value = 1 })
watch(pages, (count) => { page.value = Math.min(page.value, count) })

function stateLabel(state: string): string {
  const labels: Record<string, string> = { active: '运行中', inactive: '未运行', failed: '失败', activating: '启动中', deactivating: '停止中', reloading: '重载中', enabled: '已启用', disabled: '已禁用', static: '静态单元', masked: '已屏蔽', 'enabled-runtime': '临时启用', 'masked-runtime': '临时屏蔽', alias: '别名', indirect: '间接启用', generated: '自动生成', transient: '临时单元', unknown: '未知' }
  return labels[state] ? phrase(labels[state]) : state || phrase('未知')
}

function message(reason: unknown, fallback: string): string {
  const code = reason && typeof reason === 'object' && 'code' in reason ? String(reason.code) : ''
  const messages: Record<string, string> = {
    invalid_system_service_query: '服务名称、参数或状态无效。',
    invalid_system_service_action: '服务名称、参数或状态无效。',
    invalid_system_service: '服务名称、参数或状态无效。',
    system_services_write_disabled: '当前 Agent 未启用服务写入，仅可查看。',
    system_services_busy: '服务操作正在执行，请稍后重试。',
    system_service_conflict: '服务状态已变化，请刷新后重试。',
    system_services_timeout: '服务操作超时，请刷新核对实际状态。',
    system_services_unsupported: '当前主机不支持 systemd 服务管理。',
    system_services_unavailable: '服务操作未能确认完成。',
    system_services_needs_attention: '服务操作未能确认完成。',
  }
  if (Object.hasOwn(messages, code)) {
    const summary = phrase(messages[code]!)
    return reason instanceof Error && reason.message ? `${summary} ${reason.message}` : summary
  }
  return reason instanceof Error ? reason.message : phrase(fallback)
}

async function loadList(): Promise<void> {
  if (!props.open || !props.readable) return
  listController?.abort()
  const request = new AbortController()
  listController = request
  loading.value = true
  error.value = ''
  try {
    const result = await api.system.services(request.signal)
    if (request.signal.aborted || request !== listController) return
    snapshot.value = result
  } catch (reason) {
    if (!request.signal.aborted && request === listController) error.value = message(reason, '无法读取 systemd 服务。')
  } finally {
    if (request === listController) loading.value = false
  }
}

async function loadDetail(name: string): Promise<void> {
  if (!props.open || !props.readable) return
  detailController?.abort()
  const request = new AbortController()
  detailController = request
  detailLoading.value = true
  detailError.value = ''
  try {
    const result = await api.system.service(name, request.signal)
    if (request.signal.aborted || request !== detailController || selected.value !== name) return
    detail.value = result
  } catch (reason) {
    if (!request.signal.aborted && request === detailController) {
      detail.value = undefined
      detailError.value = message(reason, '无法读取服务详情，请刷新重试。')
    }
  } finally {
    if (request === detailController) detailLoading.value = false
  }
}

function selectService(name: string): void {
  if (running.value) return
  selected.value = name
  detail.value = undefined
  logsController?.abort()
  logs.value = undefined
  logsError.value = ''
  logsLoading.value = false
  actionMessage.value = ''
  actionError.value = ''
  void loadDetail(name)
}

async function loadLogs(): Promise<void> {
  if (!selected.value || !props.open || !props.readable) return
  logsController?.abort()
  const request = new AbortController()
  logsController = request
  const name = selected.value
  logsLoading.value = true
  logsError.value = ''
  try {
    const result = await api.system.serviceLogs(name, request.signal)
    if (!request.signal.aborted && request === logsController && selected.value === name) logs.value = result
  } catch (reason) {
    if (!request.signal.aborted && request === logsController) logsError.value = message(reason, '无法读取服务日志。')
  } finally {
    if (request === logsController) logsLoading.value = false
  }
}

function canExecute(action: SystemServiceAction): boolean {
  if (!props.writable || running.value || loading.value || detailLoading.value || !detail.value?.resourceVersion) return false
  const service = detail.value
  if (action === 'start') return service.canStart && !['active', 'activating'].includes(service.activeState)
  if (action === 'stop') return service.canStop && service.activeState !== 'inactive'
  if (action === 'restart') return service.canStart && service.canStop
  if (action === 'reload') return service.canReload && service.activeState === 'active'
  return true
}

async function execute(action: SystemServiceAction): Promise<void> {
  if (!canExecute(action) || !detail.value) return
  const target = detail.value
  const label = phrase(actions.find((item) => item.id === action)!.label)
  if (!window.confirm(phrase('确认对服务 {0} 执行“{1}”？这可能影响网络、SSH 或面板连接。').replace('{0}', target.name).replace('{1}', label))) return
  const currentEpoch = epoch
  running.value = true
  actionError.value = ''
  actionMessage.value = ''
  try {
    const result = await api.system.serviceAction({ name: target.name, action, expectedResourceVersion: target.resourceVersion })
    if (epoch !== currentEpoch || !props.open) return
    actionMessage.value = result.accepted
      ? '请求已提交给 systemd；请刷新确认最终状态。'
      : '开机自启设置已更新，并已重新读取服务状态。'
    await Promise.all([loadList(), loadDetail(target.name)])
  } catch (reason) {
    if (epoch !== currentEpoch || !props.open) return
    actionError.value = message(reason, '服务操作未能确认完成。')
    actionMessage.value = '请刷新查看实际状态；连接中断不代表服务操作失败，请勿重复提交。'
    await Promise.all([loadList(), loadDetail(target.name)])
  } finally {
    running.value = false
  }
}

function refresh(): void {
  if (running.value) return
  void loadList()
  if (selected.value) void loadDetail(selected.value)
}

function abortReads(): void {
  epoch++
  listController?.abort()
  detailController?.abort()
  logsController?.abort()
}

watch(() => [props.open, props.readable] as const, ([open, readable]) => {
  abortReads()
  snapshot.value = undefined
  detail.value = undefined
  logs.value = undefined
  selected.value = ''
  error.value = detailError.value = logsError.value = actionError.value = actionMessage.value = ''
  loading.value = detailLoading.value = logsLoading.value = false
  search.value = ''
  filter.value = 'all'
  if (open && readable) void loadList()
}, { immediate: true })
onBeforeUnmount(abortReads)
</script>

<template>
  <ModalDialog :open="open" title="systemd 服务管理" description="管理当前主机的系统服务，查看运行状态、开机自启和最近日志。" size="wide" @close="emit('close')">
    <div class="services-manager">
      <p v-if="!readable" class="services-notice" role="status">{{ unavailableReason || phrase('当前主机不支持 systemd 服务管理。') }}</p>
      <template v-else>
        <div class="services-toolbar">
          <label class="services-search"><Search :size="18" aria-hidden="true" /><input v-model="search" aria-label="搜索服务" placeholder="搜索服务名称或描述" type="search" /></label>
          <select v-model="filter" aria-label="筛选运行状态"><option value="all">全部状态</option><option value="active">运行中</option><option value="inactive">未运行</option><option value="failed">失败</option></select>
          <button type="button" class="button button--secondary" :disabled="loading || running" @click="refresh"><RefreshCw :size="16" aria-hidden="true" />{{ loading ? phrase('刷新中') : phrase('刷新') }}</button>
        </div>
        <p v-if="error" class="services-error" role="alert">{{ error }}</p>
        <LoadingState v-if="loading && !snapshot" title="正在读取系统服务" />
        <p v-else-if="snapshot && !snapshot.supported" class="services-notice" role="status">{{ snapshot.reason || phrase('当前主机不支持 systemd 服务管理。') }}</p>
        <template v-else-if="snapshot">
          <p v-if="snapshot.truncated" class="services-notice" role="status">服务列表已达到显示上限，部分服务未显示。</p>
          <div class="services-layout">
            <section class="services-list" aria-label="服务列表" :aria-busy="loading">
              <p class="services-count">{{ filtered.length }} 个服务</p>
              <p v-if="!visible.length" class="services-empty" role="status">没有匹配的服务。</p>
              <button v-for="service in visible" :key="service.name" type="button" class="services-row" :class="{ 'is-selected': selected === service.name }" :aria-pressed="selected === service.name" :disabled="running" @click="selectService(service.name)">
                <span class="services-row__name" data-i18n-ignore>{{ service.name }}</span>
                <span class="services-state" :class="{ 'is-failed': service.activeState === 'failed' }">{{ stateLabel(service.activeState) }}</span>
                <span class="services-row__description" data-i18n-ignore>{{ service.description || service.name }}</span>
                <span class="services-row__boot">{{ stateLabel(service.unitFileState) }}</span>
              </button>
              <nav v-if="pages > 1" class="services-pagination" aria-label="服务分页"><button type="button" class="button button--secondary" :disabled="page <= 1" @click="page--">上一页</button><span>{{ page }} / {{ pages }}</span><button type="button" class="button button--secondary" :disabled="page >= pages" @click="page++">下一页</button></nav>
            </section>
            <section class="services-detail" aria-label="服务详情" :aria-busy="detailLoading">
              <div v-if="!selected" class="services-empty"><Server :size="28" aria-hidden="true" /><p>选择服务，查看详情和管理操作。</p></div>
              <template v-else>
                <h3 data-i18n-ignore>{{ selected }}</h3>
                <p v-if="detailError" class="services-error" role="alert">{{ detailError }}</p>
                <LoadingState v-if="detailLoading && !detail" title="正在读取服务详情" />
                <template v-if="detail">
                  <p class="services-description" data-i18n-ignore>{{ detail.description }}</p>
                  <dl><dt>运行状态</dt><dd>{{ stateLabel(detail.activeState) }} <span data-i18n-ignore>({{ detail.subState }})</span></dd><dt>开机自启</dt><dd>{{ stateLabel(detail.unitFileState) }}</dd><dt>加载状态</dt><dd data-i18n-ignore>{{ detail.loadState }}</dd><dt>主进程 PID</dt><dd>{{ detail.mainPID || '—' }}</dd><dt>单元文件</dt><dd data-i18n-ignore>{{ detail.fragmentPath || '—' }}</dd></dl>
                  <p v-if="!writable" class="services-notice">当前 Agent 未启用服务写入，仅可查看。</p>
                  <div class="services-actions"><button v-for="action in actions" :key="action.id" type="button" class="button button--secondary" :data-action="action.id" :disabled="!canExecute(action.id)" @click="execute(action.id)">{{ phrase(action.label) }}</button></div>
                  <p class="services-hint">开机自启设置不会立即启动或停止服务；静态、屏蔽等限制由 systemd 返回。</p>
                </template>
                <p v-if="running" role="status">正在提交服务操作…</p>
                <p v-if="actionError" class="services-error" role="alert">{{ actionError }}</p>
                <p v-if="actionMessage" class="services-notice" role="status">{{ phrase(actionMessage) }}</p>
                <div class="services-log-header"><h4>最近日志</h4><button type="button" class="button button--secondary" :disabled="logsLoading" @click="loadLogs">{{ logsLoading ? phrase('读取中') : phrase('读取日志') }}</button></div>
                <p v-if="logsError" class="services-error" role="alert">{{ logsError }}</p>
                <p v-if="logs?.truncated" class="services-hint">日志已截断，仅显示最近的有界输出。</p>
                <pre v-if="logs" class="services-logs" tabindex="0" aria-label="服务日志" data-i18n-ignore>{{ logs.content || phrase('暂无服务日志。') }}</pre>
              </template>
            </section>
          </div>
        </template>
      </template>
    </div>
  </ModalDialog>
</template>

<style scoped>
.services-manager { font-size: 14px; line-height: 1.55; min-width: 0; }
.services-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-bottom: 16px; }
.services-search { display: flex; align-items: center; gap: 8px; flex: 1 1 220px; min-width: 0; }
.services-search input { width: 100%; min-width: 0; }
.services-toolbar input, .services-toolbar select { background: var(--surface); color: var(--text); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 10px 12px; font: inherit; }
.services-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.15fr); gap: 20px; }
.services-list, .services-detail { min-width: 0; }
.services-list { max-height: 55vh; overflow: auto; padding: 2px; }
.services-count, .services-hint { color: var(--text-soft); font-size: 13px; }
.services-row { width: 100%; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 4px 12px; text-align: start; background: var(--surface); color: var(--text); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 12px; margin-bottom: 8px; font: inherit; cursor: pointer; }
.services-row:hover, .services-row.is-selected { background: var(--brand-soft); border-color: var(--brand); }
.services-row__name { font-weight: 600; overflow-wrap: anywhere; }
.services-row__description { font-size: 13px; color: var(--text-soft); overflow-wrap: anywhere; }
.services-row__boot { font-size: 13px; color: var(--text-soft); }
.services-state { font-size: 14px; }
.services-state.is-failed { color: var(--danger); }
.services-detail { border: 1px solid var(--border); border-radius: var(--radius); padding: 16px; align-self: start; background: var(--surface); }
.services-detail h3 { font-size: 18px; margin: 0 0 8px; overflow-wrap: anywhere; }
.services-description { overflow-wrap: anywhere; color: var(--text-soft); }
.services-detail dl { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 8px 16px; margin: 20px 0; }
.services-detail dt { color: var(--text-soft); }
.services-detail dd { margin: 0; overflow-wrap: anywhere; }
.services-actions, .services-pagination { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.services-manager .button { min-height: 40px; font-size: 14px; white-space: normal; }
.services-notice, .services-error { padding: 12px; border-radius: var(--radius-sm); background: var(--surface-subtle); overflow-wrap: anywhere; }
.services-error { color: var(--danger); border: 1px solid var(--border); }
.services-empty { color: var(--text-soft); padding: 28px 12px; text-align: center; }
.services-log-header { display: flex; gap: 12px; align-items: center; justify-content: space-between; margin-top: 20px; }
.services-log-header h4 { font-size: 16px; margin: 0; }
.services-logs { background: var(--surface-subtle); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 12px; max-height: 300px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 13px; line-height: 1.6; }
.services-manager :is(button, input, select, pre):focus-visible { outline: 2px solid var(--brand); outline-offset: 2px; }
@media (max-width: 760px) { .services-layout { grid-template-columns: minmax(0, 1fr); } .services-list { max-height: 35vh; } .services-detail { padding: 12px; } }
</style>
