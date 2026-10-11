import type { IDisposable, Terminal } from '@xterm/xterm'

// BusyBox read_key can mistake these two CPRs for modified arrow-key prefixes
// and consume the command that follows. A leading zero keeps the same cursor
// position while avoiding that ambiguity; every other report stays with xterm.
export function installTerminalCursorReport(terminal: Terminal): IDisposable {
  return terminal.parser.registerCsiHandler({ final: 'n' }, (params) => {
    if (params.length !== 1 || params[0] !== 6) return false
    const { cursorX, cursorY } = terminal.buffer.active
    if (cursorY !== 0 || (cursorX !== 2 && cursorX !== 4)) return false
    terminal.input(`\x1b[01;${cursorX + 1}R`, false)
    return true
  })
}
