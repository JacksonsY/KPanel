<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import ModalDialog from '@/components/common/ModalDialog.vue'
import { useVMText } from '@/i18n/virtualMachines'
import { api } from '@/lib/api'
import type { VirtualMachine } from '@/types/virtualMachines'
import type RFB from '@novnc/novnc'

const props = defineProps<{ machine: VirtualMachine }>()
const emit = defineEmits<{ close: [] }>()
const t = useVMText()
const screen = ref<HTMLElement>()
const status = ref('connecting')
const password = ref('')
const needsPassword = ref(false)
let rfb: RFB | undefined
let socket: WebSocket | undefined
let epoch = 0
let timeout: ReturnType<typeof setTimeout> | undefined
function disconnect(): void { epoch++; clearTimeout(timeout); socket?.close(); socket = undefined; rfb?.disconnect(); rfb = undefined; password.value = ''; needsPassword.value = false }
async function connect(): Promise<void> {
  disconnect(); const current = epoch; status.value = 'connecting'
  try {
    const { default: Client } = await import('@novnc/novnc')
    await nextTick()
    if (current !== epoch || !screen.value) return
    const url = new URL(`/api/v1/virtual-machines/${props.machine.id}/console`, location.href)
    url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const channel = new WebSocket(url)
    socket = channel; channel.binaryType = 'arraybuffer'
    timeout = setTimeout(() => { if (current === epoch) { disconnect(); status.value = 'consoleFailed' } }, 15000)
    channel.onopen = () => channel.send(JSON.stringify({ csrf: api.virtualMachines.csrf() }))
    channel.onerror = () => { if (current === epoch) status.value = 'consoleFailed' }
    channel.onclose = () => { if (current === epoch) { clearTimeout(timeout); status.value = 'disconnected' } }
    channel.onmessage = event => {
      if (current !== epoch || typeof event.data !== 'string' || !screen.value) return
      try { if (JSON.parse(event.data).ready !== true) throw new Error('invalid handshake') } catch { disconnect(); status.value = 'consoleFailed'; return }
      channel.onmessage = null
      rfb = new Client(screen.value, channel)
      rfb.scaleViewport = true
      rfb.resizeSession = false
      rfb.addEventListener('connect', () => { if (current === epoch) { clearTimeout(timeout); status.value = 'running' } })
      rfb.addEventListener('disconnect', () => { if (current === epoch) { clearTimeout(timeout); status.value = 'disconnected' } })
      rfb.addEventListener('credentialsrequired', () => { if (current === epoch) { clearTimeout(timeout); needsPassword.value = true } })
      rfb.addEventListener('securityfailure', () => { if (current === epoch) status.value = 'consoleFailed' })
    }
  } catch { if (current === epoch) { disconnect(); status.value = 'consoleFailed' } }
}
function credentials(): void { rfb?.sendCredentials({ password: password.value }); password.value = ''; needsPassword.value = false }
onMounted(() => void connect())
onBeforeUnmount(disconnect)
</script>
<template>
  <ModalDialog open :title="`${t('console')} · ${machine.name}`" variant="workspace" size="wide" @close="emit('close')">
    <div class="vm-console"><div class="vm-console__toolbar"><span role="status">{{ t(status as 'connecting' | 'running' | 'disconnected' | 'consoleFailed') }}</span><button class="button button--secondary" @click="connect">{{ t('reconnect') }}</button><button class="button button--secondary" :disabled="status !== 'running'" @click="rfb?.sendCtrlAltDel()">{{ t('ctrlAltDel') }}</button></div><form v-if="needsPassword" class="vm-console__password" @submit.prevent="credentials"><label>{{ t('password') }}<input v-model="password" type="password" autocomplete="off" maxlength="256" /></label><button class="button button--primary">{{ t('connect') }}</button></form><div ref="screen" class="vm-console__screen" :aria-label="`${t('console')} · ${machine.name}`" /></div>
  </ModalDialog>
</template>
<style scoped>
.vm-console { display: flex; flex-direction: column; height: min(72vh,50rem); min-height: 18rem; background: var(--surface); color: var(--text); font-size: .875rem; }
.vm-console__toolbar,.vm-console__password { display: flex; align-items: center; flex-wrap: wrap; gap: .75rem; padding: .75rem; border-bottom: 1px solid var(--border); }
.vm-console__toolbar>span { flex:1; }
.vm-console__toolbar .button { font-size: .875rem; }
.vm-console__password label { display: flex; flex-wrap: wrap; gap: .5rem; }
.vm-console__screen { flex:1; min-height: 0; overflow: auto; background: #20242a; }
</style>
