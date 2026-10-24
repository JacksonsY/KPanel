// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi, afterEach } from 'vitest'
import Channels from './NotificationChannels.vue'
import { ApiError } from '@/lib/api'
import type { ClusterNotificationSnapshot } from '@/types/api'
const mocks = vi.hoisted(() => ({ update: vi.fn(), action: vi.fn() }))
vi.mock('@/lib/api', () => ({
  api: { cluster: { updateNotificationChannel: mocks.update, notificationChannelAction: mocks.action } },
  ApiError: class extends Error {
    constructor(
      message: string,
      public status = 0,
      public code = 'request_failed',
    ) {
      super(message)
    }
  },
}))
const wrappers: ReturnType<typeof mount>[] = []
afterEach(() => {
  wrappers.splice(0).forEach((item) => item.unmount())
  vi.clearAllMocks()
})
const snapshot = {
  channels: [{ provider: 'telegram', enabled: true, configured: true, ready: true, status: 'ready' }],
  resourceVersion: 'v1',
} as ClusterNotificationSnapshot
function open() {
  const wrapper = mount(Channels, { props: { snapshot } })
  wrappers.push(wrapper)
  return wrapper
}
describe('independent notification channels', () => {
  it('shows the session challenge and keeps an unfinished discovery actionable', async () => {
    const pending = { ...snapshot, channels: [{ provider: 'telegram', enabled: true, configured: true, ready: false, status: 'waiting_for_chat' }], telegramBinding: { command: '/start random-proof', expiresAt: '2026-10-25T01:05:00Z' } } as ClusterNotificationSnapshot
    const wrapper = open()
    await wrapper.setProps({ snapshot: pending })
    mocks.action.mockResolvedValue(pending)
    expect(wrapper.text()).toContain('/start random-proof')
    expect(wrapper.text()).toContain('验证截止时间')
    expect(wrapper.text()).not.toContain('保存新凭据时会发送验证消息。')
    await wrapper.findAll('button').find(button => button.text() === '重新发现私聊')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('请发送验证命令，再点击发现私聊。')
    expect(wrapper.text()).not.toContain('已找到私聊会话，通知通道已就绪。')
    expect(mocks.action).toHaveBeenCalledWith('discover', { provider: 'telegram', expectedResourceVersion: 'v1' })
  })
  it('retains credentials and an unsaved toggle when switching channels and refreshing snapshots', async () => {
    const wrapper = open()
    await wrapper.get('input[type="password"]').setValue('draft-token')
    await wrapper.get('input[type="checkbox"]').setValue(false)
    await wrapper
      .findAll('button')
      .find((button) => button.text().startsWith('ntfy'))!
      .trigger('click')
    await wrapper.get('input[placeholder="https://ntfy.sh"]').setValue('https://ntfy.sh')
    await wrapper.setProps({ snapshot: { ...snapshot, resourceVersion: 'v2' } })
    await wrapper
      .findAll('button')
      .find((button) => button.text().startsWith('Telegram'))!
      .trigger('click')
    expect((wrapper.get('input[type="password"]').element as HTMLInputElement).value).toBe('draft-token')
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper
      .findAll('button')
      .find((button) => button.text().startsWith('ntfy'))!
      .trigger('click')
    expect((wrapper.get('input[placeholder="https://ntfy.sh"]').element as HTMLInputElement).value).toBe(
      'https://ntfy.sh',
    )
  })
  it('reports the selected channel operation while a Telegram challenge remains pending', async () => {
    const pending = {
      ...snapshot,
      channels: [...snapshot.channels!, { provider: 'ntfy', enabled: true, configured: true, ready: true, status: 'ready' }],
      telegramBinding: { command: '/start pending-proof', expiresAt: '2026-10-25T01:05:00Z' },
    } as ClusterNotificationSnapshot
    const wrapper = open()
    await wrapper.setProps({ snapshot: pending })
    await wrapper.findAll('button').find(button => button.text().startsWith('ntfy'))!.trigger('click')
    mocks.update.mockResolvedValue(pending)
    mocks.action.mockResolvedValue(pending)
    for (const [button, feedback] of [['保存渠道', '渠道设置已保存。'], ['发送测试消息', '测试消息已发送。'], ['移除渠道', '渠道已移除。']]) {
      await wrapper.findAll('button').find(item => item.text() === button)!.trigger('click')
      await flushPromises()
      expect(wrapper.get('[role="status"]').text()).toBe(feedback)
      expect(wrapper.text()).not.toContain('请发送验证命令，再点击发现私聊。')
    }
    await wrapper.findAll('button').find(button => button.text().startsWith('Telegram'))!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '发送测试消息')!.trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[role="status"]').at(-1)!.text()).toBe('测试消息已发送。')
  })
  it('keeps Telegram failures actionable and reports mutation activity to the dialog', async () => {
    mocks.update.mockRejectedValue(new ApiError('failed', 422, 'cluster_notifications_webhook_active'))
    const wrapper = open()
    await wrapper.get('input[type="password"]').setValue('new-token')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '保存渠道')!
      .trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('启用了 webhook')
    expect(wrapper.emitted('busy')).toEqual([[true], [false]])
    await wrapper.setProps({ disabled: true })
    expect(wrapper.findAll('button').every((button) => button.attributes('disabled') !== undefined)).toBe(true)
  })
  it('preserves a paused Telegram on the initial mount', async () => {
    const paused = { ...snapshot, channels: [{ ...snapshot.channels![0]!, enabled: false }] }
    const wrapper = mount(Channels, { props: { snapshot: paused } })
    wrappers.push(wrapper)
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    mocks.update.mockResolvedValue(paused)
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '保存渠道')!
      .trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith(expect.objectContaining({ provider: 'telegram', enabled: false }))
  })
  it('keeps Telegram configured while saving another channel and clears its credentials afterwards', async () => {
    mocks.update.mockResolvedValue({
      ...snapshot,
      resourceVersion: 'v2',
      channels: [
        ...snapshot.channels!,
        { provider: 'ntfy', enabled: true, configured: true, ready: true, status: 'ready' },
      ],
    })
    const wrapper = open()
    await wrapper
      .findAll('button')
      .find((button) => button.text().startsWith('ntfy'))!
      .trigger('click')
    await wrapper.get('input[placeholder="https://ntfy.sh"]').setValue('https://ntfy.sh')
    await wrapper.findAll('input[type="text"]')[1]!.setValue('ops')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '保存渠道')!
      .trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'ntfy',
        enabled: true,
        credential: 'ntfy://:@ntfy.sh/ops',
        expectedResourceVersion: 'v1',
      }),
    )
    expect(wrapper.emitted('updated')?.[0]?.[0]).toEqual(
      expect.objectContaining({
        channels: expect.arrayContaining([
          expect.objectContaining({ provider: 'telegram' }),
          expect.objectContaining({ provider: 'ntfy' }),
        ]),
      }),
    )
    expect((wrapper.get('input[placeholder="https://ntfy.sh"]').element as HTMLInputElement).value).toBe('')
  })
  it('toggles only the selected channel and tests that same destination', async () => {
    mocks.update.mockResolvedValue(snapshot)
    mocks.action.mockResolvedValue(snapshot)
    const wrapper = open()
    await wrapper.get('input[type="checkbox"]').setValue(false)
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '保存渠道')!
      .trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith(
      expect.objectContaining({ provider: 'telegram', enabled: false, credential: undefined }),
    )
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '发送测试消息')!
      .trigger('click')
    await flushPromises()
    expect(mocks.action).toHaveBeenCalledWith('test', { provider: 'telegram', expectedResourceVersion: 'v1' })
  })
  it('retains editable configuration after a failed save', async () => {
    mocks.update.mockRejectedValue(new Error('failure'))
    const wrapper = open()
    await wrapper.get('input[type="password"]').setValue('new-token')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '保存渠道')!
      .trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('检查网络')
    expect((wrapper.get('input[type="password"]').element as HTMLInputElement).value).toBe('new-token')
    expect(wrapper.emitted('updated')).toBeUndefined()
  })
})
