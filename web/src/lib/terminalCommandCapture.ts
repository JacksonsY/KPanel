import type { IBuffer, IDisposable, IMarker, Terminal } from '@xterm/xterm'

// Recovers the commands typed at a shell prompt from what the terminal shows.
// The text left of the cursor at the first keystroke of a line is that line's
// prompt; it must end like a shell prompt, so script menus ("请输入你的选择: "),
// password prompts and REPLs are never recorded. When Enter is pressed the
// line is read only after the shell echoes the newline, so tab completion,
// history recall and line editing are reflected as the shell ran them. Text
// printed before the prompt stays part of the remembered prompt and can never
// be mistaken for the command.
const promptEnding = /[#$%❯➜»]\s*$/u
const maxPromptLength = 512
const maxLogicalRows = 64
const settleTimeout = 5000
const maxPending = 4
// Replies the terminal sends on its own (cursor and device reports, focus and
// mouse events, OSC/DCS answers) are not keystrokes.
const terminalReport = /^(?:\x1b\[(?:\d+;\d+R|[IO]|[?>][\d;]*c|[\d;]*t|<\d+;\d+;\d+[Mm])|\x1b\[M[\s\S]{3}|\x1b\][\s\S]*(?:\x07|\x1b\\)|\x1bP[\s\S]*\x1b\\)$/

type CaptureTerminal = Pick<Terminal, 'buffer' | 'registerMarker' | 'onLineFeed' | 'onWriteParsed'>

export interface TerminalCommandCapture {
  /** Observes input the user sent to the shell, before it is transmitted. */
  input(data: string): void
  /** Forgets the line in progress, e.g. after input the user did not type. */
  reset(): void
  dispose(): void
}

interface PendingCapture {
  marker: IMarker
  prompt: string
  timer: ReturnType<typeof setTimeout>
}

export function extractPromptCommand(prompt: string, line: string): string | undefined {
  if (!prompt.trim() || prompt.length > maxPromptLength || !promptEnding.test(prompt)) return undefined
  if (!line.startsWith(prompt)) return undefined
  const typed = line.slice(prompt.length)
  // A leading space keeps a command out of history, as with bash ignorespace.
  if (!typed.trim() || /^\s/.test(typed)) return undefined
  return typed.trimEnd()
}

function logicalRange(buffer: IBuffer, row: number): { start: number; end: number } {
  let start = row
  while (start > 0 && row - start < maxLogicalRows && buffer.getLine(start)?.isWrapped) start -= 1
  let end = row
  while (end - start < maxLogicalRows && buffer.getLine(end + 1)?.isWrapped) end += 1
  return { start, end }
}

function promptBeforeCursor(buffer: IBuffer): string {
  const row = buffer.baseY + buffer.cursorY
  const { start } = logicalRange(buffer, row)
  let text = ''
  for (let y = start; y < row; y += 1) text += buffer.getLine(y)?.translateToString(false) ?? ''
  return text + (buffer.getLine(row)?.translateToString(false, 0, buffer.cursorX) ?? '')
}

function logicalLineText(buffer: IBuffer, start: number, end: number): string {
  let text = ''
  for (let y = start; y <= end; y += 1) text += buffer.getLine(y)?.translateToString(y === end) ?? ''
  return text
}

function isEnter(data: string): boolean {
  return data.endsWith('\r') && data.indexOf('\r') === data.length - 1 && !data.includes('\n') && !data.includes('\x1b[200~')
}

export function createTerminalCommandCapture(
  terminal: CaptureTerminal,
  onCommand: (command: string) => void,
): TerminalCommandCapture {
  // undefined: no keystroke on this line yet; null: this line is not at a prompt.
  let prompt: string | null | undefined
  let disposed = false
  const pending: PendingCapture[] = []

  function drop(item: PendingCapture): void {
    clearTimeout(item.timer)
    item.marker.dispose()
    const index = pending.indexOf(item)
    if (index >= 0) pending.splice(index, 1)
  }

  // Reads the oldest Enter once the shell has moved past its line.
  function settle(): void {
    while (pending.length) {
      const item = pending[0]!
      if (item.marker.isDisposed || item.marker.line < 0) {
        drop(item)
        continue
      }
      const buffer = terminal.buffer.normal
      const { start, end } = logicalRange(buffer, item.marker.line)
      // A full-screen program started by the command switches buffers right
      // after the newline; the command line stays intact in the normal buffer.
      if (terminal.buffer.active.type === 'normal' && buffer.baseY + buffer.cursorY <= end) return
      const command = extractPromptCommand(item.prompt, logicalLineText(buffer, start, end))
      drop(item)
      if (command) onCommand(command)
    }
  }

  const subscriptions: IDisposable[] = [terminal.onLineFeed(settle), terminal.onWriteParsed(settle)]

  function capture(linePrompt: string): void {
    const marker = terminal.registerMarker(0)
    if (!marker) return
    while (pending.length >= maxPending) drop(pending[0]!)
    const item: PendingCapture = { marker, prompt: linePrompt, timer: setTimeout(() => drop(item), settleTimeout) }
    pending.push(item)
  }

  return {
    input(data) {
      if (disposed || !data || terminalReport.test(data)) return
      if (terminal.buffer.active.type !== 'normal') {
        prompt = undefined
        return
      }
      if (prompt === undefined) {
        const text = promptBeforeCursor(terminal.buffer.active)
        prompt = promptEnding.test(text) && text.length <= maxPromptLength ? text : null
      }
      if (isEnter(data)) {
        if (prompt) capture(prompt)
        prompt = undefined
      } else if (/[\r\n\x03\x04]/.test(data)) {
        prompt = undefined
      }
    },
    reset() {
      prompt = undefined
    },
    dispose() {
      disposed = true
      for (const item of [...pending]) drop(item)
      for (const subscription of subscriptions) subscription.dispose()
    },
  }
}
