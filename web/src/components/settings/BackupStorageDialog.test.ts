// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import BackupStorageDialog from './BackupStorageDialog.vue'
import { backups, type BackupSettings } from '@/lib/backup'
vi.mock('@/lib/backup', async (original) => ({ ...await original<typeof import('@/lib/backup')>(), backups: { startOneDrive: vi.fn(), pollOneDrive: vi.fn(), cancelOneDrive: vi.fn().mockResolvedValue({ ok: true }), saveStorage: vi.fn() } }))
const settings: BackupSettings = { revision: 'revision', storages: [], schedule: { enabled: false, modules: ['panel'], storageId: '', frequency: 'daily', hour: 3, minute: 0, weekday: 0, day: 1, timezone: 'UTC', keep: 7 } }
const auth = { id: 'authorization', userCode: 'ABCD-EFGH', verificationUri: 'https://microsoft.com/devicelogin', expiresIn: 900, interval: 5 }
function render() { return mount(BackupStorageDialog, { props: { settings }, global: { stubs: { ModalDialog: { template: '<div><slot /></div>' } } } }) }
async function prepare(wrapper: ReturnType<typeof render>) {
  await wrapper.get('button.button--primary').trigger('click')
  await wrapper.get('select').setValue('onedrive')
  await wrapper.get('input[maxlength="80"]').setValue('My OneDrive')
  await wrapper.get('input[maxlength="36"]').setValue('11111111-2222-3333-4444-555555555555')
}
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks() })
describe('OneDrive backup storage', () => {
  it('connects using a device code and saves only after authorization completes', async () => {
    vi.useFakeTimers()
    vi.mocked(backups.startOneDrive).mockResolvedValue(auth)
    vi.mocked(backups.pollOneDrive).mockResolvedValueOnce({ pending: true }).mockResolvedValueOnce({ pending: false, settings: { ...settings, storages: [{ id: 'drive', name: 'My OneDrive', kind: 'onedrive', endpoint: 'https://graph.microsoft.com/v1.0', prefix: 'kpanel', hasSecret: true }] } })
    const wrapper = render(); await prepare(wrapper)
    expect(wrapper.find('input[type=password]').exists()).toBe(false)
    expect(wrapper.get('button[type=submit]').attributes('disabled')).toBeDefined()
    await wrapper.get('button.button--secondary').trigger('click'); await flushPromises()
    expect(backups.startOneDrive).toHaveBeenCalledWith('revision', expect.objectContaining({ kind: 'onedrive', tenantId: 'common', secret: '' }))
    expect(wrapper.text()).toContain(auth.userCode)
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(wrapper.emitted('saved')).toBeUndefined()
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.text()).toContain('OneDrive 已连接。')
    wrapper.unmount()
  })
  it('cancels polling on close and ignores late authorization results', async () => {
    vi.useFakeTimers(); vi.mocked(backups.startOneDrive).mockResolvedValue(auth)
    let finish!: (value: { pending: boolean; settings: BackupSettings }) => void
    vi.mocked(backups.pollOneDrive).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const wrapper = render(); await prepare(wrapper)
    await wrapper.get('button.button--secondary').trigger('click'); await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    wrapper.unmount()
    finish({ pending: false, settings }); await flushPromises()
    expect(backups.cancelOneDrive).toHaveBeenCalledWith(auth.id)
    expect(wrapper.emitted('saved')).toBeUndefined()
  })
  it('shows a recoverable failure without reporting a saved connection', async () => {
    vi.mocked(backups.startOneDrive).mockRejectedValue({ code: 'backup_remote_unavailable' })
    const wrapper = render(); await prepare(wrapper)
    await wrapper.get('button.button--secondary').trigger('click'); await flushPromises()
    expect(wrapper.get('[role=alert]').text()).toContain('远程连接失败')
    expect(wrapper.emitted('saved')).toBeUndefined()
    expect(wrapper.get('fieldset').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})
