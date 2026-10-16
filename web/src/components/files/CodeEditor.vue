<script setup lang="ts">
import { isTouchPrimary } from '../../lib/touchFocus'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ArrowDown,
  ArrowUp,
  CaseSensitive,
  ChevronDown,
  ChevronRight,
  CornerDownLeft,
  Regex,
  Replace,
  ReplaceAll,
  WholeWord,
  X,
} from '@lucide/vue'
import type {
  Compartment as CompartmentType,
  EditorState as EditorStateType,
  Extension,
} from '@codemirror/state'
import type { EditorView as EditorViewType } from '@codemirror/view'
import { loadCodeLanguage, type CodeLanguage } from '@/lib/code-editor-language'
import type { CodeEditorSession, CodeEditorStatus } from '@/lib/code-editor-session'
import type { SearchMatchCount } from '@/lib/code-editor-search'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'

const props = withDefaults(
  defineProps<{
    modelValue: string
    fileName: string
    mime?: string
    sizeBytes: number
    editable?: boolean
    lineWrap?: boolean
    session?: CodeEditorSession
    autoFocus?: boolean
  }>(),
  {
    mime: '',
    editable: true,
    lineWrap: false,
    autoFocus: true,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  dirty: []
  change: []
  save: [value: string]
  status: [value: CodeEditorStatus]
  ready: [value: CodeLanguage & { loadMs: number }]
}>()

const host = ref<HTMLElement>()
const searchInput = ref<HTMLInputElement>()
const gotoInput = ref<HTMLInputElement>()
const loading = ref(true)
const loadError = ref('')
const searchOpen = ref(false)
const replaceOpen = ref(false)
const searchValue = ref('')
const replaceValue = ref('')
const searchCaseSensitive = ref(false)
const searchRegexp = ref(false)
const searchWholeWord = ref(false)
const searchMessage = ref('')
const searchCount = ref<SearchMatchCount>()
const gotoOpen = ref(false)
const gotoValue = ref('')
const gotoMessage = ref('')
const documentLines = ref(1)
let editor: EditorViewType | undefined
let lineWrapCompartment: CompartmentType | undefined
let lineWrappingExtension: Extension | undefined
let searchRuntime: (Pick<
  typeof import('@codemirror/search'),
  'SearchQuery' | 'setSearchQuery' | 'getSearchQuery' | 'findNext' | 'findPrevious' | 'replaceNext' | 'replaceAll'
> & Pick<typeof import('@/lib/code-editor-search'), 'countSearchMatches'>) | undefined
let scrollIntoView: typeof EditorViewType.scrollIntoView | undefined
let applyingExternalValue = false
let dirty = false
let cancelled = false
let countTimer: ReturnType<typeof setTimeout> | undefined

function phrase(text: string): string {
  phraseCatalogVersion.value
  return translatePhrase(text)
}

// One live summary replaces the old "no match" flash: position, total or error.
const searchStatus = computed(() => {
  if (searchMessage.value) return phrase(searchMessage.value)
  const count = searchCount.value
  if (!searchValue.value || !count) return ''
  if (!count.total) return phrase('无匹配')
  const total = `${count.total}${count.capped ? '+' : ''}`
  return count.current ? `${count.current}/${total}` : phrase(`${total} 处`)
})

function currentValue(): string {
  return editor?.state.doc.toString() ?? props.modelValue
}

function markClean(): void {
  dirty = false
}

function getSession(): CodeEditorSession | undefined {
  if (!editor) return props.session
  return { state: editor.state, scrollTop: editor.scrollDOM.scrollTop, scrollLeft: editor.scrollDOM.scrollLeft }
}

function emitStatus(state: EditorStateType): void {
  const head = state.selection.main.head
  const line = state.doc.lineAt(head)
  documentLines.value = state.doc.lines
  emit('status', {
    line: line.number,
    column: head - line.from + 1,
    lines: state.doc.lines,
    selected: state.selection.ranges.reduce((total, range) => total + range.to - range.from, 0),
    cursors: state.selection.ranges.length,
  })
}

function requestSave(): void {
  const value = currentValue()
  emit('update:modelValue', value)
  emit('save', value)
}

function refreshSearchCount(): void {
  clearTimeout(countTimer)
  if (!editor || !searchRuntime || !searchOpen.value) return
  searchCount.value = searchRuntime.countSearchMatches(editor.state, searchRuntime.getSearchQuery(editor.state))
}

// Edits and cursor moves recount after a short pause rather than per keystroke.
function scheduleSearchCount(): void {
  clearTimeout(countTimer)
  countTimer = setTimeout(refreshSearchCount, 120)
}

function updateSearchQuery(): boolean {
  if (!editor || !searchRuntime) return false
  const query = new searchRuntime.SearchQuery({
    search: searchValue.value,
    replace: replaceValue.value,
    caseSensitive: searchCaseSensitive.value,
    regexp: searchRegexp.value,
    wholeWord: searchWholeWord.value,
  })
  editor.dispatch({ effects: searchRuntime.setSearchQuery.of(query) })
  searchMessage.value = searchValue.value && !query.valid ? '表达式无效' : ''
  refreshSearchCount()
  return query.valid
}

function runSearch(direction: 'next' | 'previous'): void {
  if (!editor || !searchRuntime || !updateSearchQuery()) return
  if (direction === 'next') searchRuntime.findNext(editor)
  else searchRuntime.findPrevious(editor)
  refreshSearchCount()
}

function runReplace(all: boolean): void {
  if (!editor || !searchRuntime || !props.editable || !updateSearchQuery()) return
  if (all) searchRuntime.replaceAll(editor)
  else searchRuntime.replaceNext(editor)
  refreshSearchCount()
}

function toggleSearchOption(option: 'case' | 'regexp' | 'word'): void {
  if (option === 'case') searchCaseSensitive.value = !searchCaseSensitive.value
  else if (option === 'regexp') searchRegexp.value = !searchRegexp.value
  else searchWholeWord.value = !searchWholeWord.value
  updateSearchQuery()
}

function openSearch(showReplace = false): void {
  if (!editor) return
  gotoOpen.value = false
  const selection = editor.state.selection.main
  const selectedText = selection.empty
    ? ''
    : editor.state.sliceDoc(selection.from, selection.to)
  if (!searchValue.value && selectedText && !/[\r\n]/.test(selectedText)) {
    searchValue.value = selectedText
  }
  searchOpen.value = true
  replaceOpen.value = replaceOpen.value || showReplace
  updateSearchQuery()
  void nextTick(() => {
    searchInput.value?.focus()
    searchInput.value?.select()
  })
}

function closeSearch(focusEditor = true): void {
  if (!searchOpen.value) return
  searchOpen.value = false
  searchMessage.value = ''
  searchCount.value = undefined
  clearTimeout(countTimer)
  if (editor && searchRuntime) {
    editor.dispatch({
      effects: searchRuntime.setSearchQuery.of(new searchRuntime.SearchQuery({ search: '' })),
    })
    if (focusEditor) editor.focus()
  }
}

function openGotoLine(): void {
  if (!editor) return
  closeSearch(false)
  gotoOpen.value = true
  gotoMessage.value = ''
  gotoValue.value = ''
  void nextTick(() => gotoInput.value?.focus())
}

function closeGotoLine(focusEditor = true): void {
  gotoOpen.value = false
  gotoMessage.value = ''
  if (focusEditor) editor?.focus()
}

/** Accepts "120" or "120:8" (also with a full-width colon or comma). */
function runGotoLine(): void {
  if (!editor) return
  const match = /^\s*(\d+)(?:\s*[:：,，]\s*(\d+))?\s*$/.exec(gotoValue.value)
  const { doc } = editor.state
  const lineNumber = match ? Number(match[1]) : 0
  if (!match || lineNumber < 1 || lineNumber > doc.lines) {
    gotoMessage.value = phrase(`请输入 1 到 ${doc.lines} 之间的行号`)
    return
  }
  const line = doc.line(lineNumber)
  const position = line.from + Math.min(Math.max(Number(match[2] || 1), 1) - 1, line.length)
  editor.dispatch({
    selection: { anchor: position },
    effects: scrollIntoView?.(position, { y: 'center' }),
  })
  closeGotoLine()
}

defineExpose({
  getValue: currentValue,
  markClean,
  getSession,
  openSearch,
  openGotoLine,
  focus: () => editor?.focus(),
})

async function initialize(): Promise<void> {
  const startedAt = performance.now()
  try {
    const [
      { Compartment, EditorState, StateEffect },
      {
        EditorView,
        crosshairCursor,
        drawSelection,
        dropCursor,
        highlightActiveLine,
        highlightActiveLineGutter,
        highlightSpecialChars,
        keymap,
        rectangularSelection,
      },
      { defaultKeymap, history, historyKeymap, indentWithTab },
      { bracketMatching, foldGutter, foldKeymap, HighlightStyle, indentOnInput, syntaxHighlighting },
      {
        findNext,
        findPrevious,
        getSearchQuery,
        highlightSelectionMatches,
        replaceAll,
        replaceNext,
        search,
        SearchQuery,
        selectNextOccurrence,
        setSearchQuery,
      },
      { tags },
      language,
      { selectableLineNumbers },
      { countSearchMatches, searchMatchHighlighter },
    ] = await Promise.all([
      import('@codemirror/state'),
      import('@codemirror/view'),
      import('@codemirror/commands'),
      import('@codemirror/language'),
      import('@codemirror/search'),
      import('@lezer/highlight'),
      loadCodeLanguage(props.fileName, props.mime, props.sizeBytes),
      import('@/lib/code-editor-line-selection'),
      import('@/lib/code-editor-search'),
    ])
    if (cancelled || !host.value) return

    const highlightStyle = HighlightStyle.define([
      { tag: tags.comment, color: 'var(--code-comment)', fontStyle: 'italic' },
      { tag: [tags.keyword, tags.controlKeyword, tags.operatorKeyword], color: 'var(--code-keyword)' },
      { tag: [tags.string, tags.special(tags.string)], color: 'var(--code-string)' },
      { tag: [tags.number, tags.bool, tags.null], color: 'var(--code-number)' },
      { tag: [tags.function(tags.variableName), tags.definition(tags.variableName)], color: 'var(--code-function)' },
      { tag: [tags.typeName, tags.className], color: 'var(--code-type)' },
      { tag: tags.tagName, color: 'var(--code-tag)' },
      { tag: [tags.attributeName, tags.propertyName], color: 'var(--code-property)' },
      { tag: tags.heading, color: 'var(--code-function)', fontWeight: '600' },
      { tag: tags.link, color: 'var(--code-function)', textDecoration: 'underline' },
      { tag: tags.invalid, color: 'var(--code-tag)', textDecoration: 'underline' },
    ])

    lineWrapCompartment = new Compartment()
    lineWrappingExtension = EditorView.lineWrapping
    scrollIntoView = EditorView.scrollIntoView
    searchRuntime = {
      SearchQuery,
      setSearchQuery,
      getSearchQuery,
      findNext,
      findPrevious,
      replaceNext,
      replaceAll,
      countSearchMatches,
    }
    const extensions = [
      selectableLineNumbers(),
      ...(language.highlighted ? [foldGutter()] : []),
      highlightActiveLineGutter(),
      highlightSpecialChars(),
      history(),
      drawSelection(),
      dropCursor(),
      // Alt-drag selects a column; selectableLineNumbers already allows several ranges.
      rectangularSelection(),
      crosshairCursor(),
      indentOnInput(),
      bracketMatching(),
      highlightActiveLine(),
      search({ top: true }),
      searchMatchHighlighter,
      highlightSelectionMatches(),
      syntaxHighlighting(highlightStyle),
      EditorState.tabSize.of(2),
      EditorState.readOnly.of(!props.editable),
      EditorView.editable.of(props.editable),
      lineWrapCompartment.of(props.lineWrap ? lineWrappingExtension : []),
      EditorView.contentAttributes.of({ 'aria-label': '文件内容' }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged && !applyingExternalValue) emit('change')
        if (update.docChanged && !applyingExternalValue && !dirty) {
          dirty = true
          emit('dirty')
        }
        if (update.docChanged || update.selectionSet) {
          emitStatus(update.state)
          if (searchOpen.value) scheduleSearchCount()
        }
      }),
      EditorView.theme({
        '&': {
          height: '100%',
          color: 'var(--code-text)',
          backgroundColor: 'var(--code-background)',
          fontSize: '14px',
        },
        '.cm-scroller': {
          overflow: 'auto',
          fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace',
          lineHeight: '1.65',
        },
        '.cm-content': {
          minWidth: 'max-content',
          padding: '14px 0',
          caretColor: 'var(--code-caret)',
        },
        '.cm-content.cm-lineWrapping': { minWidth: '0' },
        '.cm-cursor, .cm-dropCursor': {
          borderLeftColor: 'var(--code-caret)',
          borderLeftWidth: '2px',
        },
        '.cm-line': { padding: '0 16px' },
        '.cm-gutters': {
          minWidth: '52px',
          color: 'var(--code-line-number)',
          backgroundColor: 'var(--code-gutter)',
          border: '0',
          boxShadow: 'inset -1px 0 var(--code-gutter-edge)',
        },
        '.cm-lineNumbers .cm-gutterElement': {
          minWidth: '46px',
          padding: '0 10px 0 6px',
          cursor: 'pointer',
        },
        '.cm-lineNumbers .cm-gutterElement:hover': {
          color: 'var(--code-text)',
          backgroundColor: 'var(--code-active-line)',
        },
        '.cm-foldGutter .cm-gutterElement': {
          padding: '0 4px 0 2px',
          color: 'var(--code-line-number)',
          cursor: 'pointer',
        },
        '.cm-foldGutter .cm-gutterElement:hover': { color: 'var(--code-text)' },
        '.cm-activeLine': { backgroundColor: 'var(--code-active-line)' },
        '.cm-activeLineGutter': {
          color: 'var(--code-text)',
          backgroundColor: 'var(--code-active-line)',
        },
        '&.cm-focused': { outline: 'none' },
        '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
          backgroundColor: 'var(--code-selection) !important',
        },
        // drawSelection paints the background; keep each token's foreground.
        '.cm-content ::selection, .cm-content::selection': {
          color: 'currentColor',
          backgroundColor: 'transparent',
        },
        // CodeMirror's own defaults assume a light page; follow the workbench instead.
        '.cm-selectionMatch': { backgroundColor: 'var(--code-selection-match) !important' },
        '&.cm-focused .cm-matchingBracket': {
          color: 'var(--code-text)',
          backgroundColor: 'var(--code-bracket) !important',
          outline: '1px solid var(--code-bracket-border)',
        },
        '&.cm-focused .cm-nonmatchingBracket': {
          backgroundColor: 'var(--code-bracket-mismatch) !important',
        },
        '.cm-foldPlaceholder': {
          margin: '0 4px',
          padding: '0 6px',
          border: '1px solid var(--code-border)',
          borderRadius: 'var(--radius-sm)',
          color: 'var(--code-line-number)',
          backgroundColor: 'var(--code-panel)',
        },
        '.cm-tooltip': {
          color: 'var(--code-text)',
          backgroundColor: 'var(--code-panel)',
          border: '1px solid var(--code-border)',
          borderRadius: 'var(--radius-sm)',
        },
        '.cm-panels': {
          color: 'var(--code-text)',
          backgroundColor: 'var(--code-gutter)',
        },
        '.cm-panels-top': { borderBottom: '1px solid var(--code-border)' },
        '.cm-searchMatch': {
          backgroundColor: 'var(--code-search-match)',
          outline: '1px solid var(--code-search-match-border)',
        },
        '.cm-searchMatch.cm-searchMatch-selected': {
          backgroundColor: 'var(--code-search-match-selected)',
        },
      }),
      keymap.of([
        {
          key: 'Mod-s',
          preventDefault: true,
          run: () => {
            requestSave()
            return true
          },
        },
        indentWithTab,
        {
          key: 'Mod-f',
          preventDefault: true,
          run: () => {
            openSearch()
            return true
          },
        },
        {
          key: 'Mod-h',
          preventDefault: true,
          run: () => {
            openSearch(true)
            return true
          },
        },
        {
          key: 'Mod-g',
          preventDefault: true,
          run: () => {
            openGotoLine()
            return true
          },
        },
        { key: 'Mod-d', preventDefault: true, run: selectNextOccurrence },
        {
          key: 'F3',
          run: () => {
            runSearch('next')
            return true
          },
        },
        {
          key: 'Shift-F3',
          run: () => {
            runSearch('previous')
            return true
          },
        },
        ...foldKeymap,
        ...defaultKeymap,
        ...historyKeymap,
      ]),
    ]
    if (language.extension) extensions.push(language.extension)

    editor = new EditorView({
      parent: host.value,
      state: props.session
        ? props.session.state.update({ effects: StateEffect.reconfigure.of(extensions) }).state
        : EditorState.create({ doc: props.modelValue, extensions }),
    })
    if (props.session) {
      editor.scrollDOM.scrollTop = props.session.scrollTop
      editor.scrollDOM.scrollLeft = props.session.scrollLeft
    }
    if (props.autoFocus && !isTouchPrimary()) editor.focus()
    emitStatus(editor.state)
    emit('ready', { ...language, loadMs: performance.now() - startedAt })
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '编辑器加载失败'
  } finally {
    loading.value = false
  }
}

