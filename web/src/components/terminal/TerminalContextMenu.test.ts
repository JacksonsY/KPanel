// @vitest-environment jsdom

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { Terminal } from '@xterm/xterm'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import TerminalContextMenu from './TerminalContextMenu.vue'

interface ExposedMenu {
  handleTouchHold: (action: 'copy' | 'paste' | 'menu', x: number, y: number) => Promise<void>
  open: (event: MouseEvent) => void
  handlePaste: (event: ClipboardEvent) => void
  handleKeyEvent: (event: KeyboardEvent) => boolean
  handleSelectionPointerDown: (event: PointerEvent) => void
}

function rect(left: number, top: number, width: number, height: number): DOMRect {
  return {
    x: left,
    y: top,
    left,
    top,
    right: left + width,
    bottom: top + height,
    width,
    height,
    toJSON: () => ({}),
  }
}

describe('TerminalContextMenu', () => {
  const terminal = {
    focus: vi.fn(),
    getSelection: vi.fn(() => 'selected output'),
    hasSelection: vi.fn(() => true),
    paste: vi.fn(),
    selectAll: vi.fn(),
  }
  const writeText = vi.fn(() => Promise.resolve())
  const readText = vi.fn(() => Promise.resolve('pasted command'))
  let wrapper: VueWrapper

  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText, readText },
    })
    wrapper = mount(TerminalContextMenu, {
      attachTo: document.body,
      props: {
        getTerminal: () => terminal as unknown as Terminal,
        canPaste: true,
      },
    })
  })

  afterEach(() => {
    wrapper.unmount()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  async function openMenu(): Promise<HTMLElement> {
    const event = new MouseEvent('contextmenu', { button: 2, shiftKey: true, clientX: 40, clientY: 50, cancelable: true })
    ;(wrapper.vm as unknown as ExposedMenu).open(event)
    await wrapper.vm.$nextTick()
    return document.body.querySelector<HTMLElement>('.terminal-context-menu')!
  }

  it('copies mouse selections once the selection is complete, but not touch selections', async () => {
    const menu = wrapper.vm as unknown as ExposedMenu
    menu.handleSelectionPointerDown({ button: 0, pointerType: 'mouse' } as PointerEvent)
    document.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith('selected output')

    writeText.mockClear()
    menu.handleSelectionPointerDown({ button: 0, pointerType: 'touch' } as PointerEvent)
    document.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    expect(writeText).not.toHaveBeenCalled()
  })

  it('pastes directly on a desktop right click', async () => {
    const menu = wrapper.vm as unknown as ExposedMenu
    menu.open(new MouseEvent('contextmenu', { button: 2, cancelable: true }))
    await flushPromises()

    expect(terminal.paste).toHaveBeenCalledWith('pasted command')
    expect(document.body.querySelector('.terminal-context-menu')).toBeNull()
  })

  it('opens the menu when direct clipboard access is blocked', async () => {
    readText.mockRejectedValueOnce(new Error('blocked'))
    const menu = wrapper.vm as unknown as ExposedMenu
    menu.open(new MouseEvent('contextmenu', { button: 2, cancelable: true }))
    await flushPromises()

    expect(terminal.paste).not.toHaveBeenCalled()
    expect(document.body.querySelector('.terminal-context-menu [role="status"]')?.textContent).toContain('Ctrl+V')
  })

  it('keeps the context menu for coarse pointers', async () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })))
    const menu = wrapper.vm as unknown as ExposedMenu
    menu.open(new MouseEvent('contextmenu', { button: 2, cancelable: true }))
    await wrapper.vm.$nextTick()

    expect(readText).not.toHaveBeenCalled()
    expect(document.body.querySelector('.terminal-context-menu')).not.toBeNull()
  })

  it('copies the current terminal selection', async () => {
    const menu = await openMenu()
    const copy = menu.querySelectorAll<HTMLButtonElement>('button')[0]!
    copy.click()
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith('selected output')
    expect(terminal.focus).toHaveBeenCalled()
  })

  it('automatically copies a completed touch selection without focusing or reading the clipboard', async () => {
    await (wrapper.vm as unknown as ExposedMenu).handleTouchHold('copy', 40, 50)
    expect(writeText).toHaveBeenCalledExactlyOnceWith('selected output')
    expect(readText).not.toHaveBeenCalled()
    expect(terminal.focus).not.toHaveBeenCalled()
    expect(document.body.querySelector('.terminal-context-menu')).toBeNull()
  })

  it('automatically pastes on a focused touch hold without overwriting the clipboard', async () => {
    await (wrapper.vm as unknown as ExposedMenu).handleTouchHold('paste', 40, 50)
    expect(readText).toHaveBeenCalledOnce()
    expect(terminal.paste).toHaveBeenCalledExactlyOnceWith('pasted command')
    expect(writeText).not.toHaveBeenCalled()
    expect(document.body.querySelector('.terminal-context-menu')).toBeNull()
  })

  it('offers the clipboard menu for an unfocused blank hold', async () => {
    await (wrapper.vm as unknown as ExposedMenu).handleTouchHold('menu', 40, 50)
    expect(document.body.querySelector('.terminal-context-menu')).not.toBeNull()
    expect(readText).not.toHaveBeenCalled()
    expect(writeText).not.toHaveBeenCalled()
  })

  it('keeps the menu and fallback instructions when automatic paste is denied', async () => {
    readText.mockRejectedValueOnce(new Error('denied'))
    await (wrapper.vm as unknown as ExposedMenu).handleTouchHold('paste', 40, 50)
    expect(terminal.paste).not.toHaveBeenCalled()
    expect(document.body.querySelector('[role="status"]')?.textContent).toContain('Ctrl+V')
  })

  it('keeps manual copy available when automatic copy and the legacy fallback fail', async () => {
    writeText.mockRejectedValueOnce(new Error('denied'))
    Object.defineProperty(document, 'execCommand', { configurable: true, value: vi.fn(() => false) })
    await (wrapper.vm as unknown as ExposedMenu).handleTouchHold('copy', 40, 50)
    expect(document.body.querySelector('.terminal-context-menu')).not.toBeNull()
    expect(document.body.querySelector('[role="status"]')).not.toBeNull()
    expect(terminal.focus).not.toHaveBeenCalled()
  })

  it('does not send input for an empty clipboard or read-only terminal', async () => {
    readText.mockResolvedValueOnce('')
    const menu = wrapper.vm as unknown as ExposedMenu
    await menu.handleTouchHold('paste', 40, 50)
    expect(terminal.paste).not.toHaveBeenCalled()
    readText.mockClear()
    await wrapper.setProps({ canPaste: false })
    await menu.handleTouchHold('paste', 40, 50)
    expect(readText).not.toHaveBeenCalled()
    expect(document.body.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')[1]?.disabled).toBe(true)
  })

  it.each(['closed', 'replaced', 'unmounted'])('does not paste a permission result after the terminal is %s', async kind => {
    let resolve!: (text: string) => void
    readText.mockImplementationOnce(() => new Promise<string>(done => { resolve = done }))
    const pending = (wrapper.vm as unknown as ExposedMenu).handleTouchHold('paste', 40, 50)
    if (kind === 'closed') await wrapper.setProps({ canPaste: false })
    if (kind === 'replaced') await wrapper.setProps({ getTerminal: () => undefined })
    if (kind === 'unmounted') wrapper.unmount()
    resolve('delayed command')
    await pending
    expect(terminal.paste).not.toHaveBeenCalled()
    expect(terminal.focus).not.toHaveBeenCalled()
  })

  it('pastes clipboard text through xterm', async () => {
    const menu = await openMenu()
    const paste = menu.querySelectorAll<HTMLButtonElement>('button')[1]!
    paste.click()
    await flushPromises()

    expect(readText).toHaveBeenCalled()
    expect(terminal.paste).toHaveBeenCalledWith('pasted command')
  })

  it('captures native paste data without sending a duplicate browser event', () => {
    const preventDefault = vi.fn()
    const stopPropagation = vi.fn()
    ;(wrapper.vm as unknown as ExposedMenu).handlePaste({
      clipboardData: { getData: () => 'native paste' },
      preventDefault,
      stopPropagation,
    } as unknown as ClipboardEvent)

    expect(terminal.paste).toHaveBeenCalledWith('native paste')
    expect(preventDefault).toHaveBeenCalled()
    expect(stopPropagation).toHaveBeenCalled()
  })

  it('handles terminal copy shortcuts but leaves normal Ctrl+C available for SIGINT', async () => {
    const menu = wrapper.vm as unknown as ExposedMenu
    const copyEvent = new KeyboardEvent('keydown', {
      key: 'c',
      ctrlKey: true,
      shiftKey: true,
      cancelable: true,
    })
    expect(menu.handleKeyEvent(new KeyboardEvent('keydown', { key: 'c', ctrlKey: true }))).toBe(true)
    expect(menu.handleKeyEvent(copyEvent)).toBe(false)
    expect(menu.handleKeyEvent(new KeyboardEvent('keyup', { key: 'c', ctrlKey: true, shiftKey: true }))).toBe(true)
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith('selected output')
    expect(writeText).toHaveBeenCalledTimes(1)
    expect(copyEvent.defaultPrevented).toBe(true)
  })

  it.each([
    { name: 'Ctrl+Shift+V', init: { key: 'v', ctrlKey: true, shiftKey: true } },
    { name: 'Shift+Insert', init: { key: 'Insert', shiftKey: true } },
  ])('pastes with $name while bypassing xterm for native Ctrl+V', async ({ init }) => {
    const menu = wrapper.vm as unknown as ExposedMenu
    const pasteEvent = new KeyboardEvent('keydown', { ...init, cancelable: true })
    const nativePasteEvent = new KeyboardEvent('keydown', {
      key: 'v',
      ctrlKey: true,
      cancelable: true,
    })

    expect(menu.handleKeyEvent(nativePasteEvent)).toBe(false)
    expect(nativePasteEvent.defaultPrevented).toBe(false)
    expect(nativePasteEvent.cancelBubble).toBe(false)
    expect(readText).not.toHaveBeenCalled()
    expect(terminal.paste).not.toHaveBeenCalled()
    expect(menu.handleKeyEvent(new KeyboardEvent('keyup', { key: 'v', ctrlKey: true }))).toBe(true)
    expect(menu.handleKeyEvent(new KeyboardEvent('keydown', { key: 'v', ctrlKey: true, altKey: true }))).toBe(true)
    expect(menu.handleKeyEvent(new KeyboardEvent('keydown', { key: 'v', ctrlKey: true, metaKey: true }))).toBe(true)
    expect(menu.handleKeyEvent(pasteEvent)).toBe(false)
    await flushPromises()

    expect(readText).toHaveBeenCalledTimes(1)
    expect(terminal.paste).toHaveBeenCalledWith('pasted command')
    expect(pasteEvent.defaultPrevented).toBe(true)
  })

  it('does not intercept terminal paste shortcuts after input has closed', () => {
    wrapper.unmount()
    wrapper = mount(TerminalContextMenu, {
      attachTo: document.body,
      props: {
        getTerminal: () => terminal as unknown as Terminal,
        canPaste: false,
      },
    })

    const menu = wrapper.vm as unknown as ExposedMenu
    const pasteEvent = new KeyboardEvent('keydown', {
      key: 'v',
      ctrlKey: true,
      shiftKey: true,
      cancelable: true,
    })

    expect(menu.handleKeyEvent(pasteEvent)).toBe(true)
    expect(menu.handleKeyEvent(new KeyboardEvent('keydown', { key: 'v', ctrlKey: true }))).toBe(true)
    expect(readText).not.toHaveBeenCalled()
    expect(pasteEvent.defaultPrevented).toBe(false)
  })

  it('closes on Escape without also closing a parent modal', async () => {
    await openMenu()
    const parentEscape = vi.fn()
    window.addEventListener('keydown', parentEscape)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
    await wrapper.vm.$nextTick()

    expect(parentEscape).not.toHaveBeenCalled()
    expect(document.body.querySelector('.terminal-context-menu')).toBeNull()
    window.removeEventListener('keydown', parentEscape)
  })

  it('selects the full terminal buffer from the menu', async () => {
    const menu = await openMenu()
    menu.querySelectorAll<HTMLButtonElement>('button')[2]!.click()

    expect(terminal.selectAll).toHaveBeenCalled()
  })

  it('keeps pointer-open focus neutral until keyboard navigation begins', async () => {
    const menu = await openMenu()
    const items = [...menu.querySelectorAll<HTMLButtonElement>('[role="menuitem"]:not(:disabled)')]

    expect(document.activeElement).toBe(items[0])
    expect(menu.dataset.contextMenuFocus).toBe('pointer')
    expect(items[0]?.hasAttribute('aria-selected')).toBe(false)

    menu.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }))
    expect(menu.dataset.contextMenuFocus).toBe('keyboard')
    expect(document.activeElement).toBe(items[1])
  })

  it('keeps the teleported menu inside its desktop window and above the taskbar', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1280 })
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 800 })
    const desktop = document.createElement('div')
    desktop.className = 'desktop'
    const windowBody = document.createElement('div')
    windowBody.className = 'desktop-window__body'
    const anchor = document.createElement('div')
    windowBody.append(anchor)
    const taskbar = document.createElement('div')
    taskbar.className = 'desktop__taskbar'
    desktop.append(windowBody, taskbar)
    document.body.append(desktop)
    const originalBounds = HTMLElement.prototype.getBoundingClientRect
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      if (this === desktop) return rect(0, 0, 1280, 800)
      if (this === windowBody) return rect(100, 80, 1080, 660)
      if (this === taskbar) return rect(8, 728, 1264, 56)
      if (this.matches('.terminal-context-menu')) return rect(0, 0, 228, 140)
      return originalBounds.call(this)
    })

    ;(wrapper.vm as unknown as ExposedMenu).open({
      clientX: 1200,
      clientY: 760,
      currentTarget: anchor,
      preventDefault: vi.fn(),
      stopPropagation: vi.fn(),
    } as unknown as MouseEvent)
    await flushPromises()

    const menu = document.body.querySelector<HTMLElement>('.terminal-context-menu')!
    expect(menu.style.left).toBe('944px')
    expect(menu.style.top).toBe('580px')
    expect(Number.parseFloat(menu.style.top) + 140).toBeLessThanOrEqual(720)
    expect(menu.style.getPropertyValue('--context-menu-max-height')).toBe('632px')
    desktop.remove()
  })
})
