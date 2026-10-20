// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BatchTerminalPanel from './BatchTerminalPanel.vue'
import { resetTerminalHistoryForTest } from '@/stores/terminalHistory'
import type { ClusterHost } from '@/types/api'
import type { TerminalBatch } from '@/types/terminalBatch'
const mocks = vi.hoisted(() => ({ batches: vi.fn(), getBatch: vi.fn(), startBatch: vi.fn(), stopBatch: vi.fn(), removeBatch: vi.fn(), appendHistory: vi.fn(), close: vi.fn(), open: vi.fn() }))
vi.mock('@/lib/api', () => ({ ApiError: class extends Error { code = 'request_failed' }, api: { terminals: mocks } }))
const host = { id: 'local', name: '本机', terminalAvailable: true } as ClusterHost
function job(state: TerminalBatch['state'] = 'running'): TerminalBatch {
  return { id: 'a'.repeat(32), command: 'k 更新', state, createdAt: '2026-10-20T10:00:00Z', updatedAt: '2026-10-20T10:00:00Z', results: [{ host: { id: 'local', name: '本机', shell: 'posix' }, state: state === 'running' ? 'running' : state === 'interrupted' ? 'interrupted' : 'succeeded', output: 'last output', truncated: false, outputAvailable: true }] }
}
function mountPanel(capacity = 4) { return mount(BatchTerminalPanel, { props: { hosts: [host], sessionCapacity: capacity } }) }
function button(wrapper: ReturnType<typeof mountPanel>, text: string) { return wrapper.findAll('button').find(item => item.text().includes(text))! }
describe('server scheduled terminal batches', () => {
  beforeEach(() => { vi.useFakeTimers(); vi.clearAllMocks(); resetTerminalHistoryForTest(); mocks.batches.mockResolvedValue({ available: true, items: [] }); mocks.getBatch.mockImplementation(async (id: string) => (await mocks.batches.mock.results.at(-1)?.value).items.find((item: TerminalBatch) => item.id === id)); mocks.appendHistory.mockResolvedValue({ items: [], available: true }); mocks.startBatch.mockResolvedValue(job()); mocks.stopBatch.mockResolvedValue({ accepted: true }); mocks.removeBatch.mockResolvedValue({ accepted: true }) })
  afterEach(() => { vi.useRealTimers() })
  it('submits one backend job even when local session capacity is zero', async () => {
    const wrapper = mountPanel(0); await flushPromises()
    await wrapper.get('textarea').setValue('k 更新'); await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    expect(mocks.startBatch).toHaveBeenCalledTimes(1)
    expect(mocks.startBatch.mock.calls[0]![0]).toMatchObject({ command: 'k 更新', hostIds: ['local'] })
    expect(mocks.startBatch.mock.calls[0]![0].id).toMatch(/^[a-f0-9]{32}$/)
    expect(mocks.open).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('关闭浏览器后仍在后台执行')
    wrapper.unmount(); expect(mocks.stopBatch).not.toHaveBeenCalled(); expect(mocks.close).not.toHaveBeenCalled()
  })
  it('restores the last running job and output without submitting or opening a terminal', async () => {
    mocks.batches.mockResolvedValue({ available: true, items: [job()] })
    const wrapper = mountPanel(); await flushPromises()
    expect(wrapper.text()).toContain('命令执行中'); expect(wrapper.text()).toContain('k 更新')
    await button(wrapper, '全部展开').trigger('click'); expect(wrapper.get('.batch-result__detail pre').text()).toBe('last output')
    expect(wrapper.get('textarea').element.value).toBe('')
    expect(wrapper.emitted('restoreSelection')).toEqual([[['local']]])
    expect(mocks.startBatch).not.toHaveBeenCalled(); expect(mocks.open).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('stops only on an explicit stop action and waits for backend confirmation', async () => {
    mocks.batches.mockResolvedValue({ available: true, items: [job()] })
    const wrapper = mountPanel(); await flushPromises()
    await button(wrapper, '终止执行').trigger('click'); await flushPromises()
    expect(mocks.stopBatch).toHaveBeenCalledWith('a'.repeat(32))
    expect(wrapper.text()).toContain('命令执行中')
    wrapper.unmount()
  })
  it('reuses the submission identity after a lost acknowledgement', async () => {
    mocks.startBatch.mockRejectedValue(new Error('network'))
    const wrapper = mountPanel(); await flushPromises(); await wrapper.get('textarea').setValue('true')
    await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    const first = mocks.startBatch.mock.calls[0]![0].id
    await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    expect(mocks.startBatch.mock.calls[1]![0].id).toBe(first)
    wrapper.unmount()
  })
  it('shows restart interruption and deletes a completed record without executing its summary', async () => {
    const saved = job('interrupted'); saved.results[0]!.error = 'panel_restarted'; saved.results[0]!.outputAvailable = false; saved.results[0]!.output = ''
    mocks.batches.mockResolvedValue({ available: true, items: [saved] })
    const wrapper = mountPanel(); await flushPromises(); await button(wrapper, '全部展开').trigger('click')
    expect(wrapper.text()).toContain('未完成命令未自动重放'); expect(wrapper.text()).toContain('原终端输出已释放')
    expect(wrapper.get('textarea').element.value).toBe('')
    await button(wrapper, '删除记录').trigger('click'); await flushPromises()
    expect(mocks.removeBatch).toHaveBeenCalledWith(saved.id); expect(mocks.startBatch).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('disables submission when initial state cannot be synchronized', async () => {
    mocks.batches.mockRejectedValue(new Error('offline'))
    const wrapper = mountPanel(); await flushPromises(); await wrapper.get('textarea').setValue('true')
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('后台任务可能仍在执行')
    wrapper.unmount()
  })
  it('ignores a stale poll response after accepting a new job', async () => {
    const wrapper = mountPanel(); await flushPromises()
    let resolvePoll!: (value: { available: boolean; items: TerminalBatch[] }) => void
    mocks.batches.mockReturnValueOnce(new Promise(resolve => { resolvePoll = resolve }))
    await vi.advanceTimersByTimeAsync(4000)
    await wrapper.get('textarea').setValue('true'); await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    resolvePoll({ available: true, items: [] }); await flushPromises()
    expect(wrapper.text()).toContain('命令执行中'); wrapper.unmount()
  })
})
