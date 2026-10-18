# KPanel v1.27.0-rc.11 发布验收记录

已验证：RC11 已作为 GitHub prerelease 和 Docker Hub preview 镜像公开；候选 CI、GitHub-hosted L3、主线 CI、Release 与公开浏览器/隔离容器验收通过。稳定通道仍为 v1.26.0，生产未部署。

日期：2026-10-18

发布级别：L3

候选提交 / 标签：78d74cefa760de2e874587352f7f9561ba988ebc / v1.27.0-rc.11

候选 tree：d6362440eab250804779fe0708cc069be3777297

上一稳定版本 / 回滚点：v1.26.0 / Docker index sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9

上一预览 / 预览回滚点：v1.27.0-rc.10 / Docker index sha256:1868915e2be7837ee087c3e4d13324b470aab3acefef3411e241b57a7028c9fe

releaseChannel：preview

releaseTrain：1.27.0

候选分支与发布后处置：release/v1.27.0-candidate / 预览版保留

- 原分支 / 精确 tip / 处置分类：release/v1.27.0-candidate / 78d74cefa760de2e874587352f7f9561ba988ebc / RC11 预览候选，RC 周期内保留。
- 归档 ref 与 SHA / 远端复核结果：预览版单次发布不归档。Annotated tag object 为 1793fc009ad3693c157f2eefa2a4a7a0759b2a1c，peeled commit 与候选一致。promotion 完成时 GitHub main 与候选分支都指向 78d74cefa760de2e874587352f7f9561ba988ebc；验收文档随后以 docs-only 提交追加到 main，不移动 RC11 tag 或候选分支。
- 本次来源任务分支：fix/gallery-scroll-priority（4a7f02f0f0d52a270763cc18ef1655424bcf1272）和 fix/gallery-thumbnail-persistence（75b476c8000b89db11cfec0bc58fbfdd4d4856c9，包含 4a7f02f 与 ab26aba496855e7ad37542829ccc5b91c20669ee）；均已纳入 RC11，原任务 worktree 按其所有权保留。此前 Gallery 虚拟时间线与 POSIX IPC ACL 已在 RC10 发布，本版没有改动 ACL。
- 本地分支/upstream/worktree：候选 worktree C:\GitHub\_release-candidates\kpanel-v127-rc1 保持干净并保留；验收记录使用独立 docs-only worktree C:\GitHub\_release-docs\kpanel-v127-rc11。原始 main worktree 未用于构建或发布。
- 未完成归档项 / 责任人 / 下次复核触发条件：无候选归档项；RC11 候选按预览规则保留，到 v1.27.0 稳定版发布时再按 project-management.md 10.2 复核处置。

归档不代表生产上线。本次产物已发布，生产未部署。

## 发布画像

- 业务域：媒体图库缩略图生成、缓存与时间线浏览。
- 变更面：展示与性能；图片解码会占用 Agent 有界 CPU/内存资源。
- 受影响用户旅程：在较大的图库中连续滚动时间线、加载缩略图、再次浏览缓存图片并打开查看器。
- 未变化契约：未修改公开 API、数据库 schema、文件 API 协议、端口、Compose、Agent 权限模型、kejilion.sh 或应用市场契约。
- 风险等级及理由：L3。改动涉及图库大量条目的渲染、跨重启缓存、原生图片解码和可选外部解码器；另需维护上一版本缓存兼容与预览回滚。

## 发布范围与未纳入内容

- 用户可见更新：
  - 连续滚动时优先保持滚动响应，延后新的媒体工作。
  - 增加有界、可跨重启复用的缩略图磁盘缓存；扩展 WebP、BMP、TIFF 原生解码，并在宿主机已有兼容 FFmpeg 时支持 HEIF/AVIF。
  - 内存缓存命中时也按有界频率更新磁盘 LRU 记录，避免活跃图片被过早淘汰。
