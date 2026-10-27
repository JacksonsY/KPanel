import { createSSRApp, ssrContextKey, type Ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import EnvironmentView from './EnvironmentView.vue'
import { api } from '@/lib/api'
import type { WebEnvironmentBackup, WebEnvironmentJob, WebEnvironmentSummary } from '@/types/api'

const mocks = vi.hoisted(() => ({
  start: vi.fn(),
  toastSuccess: vi.fn(),
  toastDanger: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  ApiError: class MockApiError extends Error {},
  api: {
    webEnvironment: {
      summary: vi.fn(),
      catalog: vi.fn(),
      backups: vi.fn(),
      jobs: vi.fn(),
      job: vi.fn(),
      terminal: vi.fn(),
      terminalInput: vi.fn(),
      start: mocks.start,
      backupDownloadURL: vi.fn(),
    },
  },
}))

vi.mock('@/stores/toast', () => ({
  useToast: () => ({
    success: mocks.toastSuccess,
    danger: mocks.toastDanger,
  }),
}))

interface EnvironmentBindings {
  wafLabel: Readonly<Ref<string>>
  wafBadge: Readonly<Ref<string>>
  summary: Ref<WebEnvironmentSummary | undefined>
  backups: Ref<WebEnvironmentBackup[]>
  jobs: Ref<WebEnvironmentJob[]>
  error: Ref<string>
  auxiliaryWarning: Readonly<Ref<string>>
  terminalOpen: Ref<boolean>
  terminalJob: Ref<WebEnvironmentJob | undefined>
  cloudflare: { account: string; token: string; zoneId: string }
  cloudflareFormVisible: Readonly<Ref<boolean>>
  cloudflareAccountInput: Ref<HTMLInputElement | undefined>
  cloudflareReconfigureButton: Ref<HTMLButtonElement | undefined>
  editCloudflare: (editing: boolean) => Promise<void>
  configureCloudflare: (operation: 'cloudflare-fail2ban' | 'cloudflare-shield') => void
  load: (silent?: boolean) => Promise<void>
  loadBackups: (force?: boolean) => Promise<void>
  start: (input: Record<string, unknown>) => Promise<void>
  stopJobPolling: () => void
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((next) => {
    resolve = next
  })
  return { promise, resolve }
}

function setupView(): EnvironmentBindings {
  const component = EnvironmentView as unknown as {
    setup: (props: Record<string, never>, context: { expose: () => void }) => EnvironmentBindings
  }
  const app = createSSRApp({ render: () => null })
  app.provide(ssrContextKey, { modules: new Set<string>() })
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
  try {
    return app.runWithContext(() => component.setup({}, { expose: () => undefined }))
  } finally {
    warn.mockRestore()
  }
}

function summary(): WebEnvironmentSummary {
  return {
    protocolVersion: '1',
    state: 'installed',
    profile: 'full',
    health: 'healthy',
    webRoot: '/home/web',
    diskBytes: 1,
    siteCount: 1,
    databaseCount: 1,
    certificateCount: 1,
    composeValid: true,
    nginxValid: true,
    resourceVersion: `sha256:${'a'.repeat(64)}`,
    scriptVersion: 'test',
    latestBackup: '',
    portConflicts: [],
    components: [],
    protection: { fail2ban: false, waf: false, cloudflare: false, ddos: false },
    optimization: { mode: 'custom', gzip: false, brotli: false, zstd: false },
    observedAt: '2026-07-28T00:00:00Z',
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  vi.stubGlobal('window', {
    setTimeout: globalThis.setTimeout,
    clearTimeout: globalThis.clearTimeout,
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('EnvironmentView Cloudflare configuration', () => {
  it('refreshes an existing configuration without requesting credentials or starting a job', async () => {
    const configured = summary()
    configured.protection.cloudflare = true
    vi.mocked(api.webEnvironment.summary).mockResolvedValue(configured)
    vi.mocked(api.webEnvironment.jobs).mockResolvedValue({ items: [], total: 0 })
    const view = setupView()
    view.summary.value = configured

    expect(view.cloudflareFormVisible.value).toBe(false)
    await view.load(true)

    expect(api.webEnvironment.summary).toHaveBeenCalledOnce()
    expect(view.cloudflareFormVisible.value).toBe(false)
    expect(view.cloudflare.token).toBe('')
    expect(mocks.start).not.toHaveBeenCalled()
    expect(mocks.toastDanger).not.toHaveBeenCalled()
  })

  it('opens explicit reconfiguration and clears the secret when returning to the status view', async () => {
    const view = setupView()
    view.summary.value = summary()
    view.summary.value.protection.cloudflare = true
    const focusAccount = vi.fn()
    const focusButton = vi.fn()
    view.cloudflareAccountInput.value = { focus: focusAccount } as unknown as HTMLInputElement
    view.cloudflareReconfigureButton.value = { focus: focusButton } as unknown as HTMLButtonElement

    await view.editCloudflare(true)
    expect(view.cloudflareFormVisible.value).toBe(true)
    expect(focusAccount).toHaveBeenCalledOnce()
    view.cloudflare.token = 'unsent-test-secret'
    await view.editCloudflare(false)

    expect(view.cloudflareFormVisible.value).toBe(false)
    expect(view.cloudflare.token).toBe('')
    expect(focusButton).toHaveBeenCalledOnce()
    expect(mocks.start).not.toHaveBeenCalled()
  })

  it('shows setup when no configuration is detected and still validates required credentials', () => {
    const view = setupView()
    view.summary.value = summary()
    expect(view.cloudflareFormVisible.value).toBe(true)
    view.configureCloudflare('cloudflare-fail2ban')
    expect(mocks.toastDanger).toHaveBeenCalledWith('Cloudflare 参数不完整', expect.any(String))
    expect(mocks.start).not.toHaveBeenCalled()
  })

  it('keeps reconfiguration available and preserves the existing write payload', async () => {
    const view = setupView()
    view.summary.value = summary()
    view.summary.value.protection.cloudflare = true
    await view.editCloudflare(true)
    view.cloudflare.account = 'operator@example.com'
    view.cloudflare.token = 'new-test-secret'
    mocks.start.mockImplementationOnce(() => new Promise(() => undefined))

    view.configureCloudflare('cloudflare-fail2ban')

    expect(mocks.start).toHaveBeenCalledWith({
      action: 'protection.configure', operation: 'cloudflare-fail2ban',
      cloudflareAccount: 'operator@example.com', cloudflareToken: 'new-test-secret',
      expectedResourceVersion: view.summary.value.resourceVersion,
    })
    expect(view.cloudflare.token).toBe('')
  })

  it('still requires a Zone ID when enabling the shield on a configured host', async () => {
    const view = setupView()
    view.summary.value = summary()
    view.summary.value.protection.cloudflare = true
    await view.editCloudflare(true)
    view.cloudflare.account = 'operator@example.com'
    view.cloudflare.token = 'test-secret'
    view.configureCloudflare('cloudflare-shield')
    expect(mocks.toastDanger).toHaveBeenCalledWith('Cloudflare 参数不完整', expect.any(String))
    expect(mocks.start).not.toHaveBeenCalled()
  })
})

describe('EnvironmentView background jobs', () => {
  it('distinguishes detection, mixed and unknown WAF state from blocking', () => {
    const view = setupView()
    view.summary.value = summary()
    view.summary.value.protection.wafMode = 'detect'
    expect(view.wafLabel.value).toBe('检测模式')
    expect(view.wafBadge.value).toBe('warning')
    view.summary.value.protection.wafMode = 'mixed'
    expect(view.wafLabel.value).toBe('按站点配置')
    view.summary.value.protection.wafMode = 'unknown'
    expect(view.wafBadge.value).toBe('unknown')
    view.summary.value.protection.wafMode = 'constructor' as WebEnvironmentSummary['protection']['wafMode']
    expect(view.wafLabel.value).toBe('状态待核验')
    expect(view.wafBadge.value).toBe('unknown')
    view.summary.value.protection.wafMode = 'on'
    expect(view.wafLabel.value).toBe('拦截模式')
    expect(view.wafBadge.value).toBe('healthy')
  })

  it('renders the environment summary without requesting heavy section data', async () => {
    vi.mocked(api.webEnvironment.summary).mockResolvedValueOnce(summary())
    vi.mocked(api.webEnvironment.jobs).mockImplementationOnce(() => new Promise(() => undefined))
    const view = setupView()

    await view.load()

    expect(view.summary.value).toEqual(summary())
    expect(view.error.value).toBe('')
    expect(api.webEnvironment.catalog).not.toHaveBeenCalled()
    expect(api.webEnvironment.backups).not.toHaveBeenCalled()
  })

  it('keeps environment management available when a lazy backup list fails', async () => {
    vi.mocked(api.webEnvironment.backups).mockRejectedValueOnce(new Error('backup unavailable'))
    const view = setupView()

    await view.loadBackups()

    expect(view.backups.value).toEqual([])
    expect(view.auxiliaryWarning.value).toBe('备份列表暂时无法读取')
  })

  it('always submits the current environment resource version and reopens the terminal', async () => {
    const job: WebEnvironmentJob = {
      id: 'b'.repeat(32),
      action: 'backup.create',
      target: 'web',
      status: 'running',
      stage: 'running',
      progress: 1,
      message: 'running',
      createdAt: '2026-07-28T00:00:00Z',
    }
    mocks.start.mockResolvedValueOnce(job)
    const view = setupView()
    view.summary.value = summary()

    await view.start({ action: 'backup.create' })

    expect(mocks.start).toHaveBeenCalledWith({
      action: 'backup.create',
      expectedResourceVersion: view.summary.value?.resourceVersion,
    })
    expect(view.jobs.value).toEqual([job])
    expect(view.terminalJob.value).toEqual(job)
    expect(view.terminalOpen.value).toBe(true)
    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      '任务已转入后台',
      '关闭终端或刷新页面都不会中断执行。',
    )
  })

  it('does not restart job polling after lifecycle cleanup while a request is in flight', async () => {
    const job: WebEnvironmentJob = {
      id: 'c'.repeat(32),
      action: 'backup.create',
      target: 'web',
      status: 'running',
      stage: 'running',
      progress: 1,
      message: 'running',
      createdAt: '2026-07-28T00:00:00Z',
    }
    const pending = deferred<WebEnvironmentJob>()
    let requestSignal: AbortSignal | undefined
    mocks.start.mockResolvedValueOnce(job)
    vi.mocked(api.webEnvironment.job).mockImplementationOnce((_id, signal) => {
      requestSignal = signal
      return pending.promise
    })
    const view = setupView()
    view.summary.value = summary()
    await view.start({ action: 'backup.create' })

    await vi.advanceTimersByTimeAsync(1_200)
    expect(api.webEnvironment.job).toHaveBeenCalledTimes(1)

    view.stopJobPolling()
    expect(requestSignal?.aborted).toBe(true)
    pending.resolve({ ...job, progress: 20 })
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(2_300)

    expect(api.webEnvironment.job).toHaveBeenCalledTimes(1)
  })
})
