<script setup lang="ts">
import { focusTextInput } from '../../lib/touchFocus'
import {
  computed,
  markRaw,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  useId,
  watch,
  type Raw,
} from 'vue'
import {
  ArrowUp,
  Check,
  MoreHorizontal,
  PanelLeft,
  RefreshCw,
  Save,
  Search,
  SquareTerminal,
  WrapText,
  X,
} from '@lucide/vue'
import CodeEditor from './CodeEditor.vue'
import FileEditorLayoutMenu from './FileEditorLayoutMenu.vue'
import FileEditorPanes from './FileEditorPanes.vue'
import FileEntryIcon from './FileEntryIcon.vue'
import { fileAPIForHost } from '@/lib/fileHostContext'
import { canExecuteFile } from '@/lib/fileExecution'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
import type { TerminalLayout } from '@/composables/useTerminalWorkspace'
import type { CodeEditorSession, CodeEditorStatus } from '@/lib/code-editor-session'
import type { FileDirectory, FileEntry } from '@/types/api'

const props = withDefaults(defineProps<{
  entry: FileEntry
  content: string
  hostId: string
  navigationPath?: string
  showDirectoryNavigation?: boolean
}>(), {
  showDirectoryNavigation: true,
})
const emit = defineEmits<{
  dirty: [value: boolean]
  saving: [value: boolean]
  saved: [entry: FileEntry]
  execute: [entry: FileEntry, hostId: string]
  close: []
  navigate: [path: string]
}>()
const hostId = props.hostId
const files = fileAPIForHost(hostId)
const executing = ref(false)
const MAX_TABS = 12
const MAX_ENTRIES = 500
interface EditorTab {
  entry: FileEntry
  content: string
  session?: Raw<CodeEditorSession>
  dirty: boolean
  revision: number
  saving: boolean
  loading: boolean
  error: string
  lineWrap: boolean
  /** A live editor for this tab is mounted in a pane and finished loading. */
  ready: boolean
  language: string
  status: CodeEditorStatus
}
function createTab(entry: FileEntry, content = '', loading = false): EditorTab {
  return {
    entry,
    content,
    dirty: false,
    revision: 0,
    saving: false,
    loading,
    error: '',
    lineWrap: false,
    ready: false,
    language: '',
    status: { line: 1, column: 1, lines: 1, selected: 0, cursors: 1 },
  }
}
type EditorInstance = InstanceType<typeof CodeEditor>
interface PanesHandle {
  layout: TerminalLayout
  setLayout(next: TerminalLayout): void
  selectSession(path: string): void
  activeEmpty: boolean
  cramped: boolean
  isShown(path: string): boolean
}
const tabs = ref<EditorTab[]>([createTab(props.entry, props.content)])
const activePath = ref(props.entry.path)
const active = computed(() => tabs.value.find((tab) => tab.entry.path === activePath.value)!)
const panes = ref<PanesHandle>()
// Toolbar, status bar and Ctrl+S act on the active pane's file; an empty
// split pane has none, so they wait until a file is opened there.
const target = computed(() => (panes.value?.activeEmpty ? undefined : active.value))
const editorReady = computed(() => Boolean(target.value?.ready && !target.value.loading))
// Mounted editors only; a hidden tab parks its CodeMirror state in `session`.
const editors = new Map<EditorTab, EditorInstance>()
const editorRefs = new WeakMap<EditorTab, (instance: unknown) => void>()
const root = ref<HTMLElement>()
const tabList = ref<HTMLElement>()
const sidebarToggle = ref<HTMLButtonElement>()
const sidebarSearch = ref<HTMLInputElement>()
const moreRoot = ref<HTMLElement>()
const moreToggle = ref<HTMLButtonElement>()
const moreOpen = ref(false)
const compact = ref(false)
// Phones (portrait, or landscape with little height) keep one editor: the
// split control is hidden there, as with the terminal.
const PHONE_VIEWPORT = '(max-width: 900px), (max-height: 500px) and (pointer: coarse)'
const phoneViewport = ref(false)
let phoneQuery: MediaQueryList | undefined
const splitAvailable = computed(() => !compact.value && !phoneViewport.value)
const sidebarOpen = ref(props.showDirectoryNavigation)
const directory = ref<FileDirectory>()
const directoryPath = ref(props.navigationPath || parentPath(props.entry.path))
const directoryLoading = ref(false)
const directoryError = ref('')
let failedDirectory: { path: string; append: boolean; record: boolean } | undefined
const query = ref('')
const notice = ref('')
const id = `file-editor-${useId()}`
let directoryRequest = 0
let directoryController: AbortController | undefined
let searchTimer: ReturnType<typeof setTimeout> | undefined
let observer: ResizeObserver | undefined
let disposed = false