- 精确提交清单：
  - 4a7f02f0f0d52a270763cc18ef1655424bcf1272 — perf(gallery): prioritize scrolling over thumbnail work
  - ab26aba496855e7ad37542829ccc5b91c20669ee — feat(gallery): persist thumbnails and broaden photo decoding
  - 75b476c8000b89db11cfec0bc58fbfdd4d4856c9 — fix(gallery): keep active disk thumbnails fresh on memory hits
  - 78d74cefa760de2e874587352f7f9561ba988ebc — v1.27.0-rc.11 组装与版本身份更新
- 明确未纳入的事项：没有新增 Gallery 虚拟时间线或 POSIX IPC ACL 改动；这两项已包含于 RC10 基线。没有生产部署、真实 NAS 图片库、长时间 soak 或读屏器专项验收。没有引入运行时依赖。

## 外部审计与修复交付

安全复核是针对本次 Gallery 代码范围的盲审与独立调用路径核实，不等于整仓安全审计。

- 安全盲审补漏（quality-audit review 档）：基线 v1.26.0；本轮盲审包组 internal/agent；其余待审包组 17 个，见下方 scope 输出；发现 H0/M0/L1，confirmed=0、fixed=0、deferred=1。盲审模型 gpt-6-luna / max；独立验证者与记录核验者不同于发现者。
- node scripts/security-review-scope.mjs --base v1.26.0 --head 78d74cefa760de2e874587352f7f9561ba988ebc --format json：overall=missing，35 组中 18 recorded、17 missing，invalidTrailer=true。internal/agent 已记录；缺失组为 cmd/paneld、internal/cluster/client_v2.go、internal/cluster/light_platform.go、internal/cluster/light_terminal.go、internal/cluster/protocol_v2.go、internal/cluster/service_v2.go、internal/cluster/stream_v3.go、internal/cluster/terminal_retained.go、internal/cluster/terminal_stream.go、internal/cluster/terminal_v2.go、internal/panel/panel_access.go、internal/panel/self_signed_tls.go、internal/panel/server.go、internal/panel/terminal_history.go、internal/panel/terminal_input.go、internal/panel/terminal.go、internal/terminal。invalidTrailer 是历史来源提交格式提醒；RC11 组装提交已写入有效的 internal/agent 审查 trailer。
- finding fingerprint / 修复 commit / 独立复核与回归证据：internal/agent/files.go:726，SEVERITY low / CONFIDENCE likely。同步原生 image.Decode 不响应 20 秒请求 deadline；若高成本图片由管理员在图库中查看，解码完成前可能占用唯一生成闸门并延迟后续缩略图和预览。来源字节、像素、估算解码内存和并发闸门有上限；未建立攻击者到图库图片的路径，也未验证 crafted-input exploit。登记为 needs-validation / deferred；不描述成无限 DoS 或已确认外部漏洞。本版没有因此修改代码；发布说明已披露。处理记录：C:\GitHub\_codex-evidence\security-backlog\reviews\2026-10-18-v1.27.0-rc.11-gallery\raw\internal-agent-final.md 与 coordinator-verification.md。
- 盲审提出的 handler-local auth 候选经 coordinator 追踪后拒绝：根 broker 不开放本地 HTTP listener，NewCachedFileHandler 只由 RunFileStream 经 authenticated Noise relay 使用；directory import 通过 receiveLimit(input)，另有解压字节/条目预算，因此不属于无界导入。独立验证细节保存在同一 security-backlog 证据目录。
- 修复交付状态：RC11 源码、tag、GitHub prerelease 和 Docker preview image 均指向 78d74ce；stable 未发布，生产未部署。
- OCR 观察区间 / 适用候选计数及口径 / 有效、skipped、unreported 数 / 经抽查成立的 constrained-only：OCR v1.12.11，candidate 模式，基线 e5ddd2b50af9a80adf0f2cd16a7b2b1426430adc 至 78d74cefa760de2e874587352f7f9561ba988ebc；30 文件中 25 reviewable、25/25 已审（100%），5 个预期排除项，无 reviewable 测试被排除；free-form 新 finding=0，constrained 有效 H0/M0/L0，constrained-only=0，skipped/unreported=0。结果位于 C:\GitHub\_codex-evidence\ocr-reviews\2026-10-18-v1.27.0-rc.11。
- 按 PROJECT_RULES.md 5.5 记录本稳定周期 OCR 观察结果；当前不足三个稳定周期，不评价工具有效性或退出。

