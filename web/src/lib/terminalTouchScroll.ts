import type { Terminal } from '@xterm/xterm'

interface TerminalScrollTarget {
  rows: number
  scrollLines(lines: number): void
}

interface TerminalTouchScrollOptions {
  getTerminal: () => TerminalScrollTarget | undefined
  getScreen: () => HTMLElement | undefined
  getSelectionTerminal?: () => Terminal | undefined
  onLongPress?: (action: 'copy' | 'paste' | 'menu', x: number, y: number) => void
  onTap?: () => void
}

export function createTerminalTouchScroll(options: TerminalTouchScrollOptions) {
  let lastY: number | undefined
  let remainder = 0
  let origin: { x: number; y: number } | undefined
  let anchor: { start: number; end: number } | undefined
  let lastPoint: { x: number; y: number } | undefined
  let holdTimer: ReturnType<typeof setTimeout> | undefined
  let held = false
  let pasteOnHold = false

  function cancelHold(): void {
    clearTimeout(holdTimer)
    holdTimer = undefined
  }

  function cellAt(x: number, y: number): number | undefined {
    const terminal = options.getSelectionTerminal?.()
    const rect = options.getScreen()?.getBoundingClientRect()
    if (!terminal || !rect?.width || !rect.height) return undefined
    const column = Math.max(0, Math.min(terminal.cols - 1, Math.floor((x - rect.left) / rect.width * terminal.cols)))
    const row = Math.max(0, Math.min(terminal.rows - 1, Math.floor((y - rect.top) / rect.height * terminal.rows)))
    return (terminal.buffer.active.viewportY + row) * terminal.cols + column
  }

  function select(start: number, end: number): void {
    const terminal = options.getSelectionTerminal?.()
    if (terminal) terminal.select(start % terminal.cols, Math.floor(start / terminal.cols), end - start)
  }

  function selectWord(): void {
    holdTimer = undefined
    held = true
    const terminal = options.getSelectionTerminal?.()
    if (!origin || !terminal) return
    const index = cellAt(origin.x, origin.y)
    if (index === undefined) return
    const row = Math.floor(index / terminal.cols)
    const line = terminal.buffer.active.getLine(row)
    if (!line) return
    let start = index % terminal.cols
    while (start > 0 && line.getCell(start)?.getWidth() === 0) start--
    const isText = (column: number) => {
      const cell = line.getCell(column)
      return cell && (cell.getWidth() === 0 || /\S/u.test(cell.getChars()))
    }
    if (!isText(start)) { pasteOnHold = true; return }
    let end = start + 1
    while (start > 0 && isText(start - 1)) start--
    while (end < terminal.cols && isText(end)) end++
    anchor = { start: row * terminal.cols + start, end: row * terminal.cols + end }
    terminal.blur()
    select(anchor.start, anchor.end)
  }

  function touchY(event: TouchEvent): number | undefined {
    if (event.touches.length !== 1) return undefined
    return event.touches.item(0)?.clientY
  }

  function start(event: TouchEvent): void {
    end()
    lastY = touchY(event)
    remainder = 0
    const touch = event.touches.length === 1 ? event.touches.item(0) : null
    const screen = options.getScreen()
    if (!touch || !screen || !options.getSelectionTerminal?.()) return
    const rect = screen.getBoundingClientRect()
    if (touch.clientX < rect.left || touch.clientX >= rect.right || touch.clientY < rect.top || touch.clientY >= rect.bottom) return
    origin = lastPoint = { x: touch.clientX, y: touch.clientY }
    // xterm renders into a canvas; suppress native callouts and synthesize its buffer selection.
    event.preventDefault()
    holdTimer = setTimeout(selectWord, 550)
  }

  function move(event: TouchEvent): void {
    const currentY = touchY(event)
    const target = options.getTerminal()
    if (currentY === undefined || lastY === undefined || !target) {
      end()
      return
    }

    const touch = event.touches.item(0)!
    lastPoint = { x: touch.clientX, y: touch.clientY }
    if (held && !anchor) {
      // A paste hold must remain stationary; dragging away cancels the action.
      if (origin && Math.hypot(touch.clientX - origin.x, touch.clientY - origin.y) >= 8) end()
      event.preventDefault()
      event.stopPropagation()
      return
    }
    if (anchor) {
      const index = cellAt(touch.clientX, touch.clientY)
      if (index !== undefined) {
        const terminal = options.getSelectionTerminal!()!
        const cell = terminal.buffer.active.getLine(Math.floor(index / terminal.cols))?.getCell(index % terminal.cols)
        const width = cell?.getWidth() ?? 1
        const start = width === 0 ? index - 1 : index
        select(Math.min(anchor.start, start), Math.max(anchor.end, index + Math.max(1, width)))
      }
      event.preventDefault()
      event.stopPropagation()
      return
    }
    if (origin && holdTimer !== undefined) {
      if (Math.hypot(touch.clientX - origin.x, touch.clientY - origin.y) < 8) {
        event.preventDefault()
        return
      }
      cancelHold()
    }

    remainder += lastY - currentY
    lastY = currentY

    const screenHeight = options.getScreen()?.getBoundingClientRect().height || 0
    const cellHeight = target.rows > 0 && screenHeight > 0 ? screenHeight / target.rows : 16
    const lines = Math.trunc(remainder / Math.max(1, cellHeight))
    if (lines !== 0) {
      target.scrollLines(lines)
      remainder -= lines * cellHeight
    }

    event.preventDefault()
    event.stopPropagation()
  }

  function end(event?: TouchEvent): void {
    const point = held && lastPoint
    let action: 'copy' | 'paste' | 'menu' = 'menu'
    if (pasteOnHold) action = 'paste'
    else if (anchor) action = 'copy'
    const tapped = origin && holdTimer !== undefined
    cancelHold()
    origin = anchor = lastPoint = undefined
    held = pasteOnHold = false
    lastY = undefined
    remainder = 0
    if (event?.type === 'touchend' && point) {
      event.preventDefault()
      event.stopPropagation()
      options.onLongPress?.(action, point.x, point.y)
    } else if (event?.type === 'touchend' && tapped) {
      // Explicit touch release is a user gesture, so mobile browsers may show the keyboard.
      event.preventDefault()
      options.onTap?.()
    }
  }

  return { start, move, end }
}
