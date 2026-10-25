<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref, watch } from 'vue'
import { Monitor, Plus, RefreshCw, Search } from '@lucide/vue'
import ModalDialog from '@/components/common/ModalDialog.vue'
import VMConsole from '@/components/virtualmachines/VMConsole.vue'
import { useVMText, type VMTextKey } from '@/i18n/virtualMachines'
import { api } from '@/lib/api'
import { usePhraseCatalog } from '@/i18n/phrase'
usePhraseCatalog(locale => locale === 'en-US' ? import('@/i18n/pages/VirtualMachinesView/en-US').then(m => m.default) : import('@/i18n/pages/VirtualMachinesView/zh-TW').then(m => m.default))
import type { VMJob, VMRequest, VMSnapshot, VirtualMachine } from '@/types/virtualMachines'

const t = useVMText()
const snapshot = ref<VMSnapshot>()
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const notice = ref('')
const search = ref('')
const filter = ref('all')
const wizard = ref(false)
const step = ref(0)
const loginMethod = ref('password')
const formError = ref('')
const installOpen = ref(false)
const detailID = ref('')
const consoleMachine = ref<VirtualMachine>()
const selectedJob = ref<VMJob>()
const confirmation = ref<{ action: string; machine: VirtualMachine }>()
const deleteDisks = ref(false)
const form = reactive({ name: '', source: 'cloud', image: 'debian-12', cpus: 2, memoryMiB: 2048, diskGiB: 20, pool: 'default', network: 'default', username: 'kpanel', publicKey: '', password: '', autostart: false })
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
let active = true
let requestCache: { value: string; id: string } | undefined
const pending = computed(() => snapshot.value?.jobs.some(job => ['queued', 'running'].includes(job.status)) ?? false)
const writable = computed(() => snapshot.value?.environment.writable && !pending.value && !submitting.value)
const machines = computed(() => (snapshot.value?.machines ?? []).filter(m => (filter.value === 'all' || m.state === filter.value) && m.name.toLowerCase().includes(search.value.toLowerCase())))
const detail = computed(() => snapshot.value?.machines.find(m => m.id === detailID.value))
const isoImages = computed(() => snapshot.value?.images.filter(image => image.path === form.pool) ?? [])
const steps = computed(() => [t('sourceStep'), t('settingsStep'), t('reviewStep')])
const envReason = computed(() => {
  const reason = snapshot.value?.environment.reason
  return reason && ['dependencies_missing', 'platform_unsupported', 'kvm_unavailable'].includes(reason) ? t(reason as VMTextKey) : t('vm_unavailable')
})
function state(value: string): string {
  const keys: Record<string, VMTextKey> = { running: 'running', 'shut off': 'stopped', paused: 'paused', queued: 'queued', succeeded: 'succeeded', failed: 'failed', needs_attention: 'needs_attention' }
  return keys[value] ? t(keys[value]!) : value || t('unknown')
}
function stage(value: string): string {
  const values = ['queued', 'checking', 'installing', 'configuring', 'preparing_disk', 'downloading', 'creating', 'verifying', 'applying', 'completed']
  return values.includes(value) ? t(value as VMTextKey) : value
}
function actionLabel(value: string): string {
  if (value === 'install') return t('installAction')
  if (value === 'create') return t('createAction')
  return t(value as VMTextKey)
}
function errorText(reason: unknown): string {
  const code = reason && typeof reason === 'object' && 'code' in reason ? String(reason.code) : ''
  if (['vm_conflict', 'vm_busy', 'vm_write_disabled', 'vm_unsupported', 'vm_invalid_request', 'vm_result_unknown', 'vm_unavailable'].includes(code)) return t(code as VMTextKey)
  return reason instanceof Error ? reason.message : t('operationFailed')
}
async function load(): Promise<void> {
  clearTimeout(timer)
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  try {
    const result = await api.virtualMachines.list(request.signal)
    if (!active || request.signal.aborted) return
    snapshot.value = result
    error.value = ''
  } catch (reason) {
    if (!request.signal.aborted && active) error.value = errorText(reason)
  } finally {
    if (request === controller) {
      loading.value = false
      if (active) timer = setTimeout(() => { if (!document.hidden) void load(); else schedule() }, pending.value ? 3000 : 15000)
    }
  }
}
function schedule(): void { if (active) { clearTimeout(timer); timer = setTimeout(() => { if (!document.hidden) void load(); else schedule() }, 15000) } }
function openCreate(): void {
  step.value = 0; formError.value = ''; notice.value = ''
  form.pool = snapshot.value?.pools.find(p => p.name === 'default')?.name || snapshot.value?.pools[0]?.name || ''
  form.network = snapshot.value?.networks.find(n => n.name === 'default')?.name || snapshot.value?.networks[0]?.name || ''
  wizard.value = true
}
function validStep(): boolean {
  formError.value = ''
  if (step.value === 0) {
    if (!form.pool || !form.network) formError.value = t('noResources')
    else if (form.source === 'iso' && !isoImages.value.some(i => i.name === form.image)) formError.value = t('noISO')
  } else {
    if (!/^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$/.test(form.name)) formError.value = t('invalidName')
    else if (form.source === 'cloud' && !/^[a-z][a-z0-9_-]{0,30}$/.test(form.username)) formError.value = t('keyRequired')
    else if (form.source === 'cloud' && loginMethod.value === 'key' && !/^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp256) [A-Za-z0-9+/=]+(?: [^\r\n]*)?$/.test(form.publicKey.trim())) formError.value = t('keyRequired')
    else if (form.source === 'cloud' && loginMethod.value === 'password' && (new TextEncoder().encode(form.password).length < 12 || new TextEncoder().encode(form.password).length > 72 || /[\r\n\x00]/.test(form.password))) formError.value = t('passwordHint')
    else if (![form.cpus, form.memoryMiB, form.diskGiB].every(Number.isInteger) || form.cpus < 1 || form.cpus > 256 || form.memoryMiB < 256 || form.memoryMiB > 1048576 || form.diskGiB < 4 || form.diskGiB > 65536) formError.value = t('vm_invalid_request')
  }
  return !formError.value
}
async function submit(input: Omit<VMRequest, 'requestId'>): Promise<void> {
  if (submitting.value) return
  submitting.value = true; formError.value = ''; error.value = ''
  const value = JSON.stringify(input)
  if (requestCache?.value !== value) requestCache = { value, id: crypto.randomUUID() }
  try {
    await api.virtualMachines.action({ ...input, requestId: requestCache.id })
    requestCache = undefined
    if (!active) return
    wizard.value = false; installOpen.value = false; confirmation.value = undefined
    notice.value = t('resultAccepted')
    await load()
  } catch (reason) {
    if (active) { formError.value = errorText(reason); error.value = formError.value; void load() }
  } finally { submitting.value = false }
}
async function create(): Promise<void> {
  if (!validStep()) return
  await submit({ action: 'create', ...form, publicKey: form.source === 'cloud' && loginMethod.value === 'key' ? form.publicKey.trim() : '', password: form.source === 'cloud' && loginMethod.value === 'password' ? form.password : '', username: form.source === 'cloud' ? form.username : '' })
}
function confirm(action: string, machine: VirtualMachine): void { confirmation.value = { action, machine: { ...machine } }; deleteDisks.value = false; formError.value = '' }
async function apply(): Promise<void> {
  const item = confirmation.value
  if (!item) return
  await submit({ action: item.action, id: item.machine.id, expectedResourceVersion: item.machine.resourceVersion, autostart: item.action === 'autostart' ? !item.machine.autostart : undefined, deleteDisks: item.action === 'delete' ? deleteDisks.value : undefined })
}
const confirmationHint = computed(() => {
  const action = confirmation.value?.action
  return action && ['delete', 'poweroff', 'shutdown', 'reboot', 'eject', 'autostart'].includes(action) ? t(`${action}Hint` as VMTextKey) : ''
})
function resume(): void { active = true; void load() }
watch(wizard, open => { if (!open) { form.password = ''; requestCache = undefined } })
function pause(): void { wizard.value = false; form.password = ''; active = false; clearTimeout(timer); controller?.abort(); consoleMachine.value = undefined }
onMounted(resume)
onActivated(() => { if (!active) resume() })
onDeactivated(pause)
onBeforeUnmount(pause)
</script>

