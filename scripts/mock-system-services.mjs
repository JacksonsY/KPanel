// In-memory preview data only; no host services or commands are used.
const services = [
  ['nginx.service', 'A high performance web server', 'active', 'enabled'],
  ['ssh.service', 'OpenBSD Secure Shell server', 'active', 'enabled'],
  ['docker.service', 'Docker Application Container Engine', 'active', 'enabled'],
  ['backup.service', 'Scheduled backup worker', 'inactive', 'disabled'],
  ['example-failed.service', 'Example service with a failed start', 'failed', 'disabled'],
  ['example-static.service', 'Example static service without an install section', 'inactive', 'static'],
  ['worker@long-customer-instance-name-for-responsive-layout.service', 'An example instance service with a long name and description', 'inactive', 'disabled'],
].map(([name, description, activeState, unitFileState]) => ({ name, description, activeState, unitFileState,
  loadState: 'loaded', subState: activeState === 'active' ? 'running' : activeState === 'failed' ? 'failed' : 'dead',
  resourceVersion: '1'.padStart(64, '0'), fragmentPath: `/usr/lib/systemd/system/${name}`, mainPID: activeState === 'active' ? 1234 : 0,
  canReload: name === 'nginx.service', canStart: true, canStop: true,
}))

export async function handleMockSystemServices(request, response, url, { send, readJSON }) {
  if (request.method === 'GET' && url.pathname === '/api/v1/system/services') {
    send(response, 200, { supported: true, services, observedAt: new Date().toISOString(), truncated: false })
    return true
  }
  const match = url.pathname.match(/^\/api\/v1\/system\/services\/([^/]+)(\/logs)?$/)
  if (request.method === 'GET' && match) {
    const name = decodeURIComponent(match[1])
    const service = services.find((item) => item.name === name)
    if (!service) send(response, 404, { code: 'system_service_not_found', title: 'Service not found' })
    else if (match[2]) send(response, 200, { name, content: `${new Date().toISOString()} [mock] ${name}: ${service.activeState}\nExample service journal output.`, truncated: false, observedAt: new Date().toISOString() })
    else send(response, 200, service)
    return true
  }
  if (request.method !== 'POST' || url.pathname !== '/api/v1/system/service-actions') return false
  const input = await readJSON(request)
  const service = services.find((item) => item.name === input.name)
  if (!service || !['start', 'stop', 'restart', 'reload', 'enable', 'disable'].includes(input.action)) {
    send(response, 422, { code: 'invalid_system_service_action', title: 'Invalid service action' })
    return true
  }
  if (input.expectedResourceVersion !== service.resourceVersion) {
    send(response, 409, { code: 'system_service_changed', title: 'Service state changed. Refresh and retry.' })
    return true
  }
  const boot = ['enable', 'disable'].includes(input.action)
  if (boot && service.unitFileState === 'static') {
    send(response, 422, { code: 'system_service_action_failed', title: 'This static unit has no installation configuration.' })
    return true
  }
  if (service.name === 'example-failed.service' && ['start', 'restart'].includes(input.action)) {
    send(response, 202, { name: service.name, action: input.action, accepted: true, message: 'Mock request queued; the example service will remain failed.', service })
    return true
  }
  if (boot) service.unitFileState = input.action === 'enable' ? 'enabled' : 'disabled'
  else {
    service.activeState = input.action === 'stop' ? 'inactive' : 'active'
    service.subState = input.action === 'stop' ? 'dead' : 'running'
    service.mainPID = input.action === 'stop' ? 0 : 1234
  }
  service.resourceVersion = (BigInt(`0x${service.resourceVersion}`) + 1n).toString(16).padStart(64, '0')
  send(response, boot ? 200 : 202, { name: service.name, action: input.action, accepted: !boot, message: boot ? 'Mock boot settings updated' : 'Mock request queued', service })
  return true
}
