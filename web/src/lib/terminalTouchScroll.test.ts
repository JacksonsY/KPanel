import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Terminal } from '@xterm/xterm'
import { createTerminalTouchScroll } from './terminalTouchScroll'

function touchEvent(clientY: number): TouchEvent {
  return {
    touches: {
      length: 1,
      item: () => ({ clientY }),
    },
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  } as unknown as TouchEvent
}

describe('terminal touch scrolling', () => {
  it('converts a vertical swipe into terminal buffer rows', () => {
    const scrollLines = vi.fn()
    const touchScroll = createTerminalTouchScroll({
      getTerminal: () => ({ rows: 10, scrollLines }),
      getScreen: () => ({
        getBoundingClientRect: () => ({ height: 160 }),
      }) as HTMLElement,
    })
    const start = touchEvent(100)
    const move = touchEvent(68)

    touchScroll.start(start)
    touchScroll.move(move)

    expect(scrollLines).toHaveBeenCalledWith(2)
    expect(move.preventDefault).toHaveBeenCalledOnce()
    expect(move.stopPropagation).toHaveBeenCalledOnce()
  })

  it('clears the gesture after touch end', () => {
    const scrollLines = vi.fn()
    const touchScroll = createTerminalTouchScroll({
      getTerminal: () => ({ rows: 10, scrollLines }),
      getScreen: () => ({
        getBoundingClientRect: () => ({ height: 160 }),
      }) as HTMLElement,
    })

    touchScroll.start(touchEvent(100))
    touchScroll.end()
    touchScroll.move(touchEvent(60))

    expect(scrollLines).not.toHaveBeenCalled()
  })
})

describe('terminal touch selection', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  function setup(text = 'hello world         ') {
    const select = vi.fn(), scrollLines = vi.fn(), showSelectionMenu = vi.fn()
    const terminal = {
      rows: 3, cols: 20, scrollLines, select, blur: vi.fn(),
      buffer: { active: { viewportY: 7, getLine: vi.fn(() => ({ getCell: (column: number) => ({
        getChars: () => text[column] ?? '', getWidth: () => 1,
      }) })) } },
    } as unknown as Terminal
    const gesture = createTerminalTouchScroll({
      getTerminal: () => terminal, getSelectionTerminal: () => terminal, showSelectionMenu,
      getScreen: () => ({ getBoundingClientRect: () => ({ left: 10, top: 20, right: 210, bottom: 80, width: 200, height: 60 }) }) as HTMLElement,
    })
    const event = (x: number, y: number, type = 'touchmove', count = 1) => ({
      type, touches: { length: count, item: () => ({ clientX: x, clientY: y }) },
      preventDefault: vi.fn(), stopPropagation: vi.fn(),
    }) as unknown as TouchEvent
    return { terminal, gesture, event, select, scrollLines, showSelectionMenu }
  }

  it('selects the pressed word at the scrolled buffer row and opens copy only on release', () => {
    const { gesture, event, terminal, select, showSelectionMenu } = setup()
    const start = event(85, 30, 'touchstart')
    gesture.start(start)
    vi.advanceTimersByTime(550)
    expect(select).toHaveBeenLastCalledWith(6, 7, 5)
    expect(terminal.blur).toHaveBeenCalledOnce()
    expect(start.preventDefault).toHaveBeenCalledOnce()
    expect(showSelectionMenu).not.toHaveBeenCalled()
    gesture.end(event(85, 30, 'touchend', 0))
    expect(showSelectionMenu).toHaveBeenCalledWith(85, 30)
  })

  it('extends the selected word across rows while dragging without scrolling', () => {
    const { gesture, event, select, scrollLines } = setup()
    gesture.start(event(85, 30))
    vi.advanceTimersByTime(550)
    gesture.move(event(115, 50))
    expect(select).toHaveBeenLastCalledWith(6, 7, 25)
    gesture.move(event(15, 30))
    expect(select).toHaveBeenLastCalledWith(0, 7, 11)
    expect(scrollLines).not.toHaveBeenCalled()
  })

  it('allows small finger jitter but cancels long press when swiping', () => {
    const { gesture, event, select, scrollLines, showSelectionMenu } = setup()
    gesture.start(event(85, 70))
    gesture.move(event(87, 68))
    vi.advanceTimersByTime(200)
    gesture.move(event(87, 28))
    vi.advanceTimersByTime(600)
    gesture.end(event(87, 28, 'touchend', 0))
    expect(select).not.toHaveBeenCalled()
    expect(scrollLines).toHaveBeenCalledWith(2)
    expect(showSelectionMenu).not.toHaveBeenCalled()
  })

  it.each(['touchend', 'touchcancel'])('cancels a pending hold on %s', type => {
    const { gesture, event, select, showSelectionMenu } = setup()
    gesture.start(event(85, 30))
    gesture.end(event(85, 30, type, 0))
    vi.advanceTimersByTime(600)
    expect(select).not.toHaveBeenCalled()
    expect(showSelectionMenu).not.toHaveBeenCalled()
  })

  it('does not open the copy menu on cancellation or multiple fingers', () => {
    const { gesture, event, showSelectionMenu } = setup()
    gesture.start(event(85, 30))
    vi.advanceTimersByTime(550)
    gesture.end(event(85, 30, 'touchcancel', 0))
    gesture.start(event(85, 30))
    vi.advanceTimersByTime(550)
    gesture.move(event(85, 30, 'touchmove', 2))
    gesture.end(event(85, 30, 'touchend', 0))
    expect(showSelectionMenu).not.toHaveBeenCalled()
  })

  it('ignores blank cells and touches on the scrollbar outside the screen', () => {
    const { gesture, event, select, showSelectionMenu } = setup()
    gesture.start(event(165, 30))
    vi.advanceTimersByTime(550)
    gesture.end(event(165, 30, 'touchend', 0))
    gesture.start(event(215, 30))
    vi.advanceTimersByTime(550)
    expect(select).not.toHaveBeenCalled()
    expect(showSelectionMenu).not.toHaveBeenCalled()
  })

  it('selects complete wide characters from their trailing cell', () => {
    const { gesture, event, terminal, select } = setup()
    vi.mocked(terminal.buffer.active.getLine).mockReturnValue({ getCell: (column: number) => ({
      getChars: () => column === 0 ? '中' : '', getWidth: () => column === 0 ? 2 : column === 1 ? 0 : 1,
    }) } as ReturnType<typeof terminal.buffer.active.getLine>)
    gesture.start(event(25, 30))
    vi.advanceTimersByTime(550)
    expect(select).toHaveBeenLastCalledWith(0, 7, 2)
  })
})
