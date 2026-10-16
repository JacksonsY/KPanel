// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, onMounted, ref } from 'vue'
import { EditorState } from '@codemirror/state'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import FileEditorWorkspace from './FileEditorWorkspace.vue'
import type { FileEntry } from '@/types/api'

const mocks = vi.hoisted(() => ({ list: vi.fn(), text: vi.fn(), write: vi.fn(), openSearch: vi.fn(), openGotoLine: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: { files: mocks } }))
// Each pane mounts its own editor; the stub parks its text like CodeMirror's state.
const Editor = defineComponent({
  props: ['modelValue', 'session', 'fileName'],
  emits: ['change', 'ready', 'save'],
  setup(props, { expose, emit }) {
    const value = ref(props.session?.state.doc.toString() ?? props.modelValue)
    expose({
      getValue: () => value.value,
      getSession: () => ({ state: EditorState.create({ doc: value.value }), scrollTop: 0, scrollLeft: 0 }),
      markClean: vi.fn(),
      openSearch: () => mocks.openSearch(props.fileName),
      openGotoLine: () => mocks.openGotoLine(props.fileName),
    })
    onMounted(() => emit('ready', { label: 'Text' }))
    return () => h('textarea', {
      'data-file': props.fileName,
      value: value.value,
      onInput: (event: Event) => {
        value.value = (event.target as HTMLTextAreaElement).value
        emit('change')
      },
    })
  },
})
function entry(name: string): FileEntry {
  return {
    name, path: `/demo/${name}`, kind: 'file', sizeBytes: 10, mode: '644', owner: 'root', group: 'root',
    modifiedAt: '', resourceVersion: `v-${name}`, editable: true, previewable: true,
  }
}
const a = entry('a.conf'), b = entry('b.conf'), c = entry('c.conf')
const wrappers: ReturnType<typeof mount>[] = []
// Every observer hears the resize; the panes' own one reads real layout and stays quiet in jsdom.
const observers: ((entries: unknown[]) => void)[] = []
const resize = (entries: unknown[]) => observers.forEach((callback) => callback(entries))
class ResizeObserverStub {
  constructor(callback: (entries: unknown[]) => void) { observers.push(callback) }
  observe() {}
  disconnect() {}
}
async function setup() {
  const wrapper = mount(FileEditorWorkspace, {
    props: { entry: a, content: 'text a', hostId: 'local' },
    global: { stubs: { CodeEditor: Editor } },
    attachTo: document.body,
  })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}
type Wrapper = Awaited<ReturnType<typeof setup>>
async function layout(wrapper: Wrapper, value: string) {
  await wrapper.get('.editor-layout__trigger').trigger('click')
  await wrapper.get(`[data-layout="${value}"]`).trigger('click')
  await flushPromises()
}
const shown = (wrapper: Wrapper) => wrapper.findAll('.editor-pane[data-pane-path]').map((pane) => pane.attributes('data-pane-path'))
const activePane = (wrapper: Wrapper) => wrapper.find('.editor-pane.is-active').attributes('data-pane-path')
const textarea = (wrapper: Wrapper, name: string) => wrapper.get<HTMLTextAreaElement>(`textarea[data-file="${name}"]`)

