<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { Info, LoaderCircle, RefreshCw } from '@lucide/vue'
import { ApiError, api } from '@/lib/api'
import { clusterRemoteAccessChanged } from '@/lib/clusterSettings'
import { phraseCatalogVersion, translatePhrase, usePhraseCatalog } from '@/i18n/phrase'
import type { ClusterRemoteAccess } from '@/types/api'

usePhraseCatalog(locale => locale === 'en-US'
  ? import('@/i18n/pages/ClusterView/en-US').then(module => module.default)
  : import('@/i18n/pages/ClusterView/zh-TW').then(module => module.default))
const props = defineProps<{ open: boolean; active: boolean; formId: string }>()
const emit = defineEmits<{ busy: [value: boolean]; ready: [value: boolean]; saved: [] }>()
const snapshot = ref<ClusterRemoteAccess>()
const form = reactive({ terminalEnabled: true, filesEnabled: true })
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
let controller: AbortController | undefined
let sequence = 0
let loadedForOpen = false
const canSave = computed(() => !!snapshot.value?.available && !loading.value && !saving.value &&
  (form.terminalEnabled !== snapshot.value.terminalEnabled || form.filesEnabled !== snapshot.value.filesEnabled || !!snapshot.value.pendingTerminalCleanup))
watch(saving, value => emit('busy', value))
watch(canSave, value => emit('ready', value), { immediate: true })
function phrase(value: string): string { phraseCatalogVersion.value; return translatePhrase(value) }
async function load(): Promise<void> {
  const current = ++sequence
  controller?.abort()
  controller = new AbortController()
  loading.value = true; error.value = ''; message.value = ''
  try {
    const value = await api.cluster.remoteAccess(controller.signal)
    if (current !== sequence || !props.open) return
    snapshot.value = value
    form.terminalEnabled = value.terminalEnabled; form.filesEnabled = value.filesEnabled
    if (!value.available) error.value = '配置不可读取，远程操作已停用。请检查当前中心的数据目录。'
  } catch (reason) {
    if (current === sequence && !controller.signal.aborted) error.value = '远程操作设置读取失败，请重试。'
  } finally { if (current === sequence) loading.value = false }
}
async function save(): Promise<void> {
  if (!canSave.value || !snapshot.value) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    const value = await api.cluster.updateRemoteAccess({ ...form, expectedResourceVersion: snapshot.value.resourceVersion })
    snapshot.value = value
    form.terminalEnabled = value.terminalEnabled; form.filesEnabled = value.filesEnabled
    message.value = value.pendingTerminalCleanup
      ? '设置已保存，部分远端会话尚待关闭；远程输入与重连已禁用。可再次保存设置重试关闭。'
      : '远程操作设置已保存。'
    window.dispatchEvent(new Event(clusterRemoteAccessChanged))
    emit('saved')
  } catch (reason) {
    error.value = reason instanceof ApiError && reason.status === 409
      ? '设置已在其他页面更新，请重新读取后再修改。'
      : '远程操作设置保存失败，当前选择尚未保存，请重试。'
  } finally { saving.value = false }
}
watch(() => [props.open, props.active] as const, ([open, active]) => {
  if (open && active && !loadedForOpen) { loadedForOpen = true; void load() }
  else if (!open) { loadedForOpen = false; sequence++; controller?.abort(); loading.value = false }
}, { immediate: true })
onBeforeUnmount(() => { sequence++; controller?.abort() })
</script>

