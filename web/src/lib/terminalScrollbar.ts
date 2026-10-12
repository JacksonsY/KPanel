import type { Terminal } from '@xterm/xterm'

/** Use one native rail for arrows and dragging, synchronized with xterm's buffer. */
export function installTerminalScrollbar(host: HTMLElement, terminal: Terminal): () => void {
  const viewport = host.querySelector<HTMLElement>('.xterm-viewport')
  const screen = host.querySelector<HTMLElement>('.xterm-screen')
  if (!viewport || !screen) return () => {}

  const spacer = document.createElement('div')
  spacer.setAttribute('aria-hidden', 'true')
  spacer.style.width = '1px'
  viewport.appendChild(spacer)
  let lineHeight = 1
  let disposed = false
  const sync = () => {
    if (disposed) return
    const buffer = terminal.buffer.active
    lineHeight = Math.max(1, screen.clientHeight / Math.max(1, terminal.rows))
    spacer.style.height = `${viewport.clientHeight + buffer.baseY * lineHeight}px`
    viewport.scrollTop = buffer.viewportY * lineHeight
  }
  const scroll = () => {
    const buffer = terminal.buffer.active
    const line = Math.min(buffer.baseY, Math.max(0, Math.round(viewport.scrollTop / lineHeight)))
    if (line !== buffer.viewportY) terminal.scrollToLine(line)
  }
  viewport.addEventListener('scroll', scroll)
  const subscriptions = [terminal.onScroll(sync), terminal.onWriteParsed(sync), terminal.onResize(sync)]
  const observer = new ResizeObserver(sync)
  observer.observe(viewport)
  observer.observe(screen)
  sync()
  return () => {
    disposed = true
    viewport.removeEventListener('scroll', scroll)
    subscriptions.forEach(subscription => subscription.dispose())
    observer.disconnect()
    spacer.remove()
  }
}
