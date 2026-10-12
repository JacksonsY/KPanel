const encoder = new TextEncoder()
export const terminalStartupEchoLimit = 64 << 10

// Only the initial, generated shell line is hidden. Once the shell emits this
// boundary, every byte (including ANSI, CR and split UTF-8) goes straight to xterm.
// The boundary is escaped in the input, so PTY/readline echo cannot match it.
export class TerminalCommandPresentation {
  private buffered = new Uint8Array(0)
  private boundary: Uint8Array
  pending = true
  readonly command: string

  constructor(command: string, token = Array.from(crypto.getRandomValues(new Uint8Array(16)), byte => byte.toString(16).padStart(2, '0')).join('')) {
    if (!/^[a-zA-Z0-9-]+$/.test(token)) throw new Error('Invalid terminal boundary')
    this.boundary = encoder.encode(`\x1b]kpanel-start;${token}\x07`)
    this.command = `command printf '\\033]kpanel-start;${token}\\007'; ${command}`
  }

  push(data: Uint8Array): Uint8Array {
    if (!this.pending) return data
    const joined = new Uint8Array(this.buffered.length + data.length)
    joined.set(this.buffered)
    joined.set(data, this.buffered.length)
    this.buffered = joined
    // Only rescan the previous partial suffix and this chunk.
    const start = Math.max(0, joined.length - data.length - this.boundary.length + 1)
    for (let i = start; i <= joined.length - this.boundary.length; i++) {
      if (this.boundary.every((byte, index) => joined[i + index] === byte)) {
        this.pending = false
        this.buffered = new Uint8Array(0)
        return joined.slice(i + this.boundary.length)
      }
    }
    // Unsupported shell / missing boundary: never swallow output indefinitely.
    return joined.length >= terminalStartupEchoLimit ? this.release() : new Uint8Array(0)
  }

  release(): Uint8Array {
    this.pending = false
    const data = this.buffered
    this.buffered = new Uint8Array(0)
    return data
  }
}