beforeEach(() => {
  vi.clearAllMocks()
  observers.length = 0
  mocks.list.mockResolvedValue({ path: '/demo', entries: [a, b, c], offset: 0, truncated: false })
  mocks.text.mockImplementation(async (path: string) => `text ${path.at(-6)}`)
  mocks.write.mockImplementation(async (path: string) => ({ entry: { ...entry(path.split('/').at(-1)!), resourceVersion: 'saved' } }))
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  vi.stubGlobal('ResizeObserver', ResizeObserverStub)
})
afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('file editor split panes', () => {
  it('shows open files side by side and routes save, search and go-to-line to the clicked pane', async () => {
    const wrapper = await setup()
    await wrapper.vm.openFile(b); await flushPromises()
    await layout(wrapper, 'columns')
    expect(shown(wrapper)).toEqual([a.path, b.path])
    expect(wrapper.get(`[data-pane-path="${b.path}"]`).attributes('style')).toContain('grid-column: 3')
    expect(activePane(wrapper)).toBe(b.path)
    expect(wrapper.get('.editor-tab.is-shown').text()).toContain('a.conf')

    await wrapper.get(`[data-pane-path="${a.path}"]`).trigger('pointerdown')
    expect(activePane(wrapper)).toBe(a.path)
    expect(wrapper.get('[aria-selected="true"]').text()).toContain('a.conf')
    await textarea(wrapper, 'a.conf').setValue('edited a')
    await textarea(wrapper, 'b.conf').setValue('edited b')
    await wrapper.get(`[data-pane-path="${a.path}"]`).trigger('pointerdown')
    await wrapper.get('.editor-save').trigger('click')
    await flushPromises()
    expect(mocks.write).toHaveBeenCalledExactlyOnceWith(a.path, 'edited a', a.resourceVersion, 'local')
    expect(wrapper.find(`[data-pane-path="${b.path}"] .editor-pane__dirty`).exists()).toBe(true)

    await wrapper.get('[aria-label="查找或替换"]').trigger('click')
    await flushPromises()
    expect(mocks.openSearch).toHaveBeenCalledWith('a.conf')
    await wrapper.get('.editor-status__position').trigger('click')
    expect(mocks.openGotoLine).toHaveBeenCalledWith('a.conf')

    await wrapper.get(`[data-pane-path="${b.path}"] [aria-label="最大化窗格"]`).trigger('click')
    expect(shown(wrapper)).toEqual([b.path])
    expect(wrapper.find('.editor-panes__divider').exists()).toBe(false)
    await wrapper.get(`[data-pane-path="${b.path}"] [aria-label="恢复分屏"]`).trigger('click')
    expect(shown(wrapper)).toEqual([a.path, b.path])
    expect(textarea(wrapper, 'a.conf').element.value).toBe('edited a')
  })

  it('opens new files into an empty pane first and otherwise into the active pane', async () => {
    const wrapper = await setup()
    await layout(wrapper, 'columns')
    expect(wrapper.find('.editor-pane--empty').exists()).toBe(true)
    await wrapper.vm.openFile(b); await flushPromises()
    expect(shown(wrapper)).toEqual([a.path, b.path])
    expect(activePane(wrapper)).toBe(b.path)
    await wrapper.get(`[data-pane-path="${a.path}"]`).trigger('pointerdown')
    await wrapper.vm.openFile(c); await flushPromises()
    expect(wrapper.get(`[data-pane-path="${c.path}"]`).attributes('style')).toContain('grid-column: 1')
    expect(wrapper.get(`[data-pane-path="${b.path}"]`).attributes('style')).toContain('grid-column: 3')
    expect(wrapper.find(`[data-pane-path="${a.path}"]`).exists()).toBe(false)
  })

  it('parks drafts of files leaving a pane and restores them when they return', async () => {
    const wrapper = await setup()
    await wrapper.vm.openFile(b); await flushPromises()
    await layout(wrapper, 'columns')
    await textarea(wrapper, 'a.conf').setValue('draft a')
    await layout(wrapper, 'single')
    expect(shown(wrapper)).toEqual([b.path])
    await layout(wrapper, 'grid')
    expect(textarea(wrapper, 'a.conf').element.value).toBe('draft a')
    expect(mocks.text).toHaveBeenCalledTimes(1)
  })

  it('lets an empty pane pick a file from the directory list and pauses toolbar actions meanwhile', async () => {
    const wrapper = await setup()
    await layout(wrapper, 'grid')
    await wrapper.findAll('.editor-pane--empty button')[0]!.trigger('click')
    await flushPromises()
    expect(wrapper.get('.editor-status').text()).toContain('空白窗格')
    expect(wrapper.get('.editor-save').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[aria-label="查找或替换"]').attributes('disabled')).toBeDefined()
    expect(document.activeElement).toBe(wrapper.get('.editor-filter input').element)
    await wrapper.findAll('.editor-file').find((button) => button.text() === 'c.conf')!.trigger('click')
    await flushPromises()
    expect(activePane(wrapper)).toBe(c.path)
    expect(wrapper.get(`[data-pane-path="${c.path}"]`).attributes('style')).toContain('grid-column: 3')
    expect(wrapper.get('.editor-status').text()).not.toContain('空白窗格')
  })

  it('closes files from pane headers and with a middle click on their tab', async () => {
    const wrapper = await setup()
    await wrapper.vm.openFile(b); await flushPromises()
    await wrapper.vm.openFile(c); await flushPromises()
    await layout(wrapper, 'columns')
    await textarea(wrapper, 'a.conf').setValue('unsaved a')
    vi.mocked(window.confirm).mockReturnValueOnce(false)
    await wrapper.get(`[data-pane-path="${a.path}"] [aria-label="关闭 a.conf"]`).trigger('click')
    expect(window.confirm).toHaveBeenCalledOnce()
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(3)
    await wrapper.findAll('.editor-tab').find((tab) => tab.text().includes('b.conf'))!.trigger('auxclick', { button: 1 })
    expect(wrapper.findAll('[role="tab"]').map((tab) => tab.text())).toEqual(['a.conf', 'c.conf'])
    expect(shown(wrapper)).toEqual([a.path, c.path])
  })

  it('hides the split control on phones and narrow editors and shows only the active file', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: query.includes('max-width: 900px'), addEventListener() {}, removeEventListener() {} }))
    const phone = await setup()
    expect(phone.find('.editor-layout').exists()).toBe(false)
    phone.unmount(); wrappers.splice(0)
    vi.unstubAllGlobals()
    vi.stubGlobal('ResizeObserver', ResizeObserverStub)
    observers.length = 0
    const wrapper = await setup()
    await wrapper.vm.openFile(b); await flushPromises()
    await layout(wrapper, 'columns')
    expect(shown(wrapper)).toHaveLength(2)
    resize([{ contentRect: { width: 520 } }])
    await flushPromises()
    expect(wrapper.find('.editor-layout').exists()).toBe(false)
    expect(shown(wrapper)).toEqual([b.path])
    expect(wrapper.find('.editor-pane__header').exists()).toBe(false)
    resize([{ contentRect: { width: 1100 } }])
    await flushPromises()
    expect(shown(wrapper)).toEqual([a.path, b.path])
    await layout(wrapper, 'grid')
    await wrapper.findAll('.editor-pane--empty')[0]!.trigger('pointerdown')
    expect(wrapper.get('.editor-status').text()).toContain('空白窗格')
    resize([{ contentRect: { width: 520 } }])
    await flushPromises()
    expect(shown(wrapper)).toEqual([b.path])
    expect(wrapper.find('.editor-pane--empty').exists()).toBe(false)
    expect(wrapper.get('.editor-status').text()).not.toContain('空白窗格')
  })
})
