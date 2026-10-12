# KPanel v1.26.0-rc.13 发布验收记录

发布级别：L3。`releaseChannel=preview`，`releaseTrain=1.26.0`。

批准 main 基线 `3f55a961cac40f44959b51f5ca4abbaba1e42f7b`；冻结产品提交 `63dc3afe50fc9354446cc0af490e475409700e6e`。已创建注释标签 v1.26.0-rc.13，标签对象 1cbd840c4ea8a1fef398f453cab4717034213ee9 指向冻结产品提交 63dc3afe50fc9354446cc0af490e475409700e6e。发布时 main 与 release/v1.26.0-candidate 均在 63dc3afe。

上一公开预览 / 回滚点：`v1.26.0-rc.12` / `a6b8d2aac975e4708d216b989d8714f92f871083` / `sha256:57de86879881985d166bbc68165d2a669bf709458f496c06c305709e949d5643`。稳定版 `v1.25.1`、Docker `latest` `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。

## 发布画像与来源

本轮发布终端展示和选择交互、图库媒体调度和手势；没有新增服务端 API、宿主权限、数据库 schema、依赖、Agent/Node 协议或安装配置。展示用命令不替代执行前的文件版本、类型、主机、路径与权限检查；主机选择不自动执行批量任务。图库缓存只在浏览器内短期保留，每次导航继续向主机核验。

| 原始来源 | 精确 tip | 纳入内容 |
| --- | --- | --- |
| `feature/files-terminal-execute` | `1dca43abc7b552ee72df9cb90f9bf411819aa9f2` | 文件执行先显示简短命令；有界启动回显过滤，保留后续原始字节；处理分段提示符 |
| `feature/terminal-batch-selection` | `d4722419f707a71019256a76bc1d78808874d624` | 行选择、复选框、Shift 范围、Ctrl/Command 增减和列表全选快捷键；禁用主机与执行中保护 |
| `fix/terminal-scrollbar-wallpaper` | `9846d2dd63a62018925c1ae4fa68e87b5751fc51` | 主机/应用终端共享原生轨道，透明背景与缓冲区同步，释放监听 |
| `fix/gallery-media-performance` | `3f9f96930ff4e142f845652c2af3e998b839135b` | 图片 2 路、海报 1 路调度；有界重试/大小/缓存，导航快照与同版本图片稳定显示 |
| `fix/gallery-shift-selection` | `c4d437a8a307d32032a97b29ffdcfb9eac77106e` | 从点击快照重建范围，支持收缩、撤回和连续取消 |
| `fix/gallery-touch-gestures` | `0271f88a43a4e6cd9d202ee3d07f450022f0b8af` | 长按范围、原生纵向滚动、边缘自动滚动、误点抑制、双指缩放与横滑反馈 |

图库后三条是递进祖先，只合并最终组合；所有六个 tip 均为产品祖先，保留原始历史。发布工作树基于批准 main 组装；不声称作者各分支统一 rebase。RC12 五组修复和 systemd 服务管理继承，旧已归档快捷方式图标/分页/文件布局未重新引入。旧 login 审计原件及未完成的 backup 审计不混入本版；已 patch-equivalent 的旧功能不重复当新候选。

## 多维质量与边界

| 维度 | 状态 | 证据及限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已实现未实机验证 | 整合 Mock、自动门禁；没有新 Linux PTY/真实 Docker 执行旅程 |
| 网络入侵与供应链安全 | 已实现未实机验证 | 精确源身份、登记扫描政策；全局 CF 风险仍开放 |
| 稳定性、失败恢复与兼容 | 已实现未实机验证 | 有界回显回退、取消及媒体释放、安装恢复夹具；无生产恢复演练 |
| 性能与资源预算 | 已实现未实机验证 | 2,000 项浏览器 Blob 释放实测；没有最终组合 WAN/真实宿主性能准入结论 |
| 用户体验与可访问性 | 已实现未实机验证 | 桌面/390px、两主题、CSS zoom 与手势；实体手机、iOS Safari、读屏未验证 |
| 数据、配置与迁移 | 已实现未实机验证 | 无 schema 迁移，短期浏览器缓存；真实宿主更新未新增验收 |

### 安全审计与源码身份

本版未运行新的 CF scoped/full 审计。冻结覆盖检查为 `scoped-required`，88 待审提交，最早 8 天、上次完整审计 21 天；这是覆盖状态，不是补审通过。既有 BT/offers/终端与 run-34 systemd 风险、主机组成员/token ACL 条件验证继续开放；稳定提升前补齐所需审计。预览不关闭生产 `Service.Close`、真实服务写入或已有性能限制。

first-party 901 个 Go 文件 SHA-256 `093e3efcfe092cccdfe19a28d8cb70b7801a4ec835b9e646ef986e0f0d932e13`；closure `7d8cb6312aab83626856dbfc60ba4d2728d71ab6a0d46fd025680f913a2965d3`。Go 差异仅 `internal/version/version.go` 的版本元数据，文件名/import/build tags、依赖声明与构建目标未变。完整 L3 仍重新计算真实/原版闭包，不以人工复用说明代替扫描。

OCR 1.12.11，`3f55a961..0dedcb1a`，42 文件 / 39 reviewable / 3 排除，39/39 直接审阅；自由臂先于约束臂落盘，最终 H0/M0/L0。先前已接触来源 OCR，记录 `blind=false`、`constrained-only=unreported`。初看怀疑封面取消后缓存写回，读到 `readCoverMarker` 的 signal 检查及 catch 重抛后判为不成立，原判断与排除理由保留；没有为此修改代码。

### 跨仓库联动

`scriptLinkageState=not-required`（无需发布脚本，不适用）。内置脚本 `1800d955f216aebd2776674368a8e469a671c489` / SHA-256 `5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c`；本轮不改协议、动作或镜像内脚本。apps SSH fetch main `6602ac36e0c329e35b74ff621721b23583e94f5f`，公开及候选 `kpanel.conf` blob 同为 `9d2d2efdc334db1ba014203370fdc8281f1a5870`。无 apps/sh 写入。

## 自动门禁

固定 Runner 的 L3 验证在 arena-154 通过，候选精确为 63dc3afe50fc9354446cc0af490e475409700e6e；release L3 run-id 为 v1.26.0-rc.13-63dc3af-l3-r2，Runner 镜像 kpanel-go127-security-runner:go1.27.2-node24.21.0，镜像摘要 sha256:20667a9f1fad6590219ba8d8acdabc09a49f8683d4a877b847df17d28b83f8fb。
运行时间 2026-10-12T15:33:03Z–2026-10-12T15:56:41Z，exit_code=0，release_l3_gate=pass、release_l3_remote=pass；source-checks web/go/deploy lanes 均通过。原始日志 SHA-256 619d67dbc13421c002f0c96b346e42b01376bf512951f70b254e9f43f38696a8。
首次 r1 仅因外部 evidence-dir 已存在而被预检拒绝，候选代码未执行；保留 l3-r1-orchestrator.log。随后分配唯一 r2 路径与 run-id 完成验证，status.txt 及完整日志保留。

冻结提交 63dc3afe 在候选分支 CI [run 38274992262](https://github.com/kejilion/KPanel/actions/runs/38274992262) 与依赖新鲜度 [run 38274993310](https://github.com/kejilion/KPanel/actions/runs/38274993310) 通过。
同一 SHA 快进 main 后，main CI [run 38276035763](https://github.com/kejilion/KPanel/actions/runs/38276035763) 与依赖新鲜度 [run 38276035697](https://github.com/kejilion/KPanel/actions/runs/38276035697) 通过。
标签发布工作流 [run 38277465690](https://github.com/kejilion/KPanel/actions/runs/38277465690) 的来源扫描、漏洞与依赖审计、源码安全扫描、安装生命周期、二进制构建、本机镜像契约、多架构镜像、来源证明、最终发布说明和 Release 发布均成功。

首次 L3 准备因传入已存在的外部证据根目录被入口关闭失败；候选代码未执行，原 `l3-r1-orchestrator.log` 保留。随后使用独立新路径与 run ID `v1.26.0-rc.13-63dc3af-l3-r2`，不是覆盖重写首轮成功。

发布说明在固定 Runner 通过 `TestRenderedReleaseNotesForPublication`，2 条新增、3 条修复、1 条升级提示。最终正文以公开 Release 读回为准。

## 界面与资源验收

整合 acceptance Mock `http://127.0.0.1:4182`，manifest 绑定冻结产品，`visual-composition`；模拟脚本不执行真实宿主命令。停止命令为 `node scripts/local-feature-preview.mjs stop --evidence-dir C:/GitHub/_release-evidence/v1.26.0-rc.13/preview`，当前发布任务负责，在用户验收结束或移交前保留。

