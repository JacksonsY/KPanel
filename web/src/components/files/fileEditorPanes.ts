import type { FileEntry } from '@/types/api'

/** The part of an editor tab that the split panes read. */
export interface EditorPaneTab {
  entry: FileEntry
  dirty: boolean
  saving: boolean
}
