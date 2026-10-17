import { readFileSync } from 'node:fs'
import { Terminal } from '@xterm/xterm'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { installTerminalCursorReport } from './terminalCursorReport'

const terminals: Terminal[] = []
const write = (terminal: Terminal, data: string) => new Promise<void>((resolve) => terminal.write(data, resolve))

function setup() {
  const terminal = new Terminal({ cols: 80, rows: 24 })
  terminals.push(terminal)
  const data = vi.fn()
  terminal.onData(data)
  const handler = installTerminalCursorReport(terminal)
  return { terminal, data, handler }
}

afterEach(() => { for (const terminal of terminals.splice(0)) terminal.dispose() })

describe('terminal cursor reports', () => {
  it.each([3, 5])('disambiguates the first-row column %i CPR', async (column) => {
    const { terminal, data } = setup()
    const input = vi.spyOn(terminal, 'input')
    await write(terminal, `\x1b[1;${column}H\x1b[6n`)
    expect(input).toHaveBeenCalledExactlyOnceWith(`\x1b[01;${column}R`, false)
    expect(data).toHaveBeenCalledExactlyOnceWith(`\x1b[01;${column}R`)
  })

  it.each([[1, 1], [1, 4], [1, 6], [2, 3], [2, 5], [24, 80]])(
    'leaves row %i column %i with xterm', async (row, column) => {
      const { terminal, data } = setup()
      const input = vi.spyOn(terminal, 'input')
      await write(terminal, `\x1b[${row};${column}H\x1b[6n`)
      expect(input).not.toHaveBeenCalled()
      expect(data).toHaveBeenCalledExactlyOnceWith(`\x1b[${row};${column}R`)
    },
  )

  it.each(['\x1b[5n', '\x1b[6;1n', '\x1b[6:1n', '\x1b[?6n', '\x1b[>6n', '\x1b[6 n'])(
    'leaves other status queries with xterm: %j', async (query) => {
      const { terminal, data } = setup()
      const reference = new Terminal({ cols: 80, rows: 24 })
      terminals.push(reference)
      const originalData = vi.fn()
      reference.onData(originalData)
      const input = vi.spyOn(terminal, 'input')
      await write(terminal, `\x1b[1;5H${query}`)
      await write(reference, `\x1b[1;5H${query}`)
      expect(input).not.toHaveBeenCalled()
      expect(data.mock.calls).toEqual(originalData.mock.calls)
    },
  )

  it('keeps user-provided text and escape-sequence bytes unchanged', () => {
    const { terminal, data } = setup()
    const typed = 'printf test\r'
    const escapeSequences = '\x1b[1;5R\x1b[6n\u4e2d\u6587'
    terminal.input(typed, true)
    terminal.input(escapeSequences, true)
    expect(data.mock.calls).toEqual([[typed], [escapeSequences]])
  })

  it('restores xterm reports when disposed', async () => {
    const { terminal, data, handler } = setup()
    handler.dispose()
    await write(terminal, '\x1b[1;5H\x1b[6n')
    expect(data).toHaveBeenCalledExactlyOnceWith('\x1b[1;5R')
  })

  it.each([
    ['HostTerminal', new URL('../components/terminal/HostTerminal.vue', import.meta.url)],
    ['AppInteractiveTerminal', new URL('../components/apps/AppInteractiveTerminal.vue', import.meta.url)],
  ])('installs before output and disposes with %s', (_name, path) => {
    expect(readFileSync(path, 'utf8')).toContain('useTerminalController(')
    const source = readFileSync(new URL('../composables/useTerminalController.ts', import.meta.url), 'utf8')
    const mounted = source.slice(source.indexOf('onMounted(() => {'))
    expect(mounted.indexOf('screen.onData(data => queueInput(data))')).toBeGreaterThan(-1)
    expect(mounted.indexOf('screen.onData(data => queueInput(data))')).toBeLessThan(mounted.indexOf('installTerminalCursorReport(screen)'))
    expect(mounted.indexOf('installTerminalCursorReport(screen)')).toBeLessThan(mounted.indexOf('startOutput()'))
    const unmounted = source.slice(source.indexOf('onBeforeUnmount(() => {'))
    expect(unmounted).toContain('cursorReport?.dispose()')
    expect(unmounted.indexOf('cursorReport?.dispose()')).toBeLessThan(unmounted.indexOf('terminal.value?.dispose()'))
  })
})