- 文件执行 6 个浏览器场景通过：先显示命令后发送、17 字节分段回显边界/中文日志、手动输入、焦点恢复、窄屏 Docker 命令与失败提示；不是 Linux PTY 或真实 Docker 构建证明。
- 终端多选的实际点击、Shift、Ctrl 和全选快捷键通过，禁用主机不选中。脚本终端与主机终端的原生滚动位置/缓冲同步、透明轨道及隐藏重复 xterm 轨道通过。1280px dark/1、light/1.25、light/2 和 640px dark/1；这里 1.25/2 是 CSS zoom，不冒充浏览器原生缩放或操作系统缩放。没有新增跨浏览器原生箭头/拖动全矩阵验收。
- 图库 390×844 触屏模拟：长按、范围扩展/缩回/取消、纵向原生滚动、边缘滚动和停止、双指 scale(2)、松开不误翻页、横滑跟手、视频原生控件和空相册通过；无页面异常。操作文字 14px、可见最小 12px。
- 桌面窗口的视频海报、实际 Mock 媒体播放（640×360、currentTime>0.8、paused=false）、空相册和 800px 窗口/键盘焦点通过。
- 2,000 项浏览器压力夹具从 480 逐步显示至 2,000 项，滚动样本 live Blob 为 22/13/7/9/5/5/5/5；离开页面 created=244、revoked=244、live=0。记录 originals=4、thumbs=289、viewerOriginals=1，不将该次数或样例体积外推为 WAN 吞吐结论。
- 查看了实际截图；首张缩放截图截在淡入阶段，保留原件并另跑带 350ms 完成等待的确认捕获，未改产品动画。截图清单与查看记录在外部证据，未将 Mock v0.16.0 fixture 徽标冒充实际发行版本。
- 未验证实体手机、iOS Safari/Firefox、读屏、原生 ARM、实际 OS 媒体偏好、生产更新/回滚及长时 soak。既有 GPU/WAN 和生产生命周期风险保留。

