<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api } from '@/lib/api'
import {
  notificationProviders,
  notificationConnection,
  hasNotificationConnection,
  buildNotificationCredential,
  notificationErrorMessage,
} from '@/lib/notificationChannels'
import type { NotificationConnection } from '@/lib/notificationChannels'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import type { ClusterNotificationProvider, ClusterNotificationSnapshot } from '@/types/api'

const props = defineProps<{ snapshot: ClusterNotificationSnapshot; disabled?: boolean }>()
const emit = defineEmits<{ updated: [value: ClusterNotificationSnapshot]; reload: []; busy: [value: boolean] }>()
const selected = ref<ClusterNotificationProvider>('telegram')
interface ChannelDraft {
  form: NotificationConnection
  enabled: boolean
  savedEnabled: boolean
}
type ChannelOperation = 'save' | 'remove' | 'test' | 'discover'
const operationMessages: Record<ChannelOperation, string> = {
  save: '渠道设置已保存。',
  remove: '渠道已移除。',
  test: '测试消息已发送。',
  discover: '已找到私聊会话，通知通道已就绪。',
}
const drafts = reactive(
  Object.fromEntries(
    notificationProviders.map((provider) => [
      provider.id,
      {
        form: notificationConnection(),
        enabled: true,
        savedEnabled: true,
      },
    ]),
  ) as Record<ClusterNotificationProvider, ChannelDraft>,
)
const draft = computed(() => drafts[selected.value])
const form = computed(() => draft.value.form)
const enabled = computed({
  get: () => draft.value.enabled,
  set: (value) => {
    draft.value.enabled = value
  },
})
const operating = ref(false)
const busy = computed(() => operating.value || props.disabled)
const error = ref('')
const status = ref('')
const channel = computed(() => props.snapshot.channels?.find((item) => item.provider === selected.value))
const isWebhook = computed(() => ['feishu', 'dingtalk', 'wecom', 'discord', 'slack'].includes(selected.value))
const isServer = computed(() => ['bark', 'ntfy', 'gotify'].includes(selected.value))
const edited = computed(() => hasNotificationConnection(form.value))
function phrase(value: string): string {
  phraseCatalogVersion.value
  return translatePhrase(value)
}
watch(selected, () => {
  error.value = ''
  status.value = ''
})
watch(
  () => props.snapshot,
  (value) => {
    for (const provider of notificationProviders) {
      const item = drafts[provider.id]
      const saved = value.channels?.find((channel) => channel.provider === provider.id)?.enabled ?? true
      if (item.enabled === item.savedEnabled) item.enabled = saved
      item.savedEnabled = saved
    }
  },
  { immediate: true },
)
function channelStatus(provider: ClusterNotificationProvider): string {
  const item = props.snapshot.channels?.find((value) => value.provider === provider)
  if (!item) return '未配置'
  if (!item.enabled) return '已暂停'
  switch (item.status) {
    case 'ready':
      return '已连接'
    case 'waiting_for_chat':
      return '等待私聊'
    default:
      return '连接异常'
  }
}
async function operate(action: ChannelOperation): Promise<void> {
  if (busy.value) return
  operating.value = true
  emit('busy', true)
  error.value = ''
  status.value = ''
  try {
    const provider = selected.value
    const expectedResourceVersion = props.snapshot.resourceVersion
    let value: ClusterNotificationSnapshot
    if (action === 'save' || action === 'remove') {
      value = await api.cluster.updateNotificationChannel({
        provider,
        enabled: enabled.value,
        remove: action === 'remove',
        credential: action === 'save' && edited.value ? buildNotificationCredential(provider, form.value) : undefined,
        expectedResourceVersion,
      })
    } else {
      value = await api.cluster.notificationChannelAction(action, { provider, expectedResourceVersion })
    }
    emit('updated', value)
    if (action === 'save' || action === 'remove') {
      Object.assign(form.value, notificationConnection())
      draft.value.enabled = value.channels?.find((channel) => channel.provider === provider)?.enabled ?? true
      draft.value.savedEnabled = draft.value.enabled
    }
    status.value = operationMessages[action]
  } catch (reason) {
    error.value = notificationErrorMessage(reason)
  } finally {
    operating.value = false
    emit('busy', false)
  }
}
</script>