function phrase(text: string): string {
  phraseCatalogVersion.value
  return translatePhrase(text)
}
function parentPath(path: string): string {
  return path.slice(0, path.lastIndexOf('/')) || '/'
}
// The sidebar header names the directory in one line; deep paths keep their
// trailing segments, and the full path stays in the tooltip.
const DIRECTORY_LABEL_LENGTH = 26
const directoryLabel = computed(() => {
  const path = directoryPath.value
  if (path.length <= DIRECTORY_LABEL_LENGTH) return path
  const segments = path.split('/').filter(Boolean)
  let label = segments.pop() || path
  while (segments.length && label.length + segments[segments.length - 1]!.length + 1 <= DIRECTORY_LABEL_LENGTH - 2) {
    label = `${segments.pop()}/${label}`
  }
  return `…/${label}`
})
function errorText(error: unknown): string {
  return error instanceof Error ? error.message : phrase('请求失败，请重试。')
}
function publishState(): void {
  emit(
    'dirty',
    tabs.value.some((tab) => tab.dirty),
  )
  emit(
    'saving',
    tabs.value.some((tab) => tab.saving),
  )
}
// Stable per tab, so Vue calls it with null only when that editor unmounts:
// the moment to park its document, history, selection and scroll.
function editorRef(tab: EditorTab): (instance: unknown) => void {
  let bind = editorRefs.get(tab)
  if (!bind) {
    bind = (instance) => {
      if (instance) {
        editors.set(tab, instance as EditorInstance)
        return
      }
      const session = editors.get(tab)?.getSession()
      editors.delete(tab)
      tab.ready = false
      if (session) tab.session = markRaw(session)
    }
    editorRefs.set(tab, bind)
  }
  return bind
}
function revealActiveTab(): void {
  tabList.value
    ?.querySelector('.editor-tab.is-active')
    ?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
}
function focusTab(): void {
  void nextTick(() => {
    const button = tabList.value?.querySelector<HTMLButtonElement>('[aria-selected="true"]')
    revealActiveTab()
    button?.focus({ preventScroll: true })
  })
}
function selectTab(tab: EditorTab, keyboard = false): void {
  // A new active path reaches the panes through their watcher, which lets a
  // new tab fill an empty pane first. The same path only refocuses its pane.
  if (tab.entry.path !== activePath.value) activePath.value = tab.entry.path
  else panes.value?.selectSession(tab.entry.path)
  notice.value = ''
  if (compact.value) sidebarOpen.value = false
  if (keyboard || compact.value) focusTab()
  else void nextTick(revealActiveTab)
}
function tabsKeydown(event: KeyboardEvent): void {
  if (!(event.target as HTMLElement).matches('[role="tab"]')) return
  const index = tabs.value.indexOf(active.value)
  const target =
    event.key === 'ArrowRight'
      ? (index + 1) % tabs.value.length
      : event.key === 'ArrowLeft'
        ? (index - 1 + tabs.value.length) % tabs.value.length
        : event.key === 'Home'
          ? 0
          : event.key === 'End'
            ? tabs.value.length - 1
            : -1
  if (target >= 0) {
    event.preventDefault()
    selectTab(tabs.value[target]!, true)
  } else if (event.key === 'Delete') {
    event.preventDefault()
    closeTab(active.value)
  }
}
function closeTab(tab: EditorTab): void {
  if (tab.saving) return
  if (tab.dirty && !window.confirm(phrase(`放弃 ${tab.entry.name} 的未保存修改并关闭？`))) return
  const index = tabs.value.indexOf(tab)
  if (tabs.value.length === 1) {
    tab.dirty = false
    publishState()
    emit('close')
    return
  }
  if (tab === active.value) selectTab(tabs.value[index + 1] || tabs.value[index - 1]!, true)
  tabs.value.splice(index, 1)
  publishState()
  focusTab()
}
async function loadTab(tab: EditorTab): Promise<void> {
  tab.loading = true
  tab.error = ''
  try {
    const content = await files.text(tab.entry.path)
    if (!disposed && tabs.value.includes(tab)) tab.content = content
  } catch (error) {
    if (!disposed && tabs.value.includes(tab)) tab.error = errorText(error)
  } finally {
    if (!disposed) tab.loading = false
  }
}
async function openFile(entry: FileEntry): Promise<void> {
  if (entry.kind === 'directory') {
    await browse(entry.path)
    return
  }
  const existing = tabs.value.find((tab) => tab.entry.path === entry.path)
  if (existing) {
    selectTab(existing)
    return
  }
  if (!entry.editable) return
  if (tabs.value.length >= MAX_TABS) {
    notice.value = phrase('最多打开 12 个文件，请先关闭一个标签。')
    return
  }
  tabs.value.push(createTab(entry, '', true))
  const tab = tabs.value[tabs.value.length - 1]!
  selectTab(tab)
  await loadTab(tab)
}
function activatePane(path: string): void {
  activePath.value = path
  notice.value = ''
}
function closeAuxiliary(event: MouseEvent, tab: EditorTab): void {
  if (event.button !== 1) return
  event.preventDefault()
  closeTab(tab)
}
function changed(tab: EditorTab): void {
  tab.dirty = true
  tab.revision += 1
  publishState()
}
function ready(tab: EditorTab, info: { label: string }): void {
  tab.ready = true
  tab.language = info.label
}
async function save(tab: EditorTab | undefined = target.value): Promise<void> {
  const instance = tab && editors.get(tab)
  if (!tab || !instance || !tab.ready || tab.loading || !tab.dirty || tab.saving) return
  const content = instance.getValue()
  const revision = tab.revision
  tab.saving = true
  tab.error = ''
  publishState()
  try {
    const result = await files.write(tab.entry.path, content, tab.entry.resourceVersion)
    if (disposed) return
    tab.entry = result.entry
    const listed = directory.value?.entries.findIndex((entry) => entry.path === result.entry.path)
    if (directory.value && listed !== undefined && listed >= 0) {
      directory.value.entries[listed] = result.entry
    }
    if (tab.revision === revision) {
      tab.dirty = false
      editors.get(tab)?.markClean()
    }
    emit('saved', result.entry)
  } catch (error) {
    if (!disposed) tab.error = errorText(error)
  } finally {
    if (!disposed) {
      tab.saving = false
      publishState()
    }
  }
}
async function execute(): Promise<void> {
  const tab = target.value
  if (!tab || executing.value || !editorReady.value || tab.saving || !canExecuteFile(tab.entry)) return
  const revision = tab.revision
  executing.value = true
  try {
    if (tab.dirty) await save(tab)
    if (disposed || target.value !== tab || tab.dirty || tab.error || tab.saving
      || tab.revision !== revision || !canExecuteFile(tab.entry)) return
    emit('execute', { ...tab.entry }, hostId)
  } finally {
    executing.value = false
  }
}

