<script setup lang="ts">
import { inject, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { LoaderCircle } from '@lucide/vue'
import { desktopWindowCloseGuardKey } from '@/lib/desktopRouteKeys'
import HostTerminal from '@/components/terminal/HostTerminal.vue'
import { api, ApiError } from '@/lib/api'
import { useI18n } from '@/i18n'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import type { TerminalSession } from '@/types/api'

const props = defineProps<{ name: string; command: string; hostName: string; hostId?: string; shortcutId?: string; shortcutVersion?: string; existingSession?: TerminalSession }>()
const { t } = useI18n()
const emit = defineEmits<{ close: [] }>()
const closeGuards = inject(desktopWindowCloseGuardKey, undefined)
let unregisterCloseGuard: (() => void) | undefined
let openingRequest: Promise<void> | undefined
const terminal = ref<InstanceType<typeof HostTerminal>>()
const session = ref<TerminalSession>()
const opening = ref(true)
const closing = ref(false)
const error = ref('')
// Snapshot the command for this terminal session.
const command = props.command
const hostName = props.hostName
const hostId = props.hostId || 'local'
let disposed = false
let submitted = false

function phrase(value: string): string {
  phraseCatalogVersion.value
  return translatePhrase(value)
}

async function reclaim(opened: TerminalSession): Promise<void> {
  const result = await api.terminals.close(opened.sessionId)
  if (!result.closed) throw new Error('Terminal close was not confirmed')
}

async function open(): Promise<void> {
  try {
    const opened = props.existingSession || await (props.shortcutId
      ? api.terminals.open(hostId, 30, 120, { retained: true, shortcutId: props.shortcutId, shortcutVersion: props.shortcutVersion })
      : api.terminals.open(hostId, 30, 120))
    if (disposed) {
      if (!opened.retained) await reclaim(opened).catch(() => api.terminals.closeOnUnload(opened.sessionId))
      return
    }
    session.value = opened
    if (opened.launchUncertain) error.value = t('terminal.launchUncertain')
  } catch (reason) {
    if (!disposed) error.value = reason instanceof ApiError && reason.code === 'terminal_limit'
      ? phrase('已达到终端会话上限，请先关闭不用的终端。')
      : phrase('终端启动失败，请检查主机连接后重新打开。')
  } finally {
    opening.value = false
  }
}

async function onStateChange(state: string): Promise<void> {
  if (props.shortcutId || state !== 'connected' || submitted || disposed || closing.value || !command) return
  // A reconnect must never repeat a command. Mark before awaiting Vue's ref update.
  submitted = true
  await nextTick()
  if (disposed || closing.value) return
  const sent = await terminal.value?.executeCommand(command)
  if (!sent && !disposed && !closing.value) error.value = phrase('命令未发送，请关闭窗口后重新执行。')
}

async function close(): Promise<boolean> {
  if (closing.value) return false
  closing.value = true
  try {
    await openingRequest
    if (session.value) {
      if (terminal.value) await terminal.value.closeSession()
      else await reclaim(session.value)
      session.value = undefined
    }
    disposed = true
    emit('close')
    return true
  } catch {
    error.value = phrase('终端关闭失败，请重试关闭。')
    return false
  } finally {
    closing.value = false
  }
}

onMounted(() => {
  unregisterCloseGuard = closeGuards?.register(close)
  openingRequest = open()
})
onBeforeUnmount(() => {
  unregisterCloseGuard?.()
  disposed = true
  // HostTerminal owns mounted sessions, including page-hide cleanup.
  if (session.value && !session.value.retained && !terminal.value) {
    const opened = session.value
    void reclaim(opened).catch(() => api.terminals.closeOnUnload(opened.sessionId))
  }
})
</script>

<template>
  <section class="shortcut-command-terminal" :aria-label="name">
      <div v-if="opening" class="shortcut-command-terminal__state" role="status"><LoaderCircle class="spin" :size="20" />{{ phrase('正在连接终端…') }}</div>
      <div v-if="error" class="inline-alert inline-alert--danger" role="alert">{{ error }}</div>
      <HostTerminal v-if="session" ref="terminal" :session-id="session.sessionId" :retained="session.retained" :host-name="hostName" :initial-offset="session.retained ? 0 : session.offset" @state-change="onStateChange" />
  </section>
</template>

<style scoped>
.shortcut-command-terminal { height: 100%; overflow: hidden; background: var(--terminal-wallpaper-background, var(--terminal-shell-background)); display: flex; flex: 1 1 auto; min-height: 0; flex-direction: column; }
.shortcut-command-terminal__state { display: flex; flex: 1; align-items: center; justify-content: center; gap: 8px; color: var(--muted); font-size: 14px; }
.shortcut-command-terminal > .inline-alert { margin: 16px; }
.shortcut-command-terminal > :deep(.host-terminal) { flex: 1 1 auto; height: auto; min-width: 0; border: 0; border-radius: 0; box-shadow: none; }
</style>
