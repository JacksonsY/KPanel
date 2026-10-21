export type BatchState = 'queued' | 'connecting' | 'running' | 'closing' | 'succeeded' | 'failed' | 'timed_out' | 'stopped' | 'interrupted'
export interface BatchResult {
  host: { id: string; name: string; shell: 'posix' | 'powershell' }
  state: BatchState
  error?: string
  output?: string
  truncated: boolean
  outputAvailable: boolean
}
export interface TerminalBatch {
  id: string
  command: string
  state: 'running' | 'completed' | 'stopped' | 'interrupted'
  createdAt: string
  updatedAt: string
  results: BatchResult[]
  cleanupPending?: boolean
  cleanupRunning?: boolean
}
export interface TerminalBatchList {
  items: TerminalBatch[]
  available: boolean
  warning?: string
}
