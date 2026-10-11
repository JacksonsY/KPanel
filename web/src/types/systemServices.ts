export interface SystemService {
  name: string
  description: string
  loadState: string
  activeState: string
  subState: string
  unitFileState: string
}

export interface SystemServiceDetail extends SystemService {
  resourceVersion: string
  fragmentPath: string
  mainPID: number
  canReload: boolean
  canStart: boolean
  canStop: boolean
}

export interface SystemServicesSnapshot {
  supported: boolean
  reason?: string
  services: SystemService[]
  observedAt: string
  truncated: boolean
}

export interface SystemServiceLogs {
  name: string
  content: string
  truncated: boolean
  observedAt: string
}

export type SystemServiceAction = 'start' | 'stop' | 'restart' | 'reload' | 'enable' | 'disable'

export interface SystemServiceActionInput {
  name: string
  action: SystemServiceAction
  expectedResourceVersion: string
}

export interface SystemServiceActionResult {
  name: string
  action: SystemServiceAction
  accepted: boolean
  message: string
  service?: SystemServiceDetail
}
