// @vitest-environment jsdom
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import HostSwitcher from './HostSwitcher.vue'
import ModalDialog from './ModalDialog.vue'
import type { ClusterHost } from '@/types/api'

const hosts = [
  { id: 'local', isLocal: true, name: 'Local' },
  { id: 'blocked', isLocal: false, name: 'Unavailable' },
  { id: 'remote', isLocal: false, name: 'Remote' },
] as ClusterHost[]
const statusOf = (host: ClusterHost) => ({ label: host.name, disabled: host.id === 'blocked' })
const wrappers: ReturnType<typeof mount>[] = []
const body = () => new DOMWrapper(document.body)
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = '' })

describe('shared host switcher in shortcut dialogs', () => {
  it('skips unavailable targets during keyboard navigation and refuses their selection', async () => {
    const w = mount(HostSwitcher, { attachTo: document.body, props: { hosts, activeId: 'local', label: 'Local', statusOf } })
    wrappers.push(w)
    await w.get('button').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    await body().get('input').trigger('keydown', { key: 'ArrowDown' })
    await body().get('[data-host-id="local"]').trigger('keydown', { key: 'ArrowDown' })
    expect(document.activeElement).toBe(body().get('[data-host-id="remote"]').element)
    await body().get('[data-host-id="blocked"]').trigger('click')
    expect(w.emitted('select')).toBeUndefined()
    await w.setProps({ disabled: true })
    expect(body().find('.host-switcher__menu').exists()).toBe(false)
    expect(w.get('button').attributes('disabled')).toBeDefined()
  })

  it('closes only the mouse-opened host menu on Escape and preserves the draft', async () => {
    const closed = vi.fn()
    const w = mount({ components: { HostSwitcher, ModalDialog }, setup: () => ({ hosts, statusOf, closed }),
      template: '<ModalDialog :open="true" title="Shortcut" @close="closed"><input aria-label="Draft" value="Unsaved command" /><HostSwitcher :hosts="hosts" active-id="local" label="Local" :status-of="statusOf" /></ModalDialog>',
    }, { attachTo: document.body })
    wrappers.push(w)
    await flushPromises()
    const trigger = body().get('.host-switcher__trigger')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click')
    expect(body().find('.host-switcher__menu').exists()).toBe(true)
    await trigger.trigger('keydown', { key: 'Escape' })
    await flushPromises()
    expect(closed).not.toHaveBeenCalled()
    expect(body().find('.host-switcher__menu').exists()).toBe(false)
    expect((body().get('[aria-label="Draft"]').element as HTMLInputElement).value).toBe('Unsaved command')
    expect(document.activeElement).toBe(trigger.element)
    await trigger.trigger('keydown', { key: 'Escape' })
    expect(closed).toHaveBeenCalledOnce()
  })

  it('keeps its floating menu above the modal and handles Tab and Escape without closing the form', async () => {
    const closed = vi.fn()
    const w = mount({ components: { HostSwitcher, ModalDialog }, setup: () => ({ hosts, statusOf, closed }),
      template: '<ModalDialog :open="true" title="Shortcut" @close="closed"><HostSwitcher :hosts="hosts" active-id="local" label="Local" :status-of="statusOf" /></ModalDialog>',
    }, { attachTo: document.body })
    wrappers.push(w)
    await flushPromises()
    const trigger = body().get('.host-switcher__trigger')
    await trigger.trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    expect(body().get('.host-switcher__menu').element.parentElement).toBe(document.body)
    expect(body().get('.host-switcher__menu').classes()).toContain('is-in-modal')
    expect(Number((body().get('.host-switcher__menu').element as HTMLElement).style.zIndex)).toBeGreaterThanOrEqual(101)
    await body().get('input').trigger('keydown', { key: 'Tab' })
    expect(document.activeElement).toBe(body().get('.host-switcher__refresh').element)
    await body().get('.host-switcher__refresh').trigger('keydown', { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(body().get('input').element)
    await body().get('input').trigger('keydown', { key: 'Escape' })
    await flushPromises()
    expect(closed).not.toHaveBeenCalled()
    expect(body().find('.host-switcher__menu').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
  })
})