async function loadDirectory(
  path = directoryPath.value,
  append = false,
  record = false,
): Promise<void> {
  const request = ++directoryRequest
  directoryController?.abort()
  directoryController = new AbortController()
  directoryLoading.value = true
  directoryError.value = ''
  failedDirectory = undefined
  try {
    const result = await files.list(
      path,
      { search: query.value.trim(), offset: append ? directory.value?.nextOffset : 0 },
      directoryController.signal,
    )
    if (disposed || request !== directoryRequest) return
    directory.value = {
      ...result,
      entries: (append
        ? [...(directory.value?.entries || []), ...result.entries]
        : result.entries
      ).slice(0, MAX_ENTRIES),
    }
    directoryPath.value = result.path
    if (record) emit('navigate', result.path)
  } catch (error) {
    if (!disposed && request === directoryRequest) {
      directoryError.value = errorText(error)
      failedDirectory = { path, append, record }
    }
  } finally {
    if (!disposed && request === directoryRequest) directoryLoading.value = false
  }
}
async function browse(path: string, record = true): Promise<void> {
  clearTimeout(searchTimer)
  query.value = ''
  await loadDirectory(path, false, record)
}
function retryDirectory(): void {
  if (failedDirectory) {
    const { path, append, record } = failedDirectory
    void loadDirectory(path, append, record)
  }
}
watch(
  () => props.navigationPath,
  (path) => {
    if (!props.showDirectoryNavigation) return
    // Own successful pushes already have this directory. History traversal must
    // still cancel an in-flight read, even when returning to the displayed path.
    const target = path || parentPath(props.entry.path)
    if (directoryLoading.value || target !== directoryPath.value) void browse(target, false)
  },
)
function searchDirectory(): void {
  clearTimeout(searchTimer)
  directoryController?.abort()
  directoryRequest += 1
  searchTimer = setTimeout(() => {
    void loadDirectory()
  }, 220)
}
const entries = computed(() =>
  [...(directory.value?.entries || [])].sort(
    (a, b) =>
      Number(b.kind === 'directory') - Number(a.kind === 'directory') ||
      a.name.localeCompare(b.name),
  ),
)
function toggleSidebar(): void {
  moreOpen.value = false
  sidebarOpen.value = !sidebarOpen.value
  if (sidebarOpen.value && compact.value) void nextTick(() => focusTextInput(sidebarSearch.value))
}
function dismissMore(): void {
  moreOpen.value = false
  moreToggle.value?.focus({ preventScroll: true })
}
function closeMoreOnEscape(event: KeyboardEvent): void {
  if (!moreOpen.value) return
  event.stopPropagation()
  event.preventDefault()
  dismissMore()
}
function toggleMore(): void {
  moreOpen.value = !moreOpen.value
  if (moreOpen.value)
    void nextTick(() =>
      moreRoot.value
        ?.querySelector<HTMLButtonElement>('.editor-more__panel button:not(:disabled)')
        ?.focus(),
    )
}
function search(): void {
  moreOpen.value = false
  const tab = target.value
  void nextTick(() => (tab ? editors.get(tab)?.openSearch() : undefined))
}
function gotoLine(): void {
  const tab = target.value
  if (tab) editors.get(tab)?.openGotoLine()
}
function toggleWrap(): void {
  if (target.value) target.value.lineWrap = !target.value.lineWrap
  if (compact.value) dismissMore()
}
// An empty split pane asks for a file: the directory list is the picker.
function chooseFile(): void {
  if (!props.showDirectoryNavigation) return
  moreOpen.value = false
  sidebarOpen.value = true
  void nextTick(() => focusTextInput(sidebarSearch.value))
}
const selectionLabel = computed(() => {
  const status = target.value?.status
  if (!status) return ''
  const parts = []
  if (status.cursors > 1) parts.push(phrase(`${status.cursors} 个光标`))
  if (status.selected) parts.push(phrase(`已选 ${status.selected} 个字符`))
  return parts.join(' · ')
})
function outsideMore(event: PointerEvent): void {
  if (event.target instanceof Node && !moreRoot.value?.contains(event.target))
    moreOpen.value = false
}
function dismissSidebar(): void {
  sidebarOpen.value = false
  sidebarToggle.value?.focus({ preventScroll: true })
}
function beforeUnload(event: BeforeUnloadEvent): void {
  if (tabs.value.some((tab) => tab.dirty || tab.saving)) {
    event.preventDefault()
    event.returnValue = ''
  }
}
function syncPhoneViewport(): void {
  phoneViewport.value = Boolean(phoneQuery?.matches)
}
onMounted(() => {
  if (props.showDirectoryNavigation) void loadDirectory()
  if (typeof window.matchMedia === 'function') {
    phoneQuery = window.matchMedia(PHONE_VIEWPORT)
    syncPhoneViewport()
    phoneQuery.addEventListener?.('change', syncPhoneViewport)
  }
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(([entry]) => {
      if (!entry) return
      const next = entry.contentRect.width < 700
      if (next !== compact.value) {
        moreOpen.value = false
        compact.value = next
        sidebarOpen.value = props.showDirectoryNavigation && !next
      }
      void nextTick(revealActiveTab)
    })
    if (root.value) observer.observe(root.value)
  }
  window.addEventListener('beforeunload', beforeUnload)
  document.addEventListener('pointerdown', outsideMore)
})
onBeforeUnmount(() => {
  disposed = true
  directoryController?.abort()
  clearTimeout(searchTimer)
  observer?.disconnect()
  phoneQuery?.removeEventListener?.('change', syncPhoneViewport)
  window.removeEventListener('beforeunload', beforeUnload)
  document.removeEventListener('pointerdown', outsideMore)
})
defineExpose({ openFile })
</script>

