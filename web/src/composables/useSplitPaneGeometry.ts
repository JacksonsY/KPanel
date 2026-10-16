import { computed, nextTick, onBeforeUnmount, ref, type CSSProperties, type Ref } from 'vue'
import type { TerminalLayout } from './useTerminalWorkspace'

type Axis = 'column' | 'row'

interface SplitPaneGeometryOptions {
  layout: Ref<TerminalLayout>
  singleVisible: Ref<boolean>
  activePane: Ref<number>
  paneCount: Ref<number>
  /** Smallest comfortable pane size in CSS px; ratios never squeeze a pane below it. */
  minimumWidth?: number
  minimumHeight?: number
  /** Runs after a divider settles, for content that must refit itself. */
  onResize?: () => void
}

// Shared by the terminal and file editor split views: a 2x2 grid whose 8px
// dividers follow pointer drags and arrow keys. Panes own no content here.
export function useSplitPaneGeometry(options: SplitPaneGeometryOptions) {
  const { layout, singleVisible, activePane, paneCount } = options
  const minimumWidth = options.minimumWidth ?? 280
  const minimumHeight = options.minimumHeight ?? 150
  const grid = ref<HTMLElement>()
  const columnRatio = ref(50)
  const rowRatio = ref(50)
  const gridSize = ref({ width: 0, height: 0 })
  const minimumColumnRatio = computed(() => gridSize.value.width > 8 ? Math.min(50, Math.max(25, minimumWidth / (gridSize.value.width - 8) * 100)) : 25)
  const minimumRowRatio = computed(() => gridSize.value.height > 8 ? Math.min(50, Math.max(25, minimumHeight / (gridSize.value.height - 8) * 100)) : 25)
  const hasColumns = computed(() => !singleVisible.value && (layout.value === 'columns' || layout.value === 'grid'))
  const hasRows = computed(() => !singleVisible.value && (layout.value === 'rows' || layout.value === 'grid'))
  const gridStyle = computed<CSSProperties>(() => ({
    gridTemplateColumns: hasColumns.value ? `minmax(0, ${columnRatio.value}fr) 8px minmax(0, ${100 - columnRatio.value}fr)` : 'minmax(0, 1fr)',
    gridTemplateRows: hasRows.value ? `minmax(0, ${rowRatio.value}fr) 8px minmax(0, ${100 - rowRatio.value}fr)` : 'minmax(0, 1fr)',
  }))
  function isVisible(index: number): boolean {
    return index >= 0 && index < paneCount.value && (!singleVisible.value || index === activePane.value)
  }
  function cellStyle(index: number): CSSProperties {
    if (singleVisible.value) return { gridColumn: '1', gridRow: '1' }
    return {
      gridColumn: hasColumns.value && index % 2 === 1 ? '3' : '1',
      gridRow: hasRows.value && (layout.value === 'rows' ? index === 1 : index >= 2) ? '3' : '1',
    }
  }
  function refresh(): void {
    if (options.onResize) void nextTick(options.onResize)
  }
  function minimumRatio(axis: Axis): number { return axis === 'column' ? minimumColumnRatio.value : minimumRowRatio.value }
  function clampRatio(axis: Axis, ratio: number): number {
    const minimum = minimumRatio(axis)
    return Math.max(minimum, Math.min(100 - minimum, ratio))
  }
  /** Reads the grid size so ratios keep every visible pane above its minimum. */
  function measure(): void {
    if (singleVisible.value || !grid.value) return
    gridSize.value = { width: grid.value.clientWidth, height: grid.value.clientHeight }
    columnRatio.value = clampRatio('column', columnRatio.value)
    rowRatio.value = clampRatio('row', rowRatio.value)
  }
  let drag: { axis: Axis; pointerId: number; element: HTMLElement } | undefined
  function endDrag(): void {
    const previous = drag
    drag = undefined
    if (previous?.element.hasPointerCapture(previous.pointerId)) previous.element.releasePointerCapture(previous.pointerId)
    refresh()
  }
  function startDrag(axis: Axis, event: PointerEvent): void {
    if (event.button !== 0) return
    endDrag()
    const element = event.currentTarget as HTMLElement
    drag = { axis, pointerId: event.pointerId, element }
    element.setPointerCapture(event.pointerId)
  }
  function moveDrag(event: PointerEvent): void {
    if (!drag || drag.pointerId !== event.pointerId || !grid.value) return
    const rect = grid.value.getBoundingClientRect()
    const size = drag.axis === 'column' ? rect.width : rect.height
    if (size <= 8) return
    const position = drag.axis === 'column' ? event.clientX - rect.left : event.clientY - rect.top
    const ratio = clampRatio(drag.axis, (position - 4) / (size - 8) * 100)
    if (drag.axis === 'column') columnRatio.value = ratio
    else rowRatio.value = ratio
  }
  function resizeWithKeyboard(axis: Axis, event: KeyboardEvent): void {
    const value = axis === 'column' ? columnRatio : rowRatio
    const decrease = axis === 'column' ? 'ArrowLeft' : 'ArrowUp'
    const increase = axis === 'column' ? 'ArrowRight' : 'ArrowDown'
    if (![decrease, increase, 'Home', 'End', 'Enter'].includes(event.key)) return
    event.preventDefault()
    value.value = clampRatio(axis, event.key === 'Home' ? 25 : event.key === 'End' ? 75 : event.key === 'Enter' ? 50
      : value.value + (event.key === increase ? 5 : -5))
    refresh()
  }
  function resetRatio(axis: Axis): void {
    if (axis === 'column') columnRatio.value = 50
    else rowRatio.value = 50
    refresh()
  }
  onBeforeUnmount(endDrag)
  return {
    grid, columnRatio, rowRatio, minimumColumnRatio, minimumRowRatio, hasColumns, hasRows, gridStyle,
    isVisible, cellStyle, measure, startDrag, moveDrag, endDrag, resizeWithKeyboard, resetRatio,
  }
}
