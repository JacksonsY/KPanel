import { RangeSetBuilder, type EditorState } from '@codemirror/state'
import { Decoration, ViewPlugin, type DecorationSet, type EditorView, type ViewUpdate } from '@codemirror/view'
import { getSearchQuery, type SearchQuery } from '@codemirror/search'

/** Counting stops here so each keystroke stays cheap on large files. */
export const SEARCH_COUNT_LIMIT = 1000

export interface SearchMatchCount {
  total: number
  /** 1-based position of the main selection among the matches, or 0. */
  current: number
  capped: boolean
}

const matchMark = Decoration.mark({ class: 'cm-searchMatch' })
const selectedMatchMark = Decoration.mark({ class: 'cm-searchMatch cm-searchMatch-selected' })

function isSelected(state: EditorState, from: number, to: number): boolean {
  return state.selection.ranges.some((range) => range.from === from && range.to === to)
}

/** CodeMirror paints matches only while its own search panel is open. The
 * editor draws its own find bar, so this paints every valid query instead. */
export const searchMatchHighlighter = ViewPlugin.fromClass(class {
  decorations: DecorationSet
  constructor(readonly view: EditorView) {
    this.decorations = this.build()
  }
  update(update: ViewUpdate): void {
    if (update.docChanged || update.selectionSet || update.viewportChanged
      || getSearchQuery(update.state) !== getSearchQuery(update.startState)) this.decorations = this.build()
  }
  build(): DecorationSet {
    const { state, visibleRanges } = this.view
    const query = getSearchQuery(state)
    if (!query.valid) return Decoration.none
    const builder = new RangeSetBuilder<Decoration>()
    let last = -1
    for (const { from, to } of visibleRanges) {
      const cursor = query.getCursor(state, from, to)
      for (let next = cursor.next(); !next.done; next = cursor.next()) {
        const match = next.value
        if (match.from === match.to || match.from < last) continue
        builder.add(match.from, match.to, isSelected(state, match.from, match.to) ? selectedMatchMark : matchMark)
        last = match.to
      }
    }
    return builder.finish()
  }
}, { decorations: (plugin) => plugin.decorations })

export function countSearchMatches(state: EditorState, query: SearchQuery, limit = SEARCH_COUNT_LIMIT): SearchMatchCount {
  if (!query.valid) return { total: 0, current: 0, capped: false }
  const { from, to } = state.selection.main
  let total = 0
  let current = 0
  const cursor = query.getCursor(state)
  for (let next = cursor.next(); !next.done; next = cursor.next()) {
    if (next.value.from === next.value.to) continue
    total += 1
    if (next.value.from === from && next.value.to === to) current = total
    if (total >= limit) return { total, current, capped: true }
  }
  return { total, current, capped: false }
}