<template>
  <main class="vm-page">
    <header class="vm-header">
      <div><h1>{{ t('title') }}</h1><p>{{ t('subtitle') }}</p></div>
      <div class="vm-actions"><button class="button button--secondary" :disabled="loading" @click="load"><RefreshCw :size="16" />{{ t('refresh') }}</button><button class="button button--primary" :disabled="!snapshot?.environment.ready || !writable" @click="openCreate"><Plus :size="17" />{{ t('create') }}</button></div>
    </header>
    <p v-if="error" class="vm-alert vm-alert--error" role="alert">{{ error }} <button class="button button--secondary" @click="load">{{ t('retry') }}</button></p>
    <p v-if="notice" class="vm-alert" role="status">{{ notice }}</p>
    <p v-if="loading && !snapshot" role="status">{{ t('loading') }}</p>
    <template v-if="snapshot">
      <section v-if="!snapshot.environment.ready" class="vm-environment">
        <Monitor :size="36" aria-hidden="true" /><div><h2>{{ t('installTitle') }}</h2><p>{{ envReason }}</p><p v-if="snapshot.environment.missing.length" class="vm-muted">{{ t('missing') }}：<code>{{ snapshot.environment.missing.join(', ') }}</code></p><p v-if="!snapshot.environment.installable" class="vm-muted">{{ snapshot.environment.reason === 'dependencies_missing' ? t('manualInstall') : t('unsupportedHelp') }}</p></div>
        <button v-if="snapshot.environment.installable" class="button button--primary" :disabled="!writable" @click="installOpen = true; formError = ''">{{ t('install') }}</button>
      </section>
      <p v-if="!snapshot.environment.writable" class="vm-alert">{{ t('readOnly') }}</p>
      <template v-if="snapshot.environment.ready || snapshot.environment.connected">
        <section class="vm-summary" :aria-label="t('resources')"><div><strong>{{ snapshot.machines.length }}</strong><span>{{ t('title') }}</span></div><div><strong>{{ snapshot.machines.filter(m => m.state === 'running').length }}</strong><span>{{ t('running') }}</span></div><div><strong>{{ snapshot.environment.cpus }} CPU · {{ Math.round(snapshot.environment.memoryMiB / 1024) }} GiB</strong><span>{{ t('capacityHint') }}</span></div></section>
        <div class="vm-toolbar"><label class="vm-search"><Search :size="17" /><input v-model="search" :placeholder="t('search')" :aria-label="t('search')" /></label><select v-model="filter" :aria-label="t('state')"><option value="all">{{ t('all') }}</option><option value="running">{{ t('running') }}</option><option value="shut off">{{ t('stopped') }}</option></select></div>
        <p v-if="snapshot.truncated" class="vm-alert">{{ t('truncated') }}</p>
        <div v-if="!machines.length" class="vm-empty"><Monitor :size="40" /><h2>{{ snapshot.machines.length ? t('noMatch') : t('empty') }}</h2><p v-if="!snapshot.machines.length">{{ t('emptyHint') }}</p><button v-if="!snapshot.machines.length" class="button button--primary" :disabled="!writable" @click="openCreate">{{ t('create') }}</button></div>
        <section v-else class="vm-list" :aria-label="t('title')">
          <article v-for="machine in machines" :key="machine.id" class="vm-row">
            <div class="vm-machine"><Monitor :size="24" aria-hidden="true" /><div><button class="vm-name" @click="detailID = machine.id">{{ machine.name }}</button><span class="vm-state" :data-state="machine.state">{{ state(machine.state) }}</span></div></div>
            <div class="vm-resource">{{ machine.cpus }} CPU · {{ machine.memoryMiB }} MiB<span>{{ t('autostart') }}：{{ machine.autostart ? t('enabled') : t('disabled') }}</span></div>
            <div class="vm-actions"><button v-if="machine.state === 'running'" class="button button--secondary" :disabled="!machine.console" @click="consoleMachine = machine">{{ t('console') }}</button><button v-if="machine.state === 'shut off'" class="button button--primary" :disabled="!writable" @click="confirm('start', machine)">{{ t('start') }}</button><button v-if="machine.state === 'running'" class="button button--secondary" :disabled="!writable" @click="confirm('shutdown', machine)">{{ t('shutdown') }}</button><button class="button button--secondary" @click="detailID = machine.id">{{ t('details') }}</button></div>
          </article>
        </section>
      </template>
      <section class="vm-tasks"><h2>{{ t('tasks') }}</h2><p class="vm-muted">{{ t('taskNotice') }}</p><p v-if="!snapshot.jobs.length" class="vm-muted">{{ t('noJobs') }}</p><button v-for="job in snapshot.jobs.slice(0, 8)" :key="job.id" class="vm-task" @click="selectedJob = job"><span><strong>{{ actionLabel(job.action) }}{{ job.name ? ` · ${job.name}` : '' }}</strong><span>{{ stage(job.stage) }}</span></span><span class="vm-state" :data-state="job.status">{{ job.status === 'running' ? t('taskRunning') : state(job.status) }}</span></button></section>
    </template>

    <ModalDialog :open="wizard" :title="t('create')" size="large" :close-disabled="submitting" @close="wizard = false">
      <form class="vm-form" @submit.prevent="step < 2 ? validStep() && step++ : create()">
        <ol class="vm-steps"><li v-for="(label, index) in steps" :key="label" :aria-current="index === step ? 'step' : undefined" :class="{ current: index === step }"><span>{{ index + 1 }}</span>{{ label }}</li></ol>
        <p v-if="formError" role="alert" class="vm-alert vm-alert--error">{{ formError }}</p>
        <template v-if="step === 0">
          <fieldset class="vm-source"><legend>{{ t('sourceStep') }}</legend><label :class="{ selected: form.source === 'cloud' }"><input v-model="form.source" type="radio" value="cloud" @change="form.image = 'debian-12'" /><strong>{{ t('quick') }}</strong><span>{{ t('quickHint') }}</span></label><label :class="{ selected: form.source === 'iso' }"><input v-model="form.source" type="radio" value="iso" @change="form.image = isoImages[0]?.name || ''" /><strong>{{ t('iso') }}</strong><span>{{ t('isoHint') }}</span></label></fieldset>
          <label v-if="form.source === 'cloud'">{{ t('image') }}<select v-model="form.image"><option value="debian-12">Debian 12</option><option value="ubuntu-24.04">Ubuntu 24.04 LTS</option></select></label>
          <template v-else><label>{{ t('pool') }}<select v-model="form.pool" @change="form.image = isoImages[0]?.name || ''"><option v-for="pool in snapshot?.pools" :key="pool.name">{{ pool.name }}</option></select></label><label>{{ t('image') }}<select v-model="form.image"><option v-for="image in isoImages" :key="image.name">{{ image.name }}</option></select></label><p v-if="!isoImages.length" class="vm-alert">{{ t('noISO') }} <a href="/files" target="_blank" rel="noopener">{{ t('files') }}</a></p></template>
          <p v-if="form.source === 'cloud'" class="vm-muted">{{ t('downloadHint') }}</p>
        </template>
        <template v-else-if="step === 1">
          <label>{{ t('name') }}<input v-model="form.name" maxlength="63" required autocomplete="off" placeholder="my-server" /></label>
          <div class="vm-form-grid"><label>{{ t('cpus') }}<input v-model.number="form.cpus" type="number" min="1" max="256" required /></label><label>{{ t('memory') }}<input v-model.number="form.memoryMiB" type="number" min="256" max="1048576" step="256" required /></label><label>{{ t('disk') }}<input v-model.number="form.diskGiB" type="number" min="4" max="65536" required /></label></div>
          <template v-if="form.source === 'cloud'"><label>{{ t('username') }}<input v-model="form.username" required autocomplete="off" maxlength="31" /></label><label>{{ t('loginMethod') }}<select v-model="loginMethod"><option value="password">{{ t('passwordLogin') }}</option><option value="key">{{ t('keyLogin') }}</option></select></label><label v-if="loginMethod === 'password'">{{ t('loginPassword') }}<input v-model="form.password" type="password" autocomplete="new-password" required minlength="12" maxlength="72" /></label><p v-if="loginMethod === 'password'" class="vm-muted">{{ t('passwordHint') }}</p><label v-if="loginMethod === 'key'">{{ t('publicKey') }}<textarea v-model="form.publicKey" required rows="3" maxlength="8192" spellcheck="false" :aria-description="t('keyHint')" /></label><p v-if="loginMethod === 'key'" class="vm-muted">{{ t('keyHint') }}</p></template>
          <details><summary>{{ t('advanced') }}</summary><div class="vm-form-grid"><label>{{ t('pool') }}<select v-model="form.pool"><option v-for="pool in snapshot?.pools" :key="pool.name">{{ pool.name }}</option></select></label><label>{{ t('network') }}<select v-model="form.network"><option v-for="network in snapshot?.networks" :key="network.name">{{ network.name }}</option></select></label></div><p class="vm-muted">{{ t('networkHint') }}</p><label class="vm-check"><input v-model="form.autostart" type="checkbox" />{{ t('autostart') }}</label></details>
        </template>
        <template v-else><dl class="vm-review"><dt>{{ t('name') }}</dt><dd>{{ form.name }}</dd><dt>{{ t('image') }}</dt><dd>{{ form.image }}</dd><dt>{{ t('resources') }}</dt><dd>{{ form.cpus }} CPU · {{ form.memoryMiB }} MiB · {{ form.diskGiB }} GiB</dd><dt>{{ t('pool') }}</dt><dd>{{ form.pool }}</dd><dt>{{ t('network') }}</dt><dd>{{ form.network }}</dd><dt v-if="form.source === 'cloud'">{{ t('username') }}</dt><dd v-if="form.source === 'cloud'">{{ form.username }}</dd></dl><p class="vm-alert">{{ t('reviewHint') }}</p><p class="vm-muted">{{ t('readyHint') }}</p></template>
        <footer class="vm-actions"><button type="button" class="button button--secondary" :disabled="submitting" @click="step ? step-- : wizard = false">{{ step ? t('back') : t('cancel') }}</button><button type="submit" class="button button--primary" :disabled="submitting || !writable">{{ submitting ? t('queued') : step < 2 ? t('next') : t('submit') }}</button></footer>
      </form>
    </ModalDialog>

    <ModalDialog :open="installOpen" :title="t('installTitle')" :close-disabled="submitting" @close="installOpen = false"><div class="vm-form"><p>{{ t('installHint') }}</p><p class="vm-muted">{{ t('installNote') }}</p><p v-if="formError" role="alert" class="vm-alert vm-alert--error">{{ formError }}</p><footer class="vm-actions"><button class="button button--secondary" :disabled="submitting" @click="installOpen = false">{{ t('cancel') }}</button><button class="button button--primary" :disabled="submitting || !writable" @click="submit({ action: 'install' })">{{ t('install') }}</button></footer></div></ModalDialog>
    <ModalDialog :open="!!detail" :title="detail?.name || t('details')" size="large" @close="detailID = ''"><div v-if="detail" class="vm-form"><dl class="vm-review"><dt>{{ t('state') }}</dt><dd>{{ state(detail.state) }}</dd><dt>{{ t('resources') }}</dt><dd>{{ detail.cpus }} CPU · {{ detail.memoryMiB }} MiB</dd><dt>{{ t('autostart') }}</dt><dd>{{ detail.autostart ? t('enabled') : t('disabled') }}</dd></dl><p class="vm-muted">{{ detail.addresses.length ? detail.addresses.join(', ') : t('addressPending') }}</p><h3>{{ t('diskFiles') }}</h3><p v-for="disk in detail.disks" :key="disk.target" class="vm-path"><code>{{ disk.target }} · {{ disk.path }}</code></p><div class="vm-actions"><button class="button button--secondary" :disabled="detail.state !== 'running' || !detail.console" @click="consoleMachine = detail">{{ t('console') }}</button><button v-for="action in ['start', 'shutdown', 'reboot', 'poweroff', 'autostart', 'eject', 'delete']" :key="action" class="button button--secondary" :disabled="!writable || (['shutdown', 'reboot', 'poweroff'].includes(action) && detail.state !== 'running') || (action === 'start' && detail.state !== 'shut off')" @click="confirm(action, detail)">{{ actionLabel(action) }}</button></div></div></ModalDialog>
    <ModalDialog :open="!!confirmation" :title="confirmation ? actionLabel(confirmation.action) : t('confirm')" :close-disabled="submitting" @close="confirmation = undefined"><div class="vm-form"><p>{{ t('confirmTarget') }}：<strong>{{ confirmation?.machine.name }}</strong></p><p>{{ confirmationHint }}</p><template v-if="confirmation?.action === 'delete'"><label class="vm-check"><input v-model="deleteDisks" type="checkbox" />{{ t('deleteDisks') }}</label><p class="vm-muted">{{ t('keepDisks') }}</p></template><p v-if="formError" role="alert" class="vm-alert vm-alert--error">{{ formError }}</p><footer class="vm-actions"><button class="button button--secondary" :disabled="submitting" @click="confirmation = undefined">{{ t('cancel') }}</button><button class="button" :class="confirmation?.action === 'delete' ? 'button--danger' : 'button--primary'" :disabled="submitting || !writable" @click="apply">{{ t('confirm') }}</button></footer></div></ModalDialog>
    <ModalDialog :open="!!selectedJob" :title="t('viewTask')" @close="selectedJob = undefined"><div v-if="selectedJob" class="vm-form"><p>{{ actionLabel(selectedJob.action) }} · {{ selectedJob.name }}</p><p>{{ state(selectedJob.status) }} · {{ stage(selectedJob.stage) }}</p><p v-if="selectedJob.message" data-i18n-ignore class="vm-path">{{ selectedJob.message === 'worker_interrupted' ? t('worker_interrupted') : selectedJob.message }}</p><p v-if="selectedJob.status === 'needs_attention'" class="vm-alert">{{ t('taskFailureHint') }}</p><code class="vm-path">{{ selectedJob.id }}</code><code v-for="artifact in selectedJob.artifacts" :key="artifact" class="vm-path" data-i18n-ignore>{{ artifact }}</code></div></ModalDialog>
    <VMConsole v-if="consoleMachine" :machine="consoleMachine" @close="consoleMachine = undefined" />
  </main>
