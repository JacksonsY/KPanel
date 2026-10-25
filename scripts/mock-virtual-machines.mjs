// Preview-only in-memory fixtures. Never invoke a runtime or access host disks.
const version = '1'.repeat(64)
const machines = [
  { id: '11111111-1111-1111-1111-111111111111', name: 'ubuntu-services', state: 'running', cpus: 2, memoryMiB: 2048, autostart: true, resourceVersion: version, addresses: ['192.168.122.42'], disks: [{ target: 'vda', path: '/var/lib/libvirt/images/ubuntu-services.qcow2', device: 'disk' }], console: true },
  { id: '22222222-2222-2222-2222-222222222222', name: 'debian-test', state: 'shut off', cpus: 1, memoryMiB: 1024, autostart: false, resourceVersion: version, addresses: [], disks: [{ target: 'vda', path: '/var/lib/libvirt/images/debian-test.qcow2', device: 'disk' }], console: true },
]
const jobs = []
const requests = new Map()
const environment = { supported: true, ready: true, writable: true, installable: true, reason: '', architecture: 'amd64', missing: [], cpus: 8, memoryMiB: 16384 }

export async function handleMockVirtualMachines(request, response, url, { send, readJSON }) {
  if (request.method === 'GET' && url.pathname === '/api/v1/virtual-machines') {
    send(response, 200, { environment, machines, networks: [{ name: 'default' }], pools: [{ name: 'default' }], images: [{ name: 'debian-12.iso', path: 'default', kind: 'iso' }], jobs, observedAt: new Date().toISOString(), truncated: false })
    return true
  }
  if (request.method !== 'POST' || url.pathname !== '/api/v1/virtual-machines/actions') return false
  const input = await readJSON(request)
  const existing = requests.get(input.requestId)
  if (existing) { send(response, 202, existing); return true }
  if (jobs.some(job => ['queued', 'running'].includes(job.status))) { send(response, 429, { code: 'vm_busy', title: 'Task in progress' }); return true }
  const machine = machines.find(item => item.id === input.id)
  if (!['create', 'install'].includes(input.action) && (!machine || input.expectedResourceVersion !== machine.resourceVersion)) { send(response, 409, { code: 'vm_conflict', title: 'Resource changed' }); return true }
  const now = new Date().toISOString()
  const job = { id: input.requestId, action: input.action, name: input.name || machine?.name || '', status: 'running', stage: input.action === 'create' ? 'downloading' : 'applying', message: '', createdAt: now, updatedAt: now }
  jobs.unshift(job); requests.set(input.requestId, job)
  if (jobs.length > 32) { const old = jobs.pop(); requests.delete(old.id) }
  setTimeout(() => {
    if (input.name === 'fail-create') { job.status = 'needs_attention'; job.message = 'Mock: image checksum verification failed. No virtual machine was created.'; return }
    if (input.action === 'create') machines.push({ id: input.requestId, name: input.name, cpus: input.cpus, memoryMiB: input.memoryMiB, state: 'running', autostart: input.autostart, resourceVersion: version, addresses: [], disks: [{ target: 'vda', path: `/var/lib/libvirt/images/${input.name}.qcow2`, device: 'disk' }], console: true })
    else if (input.action === 'delete') machines.splice(machines.indexOf(machine), 1)
    else if (input.action === 'autostart') machine.autostart = input.autostart
    else if (['start', 'reboot', 'shutdown', 'poweroff'].includes(input.action)) machine.state = ['shutdown', 'poweroff'].includes(input.action) ? 'shut off' : 'running'
    if (machine) machine.resourceVersion = (BigInt(`0x${machine.resourceVersion}`) + 1n).toString(16).padStart(64, '0')
    job.status = 'succeeded'; job.stage = 'completed'; job.updatedAt = new Date().toISOString()
  }, 1000).unref()
  send(response, 202, job)
  return true
}
