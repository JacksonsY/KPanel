import { computed, ref, watch, type Ref } from 'vue'

export type TerminalLayout = 'single' | 'columns' | 'rows' | 'grid'

// Layout owns references to sessions, never their connection lifecycle.
export function useTerminalWorkspace(sessionIds: Ref<string[]>, selectedId: Ref<string>) {
  const layout = ref<TerminalLayout>('single')
  const panes = ref<(string | null)[]>([null, null, null, null])
  const activePane = ref(0)
  const maximized = ref(false)
  const paneCount = computed(() => layout.value === 'single' ? 1 : layout.value === 'grid' ? 4 : 2)

  function fillEmptyPanes(): void {
    for (let index = 0; index < paneCount.value; index++) {
      if (!panes.value[index]) {
        panes.value[index] = sessionIds.value.find((id) => !panes.value.includes(id)) ?? null
      }
    }
  }

  function selectSession(id: string): void {
    if (!sessionIds.value.includes(id)) return
    const existing = panes.value.indexOf(id)
    if (existing >= 0 && existing < paneCount.value) activePane.value = existing
    else {
      if (existing >= 0) panes.value[existing] = panes.value[activePane.value] ?? null
      panes.value[activePane.value] = id
    }
  }

  function setLayout(next: TerminalLayout): void {
    layout.value = next
    maximized.value = false
    if (activePane.value >= paneCount.value) {
      const previous = panes.value[0] ?? null
      panes.value[0] = panes.value[activePane.value] ?? null
      panes.value[activePane.value] = previous
      activePane.value = 0
    }
    fillEmptyPanes()
  }

  watch(sessionIds, (ids) => {
    panes.value = panes.value.map((id) => id && ids.includes(id) ? id : null)
    if (selectedId.value && ids.includes(selectedId.value)) selectSession(selectedId.value)
    fillEmptyPanes()
  }, { immediate: true })
  watch(selectedId, selectSession)

  return { layout, panes, activePane, maximized, paneCount, selectSession, setLayout }
}