## 跨仓库联动判定

- scriptLinkageState：not-required。
- 变更集编号：不适用。
- KPanel 实际内置脚本基线 commit / SHA-256：160924a498d9b8d071fed72f165d23c1b9e886de / 39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1。
- 脚本候选 commit / SHA-256：不适用。
- 状态判定依据与兼容性证据：Gallery 变更不要求跨仓库同步 kejilion.sh；package/source closure 没有引入模块、源清单或构建目标变化。
- 本版发布决定：脚本不在范围。
- 阻断或移除的依赖范围：不适用。

## 多维质量结论

| 维度 | 状态 | 证据 | 未验证风险或不适用依据 |
| --- | --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 候选 CI 与 L3 通过；本地 61 张 JPEG、2 个相册的图库交互通过；公开容器版本与预览通道核对通过。 | 本地图库是 mock，不证明真实 NAS/宿主机路径或图库样本行为。 |
| 网络入侵与供应链安全 | 已验证 | Go 调用路径、Node 依赖、源码/密钥/配置、镜像扫描及运行契约通过；OCR 无有效 finding。 | 一项低级 needs-validation 风险已延期；scope 仍有 17 个历史包组待审，invalidTrailer=true。 |
| 稳定性、失败恢复与兼容 | 已验证 | L3 Web/Go/deploy lanes、候选/主线 CI、真实隔离容器初次启动与重启、公开升级/失败态浏览器验收全部通过。 | 未在生产主机、多发行版或长时间运行环境执行；无生产回滚演练。 |
| 性能与资源预算 | 已实现未实机验证 | 本地 mock 连续滚动时立即显示 19 个轻量占位，停止后占位清空并加载当前 24/24 张图；缓存命中和视口扩展可观察。 | 样本仅 61 张；没有真实 NAS、大型用户库或正式硬件上的帧率/资源 soak。同步解码风险见审计条目。 |
| 用户体验与可访问性 | 已实现未实机验证 | 查看器关闭后焦点恢复；390×844 mock 视口无横向溢出，页面控制台无错误/警告。 | 更窄有效视口实际为 240px 时文档宽 320px，仍横向溢出；浏览器原生 200% 缩放未能触发，精确 200% 未验证；无读屏器测试。 |
| 数据、配置与迁移 | 不适用 | 不改数据库 schema、公开 API 或用户数据格式；缩略图是有界派生缓存，不含原图。 | 生产配置未操作；旧版本忽略缓存，缓存可重建。 |

## 自动门禁

- 定向测试及结果：candidate push CI 与发布编排内的候选 CI 均通过；包含图库 Web 测试、类型检查、构建、Go 检查及仓库要求的验证。未在本地另行跑测试。
- make verify-release 环境和结果：未在本地单独执行；权威 GitHub-hosted L3 的 web/go/deploy source lanes 全部通过。
- L3 外层入口 run ID、计划/脚本/bundle SHA-256、不可变 Runner ID、终态与证据目录：workflow run 39133532840；job 117693394994；candidate/tree 匹配且 identityUnchanged=true；Ubuntu 24 image 20261012.334.1，Runner ID 1000003929，Go 1.27.2、Node 24.21.0、Docker 28.0.4；开始 2026-10-18T13:54:03.608Z，结束 14:07:42.119Z，status=passed。L3 entry SHA-256 c286f7f2ad692ea17b8ec0537fe2a84e8d049d22adec63a2b00ceba39b165d57。source lanes：web 270207ms、go 524830ms、deploy 2176ms；jobs=2、GOMAXPROCS=2；receipt 与原始日志保存在 C:\GitHub\_release-evidence\v1.27.0-rc.11\published-final\l3。
- 候选 CI：push run 39132269345 对 78d74cefa760de2e874587352f7f9561ba988ebc 成功；发布编排中的 candidate-ci job 117693394979 同样成功。链接：https://github.com/kejilion/KPanel/actions/runs/39132269345。
- 主线 CI：workflow run 39133532840 的 main-ci job 117696988818 对冻结候选成功。promotion 先以 fast-forward 将 main 提升到候选，再运行主线 CI。
- Release workflow：https://github.com/kejilion/KPanel/actions/runs/39133532840，inputs、candidate CI、L3、promote、main CI、publish/release、public-acceptance 和 preview-completion 均成功。候选 Dependency freshness run 39132269322 成功；独立 security-advisories job 为 skipped。
- 发布说明门禁：draft 与最终正文检查通过；用户摘要 2 条，升级提示 3 条；预览不会自动安装，稳定通道仍为 v1.26.0，解码限制与回滚提示已发布。
- 安全扫描、镜像契约、SBOM/provenance：Go 漏洞调用路径、Node audit、源码/依赖/密钥/配置扫描、原生镜像扫描与镜像运行契约通过；已核实版本化与 preview digest 一致。未单独生成本地 SBOM 报告。

