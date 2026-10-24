// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { readClusterSettingsTab, saveClusterSettingsTab } from './clusterSettings'
beforeEach(() => localStorage.clear())
describe('cluster settings tab preference', () => {
  it('starts with notifications and remembers the last tab', () => {
    expect(readClusterSettingsTab()).toBe('notifications')
    saveClusterSettingsTab('remote'); expect(readClusterSettingsTab()).toBe('remote')
    saveClusterSettingsTab('share'); expect(readClusterSettingsTab()).toBe('share')
  })
  it('tolerates unknown and blocked browser storage', () => {
    localStorage.setItem('kpanel:cluster-settings:tab', 'unknown'); expect(readClusterSettingsTab()).toBe('notifications')
    const read = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    expect(readClusterSettingsTab()).toBe('notifications'); read.mockRestore()
  })
})