<template>
  <section class="notification-channels" :aria-label="phrase('通知渠道')">
    <h3>{{ phrase('通知渠道') }}</h3>
    <p>{{ phrase('可同时启用多个渠道；各渠道独立发送，失败时只重试失败的渠道。') }}</p>
    <div class="notification-channels__grid" role="group" :aria-label="phrase('选择渠道')">
      <button
        v-for="provider in notificationProviders"
        :key="provider.id"
        type="button"
        :aria-pressed="selected === provider.id"
        :disabled="busy"
        @click="selected = provider.id"
      >
        <strong>{{ phrase(provider.name) }}</strong
        ><span>{{ phrase(channelStatus(provider.id)) }}</span>
      </button>
    </div>
    <div class="notification-channels__form">
      <label class="notification-channels__toggle"
        ><input v-model="enabled" type="checkbox" :disabled="busy" />{{ phrase('启用此渠道') }}</label
      >
      <p v-if="channel?.configured">{{ phrase('凭据已保存。留空可保留；修改时请完整填写该渠道配置。') }}</p>
      <p v-if="selected === 'telegram'">{{ phrase('私聊机器人发送 /start，KPanel 会自动发现接收会话。') }}</p>
      <template v-if="selected === 'telegram' || isWebhook">
        <label class="field"
          ><span>{{ phrase(selected === 'telegram' ? 'Bot API key' : 'Webhook 地址') }}</span
          ><input
            v-model="form.credential"
            type="password"
            autocomplete="new-password"
            :disabled="busy"
            :aria-label="phrase(selected === 'telegram' ? 'Bot API key' : 'Webhook 地址')"
        /></label>
      </template>
      <template v-if="isServer || selected === 'smtp'">
        <label class="field"
          ><span>{{ phrase(selected === 'smtp' ? 'SMTP 服务器' : '服务地址') }}</span
          ><input
            v-model="form.server"
            type="text"
            :disabled="busy"
            :placeholder="
              selected === 'smtp'
                ? 'smtp.example.com'
                : selected === 'ntfy'
                  ? 'https://ntfy.sh'
                  : selected === 'bark'
                    ? 'https://api.day.app'
                    : 'https://gotify.example.com'
            "
        /></label>
      </template>
      <label v-if="selected === 'bark' || selected === 'gotify'" class="field"
        ><span>{{ phrase(selected === 'bark' ? '推送 Key' : '应用 Token') }}</span
        ><input v-model="form.key" type="password" autocomplete="new-password" :disabled="busy"
      /></label>
      <label v-if="selected === 'ntfy'" class="field"
        ><span>Topic</span><input v-model="form.topic" type="text" :disabled="busy"
      /></label>
      <template v-if="selected === 'smtp' || selected === 'ntfy'">
        <label class="field"
          ><span>{{ phrase('用户名（可选）') }}</span
          ><input v-model="form.username" type="text" autocomplete="off" :disabled="busy"
        /></label>
        <label class="field"
          ><span>{{ phrase('密码（可选）') }}</span
          ><input v-model="form.password" type="password" autocomplete="new-password" :disabled="busy"
        /></label>
      </template>
      <template v-if="selected === 'smtp'">
        <label class="field"
          ><span>{{ phrase('TLS 端口') }}</span
          ><input v-model="form.port" type="number" min="1" max="65535" :disabled="busy"
        /></label>
        <label class="field"
          ><span>{{ phrase('发件人邮箱') }}</span
          ><input v-model="form.from" type="email" :disabled="busy"
        /></label>
        <label class="field"
          ><span>{{ phrase('收件人邮箱') }}</span
          ><input v-model="form.to" type="text" :disabled="busy"
        /></label>
        <p>{{ phrase('使用隐式 TLS，通常为 465 端口；多个收件人以逗号分隔。') }}</p>
      </template>
      <p v-if="isServer">{{ phrase('填写 HTTPS 服务地址，支持自建服务。') }}</p>
      <p>{{ phrase('保存新凭据时会发送验证消息。渠道开关与下方外部推送总开关同时开启后，才会自动推送。') }}</p>
    </div>
    <div class="notification-channels__actions">
      <button
        class="button button--secondary"
        type="button"
        :disabled="busy || (!channel?.configured && !edited)"
        @click="operate('save')"
      >
        {{ phrase(busy ? '正在处理…' : '保存渠道') }}
      </button>
      <button
        class="button button--secondary"
        type="button"
        :disabled="busy || !channel?.configured || edited"
        @click="operate('test')"
      >
        {{ phrase('发送测试消息') }}
      </button>
      <button
        v-if="selected === 'telegram'"
        class="button button--secondary"
        type="button"
        :disabled="busy || !channel?.configured || edited"
        @click="operate('discover')"
      >
        {{ phrase('重新发现私聊') }}
      </button>
      <button
        class="button button--secondary"
        type="button"
        :disabled="busy || !channel?.configured"
        @click="operate('remove')"
      >
        {{ phrase('移除渠道') }}
      </button>
    </div>
    <p v-if="error" role="alert">{{ phrase(error) }}</p>
    <button v-if="error" class="button button--secondary" type="button" :disabled="busy" @click="emit('reload')">
      {{ phrase('重新读取设置') }}
    </button>
    <p v-if="status" role="status">{{ phrase(status) }}</p>
  </section>
</template>

<style scoped>
.notification-channels {
  display: grid;
  gap: 14px;
}
.notification-channels h3 {
  margin: 0;
  font-size: 16px;
}
.notification-channels p {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.notification-channels__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
  gap: 8px;
}
.notification-channels__grid button {
  display: grid;
  gap: 5px;
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: inherit;
  background: var(--surface-subtle);
  text-align: left;
}
.notification-channels__grid button[aria-pressed='true'] {
  border-color: var(--brand);
  box-shadow: inset 0 0 0 1px var(--brand);
}
.notification-channels__grid strong,
.notification-channels input,
.notification-channels__toggle {
  font-size: 14px;
}
.notification-channels__grid span {
  font-size: 13px;
}
.notification-channels__form {
  display: grid;
  gap: 12px;
  min-width: 0;
}
.notification-channels__form input:not([type='checkbox']) {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.notification-channels__toggle {
  display: flex;
  align-items: center;
  gap: 8px;
}
.notification-channels__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.notification-channels [role='alert'],
.notification-channels [role='status'] {
  font-size: 14px;
}
</style>