watch(
  () => props.modelValue,
  (value) => {
    if (!editor || editor.state.doc.toString() === value) return
    applyingExternalValue = true
    editor.dispatch({
      changes: { from: 0, to: editor.state.doc.length, insert: value },
    })
    applyingExternalValue = false
  },
)

watch(
  () => props.lineWrap,
  (enabled) => {
    if (!editor || !lineWrapCompartment || !lineWrappingExtension) return
    editor.dispatch({
      effects: lineWrapCompartment.reconfigure(enabled ? lineWrappingExtension : []),
    })
  },
)

onMounted(() => {
  void initialize()
})

onBeforeUnmount(() => {
  cancelled = true
  clearTimeout(countTimer)
  editor?.destroy()
  editor = undefined
})
</script>

<template>
  <div class="code-editor-shell">
    <div ref="host" class="code-editor-host" />
    <div
      v-if="searchOpen"
      class="code-search"
      role="dialog"
      :aria-label="phrase('查找或替换')"
      @keydown.esc.stop.prevent="closeSearch()"
    >
      <div class="code-search__row">
        <button
          class="code-search__icon code-search__toggle"
          type="button"
          :title="phrase(replaceOpen ? '收起替换' : '展开替换')"
          :aria-label="phrase(replaceOpen ? '收起替换' : '展开替换')"
          :aria-expanded="replaceOpen"
          @click="replaceOpen = !replaceOpen"
        >
          <ChevronDown v-if="replaceOpen" :size="16" /><ChevronRight v-else :size="16" />
        </button>
        <div class="code-search__field">
          <input
            ref="searchInput"
            v-model="searchValue"
            type="text"
            :placeholder="phrase('查找')"
            :aria-label="phrase('查找内容')"
            spellcheck="false"
            @input="updateSearchQuery"
            @keydown.enter.exact.prevent="runSearch('next')"
            @keydown.shift.enter.prevent="runSearch('previous')"
          />
          <span
            v-if="searchStatus"
            class="code-search__count"
            :class="{ 'is-error': searchMessage || searchCount?.total === 0 }"
            role="status"
          >{{ searchStatus }}</span>
          <button
            class="code-search__option"
            :class="{ 'is-active': searchCaseSensitive }"
            type="button"
            :title="phrase('区分大小写')"
            :aria-label="phrase('区分大小写')"
            :aria-pressed="searchCaseSensitive"
            @click="toggleSearchOption('case')"
          ><CaseSensitive :size="17" /></button>
          <button
            class="code-search__option"
            :class="{ 'is-active': searchWholeWord }"
            type="button"
            :title="phrase('全字匹配')"
            :aria-label="phrase('全字匹配')"
            :aria-pressed="searchWholeWord"
            @click="toggleSearchOption('word')"
          ><WholeWord :size="17" /></button>
          <button
            class="code-search__option"
            :class="{ 'is-active': searchRegexp }"
            type="button"
            :title="phrase('使用正则表达式')"
            :aria-label="phrase('使用正则表达式')"
            :aria-pressed="searchRegexp"
            @click="toggleSearchOption('regexp')"
          ><Regex :size="16" /></button>
        </div>
        <button class="code-search__icon" type="button" :title="phrase('上一个（Shift+F3）')" :aria-label="phrase('上一个匹配')" :disabled="!searchCount?.total" @click="runSearch('previous')">
          <ArrowUp :size="16" />
        </button>
        <button class="code-search__icon" type="button" :title="phrase('下一个（F3）')" :aria-label="phrase('下一个匹配')" :disabled="!searchCount?.total" @click="runSearch('next')">
          <ArrowDown :size="16" />
        </button>
        <button class="code-search__icon" type="button" :title="phrase('关闭（Esc）')" :aria-label="phrase('关闭查找')" @click="closeSearch()">
          <X :size="16" />
        </button>
      </div>
      <div v-if="replaceOpen" class="code-search__row code-search__replace">
        <span class="code-search__toggle" aria-hidden="true" />
        <div class="code-search__field">
          <input
            v-model="replaceValue"
            type="text"
            :placeholder="phrase('替换')"
            :aria-label="phrase('替换内容')"
            spellcheck="false"
            :disabled="!props.editable"
            @input="updateSearchQuery"
            @keydown.enter.prevent="runReplace(false)"
          />
        </div>
        <button class="code-search__icon" type="button" :title="phrase('替换')" :aria-label="phrase('替换当前匹配')" :disabled="!props.editable || !searchCount?.total" @click="runReplace(false)">
          <Replace :size="16" />
        </button>
        <button class="code-search__icon" type="button" :title="phrase('全部替换')" :aria-label="phrase('替换全部匹配')" :disabled="!props.editable || !searchCount?.total" @click="runReplace(true)">
          <ReplaceAll :size="16" />
        </button>
        <span class="code-search__icon" aria-hidden="true" />
      </div>
    </div>
    <div
      v-else-if="gotoOpen"
      class="code-search code-goto"
      role="dialog"
      :aria-label="phrase('跳转到行')"
      @keydown.esc.stop.prevent="closeGotoLine()"
    >
      <div class="code-goto__row">
        <div class="code-search__field">
          <input
            ref="gotoInput"
            v-model="gotoValue"
            type="text"
            inputmode="numeric"
            :placeholder="phrase('行号，或 行:列')"
            :aria-label="phrase('跳转到行')"
            :aria-invalid="Boolean(gotoMessage)"
            spellcheck="false"
            @input="gotoMessage = ''"
            @keydown.enter.prevent="runGotoLine"
          />
          <span class="code-search__count" data-i18n-ignore>1–{{ documentLines }}</span>
        </div>
        <button class="code-search__icon" type="button" :title="phrase('跳转')" :aria-label="phrase('跳转')" @click="runGotoLine">
          <CornerDownLeft :size="16" />
        </button>
        <button class="code-search__icon" type="button" :title="phrase('关闭（Esc）')" :aria-label="phrase('关闭跳转')" @click="closeGotoLine()">
          <X :size="16" />
        </button>
      </div>
      <p v-if="gotoMessage" class="code-goto__message" role="alert">{{ gotoMessage }}</p>
    </div>
    <div v-if="loading" class="code-editor-state">{{ phrase('正在加载代码编辑器…') }}</div>
    <div v-else-if="loadError" class="code-editor-state code-editor-state--error">
      {{ phrase('代码编辑器加载失败：') }}{{ loadError }}
    </div>
  </div>
