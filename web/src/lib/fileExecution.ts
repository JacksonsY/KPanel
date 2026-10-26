import type { FileEntry } from '@/types/api'

const interpreters: Record<string, string> = {
  sh: 'bash', bash: 'bash', py: 'python3', js: 'node', mjs: 'node', cjs: 'node', php: 'php',
}

function fileParts(entry: FileEntry): { directory: string; file: string; extension: string } | undefined {
  // Terminal input normalizes line endings. Never allow a path to become a second command.
  if (entry.kind !== 'file' || !entry.path.startsWith('/') || entry.path.length > 4096
    || /[\x00-\x1f\x7f-\x9f]/.test(entry.path)) return undefined
  const parts = entry.path.split('/').slice(1)
  if (parts.some((part) => !part || part === '.' || part === '..')) return undefined
  const name = parts.pop()!
  return { directory: `/${parts.join('/')}`, file: `./${name}`, extension: name.split('.').slice(1).at(-1)?.toLowerCase() || '' }
}

export function canExecuteFile(entry: FileEntry, shell: 'posix' | 'powershell' = 'posix'): boolean {
  if (shell === 'powershell') return Boolean(fileParts(entry) && /^\/[A-Za-z]\//.test(entry.path) && /\.(ps1|exe|py|js|mjs|cjs|php)$/i.test(entry.path))
  const parts = fileParts(entry)
  return Boolean(parts && (dockerFileKind(parts.file) || Object.hasOwn(interpreters, parts.extension)
    || ['run', 'bin'].includes(parts.extension) || /x/.test(entry.mode.slice(-9))))
}

function dockerFileKind(file: string): 'build' | 'compose' | undefined {
  const name = file.slice(2).toLowerCase()
  if (/^dockerfile(?:\.[a-z0-9][a-z0-9._-]*)?$/.test(name)) return 'build'
  if (/^(?:docker-)?compose\.ya?ml$/.test(name)) return 'compose'
}

function quote(value: string): string {
  return `'${value.replaceAll("'", "'\\''")}'`
}

export function fileExecutionDisplayCommand(entry: FileEntry, shell: 'posix' | 'powershell' = 'posix'): string | undefined {
  if (shell === 'powershell') return windowsFileCommand(entry)
  const parts = fileParts(entry)
  if (!parts || !canExecuteFile(entry)) return undefined
  const displayQuote = (value: string) => /^[a-zA-Z0-9_./-]+$/.test(value) ? value : quote(value)
  const file = displayQuote(parts.file)
  const kind = dockerFileKind(parts.file)
  const interpreter = Object.hasOwn(interpreters, parts.extension) ? interpreters[parts.extension] : undefined
  const execute = kind === 'build' ? `docker build --file ${file} .`
    : kind === 'compose' ? `docker compose --file ${file} up -d`
    : interpreter ? `${interpreter} ${file}` : file
  return `cd -- ${displayQuote(parts.directory)} && ${execute}`
}

export function fileExecutionCommand(entry: FileEntry, messages: {
  unavailable: string; permission: string; interpreter: (name: string) => string; tool: (name: string) => string
}, shell: 'posix' | 'powershell' = 'posix'): string | undefined {
  if (shell === 'powershell') return windowsFileCommand(entry)
  const parts = fileParts(entry)
  if (!parts || !canExecuteFile(entry)) return undefined
  const file = quote(parts.file)
  const interpreter = Object.hasOwn(interpreters, parts.extension) ? interpreters[parts.extension] : undefined
  const print = (message: string) => `printf '%s\\n' ${quote(message)} >&2`
  let execute = interpreter
    ? `if command -v ${interpreter} >/dev/null 2>&1; then command ${interpreter} ${file}; else ${print(messages.interpreter(interpreter))}; fi`
    : `if [ -x ${file} ]; then ${file}; else ${print(messages.permission)}; fi`
  const dockerKind = dockerFileKind(parts.file)
  if (dockerKind === 'build') {
    execute = `if command -v docker >/dev/null 2>&1; then command docker build --file ${file} .; else ${print(messages.tool('Docker'))}; fi`
  } else if (dockerKind === 'compose') {
    execute = `if command docker compose version >/dev/null 2>&1; then command docker compose --file ${file} up -d; elif command -v docker-compose >/dev/null 2>&1; then command docker-compose --file ${file} up -d; else ${print(messages.tool('Docker Compose'))}; fi`
  }
  // Only generated syntax and fixed interpreter names enter the shell; all paths/messages are quoted.
  return `cd -- ${quote(parts.directory)} && if [ -f ${file} ] && [ ! -L ${file} ]; then ${execute}; else ${print(messages.unavailable)}; fi`
}

function windowsFileCommand(entry: FileEntry): string | undefined {
  if (!canExecuteFile(entry, 'powershell')) return undefined
  const parts = fileParts(entry)!
  const native = entry.path.slice(1, 2) + ':' + entry.path.slice(2).replaceAll('/', '\\')
  const directory = native.slice(0, native.lastIndexOf('\\') + 1)
  // Unicode curly quotes are PowerShell delimiters too. Data is decoded only
  // after parsing, so neither filenames nor display text can become syntax.
  const value = (text: string) => {
    let binary = ''; for (const byte of new TextEncoder().encode(text)) binary += String.fromCharCode(byte)
    return "([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + btoa(binary) + "')))"
  }
  const interpreter = ({py: 'python', js: 'node', mjs: 'node', cjs: 'node', php: 'php'} as Record<string,string>)[parts.extension]
  return 'Set-Location -LiteralPath ' + value(directory) + '; if ($?) { & ' + (interpreter ? interpreter + ' ' : '') + value(native) + ' }'
}