## 公开产物与镜像

GitHub Release [v1.26.0-rc.13](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.13) 于 2026-10-12T16:39:53Z 发布，prerelease=true、draft=false。
DockerHub 版本标签与 preview 的 OCI index digest 相同：sha256:25ba652bc2f75e6292d405ff37c645a4092268c6ab48b6ed042ce10cf0e6dadc；稳定 latest 仍为 v1.25.1 / sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325。
linux/amd64 manifest sha256:6c7f2683df4aa66213ba64f0e8a5088ab1cdce8ad4f262d7d1c8798f15c91746，linux/arm64 manifest sha256:2527e45a1a49966bbf5cbdc480437a73a547555f97806e6c18c8c0305eb385c3；两者 OCI version/revision 标签均为 1.26.0-rc.13 / 63dc3afe50fc9354446cc0af490e475409700e6e。
15 个 Release 附件均逐个通过 GitHub digest 对比；其中 SHA256SUMS 所列 11 个文件也全部匹配。公开镜像 E2E 在 arena-154 于 2026-10-12T16:42:11Z–16:42:32Z 运行，exit_code=0、image_e2e=pass；退出后临时容器和网络均为 0，18080 端口空闲。
本机未能直连 Docker Registry；最终 Registry index/header 与两个架构标签在 arena-154 复核，原始公开证明与 15 项附件摘要记录于公开核验回执。

## 自更新与生产适用性

