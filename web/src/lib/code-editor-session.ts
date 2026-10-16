import type { EditorState } from '@codemirror/state'

/** In-memory only: immutable document, history and selection, without an idle DOM view. */
export interface CodeEditorSession {
  state: EditorState
  scrollTop: number
  scrollLeft: number
}

/** Cursor and document facts the editor reports after every change. */
export interface CodeEditorStatus {
  line: number
  column: number
  lines: number
  /** Characters across every selection range. */
  selected: number
  cursors: number
}
