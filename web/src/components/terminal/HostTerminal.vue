<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import TerminalContextMenu from '@/components/terminal/TerminalContextMenu.vue'
import { api, ApiError } from '@/lib/api'
import { closeTerminalsOnPageHide } from '@/lib/terminalPageHide'
import { hostTerminalAdapter } from '@/lib/terminalAdapters'
import { useTerminalController } from '@/composables/useTerminalController'
import { useI18n } from '@/i18n'

const props = withDefaults(defineProps<{
  sessionId: string
  hostName: string
  initialOffset: number
  visible?: boolean
  autoFocus?: boolean
  retained?: boolean
}>(), { visible: true, autoFocus: true, retained: false })
const emit = defineEmits<{ stateChange: [state: 'connecting' | 'connected' | 'reconnecting' | 'finished'] }>()
const { t } = useI18n()
const host = ref<HTMLElement>()
const clipboardMenu = ref<InstanceType<typeof TerminalContextMenu>>()
const adapter = computed(() => hostTerminalAdapter(props.sessionId))
const {
  terminal, state, pendingLine, canInput,
  focusTerminal, executeCommand, executePresentedCommand, scheduleResize, blockInput,
  containTerminalWheel, terminalTouchScroll, submitPendingLine, handlePendingLineEnter,
} = useTerminalController({
  adapter: () => adapter.value, host, clipboardMenu,
  initialOffset: () => props.initialOffset, inputOpen: () => true,
  visible: () => props.visible, autoFocus: () => props.autoFocus,
})
let closeRequest: Promise<void> | undefined
let closeConfirmed = false
let stopPageHideClose: (() => void) | undefined

function closeSession(): Promise<void> {
  blockInput()
  if (closeConfirmed) return Promise.resolve()
  if (closeRequest) return closeRequest
  const id = props.sessionId
  const request = api.terminals.close(id).then(result => {
    if (!result.closed) throw new Error('Terminal close was not confirmed')
    if (id === props.sessionId) closeConfirmed = true
  }).catch((reason: unknown) => {
    if (reason instanceof ApiError && reason.code === 'terminal_not_found') {
      if (id === props.sessionId) closeConfirmed = true
      return
    }
    throw reason
  }).finally(() => { if (closeRequest === request) closeRequest = undefined })
  closeRequest = request
  return request
}
watch(state, value => emit('stateChange', value === 'error' ? 'reconnecting' : value), { immediate: true })
watch(() => props.sessionId, (_id, previous) => {
  closeConfirmed = false
  closeRequest = undefined
  // A reused temporary viewer must not leave its previous PTY behind.
  if (previous && !previousRetained) void api.terminals.close(previous).catch(() => api.terminals.closeOnUnload(previous))
  previousRetained = props.retained
}, { flush: 'sync' })
let previousRetained = props.retained
watch(() => props.retained, value => { previousRetained = value })
onMounted(() => {
  stopPageHideClose = closeTerminalsOnPageHide(() => closeConfirmed || props.retained ? [] : [props.sessionId])
})
onBeforeUnmount(() => {
  stopPageHideClose?.()
  if (!props.retained) void closeSession().catch(() => undefined)
})
defineExpose({ focusTerminal, executeCommand, executePresentedCommand, scheduleResize, closeSession })
</script>

<template>
  <section class="host-terminal terminal-theme-scope">
    <div
      ref="host"
      class="host-terminal__screen terminal-screen"
      @click="focusTerminal"
      @wheel="containTerminalWheel"
      @touchstart="terminalTouchScroll.start"
      @touchmove="terminalTouchScroll.move"
      @touchend="terminalTouchScroll.end"
      @touchcancel="terminalTouchScroll.end"
      @pointerdown="clipboardMenu?.handleSelectionPointerDown($event)"
      @contextmenu="clipboardMenu?.open($event)"
      @paste.capture="clipboardMenu?.handlePaste($event)"
    >
    </div>
    <form class="host-terminal__composer" @submit.prevent="submitPendingLine">
      <input
        v-model="pendingLine"
        type="text"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        maxlength="8192"
        :placeholder="t('terminal.inputPlaceholder')"
        :disabled="!canInput"
        @keydown.enter="handlePendingLineEnter"
      />
      <button type="submit" :disabled="!canInput">{{ t('terminal.send') }}</button>
    </form>
    <TerminalContextMenu
      ref="clipboardMenu"
      :get-terminal="() => terminal"
      :can-paste="canInput"
    />
  </section>
</template>

<style scoped>
.host-terminal { display:grid; height:100%; grid-template-rows:minmax(0,1fr) auto; min-height:0; overflow:hidden; border:1px solid var(--terminal-shell-border,#29383a); border-radius:var(--terminal-shell-radius,12px); background:var(--terminal-shell-background,#0b1214); box-shadow:var(--terminal-shell-shadow); }
.host-terminal__screen { position:relative; min-width:0; min-height:0; overflow:hidden; overscroll-behavior:contain; padding:0; }
.host-terminal__screen :deep(.xterm) { box-sizing:border-box; height:100%; padding:6px 8px 4px; touch-action:none; }
.host-terminal__screen :deep(.xterm-viewport) { overflow-y:scroll !important; overscroll-behavior:contain; background:transparent; }
.host-terminal__screen :deep(.xterm-scrollable-element) { overscroll-behavior:contain; }
.host-terminal__composer { position:relative; z-index:2; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:8px; padding:9px 10px; border-top:1px solid var(--terminal-shell-border,#29383a); background:var(--terminal-shell-panel,#111a1d); }
.host-terminal__composer input { min-width:0; border:1px solid var(--terminal-shell-border,#29383a); border-radius:8px; padding:9px 11px; color:var(--terminal-shell-text,#d8dddc); background:var(--terminal-shell-background,#0b1214); font:14px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; }
.host-terminal__composer button { border:0; border-radius:8px; padding:0 16px; color:var(--on-brand,#05251c); background:var(--brand-action,#35cba6); font-weight: 700; }
/* Workspace dialogs run the terminal to the window edges. */
:global(.modal-panel--workspace .host-terminal) { border:0; border-radius:0; box-shadow:none; }
</style>
