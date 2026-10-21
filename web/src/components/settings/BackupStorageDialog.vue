<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import ModalDialog from '@/components/common/ModalDialog.vue'
import { backups, backupMessage, type BackupSettings, type BackupStorage, type OneDriveAuthorization } from '@/lib/backup'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
const props = defineProps<{ settings: BackupSettings }>()
const emit = defineEmits<{ close: []; saved: [settings: BackupSettings] }>()
function phrase(value: string) { phraseCatalogVersion.value; return translatePhrase(value) }
const editing = ref<BackupStorage>()
const removing = ref<BackupStorage>()
const busy = ref(false)
const error = ref('')
const notice = ref('')
const authorizing = ref<OneDriveAuthorization>()
let pollTimer: ReturnType<typeof setTimeout> | undefined
let authGeneration = 0
let disposed = false
const valid = computed(() => !!editing.value?.name.trim() && (editing.value?.hasSecret || !!editing.value?.secret) && (editing.value.kind === 'onedrive' ? !!editing.value.clientId : !!editing.value.endpoint.trim() && (editing.value.kind === 's3' ? !!editing.value.bucket && !!editing.value.accessKey : !!editing.value.username)))
function cancelAuthorization() {
  authGeneration++; clearTimeout(pollTimer)
  const id = authorizing.value?.id; authorizing.value = undefined
  if (id) void backups.cancelOneDrive(id).catch(() => {})
}
onUnmounted(() => { disposed = true; cancelAuthorization() })
function changeKind() {
  cancelAuthorization()
  if (!editing.value) return
  editing.value.secret = ''; editing.value.hasSecret = false; editing.value.driveId = undefined
  if (editing.value.kind === 'onedrive') { editing.value.endpoint = 'https://graph.microsoft.com/v1.0'; editing.value.tenantId = 'common' }
}
function connectOneDrive() {
  if (!editing.value || busy.value) return
  cancelAuthorization()
  const generation = authGeneration
  return perform(async () => {
    const auth = await backups.startOneDrive(props.settings.revision, { ...editing.value!, secret: '' })
    if (disposed || generation !== authGeneration) { void backups.cancelOneDrive(auth.id).catch(() => {}); return }
    authorizing.value = auth
    const expires = Date.now() + auth.expiresIn * 1000
    async function poll() {
      if (disposed || generation !== authGeneration) return
      try {
        if (Date.now() >= expires) throw new Error(phrase('授权已过期，请重新连接 OneDrive。'))
        const result = await backups.pollOneDrive(auth.id)
        if (disposed || generation !== authGeneration) return
        if (result.pending) { pollTimer = setTimeout(poll, Math.max(auth.interval, 5) * 1000); return }
        if (!result.settings) throw new Error(phrase('连接未完成，请重新连接 OneDrive。'))
        authorizing.value = undefined; editing.value = undefined
        emit('saved', result.settings); notice.value = 'OneDrive 已连接。'
      } catch (reason) {
        if (disposed || generation !== authGeneration) return
        cancelAuthorization(); error.value = backupMessage(reason)
      }
    }
    pollTimer = setTimeout(poll, Math.max(auth.interval, 5) * 1000)
  })
}
function edit(storage?: BackupStorage) {
  editing.value = storage ? { ...storage, secret: '' } : { id: '', name: '', kind: 's3', endpoint: '', prefix: 'kpanel', region: 'us-east-1', pathStyle: false, secret: '' }
  error.value = ''; notice.value = ''; removing.value = undefined
}
async function perform(task: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try { await task() } catch (reason) { error.value = backupMessage(reason) }
  finally { busy.value = false }
}
function save() {
  if (!valid.value || !editing.value) return
  return perform(async () => { emit('saved', await backups.saveStorage(props.settings.revision, editing.value!)); editing.value = undefined; notice.value = '远程存储已保存。' })
}
function test(id: string) { return perform(async () => { await backups.testStorage(id); notice.value = '连接正常，已验证上传、读取、列出和删除权限。' }) }
function remove() { if (removing.value) return perform(async () => { emit('saved', await backups.deleteStorage(props.settings.revision, removing.value!.id)); removing.value = undefined }) }
</script>

