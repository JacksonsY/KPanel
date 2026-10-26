// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BatchTerminalPanel from './BatchTerminalPanel.vue'
import { resetTerminalHistoryForTest } from '@/stores/terminalHistory'
import type { ClusterHost } from '@/types/api'
import type { TerminalBatch } from '@/types/terminalBatch'
const mocks = vi.hoisted(() => ({ batches: vi.fn(), getBatch: vi.fn(), startBatch: vi.fn(), stopBatch: vi.fn(), appendHistory: vi.fn(), close: vi.fn(), open: vi.fn() }))
vi.mock('@/lib/api', () => ({ ApiError: class extends Error { code = 'request_failed' }, api: { terminals: mocks } }))
const host = { id: 'local', name: '本机', terminalAvailable: true } as ClusterHost
function job(state: TerminalBatch['state'] = 'running'): TerminalBatch {
  return { id: 'a'.repeat(32), command: 'k 更新', state, createdAt: '2026-10-20T10:00:00Z', updatedAt: '2026-10-20T10:00:00Z', results: [{ host: { id: 'local', name: '本机', shell: 'posix' }, state: state === 'running' ? 'running' : state === 'interrupted' ? 'interrupted' : 'succeeded', output: 'last output', truncated: false, outputAvailable: true }] }
}
function mountPanel(capacity = 4) { return mount(BatchTerminalPanel, { props: { hosts: [host], sessionCapacity: capacity } }) }
function button(wrapper: ReturnType<typeof mountPanel>, text: string) { return wrapper.findAll('button').find(item => item.text().includes(text))! }
describe('server scheduled terminal batches', () => {
  beforeEach(() => { vi.useFakeTimers(); vi.clearAllMocks(); resetTerminalHistoryForTest(); mocks.batches.mockResolvedValue({ available: true, items: [] }); mocks.getBatch.mockImplementation(async (id: string) => (await mocks.batches.mock.results.at(-1)?.value).items.find((item: TerminalBatch) => item.id === id)); mocks.appendHistory.mockResolvedValue({ items: [], available: true }); mocks.startBatch.mockResolvedValue(job()); mocks.stopBatch.mockResolvedValue({ accepted: true }) })
  afterEach(() => { vi.useRealTimers() })
  it('submits one backend job even when local session capacity is zero', async () => {
    const wrapper = mountPanel(0); await flushPromises()
    await wrapper.get('textarea').setValue('k 更新'); await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    expect(mocks.startBatch).toHaveBeenCalledTimes(1)
    expect(mocks.startBatch.mock.calls[0]![0]).toMatchObject({ command: 'k 更新', hostIds: ['local'] })
    expect(mocks.startBatch.mock.calls[0]![0].id).toMatch(/^[a-f0-9]{32}$/)
    expect(mocks.open).not.toHaveBeenCalled()
    expect(wrapper.find('select, details, .batch-record-command').exists()).toBe(false)
    wrapper.unmount(); expect(mocks.stopBatch).not.toHaveBeenCalled(); expect(mocks.close).not.toHaveBeenCalled()
  })
  it('restores the last running job and output without submitting or opening a terminal', async () => {
    mocks.batches.mockResolvedValue({ available: true, items: [job()] })
    const wrapper = mountPanel(); await flushPromises()
    expect(wrapper.text()).toContain('命令执行中')
    await button(wrapper, '全部展开').trigger('click'); expect(wrapper.get('.batch-result__detail pre').text()).toBe('last output')
    expect(wrapper.get('textarea').element.value).toBe('')
    expect(wrapper.emitted('restoreSelection')).toEqual([[['local']]])
    expect(mocks.startBatch).not.toHaveBeenCalled(); expect(mocks.open).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('collapses the input while running and restores the same command when execution finishes', async () => {
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.get('textarea').setValue('k 更新')
    expect(wrapper.get('textarea').isVisible()).toBe(true)
    await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    expect(wrapper.get('.batch-command label').attributes('style')).toContain('display: none')
    expect(button(wrapper, '终止执行').isVisible()).toBe(true)
    mocks.batches.mockResolvedValue({ available: true, items: [job('completed')] })
    await vi.advanceTimersByTimeAsync(4000); await flushPromises()
    expect(wrapper.get('.batch-command label').attributes('style') || '').not.toContain('display: none')
    expect(wrapper.get('textarea').isVisible()).toBe(true)
    expect(wrapper.get('textarea').element.value).toBe('k 更新')
    expect(button(wrapper, '执行命令').isVisible()).toBe(true)
    expect(mocks.startBatch).toHaveBeenCalledTimes(1)
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
  it('shows only execute while temporary connections are handled in the background', async () => {
    const saved = job('completed'); saved.cleanupPending = true; saved.cleanupRunning = true
    mocks.batches.mockResolvedValue({ available: true, items: [saved] })
    const wrapper = mountPanel(); await flushPromises(); await wrapper.get('textarea').setValue('true')
    expect(wrapper.findAll('.batch-command footer button').map(item => item.text())).toEqual(['执行命令'])
    expect(wrapper.text()).not.toMatch(/释放|名额|会话/)
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeUndefined()
    await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    expect(mocks.startBatch).toHaveBeenCalledTimes(1)
    expect(mocks.stopBatch).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('finishes stop with a pending cleanup and restores that state without showing another stop action', async () => {
    mocks.batches.mockResolvedValue({ available:true, items:[job()] })
    const saved=job('stopped'); saved.cleanupPending=true; saved.results[0]!.state='failed'; saved.results[0]!.error='terminal_close_failed'
    mocks.stopBatch.mockImplementation(async () => { mocks.batches.mockResolvedValue({available:true,items:[saved]}) })
    const wrapper=mountPanel(); await flushPromises()
    await button(wrapper,'终止执行').trigger('click');await flushPromises()
    expect(wrapper.text()).toContain('已完成 1 / 1 台')
    expect(wrapper.text()).not.toContain('终止执行')
    expect(button(wrapper,'执行命令').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('释放')
    expect(wrapper.emitted('runningChange')?.at(-1)).toEqual([false])
    wrapper.unmount()
    const reopened=mountPanel();await flushPromises()
    expect(reopened.text()).not.toContain('终止执行')
    expect(button(reopened,'执行命令').exists()).toBe(true)
    expect(reopened.text()).not.toContain('释放')
    expect(mocks.startBatch).not.toHaveBeenCalled()
    reopened.unmount()
  })
  it('keeps result outputs and system identities independent of the next host selection', async () => {
    const ubuntu={...host,lastSnapshot:{telemetry:{osId:'ubuntu',os:'Ubuntu'}}} as ClusterHost
    mocks.batches.mockResolvedValue({available:true,items:[job('completed')]})
    const wrapper=mount(BatchTerminalPanel,{props:{hosts:[ubuntu],hostInventory:[ubuntu],sessionCapacity:4}})
    await flushPromises();await button(wrapper,'全部展开').trigger('click')
    expect(wrapper.findComponent({name:'OperatingSystemIcon'}).props('distro')).toBe('ubuntu')
    await wrapper.setProps({hosts:[]})
    expect(wrapper.findComponent({name:'OperatingSystemIcon'}).props('distro')).toBe('ubuntu')
    expect(wrapper.get('.batch-result__detail pre').text()).toBe('last output')
    expect(wrapper.findAll('.batch-result')).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(4000);await flushPromises()
    expect(wrapper.get('.batch-result__detail pre').text()).toBe('last output')
    wrapper.unmount()
  })
  it('clears the previous display while submitting and then shows only the accepted run', async () => {
    const previous=job('completed');mocks.batches.mockResolvedValue({available:true,items:[previous]})
    let accept!: (value:TerminalBatch)=>void
    mocks.startBatch.mockReturnValueOnce(new Promise(resolve=>{accept=resolve}))
    const wrapper=mountPanel();await flushPromises()
    expect(wrapper.findAll('.batch-result')).toHaveLength(1)
    await wrapper.get('textarea').setValue('true');await button(wrapper,'执行命令').trigger('click')
    expect(wrapper.findAll('.batch-result')).toHaveLength(0)
    expect(wrapper.text()).toContain('正在提交…')
    const next=job();next.id='b'.repeat(32);next.results[0]!.host.name='本轮主机'
    accept(next);await flushPromises()
    expect(wrapper.findAll('.batch-result')).toHaveLength(1)
    expect(wrapper.get('.batch-result__summary strong').text()).toBe('本轮主机')
    expect(previous.results[0]!.output).toBe('last output')
    expect(mocks.startBatch).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('restores previous results when a new submission is not confirmed', async () => {
    mocks.batches.mockResolvedValue({available:true,items:[job('completed')]})
    mocks.startBatch.mockRejectedValue(new Error('offline'))
    const wrapper=mountPanel();await flushPromises();await wrapper.get('textarea').setValue('true')
    await button(wrapper,'执行命令').trigger('click');await flushPromises()
    await button(wrapper,'全部展开').trigger('click')
    expect(wrapper.get('.batch-result__detail pre').text()).toBe('last output')
    expect(wrapper.text()).toContain('提交结果未确认')
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
  it('restores restart interruption in existing results without executing its summary', async () => {
    const saved = job('interrupted'); saved.results[0]!.error = 'panel_restarted'; saved.results[0]!.outputAvailable = false; saved.results[0]!.output = ''
    mocks.batches.mockResolvedValue({ available: true, items: [saved] })
    const wrapper = mountPanel(); await flushPromises(); await button(wrapper, '全部展开').trigger('click')
    expect(wrapper.text()).toContain('未完成命令未自动重放'); expect(wrapper.text()).toContain('本次执行输出已不可用')
    expect(wrapper.get('textarea').element.value).toBe('')
    expect(wrapper.find('select, details, .batch-record-command').exists()).toBe(false)
    expect(mocks.startBatch).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('does not turn a historical close failure into a permanent execution lock after restart', async () => {
    const saved = { ...job('completed'), cleanupPending: false, cleanupRunning: false }
    saved.results[0]!.state = 'failed'
    saved.results[0]!.error = 'terminal_close_failed'
    saved.results[0]!.outputAvailable = false
    mocks.batches.mockResolvedValue({ available: true, items: [saved] })
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.get('textarea').setValue('true')
    expect(wrapper.text()).not.toContain('重试释放会话')
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeUndefined()
    await button(wrapper, '执行命令').trigger('click'); await flushPromises()
    expect(mocks.startBatch).toHaveBeenCalledTimes(1)
    expect(mocks.stopBatch).not.toHaveBeenCalled()
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
  it('allows new work while older background cleanup is still running', async () => {
    const saved = job('stopped'); saved.cleanupPending = true; saved.cleanupRunning = true
    const recent = job('completed'); recent.id = 'b'.repeat(32)
    mocks.batches.mockResolvedValue({ available: true, items: [recent, saved] })
    const wrapper = mountPanel(); await flushPromises(); await wrapper.get('textarea').setValue('true')
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toMatch(/释放|名额|会话/)
    await vi.advanceTimersByTimeAsync(4000); await flushPromises()
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeUndefined()
    expect(mocks.stopBatch).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('ignores a stale poll failure after stopping a batch', async () => {
    mocks.batches.mockResolvedValue({ available: true, items: [job()] })
    const wrapper = mountPanel(); await flushPromises()
    let rejectPoll!: (reason: Error) => void
    mocks.batches.mockReturnValueOnce(new Promise((_resolve, reject) => { rejectPoll = reject }))
    await vi.advanceTimersByTimeAsync(1000)
    mocks.stopBatch.mockImplementation(async () => { mocks.batches.mockResolvedValue({ available: true, items: [job('stopped')] }) })
    await button(wrapper, '终止执行').trigger('click'); await flushPromises()
    rejectPoll(new Error('stale offline')); await flushPromises()
    await wrapper.get('textarea').setValue('true')
    expect(wrapper.text()).not.toContain('无法同步批量进度')
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('waits for synchronization after losing contact with a previously idle backend', async () => {
    const wrapper = mountPanel(); await flushPromises(); await wrapper.get('textarea').setValue('true')
    mocks.batches.mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(4000); await flushPromises()
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(4000); await flushPromises()
    expect(button(wrapper, '执行命令').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('can stop the active batch while the backend handles an older batch', async () => {
    const older = job('stopped'); older.id = 'b'.repeat(32)
    older.cleanupPending = true; older.cleanupRunning = true
    const active = job()
    mocks.batches.mockResolvedValue({ available: true, items: [active, older] })
    const wrapper = mountPanel(); await flushPromises()
    expect(button(wrapper, '终止执行').attributes('disabled')).toBeUndefined()
    await button(wrapper, '终止执行').trigger('click'); await flushPromises()
    expect(mocks.stopBatch).toHaveBeenCalledWith(active.id)
    wrapper.unmount()
  })
  it('keeps stop available after a failed stop request without claiming completion', async () => {
    mocks.batches.mockResolvedValue({ available: true, items: [job()] })
    mocks.stopBatch.mockRejectedValue(new Error('offline'))
    const wrapper = mountPanel(); await flushPromises()
    await button(wrapper, '终止执行').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('停止请求未确认')
    expect(button(wrapper, '终止执行').attributes('disabled')).toBeUndefined()
    expect(wrapper.findAll('.batch-command footer button')).toHaveLength(1)
    wrapper.unmount()
  })
  describe('output scroll position', () => {
    beforeEach(() => {
      // jsdom has no layout: model a scrollable output and display:none so
      // scrolling before Vue reveals the detail has the same effect as a browser.
      const positions = new WeakMap<HTMLElement, number>()
      function visible(element: HTMLElement): boolean {
        return !element.closest<HTMLElement>('.batch-result__detail')?.style.display.includes('none')
      }
      vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockImplementation(function (this: HTMLElement) {
        return visible(this) ? 1000 + (this.textContent?.length || 0) : 0
      })
      vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function (this: HTMLElement) {
        return visible(this) ? 190 : 0
      })
      vi.spyOn(HTMLElement.prototype, 'scrollTop', 'get').mockImplementation(function (this: HTMLElement) { return positions.get(this) || 0 })
      vi.spyOn(HTMLElement.prototype, 'scrollTop', 'set').mockImplementation(function (this: HTMLElement, value: number) {
        positions.set(this, Math.max(0, Math.min(value, this.scrollHeight - this.clientHeight)))
      })
    })
    afterEach(() => { vi.restoreAllMocks() })
    const output = (wrapper: ReturnType<typeof mountPanel>) => wrapper.get<HTMLPreElement>('.batch-result__detail pre').element
    const expectBottom = (pre: HTMLPreElement) => expect(pre.scrollTop).toBe(pre.scrollHeight - pre.clientHeight)

    it.each(['single', 'all'])('opens completed output at the bottom with %s expansion', async mode => {
      mocks.batches.mockResolvedValue({ available: true, items: [job('completed')] })
      const wrapper = mountPanel(); await flushPromises()
      if (mode === 'single') await wrapper.get('.batch-result__summary').trigger('click')
      else await button(wrapper, '全部展开').trigger('click')
      await flushPromises()
      expectBottom(output(wrapper))
      wrapper.unmount()
    })

    it('scrolls first output to the bottom when it arrives after expansion', async () => {
      const empty = job(); empty.results[0]!.output = ''
      mocks.batches.mockResolvedValue({ available: true, items: [empty] })
      const wrapper = mountPanel(); await flushPromises()
      await wrapper.get('.batch-result__summary').trigger('click')
      expect(wrapper.find('.batch-result__detail pre').exists()).toBe(false)
      mocks.batches.mockResolvedValue({ available: true, items: [job('completed')] })
      await vi.advanceTimersByTimeAsync(1000); await flushPromises()
      expectBottom(output(wrapper))
      wrapper.unmount()
    })

    it('follows appended output at the bottom but preserves manual scrolling during a pending refresh', async () => {
      mocks.batches.mockResolvedValue({ available: true, items: [job()] })
      const wrapper = mountPanel(); await flushPromises()
      await button(wrapper, '全部展开').trigger('click'); await flushPromises()
      const appended = job(); appended.results[0]!.output += '\nmore output'
      mocks.batches.mockResolvedValue({ available: true, items: [appended] })
      await vi.advanceTimersByTimeAsync(1000); await flushPromises()
      expectBottom(output(wrapper))

      let accept!: (value: TerminalBatch) => void
      mocks.getBatch.mockReturnValueOnce(new Promise(resolve => { accept = resolve }))
      await vi.advanceTimersByTimeAsync(1000); await flushPromises()
      output(wrapper).scrollTop = 30
      const completed = job('completed'); completed.results[0]!.output += '\nlast lines'
      accept(completed); await flushPromises()
      expect(output(wrapper).scrollTop).toBe(30)
      mocks.batches.mockResolvedValue({ available: true, items: [completed] })
      await vi.advanceTimersByTimeAsync(4000); await flushPromises()
      expect(output(wrapper).scrollTop).toBe(30)
      await wrapper.get('.batch-result__summary').trigger('click')
      await wrapper.get('.batch-result__summary').trigger('click'); await flushPromises()
      expectBottom(output(wrapper))
      wrapper.unmount()
    })

    it.each([20, 32])('uses two rendered lines as the follow range with %spx line height', async lineHeight => {
      mocks.batches.mockResolvedValue({ available: true, items: [job()] })
      const wrapper = mountPanel(); await flushPromises()
      await button(wrapper, '全部展开').trigger('click'); await flushPromises()
      const pre = output(wrapper); pre.style.lineHeight = `${lineHeight}px`
      pre.scrollTop = pre.scrollHeight - pre.clientHeight - 2 * lineHeight
      await vi.advanceTimersByTimeAsync(1000); await flushPromises()
      expectBottom(pre)
      pre.scrollTop = pre.scrollHeight - pre.clientHeight - 2 * lineHeight - 1
      const reading = pre.scrollTop
      await vi.advanceTimersByTimeAsync(1000); await flushPromises()
      expect(pre.scrollTop).toBe(reading)
      wrapper.unmount()
    })
  })
})
