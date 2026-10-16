// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { EditorState } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { SearchQuery } from '@codemirror/search'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CodeEditor from './CodeEditor.vue'
import { countSearchMatches } from '@/lib/code-editor-search'

const wrappers: ReturnType<typeof mount>[] = []
beforeEach(() => {
  Range.prototype.getClientRects = () => [] as unknown as DOMRectList
  Range.prototype.getBoundingClientRect = () => new DOMRect()
})
afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  vi.restoreAllMocks()
})
async function setup(modelValue: string) {
  const wrapper = mount(CodeEditor, {
    props: { modelValue, fileName: 'notes.txt', sizeBytes: modelValue.length, autoFocus: false },
    attachTo: document.body,
  })
  wrappers.push(wrapper)
  await vi.waitFor(() => expect(wrapper.emitted('ready')).toHaveLength(1))
  return wrapper
}
const view = (wrapper: Awaited<ReturnType<typeof setup>>) => EditorView.findFromDOM(wrapper.get<HTMLElement>('.cm-content').element)!

describe('CodeEditor find, go to line and selections', () => {
  it('counts matches, reports the current one and paints them without CodeMirror\'s panel', async () => {
    const wrapper = await setup('alpha\nbeta alpha\ngamma alpha')
    wrapper.vm.openSearch()
    await flushPromises()
    const input = wrapper.get<HTMLInputElement>('[aria-label="查找内容"]')
    await input.setValue('alpha')
    expect(wrapper.get('.code-search__count').text()).toBe('3 处')
    expect(wrapper.findAll('.cm-searchMatch')).toHaveLength(3)
    await input.trigger('keydown', { key: 'Enter' })
    await input.trigger('keydown', { key: 'Enter' })
    expect(wrapper.get('.code-search__count').text()).toBe('2/3')
    expect(wrapper.findAll('.cm-searchMatch-selected')).toHaveLength(1)
    await input.setValue('missing')
    expect(wrapper.get('.code-search__count').text()).toBe('无匹配')
    expect(wrapper.get('[aria-label="下一个匹配"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[aria-label="使用正则表达式"]').trigger('click')
    await input.setValue('(')
    expect(wrapper.get('.code-search__count').text()).toBe('表达式无效')
    await wrapper.get('[aria-label="使用正则表达式"]').trigger('click')
    await input.setValue('alpha')
    await wrapper.get('[aria-label="关闭查找"]').trigger('click')
    expect(wrapper.findAll('.cm-searchMatch')).toHaveLength(0)
    // F3 without the bar still finds the last term but leaves nothing painted.
    view(wrapper).contentDOM.dispatchEvent(new KeyboardEvent('keydown', { key: 'F3', bubbles: true, cancelable: true }))
    await flushPromises()
    expect(view(wrapper).state.selection.main).toMatchObject({ from: 23, to: 28 })
    expect(wrapper.findAll('.cm-searchMatch')).toHaveLength(0)
  })

  it('jumps to a line and column, and explains an out-of-range line', async () => {
    const wrapper = await setup('one\ntwo\nthree\nfour')
    wrapper.vm.openGotoLine()
    await flushPromises()
    const input = wrapper.get<HTMLInputElement>('.code-goto input')
    expect(document.activeElement).toBe(input.element)
    expect(wrapper.get('.code-goto .code-search__count').text()).toBe('1–4')
    await input.setValue('9')
    await input.trigger('keydown', { key: 'Enter' })
    expect(wrapper.get('[role="alert"]').text()).toBe('请输入 1 到 4 之间的行号')
    await input.setValue('3：4')
    await input.trigger('keydown', { key: 'Enter' })
    expect(wrapper.find('.code-goto').exists()).toBe(false)
    expect(wrapper.emitted('status')?.at(-1)?.[0]).toMatchObject({ line: 3, column: 4, lines: 4 })
  })

  it('reports multiple cursors and selected characters', async () => {
    const wrapper = await setup('port=1\nport=2\n')
    const editor = view(wrapper)
    editor.dispatch({ selection: { anchor: 0, head: 4 } })
    editor.contentDOM.dispatchEvent(new KeyboardEvent('keydown', { key: 'd', ctrlKey: true, bubbles: true, cancelable: true }))
    expect(editor.state.selection.ranges).toHaveLength(2)
    expect(wrapper.emitted('status')?.at(-1)?.[0]).toMatchObject({ selected: 8, cursors: 2 })
  })
})

describe('search match counting', () => {
  it('stops at the limit and tolerates an invalid query', () => {
    const state = EditorState.create({ doc: 'a '.repeat(30), selection: { anchor: 2, head: 3 } })
    expect(countSearchMatches(state, new SearchQuery({ search: 'a' }), 10)).toEqual({ total: 10, current: 2, capped: true })
    expect(countSearchMatches(state, new SearchQuery({ search: 'a' }))).toEqual({ total: 30, current: 2, capped: false })
    expect(countSearchMatches(state, new SearchQuery({ search: '(', regexp: true }))).toEqual({ total: 0, current: 0, capped: false })
  })
})
