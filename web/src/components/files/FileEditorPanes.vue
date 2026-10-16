<script setup lang="ts" generic="T extends EditorPaneTab">
import { computed, onBeforeUnmount, onMounted, ref, toRef, watch } from 'vue'
import { FolderOpen, Maximize2, Minimize2, X } from '@lucide/vue'
import FileEntryIcon from './FileEntryIcon.vue'
import { useTerminalWorkspace, type TerminalLayout } from '@/composables/useTerminalWorkspace'
import { useSplitPaneGeometry } from '@/composables/useSplitPaneGeometry'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import type { EditorPaneTab } from './fileEditorPanes'

// Pane assignment is the terminal's: each open file shows in at most one pane,
// a hidden tab swaps into the active pane and a new tab fills an empty one.
const props = defineProps<{
  tabs: T[]
  activePath: string
  /** False on phones and narrow editors: the active file fills the editor. */
  splitAvailable: boolean
  canChooseFile: boolean
  idPrefix: string
}>()
const emit = defineEmits<{ activate: [path: string]; close: [tab: T]; chooseFile: [] }>()
defineSlots<{ default(props: { tab: T }): unknown }>()
const paths = computed(() => props.tabs.map((tab) => tab.entry.path))
const { layout, panes, activePane, maximized, paneCount, selectSession, setLayout } = useTerminalWorkspace(paths, toRef(props, 'activePath'))
const root = ref<HTMLElement>()
// A desktop window can still be too small for two readable code panes.
const cramped = ref(false)
const singleVisible = computed(() => !props.splitAvailable || cramped.value || maximized.value || layout.value === 'single')
const showHeaders = computed(() => layout.value !== 'single' && props.splitAvailable && !cramped.value)
const activeEmpty = computed(() => !panes.value[activePane.value])
const {
  grid, columnRatio, rowRatio, minimumColumnRatio, minimumRowRatio, hasColumns, hasRows, gridStyle,
  isVisible, cellStyle, measure, startDrag, moveDrag, endDrag, resizeWithKeyboard, resetRatio,
} = useSplitPaneGeometry({ layout, singleVisible, activePane, paneCount, minimumWidth: 260, minimumHeight: 120 })
let observer: ResizeObserver | undefined

function phrase(text: string): string {
  phraseCatalogVersion.value
  return translatePhrase(text)
}
function paneOf(tab: T): number {
  return panes.value.indexOf(tab.entry.path)
}
function isShown(path: string): boolean {
  return isVisible(panes.value.indexOf(path))
}
function activate(index: number): void {
  if (index < 0 || index >= paneCount.value) return
  activePane.value = index
  const path = panes.value[index]
  if (path && path !== props.activePath) emit('activate', path)
}
function chooseFile(index: number): void {
  activate(index)
  emit('chooseFile')
}
function toggleMaximized(index: number): void {
  activate(index)
  maximized.value = !maximized.value
}
function applyLayout(next: TerminalLayout): void {
  setLayout(next)
  const path = panes.value[activePane.value]
  if (path && path !== props.activePath) emit('activate', path)
}
// One visible pane must show a file: an empty pane chosen in the split hands
// focus back to the active file when the editor narrows or goes to a phone.
watch(singleVisible, (single) => {
  if (single && activeEmpty.value) selectSession(props.activePath)
})
onMounted(() => {
  if (typeof ResizeObserver === 'undefined') return
  observer = new ResizeObserver(() => {
    if (!root.value || root.value.clientWidth <= 0 || root.value.clientHeight <= 0) return
    cramped.value = root.value.clientWidth < 560 || root.value.clientHeight < 240
    measure()
  })
  if (root.value) observer.observe(root.value)
  if (grid.value) observer.observe(grid.value)
})
onBeforeUnmount(() => observer?.disconnect())
defineExpose({ layout, setLayout: applyLayout, selectSession, activeEmpty, cramped, isShown })
</script>

