import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, resetApiSecurityState } from './api'

afterEach(() => { vi.unstubAllGlobals(); resetApiSecurityState() })
describe('systemd service API', () => {
  it('encodes instance and escaped unit names as a single path segment', async () => {
    const fetch = vi.fn().mockImplementation(async () => new Response('{}', { headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetch)
    const name = 'worker@mount\\x2dname.service'
    const controller = new AbortController()
    await api.system.service(name, controller.signal)
    await api.system.serviceLogs(name, controller.signal)
    expect(fetch.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/system/services/worker%40mount%5Cx2dname.service',
      '/api/v1/system/services/worker%40mount%5Cx2dname.service/logs',
    ])
    expect(fetch.mock.calls[0]![1].signal).toBe(controller.signal)
  })
  it('preserves the server acceptance flag and posts a versioned action', async () => {
    const fetch = vi.fn().mockImplementation(async () => new Response('{"accepted":true}', { status: 202, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetch)
    const input = { name: 'ssh.service', action: 'stop' as const, expectedResourceVersion: 'a'.repeat(64) }
    expect(await api.system.serviceAction(input)).toEqual({ accepted: true })
    expect(fetch.mock.calls[0]![0]).toBe('/api/v1/system/service-actions')
    expect(fetch.mock.calls[0]![1].method).toBe('POST')
    expect(JSON.parse(fetch.mock.calls[0]![1].body)).toEqual(input)
  })
})