## 依赖与技术栈变化

- make dependency-report 生成时间及检测源完整性：未单独运行 make dependency-report；Dependency freshness workflow 39132269322 成功检测锁定 Node graph 与稳定依赖候选。
- 最近每日安全通告审计、EOL 复核状态及证据：未单独进行 EOL 审查；依赖新鲜度与本次 CI 漏洞扫描通过。
- 直接/基座行动项、传递依赖归属信号及首次完整检测后的启动/决策/处置期限：未报告本次新增依赖项；没有新增运行时依赖。
- 本版采用的依赖、工具链、基础镜像、Action、扫描器或受管脚本候选：产品代码不新增依赖；Go 1.27.2、Node 24.21.0、Docker 28.0.4；依赖版本与 Action 仍按仓库锁定配置。
- 版本/锁文件/Action SHA/镜像 digest/脚本提交与摘要：Go/Node 版本锁定如上；仅更新 package 与 lockfile 的 RC 版本号；受管脚本基线 160924a498d9b8d071fed72f165d23c1b9e886de，SHA-256 39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1。
- 暂缓或拒绝候选、证据、负责人、复核日期和退出条件：无依赖候选暂缓项；17 个安全复核包组按项目 review 规则留待后续分批复核，stable 发布前继续推进。
- 升级后的兼容、安全、构建、性能资源和回滚结论：无 schema/API 迁移；缓存有界且可重建；兼容与构建门禁通过；回滚可固定到 RC10 或 stable digest。存在同步解码和窄视口遗留风险，见质量与遗留风险章节。

## 隔离真机与浏览器验收

