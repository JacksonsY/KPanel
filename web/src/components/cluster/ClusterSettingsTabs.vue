<script setup lang="ts">
import { useI18n } from '@/i18n'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import type { ClusterSettingsTab } from '@/lib/clusterSettings'
const props = defineProps<{ modelValue: ClusterSettingsTab; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [tab: ClusterSettingsTab] }>()
const { locale } = useI18n()
const tabs = [{ id: 'notifications', label: '通知设置' }, { id: 'share', label: '公开分享' }, { id: 'remote', label: '远程操作' }] as const
function phrase(value: string): string { locale.value; phraseCatalogVersion.value; return translatePhrase(value) }
function onKey(event: KeyboardEvent, index: number): void {
  if (props.disabled || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? 2 : (index + (event.key === 'ArrowRight' ? 1 : 2)) % 3
  const tab = tabs[next]!
  emit('update:modelValue', tab.id)
  const target = event.currentTarget as HTMLElement
  target.parentElement?.querySelector<HTMLButtonElement>(`#cluster-settings-tab-${tab.id}`)?.focus()
}
</script>
<template>
  <nav class="cluster-settings-tabs" role="tablist" :aria-label="phrase('集群设置')">
    <button v-for="(tab, index) in tabs" :id="`cluster-settings-tab-${tab.id}`" :key="tab.id" type="button" role="tab" :aria-selected="modelValue === tab.id" :aria-controls="`cluster-settings-${tab.id}`" :tabindex="modelValue === tab.id ? 0 : -1" :disabled="disabled" @click="emit('update:modelValue', tab.id)" @keydown="onKey($event, index)">{{ phrase(tab.label) }}</button>
  </nav>
</template>
<style scoped>
.cluster-settings-tabs { display:flex; flex-wrap:wrap; gap:8px; margin-bottom:22px; }
.cluster-settings-tabs button { min-height:36px; padding:7px 12px; border:1px solid var(--border); border-radius:999px; background:var(--surface); color:var(--text-soft); font:inherit; font-size:14px; cursor:pointer; }
.cluster-settings-tabs button[aria-selected="true"] { border-color:var(--brand); background:var(--brand-soft); color:var(--brand-strong); }
.cluster-settings-tabs button:disabled { opacity:.6; cursor:default; }
.cluster-settings-tabs button:focus-visible { outline:2px solid var(--brand); outline-offset:2px; }
@media(pointer:coarse) { .cluster-settings-tabs button { min-height:44px; } }
</style>
