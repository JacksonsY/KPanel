import { execFileSync } from 'node:child_process'
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { canExecuteFile, fileExecutionCommand } from './fileExecution'
import type { FileEntry } from '@/types/api'

const entry = (path: string, mode = '-rw-r--r--'): FileEntry => ({
  path, name: path.split('/').at(-1)!, mode, kind: 'file', sizeBytes: 12,
  owner: 'root', group: 'root', resourceVersion: 'v1', modifiedAt: '', editable: true, previewable: true,
})
const messages = { unavailable: 'missing file', permission: 'missing execute permission', interpreter: (name: string) => `missing ${name}` }
const temporaryDirectories: string[] = []
afterEach(() => { for (const directory of temporaryDirectories.splice(0)) rmSync(directory, { recursive: true, force: true }) })

describe('file execution', () => {
  it.each([['sh', 'bash'], ['bash', 'bash'], ['py', 'python3'], ['js', 'node'], ['mjs', 'node'], ['cjs', 'node'], ['php', 'php']])('runs .%s using %s', (extension, interpreter) => {
    const file = entry(`/home/test.${extension}`)
    expect(canExecuteFile(file)).toBe(true)
    expect(fileExecutionCommand(file, messages)).toContain(`command ${interpreter} './test.${extension}'`)
  })

  it('recognizes executable Go binaries without a suffix and permission checks .bin/.run', () => {
    expect(canExecuteFile(entry('/home/server', '-rwxr-xr-x'))).toBe(true)
    expect(fileExecutionCommand(entry('/home/server', '-rwxr-xr-x'), messages)).toContain("[ -x './server' ]")
    expect(canExecuteFile(entry('/home/setup.run'))).toBe(true)
    expect(canExecuteFile(entry('/home/tool.bin'))).toBe(true)
    expect(canExecuteFile(entry('/home/server'))).toBe(false)
    expect(canExecuteFile(entry('/home/source.go'))).toBe(false)
    expect(canExecuteFile(entry('/home/document.pdf'))).toBe(false)
  })

  it.each(['/home/a\ncommand.sh', '/home/a\rcommand.sh', '/home/a\tcommand.sh', '/home/a\x00.sh', '/home/a\x7f.sh', 'relative.sh', '/home/../a.sh', '/home//a.sh', '/home/'])('rejects unsafe terminal paths: %j', (path) => {
    expect(fileExecutionCommand(entry(path), messages)).toBeUndefined()
  })

  it('never accepts a directory, symlink, special file, or inherited interpreter property', () => {
    for (const kind of ['directory', 'symlink', 'special'] as const) expect(canExecuteFile({ ...entry('/home/a.sh'), kind })).toBe(false)
    expect(canExecuteFile(entry('/home/a.constructor'))).toBe(false)
  })

  it('executes a quoted filename literally in the requested directory without shell expansion', () => {
    const directory = mkdtempSync(join(tmpdir(), 'kpanel-execution-'))
    temporaryDirectories.push(directory)
    const name = "test '$(touch PWNED)`touch PWNED`.sh"
    writeFileSync(join(directory, name), "printf '%s' \"$PWD\" > result.txt\nprintf 'script ran\\n'\n")
    const linuxDirectory = directory.replaceAll('\\', '/').replace(/^([A-Za-z]):/, (_, drive: string) => `/${drive.toLowerCase()}`)
    const command = fileExecutionCommand(entry(`${linuxDirectory}/${name}`), messages)!
    const bash = process.platform === 'win32' ? 'C:/Program Files/Git/bin/bash.exe' : '/bin/bash'
    expect(execFileSync(bash, ['--noprofile', '--norc', '-c', command], { encoding: 'utf8', cwd: directory })).toContain('script ran')
    expect(readFileSync(join(directory, 'result.txt'), 'utf8')).toContain('kpanel-execution-')
    expect(existsSync(join(directory, 'PWNED'))).toBe(false)
    rmSync(join(directory, name))
    const result = execFileSync(bash, ['--noprofile', '--norc', '-c', command], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
    expect(result).toBe('')
  })
})