- 主机/发行版/架构/运行时版本：GitHub-hosted Ubuntu 24，runner image 20261012.334.1；L3 Runner ID 1000003929；公开验收 Runner ID 1000003933；Docker 28.0.4；镜像覆盖 linux/amd64、linux/arm64。
- 环境策略 ID 与允许用途：environment-policy.json 的 github-hosted；用途为 candidate-validation、browser-validation、failure-injection；未连接生产主机。
- 使用的精确候选或公开产物：78d74cefa760de2e874587352f7f9561ba988ebc；公开 image index sha256:ec869f3f8ed9d8f21ca2ff733f7ec4c59a886be52b2eed1b2a503bc1c7c7199b。
- 后台作业 ID、终态、退出码、超时、证据目录、命令规格路径及 SHA-256：public acceptance job 117704002811 success，Runner ID 1000003933；workflow run 39133532840 attempt 1。browser-published 45 checks、browser-upgrade 31、container initial 44、restart 26，全部 passed、errors=0。报告与截图在 C:\GitHub\_release-evidence\v1.27.0-rc.11\published-final\public-acceptance；公开 artifact zip SHA-256 405f0d4dd6097221cd949946990cbc019a42aca4dc36887787ad08839306d175。public checks 的目标 digest 与 tag 一致。
- 测试窗口/循环数及风险依据：一次发布窗口内执行；没有多小时 soak。RC 预览不证明生产长期运行。
- 受影响用户旅程、视口、100%/125%/200% 缩放、最小计算字号、主题、键盘/焦点、语言和失败态：本地 mock 61 张图、2 个相册；首屏 7 个时间线项目/9 张图已加载。快速滚动时出现 19 个占位，滚动停止约 220ms 后占位清空，扩展到 22 个项目，当前浏览窗口 24/24 张图加载。查看器关闭后焦点回到图片按钮。390×844 时 document width=380，无横向溢出，27/27 张图加载；240px 有效视口时 document width=320，仍溢出。浏览器原生 200% 缩放没有触发，故 exact 200% 不记通过。公共更新对话框另在 1280/390/320 视口检查无溢出；两处报告 page errors=0。
- 宿主机写入、失败注入、重启恢复和回滚结果：公开容器初始启动与 restart 报告通过；浏览器验证失败派发以 503 fail-closed、队列清理；仅使用隔离容器和测试 fixture，没有生产主机或用户数据写入；没有执行生产回滚。
- 未执行场景及原因：真实 NAS/宿主机图库、长期 soak、读屏器、OpenRC/真实 systemd 更新和生产安装均未执行；本次仅授权发布 preview。

## 发布产物与公开仓库复核

- 公开 Release 正文、面板发布信息接口与更新弹窗：v1.27.0-rc.11；浏览器报告验证 preview version、官方 immutable digest、非空摘要、3 条升级提示及 Release URL；published 与 upgrade 报告 errors=0。
- GitHub Release 的 draft / prerelease / Latest 状态：https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.11，published 2026-10-18T14:42:05Z，draft=false、prerelease=true；GitHub Latest 仍为 v1.26.0。
- Docker 版本与通道 OCI index：docker.io/kjlion/kejilion-panel:1.27.0-rc.11 与 :preview 均为 sha256:ec869f3f8ed9d8f21ca2ff733f7ec4c59a886be52b2eed1b2a503bc1c7c7199b。:latest 仍是 v1.26.0 digest sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9。
- linux/amd64、linux/arm64 digest：amd64 sha256:d24537ef8248e72fb785a29f5bee904880b6f5dd1d2658ce0478c951a0b2774d；arm64 sha256:0fd4bce18e0b31d1e97e64c15ba30442786df63e15fadbf55d55d4a730194e8a。
- 附件及 SHA256SUMS：GitHub Release 上传 15 个附件；SHA256SUMS 有 11 个条目，下载后 hash SHA-256 87b2a7738b024ac2176cfe8bb37e30f3de0420bb46c7e4de6fc1ad7357049ce4；包含 Agent、轻量节点、MCP 多平台二进制和元数据包。KPANEL_MODIFIED_COMPONENTS.json 已上传。
- 公开镜像 image_e2e=pass：L3 镜像契约与 public-acceptance 容器初始启动/restart 报告均通过，4 个真实隔离验收报告共 146 项检查，errors=0。
- kejilion/apps / kejilion.sh 契约结论：应用生命周期门禁通过；本版未修改 kejilion.sh，script linkage 为 not-required。

## 自更新通道验收

- 稳定来源只选择正式 GitHub Latest，预览来源只选择规范稳定版或 RC，并校验唯一官方镜像 digest：公开 browser report 确认 RC11 与 digest sha256:ec869f3f8ed9d8f21ca2ff733f7ec4c59a886be52b2eed1b2a503bc1c7c7199b 匹配。
- 加入预览只切换来源并立即检查，没有自动安装：报告 verified，无安装请求。
- 自动安装开关与一次性立即安装相互独立：自动安装保持 disabled，加入预览未触发安装。
- 旧状态默认迁移到 stable，重启后通道选择保持：隔离容器报告 checked stable version v1.26.0；restart 后配置保留。
- 退出预览且稳定版较低时没有产生降级候选：public report 确认 exit does not downgrade。
- systemd 后台执行、更新前备份、失败恢复和失败版本隔离：隔离 browser upgrade 测试覆盖 systemd 不存在时 503 fail-closed 并清队列；本版未执行真实生产 systemd 更新。
- OpenRC 与轻量 Node 的当前边界已按 docs/release-channels.md 明确呈现：本版未在生产或 OpenRC 主机验证；遵守预览不生产部署限制。