stable/preview 来源选择、预览只切检查、退出不自动降级、恢复契约沿用。没有新增 live 更新弹窗、真实 Provider 或自动安装旅程；公开正文与 Mock 不替代实机证据。

用户授权发布预览产物及所需 SSH 分支/main/tag/Release 写入；生产未部署，未连接禁用的 prod-108/108。没有生产备份恢复点或生产回滚演练。需要回退公开预览时选择上述 RC12 不可变 tag/digest；用户环境部署或恢复另行授权。

## 分支与资源处置

六个来源 tip 均已纳入冻结产品历史；见上表的文件终端执行、终端多选/滚动条、图库资源预算、Shift 范围和触屏手势。其作者分支、工作树与活跃预览继续保留，因为所有权尚未释放；本轮不删除或改写作者分支。
release/v1.26.0-candidate 与 main 当前同为 63dc3afe。Release 工作流的 Archive published candidate branch 仅适用于稳定版，本次预览跳过该步骤，候选线保留给后续 RC。
本轮未纳入 offers 新改动、旧 login 审计原件或未完成的 backup 审计；旧桌面快捷方式图标、分页与文件页布局保持已归档状态。生产未部署。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：未记录
- 候选冻结时间：2026-10-12T23:30:37+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：3
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "release-l3/preflight/existing-evidence-root",
    "position": "before-production-write",
    "count": 1,
    "impact": "首轮 L3 入口因目标外部证据目录已存在而拒绝执行；候选代码没有运行，未发生验证机写入。",
    "recoveryEvidence": "l3-r1-orchestrator.log；独立 run-id r2 的 status.txt、l3-verify-release.log 与 SHA-256 清单。",
    "permanentAction": "保留拒绝覆盖已有证据目录的保护；每次分配唯一 run-id 与 artifact-dir。RC13 已按该规则在 r2 完成并通过。",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-verifier/windows-powershell/runtime-compatibility",
    "position": "before-production-write",
    "count": 1,
    "impact": "桌面无法直连 Docker Registry，且 Windows PowerShell 的 Invoke-WebRequest 和 .NET API 兼容性使本机公开核验中止；Release 本身已成功发布，公开镜像随后在 arena-154 验证。",
    "recoveryEvidence": "public-verifier-attempt-1.log 至 attempt-4.log；public-registry-verification.log、public-registry-proof.json、public-asset-checksums.json 与最终核验报告。",
    "permanentAction": "后续公开镜像摘要和平台标签继续在 arena-154 取证；附件下载使用已验证兼容的 HttpClient/BitConverter 路径，先归档 Registry 与 PowerShell 兼容性检查结果。",
    "historicalReleases": []
  },
  {
    "fingerprint": "docs-writer-check/missing-local-go/go-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "本机 Git Bash 未找到 go，导致三个 Go 依赖的 release-channel 合约检查无法启动；错误是工具缺失（go: command not found / spawnSync go ENOENT），没有代码断言失败。",
    "recoveryEvidence": "C:/GitHub/_release-evidence/v1.26.0-rc.13/writer-check-release-channel-attempt.log；同一文档提交的 GitHub candidate/main CI 作为恢复验证，run ID 记录在外部交接回执。",
    "permanentAction": "本地 writer check 需配置可用的 Go 路径；以同一提交在配置 setup-go 的 GitHub candidate/main CI 结果完成验证，不把工具缺失记为通过。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

L3 首轮证据目录拦截、本地公开核验器兼容问题与文档 writer check 缺少本机 Go 工具按三个同源批次计数，原始失败证据均保留；均发生在生产写入前。第三项是环境依赖缺失，需由同一文档提交的 candidate/main CI 确认 Go 相关门禁；生产指标不把预览门禁失败算成生产退化。

## 证据、限制与后续准入

证据根目录 `C:/GitHub/_release-evidence/v1.26.0-rc.13`，包含契约、来源处置、OCR、源身份、L3、实际浏览器报告/截图、公开下载摘要及镜像 E2E。验收文档提交自身的候选/main CI 与归档状态写入外部交接回执，不改变本次冻结产品提交与预览标签。复用既有工作流，未新增知识或工作流。稳定提升前完成 CF 与既有真实主机/性能准入。
