import { effectScope, nextTick, ref } from 'vue'
import { describe, expect, it } from 'vitest'
import { useTerminalWorkspace } from './useTerminalWorkspace'

function setup(ids = ['a', 'b', 'c', 'd']) {
  const scope = effectScope()
  const sessions = ref(ids)
  const selected = ref(ids[0] ?? '')
  const workspace = scope.run(() => useTerminalWorkspace(sessions, selected))!
  return { ...workspace, sessions, selected, stop: () => scope.stop() }
}

describe('terminal pane assignment', () => {
  it('keeps the selected session while shrinking and restores the other assignments', () => {
    const w = setup()
    w.setLayout('grid')
    w.selectSession('d')
    w.setLayout('single')
    expect(w.panes.value[0]).toBe('d')
    w.setLayout('columns')
    expect(w.panes.value.slice(0, 2)).toEqual(['a', 'b'])
    w.setLayout('grid')
    expect(new Set(w.panes.value)).toEqual(new Set(['a', 'b', 'c', 'd']))
    w.stop()
  })

  it('appends new sessions after existing panes and keeps tab order when expanding single view', async () => {
    const w = setup(['a'])
    w.sessions.value = ['a', 'b']
    w.selected.value = 'b'
    await nextTick()
    w.setLayout('columns')
    expect(w.panes.value.slice(0, 2)).toEqual(['a', 'b'])
    expect(w.activePane.value).toBe(1)
    w.setLayout('grid')
    w.sessions.value = ['a', 'b', 'c']
    w.selected.value = 'c'
    await nextTick()
    expect(w.panes.value).toEqual(['a', 'b', 'c', null])
    expect(w.activePane.value).toBe(2)
    w.sessions.value = ['a', 'b', 'c', 'd']
    w.selected.value = 'd'
    await nextTick()
    expect(w.panes.value).toEqual(['a', 'b', 'c', 'd'])
    expect(w.activePane.value).toBe(3)
    w.stop()
  })

  it('moves a hidden session into the current pane without duplicating it', () => {
    const w = setup()
    w.setLayout('grid')
    w.setLayout('columns')
    w.selectSession('b')
    w.selectSession('d')
    expect(w.panes.value).toEqual(['a', 'd', 'c', 'b'])
    w.stop()
  })

  it('leaves empty panes until a user opens a session and targets that pane', async () => {
    const w = setup(['a'])
    w.setLayout('grid')
    expect(w.panes.value).toEqual(['a', null, null, null])
    w.activePane.value = 2
    w.selected.value = ''
    await nextTick()
    w.sessions.value.push('new')
    w.selected.value = 'new'
    await nextTick()
    expect(w.panes.value).toEqual(['a', null, 'new', null])
    w.stop()
  })

  it('removes closed references, reuses unassigned sessions and ignores stale selections', async () => {
    const w = setup()
    w.setLayout('columns')
    w.selectSession('c')
    expect(w.panes.value.slice(0, 2)).toEqual(['c', 'b'])
    w.sessions.value = ['a', 'b', 'd']
    w.selected.value = 'b'
    await nextTick()
    expect(w.panes.value.slice(0, 2)).toEqual(['a', 'b'])
    const before = [...w.panes.value]
    w.selectSession('stale')
    expect(w.panes.value).toEqual(before)
    w.stop()
  })
})