<template>
  <div ref="root" class="editor-panes" :class="{ 'is-split': !singleVisible }">
    <div ref="grid" class="editor-panes__grid" :style="gridStyle">
      <!-- Only shown files keep a live editor; hidden tabs park their session. -->
      <template v-for="(tab, index) in tabs" :key="tab.entry.path">
        <section v-if="isVisible(paneOf(tab))" :id="`${idPrefix}-panel-${index}`" class="editor-pane"
          :class="{ 'is-active': !singleVisible && tab.entry.path === activePath && panes[activePane] === tab.entry.path }"
          :style="cellStyle(paneOf(tab))" role="tabpanel" :aria-labelledby="`${idPrefix}-tab-${index}`"
          :data-pane-path="tab.entry.path" @pointerdown="activate(paneOf(tab))" @focusin="activate(paneOf(tab))">
          <header v-if="showHeaders" class="editor-pane__header">
            <FileEntryIcon :entry="tab.entry" :size="16" />
            <span class="editor-pane__title" :title="tab.entry.path" data-i18n-ignore>{{ tab.entry.name }}</span>
            <span v-if="tab.dirty" class="editor-pane__dirty" role="img" :aria-label="phrase('未保存')" />
            <button type="button" :aria-label="phrase(maximized ? '恢复分屏' : '最大化窗格')" :title="phrase(maximized ? '恢复分屏' : '最大化窗格')"
              @click="toggleMaximized(paneOf(tab))">
              <Minimize2 v-if="maximized" :size="15" /><Maximize2 v-else :size="15" />
            </button>
            <button type="button" :disabled="tab.saving" :aria-label="phrase(`关闭 ${tab.entry.name}`)" :title="phrase('关闭标签')"
              @click="emit('close', tab)"><X :size="15" /></button>
          </header>
          <slot :tab="tab" />
        </section>
      </template>
      <template v-for="(path, index) in panes" :key="`empty-${index}`">
        <div v-if="!path && isVisible(index)" class="editor-pane editor-pane--empty" :class="{ 'is-active': activePane === index }"
          :style="cellStyle(index)" :data-pane-index="index" @pointerdown="activate(index)">
          <button v-if="canChooseFile" type="button" @click="chooseFile(index)"><FolderOpen :size="17" />{{ phrase('从文件列表打开') }}</button>
          <small>{{ phrase('已打开的文件可从顶部标签切换到此窗格') }}</small>
        </div>
      </template>
      <div v-if="hasColumns" class="editor-panes__divider is-column" role="separator" tabindex="0" aria-orientation="vertical"
        :aria-label="phrase('调整左右分屏比例')" :aria-valuenow="Math.round(columnRatio)" :aria-valuemin="Math.ceil(minimumColumnRatio)" :aria-valuemax="Math.floor(100 - minimumColumnRatio)"
        @pointerdown.prevent="startDrag('column', $event)" @pointermove="moveDrag" @pointerup="endDrag" @pointercancel="endDrag" @lostpointercapture="endDrag"
        @keydown="resizeWithKeyboard('column', $event)" @dblclick="resetRatio('column')" />
      <div v-if="hasRows" class="editor-panes__divider is-row" role="separator" tabindex="0" aria-orientation="horizontal"
        :aria-label="phrase('调整上下分屏比例')" :aria-valuenow="Math.round(rowRatio)" :aria-valuemin="Math.ceil(minimumRowRatio)" :aria-valuemax="Math.floor(100 - minimumRowRatio)"
        @pointerdown.prevent="startDrag('row', $event)" @pointermove="moveDrag" @pointerup="endDrag" @pointercancel="endDrag" @lostpointercapture="endDrag"
        @keydown="resizeWithKeyboard('row', $event)" @dblclick="resetRatio('row')" />
    </div>
  </div>
</template>

<style scoped>
.editor-panes {
  display: grid;
  flex: 1;
  grid-template-rows: minmax(0, 1fr);
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.editor-panes__grid {
  display: grid;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.editor-pane {
  position: relative;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
/* The active pane carries the accent once: a hairline frame plus its header rule. */
.editor-pane.is-active {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--file-preview-accent) 70%, transparent);
}
.editor-pane__header {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 2px 4px 2px 10px;
  border-top: 2px solid transparent;
  border-bottom: 1px solid var(--file-preview-border);
  color: var(--file-preview-muted);
  background: var(--file-preview-panel);
  font-size: 14px;
  transition: color var(--motion-duration-fast) var(--motion-ease-fade), border-color var(--motion-duration-fast) var(--motion-ease-fade);
}
.editor-pane.is-active .editor-pane__header {
  border-top-color: var(--file-preview-accent);
  color: var(--file-preview-text);
}
.editor-pane__header svg {
  flex: 0 0 auto;
}
.editor-pane__title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.editor-pane__dirty {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--file-preview-accent);
}
.editor-pane__header button {
  display: grid;
  flex: 0 0 28px;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--file-preview-muted);
  background: transparent;
  cursor: pointer;
}
.editor-pane__header button:hover:not(:disabled) {
  color: var(--file-preview-text);
  background: var(--file-preview-panel-raised);
}
.editor-pane__header button:disabled {
  opacity: 0.5;
  cursor: default;
}
.editor-pane--empty {
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 16px;
  color: var(--file-preview-muted);
  background: var(--file-preview-background);
  text-align: center;
}
.editor-pane--empty button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 38px;
  padding: 8px 14px;
  border: 1px solid var(--file-preview-border);
  border-radius: var(--radius-sm);
  color: var(--file-preview-text);
  background: var(--file-preview-panel);
  font: inherit;
  font-size: 14px;
  cursor: pointer;
}
.editor-pane--empty button:hover {
  border-color: var(--file-preview-accent);
  background: var(--file-preview-panel-raised);
}
.editor-pane--empty small {
  max-width: 28em;
  font-size: 13px;
  line-height: 1.5;
}
.editor-panes__divider {
  position: relative;
  z-index: 1;
  background: var(--file-preview-panel);
  touch-action: none;
}
.editor-panes__divider::after {
  position: absolute;
  content: '';
  background: var(--file-preview-border);
  transition: background-color var(--motion-duration-fast) var(--motion-ease-fade);
}
.editor-panes__divider.is-column {
  grid-column: 2;
  grid-row: 1 / -1;
  cursor: col-resize;
}
.editor-panes__divider.is-column::after {
  top: 0;
  bottom: 0;
  left: 3px;
  width: 2px;
}
.editor-panes__divider.is-row {
  grid-column: 1 / -1;
  grid-row: 2;
  cursor: row-resize;
}
.editor-panes__divider.is-row::after {
  top: 3px;
  right: 0;
  left: 0;
  height: 2px;
}
.editor-panes__divider:hover::after,
.editor-panes__divider:focus-visible::after {
  background: var(--file-preview-accent);
}
.editor-panes button:focus-visible,
.editor-panes__divider:focus-visible {
  outline: 2px solid var(--file-preview-accent);
  outline-offset: -2px;
}
</style>