## 生产部署安全核对

- 生产目标和部署授权范围：不适用（预览版禁止生产部署）。
- 验证/灰度环境：environment-policy.json 的 github-hosted 隔离验证；未使用 arena-154。
- 正式部署环境：未使用，未连接生产主机。
- prod-108：禁用全部 KPanel 操作；本次未连接、未备份、未部署、未升级、未核对。
- 部署前版本、健康、备份位置及摘要：不适用，未执行生产部署。
- 部署命令/入口：不适用。
- 部署后版本、Panel/Agent 状态、重启、日志、数据完整性和公网入口：不适用。
- 生产已执行写操作：无。
- 仅在隔离真机执行、未在生产执行的场景：公开容器启动、重启、浏览器发布/升级、失败态与更新通道验收。

## 回滚

- 源码/tag：RC11 tag v1.27.0-rc.11 对应 78d74cefa760de2e874587352f7f9561ba988ebc；上一预览 v1.27.0-rc.10 与稳定 tag v1.26.0 均保留。
- 镜像 digest：RC11 sha256:ec869f3f8ed9d8f21ca2ff733f7ec4c59a886be52b2eed1b2a503bc1c7c7199b；RC10 sha256:1868915e2be7837ee087c3e4d13324b470aab3acefef3411e241b57a7028c9fe；stable sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9。
- 数据/配置备份：不适用；没有生产数据写入。Gallery thumbnails 是可重建派生缓存，不包含原图；无 schema migration。
- 回滚步骤和回滚后复核：预览消费者可固定到 RC10 immutable digest；必要时保持稳定通道 v1.26.0。未执行实际回滚。
- 回滚后生产实际版本与健康状态：不适用，未生产部署。
- GitHub Latest、Docker latest 与标准更新入口实际指向：GitHub Latest=v1.26.0；Docker latest digest=e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9；标准 stable 通道为 v1.26.0。
- 公共默认更新通道决策：恢复上一稳定版本作为默认 stable；preview 仍需用户显式选择。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-18T19:57:58+08:00
- 候选冻结时间：2026-10-18T21:36:13+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（未发生产品失败；未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：6
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

