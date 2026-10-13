# KPanel v1.26.0-rc.16 发布验收记录

发布级别：L3。releaseChannel=preview，releaseTrain=1.26.0。批准的 main 基线 f0d289d865a9d794ede476ccaa4d9833bbd6c0e2；最终冻结产品提交 77cdfb4b5b0dd510f7610e822f8e33663c0ba24a。

[v1.26.0-rc.16](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.16) 于 2026-10-13T10:54:32Z（18:54:32+08:00）公开，draft=false、prerelease=true，非 GitHub Latest。注释标签对象 d5ce2ab99bb58f446456437ffbaafad91bc08c03 指向最终冻结提交。Release 工作流 [38384712721](https://github.com/kejilion/KPanel/actions/runs/38384712721) 全部必需步骤通过，预览候选归档按策略跳过。

候选 CI [38381496419](https://github.com/kejilion/KPanel/actions/runs/38381496419) 与依赖检查 [38381496423](https://github.com/kejilion/KPanel/actions/runs/38381496423) 通过后，同一提交快进 main；主线 CI [38383089221](https://github.com/kejilion/KPanel/actions/runs/38383089221) 与依赖检查 [38383088937](https://github.com/kejilion/KPanel/actions/runs/38383088937) 通过。标签依赖检查 [38384712687](https://github.com/kejilion/KPanel/actions/runs/38384712687) 通过。

## 公开产物

1.26.0-rc.16 与 preview 同指 OCI index sha256:3f79dc7686e8c1ad015b4b8698535f0425e819e901e659f42eec907427605a17。linux/amd64 manifest 为 sha256:446372df18d25311db14649e2e8b95db3401346ada1532594eaa01df1613973c，linux/arm64 为 sha256:169b9aa060e0bd29ef4734fd9c401ea79b626d49f51daf6c566e03437a3482fd。分别读取 Registry 配置 blob，两个平台的 version=1.26.0-rc.16、revision=77cdfb4b5b0dd510f7610e822f8e33663c0ba24a 均匹配。GitHub Latest 仍为 v1.25.1；Docker latest 与下述稳定基线一致。

arena-154 显式从公开仓库拉取上述不可变 index，使用冻结源码的 packaging/tests/image-e2e.sh 验收，image_e2e=pass、exit 0。测试后临时容器与网络数量均为 0，18080 端口无监听。证据覆盖公开镜像启动、版本/健康、代理头、静态资源字节和 bootstrap；不证明真实宿主机特权写入或生产升级。

GitHub Release 的 15 个附件全部下载，长度与 API SHA-256 摘要 15/15 匹配；SHA256SUMS 内 11 项逐一匹配。证据为 release-asset-verification.json 与 sha256sums-verification.json。

本任务本地 Mock 预览已停止、浏览器页已关闭。隔离 E2E 临时容器、网络与端口已清理；可信 Runner 归档、验收日志与发布附件保留供追溯。作者工作树保留。

## 候选处置

| 来源分支 | 原始 tip | 本次范围 |
| --- | --- | --- |
| perf/file-transfer-pipeline | 518fd4a9ce90e1448001b5dbe9de2787d97aff4a | 图库近期原图缓存、按显示尺寸加载、有界持久预览及权限重验、上传身份/恢复、接收缓冲和并发写入。 |
| fix/gallery-mobile-layout | 6e0bbac8eccc00d520c3f0e995686ca4422ca7e7 | 手机桌面图库分区高度与重叠修复。 |
| fix/desktop-container-spacing | 266ac53333e903848d39eb83afa7f4f9823c502b | 桌面小组件与分组卡片尺寸对齐。 |
| fix/cluster-telemetry-stability | e989168f03a268f4a796978945d5a8188d5099de | Agent 共享采集取消隔离、时限与并发、SSH 摘要缓存、新鲜样本确认恢复、已授权文件入口不再被遥测离线禁用。 |

systemd 服务管理前后端已随 RC9 发布，本次保留。offers 已在主线，非本次新功能。旧桌面文件快捷方式图标、分页/文件页旧布局仍按用户既有处置排除，未重新引入。backup-archive-data-path 产品补丁已有等价提交进入主线，其剩余未完成审计文档不作产品候选。share-theme-immersive 等未获当前准入证据的旧独有候选未混入本版。

四个来源 tip 均已包含于不可变 RC16 标签，可按表中精确 SHA 恢复；远端没有这四个活跃来源引用。本地来源分支/worktree 保留为作者工作区（存在独立预览与所有权，未擅自删除），不再计为待发布产品差异。唯一 release/v1.26.0-candidate 按预览策略保留在最终产品 SHA；验收文档另行走 docs 分支 CI，不改发布标签。

## 源码身份与复核

候选代码复用来源提交的 OCR/独立审阅证据；小型 CSS 修复按原提交记录 skipped。发布接管未运行新的 Cloudflare scoped/full 审计。最终覆盖检查 decision=scoped-required，100 个未覆盖边界提交、218 个文件，新增边界包 internal/bittorrent、internal/offers；此状态仅按预览版记录，不能据此宣称完成稳定版审计准入。

独立源码身份复核覆盖相对 RC15 的 41 个 Go 变更文件（25 个非测试文件）。无新增第三方 import 路径、生成 protobuf 或源码 symlink。Go 1.27.2 的六平台生产闭包包含 106 个模块、917 个第一方源文件；Linux 两架构新增五个生产 GoFiles。现有 not-in-production 模块均未进入生产闭包。复核结果 PASS WITH FOLLOW-UP，后续条件是 canonical L3 重新计算 actual/original 闭包并执行扫描。

- firstPartySourceSha256：c591cd06eaeda9c9ca1b070313d2fa7e3040fc12ad74ab38a26c81a23f336bd2。
- closureSha256：485cc7f1758b7186cf3305c9d5365667bc2116769593f6f6faaccfabc54785b8。
- 独立复核记录 SHA-256：b3ae18a2d788513fd2b322bfccba5aadab24532e7adb9e25b96ba99b27c27677。
- 另一提供商 CLI 不可用，使用与实现分离的干净 Codex 复核会话，fallback=provider-unavailable。
- 36 条原有扫描处置、观察窗口和 2026-10-25 到期日均未扩大。

## Runner 与过程证据

旧 RC15 Runner ID 缺失使 r1 在候选测试之前关闭失败。按仓库 packaging/release-runner/Dockerfile 的固定基础镜像摘要、包版本和 Node/npm smoke 重建，形成新的冻结执行计划。没有把新 Runner 冒充旧 ID。

- 镜像：kpanel-go127-security-runner:go1.27.2-node24.21.0。
- 新实际 ID：sha256:4edff59fa4c1441e3e97594593b255cc57a74e51dabbdd8db295e4ff51d0ca4f。
- 离线归档：runner-r2/kpanel-runner-v1.26.0-rc.16-d10a79a-l3-r2.tar，269331456 字节。
- 归档 SHA-256：1711d52c09cea4a8551c8390346fff299e8cb255a50a58a299db86ab4e5a984a。
- r2 已从归档加载并精确核对新 ID，随后源码身份门禁发现旧 RC15 绑定，299 项治理回归通过、1 项失败；原失败日志及 13 项哈希均保留核验。
- 修正提交 77cdfb4b 仅更新扫描证据身份与说明。r3 使用相同新 Runner 的实际 ID；可信归档保留在证据目录。

## 界面与业务边界

完整 L3 run v1.26.0-rc.16-77cdfb4-l3-r3 于 2026-10-13T09:40:54Z 开始，10:09:38Z 结束，status=passed、exit_code=0，12 项证据 SHA-256 远端与本地逐项匹配。source checks 总耗时 1191696 ms；web lane 662226 ms、Go lane 1191644 ms、deploy lane 4589 ms。

Go 全量测试、核心 panel/auth/dockerx race、go vet、torrent 上游兼容检查、前端类型检查/构建与场景包检查通过。Vitest 290 个文件通过，2745 项通过、6 项跳过。实际源码和原始上游版本 govulncheck 均无可达漏洞，保留 1 项 require-only 信号；npm audit 为 0。Trivy 生产闭包 106 模块无保留发现，selected 10 项与 standalone 24 项漏洞及 2 项配置发现按原有已复核范围保留，不能称为整个依赖图零风险。最终镜像中的 Panel/Agent 漏洞发现为 0；应用生命周期及更新/备份/中断恢复夹具通过。

最终干净候选通过本地 mock-ui / visual-composition 预览重新加载。390×844 桌面图库的封面、标题、主机选择、相册卡片与工具栏没有观察到重叠。已授权但遥测离线的夹具节点能显示模拟文件列表；前一精确产品树的认证失败/轻量节点未就绪状态亦已检查。桌面 1280×900 检查卡片和窗口组合；浏览器 warnings/errors 查询为空。

本次 Mock 不证明真实宿主机文件写入、systemd 写入、真实设备或读屏。125%/200% 实际浏览器缩放未验证，不能以视口模拟代替。冻结前后 web 与产品 Go 代码未再变更，最终变化只有审阅过的扫描身份记录。

## 跨仓库与通道

scriptLinkageState=not-required。内置脚本 revision=1800d955f216aebd2776674368a8e469a671c489，SHA-256=5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c。RC16 不改变脚本协议、安装更新契约或内置脚本内容。

应用市场 origin/main 为 6602ac36e0c329e35b74ff621721b23583e94f5f；kpanel.conf 与产品候选的 Git blob 同为 9d2d2efdc334db1ba014203370fdc8281f1a5870，默认 latest，无需新应用市场提交。本地 apps 检出较旧，未覆盖其工作树。稳定版基线为 v1.25.1，Docker latest 基线 sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325。

预览版禁止生产部署；本次生产写操作为 0。未连接 prod-108。真实生产升级/回滚、真实更新弹窗/API 尚未由本次隔离证据证明。

证据根目录：C:/GitHub/_release-evidence/v1.26.0-rc.16/。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-13T09:48:28+08:00
- 候选冻结时间：2026-10-13T17:39:07+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：2
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "l3/run-release-l3/missing-runner",
    "position": "before-production-write",
    "count": 1,
    "impact": "r1 的旧 Runner ID 不可用，候选代码尚未执行即终止。",
    "recoveryEvidence": "runner-r2 Dockerfile、image-inspect.json、archive.json，以及 r2 runner-load.log 和 runner.txt 证明新归档可加载并精确匹配新冻结 ID。",
    "permanentAction": "按仓库固定 Dockerfile 构建并保留新 Runner 的可信离线归档与 SHA-256；未来恢复须核对实际 ID，重建必须新计划重新验收。",
    "historicalReleases": []
  },
  {
    "fingerprint": "l3/source-checks/stale-source-observation",
    "position": "before-production-write",
    "count": 1,
    "impact": "r2 源码身份门禁拦截旧 RC15 哈希绑定，禁止该轮继续发布。",
    "recoveryEvidence": "source-identity-review/independent-review.json 和 observed-closure.json 记录全部源码差异、六平台实际闭包与独立重算；r2 原失败日志保留。",
    "permanentAction": "77cdfb4b 将独立复核后的源码与闭包身份绑定回唯一扫描范围记录；既有 first-party identity 回归及完整 L3 继续强制核验，未削弱门禁。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
