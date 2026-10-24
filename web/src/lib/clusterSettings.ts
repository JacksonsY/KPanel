export type ClusterSettingsTab = 'notifications' | 'share' | 'remote'
const key = 'kpanel:cluster-settings:tab'
export const clusterRemoteAccessChanged = 'kpanel:cluster-remote-access-changed'

export function readClusterSettingsTab(): ClusterSettingsTab {
  try {
    const value = localStorage.getItem(key)
    if (value === 'notifications' || value === 'share' || value === 'remote') return value
  } catch { /* Browser preferences are optional. */ }
  return 'notifications'
}

export function saveClusterSettingsTab(tab: ClusterSettingsTab): void {
  try { localStorage.setItem(key, tab) } catch { /* A blocked preference never blocks settings. */ }
}
