<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Check, ChevronDown, Columns2, Grid2X2, RectangleHorizontal, Rows2 } from '@lucide/vue'
import type { TerminalLayout } from '@/composables/useTerminalWorkspace'
import { useI18n } from '@/i18n'
import { translatePhrase } from '@/i18n/phrase'

const props = defineProps<{ modelValue: TerminalLayout; compact?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: TerminalLayout] }>()
const { locale } = useI18n()
const phrase = (text: string) => { locale.value; return translatePhrase(text) }
const options = [
  { value: 'single', label: '单屏', icon: RectangleHorizontal },
  { value: 'columns', label: '左右分屏', icon: Columns2 },
  { value: 'rows', label: '上下分屏', icon: Rows2 },
  { value: 'grid', label: '四宫格', icon: Grid2X2 },
] as const
const selected = computed(() => options.find((option) => option.value === props.modelValue) ?? options[0])
const root = ref<HTMLElement>()
const trigger = ref<HTMLButtonElement>()
const open = ref(false)
function close(restoreFocus = false): void {
  open.value = false
  if (restoreFocus) trigger.value?.focus()
}
async function show(index = options.findIndex((option) => option.value === props.modelValue)): Promise<void> {
  open.value = true
  await nextTick()
  root.value?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')[index]?.focus()
}
function choose(value: TerminalLayout): void { emit('update:modelValue', value); close(true) }
function keydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') { event.preventDefault(); close(true); return }
  if (event.key === 'Tab') { close(); return }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  if (!open.value) { void show(event.key === 'ArrowUp' || event.key === 'End' ? 3 : 0); return }
  const buttons = [...root.value!.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')]
  const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
  buttons[event.key === 'Home' ? 0 : event.key === 'End' ? 3 : (index + (event.key === 'ArrowUp' ? 3 : 1)) % 4]?.focus()
}
function outside(event: PointerEvent): void {
  if (event.target instanceof Node && !root.value?.contains(event.target)) close()
}
function focusout(event: FocusEvent): void {
  if (!(event.relatedTarget instanceof Node) || !root.value?.contains(event.relatedTarget)) close()
}
onMounted(() => document.addEventListener('pointerdown', outside))
onBeforeUnmount(() => document.removeEventListener('pointerdown', outside))
</script>

<template>
  <div ref="root" class="terminal-layout-controls" @keydown="keydown" @focusout="focusout">
    <button ref="trigger" type="button" class="terminal-layout-trigger" aria-haspopup="menu" :aria-expanded="open"
      :aria-label="`${phrase('终端布局')}: ${phrase(selected.label)}`"
      :title="phrase(compact ? '窗口较小，已显示当前窗格' : '终端布局')" @click="open ? close() : show()">
      <component :is="selected.icon" :size="16" aria-hidden="true" />
      <span>{{ phrase(selected.label) }}</span><ChevronDown class="terminal-layout-chevron" :size="14" aria-hidden="true" />
    </button>
    <div v-if="open" class="terminal-layout-menu" role="menu" :aria-label="phrase('终端布局')">
      <button v-for="option in options" :key="option.value" type="button" role="menuitemradio"
        :data-layout="option.value" :aria-checked="modelValue === option.value" @click="choose(option.value)">
        <component :is="option.icon" :size="16" aria-hidden="true" />
        <span>{{ phrase(option.label) }}</span><Check v-if="modelValue === option.value" :size="14" aria-hidden="true" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.terminal-layout-controls { position:relative; flex:0 0 auto; }
.terminal-layout-trigger, .terminal-layout-menu button { display:flex; align-items:center; gap:8px; min-height:34px; padding:6px 9px; color:var(--terminal-shell-text,#d8dddc); font:inherit; font-size:14px; cursor:pointer; }
.terminal-layout-trigger { border:1px solid var(--terminal-shell-border,#29383a); border-radius:var(--radius-sm); background:var(--terminal-shell-panel,#111a1d); }
.terminal-layout-trigger span { min-width:4em; text-align:left; white-space:nowrap; }
.terminal-layout-menu { position:absolute; z-index:21; top:calc(100% + 6px); right:0; min-width:160px; padding:4px; border:1px solid var(--terminal-shell-border,#29383a); border-radius:var(--radius-sm); background:var(--terminal-shell-panel,#111a1d); box-shadow:var(--shadow-md); }
.terminal-layout-menu button { width:100%; border:0; border-radius:var(--radius-sm); background:transparent; text-align:left; white-space:nowrap; }
.terminal-layout-menu button span { flex:1; }
.terminal-layout-menu button:hover, .terminal-layout-menu button:focus-visible { background:color-mix(in srgb,var(--brand) 15%,transparent); }
.terminal-layout-trigger:focus-visible, .terminal-layout-menu button:focus-visible { outline:2px solid var(--brand); outline-offset:1px; }
@media (max-width: 900px) {
  .terminal-layout-controls { display:none; }
}
</style>