</template>

<style scoped>
.code-editor-shell {
  --code-background: var(--file-preview-background, var(--terminal-shell-background, #0b1214));
  --code-gutter: color-mix(in srgb, var(--file-preview-panel, var(--terminal-shell-panel, #111a1d)) 72%, var(--code-background) 28%);
  --code-gutter-edge: color-mix(in srgb, var(--code-border) 55%, transparent);
  --code-panel: var(--file-preview-panel, var(--terminal-shell-panel, #111a1d));
  --code-text: var(--file-preview-text, var(--terminal-shell-text, #d8dddc));
  --code-caret: color-mix(in srgb, var(--file-preview-accent, var(--brand)) 50%, var(--code-text));
  --code-line-number: var(--file-preview-muted, var(--terminal-shell-muted, #8a9695));
  --code-active-line: var(--file-preview-active-line, rgb(53 203 166 / 8%));
  --code-selection: color-mix(in srgb, var(--file-preview-accent, var(--brand)) 18%, var(--code-background));
  --code-selection-match: color-mix(in srgb, var(--file-preview-accent, var(--brand)) 13%, transparent);
  --code-bracket: color-mix(in srgb, var(--file-preview-accent, var(--brand)) 24%, transparent);
  --code-bracket-border: color-mix(in srgb, var(--file-preview-accent, var(--brand)) 58%, transparent);
  --code-bracket-mismatch: color-mix(in srgb, var(--danger) 32%, transparent);
  --code-border: var(--file-preview-border, var(--terminal-shell-border, #29383a));
  /* Syntax colors follow the preview surface in both light and dark themes. */
  --code-comment: color-mix(in srgb, var(--code-line-number) 65%, var(--code-text));
  --code-keyword: color-mix(in srgb, var(--violet) 50%, var(--code-text));
  --code-string: color-mix(in srgb, var(--success) 50%, var(--code-text));
  --code-number: color-mix(in srgb, var(--amber) 50%, var(--code-text));
  --code-function: color-mix(in srgb, var(--blue) 50%, var(--code-text));
  --code-type: color-mix(in srgb, var(--amber) 35%, var(--code-text));
  --code-tag: color-mix(in srgb, var(--danger) 50%, var(--code-text));
  --code-property: var(--code-caret);
  --code-search-match: color-mix(in srgb, var(--amber) 22%, transparent);
  --code-search-match-border: color-mix(in srgb, var(--amber) 62%, transparent);
  --code-search-match-selected: color-mix(in srgb, var(--amber) 46%, transparent);
  --scrollbar-track: var(--code-background);
  --scrollbar-thumb: var(--file-preview-scrollbar, var(--terminal-shell-scrollbar, #35474a));
  --scrollbar-thumb-hover: var(--file-preview-scrollbar-hover, var(--terminal-shell-scrollbar-hover, #506367));
  --scrollbar-thumb-active: var(--code-caret);
  position: relative;
  height: 100%;
  overflow: hidden;
  background: var(--code-background);
  box-shadow: inset 0 0 0 1px var(--code-border);
}

.code-editor-host {
  height: 100%;
}

.code-search {
  position: absolute;
  z-index: 5;
  top: 10px;
  right: 14px;
  width: min(460px, calc(100% - 28px));
  padding: 6px;
  border: 1px solid var(--code-border);
  border-radius: var(--radius);
  color: var(--code-text);
  background: var(--code-panel);
  box-shadow: var(--shadow-md);
}

.code-goto {
  width: min(340px, calc(100% - 28px));
}

.code-search__row {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) 32px 32px 32px;
  align-items: center;
  gap: 3px;
}

.code-goto__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 32px 32px;
  align-items: center;
  gap: 3px;
}

.code-search__replace {
  margin-top: 4px;
}

.code-search__field {
  display: flex;
  min-width: 0;
  height: 34px;
  align-items: center;
  gap: 1px;
  padding-right: 2px;
  overflow: hidden;
  border: 1px solid var(--code-border);
  border-radius: var(--radius-sm);
  background: var(--code-background);
  transition: border-color var(--motion-duration-fast) var(--motion-ease-fade), box-shadow var(--motion-duration-fast) var(--motion-ease-fade);
}

.code-search__field:focus-within {
  border-color: var(--code-caret);
  box-shadow: 0 0 0 1px var(--code-caret);
}

.code-search__field input {
  min-width: 0;
  height: 100%;
  flex: 1;
  padding: 0 7px;
  border: 0;
  outline: 0;
  color: var(--code-text);
  background: transparent;
  font: 14px/1.4 ui-monospace, SFMono-Regular, Consolas, monospace;
}

.code-search__field input::placeholder {
  color: var(--code-line-number);
}

/* Match position and totals use tabular digits so the field does not jitter. */
.code-search__count {
  flex: 0 0 auto;
  padding: 0 6px;
  color: var(--code-line-number);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.code-search__count.is-error {
  color: var(--danger, #ef7a7a);
}

.code-search__icon,
.code-search__option {
  display: inline-grid;
  width: 32px;
  height: 32px;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--code-line-number);
  background: transparent;
  cursor: pointer;
  transition: color var(--motion-duration-fast) var(--motion-ease-fade), background-color var(--motion-duration-fast) var(--motion-ease-fade);
}

.code-search__option {
  width: 28px;
  height: 28px;
}

.code-search__toggle {
  width: 28px;
}

.code-search__icon:hover:not(:disabled),
.code-search__option:hover,
.code-search__option.is-active {
  color: var(--code-text);
  background: color-mix(in srgb, var(--code-caret) 14%, var(--code-panel));
}

.code-search__option.is-active {
  color: var(--code-caret);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--code-caret) 55%, transparent);
}

.code-search__icon:disabled {
  opacity: 0.35;
  cursor: default;
}

.code-goto__message {
  margin: 6px 4px 2px;
  color: var(--danger, #ef7a7a);
  font-size: 13px;
  line-height: 1.5;
}

@media (max-width: 620px) {
  .code-search {
    top: 7px;
    right: 7px;
    width: calc(100% - 14px);
  }

  .code-search__row {
    grid-template-columns: 28px minmax(0, 1fr) 36px 36px 36px;
  }

  .code-goto__row {
    grid-template-columns: minmax(0, 1fr) 36px 36px;
  }

  .code-search__field { height: auto; min-height: 40px; flex-wrap: wrap; }
  .code-search__field input { flex-basis: 100%; height: 38px; font-size: 16px; }
  .code-goto .code-search__field { flex-wrap: nowrap; }
  .code-goto .code-search__field input { flex-basis: auto; }
  .code-search__icon, .code-search__option { min-height: 40px; }
  .code-search__option { width: 36px; }
}

.code-search button:focus-visible { outline: 2px solid var(--code-caret); outline-offset: -2px; }

.code-editor-state {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: var(--code-line-number);
  background: var(--code-background);
}

.code-editor-state--error {
  color: var(--danger);
}
</style>
