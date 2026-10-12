import { execFileSync } from 'node:child_process'
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { canExecuteFile, fileExecutionCommand, fileExecutionDisplayCommand } from './fileExecution'
import type { FileEntry } from '@/types/api'

const entry = (path: string, mode = '-rw-r--r--'): FileEntry => ({
  path, name: path.split('/').at(-1)!, mode, kind: 'file', sizeBytes: 12,
  owner: 'root', group: 'root', resourceVersion: 'v1', modifiedAt: '', editable: true, previewable: true,
})
const messages = { unavailable: 'missing file', permission: 'missing execute permission', interpreter: (name: string) => `missing ${name}`, tool: (name: string) => `missing ${name}` }
const temporaryDirectories: string[] = []
const shellPath = (path: string) => path.replaceAll('\\', '/').replace(/^([A-Za-z]):/, (_, drive: string) => `/${drive.toLowerCase()}`)
const bash = process.platform === 'win32' ? 'C:/Program Files/Git/bin/bash.exe' : '/bin/bash'
afterEach(() => { for (const directory of temporaryDirectories.splice(0)) rmSync(directory, { recursive: true, force: true }) })

describe('file execution', () => {
  it.each([
    ['/root/demo.sh', 'cd -- /root && bash ./demo.sh'],
    ['/root/demo.py', 'cd -- /root && python3 ./demo.py'],
    ['/apps/Dockerfile', 'cd -- /apps && docker build --file ./Dockerfile .'],
    ['/apps/compose.yaml', 'cd -- /apps && docker compose --file ./compose.yaml up -d'],
    ['/root/server', 'cd -- /root && ./server'],
    ['/my files/a b.sh', "cd -- '/my files' && bash './a b.sh'"],
  ])('shows a concise command for %s', (path, display) => {
    expect(fileExecutionDisplayCommand(entry(path, '-rwxr-xr-x'))).toBe(display)
  })

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

  it.each(['compose.yaml', 'compose.yml', 'docker-compose.yaml', 'docker-compose.yml'])('starts %s through Compose with a legacy fallback', (name) => {
    const file = entry(`/apps/${name}`)
    expect(canExecuteFile(file)).toBe(true)
    const command = fileExecutionCommand(file, messages)!
    expect(command).toContain(`command docker compose --file './${name}' up -d`)
    expect(command).toContain(`command docker-compose --file './${name}' up -d`)
    expect(command).toContain("cd -- '/apps'")
  })

  it.each(['Dockerfile', 'Dockerfile.prod', 'Dockerfile.dev'])('builds %s with its containing directory as the context', (name) => {
    const command = fileExecutionCommand(entry(`/apps/${name}`), messages)!
    expect(command).toContain(`command docker build --file './${name}' .`)
    expect(command).not.toContain(' up ')
    expect(command).not.toContain(' run ')
  })

  it.each(['config.yaml', 'docker-compose.override.yml', 'compose.prod.yaml', '.dockerignore', '.env', 'Dockerfile.bak~'])('does not treat %s as a standalone Docker launch file', (name) => {
    expect(canExecuteFile(entry(`/apps/${name}`))).toBe(false)
  })

  it.each(['build', 'compose', 'legacy', 'missing', 'failed'] as const)('executes Docker command routing with isolated fake tools: %s', (scenario) => {
    const directory = mkdtempSync(join(tmpdir(), 'kpanel-docker-'))
    temporaryDirectories.push(directory)
    const work = join(directory, "project '$(touch PWNED)`touch PWNED`")
    const bin = join(directory, 'bin')
    mkdirSync(work)
    mkdirSync(bin)
    const name = scenario === 'build' ? 'Dockerfile.prod' : 'compose.yaml'
    writeFileSync(join(work, name), '# fixture only')
    const log = join(directory, 'calls.txt')
    const fakeTool = '#!/bin/sh\nprintf "%s\\n" "$PWD" "$@" >> "$MOCK_LOG"\nif [ "$1" = compose ] && [ "$2" = version ]; then exit "$MOCK_COMPOSE_STATUS"; fi\nexit "$MOCK_RUN_STATUS"\n'
    for (const tool of scenario === 'missing' ? [] : ['docker', 'docker-compose']) {
      writeFileSync(join(bin, tool), fakeTool)
      chmodSync(join(bin, tool), 0o755)
    }
    const command = fileExecutionCommand(entry(`${shellPath(work)}/${name}`), messages)!
    const run = () => execFileSync(bash, ['--noprofile', '--norc', '-c', command], {
      cwd: work, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env, PATH: shellPath(bin), MOCK_LOG: shellPath(log), MOCK_COMPOSE_STATUS: scenario === 'legacy' ? '1' : '0', MOCK_RUN_STATUS: scenario === 'failed' ? '7' : '0' },
    })
    if (scenario === 'failed') expect(run).toThrow(expect.objectContaining({ status: 7 }))
    else expect(run()).toBe('')
    const calls = existsSync(log) ? readFileSync(log, 'utf8').trim().split(/\r?\n/) : []
    const cwd = shellPath(work)
    if (scenario === 'build') expect(calls).toEqual([cwd, 'build', '--file', './Dockerfile.prod', '.'])
    else if (scenario === 'missing') expect(calls).toEqual([])
    else expect(calls).toEqual([cwd, 'compose', 'version', cwd, ...(scenario === 'legacy' ? [] : ['compose']), '--file', './compose.yaml', 'up', '-d'])
    expect(existsSync(join(work, 'PWNED'))).toBe(false)
  })

  it.each(['/home/a\ncommand.sh', '/home/a\rcommand.sh', '/home/a\tcommand.sh', '/home/a\x00.sh', '/home/a\x7f.sh', '/home/a\x9b31m.sh', 'relative.sh', '/home/../a.sh', '/home//a.sh', '/home/'])('rejects unsafe terminal paths: %j', (path) => {
    expect(fileExecutionCommand(entry(path), messages)).toBeUndefined()
    expect(fileExecutionDisplayCommand(entry(path))).toBeUndefined()
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
    expect(execFileSync(bash, ['--noprofile', '--norc', '-c', command], { encoding: 'utf8', cwd: directory })).toContain('script ran')
    expect(readFileSync(join(directory, 'result.txt'), 'utf8')).toContain('kpanel-execution-')
    expect(existsSync(join(directory, 'PWNED'))).toBe(false)
    rmSync(join(directory, name))
    const result = execFileSync(bash, ['--noprofile', '--norc', '-c', command], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
    expect(result).toBe('')
  })
})
