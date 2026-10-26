import { WebLinksAddon } from '@xterm/addon-web-links'
import type { ILink, ILinkProvider, Terminal } from '@xterm/xterm'
import { openTerminalURL } from './terminalLinks'

// TUI renderers address each row explicitly, so xterm's isWrapped is false.
// Only join visible, underlined URL cells; never trust OSC 8 destinations.
export function terminalWrappedLink(terminal: Terminal, y: number): ILink | undefined {
  const buffer = terminal.buffer.active
  const rows: Array<{ y: number; x: number; end: number; text: string }> = []
  const read = (row: number) => {
    const line = buffer.getLine(row - 1)
    if (!line) return undefined
    let x = 0
    while (x < line.length && !line.getCell(x)?.getChars().trim()) x++
    const start = x
    let text = ''
    for (; x < line.length; x++) {
      const cell = line.getCell(x)
      const chars = cell?.getChars() || ''
      if (!cell?.isUnderline() || !/^[\x21-\x7e]+$/.test(chars)) break
      text += chars
    }
    // Don't join prose or a second URL into the first one.
    if (!text || line.translateToString(true, x).trim()) return undefined
    return { y: row, x: start + 1, end: x, text }
  }
  let row = read(y)
  if (!row) return undefined
  rows.push(row)
  let length = row.text.length
  while (!/^https?:\/\//i.test(rows[0]!.text) && rows.length < 64 && length <= 16384) {
    row = read(rows[0]!.y - 1)
    if (!row || row.end < terminal.cols - 2) return undefined
    rows.unshift(row)
    length += row.text.length
  }
  if (!/^https?:\/\//i.test(rows[0]!.text)) return undefined
  while (rows.length < 64 && length <= 16384) {
    if (rows[rows.length - 1]!.end < terminal.cols - 2) break
    row = read(rows[rows.length - 1]!.y + 1)
    if (!row || /^https?:\/\//i.test(row.text)) break
    rows.push(row)
    length += row.text.length
  }
  if (rows.length < 2 || rows.length >= 64 || length > 16384) return undefined
  const first = rows[0]!, last = rows[rows.length - 1]!
  const text = rows.map(row => row.text).join('')
  try {
    if (!['http:', 'https:'].includes(new URL(text).protocol)) return undefined
  } catch { return undefined }
  return {
    text,
    range: { start: { x: first.x, y: first.y }, end: { x: last.end, y: last.y } },
    activate: () => { openTerminalURL(text) },
  }
}

export function terminalSelectionText(terminal: Terminal | undefined): string {
  const text = terminal?.getSelection() || ''
  const position = terminal?.getSelectionPosition?.()
  if (!terminal || !position || !text.includes('\n')) return text
  const link = terminalWrappedLink(terminal, position.start.y + 1)
  return link && text.split(/\r?\n/).map(line => line.trim()).join('') === link.text ? link.text : text
}

export class TerminalWebLinksAddon extends WebLinksAddon {
  private wrappedProvider?: ReturnType<Terminal['registerLinkProvider']>

  override activate(terminal: Terminal): void {
    const provider: ILinkProvider = {
      provideLinks(y, callback) {
        const link = terminalWrappedLink(terminal, y)
        callback(link ? [link] : undefined)
      },
    }
    this.wrappedProvider = terminal.registerLinkProvider(provider)
    // xterm gives earlier providers priority over the ordinary partial URL.
    super.activate(terminal)
  }

  override dispose(): void {
    this.wrappedProvider?.dispose()
    super.dispose()
  }
}