<template>
  <ModalDialog :open="true" :title="phrase('远程存储')" size="medium" :close-disabled="busy" @close="emit('close')">
    <div class="backup-options">
      <template v-if="!editing">
        <p class="backup-hint">{{ phrase('连接 S3、WebDAV 或 OneDrive，备份文件由面板直接传输。') }}</p>
        <p v-if="!settings.storages.length" class="backup-hint">{{ phrase('还没有远程存储') }}</p>
        <ul class="backup-storage-list">
          <li v-for="storage in settings.storages" :key="storage.id">
            <div><strong>{{ storage.name }}</strong><small>{{ storage.kind === 's3' ? 'S3' : storage.kind === 'onedrive' ? 'OneDrive' : 'WebDAV' }} · {{ storage.prefix || '/' }}</small></div>
            <div class="backup-option-actions">
              <button type="button" class="button button--secondary" :disabled="busy" @click="test(storage.id)">{{ phrase('测试连接') }}</button>
              <button type="button" class="button button--ghost" :disabled="busy" @click="edit(storage)">{{ phrase('编辑') }}</button>
              <button type="button" class="button button--ghost button--danger-text" :disabled="busy || settings.schedule.storageId === storage.id" @click="removing = storage; error = ''">{{ phrase('删除') }}</button>
            </div>
          </li>
        </ul>
        <p v-if="settings.schedule.storageId" class="backup-hint">{{ phrase('自动备份使用中的存储，请先在自动备份中更换目标再删除。') }}</p>
        <div v-if="removing" class="backup-options">
          <p>{{ phrase('仅移除连接配置，远程文件仍会保留。') }} {{ removing.name }}</p>
          <div class="backup-option-actions"><button type="button" class="button button--ghost" :disabled="busy" @click="removing = undefined">{{ phrase('取消') }}</button><button type="button" class="button button--danger" :disabled="busy" @click="remove">{{ phrase('确认删除') }}</button></div>
        </div>
        <button v-else type="button" class="button button--primary" :disabled="busy || settings.storages.length >= 8" @click="edit()">{{ phrase('添加远程存储') }}</button>
      </template>
      <form v-else class="backup-options" @submit.prevent="save">
        <fieldset :disabled="busy || !!authorizing" class="backup-options">
          <label>{{ phrase('名称') }}<input v-model="editing.name" required maxlength="80" /></label>
          <label>{{ phrase('类型') }}<select v-model="editing.kind" @change="changeKind"><option value="s3">S3</option><option value="webdav">WebDAV</option><option value="onedrive">OneDrive</option></select></label>
          <label v-if="editing.kind !== 'onedrive'">{{ phrase('服务地址') }}<input v-model="editing.endpoint" type="url" required placeholder="https://…" maxlength="2048" /></label>
          <template v-if="editing.kind === 'onedrive'">
            <p class="backup-hint">{{ phrase('使用 Microsoft 国际版个人或工作/学校账户，文件保存在应用专属目录。') }}</p>
            <label>Application (client) ID<input v-model="editing.clientId" required maxlength="36" @input="editing.hasSecret = false" /></label>
            <label>{{ phrase('账户类型') }}<select v-model="editing.tenantId" @change="editing.hasSecret = false"><option value="common">{{ phrase('个人及工作/学校账户') }}</option><option value="consumers">{{ phrase('个人账户') }}</option><option value="organizations">{{ phrase('工作/学校账户') }}</option></select></label>
            <details><summary>{{ phrase('如何配置 OneDrive') }}</summary><p class="backup-hint">{{ phrase('在 Microsoft Entra 注册应用，选择支持的账户类型，启用公共客户端流，并添加 Microsoft Graph 委托权限 Files.ReadWrite.AppFolder。无需客户端密钥或回调地址。') }}</p><a href="https://learn.microsoft.com/entra/identity-platform/v2-oauth2-device-code" target="_blank" rel="noopener noreferrer">{{ phrase('Microsoft 官方文档') }}</a></details>
            <button type="button" class="button button--secondary" :disabled="!editing.name.trim() || !editing.clientId?.trim()" @click="connectOneDrive">{{ editing.hasSecret ? phrase('重新授权 OneDrive') : phrase('连接 OneDrive') }}</button>
          </template>
          <template v-if="editing.kind === 's3'">
            <label>Bucket<input v-model="editing.bucket" required maxlength="63" /></label>
            <label>Access Key ID<input v-model="editing.accessKey" required maxlength="256" autocomplete="off" /></label>
          </template>
          <label v-else-if="editing.kind === 'webdav'">{{ phrase('用户名') }}<input v-model="editing.username" required maxlength="256" autocomplete="off" /></label>
          <label v-if="editing.kind !== 'onedrive'">{{ editing.kind === 's3' ? 'Secret Access Key' : phrase('访问密码') }}<input v-model="editing.secret" type="password" :required="!editing.hasSecret" :placeholder="editing.hasSecret ? phrase('留空保持不变') : ''" maxlength="4096" autocomplete="new-password" /></label>
          <label>{{ phrase('备份目录') }}<input v-model="editing.prefix" maxlength="240" placeholder="kpanel" /></label>
          <details v-if="editing.kind === 's3'"><summary>{{ phrase('高级设置') }}</summary><div class="backup-options"><label>Region<input v-model="editing.region" placeholder="us-east-1" maxlength="80" /></label><label class="backup-inline"><input v-model="editing.pathStyle" type="checkbox" />{{ phrase('使用路径寻址（Path style）') }}</label></div></details>
        </fieldset>
        <div v-if="authorizing" class="backup-options" role="status">
          <p>{{ phrase('在 Microsoft 页面输入以下代码并授权，完成后会自动保存连接。') }}</p>
          <strong data-i18n-ignore>{{ authorizing.userCode }}</strong>
          <a :href="authorizing.verificationUri" target="_blank" rel="noopener noreferrer">{{ phrase('打开 Microsoft 授权页面') }}</a>
          <button type="button" class="button button--ghost" @click="cancelAuthorization">{{ phrase('取消授权') }}</button>
        </div>
        <p class="backup-hint">{{ phrase('建议使用 HTTPS 和独立备份目录。凭据加密保存在当前面板，不写入备份包。') }}</p>
        <footer class="backup-option-actions"><button type="button" class="button button--ghost" :disabled="busy" @click="cancelAuthorization(); editing = undefined; error = ''">{{ phrase('返回') }}</button><button type="submit" class="button button--primary" :disabled="busy || !!authorizing || !valid">{{ phrase('保存') }}</button></footer>
      </form>
      <p v-if="busy" role="status">{{ phrase('正在处理') }}</p>
      <p v-if="notice" role="status">{{ phrase(notice) }}</p>
      <p v-if="error" class="backup-option-error" role="alert">{{ phrase(error) }}</p>
    </div>
  </ModalDialog>
</template>

<style src="./backup-options.css"></style>
