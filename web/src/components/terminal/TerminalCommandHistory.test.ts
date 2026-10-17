// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { resetTerminalCommandsForTest } from '@/stores/terminalCommands'
import { resetTerminalHistoryForTest } from '@/stores/terminalHistory'
import type { TerminalHistory } from '@/types/api'
import TerminalCommandHistory from './TerminalCommandHistory.vue'
import TerminalQuickCommands from './TerminalQuickCommands.vue'

const now = Date.now()
const stored: TerminalHistory = {
  schemaVersion: 1,
  available: true,
  items: [
    { command: 'docker ps -a', hostId: 'host-a', at: now },
    { command: 'for i in 1 2\ndo echo $i\ndone', hostId: '', at: now - 1000 },
    { command: 'journalctl -u nginx', hostId: 'gone', at: now - 2000 },
  ],
}
const hostNames = { 'host-a': '香港节点' }

describe('terminal command history', () => {
  beforeEach(() => {
    resetTerminalCommandsForTest()
    resetTerminalHistoryForTest()
    vi.restoreAllMocks()
    window.localStorage.clear()
    vi.spyOn(api.terminals, 'commands').mockResolvedValue({
      schemaVersion: 1, resourceVersion: `sha256:${'1'.repeat(64)}`, available: true,
      items: [{ id: '1'.repeat(32), name: '容器列表', command: 'docker ps' }],
    })
    vi.spyOn(api.terminals, 'history').mockResolvedValue(stored)
  })

  afterEach(() => {
    document.body.innerHTML = ''
    window.localStorage.clear()
  })

  it('switches the drawer between quick commands and history and remembers the tab', async () => {
    const wrapper = mount(TerminalQuickCommands, { props: { open: true, hostNames } })
    await flushPromises()
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs.map((tab) => tab.text())).toEqual(['快捷命令1', '历史命令'])
    expect(tabs[0]!.attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[aria-label="添加命令"]').exists()).toBe(true)

    await tabs[1]!.trigger('click')
    await flushPromises()
    expect(api.terminals.history).toHaveBeenCalled()
    expect(wrapper.findAll('[role="tab"]')[1]!.text()).toBe('历史命令3')
    expect(wrapper.find('[aria-label="添加命令"]').exists()).toBe(false)
    expect(wrapper.get('#terminal-command-panel-history').attributes('aria-labelledby')).toBe('terminal-command-tab-history')
    expect(window.localStorage.getItem('kpanel:terminal:command-panel-tab')).toBe('history')

    await wrapper.get('[role="tablist"]').trigger('keydown', { key: 'ArrowLeft' })
    expect(wrapper.findAll('[role="tab"]')[0]!.attributes('aria-selected')).toBe('true')
    wrapper.unmount()
  })

  it('shows every host in one list with where each command last ran', async () => {
    const wrapper = mount(TerminalCommandHistory, { props: { active: true, hostNames } })
    await flushPromises()
    const entries = wrapper.findAll('.terminal-history-entry')
    expect(entries.map((entry) => entry.get('code').text())).toEqual(['docker ps -a', 'for i in 1 2\ndo echo $i\ndone', 'journalctl -u nginx'])
    expect(entries.map((entry) => entry.get('small span').text())).toEqual(['香港节点', '批量执行', '其他主机'])

    await wrapper.get('input[type="search"]').setValue('香港')
    expect(wrapper.findAll('.terminal-history-entry')).toHaveLength(1)
    await wrapper.get('input[type="search"]').setValue('nothing-matches')
    expect(wrapper.text()).toContain('没有匹配的历史命令')
    wrapper.unmount()
  })

  it('fills the input box on click and runs only from the run button', async () => {
    const wrapper = mount(TerminalCommandHistory, { props: { active: true, hostNames } })
    await flushPromises()
    const entry = wrapper.findAll('.terminal-history-entry')[0]!
    expect(entry.get('.terminal-history-entry__fill').attributes('title')).toContain('填入输入框')
    await entry.get('.terminal-history-entry__fill').trigger('click')
    expect(wrapper.emitted('fill')).toEqual([['docker ps -a']])
    expect(wrapper.emitted('execute')).toBeUndefined()
    await entry.get('[aria-label="立即执行"]').trigger('click')
    expect(wrapper.emitted('execute')).toEqual([['docker ps -a']])

    await wrapper.setProps({ disabled: true })
    expect(entry.get('.terminal-history-entry__fill').attributes('disabled')).toBeDefined()
    expect(entry.get('[aria-label="立即执行"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('fills the batch command box in batch mode without a run button', async () => {
    const wrapper = mount(TerminalCommandHistory, { props: { active: true, mode: 'batch', hostNames } })
    await flushPromises()
    const entry = wrapper.findAll('.terminal-history-entry')[1]!
    expect(entry.find('[aria-label="立即执行"]').exists()).toBe(false)
    await entry.get('.terminal-history-entry__fill').trigger('click')
    expect(wrapper.emitted('execute')).toEqual([['for i in 1 2\ndo echo $i\ndone']])
    wrapper.unmount()
  })

  it('removes single commands and clears the shared list after confirmation', async () => {
    const remove = vi.spyOn(api.terminals, 'removeHistory').mockResolvedValue({ ...stored, items: stored.items.slice(1) })
    const clear = vi.spyOn(api.terminals, 'clearHistory').mockResolvedValue({ ...stored, items: [] })
    const wrapper = mount(TerminalCommandHistory, { attachTo: document.body, props: { active: true, hostNames } })
    await flushPromises()

    await wrapper.findAll('.terminal-history-entry')[0]!.get('[aria-label="从历史中删除"]').trigger('click')
    await flushPromises()
    expect(remove).toHaveBeenCalledWith('docker ps -a')
    expect(wrapper.findAll('.terminal-history-entry')).toHaveLength(2)

    await wrapper.get('[aria-label="清空历史命令"]').trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('将删除面板保存的全部 2 条历史命令')
    expect(clear).not.toHaveBeenCalled()
    document.body.querySelector<HTMLButtonElement>('.modal-panel__footer .button--danger')?.click()
    await flushPromises()
    expect(clear).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('还没有历史命令')
    expect(wrapper.get('[aria-label="清空历史命令"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('offers recovery when the stored history is unreadable', async () => {
    vi.spyOn(api.terminals, 'history').mockResolvedValue({ schemaVersion: 1, available: false, warning: 'terminal_history_unavailable', items: [] })
    const clear = vi.spyOn(api.terminals, 'clearHistory').mockResolvedValue({ schemaVersion: 1, available: true, items: [] })
    const wrapper = mount(TerminalCommandHistory, { props: { active: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('历史命令暂停记录')
    await wrapper.get('.terminal-command-history__action').trigger('click')
    await flushPromises()
    expect(clear).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('还没有历史命令')
    wrapper.unmount()
  })

  it('shows a retry when history cannot be loaded', async () => {
    vi.spyOn(api.terminals, 'history').mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(stored)
    const wrapper = mount(TerminalCommandHistory, { props: { active: true } })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('历史命令读取失败')
    await wrapper.get('.terminal-command-history__action').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.terminal-history-entry')).toHaveLength(3)
    wrapper.unmount()
  })
})
