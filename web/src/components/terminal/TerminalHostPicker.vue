<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RefreshCw } from '@lucide/vue'
import HostSwitcher from '@/components/common/HostSwitcher.vue'
import type { HostSwitcherStatus } from '@/lib/hostSwitcher'
import { api } from '@/lib/api'
import { useI18n } from '@/i18n'
import { withoutUnsupportedLightNodes } from '@/lib/nodeFeatureHosts'
import { sortClusterHosts, readClusterHostOrder } from '@/lib/clusterHostOrder'
import type { ClusterHost } from '@/types/api'

const props = defineProps<{ modelValue: string; active: boolean; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; validity: [valid: boolean] }>()
const { t } = useI18n()
const hosts = ref<ClusterHost[]>([])
const switcher = ref<InstanceType<typeof HostSwitcher>>()
const loading = ref(false)
const error = ref('')
let controller: AbortController | undefined
// A local target remains available even before inventory loads or when it fails.
const localFallback: ClusterHost = {
  id: 'local', isLocal: true, name: '', origin: '', transportSecurity: 'tls',
  remoteNodeId: '', federationProtocol: '', scope: '', terminalAvailable: true,
  mutualFileTransferAvailable: false, state: 'online', consecutiveFailures: 0,
  polling: false, resourceVersion: '', createdAt: '', updatedAt: '',
}
const options = computed(() => {
  const local = hosts.value.find(host => host.isLocal) || localFallback
  const remote = sortClusterHosts(withoutUnsupportedLightNodes(hosts.value), readClusterHostOrder())
    .filter(host => !host.isLocal && /^[a-f0-9]{32}$/.test(host.id))
  return [{ ...local, id: 'local' }, ...remote]
})
const selected = computed(() => options.value.find(host => host.id === props.modelValue))
const selectedLabel = computed(() => {
  if (!selected.value) return t('desktop.shortcutHostUnavailable')
  return selected.value.isLocal ? t('terminal.hostKind.local') : selected.value.name
})
const valid = computed(() => props.modelValue === 'local' || (!loading.value && !error.value && !!selected.value?.terminalAvailable))
watch(valid, value => emit('validity', value), { immediate: true })
function statusOf(host: ClusterHost): HostSwitcherStatus {
  const unavailable = !host.isLocal && (!host.terminalAvailable || loading.value || !!error.value)
  let label = t('terminal.hostKind.panel')
  if (host.isLocal) label = host.name || t('terminal.hostKind.local')
  if (unavailable) label = t('desktop.shortcutHostUnavailable')
  return {
    label,
    disabled: unavailable,
    tone: unavailable ? 'offline' : 'online',
  }
}
async function load(): Promise<void> {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  error.value = ''
  try {
    const inventory = await api.cluster.hosts(request.signal)
    if (!request.signal.aborted && controller === request) hosts.value = inventory.items
  } catch {
    if (!request.signal.aborted) error.value = t('desktop.shortcutHostsFailed')
  } finally {
    if (controller === request) loading.value = false
  }
}
function select(host: ClusterHost): void {
  if (props.disabled || statusOf(host).disabled) return
  emit('update:modelValue', host.id)
  switcher.value?.close(true)
}
watch(() => props.active, active => { if (active) void load(); else { controller?.abort(); switcher.value?.close() } }, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <div class="shortcut-host-picker">
    <span class="shortcut-host-picker__label">{{ t('desktop.shortcutHost') }}</span>
    <HostSwitcher ref="switcher" :hosts="options" :active-id="modelValue" :label="selectedLabel" :loading="loading" :error="!!error" :disabled="disabled" :status-of="statusOf" @refresh="load" @select="select" />
    <p v-if="error" role="alert">{{ error }} <button type="button" class="button button--ghost" :disabled="disabled || loading" @click="load"><RefreshCw :size="14" />{{ t('common.retry') }}</button></p>
    <p v-else-if="!valid && !loading" role="alert">{{ t('desktop.shortcutHostUnavailable') }}</p>
    <small>{{ t('desktop.shortcutHostHint') }}</small>
  </div>
</template>

<style scoped>
.shortcut-host-picker { display: grid; gap: 8px; min-width: 0; }
.shortcut-host-picker__label { font-size: 14px; font-weight: 600; }
.shortcut-host-picker small { color: var(--muted); font-size: 13px; line-height: 1.5; }
.shortcut-host-picker > p { margin: 0; font-size: 14px; line-height: 1.5; color: var(--muted); }
</style>