</template>

<style scoped>
.vm-page { display: grid; gap: 1.5rem; color: var(--text); font-size: .875rem; min-width: 0; }
.vm-header,.vm-actions,.vm-toolbar,.vm-machine { display: flex; align-items: center; gap: .75rem; }
.vm-header { justify-content: space-between; flex-wrap: wrap; }
.vm-header h1 { margin: 0; font-size: 1.5rem; font-weight: 600; }
.vm-header p { margin: .4rem 0 0; color: var(--text-soft); }
.vm-page h2 { margin: 0; font-size: 1.0625rem; }
.vm-actions { flex-wrap: wrap; }
.vm-actions .button { gap: .4rem; font-size: .875rem; min-height: 2.5rem; }
.vm-alert { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; margin: 0; padding: .875rem 1rem; background: var(--surface-raised); border: 1px solid var(--border-strong); border-radius: var(--radius); line-height: 1.6; overflow-wrap: anywhere; }
.vm-alert--error { border-inline-start: 3px solid var(--danger, #c53333); }
.vm-environment { display: flex; align-items: flex-start; flex-wrap: wrap; gap: 1rem; padding: 1.5rem; border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface); }
.vm-environment>div { flex: 1; min-width: min(100%,15rem); }
.vm-environment p { line-height: 1.6; }
.vm-muted { color: var(--text-soft); font-size: .8125rem; line-height: 1.6; margin: .5rem 0; }
.vm-summary { display: grid; grid-template-columns: 1fr 1fr 2fr; gap: 1rem; }
.vm-summary>div { display: grid; gap: .5rem; padding: 1rem 1.25rem; border: 1px solid var(--border); border-radius: var(--radius); background: var(--surface); }
.vm-summary strong { font-size: 1.25rem; font-weight: 600; overflow-wrap: anywhere; }
.vm-summary span { color: var(--text-soft); font-size: .8125rem; }
.vm-toolbar { justify-content: space-between; flex-wrap: wrap; }
.vm-search { display: flex; align-items: center; gap: .5rem; max-width: 26rem; flex: 1; }
.vm-search input { width: 100%; }
.vm-page input,.vm-page select,.vm-form input,.vm-form select,.vm-form textarea { min-width: 0; padding: .65rem .75rem; border: 1px solid var(--border-strong); border-radius: var(--radius-sm); background: var(--surface); color: var(--text); font: inherit; font-size: .875rem; }
.vm-list { border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface); }
.vm-row { display: grid; grid-template-columns: minmax(10rem,1.3fr) minmax(10rem,1fr) auto; gap: 1rem; align-items: center; padding: 1.125rem 1.25rem; }
.vm-row+.vm-row { border-top: 1px solid var(--border); }
.vm-machine>div,.vm-resource { display: grid; gap: .45rem; min-width: 0; }
.vm-machine>svg { color: var(--brand); flex-shrink: 0; }
.vm-name { text-align: start; padding: 0; border: 0; background: none; color: var(--text); font: inherit; font-weight: 600; font-size: .9375rem; overflow-wrap: anywhere; cursor: pointer; }
.vm-resource>span { font-size: .8125rem; color: var(--text-soft); }
.vm-state { display: inline-flex; align-items: center; gap: .35rem; color: var(--text-soft); font-size: .8125rem; }
.vm-state::before { content: ''; width: .4rem; height: .4rem; border-radius: 50%; background: var(--muted); }
.vm-state[data-state="running"]::before,.vm-state[data-state="succeeded"]::before { background: var(--success,#228356); }
.vm-state[data-state="needs_attention"]::before { background: var(--danger,#c53333); }
.vm-empty { display: grid; justify-items: center; text-align: center; padding: 3rem 1rem; gap: .9rem; border: 1px dashed var(--border-strong); border-radius: var(--radius-lg); }
.vm-empty>svg { color: var(--muted); }
.vm-empty p { margin: 0; color: var(--text-soft); }
.vm-tasks { display: grid; gap: .5rem; }
.vm-task { display: flex; align-items: center; justify-content: space-between; gap: 1rem; width: 100%; padding: .875rem 1rem; background: var(--surface); color: var(--text); border: 1px solid var(--border); border-radius: var(--radius); text-align: start; font: inherit; cursor: pointer; }
.vm-task>span:first-child { display: grid; gap: .35rem; min-width: 0; overflow-wrap: anywhere; }
.vm-task strong { font-weight: 500; }
.vm-task>span>span { font-size: .8125rem; color: var(--text-soft); }
.vm-form { display: grid; gap: 1rem; padding: .25rem; color: var(--text); font-size: .875rem; line-height: 1.6; }
.vm-form label { display: grid; gap: .4rem; }
.vm-form p { margin: 0; }
.vm-form .vm-check { display: flex; align-items: flex-start; gap: .65rem; }
.vm-check input { margin-top: .3rem; }
.vm-form footer { justify-content: flex-end; border-top: 1px solid var(--border); padding-top: 1rem; margin-top: .5rem; }
.vm-steps { display: flex; flex-wrap: wrap; gap: .75rem 1.25rem; padding: 0; margin: 0 0 .5rem; list-style: none; }
.vm-steps li { display: flex; align-items: center; gap: .45rem; color: var(--text-soft); }
.vm-steps li>span { display: inline-grid; place-items: center; width: 1.6rem; height: 1.6rem; border: 1px solid var(--border-strong); border-radius: 50%; }
.vm-steps .current { color: var(--brand-strong); font-weight: 600; }
.vm-steps .current>span { background: var(--brand-soft); border-color: var(--brand); }
.vm-source { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: .75rem; padding: 0; margin: 0; border: 0; }
.vm-source legend { margin-bottom: .75rem; }
.vm-source label { position: relative; align-content: start; padding: 1rem; border: 1px solid var(--border-strong); border-radius: var(--radius); cursor: pointer; }
.vm-source .selected { border-color: var(--brand); background: var(--brand-soft); }
.vm-source input { justify-self: start; }
.vm-source span { color: var(--text-soft); font-size: .8125rem; }
.vm-form-grid { display: grid; grid-template-columns: repeat(3,minmax(0,1fr)); gap: .75rem; }
.vm-form details { border-top: 1px solid var(--border); padding-top: .75rem; }
.vm-form summary { cursor: pointer; font-weight: 500; margin-bottom: .75rem; }
.vm-review { display: grid; grid-template-columns: minmax(5rem,1fr) minmax(0,2fr); gap: .75rem 1rem; margin: 0; }
.vm-review dt { color: var(--text-soft); }
.vm-review dd { margin: 0; overflow-wrap: anywhere; }
.vm-path { overflow-wrap: anywhere; font-size: .8125rem; }
.vm-name:focus-visible,.vm-task:focus-visible { outline: 2px solid var(--brand); outline-offset: 3px; border-radius: var(--radius-sm); }
@media(max-width:850px) { .vm-row { grid-template-columns: 1fr 1fr; }.vm-row>.vm-actions { grid-column: 1/-1; }.vm-summary { grid-template-columns: 1fr 1fr; }.vm-summary>div:last-child { grid-column: 1/-1; } }
@media(max-width:480px) { .vm-form-grid,.vm-source { grid-template-columns: 1fr; }.vm-row { grid-template-columns: 1fr; padding: 1rem; }.vm-header>.vm-actions { width:100%; }.vm-page { gap: 1rem; }.vm-steps { gap: .5rem; }.vm-review { grid-template-columns: 1fr; gap: .25rem; }.vm-review dd { margin-bottom: .5rem; } }
</style>
