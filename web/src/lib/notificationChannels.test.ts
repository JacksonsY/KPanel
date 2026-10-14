import { describe, expect, it } from 'vitest'
import {
  buildNotificationCredential,
  notificationConnection,
  hasNotificationConnection,
  notificationErrorMessage,
  notificationDeliveryReason,
} from './notificationChannels'

describe('channel connection forms', () => {
  it('does not silently discard port-only edits or mistake incomplete configuration for a network error', () => {
    const form = notificationConnection()
    expect(hasNotificationConnection(form)).toBe(false)
    form.port = '8465'
    expect(hasNotificationConnection(form)).toBe(true)
    expect(() => buildNotificationCredential('smtp', form)).toThrow()
    try {
      buildNotificationCredential('smtp', form)
    } catch (error) {
      expect(notificationErrorMessage(error)).toContain('完整填写')
    }
    expect(notificationErrorMessage(new TypeError('fetch failed'))).toContain('检查网络')
  })
  it('explains delivery cancellation and never reflects untrusted error codes', () => {
    expect(notificationDeliveryReason('channel_changed')).toContain('已停止投递')
    expect(notificationDeliveryReason('retry_expired')).toContain('重试期限')
    expect(notificationDeliveryReason('rule_disabled')).toContain('告警规则')
    expect(notificationDeliveryReason('https://secret@private.invalid')).not.toContain('secret')
  })
  it('escapes credentials and tagged email addresses without exposing library syntax in the form', () => {
    const form = {
      ...notificationConnection(),
      server: 'smtp.example.com',
      username: 'user@example.com',
      password: 'p@ss:word/#?',
      from: 'ops+panel@example.com',
      to: 'a@example.com,b@example.com',
    }
    const url = new URL(buildNotificationCredential('smtp', form))
    expect(decodeURIComponent(url.password)).toBe(form.password)
    expect(url.searchParams.get('from')).toBe(form.from)
    expect(url.searchParams.get('to')).toBe(form.to)
    expect(url.searchParams.get('encryption')).toBe('ImplicitTLS')
    expect(url.port).toBe('465')
  })
  it('converts official webhooks and rejects other hosts', () => {
    const form = notificationConnection()
    form.credential = 'https://discord.com/api/webhooks/12345678/abcdefgh'
    expect(buildNotificationCredential('discord', form)).toBe('discord://abcdefgh@12345678')
    form.credential = 'https://hooks.slack.com/services/TTEST/BTEST/TEST'
    expect(buildNotificationCredential('slack', form)).toBe('slack://@TTEST/BTEST/TEST')
    form.credential = 'https://evil.example/api/webhooks/123/secret'
    expect(() => buildNotificationCredential('discord', form)).toThrow()
  })
  it('supports self-hosted HTTPS services and keeps the Gotify subpath', () => {
    const form = { ...notificationConnection(), server: 'https://push.example.com/gotify/', key: 'secret' }
    expect(buildNotificationCredential('gotify', form)).toBe('gotify://push.example.com/gotify/secret')
    form.server = 'http://push.example.com'
    expect(() => buildNotificationCredential('gotify', form)).toThrow()
  })
})
