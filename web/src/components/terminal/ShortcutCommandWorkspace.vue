<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { LoaderCircle, Play, RefreshCw } from '@lucide/vue'
import ShortcutCommandTerminal from '@/components/terminal/ShortcutCommandTerminal.vue'
import { useI18n } from '@/i18n'
import { api } from '@/lib/api'
import { desktopWindowCloseGuardKey, desktopWindowLaunchKey } from '@/lib/desktopRouteKeys'
import type { DesktopShortcut } from '@/types/api'

const route = useRoute()
const { t } = useI18n()
const consumeLaunch = inject(desktopWindowLaunchKey, () => false)
const closeGuards = inject(desktopWindowCloseGuardKey, undefined)
const shortcut = ref<DesktopShortcut>()
const loading = ref(false)
const error = ref('')
const running = ref(false)
let controller: AbortController | undefined
let unregisterCloseGuard: (() => void) | undefined
let disposed = false

async function load(run = false): Promise<void> {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  error.value = ''
  try {
    const id = String(route.params.shortcutId || '')
    if (!/^[a-f0-9]{32}$/.test(id)) { error.value = t('desktop.shortcutCommandMissing'); return }
    const workspace = await api.desktop.workspace(request.signal)
    if (disposed || request.signal.aborted || controller !== request) return
    const item = workspace.available && workspace.shortcuts.find(item => item.id === id && item.targetType === 'command' && item.command)
    if (!item) { error.value = t('desktop.shortcutCommandMissing'); return }
    shortcut.value = { ...item }
    running.value = run
  } catch {
    if (!disposed && !request.signal.aborted) error.value = t('desktop.shortcutCommandLoadFailed')
  } finally {
    if (controller === request) loading.value = false
  }
}

onMounted(() => {
  unregisterCloseGuard = closeGuards?.register(() => {
    disposed = true
    controller?.abort()
    return true
  })
  void load(consumeLaunch())
})
onBeforeUnmount(() => {
  disposed = true
  controller?.abort()
  unregisterCloseGuard?.()
})
</script>

<template>
  <section class="shortcut-command-page terminal-theme-scope">
    <div v-if="loading" class="shortcut-command-page__state" role="status">
      <LoaderCircle class="spin" :size="24" />{{ t('common.loading') }}
    </div>
    <div v-else-if="error" class="shortcut-command-page__state" role="alert">
      <strong>{{ error }}</strong>
      <button class="button" @click="load()"><RefreshCw :size="16" />{{ t('common.retry') }}</button>
    </div>
    <ShortcutCommandTerminal v-else-if="running && shortcut" :name="shortcut.name" :command="shortcut.command || ''" :host-name="t('terminal.hostKind.local')" />
    <div v-else class="shortcut-command-page__state">
      <strong>{{ shortcut?.name }}</strong>
      <p>{{ t('desktop.shortcutCommandIdle') }}</p>
      <button class="button button--primary" @click="load(true)"><Play :size="16" />{{ t('terminal.quickCommandsRun', { name: shortcut?.name || '' }) }}</button>
    </div>
  </section>
</template>

<style scoped>
.shortcut-command-page { display: flex; height: 100%; min-height: 0; flex-direction: column; overflow: hidden; background: var(--terminal-wallpaper-background, var(--terminal-shell-background)); }
.shortcut-command-page__state { display: flex; min-height: 0; flex: 1; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 24px; color: var(--terminal-shell-text); text-align: center; font-size: 14px; }
.shortcut-command-page__state p { margin: 0; max-width: 36em; line-height: 1.5; }
</style>
