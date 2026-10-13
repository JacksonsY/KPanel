<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { ChevronDown, Monitor, RefreshCw } from '@lucide/vue'
import OperatingSystemIcon from '@/components/overview/OperatingSystemIcon.vue'
import { api } from '@/lib/api'
import { useI18n } from '@/i18n'
import { detectOperatingSystemIdentity } from '@/lib/operatingSystem'
import { withoutUnsupportedLightNodes } from '@/lib/nodeFeatureHosts'
import { sortClusterHosts, readClusterHostOrder } from '@/lib/clusterHostOrder'
import type { ClusterHost } from '@/types/api'

const props = defineProps<{ modelValue: string; active: boolean; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; validity: [valid: boolean] }>()
const { t } = useI18n()
const hosts = ref<ClusterHost[]>([])
const expanded = ref(false)
const trigger = ref<HTMLButtonElement>()
const loading = ref(false)
const error = ref('')
let controller: AbortController | undefined
const options = computed(() => {
  const local = hosts.value.find(host => host.isLocal)
  const remote = sortClusterHosts(withoutUnsupportedLightNodes(hosts.value), readClusterHostOrder()).filter(host => !host.isLocal && /^[a-f0-9]{32}$/.test(host.id))
  return [
    { id: 'local', name: local?.name || t('terminal.hostKind.local'), host: local, available: true, local: true },
    ...remote.map(host => ({ id: host.id, name: host.name, host, available: host.terminalAvailable, local: false })),
  ]
})
const selected = computed(() => options.value.find(item => item.id === props.modelValue))
const valid = computed(() => props.modelValue === 'local' || (!loading.value && !error.value && !!selected.value?.available))
watch(valid, value => emit('validity', value), { immediate: true })
function identity(host?: ClusterHost) { return detectOperatingSystemIdentity(host?.lastSnapshot?.telemetry) }
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
function select(id: string): void { emit('update:modelValue', id); expanded.value = false; void nextTick(() => trigger.value?.focus()) }
watch(() => props.active, active => { if (active) void load(); else { controller?.abort(); expanded.value = false } }, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <div class="shortcut-host-picker">
    <span class="shortcut-host-picker__label">{{ t('desktop.shortcutHost') }}</span>
    <button ref="trigger" type="button" class="shortcut-host-picker__selected" :disabled="disabled" :aria-label="t('desktop.shortcutHostChoose')" :aria-expanded="expanded" @click="expanded = !expanded">
      <OperatingSystemIcon v-if="selected?.host" :distro="identity(selected.host).key" :label="identity(selected.host).label" :show-tooltip="false" />
      <Monitor v-else :size="24" aria-hidden="true" />
      <span><strong>{{ selected?.name || t('desktop.shortcutHostUnavailable') }}</strong><small>{{ selected?.local ? t('terminal.hostKind.local') : t('terminal.hostKind.panel') }}</small></span>
      <ChevronDown :size="16" aria-hidden="true" />
    </button>
    <fieldset v-if="expanded" class="shortcut-host-picker__options" :disabled="disabled">
      <legend>{{ t('desktop.shortcutHostChoose') }}</legend>
      <p v-if="loading" role="status">{{ t('common.loading') }}</p>
      <label v-for="item in options" :key="item.id" :class="{ 'is-selected': modelValue === item.id, 'is-unavailable': !item.available }">
        <input type="radio" name="shortcut-target-host" :value="item.id" :checked="modelValue === item.id" :disabled="!item.available || (loading && !item.local)" @change="select(item.id)" />
        <OperatingSystemIcon v-if="item.host" :distro="identity(item.host).key" :label="identity(item.host).label" :show-tooltip="false" />
        <Monitor v-else :size="24" aria-hidden="true" />
        <span><strong>{{ item.name }}</strong><small>{{ item.local ? t('terminal.hostKind.local') : item.available ? t('terminal.hostKind.panel') : t('desktop.shortcutHostUnavailable') }}</small></span>
      </label>
    </fieldset>
    <p v-if="error" role="alert">{{ error }} <button type="button" class="button button--ghost" :disabled="disabled || loading" @click="load"><RefreshCw :size="14" />{{ t('common.retry') }}</button></p>
    <p v-else-if="!valid && !loading" role="alert">{{ t('desktop.shortcutHostUnavailable') }}</p>
    <small>{{ t('desktop.shortcutHostHint') }}</small>
  </div>
</template>

<style scoped>
.shortcut-host-picker { display: grid; gap: 8px; min-width: 0; }
.shortcut-host-picker__label { font-size: 14px; font-weight: 600; }
.shortcut-host-picker__selected { display: flex; align-items: center; gap: 12px; width: 100%; min-height: 60px; padding: 10px 13px; text-align: left; color: var(--text); background: var(--surface-subtle); border: 1px solid var(--border); border-radius: var(--radius); cursor: pointer; }
.shortcut-host-picker__selected > span, .shortcut-host-picker__options label > span { display: grid; flex: 1; gap: 3px; min-width: 0; overflow-wrap: anywhere; }
.shortcut-host-picker strong { font-size: 14px; }
.shortcut-host-picker small { color: var(--muted); font-size: 13px; line-height: 1.5; }
.shortcut-host-picker__options { display: grid; gap: 6px; max-height: 240px; overflow-y: auto; margin: 0; padding: 8px; border: 1px solid var(--border); border-radius: var(--radius); }
.shortcut-host-picker__options legend { padding: 0 4px; font-size: 13px; color: var(--muted); }
.shortcut-host-picker__options label { display: flex; align-items: center; gap: 10px; padding: 10px; border-radius: var(--radius-sm); cursor: pointer; }
.shortcut-host-picker__options .is-selected { background: var(--brand-soft); }
.shortcut-host-picker__options .is-unavailable { opacity: .65; cursor: not-allowed; }
.shortcut-host-picker :deep(.os-identity__mark) { width: 28px; height: 28px; flex-shrink: 0; }
.shortcut-host-picker > p, .shortcut-host-picker__options p { margin: 0; font-size: 14px; line-height: 1.5; color: var(--muted); }
</style>
