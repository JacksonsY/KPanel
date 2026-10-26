import { Terminal } from '@xterm/xterm'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TerminalWebLinksAddon, terminalSelectionText, terminalWrappedLink } from './terminalWrappedLinks'

const terminals: Terminal[] = []
const write = (terminal: Terminal, data: string) => new Promise<void>(resolve => terminal.write(data, resolve))
const url = 'https://example.test/oauth/authorize?response_type=code&client_id=test&scope=openid%20profile&code_challenge=' + 'a_-'.repeat(110) + '&state=test&originator=cli'

async function tuiLink(text = url, cols = 80, indent = 2) {
  const terminal = new Terminal({ cols, rows: 120 })
  terminals.push(terminal)
  const parts = text.match(new RegExp(`.{1,${cols - indent}}`, 'g'))!
  await write(terminal, parts.map((part, index) => `\x1b[${index + 3};${indent + 1}H\x1b[4m${part}\x1b[0m`).join(''))
  return { terminal, parts }
}

afterEach(() => {
  terminals.splice(0).forEach(terminal => terminal.dispose())
  vi.unstubAllGlobals()
})

describe('cursor-addressed terminal URLs', () => {
  it.each([40, 80, 120])('opens the complete URL from every row at %i columns', async cols => {
    const { terminal, parts } = await tuiLink(url, cols)
    const open = vi.fn()
    vi.stubGlobal('window', { open })
    for (let i = 0; i < parts.length; i++) {
      expect(terminal.buffer.active.getLine(i + 2)?.isWrapped).toBe(false)
      const link = terminalWrappedLink(terminal, i + 3)!
      expect(link.text).toBe(url)
      expect(link.range).toEqual({ start: { x: 3, y: 3 }, end: { x: parts.at(-1)!.length + 2, y: parts.length + 2 } })
      link.activate({} as MouseEvent, link.text)
    }
    expect(open).toHaveBeenCalledWith(url, '_blank', 'noopener,noreferrer')
  })

  it('handles URLs beyond the standard addon 2048-character window', async () => {
    const long = url + '&padding=' + 'a'.repeat(2600)
    const { terminal, parts } = await tuiLink(long)
    expect(terminalWrappedLink(terminal, 3)?.text).toBe(long)
    expect(terminalWrappedLink(terminal, parts.length + 2)?.text).toBe(long)
  })

  it('copies a full selected TUI URL without display newlines, while preserving ordinary multiline selections', async () => {
    const { terminal, parts } = await tuiLink()
    vi.spyOn(terminal, 'getSelection').mockReturnValue(parts.join('\n  '))
    vi.spyOn(terminal, 'getSelectionPosition').mockReturnValue({ start: { x: 2, y: 2 }, end: { x: parts.at(-1)!.length + 2, y: parts.length + 1 } })
    expect(terminal.getSelection()).toContain('\n')
    expect(terminalSelectionText(terminal)).toBe(url)
    vi.mocked(terminal.getSelection).mockReturnValue('ordinary\noutput')
    expect(terminalSelectionText(terminal)).toBe(terminal.getSelection())
  })

  it('leaves ordinary links, prose, blank separators and separate URLs to the standard addon', async () => {
    const { terminal } = await tuiLink('https://example.test/short')
    await write(terminal, '\x1b[4;3H\x1b[4mContinue\x1b[0m')
    expect(terminalWrappedLink(terminal, 3)).toBeUndefined()
    expect(terminalWrappedLink(terminal, 4)).toBeUndefined()
    await write(terminal, '\x1b[2J\x1b[3;3H\x1b[4mhttps://example.test/' + 'a'.repeat(56) + '\x1b[0m\x1b[4;3H\x1b[4mhttps://other.test/\x1b[0m')
    expect(terminalWrappedLink(terminal, 3)).toBeUndefined()
    await write(terminal, '\x1b[2J\x1b[3;3Hhttps://example.test/' + 'a'.repeat(58) + '\x1b[4;3Hcontinuation')
    expect(terminalWrappedLink(terminal, 3)).toBeUndefined()
  })

  it('stops at the row budget and rejects non-web protocols', async () => {
    const { terminal } = await tuiLink('https://example.test/?padding=' + 'a'.repeat(6000))
    expect(terminalWrappedLink(terminal, 3)).toBeUndefined()
    const unsafe = await tuiLink('javascript:alert(1)' + 'a'.repeat(100))
    expect(terminalWrappedLink(unsafe.terminal, 3)).toBeUndefined()
  })

  it('registers the repair before the standard provider and disposes both', async () => {
    const { terminal } = await tuiLink()
    const register = vi.spyOn(terminal, 'registerLinkProvider')
    const addon = new TerminalWebLinksAddon()
    terminal.loadAddon(addon)
    expect(register).toHaveBeenCalledTimes(2)
    const callback = vi.fn()
    register.mock.calls[0]![0].provideLinks(3, callback)
    expect(callback.mock.calls[0]![0][0].text).toBe(url)
    const original = vi.fn()
    register.mock.calls[1]![0].provideLinks(3, original)
    expect(original.mock.calls[0]![0][0].text).not.toBe(url)
    const disposed = register.mock.results.map(result => vi.spyOn(result.value, 'dispose'))
    addon.dispose()
    disposed.forEach(dispose => expect(dispose).toHaveBeenCalled())
  })
})
