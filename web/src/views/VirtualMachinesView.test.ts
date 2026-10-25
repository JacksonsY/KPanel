// @vitest-environment jsdom
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setLocale } from '@/i18n'
import View from './VirtualMachinesView.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), action: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: { virtualMachines: mocks } }))
const machine = { id:'11111111-1111-1111-1111-111111111111', name:'external-server', state:'running', cpus:2, memoryMiB:2048, autostart:false, resourceVersion:'a'.repeat(64), addresses:[], disks:[], console:true }
const data = { environment:{ supported:true, ready:true, writable:true, installable:true, reason:'', cpus:4, memoryMiB:8192, missing:[] }, machines:[machine], pools:[{name:'default'}], networks:[{name:'default'}], images:[{name:'debian.iso',path:'default',kind:'iso'}], jobs:[], truncated:false }
enableAutoUnmount(afterEach)
beforeEach(async () => { vi.resetAllMocks(); await setLocale('zh-CN',false); mocks.list.mockImplementation(() => Promise.resolve(structuredClone(data))); mocks.action.mockResolvedValue({status:'queued'}) })
function view() { return mount(View,{ attachTo:document.body, global:{stubs:{teleport:true,VMConsole:true}} }) }
async function click(w: ReturnType<typeof view>, text:string) { await w.findAll('button').find(b => b.text()===text)!.trigger('click'); await flushPromises() }

describe('virtual machine journey', () => {
  it('shows existing external resources and filters locally', async () => {
    const w=view(); await flushPromises(); expect(w.text()).toContain('external-server')
    await w.get('input[placeholder="搜索虚拟机"]').setValue('missing')
    expect(w.text()).toContain('没有匹配的虚拟机'); expect(mocks.list).toHaveBeenCalledTimes(1)
  })
  it('keeps runtime absence distinct from an empty inventory', async () => {
    mocks.list.mockResolvedValue({...data,environment:{...data.environment,ready:false,reason:'dependencies_missing',missing:['virsh']},machines:[]})
    const w=view();await flushPromises();expect(w.text()).toContain('安装运行环境');expect(w.text()).not.toContain('还没有虚拟机')
  })
  it('validates creation before starting a task and submits only once', async () => {
    const w=view();await flushPromises();await click(w,'创建虚拟机')
    await w.get('input[type="radio"][value="iso"]').setValue(true)
    await w.get('.vm-form').trigger('submit');await flushPromises()
    await w.get('.vm-form').trigger('submit');expect(w.text()).toContain('名称须以字母或数字开头');expect(mocks.action).not.toHaveBeenCalled()
    await w.get('input[placeholder="my-server"]').setValue('my-test')
    await w.get('.vm-form').trigger('submit');await flushPromises();expect(w.text()).toContain('确认创建')
    await w.get('.vm-form').trigger('submit');await flushPromises()
    expect(mocks.action).toHaveBeenCalledTimes(1)
    expect(mocks.action.mock.calls[0]![0]).toMatchObject({action:'create',name:'my-test',source:'iso',image:'debian.iso',pool:'default',network:'default'})
    expect(w.text()).toContain('请求已接收')
  })
  it('confirms deletion with disk retention by default', async () => {
    const w=view();await flushPromises();await click(w,'详情');await click(w,'删除')
    expect(mocks.action).not.toHaveBeenCalled()
    expect(w.get('input[type="checkbox"]').element).toHaveProperty('checked',false)
    await click(w,'确认操作')
    expect(mocks.action.mock.calls[0]![0]).toMatchObject({action:'delete',id:machine.id,deleteDisks:false,expectedResourceVersion:machine.resourceVersion})
  })
  it('renders English labels and an actual failed read', async () => {
    await setLocale('en-US',false);mocks.list.mockRejectedValue(new Error('Unavailable'))
    const w=view();await flushPromises();expect(w.text()).toContain('Unavailable');expect(w.text()).toContain('Retry');expect(w.find('h1').text()).toBe('Virtual machines')
  })
})
