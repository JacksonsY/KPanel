/** Touch ranges share the page's plain-click/Shift semantics; vertical drags remain native scrolling. */
export function createGalleryTouchSelection(options: {
  root: () => HTMLElement | undefined
  selecting: () => boolean
  begin: (path: string) => void
  update: (path: string) => void
}) {
  let press: { id: number; path: string; x: number; y: number; startX: number; startY: number } | undefined
  let active = false
  let suppressClick = false
  let suppressUntil = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let frame: number | undefined
  let lastPath = ''
  let lastFrame = 0
  const pathAt = (element: Element | null) => {
    const tile = element?.closest<HTMLElement>('[data-gallery-path]')
    return tile && options.root()?.contains(tile) ? tile.dataset.galleryPath : undefined
  }
  function cancel(): void {
    if (active) suppressUntil = performance.now() + 700
    clearTimeout(timer)
    if (frame !== undefined) cancelAnimationFrame(frame)
    timer = undefined
    frame = undefined
    press = undefined
    active = false
    lastPath = ''
    lastFrame = 0
  }
  function update(): void {
    if (!press) return
    const path = pathAt(document.elementFromPoint(press.x, press.y))
    if (path && path !== lastPath) {
      lastPath = path
      options.update(path)
    }
  }
  function scroll(timestamp: number): void {
    if (!active || !press) return
    const root = options.root()
    const nested = root?.querySelector<HTMLElement>('.gallery-scroll')
    const container = nested && nested.scrollHeight > nested.clientHeight ? nested : document.scrollingElement
    const bounds = container === nested ? nested!.getBoundingClientRect() : { top: 0, bottom: innerHeight }
    const top = Math.max(0, bounds.top)
    const bottom = Math.min(innerHeight, bounds.bottom)
    const direction = press.y < top + 48 ? -1 : press.y > bottom - 48 ? 1 : 0
    const elapsed = lastFrame ? Math.min(32, timestamp - lastFrame) : 16
    lastFrame = timestamp
    if (container && direction) {
      container.scrollTop += direction * elapsed * 0.48
      update()
    }
    frame = requestAnimationFrame(scroll)
  }
  function activate(): void {
    if (!press || active) return
    clearTimeout(timer)
    active = true
    suppressClick = true
    suppressUntil = Infinity
    lastPath = press.path
    options.begin(press.path)
    update()
    frame = requestAnimationFrame(scroll)
  }
  function start(event: TouchEvent): void {
    cancel()
    suppressClick = false
    if (!options.selecting() || event.touches.length !== 1) return
    if ((event.target as Element | null)?.closest('button.gallery-tile__check')) return
    const path = pathAt(event.target instanceof Element ? event.target : null)
    const touch = event.touches[0]
    if (!path || !touch) return
    press = { id: touch.identifier, path, x: touch.clientX, y: touch.clientY, startX: touch.clientX, startY: touch.clientY }
    timer = setTimeout(activate, 400)
  }
  function move(event: TouchEvent): void {
    if (!press) return
    if (event.touches.length !== 1) { cancel(); return }
    const touch = Array.from(event.touches).find((value) => value.identifier === press!.id)
    if (!touch) { cancel(); return }
    press.x = touch.clientX
    press.y = touch.clientY
    const x = Math.abs(press.x - press.startX)
    const y = Math.abs(press.y - press.startY)
    if (!active && Math.max(x, y) > 10) {
      if (options.selecting() && x > y * 1.5) activate()
      else { cancel(); return }
    }
    if (active) {
      // A non-passive move only consumes a deliberately activated selection gesture.
      if (event.cancelable) event.preventDefault()
      update()
    }
  }
  function click(event: MouseEvent): void {
    if (suppressClick && performance.now() < suppressUntil && event.detail !== 0 && pathAt(event.target instanceof Element ? event.target : null)) {
      event.preventDefault()
      event.stopPropagation()
    }
  }
  function contextMenu(event: Event): void {
    if (press) event.preventDefault()
  }
  return { start, move, end: cancel, cancel, click, contextMenu }
}