<template>
  <section
    ref="root"
    class="editor-workspace"
    :class="{ 'is-compact': compact }"
    :aria-label="phrase('文件编辑器')"
    @keydown.ctrl.s.prevent.stop="save()"
    @keydown.meta.s.prevent.stop="save()"
  >
    <header class="editor-toolbar">
      <button
        v-if="props.showDirectoryNavigation"
        ref="sidebarToggle"
        class="editor-tool"
        :class="{ 'is-active': sidebarOpen }"
        type="button"
        :title="phrase('文件列表')"
        :aria-label="phrase('文件列表')"
        :aria-expanded="sidebarOpen"
        :aria-controls="`${id}-files`"
        @click="toggleSidebar"
      >
        <PanelLeft :size="18" />
      </button>
      <div
        ref="tabList"
        class="editor-tabs"
        :inert="compact && sidebarOpen"
        role="tablist"
        :aria-label="phrase('已打开的文件')"
        @keydown="tabsKeydown"
      >
        <div
          v-for="(tab, index) in tabs"
          :key="tab.entry.path"
          class="editor-tab"
          :class="{ 'is-active': tab === active, 'is-shown': tab !== active && panes?.isShown(tab.entry.path) }"
          @mousedown.middle.prevent
          @auxclick="closeAuxiliary($event, tab)"
        >
          <button
            :id="`${id}-tab-${index}`"
            class="editor-tab__select"
            type="button"
            role="tab"
            :aria-selected="tab === active"
            :aria-controls="panes?.isShown(tab.entry.path) ? `${id}-panel-${index}` : undefined"
            :tabindex="tab === active ? 0 : -1"
            :title="tab.entry.path"
            @click="selectTab(tab)"
          >
            <FileEntryIcon :entry="tab.entry" :size="20" /><span data-i18n-ignore>{{ tab.entry.name }}</span
            ><span v-if="tab.dirty" class="editor-dirty" :aria-label="phrase('未保存')" />
          </button>
          <button
            class="editor-tab__close"
            type="button"
            :disabled="tab.saving"
            :aria-label="phrase(`关闭 ${tab.entry.name}`)"
            :title="phrase('关闭标签')"
            @click="closeTab(tab)"
          >
            <RefreshCw v-if="tab.saving || tab.loading" :size="14" class="spinning" /><X
              v-else
              :size="15"
            />
          </button>
        </div>
      </div>
      <div
        ref="moreRoot"
        class="editor-actions"
        :inert="compact && sidebarOpen"
        @keydown.esc="closeMoreOnEscape"
      >
        <button
          v-if="compact"
          ref="moreToggle"
          class="editor-tool"
          type="button"
          :title="phrase('更多操作')"
          :aria-label="phrase('更多操作')"
          :aria-expanded="moreOpen"
          :aria-controls="`${id}-actions`"
          @click="toggleMore"
        >
          <MoreHorizontal :size="19" />
        </button>
        <div
          v-if="!compact || moreOpen"
          :id="`${id}-actions`"
          class="editor-actions__items"
          :class="{ 'editor-more__panel': compact }"
          role="group"
          :aria-label="phrase('更多操作')"
        >
          <button
            class="editor-tool"
            type="button"
            :title="phrase('查找或替换（Ctrl+F）')"
            :aria-label="phrase('查找或替换')"
            :disabled="!editorReady"
            @click="search"
          >
            <Search :size="17" /><span v-if="compact">{{ phrase('查找或替换') }}</span>
          </button>
          <button
            class="editor-tool"
            :class="{ 'is-active': target?.lineWrap }"
            type="button"
            :title="phrase('切换自动换行')"
            :aria-label="phrase('切换自动换行')"
            :aria-pressed="Boolean(target?.lineWrap)"
            :disabled="!target"
            @click="toggleWrap"
          >
            <WrapText :size="18" /><span v-if="compact">{{ phrase('切换自动换行') }}</span
            ><Check v-if="compact && target?.lineWrap" :size="15" />
          </button>
        </div>
      </div>
      <FileEditorLayoutMenu
        v-if="splitAvailable"
        :model-value="panes?.layout ?? 'single'"
        :cramped="panes?.cramped"
        @update:model-value="panes?.setLayout($event)"
      />
      <button
        v-if="target && canExecuteFile(target.entry)"
        class="editor-tool editor-execute"
        type="button"
        :disabled="!editorReady || target.saving || executing"
        :inert="compact && sidebarOpen"
        :aria-label="phrase(target.dirty ? '保存并执行' : '在终端中执行')"
        :title="phrase(target.dirty ? '保存并执行' : '在终端中执行')"
        @click="execute"
      >
        <SquareTerminal :size="16" /><span>{{ phrase(target.dirty ? '保存并执行' : '执行') }}</span>
      </button>
      <button
        class="button button--primary editor-save"
        type="button"
        :disabled="!editorReady || !target?.dirty || target.saving"
        :inert="compact && sidebarOpen"
        :aria-label="phrase(target?.saving ? '保存中…' : '保存')"
        :title="`${phrase('保存')} (Ctrl / ⌘ + S)`"
        @click="save()"
      >
        <RefreshCw v-if="target?.saving" :size="16" class="spinning" /><Save
          v-else
          :size="16"
        /><span>{{ phrase(target?.saving ? '保存中…' : '保存') }}</span>
      </button>
      <div v-if="$slots['window-controls']" class="editor-window-controls">
        <slot name="window-controls" />
      </div>
    </header>
    <div class="editor-workspace__body">
      <button
        v-if="props.showDirectoryNavigation && compact && sidebarOpen"
        class="editor-sidebar-backdrop"
        type="button"
        :aria-label="phrase('收起文件列表')"
        @click="dismissSidebar"
      />
      <aside
        v-if="props.showDirectoryNavigation && sidebarOpen"
        :id="`${id}-files`"
        class="editor-sidebar"
        :aria-label="phrase('文件列表')"
        @keydown.esc.stop.prevent="dismissSidebar"
      >
        <header class="editor-sidebar__header">
          <strong class="editor-sidebar__path" :title="directoryPath" data-i18n-ignore>{{
            directoryLabel
          }}</strong
          ><button
            class="editor-tool"
            type="button"
            :disabled="directoryLoading || directoryPath === '/'"
            :aria-label="phrase('上级目录')"
            :title="phrase('上级目录')"
            @click="browse(parentPath(directoryPath))"
          >
            <ArrowUp :size="16" /></button
          ><button
            class="editor-tool"
            type="button"
            :disabled="directoryLoading"
            :aria-label="phrase('刷新文件列表')"
            :title="phrase('刷新文件列表')"
            @click="loadDirectory()"
          >
            <RefreshCw :size="16" :class="{ spinning: directoryLoading }" /></button
          ><button
            v-if="compact"
            class="editor-tool"
            type="button"
            :aria-label="phrase('收起文件列表')"
            @click="dismissSidebar"
          >
            <X :size="17" />
          </button>
        </header>
        <label class="editor-filter"
          ><Search :size="15" /><input
            ref="sidebarSearch"
            v-model="query"
            :placeholder="phrase('搜索当前目录')"
            :aria-label="phrase('搜索当前目录')"
            @input="searchDirectory"
        /></label>
        <div v-if="directoryError" class="editor-message" role="alert">
          <span>{{ directoryError }}</span
          ><button type="button" @click="retryDirectory">{{ phrase('重试') }}</button>
        </div>
        <div class="editor-file-list" :aria-busy="directoryLoading">
          <div v-if="directoryLoading && !directory" class="editor-empty" role="status">
            {{ phrase('正在读取目录…') }}
          </div>
          <div
            v-else-if="!entries.length && !directoryLoading && !directoryError"
            class="editor-empty"
          >
            {{ phrase('没有匹配的文件') }}
          </div>
          <button
            v-for="entry in entries"
            :key="entry.path"
            class="editor-file"
            :class="{ 'is-active': entry.path === activePath }"
            type="button"
            :disabled="entry.kind !== 'directory' && !entry.editable"
            :aria-current="entry.path === activePath ? 'true' : undefined"
            :title="
              entry.editable || entry.kind === 'directory'
                ? entry.name
                : phrase('此文件不支持文本编辑')
            "
            @click="openFile(entry)"
          >
            <FileEntryIcon :entry="entry" :size="24" /><span data-i18n-ignore>{{ entry.name }}</span
            ><span
              v-if="tabs.some((tab) => tab.entry.path === entry.path && tab.dirty)"
              class="editor-dirty"
              :aria-label="phrase('未保存')"
            />
          </button>
          <button
            v-if="directory?.nextOffset !== undefined && entries.length < MAX_ENTRIES"
            class="editor-load-more"
            type="button"
            :disabled="directoryLoading"
            @click="loadDirectory(directoryPath, true)"
          >
            {{ phrase(directoryLoading ? '正在读取目录…' : '加载更多') }}
          </button>
          <p
            v-if="
              directory?.scanTruncated ||
              (directory?.nextOffset !== undefined && entries.length >= MAX_ENTRIES)
            "
            class="editor-empty"
          >
            {{ phrase('目录较大，请搜索文件名缩小范围。') }}
          </p>
        </div>
      </aside>
      <main class="editor-main" :inert="compact && sidebarOpen">
        <div v-if="notice" class="editor-message" role="status">{{ notice }}</div>
        <FileEditorPanes
          ref="panes"
          :tabs="tabs"
          :active-path="activePath"
          :split-available="splitAvailable"
          :can-choose-file="props.showDirectoryNavigation"
          :id-prefix="id"
          @activate="activatePane"
          @close="closeTab"
          @choose-file="chooseFile"
        >
          <template #default="{ tab }">
            <div v-if="tab.error" class="editor-message" role="alert">
              <span>{{ tab.error }}</span
              ><button v-if="!tab.dirty" type="button" @click="loadTab(tab)">
                {{ phrase('重试') }}</button
              ><span v-else>{{ phrase('修改仍保留在此标签，请检查后重试保存。') }}</span>
            </div>
            <div class="editor-canvas" :aria-busy="tab.loading">
              <div v-if="tab.loading" class="editor-empty editor-empty--center" role="status">
                <RefreshCw :size="20" class="spinning" />{{ phrase('正在打开文件…') }}
              </div>
              <CodeEditor
                v-else-if="!tab.error || tab.dirty"
                :key="tab.entry.path"
                :ref="editorRef(tab)"
                :model-value="tab.content"
                :file-name="tab.entry.name"
                :mime="tab.entry.mime"
                :size-bytes="tab.entry.sizeBytes"
                :session="tab.session"
                :line-wrap="tab.lineWrap"
                :auto-focus="false"
                @change="changed(tab)"
                @save="save(tab)"
                @status="tab.status = $event"
                @ready="ready(tab, $event)"
              />
            </div>
          </template>
        </FileEditorPanes>
        <footer class="editor-status">
          <template v-if="target">
            <button
              class="editor-status__position"
              type="button"
              :disabled="!editorReady"
              :title="phrase('跳转到行（Ctrl+G）')"
              @click="gotoLine"
            >
              {{ phrase('行') }} {{ target.status.line }}<span> : {{ target.status.column }}</span
              ><span class="editor-status__extra"> · {{ target.status.lines }} {{ phrase('行') }}</span>
            </button>
            <span v-if="selectionLabel" class="editor-status__selection">{{ selectionLabel }}</span
            ><span class="editor-status__language"
              >{{ target.language || 'UTF-8' }}<span class="editor-status__extra"> · UTF-8</span></span
            ><span class="editor-status__saved" role="status"
              ><span v-if="target.dirty" class="editor-dirty" /><Check v-else :size="14" />{{
                phrase(target.saving ? '保存中…' : target.dirty ? '未保存' : '已保存')
              }}</span
            >
          </template>
          <span v-else class="editor-status__empty">{{ phrase('空白窗格：从文件列表或顶部标签打开文件') }}</span>
        </footer>
      </main>
    </div>
  </section>
