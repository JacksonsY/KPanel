import { execFileSync } from 'node:child_process'
import { describe, expect, it } from 'vitest'
import { TerminalCommandPresentation, terminalStartupEchoLimit } from './terminalCommandPresentation'

const bytes = (value: string) => new TextEncoder().encode(value)
const marker = '\x1b]kpanel-start;test-token\x07'

describe('initial terminal command presentation', () => {
  it('does not confuse literal command echo with the emitted boundary', () => {
    const launch = new TerminalCommandPresentation('echo hello', 'test-token')
    expect(launch.push(bytes(`${launch.command}\r\n`))).toHaveLength(0)
    expect(launch.pending).toBe(true)
    expect(launch.push(bytes(`${marker}hello\r\n`))).toEqual(bytes('hello\r\n'))
    expect(launch.pending).toBe(false)
  })

  it('handles a boundary split at every possible byte without changing ANSI or UTF-8', () => {
    const output = bytes('\x1b[32m中文日志\x1b[0m\r进度 50%\r\n')
    const boundary = bytes(marker)
    for (let split = 0; split <= boundary.length; split++) {
      const launch = new TerminalCommandPresentation('echo hello', 'test-token')
      launch.push(bytes('echoed wrapper\r\n'))
      expect(launch.push(boundary.slice(0, split))).toHaveLength(0)
      expect(launch.push(new Uint8Array([...boundary.slice(split), ...output]))).toEqual(output)
      const utf8Part = bytes('中文').slice(0, 2)
      expect(launch.push(utf8Part)).toBe(utf8Part)
    }
  })

  it('releases all buffered bytes when the boundary never arrives or output overflows', () => {
    const launch = new TerminalCommandPresentation('echo hello', 'test-token')
    launch.push(bytes('shell error\r\n'))
    expect(launch.release()).toEqual(bytes('shell error\r\n'))
    expect(launch.push(bytes('next'))).toEqual(bytes('next'))
    const overflow = new TerminalCommandPresentation('echo hello', 'test-token')
    const data = new Uint8Array(terminalStartupEchoLimit).fill(65)
    expect(overflow.push(data)).toEqual(data)
    expect(overflow.pending).toBe(false)
  })

  it('emits the boundary before real shell output and preserves a failing command status', () => {
    const shell = process.platform === 'win32' ? 'C:/Program Files/Git/bin/bash.exe' : '/bin/bash'
    const launch = new TerminalCommandPresentation("printf '\\033[32m中文\\033[0m\\n'; exit 7", 'test-token')
    try {
      execFileSync(shell, ['--noprofile', '--norc', '-c', launch.command], { stdio: ['ignore', 'pipe', 'pipe'] })
      throw new Error('Expected command failure')
    } catch (error) {
      expect(error).toMatchObject({ status: 7 })
      const stdout = (error as { stdout: Buffer }).stdout
      expect(stdout.subarray(0, bytes(marker).length)).toEqual(Buffer.from(bytes(marker)))
      expect(launch.push(stdout)).toEqual(bytes('\x1b[32m中文\x1b[0m\n'))
    }
  })
})
