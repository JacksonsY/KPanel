<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { History, LoaderCircle, Play, RefreshCw, Search, Trash2, X } from '@lucide/vue'
import ModalDialog from '@/components/common/ModalDialog.vue'
import { useI18n } from '@/i18n'
import { useTerminalHistory } from '@/stores/terminalHistory'
import { useToast } from '@/stores/toast'
import type { TerminalHistoryEntry } from '@/types/api'

const props = defineProps<{
  active: boolean
  disabled?: boolean
  mode?: 'interactive' | 'batch'
  hostNames?: Record<string, string>
}>()

const emit = defineEmits<{
  execute: [command: string]
  fill: [command: string]
}>()

const { locale, t } = useI18n()
const toast = useToast()
const historyStore = useTerminalHistory()
const query = ref('')
const loadError = ref('')
const clearOpen = ref(false)
let loadController: AbortController | undefined

const snapshot = computed(() => historyStore.history.value)
const items = computed(() => snapshot.value.items)
const batch = computed(() => props.mode === 'batch')
const visibleItems = computed(() => {
  const needle = query.value.trim().toLowerCase()
  if (!needle) return items.value
  return items.value.filter((item) => item.command.toLowerCase().includes(needle)
    || hostLabel(item).toLowerCase().includes(needle))
})

async function reload(): Promise<void> {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  loadError.value = ''
  try {
    await historyStore.load(controller.signal)
  } catch (error) {
    if (controller.signal.aborted) return
    if (!historyStore.loaded.value) loadError.value = t('terminal.historyLoadFailed')
  }
}

// Reload on every visit so commands run from other browsers show up.
watch(() => props.active, (active) => {
  if (active) void reload()
  else {
    loadController?.abort()
    clearOpen.value = false
  }
}, { immediate: true })

function hostLabel(item: TerminalHistoryEntry): string {
  if (!item.hostId) return t('terminal.historyBatch')
  return props.hostNames?.[item.hostId] || t('terminal.historyOtherHost')
}