</template>

<style scoped>
.editor-workspace {
  --scrollbar-track: var(--file-preview-panel);
  --scrollbar-thumb: var(--file-preview-scrollbar);
  --scrollbar-thumb-hover: var(--file-preview-scrollbar-hover);
  --scrollbar-thumb-active: var(--file-preview-accent);
  display: flex;
  flex-direction: column;
  height: min(72dvh, 780px);
  min-height: 280px;
  overflow: hidden;
  border: 1px solid var(--file-preview-border);
  border-radius: var(--radius);
  color: var(--file-preview-text);
  background: var(--file-preview-background);
}
.editor-toolbar {
  position: relative;
  z-index: 4;
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  min-width: 0;
  min-height: 44px;
  padding: 0 8px;
  border-bottom: 1px solid var(--file-preview-border);
  background: var(--file-preview-panel);
}
/* Window buttons of the surrounding workspace dialog end the toolbar, so the
   editor needs no separate title bar. */
.editor-window-controls {
  display: flex;
  flex: 0 0 auto;
  align-self: stretch;
  margin-right: -8px;
  padding-left: 4px;
}
.editor-window-controls :deep(.modal-panel__window-action) {
  min-height: 0;
  height: 100%;
  color: var(--file-preview-muted);
}
.editor-window-controls :deep(.modal-panel__window-action:hover:not(:disabled)) {
  color: var(--file-preview-text);
  background: var(--file-preview-panel-raised);
}
.editor-window-controls :deep(.modal-panel__window-action--close:hover:not(:disabled)) {
  color: var(--on-danger);
  background: var(--danger-action);
}
.editor-actions,
.editor-actions__items {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 4px;
}
.editor-actions {
  position: relative;
}
.editor-more__panel {
  position: absolute;
  z-index: 1;
  top: calc(100% + 6px);
  right: 0;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  width: max-content;
  max-width: min(270px, 75vw);
  padding: 6px;
  border: 1px solid var(--file-preview-border);
  border-radius: var(--radius);
  color: var(--file-preview-text);
  background: var(--file-preview-panel);
  box-shadow: var(--file-preview-shadow);
}
.editor-more__panel .editor-tool {
  justify-content: flex-start;
  gap: 10px;
  width: 100%;
  height: auto;
  min-height: 44px;
  padding: 10px;
  color: var(--file-preview-text);
  font-size: 14px;
  text-align: left;
}
.editor-more__panel svg {
  flex: 0 0 auto;
}
.editor-tool {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--file-preview-muted);
  background: transparent;
  cursor: pointer;
}
.editor-tool:hover:not(:disabled),
.editor-tool.is-active {
  color: var(--file-preview-text);
  background: var(--file-preview-panel-raised);
}
.editor-workspace button:focus-visible,
.editor-workspace input:focus-visible {
  outline: 2px solid var(--file-preview-accent);
  outline-offset: -2px;
}
.editor-workspace button:disabled {
  opacity: 0.5;
  cursor: default;
}
.editor-save,
.editor-execute {
  flex: 0 0 auto;
  min-height: 36px;
  padding: 6px 12px;
  font-size: 14px;
}
.editor-execute {
  gap: 6px;
  width: auto;
}
.editor-workspace__body {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
}
.editor-sidebar {
  display: flex;
  flex-direction: column;
  flex: 0 0 220px;
  width: 220px;
  min-height: 0;
  border-right: 1px solid var(--file-preview-border);
  background: var(--file-preview-panel);
}
.editor-sidebar__header {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 6px 8px 8px 14px;
}
.editor-sidebar__path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--file-preview-text);
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.editor-filter {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 10px 10px;
  padding: 0 8px;
  border: 1px solid var(--file-preview-border);
  border-radius: var(--radius-sm);
  color: var(--file-preview-muted);
  background: var(--file-preview-background);
}
.editor-filter input {
  width: 100%;
  min-width: 0;
  height: 36px;
  border: 0;
  outline: 0;
  color: var(--file-preview-text);
  background: transparent;
  font-size: 14px;
}
.editor-file-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  overscroll-behavior: contain;
  padding: 0 10px 8px;
}
.editor-file {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 38px;
  padding: 8px;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--file-preview-text);
  background: transparent;
  text-align: left;
  font-size: 14px;
  cursor: pointer;
}
.editor-file svg {
  flex: 0 0 auto;
  color: var(--file-preview-muted);
}
.editor-file > span:first-of-type {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.editor-file:hover:not(:disabled),
.editor-file.is-active {
  background: var(--file-preview-panel-raised);
}
.editor-file.is-active {
  color: var(--file-preview-text);
  box-shadow: inset 2px 0 var(--file-preview-accent);
}
.editor-main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  min-height: 0;
}
.editor-tabs {
  display: flex;
  flex: 1;
  align-self: stretch;
  min-width: 0;
  overflow-x: auto;
  overscroll-behavior-x: contain;
  background: var(--file-preview-panel);
  scrollbar-width: none;
}
.editor-tabs::-webkit-scrollbar {
  display: none;
}
.editor-tab {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  max-width: 250px;
  padding-right: 4px;
  border-right: 1px solid var(--file-preview-border);
  border-top: 2px solid transparent;
}
.editor-tab.is-active {
  border-top-color: var(--file-preview-accent);
  background: var(--file-preview-background);
}
/* Files open in another split pane stay readable with a quieter rule. */
.editor-tab.is-shown {
  border-top-color: color-mix(in srgb, var(--file-preview-accent) 38%, transparent);
}
.editor-tab__select {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  align-self: stretch;
  min-height: 41px;
  padding: 8px 10px;
  border: 0;
  color: var(--file-preview-muted);
  background: transparent;
  font-size: 14px;
  cursor: pointer;
  transition: color var(--motion-duration-fast) var(--motion-ease-fade);
}
.editor-tab.is-active .editor-tab__select,
.editor-tab.is-shown .editor-tab__select,
.editor-tab__select:hover {
  color: var(--file-preview-text);
}
.editor-tab__select > span:first-of-type {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.editor-tab__select svg {
  flex-shrink: 0;
}
.editor-tab__close {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 28px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--file-preview-muted);
  background: transparent;
  cursor: pointer;
}
.editor-tab__close:hover {
  color: var(--file-preview-text);
  background: var(--file-preview-panel-raised);
}
.editor-dirty {
  display: inline-block;
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--file-preview-accent);
}
.editor-canvas {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.editor-status {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  min-height: 34px;
  padding: 6px 12px;
  border-top: 1px solid var(--file-preview-border);
  color: var(--file-preview-muted);
  background: var(--file-preview-panel);
  font-size: 13px;
  line-height: 1.5;
}
.editor-status__position {
  display: inline-flex;
  align-items: center;
  margin: -4px -6px;
  padding: 4px 6px;
  border: 0;
  border-radius: var(--radius-sm);
  color: inherit;
  background: transparent;
  font: inherit;
  white-space: nowrap;
  cursor: pointer;
}
.editor-status__position:hover:not(:disabled) {
  color: var(--file-preview-text);
  background: var(--file-preview-panel-raised);
}
.editor-workspace .editor-status__position:disabled {
  opacity: 1;
  cursor: default;
}
.editor-status__selection {
  color: var(--file-preview-text);
  font-variant-numeric: tabular-nums;
}
.editor-status__saved {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}
.editor-message {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  max-height: 140px;
  overflow: auto;
  overflow-wrap: anywhere;
  border-bottom: 1px solid var(--file-preview-border);
  color: var(--file-preview-text);
  background: var(--file-preview-panel-raised);
  font-size: 14px;
  line-height: 1.5;
}
.editor-message button,
.editor-load-more {
  padding: 8px;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--file-preview-text);
  background: var(--file-preview-panel);
  font-size: 14px;
  cursor: pointer;
}
.editor-load-more {
  width: 100%;
}
.editor-empty {
  padding: 20px 12px;
  color: var(--file-preview-muted);
  font-size: 14px;
  line-height: 1.6;
}
.editor-empty--center {
  display: flex;
  height: 100%;
  align-items: center;
  justify-content: center;
  gap: 10px;
}
.editor-sidebar-backdrop {
  position: absolute;
  z-index: 2;
  inset: 0;
  border: 0;
  background: color-mix(in srgb, var(--file-preview-background) 60%, transparent);
}
/* 宽屏：侧边栏占满左侧整列，标签栏与工具栏被挤到其右侧；窄屏仍为顶部工具栏 + 覆盖式侧栏。 */
.editor-workspace:not(.is-compact) {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  grid-template-rows: auto minmax(0, 1fr);
}
.editor-workspace:not(.is-compact) .editor-workspace__body {
  display: contents;
}
.editor-workspace:not(.is-compact) .editor-toolbar {
  grid-column: 2;
  grid-row: 1;
}
.editor-workspace:not(.is-compact) .editor-sidebar {
  grid-column: 1;
  grid-row: 1 / -1;
}
.editor-workspace:not(.is-compact) .editor-main {
  grid-column: 2;
  grid-row: 2;
}
.editor-workspace:not(.is-compact) .editor-sidebar__header {
  min-height: 44px;
  margin-bottom: 10px;
  padding-top: 0;
  padding-bottom: 0;
  border-bottom: 1px solid var(--file-preview-border);
}
.is-compact .editor-sidebar {
  position: absolute;
  z-index: 3;
  inset: 0 auto 0 0;
  width: min(290px, 88%);
  box-shadow: var(--file-preview-shadow);
}
.is-compact .editor-toolbar {
  gap: 2px;
  padding: 0 4px;
}
/* Cancel the narrower compact padding too, or the close button is clipped. */
.is-compact .editor-window-controls {
  margin-right: -4px;
  padding-left: 0;
}
.is-compact .editor-save,
.is-compact .editor-execute {
  width: 40px;
  min-height: 40px;
  padding: 0;
  justify-content: center;
}
.is-compact .editor-save > span,
.is-compact .editor-execute > span {
  display: none;
}
.is-compact .editor-tab {
  max-width: 210px;
}
.is-compact .editor-tool:not(.editor-more__panel .editor-tool) {
  width: 40px;
  height: 40px;
}
.is-compact .editor-file {
  min-height: 44px;
}
.is-compact .editor-filter input {
  min-height: 40px;
  font-size: 16px;
}
.is-compact .editor-tab__close {
  width: 36px;
  height: 42px;
}
.is-compact .editor-status {
  gap: 8px;
}
.is-compact .editor-status__extra {
  display: none;
}
:global(.modal-panel--fullscreen .editor-workspace) {
  height: 100%;
  min-height: 0;
}
:global(.modal-panel--workspace .editor-workspace) {
  flex: 1 1 auto;
  height: auto;
  min-height: 0;
  border: 0;
  border-radius: 0;
}
@media (max-width: 620px) {
  .editor-workspace {
    height: 72dvh;
  }
}
</style>
