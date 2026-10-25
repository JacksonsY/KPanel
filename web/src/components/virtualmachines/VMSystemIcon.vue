<script setup lang="ts">
import { computed } from 'vue'
import { Monitor } from '@lucide/vue'
import OperatingSystemIcon from '@/components/overview/OperatingSystemIcon.vue'
import { useVMText } from '@/i18n/virtualMachines'
import { virtualMachineSystem } from '@/lib/virtualMachineSystem'

const props = defineProps<{ osId?: string }>()
const t = useVMText()
const system = computed(() => virtualMachineSystem(props.osId))
</script>

<template>
  <span class="vm-system-icon" :title="system ? `${t('declaredSystem')}：${system.label}` : t('unknownSystem')">
    <OperatingSystemIcon v-if="system" :distro="system.distro" :label="system.label" :show-tooltip="false" />
    <Monitor v-else :size="28" aria-hidden="true" />
  </span>
</template>

<style scoped>
.vm-system-icon { display: inline-flex; flex: 0 0 auto; align-items: center; justify-content: center; width: 28px; height: 28px; }
</style>
