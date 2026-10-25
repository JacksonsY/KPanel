export interface VirtualMachine {
  id: string; name: string; state: string; cpus: number; memoryMiB: number
  autostart: boolean; resourceVersion: string; addresses: string[]
  disks: { target: string; path: string; device: string }[]; console: boolean
}
export interface VMJob {
  id: string; action: string; name: string; machineId?: string; artifacts?: string[]
  status: string; stage: string; message: string; createdAt: string; updatedAt: string
}
export interface VMSnapshot {
  environment: { connected?: boolean; supported: boolean; ready: boolean; writable: boolean; installable: boolean; reason: string; architecture: string; missing: string[]; cpus: number; memoryMiB: number }
  machines: VirtualMachine[]; networks: { name: string }[]; pools: { name: string }[]
  images: { name: string; path: string; kind: string }[]; jobs: VMJob[]; observedAt: string; truncated: boolean
}
export interface VMRequest {
  action: string; requestId: string; id?: string; name?: string; source?: string; image?: string
  pool?: string; network?: string; cpus?: number; memoryMiB?: number; diskGiB?: number
  username?: string; publicKey?: string; autostart?: boolean; deleteDisks?: boolean; expectedResourceVersion?: string
}
