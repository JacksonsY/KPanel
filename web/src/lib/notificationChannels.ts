import type { ClusterNotificationProvider } from '@/types/api'
import { ApiError } from '@/lib/api'

export const notificationProviders = [
  { id: 'telegram', name: 'Telegram' },
  { id: 'feishu', name: '飞书' },
  { id: 'dingtalk', name: '钉钉' },
  { id: 'wecom', name: '企业微信' },
  { id: 'smtp', name: '邮件' },
  { id: 'bark', name: 'Bark' },
  { id: 'ntfy', name: 'ntfy' },
  { id: 'gotify', name: 'Gotify' },
  { id: 'discord', name: 'Discord' },
  { id: 'slack', name: 'Slack' },
] as const

export function notificationProviderName(provider: ClusterNotificationProvider): string {
  return notificationProviders.find((item) => item.id === provider)?.name || provider
}

export interface NotificationConnection {
  credential: string
  server: string
  key: string
  topic: string
  username: string
  password: string
  from: string
  to: string
  port: string
}

export function notificationConnection(): NotificationConnection {
  return { credential: '', server: '', key: '', topic: '', username: '', password: '', from: '', to: '', port: '465' }
}

export function hasNotificationConnection(form: NotificationConnection): boolean {
  return (
    form.port !== '465' ||
    [form.credential, form.server, form.key, form.topic, form.username, form.password, form.from, form.to].some(
      (value) => value !== '',
    )
  )
}

export function notificationErrorMessage(reason: unknown): string {
  if (reason instanceof NotificationConfigurationError) return '渠道配置无效，请完整填写地址和凭据。'
  if (!(reason instanceof ApiError)) return '通知操作未完成，请检查网络后重试；已填写内容仍保留。'
  const messages: Record<string, string> = {
    cluster_notifications_changed: '配置已被其他页面更新，请重新读取后再保存。',
    cluster_notifications_token_required: '请先输入 Telegram Bot API key。',
    cluster_notifications_credential_required: '请完整填写所选渠道配置。',
    cluster_notifications_not_configured: '还没有配置 Telegram Bot API key。',
    cluster_notifications_channel_not_configured: '还没有配置所选通知渠道。',
    cluster_notifications_not_ready: '请先私聊机器人发送 /start，再重新发现聊天。',
    cluster_notifications_channel_not_ready: '所选通知渠道尚未就绪，请检查配置后重新验证。',
    cluster_notifications_provider_mismatch: '凭据与所选通知渠道不匹配，请检查后重试。',
    cluster_notifications_unsupported_provider: '当前版本不支持这个通知渠道。',
    cluster_notifications_discover_unsupported: '所选通知渠道不需要发现聊天。',
    cluster_notifications_invalid_credential: '渠道配置无效，请检查地址和凭据。',
    cluster_notifications_chat_not_found: '没有找到私聊会话，请先私聊机器人发送 /start。',
    cluster_notifications_invalid_token: 'Bot API key 无效，请检查后重试。',
    cluster_notifications_webhook_active: '这个机器人启用了 webhook，暂时无法自动发现私聊；请先关闭 webhook。',
    cluster_notifications_telegram_unavailable: 'Telegram 暂时不可用，请稍后重试。',
    cluster_notifications_channel_unavailable: '渠道连接失败，请检查配置和服务器网络后重试。',
    cluster_notifications_save_failed_after_delivery:
      '验证消息已发送，但通知设置未保存；请检查 KPanel 数据目录权限后重试。',
    cluster_notifications_unavailable: '通知配置暂时不可用，请检查 KPanel 数据目录权限。',
  }
  return messages[reason.code] || '通知操作未完成，请稍后重试；已填写内容仍保留。'
}