<template>
  <form :id="formId" class="cluster-remote-access" @submit.prevent="save">
    <div v-if="loading && !snapshot" class="cluster-remote-access__loading" role="status"><LoaderCircle class="spin" :size="18" />{{ phrase('正在读取远程操作设置…') }}</div>
    <template v-else>
      <header><h3>{{ phrase('远程操作') }}</h3><p>{{ phrase('统一控制当前中心对所有远程节点的访问。') }}</p></header>
      <fieldset :disabled="loading || saving || !snapshot?.available">
        <label class="cluster-remote-access__row">
          <span><strong>{{ phrase('允许远程终端') }}</strong><small>{{ phrase('连接远程终端、恢复会话及批量执行命令。') }}</small></span>
          <input v-model="form.terminalEnabled" class="cluster-remote-access__switch" type="checkbox" role="switch" :aria-label="phrase('允许远程终端')" />
        </label>
        <label class="cluster-remote-access__row">
          <span><strong>{{ phrase('允许远程文件管理') }}</strong><small>{{ phrase('浏览、编辑、上传、下载及跨节点文件互传。') }}</small></span>
          <input v-model="form.filesEnabled" class="cluster-remote-access__switch" type="checkbox" role="switch" :aria-label="phrase('允许远程文件管理')" />
        </label>
      </fieldset>
      <aside class="cluster-remote-access__scope"><Info :size="18" /><p>{{ phrase('适用于已接入和以后新增的远程节点。') }}<br>{{ phrase('本机管理与集群监控不受影响，不修改系统 SSH 服务。') }}</p></aside>
      <p class="cluster-remote-access__hint">{{ phrase('关闭后停止远程访问，并取消可取消的连接与传输；已完成的操作不会撤销。') }}</p>
      <p class="cluster-remote-access__hint">{{ phrase('重新开启无需重新配对，不会自动恢复命令或传输。') }}</p>
      <p v-if="message" class="cluster-remote-access__message" role="status" aria-live="polite">{{ phrase(message) }}</p>
      <p v-if="error" class="cluster-remote-access__error" role="alert">{{ phrase(error) }}</p>
      <button v-if="error" type="button" class="button button--secondary button--small" :disabled="saving || loading" @click="load"><RefreshCw :size="14" />{{ phrase('重新读取设置') }}</button>
    </template>
  </form>
</template>

<style scoped>
.cluster-remote-access { display:grid; gap:16px; min-width:0; }
.cluster-remote-access header h3 { margin:0 0 6px; font-size:16px; }
.cluster-remote-access header p, .cluster-remote-access__hint { margin:0; color:var(--muted); font-size:13px; line-height:1.6; }
.cluster-remote-access fieldset { margin:0; padding:0; border:0; min-width:0; }
.cluster-remote-access__row { display:flex; align-items:center; justify-content:space-between; gap:20px; padding:18px 0; cursor:pointer; }
.cluster-remote-access__row + .cluster-remote-access__row { border-top:1px solid var(--border); }
.cluster-remote-access__row > span { min-width:0; }
.cluster-remote-access__row strong { display:block; font-size:14px; }
.cluster-remote-access__row small { display:block; margin-top:6px; color:var(--muted); font-size:13px; line-height:1.6; }
.cluster-remote-access__switch { appearance:none; position:relative; flex:0 0 40px; width:40px; height:24px; margin:0; border:1px solid var(--border-strong); border-radius:999px; background:var(--muted); cursor:pointer; }
.cluster-remote-access__switch::after { content:''; position:absolute; top:3px; left:3px; width:16px; height:16px; border-radius:50%; background:var(--surface); transition:transform var(--motion-duration-fast); }
.cluster-remote-access__switch:checked { border-color:var(--brand); background:var(--brand); }
.cluster-remote-access__switch:checked::after { transform:translateX(16px); }
.cluster-remote-access__switch:focus-visible { outline:2px solid var(--brand); outline-offset:3px; }
.cluster-remote-access__switch:disabled { opacity:.5; cursor:default; }
.cluster-remote-access__scope { display:flex; gap:10px; padding:12px; border:1px solid var(--border); border-radius:var(--radius); background:var(--surface-subtle); color:var(--text-soft); }
.cluster-remote-access__scope svg { flex-shrink:0; margin-top:2px; color:var(--brand-strong); }
.cluster-remote-access__scope p { margin:0; font-size:13px; line-height:1.6; }
.cluster-remote-access__loading { display:flex; align-items:center; gap:8px; padding:24px 0; }
.cluster-remote-access__message { margin:0; font-size:14px; color:var(--text); }
.cluster-remote-access__error { margin:0; font-size:14px; color:var(--danger); }
@media (prefers-reduced-motion:reduce) { .cluster-remote-access__switch::after { transition:none; } }
@media (forced-colors:active) { .cluster-remote-access__switch { appearance:auto; } .cluster-remote-access__switch::after { display:none; } }
</style>