以下六项均发生于生产写操作之前；最近五份可用验收记录（RC10、RC9、RC7、RC6、RC5）中没有相同 fingerprint。原始失败/不完整证据及恢复过程保存在本地预览证据目录。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "local-preview/start/unsupported-option",
    "position": "before-production-write",
    "count": 1,
    "impact": "首轮本地 acceptance 预览启动误传不支持的候选基线选项，服务没有启动，也没有远端或生产写入。",
    "recoveryEvidence": "C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11/initial-attempt.md 与 C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11.start.stderr.txt 保留失败记录；随后使用受支持参数启动并核对 manifest 中候选 SHA。",
    "permanentAction": "后续启动只采用 local-feature-preview workflow 支持的参数；启动后先比对 manifest 的 branch/commit/clean，再开始浏览器验收。",
    "historicalReleases": []
  },
  {
    "fingerprint": "local-preview/mock/unsupported-image-format",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次 mock 使用 WebP 壁纸而 mock 缩略图接口只覆盖 JPEG/PNG/GIF，截图出现格式占位，不能作为 UI 验收证据。",
    "recoveryEvidence": "C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11/initial-attempt.md 记录 WebP 壁纸与 mock 缩略图格式不匹配；停止该轮后改用 JPEG fixture，并在 61 张验收样本上重新检查。",
    "permanentAction": "Gallery mock 继续使用接口已覆盖的 JPEG fixture；遇到未支持格式时不得把占位截图判为通过，先修正夹具并重做。",
    "historicalReleases": []
  },
  {
    "fingerprint": "local-preview/fixture/windows-path-separator",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次样例复制使用 Windows 路径分隔符错误，复制未形成有效素材，预览证据无效。",
    "recoveryEvidence": "C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11/attempt-2-fixture-error.md 记录错误；改用 LiteralPath/Join-Path 后重新准备 JPEG fixture。",
    "permanentAction": "跨目录复制使用 Join-Path 与 LiteralPath，并在复制前验证源/目标绝对路径和文件数。",
    "historicalReleases": []
  },
  {
    "fingerprint": "local-preview/fixture/wrong-source-directory",
    "position": "before-production-write",
    "count": 1,
    "impact": "第二次复制选择了错误的样例源目录，只复制两张后发现，素材量不足以用于验收。",
    "recoveryEvidence": "C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11/attempt-3-copy-source-error.md 记录两张素材后中止与修正。",
    "permanentAction": "复制前用 Test-Path 核对来源目录，并抽查首尾文件名与预期样例清单，再执行批量复制。",
    "historicalReleases": []
  },
  {
    "fingerprint": "local-preview/fixture/insufficient-volume",
    "position": "before-production-write",
    "count": 1,
    "impact": "五张图片的小 fixture 无法证明时间线虚拟化行为，故将该轮标为不完整，不能作为通过证据。",
    "recoveryEvidence": "C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11-jpeg/incomplete-small-fixture.md；随后使用 61 张图片完成滚动验收。",
    "permanentAction": "Gallery 虚拟时间线验收采用至少 61 张图片的固定样本量，并同时记录占位、停稳加载和当前可见图片完成数。",
    "historicalReleases": []
  },
  {
    "fingerprint": "local-preview/acceptance/browser-zoom-control",
    "position": "before-production-write",
    "count": 1,
    "impact": "当前浏览器 UI 自动化接口无法触发原生 200% 缩放，精确 200% 旅程未验证；窄视口替代检查在实际 240px 有效宽度发现 320px 文档横向溢出。",
    "recoveryEvidence": "C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11-timeline/acceptance.md 记录快捷键未能触发、有效视口实测、overflow 结果及恢复至常规视口。",
    "permanentAction": "后续图库验收先确认可用的浏览器原生缩放控制；无法触发时将该旅程标记未验证，并用窄有效视口辅助发现问题，不得将替代检查记为 200% 通过。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

## 遗留风险与后续准入

- 本地资源回收：候选 worktree 与 Gallery source worktrees 保留供 RC 周期复核；本地 mock Gallery acceptance 服务保持 ready，地址 http://127.0.0.1:4182/gallery，manifest 与停止命令见 C:/GitHub/_codex-evidence/local-previews/2026-10-18-kpanel-gallery-rc11-timeline/manifest.json 和 acceptance.md。发布证据保存在 C:\GitHub\_release-evidence\v1.27.0-rc.11\published-final；security 与 OCR 证据分别位于 security-backlog/reviews 与 ocr-reviews 下。
- 未验证风险：internal/agent 同步解码请求超时边界的真实攻击可达性；窄屏放大下的 Gallery 横向溢出；真实 NAS/宿主机图库、大型真实图库性能、长时间 soak、读屏器和 EOL 专项审查。
- 已实现待实机准入：有界持久缩略图缓存、滚动优先与解码扩展已在源代码、CI/L3 和本地 mock UI 验收；公开容器验收验证的是更新通道与镜像运行契约，不替代真实用户图库。
- 不阻断本版的理由：本版是用户显式加入的 preview；候选 CI、L3、main CI、镜像扫描、公开 Release、公开浏览器和隔离容器验收均通过；stable、Docker latest 和生产环境未改变。低级审计风险已在 Release notes 披露并保持资源有界。
- 后续应进入的自动门禁或专项工作流：Gallery 窄视口与 200% 缩放回归；持续按 quality-audit review 流程处理 17 个 missing 包组；在 stable 候选前重新评估同步原生解码的超时影响与真实 Gallery/目标主机兼容。