export function notificationDeliveryReason(code?: string): string {
  if (!code) return ''
  if (code === 'retry_expired') return '重试期限已到，请检查渠道配置后发送测试消息。'
  if (['channel_changed', 'channel_removed', 'channel_disabled', 'push_disabled'].includes(code))
    return '渠道或推送设置已变化，已停止投递此消息。'
  if (
    [
      'expiry_reminder_changed',
      'rule_disabled',
      'traffic_threshold_changed',
      'host_removed',
      'traffic_cycle_changed',
    ].includes(code)
  )
    return '告警规则或主机状态已变化，已停止投递此消息。'
  if (code.startsWith('check_')) return '服务检测状态已变化，已停止投递旧消息。'
  if (code.endsWith('invalid_token') || code.endsWith('invalid_credential')) return '渠道凭据无效，请重新配置。'
  return '投递未完成，请检查渠道配置和服务器网络。'
}

// Normal channel forms generate the library configuration internally. Users
// never need to learn its URL syntax. The backend independently validates it.
class NotificationConfigurationError extends Error {}

export function buildNotificationCredential(
  provider: ClusterNotificationProvider,
  form: NotificationConnection,
): string {
  try {
    return serializeNotificationCredential(provider, form)
  } catch {
    throw new NotificationConfigurationError()
  }
}

function serializeNotificationCredential(provider: ClusterNotificationProvider, form: NotificationConnection): string {
  const value = form.credential.trim()
  if (['telegram', 'feishu', 'dingtalk', 'wecom'].includes(provider)) return value
  if (provider === 'discord' || provider === 'slack') {
    const url = new URL(value)
    if (url.protocol !== 'https:' || url.username || url.password || url.port || url.search || url.hash)
      throw new Error('invalid')
    if (provider === 'discord') {
      const match = url.pathname.match(/^\/api\/webhooks\/(\d+)\/([A-Za-z0-9._~-]+)$/)
      if (url.hostname !== 'discord.com' || !match) throw new Error('invalid')
      return `discord://${encodeURIComponent(match[2]!)}@${match[1]}`
    }
    if (
      url.hostname !== 'hooks.slack.com' ||
      !/^\/services\/[A-Za-z0-9]+\/[A-Za-z0-9]+\/[A-Za-z0-9]+$/.test(url.pathname)
    )
      throw new Error('invalid')
    return `slack://@${url.pathname.slice('/services/'.length)}`
  }
  if (provider === 'smtp') {
    if (!form.server.trim() || !form.from.trim() || !form.to.trim() || !/^\d+$/.test(form.port))
      throw new Error('invalid')
    const url = new URL(`smtp://${form.server.trim()}:${form.port}/`)
    if (url.pathname !== '/' || url.search || url.hash || url.username || url.password) throw new Error('invalid')
    url.username = form.username
    url.password = form.password
    // An empty userinfo is intentional for SMTP servers with no authentication.
    const user = `${encodeURIComponent(form.username)}:${encodeURIComponent(form.password)}@`
    url.search = new URLSearchParams({
      from: form.from.trim(),
      to: form.to.trim(),
      encryption: 'ImplicitTLS',
    }).toString()
    return url.toString().replace('smtp://', `smtp://${url.username || url.password ? '' : user}`)
  }
  const server = new URL(form.server.trim())
  if (server.protocol !== 'https:' || server.username || server.password || server.search || server.hash)
    throw new Error('invalid')
  const host = server.host
  if (provider === 'ntfy') {
    if (!form.topic.trim() || server.pathname !== '/') throw new Error('invalid')
    return `ntfy://${encodeURIComponent(form.username)}:${encodeURIComponent(form.password)}@${host}/${encodeURIComponent(form.topic.trim())}`
  }
  if (!form.key.trim()) throw new Error('invalid')
  if (provider === 'bark') {
    if (server.pathname !== '/') throw new Error('invalid')
    return `bark://:${encodeURIComponent(form.key.trim())}@${host}`
  }
  return `gotify://${host}${server.pathname.replace(/\/$/, '')}/${encodeURIComponent(form.key.trim())}`
}
