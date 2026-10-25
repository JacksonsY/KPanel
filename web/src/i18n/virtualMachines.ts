// Loaded with the virtual-machine route, including its three language packs.
import { useI18n } from '@/i18n'
import { shallowRef, watch } from 'vue'

const zh = {
  title: '虚拟机', subtitle: '创建独立系统，集中管理运行状态与资源。', create: '创建虚拟机', refresh: '刷新', loading: '正在读取虚拟机…', retry: '重试', close: '关闭', back: '上一步', next: '下一步', cancel: '取消', confirm: '确认操作', search: '搜索虚拟机', all: '全部', empty: '还没有虚拟机', emptyHint: '选择系统并分配资源，即可开始创建。', noMatch: '没有匹配的虚拟机', name: '名称', state: '状态', resources: '资源', actions: '操作', cpus: 'CPU 核数', memory: '内存（MiB）', disk: '磁盘（GiB）', console: '控制台', start: '启动', shutdown: '关机', reboot: '重启', poweroff: '强制断电', delete: '删除', autostart: '开机自启', eject: '弹出安装镜像', running: '运行中', stopped: '已关闭', paused: '已暂停', unknown: '未知', details: '详情', more: '更多操作', checking: '检查环境', install: '安装运行环境', installTitle: '准备虚拟机运行环境', installHint: '将安装 QEMU/KVM、libvirt 和镜像工具，启用虚拟化服务、默认存储池及默认 NAT 网络。安装需要联网，可能影响宿主机网络规则。', installNote: '已有组件会继续使用。安装失败可能保留已安装组件，可查看任务原因后继续处理。', missing: '缺少组件', dependencies_missing: '当前服务器尚未准备好虚拟机运行环境。', platform_unsupported: '当前平台尚不支持此虚拟机模块。', kvm_unavailable: '无法使用 KVM。请检查虚拟化服务、硬件虚拟化或 VPS 的嵌套虚拟化支持。', unsupportedHelp: '完成宿主机设置后，点击刷新重新检查。', manualInstall: '当前系统暂未提供自动安装，请先由管理员安装 libvirt、virt-install、qemu-img 和 cloud-localds。', readOnly: '当前 Agent 未启用宿主机写操作，仅可查看。', sourceStep: '选择系统', settingsStep: '配置资源', reviewStep: '确认创建', quick: '快速创建', quickHint: '下载官方云镜像，通过 SSH 公钥配置登录用户。', iso: '从 ISO 安装', isoHint: '选择存储池中的 ISO，创建后进入控制台安装系统。', image: '系统镜像', pool: '存储池', network: '网络', advanced: '高级设置', username: '登录用户名', publicKey: 'SSH 公钥', keyHint: '粘贴以 ssh-ed25519、ssh-rsa 或 ecdsa-sha2-nistp256 开头的公钥。请勿填写私钥。', noISO: '所选存储池中没有 ISO。可在文件管理器上传 ISO，再刷新此页。', files: '打开文件管理器', networkHint: 'NAT 网络通常允许虚拟机访问外网；外部访问仍需配置端口或路由。', capacityHint: '宿主机总资源', reviewHint: '创建任务将在后台执行。离开页面或刷新后，可在此查看进度。', readyHint: '创建完成表示虚拟机已建立；系统初始化、安装和联网仍需通过控制台或 SSH 核对。', downloadHint: '从 Debian / Ubuntu 官方站点下载并校验镜像。首次创建时间取决于网络和磁盘速度。', submit: '确认并创建', tasks: '最近任务', queued: '等待执行', taskRunning: '执行中', succeeded: '已完成', failed: '失败', needs_attention: '需要处理', installing: '安装组件', configuring: '配置运行环境', preparing_disk: '准备磁盘', downloading: '下载并校验镜像', creating: '创建虚拟机', verifying: '核对结果', applying: '执行操作', completed: '完成', taskNotice: '任务保留在服务器，关闭页面不会取消。', deleteHint: '删除会先关闭正在运行的虚拟机，再移除其配置。请核对是否保留磁盘。', deleteDisks: '同时删除关联虚拟磁盘（无法撤销）', keepDisks: '默认保留磁盘，后续可继续使用。', poweroffHint: '强制断电可能丢失未保存的数据。', shutdownHint: '将向系统发送关机请求；实际关闭时间取决于虚拟机内的系统。', rebootHint: '将向虚拟机内的系统发送重启请求。', ejectHint: '弹出安装镜像后，将从系统磁盘启动。', autostartHint: '调整宿主机启动时是否自动启动这台虚拟机。', addressPending: '地址尚未获取，可通过控制台查看。', diskFiles: '关联磁盘', taskFailureHint: '请核对实际资源后再重试。已创建的磁盘或组件可能保留，任务不会自动重复执行。', worker_interrupted: '后台任务已中断，请核对实际资源。', truncated: '资源列表已达到显示上限，部分资源未显示。', connecting: '正在连接控制台…', disconnected: '控制台已断开', reconnect: '重新连接', ctrlAltDel: '发送 Ctrl+Alt+Del', password: 'VNC 密码', connect: '连接', noConsole: '这台虚拟机尚未提供可连接的 VNC 控制台。', consoleFailed: '无法连接控制台。请核对虚拟机状态和 VNC 设置后重试。', vm_conflict: '资源状态已变化，请刷新后重试。', vm_busy: '另一项任务正在执行，请稍后重试。', vm_write_disabled: '当前 Agent 未启用宿主机写操作。', vm_unsupported: '当前主机不满足该操作的运行条件。', vm_invalid_request: '请检查名称、资源配置和 SSH 公钥。', vm_result_unknown: '提交结果未确认，请先刷新任务，避免重复创建。', vm_unavailable: '虚拟机操作未能完成，请查看详细原因。', operationFailed: '操作未能完成，请重试。', noResources: '请先准备可用的存储池和虚拟网络。', invalidName: '名称须以字母或数字开头，可含点、下划线和连字符，最多 63 个字符。', keyRequired: '快速创建需要有效的登录用户名和 SSH 公钥。', enabled: '已启用', disabled: '未启用', current: '当前配置', resultAccepted: '请求已接收，请查看任务与实际状态。', installAction: '安装环境', createAction: '创建', noJobs: '暂无任务', diskRetained: '失败后可能保留磁盘，位置可在任务详情中核对。', viewTask: '查看任务详情', confirmTarget: '目标虚拟机',
} as const
type Key = keyof typeof zh
export function useVMText() {
  const { locale } = useI18n()
  const messages = shallowRef<Record<Key,string>>(zh)
  let epoch=0
  watch(locale, async value => {
    const current=++epoch
    if(value==='zh-CN'){messages.value=zh;return}
    try {
      const loaded=value==='en-US' ? await import('@/i18n/pages/VirtualMachinesView/en-US') : await import('@/i18n/pages/VirtualMachinesView/zh-TW')
      if(current===epoch)messages.value=loaded.messages
    } catch { if(current===epoch)messages.value=zh }
  },{immediate:true})
  return (key: Key): string => messages.value[key]
}
export type VMTextKey = Key