function timeLabel(at: number): string {
  const date = new Date(at)
  const sameDay = date.toDateString() === new Date().toDateString()
  return new Intl.DateTimeFormat(locale.value, sameDay
    ? { hour: '2-digit', minute: '2-digit' }
    : { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date)
}

function primaryTitle(item: TerminalHistoryEntry): string {
  return `${t(batch.value ? 'terminal.historyFillBatch' : 'terminal.historyFill')}\n${item.command}`
}

// Batch mode always fills the command box; the interactive terminal gets the
// command in its input box so the user reviews it before it runs.
function choose(item: TerminalHistoryEntry): void {
  if (props.disabled) return
  if (batch.value) emit('execute', item.command)
  else emit('fill', item.command)
}

async function remove(item: TerminalHistoryEntry): Promise<void> {
  try {
    await historyStore.remove(item.command)
  } catch {
    toast.danger(t('terminal.historyDeleteFailed'))
  }
}

async function clearHistory(): Promise<void> {
  try {
    await historyStore.clear()
    clearOpen.value = false
    toast.success(t('terminal.historyCleared'))
  } catch {
    clearOpen.value = false
    toast.danger(t('terminal.historyClearFailed'))
  }
}
</script>

<template>
  <div class="terminal-command-history">
    <div class="terminal-command-history__toolbar">
      <label class="terminal-command-history__search">
        <Search :size="15" aria-hidden="true" />
        <span class="sr-only">{{ t('terminal.historySearch') }}</span>
        <input
          v-model="query"
          type="search"
          autocomplete="off"
          autocapitalize="off"
          spellcheck="false"
          :placeholder="t('terminal.historySearch')"
        />
      </label>
      <button
        type="button"
        class="terminal-command-history__clear"
        :title="t('terminal.historyClear')"
        :aria-label="t('terminal.historyClear')"
        :disabled="!items.length || historyStore.saving.value"
        @click="clearOpen = true"
      >
        <Trash2 :size="16" />
      </button>
    </div>
    <p v-if="historyStore.syncFailed.value && snapshot.available" class="terminal-command-history__notice" role="status">
      {{ t('terminal.historySyncFailed') }}
    </p>

    <div v-if="historyStore.loading.value && !historyStore.loaded.value" class="terminal-command-history__state">
      <LoaderCircle class="spin" :size="20" />
      <span>{{ t('terminal.historyLoading') }}</span>
    </div>
    <div v-else-if="loadError" class="terminal-command-history__state" role="alert">
      <span>{{ loadError }}</span>
      <button type="button" class="terminal-command-history__action" @click="reload">
        <RefreshCw :size="15" />{{ t('terminal.quickCommandsRetry') }}
      </button>
    </div>
    <div v-else-if="!snapshot.available" class="terminal-command-history__state" role="alert">
      <strong>{{ t('terminal.historyUnavailableTitle') }}</strong>
      <span>{{ t('terminal.historyUnavailableMessage') }}</span>
      <button type="button" class="terminal-command-history__action" :disabled="historyStore.saving.value" @click="clearHistory">
        <Trash2 :size="15" />{{ t('terminal.historyRecover') }}
      </button>
    </div>
    <div v-else-if="!items.length" class="terminal-command-history__state">
      <span class="terminal-command-history__empty-icon"><History :size="20" /></span>
      <strong>{{ t('terminal.historyEmptyTitle') }}</strong>
      <span>{{ t('terminal.historyEmptyDescription') }}</span>
    </div>
    <div v-else-if="!visibleItems.length" class="terminal-command-history__state">
      <span>{{ t('terminal.historyNoMatch') }}</span>
    </div>
    <ul v-else class="terminal-command-history__list">
      <li v-for="item in visibleItems" :key="item.command" class="terminal-history-entry">
        <button
          type="button"
          class="terminal-history-entry__fill"
          :disabled="disabled"
          :title="primaryTitle(item)"
          @click="choose(item)"
        >
          <code>{{ item.command }}</code>
          <small>
            <span>{{ hostLabel(item) }}</span>
            <time :datetime="new Date(item.at).toISOString()">{{ timeLabel(item.at) }}</time>
          </small>
        </button>
        <div class="terminal-history-entry__actions">
          <button
            v-if="!batch"
            type="button"
            :title="t('terminal.historyRun')"
            :aria-label="t('terminal.historyRun')"
            :disabled="disabled"
            @click="emit('execute', item.command)"
          >
            <Play :size="14" fill="currentColor" />
          </button>
          <button
            type="button"
            :title="t('terminal.historyDelete')"
            :aria-label="t('terminal.historyDelete')"
            @click="remove(item)"
          >
            <X :size="14" />
          </button>
        </div>
      </li>
    </ul>

    <ModalDialog
      :open="clearOpen"
      :title="t('terminal.historyClear')"
      size="compact"
      :close-disabled="historyStore.saving.value"
      @close="clearOpen = false"
    >
      <p class="terminal-command-history__confirm">
        {{ t('terminal.historyClearMessage', { count: items.length }) }}
      </p>
      <template #footer>
        <button class="button button--ghost" type="button" :disabled="historyStore.saving.value" @click="clearOpen = false">
          {{ t('common.cancel') }}
        </button>
        <button class="button button--danger" type="button" :disabled="historyStore.saving.value" @click="clearHistory">
          {{ t('terminal.historyClear') }}
        </button>
      </template>
    </ModalDialog>
  </div>
</template>

<style scoped>
.terminal-command-history { display:flex; min-width:0; min-height:0; flex-direction:column; }
.terminal-command-history__toolbar { display:flex; align-items:center; gap:6px; padding:8px 8px 4px; }
.terminal-command-history__search { display:flex; min-width:0; flex:1; align-items:center; gap:7px; border:1px solid var(--terminal-shell-border,#29383a); border-radius:var(--radius-sm); padding:0 9px; color:var(--terminal-shell-muted,#8a9695); background:var(--terminal-shell-background,#0b1214); }
.terminal-command-history__search:focus-within { border-color:var(--brand); box-shadow:0 0 0 2px color-mix(in srgb,var(--brand) 20%,transparent); }
.terminal-command-history__search>svg { flex:0 0 auto; }
.terminal-command-history__search input { min-width:0; flex:1; border:0; padding:7px 0; color:var(--terminal-shell-text,#d8dddc); background:transparent; font:inherit; font-size:14px; outline:none; }
.terminal-command-history__clear,.terminal-history-entry__actions button { display:grid; flex:0 0 auto; width:30px; height:30px; place-items:center; border:1px solid var(--terminal-shell-border,#29383a); border-radius:var(--radius-sm); color:var(--terminal-shell-muted,#8a9695); background:var(--terminal-shell-background,#0b1214); cursor:pointer; }
.terminal-command-history button:hover:not(:disabled),.terminal-command-history button:focus-visible { border-color:var(--brand); color:var(--terminal-shell-text,#d8dddc); outline:none; }
.terminal-command-history button:focus-visible { box-shadow:0 0 0 2px color-mix(in srgb,var(--brand) 20%,transparent); }
.terminal-command-history button:disabled { cursor:not-allowed; opacity:.48; }
.terminal-command-history__notice { margin:4px 8px 0; border:1px solid color-mix(in srgb,var(--warning,#d6a23c) 45%,transparent); border-radius:var(--radius-sm); padding:6px 9px; color:var(--terminal-shell-text,#d8dddc); background:color-mix(in srgb,var(--warning,#d6a23c) 10%,transparent); font-size:13px; line-height:1.45; }
.terminal-command-history__list { min-height:0; flex:1; overflow-y:auto; overscroll-behavior:contain; margin:0; padding:4px 8px 8px; list-style:none; scrollbar-color:var(--terminal-shell-scrollbar,#35474a) var(--terminal-shell-panel,#111a1d); scrollbar-width:thin; }
.terminal-history-entry { display:flex; min-width:0; align-items:flex-start; gap:5px; margin-bottom:5px; border:1px solid transparent; border-radius:var(--radius-sm); padding:4px; }
.terminal-history-entry:hover,.terminal-history-entry:focus-within { border-color:color-mix(in srgb,var(--brand) 24%,var(--terminal-shell-border,#29383a)); background:color-mix(in srgb,var(--brand) 6%,transparent); }
.terminal-history-entry__fill { display:grid; min-width:0; flex:1; gap:3px; border:1px solid transparent; border-radius:var(--radius-sm); padding:5px 7px; color:var(--terminal-shell-text,#d8dddc); background:transparent; font:inherit; text-align:left; cursor:pointer; }
.terminal-history-entry__fill code { display:-webkit-box; overflow:hidden; font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; font-size:13px; line-height:1.45; white-space:pre-wrap; word-break:break-all; -webkit-box-orient:vertical; -webkit-line-clamp:3; line-clamp:3; }
.terminal-history-entry__fill small { display:flex; min-width:0; flex-wrap:wrap; gap:0 8px; color:var(--terminal-shell-muted,#8a9695); font-size:13px; line-height:1.4; }
.terminal-history-entry__fill small span { min-width:0; overflow-wrap:anywhere; }
.terminal-history-entry__actions { display:flex; flex:0 0 auto; gap:5px; padding-top:2px; opacity:.72; }
.terminal-history-entry:hover .terminal-history-entry__actions,.terminal-history-entry:focus-within .terminal-history-entry__actions { opacity:1; }
.terminal-history-entry__actions button { width:28px; height:28px; }
.terminal-history-entry__actions button:first-child:not(:last-child) { color:var(--brand); }
.terminal-command-history__state { display:flex; min-height:0; flex:1; flex-direction:column; align-items:center; justify-content:center; gap:10px; padding:24px 18px; color:var(--terminal-shell-muted,#8a9695); font-size:13px; line-height:1.5; text-align:center; }
.terminal-command-history__state strong { color:var(--terminal-shell-text,#d8dddc); font-size:14px; }
.terminal-command-history__empty-icon { display:grid; width:42px; height:42px; place-items:center; border-radius:var(--radius-md); color:var(--brand); background:color-mix(in srgb,var(--brand) 11%,var(--terminal-shell-background,#0b1214)); }
.terminal-command-history__action { display:flex; align-items:center; gap:6px; border:1px solid var(--terminal-shell-border,#29383a); border-radius:var(--radius-sm); padding:7px 10px; color:var(--terminal-shell-text,#d8dddc); background:var(--terminal-shell-background,#0b1214); font:inherit; font-size:13px; cursor:pointer; }
.terminal-command-history__confirm { margin:0; color:var(--text-muted); font-size:14px; line-height:1.55; }
</style>
