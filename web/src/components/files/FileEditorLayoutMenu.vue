<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Check, Columns2, Grid2X2, RectangleHorizontal, Rows2 } from '@lucide/vue'
import type { TerminalLayout } from '@/composables/useTerminalWorkspace'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'

// The editor offers the terminal's four layouts from an icon button that
// matches the other editor tools; the current layout is the trigger icon.
const props = defineProps<{ modelValue: TerminalLayout; cramped?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: TerminalLayout] }>()
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
function phrase(text: string): string {
  phraseCatalogVersion.value
  return translatePhrase(text)
}
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
  if (event.key === 'Escape' && open.value) { event.preventDefault(); event.stopPropagation(); close(true); return }
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
  <div ref="root" class="editor-layout" @keydown="keydown" @focusout="focusout">
    <button ref="trigger" type="button" class="editor-layout__trigger" :class="{ 'is-split': modelValue !== 'single' }"
      aria-haspopup="menu" :aria-expanded="open" :aria-label="`${phrase('编辑器分屏')}: ${phrase(selected.label)}`"
      :title="cramped ? phrase('窗口较小，已显示当前窗格') : `${phrase('编辑器分屏')} · ${phrase(selected.label)}`"
      @click="open ? close() : show()">
      <component :is="selected.icon" :size="18" aria-hidden="true" />
    </button>
    <div v-if="open" class="editor-layout__menu" role="menu" :aria-label="phrase('编辑器分屏')">
      <button v-for="option in options" :key="option.value" type="button" role="menuitemradio"
        :data-layout="option.value" :aria-checked="modelValue === option.value" @click="choose(option.value)">
        <component :is="option.icon" :size="17" aria-hidden="true" />
        <span>{{ phrase(option.label) }}</span><Check v-if="modelValue === option.value" :size="15" aria-hidden="true" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.editor-layout {
  position: relative;
  flex: 0 0 auto;
}
.editor-layout__trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--file-preview-muted);
  background: transparent;
  cursor: pointer;
  transition: color var(--motion-duration-fast) var(--motion-ease-fade), background-color var(--motion-duration-fast) var(--motion-ease-fade);
}
.editor-layout__trigger:hover,
.editor-layout__trigger[aria-expanded='true'] {
  color: var(--file-preview-text);
  background: var(--file-preview-panel-raised);
}
.editor-layout__trigger.is-split {
  color: var(--file-preview-accent);
}
.editor-layout__menu {
  position: absolute;
  z-index: 6;
  top: calc(100% + 6px);
  right: 0;
  display: grid;
  min-width: 168px;
  padding: 4px;
  border: 1px solid var(--file-preview-border);
  border-radius: var(--radius);
  color: var(--file-preview-text);
  background: var(--file-preview-panel);
  box-shadow: var(--shadow-md);
}
.editor-layout__menu button {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 38px;
  padding: 8px 10px;
  border: 0;
  border-radius: var(--radius-sm);
  color: inherit;
  background: transparent;
  font: inherit;
  font-size: 14px;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
}
.editor-layout__menu button span {
  flex: 1;
}
.editor-layout__menu button svg {
  flex: 0 0 auto;
  color: var(--file-preview-muted);
}
.editor-layout__menu button[aria-checked='true'] svg {
  color: var(--file-preview-accent);
}
.editor-layout__menu button:hover,
.editor-layout__menu button:focus-visible {
  background: var(--file-preview-panel-raised);
}
.editor-layout button:focus-visible {
  outline: 2px solid var(--file-preview-accent);
  outline-offset: -2px;
}
</style>
