import { describe, expect, it } from 'vitest'
import { fileHostStatus } from './fileHostStatus'
import type { ClusterHost } from '@/types/api'

describe('file transport availability', () => {
  const host = { id: 'remote', kind: 'panel', fileManagementAvailable: true } as ClusterHost
  it('lets an authorized file relay report its own result after a summary outage', () => {
    for (const state of ['online', 'degraded', 'stale', 'offline'] as const) {
      expect(fileHostStatus({ ...host, state }).action).toBe('select')
    }
  })
  it('preserves authentication, certificate, protocol and pairing restrictions', () => {
    for (const state of ['auth_failed', 'tls_error', 'incompatible', 'pairing', 'revoking'] as const) {
      expect(fileHostStatus({ ...host, state }).action).toBe('manage')
    }
    expect(fileHostStatus({ ...host, state: 'offline', fileManagementAvailable: false }).action).not.toBe('select')
  })
  it('continues to use the independent light file broker capability', () => {
    expect(fileHostStatus({ ...host, kind: 'light_node', state: 'offline' }).action).toBe('select')
    expect(fileHostStatus({ ...host, kind: 'light_node', state: 'online', fileManagementAvailable: false }).action).toBe('manage')
  })
})
