# KPanel v1.27.0-rc.12 发布验收记录

已验证：RC12 已作为 GitHub prerelease 和 Docker Hub preview 镜像公开。精确候选的 push CI、L3、候选 CI、主线快进与主线 CI、Release、公开镜像核验及浏览器/隔离容器验收通过。首次公开验收遇到 GitHub API 403，按冻结 snapshot 仅重跑该验收 job 后通过；没有重发 Release 或镜像。GitHub Latest、Docker latest 和生产环境未改变。

日期：2026-10-19

发布级别：L3

候选提交 / 标签：3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15 / v1.27.0-rc.12

候选 tree：96630f4075509631c29b72f15b2dfb6933ceb79b

上一稳定版本 / 回滚点：v1.26.0 / Docker index sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9

上一预览 / 预览回滚点：v1.27.0-rc.11 / Docker index sha256:ec869f3f8ed9d8f21ca2ff733f7ec4c59a886be52b2eed1b2a503bc1c7c7199b

releaseChannel：preview

releaseTrain：1.27.0

候选分支与发布后处置：release/v1.27.0-candidate / RC 周期保留。发布时 GitHub main、release 候选分支和 v1.27.0-rc.12 peeled tag commit 均为 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15；annotated tag object 为 d4fc89d35b6b04d8483b26cf1d1fdedd0ad885d3。

- 原分支 / 精确 tip / 处置分类：release/v1.27.0-candidate / 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15 / 已发布的 RC12 预览候选，RC 周期继续保留，不归档。
- 归档 ref 与 SHA / 远端复核结果：预览序列单次发布不归档。远端 main 与 release/v1.27.0-candidate 均核实指向 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15；tag ref 为 d4fc89d35b6b04d8483b26cf1d1fdedd0ad885d3，peeled commit 与候选一致。
- 本次来源任务分支：
  - feature/desktop-xp-theme / a779ffa76abfb07f42f38376122ef06d70ba47b9：Windows XP 外观及后续修饰纳入 RC12（候选中的 XP 提交从 d7bdb4cedf91850d67f85744fb3ff7e18b786a66 起）。本地 worktree 有 22 条 git status 变更，原样保留，未发布未提交内容。
  - feature/desktop-win98-theme / 02e6dbfd50943e56a97b91e61c3da7eec2610bb1：Windows 98 外观纳入 RC12（候选提交 ad08ff2e008e0b7b9816d9247a3c7d6aba1f07ed）。
  - codex/admin-avatar / f58dc6f07ad4c24f677229a408a1397af4e5141e：管理员头像上传纳入 RC12（候选提交 bf66cd3831289c3ea1ee2beb7ffaab88160d4012）。仅供本地 mock 使用的辅助改动已在候选中由 a2f50bbfb5cb7d01aa4b4d1b7b0e1fc777df44c6 撤回，不属于最终产物。
  - feature/light-node-self-signed-cert / 440f91d500d87534764ddd096f37163e048f2612：轻量节点固定 TLS 公钥指纹连接纳入 RC12（候选提交 04e5c3dca0aa566f007f18f2c462ef58e68ded3d）。
  - fix/gallery-loading-feedback / 39dc583e50da34ac2fc92a3ef8884ad5c8a16db9：图库轻量缩略图加载反馈纳入 RC12（候选提交 908673a9c11c75f09dbac3b9a676c34baa148445）。
  五个来源分支均没有对应的 GitHub 远端 heads ref；它们的候选代码进入 RC12，原任务 worktree 保留。四个 worktree 当时 clean；XP worktree 的本地改动保持未处置。因没有原任务所有者释放声明，不创建 archive ref，也不删除本地分支或 worktree。
- 本地分支/upstream/worktree：候选 worktree C:\GitHub\_release-candidates\kpanel-v127-rc1 保持 clean；本验收记录在独立 worktree C:\GitHub\_release-docs\kpanel-v127-rc12 编写。原始 main worktree 未用于发布。
- 未完成归档项 / 责任人 / 下次复核触发条件：五个来源任务 worktree 按原所有权保留；责任人为各原任务所有者。作者释放工作区时或 v1.27.0 稳定发布收尾时再复核归档条件。RC 候选本身按预览规则保留到稳定版发布或该序列获明确废弃处置授权。

归档不代表生产上线。本次产物已发布，生产未部署。

## 发布画像

- 业务域：桌面外观、管理员账号头像、轻量节点 TLS 配对、图库缩略图反馈。
- 变更面：展示与设置、管理员账号信息/备份、节点连接信任配置；未修改部署编排或宿主机资源。
- 受影响用户旅程：切换 XP/Win98 桌面外观；上传并备份管理员头像；为使用自签名 HTTPS 的轻量节点固定 TLS 公钥指纹；查看图库缩略图加载状态。
- 未变化契约：未改变数据库 schema、端口、Compose、Agent 权限模型、kejilion.sh 或应用市场契约。新增头像管理 API 和账号备份字段；轻量节点继续校验证书主机名、有效期与服务器用途。
- 风险等级及理由：L3。包含新增账号备份字段和节点 TLS 信任路径。自签名 TLS 监听器的并发资源上限尚未补齐，按中风险仅限可信网络预览，并已在公开说明中披露。

## 发布范围与未纳入内容

- 用户可见更新：
  - 桌面可切换 Windows XP 与 Windows 98 风格外观。
  - 设置页可上传 PNG、JPEG 或 WebP 管理员头像；文件最大 512 KiB，宽高各不超过 2048 像素，头像显示在侧边栏与开始菜单并进入账号备份。
  - 轻量节点可通过固定 TLS 公钥指纹连接内置自签名 HTTPS 面板；信任 CA 连接仍按系统证书校验。
  - 图库恢复轻量缩略图淡入反馈，同时保留滚动期间的媒体工作优先级。
- 精确候选与功能提交：Windows XP 外观及修饰从 d7bdb4cedf91850d67f85744fb3ff7e18b786a66 起，Win98 外观 ad08ff2e008e0b7b9816d9247a3c7d6aba1f07ed，头像 bf66cd3831289c3ea1ee2beb7ffaab88160d4012，自签名 TLS 04e5c3dca0aa566f007f18f2c462ef58e68ded3d，图库反馈 908673a9c11c75f09dbac3b9a676c34baa148445，RC12 身份组装 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15。
- 明确未纳入的分支、文件或后续事项：XP 来源 worktree 中 22 条本地状态变更未纳入冻结候选；RC11 不支持 XP 壁纸标识与头像字段，回退注意事项已列入 Release。Gallery 虚拟时间线 14a7f182 与 POSIX IPC ACL 7e3c0e89 是 RC10 已公开内容，不是本轮新提交；RC12 未另行改动 ACL。未做生产部署、真实 NAS 图库、长时间 soak 或读屏器专项验证。

## 外部审计与修复交付

本节记录候选范围审查和当前证据边界；范围覆盖不等于全仓安全结论。

- 安全盲审补漏（quality-audit review 档）：本轮 RC12 来源候选完成差异审阅；未发现新增已确认漏洞，free-form findings=0，constrained high/medium/low=0。61 个 reviewable 文件均有审阅记录。仍有 15 个历史安全包组缺少 review 记录，见下项。
- node scripts/security-review-scope.mjs --base v1.26.0 --head 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15 --format json：overall=missing，38 组中 23 recorded、15 missing，invalidTrailer=true。新增 internal/panel/self_signed_tls.go 与本轮变更范围已有 review 记录；不能据此宣称整仓审计完整。
- OCR：open-code-review 1.12.11 delegation mode；差异 d35e0e0e70d913e1545741b298322a2de32785e0..006ff513255d9b74bed80021afabd2fbfe91d40b；66 个文件、61 个 reviewable、5 个排除、61/61 已审。free-form 新 finding=0；constrained H0/M0/L0；constrained-only=unreported。RC12 后续 Go 格式对齐与版本身份变更另有范围记录，六平台闭包未变。
- finding fingerprint / 修复 commit / 独立复核与回归证据：已知中风险 light-node-self-signed-tls-accept-loop-resource-cap，状态 confirmed-deferred，owner=KPanel release maintainers，backlog #40，目标日期 2026-11-01。现有接受循环并发资源上限待补；Release notes 将预览限定于可信网络。该项不是新确认的外部攻击利用路径。
- 另一个 medium store/backup 审计条目来自 XP 审阅，位于本次桌面差异之外，未由本次 RC12 修复；不得以本版发布关闭该事项。
- 修复交付状态：本轮五项已纳入 RC12 源码、GitHub prerelease 和 Docker preview；未提升稳定版，未部署生产。
- OCR 观察区间 / 适用候选计数及口径 / 有效、skipped、unreported 数 / 经抽查成立的 constrained-only：本次候选记录 66 个差异文件、61 个 reviewable 均完成、5 个明确排除；free-form=0 finding，constrained=0 H/M/L，constrained-only=unreported。此单次候选结果不代表长期工具效能。
- 按 PROJECT_RULES.md 5.5 记录本稳定周期 OCR 观察结果；当前不足三个稳定周期，不评价工具有效性或退出。

## 跨仓库联动判定

- scriptLinkageState：not-required。
- 变更集编号：不适用。
- KPanel 实际内置脚本基线 commit / SHA-256：160924a498d9b8d071fed72f165d23c1b9e886de / 39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1。
- 脚本候选 commit / SHA-256：不适用。
- 状态判定依据与兼容性证据：本版不修改 kejilion.sh 或应用市场集成。
- 本版发布决定：脚本不在范围。
- 阻断或移除的依赖范围：不适用；未新增运行时依赖。

## 多维质量结论

| 维度 | 状态 | 证据 | 未验证风险或不适用依据 |
| --- | --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 候选 CI 与 L3 通过；头像、桌面主题、轻量节点 TLS 及图库改动的候选自动化检查通过；公开预览版本和摘要一致。 | 未在真实用户账户、真实轻量节点或生产图库进行端到端业务验收。 |
| 网络入侵与供应链安全 | 已验证 | 候选范围 OCR 覆盖 61/61 reviewable；L3 安全与构建 lanes 通过；发布产物摘要与双架构身份通过复核。 | 全仓 security-review-scope 仍为 missing，15 个历史包组待补；自签名 TLS 接受循环资源上限为已披露中风险，不能视为安全关闭。 |
| 稳定性、失败恢复与兼容 | 已验证 | 候选/主线 CI、L3、公开容器初次启动和重启、更新与失败态浏览器验收通过。 | 未在生产、多发行版或真实 OpenRC/systemd 主机验证；RC11 不理解新增头像/XP 壁纸字段，回退按 Release notes 操作。 |
| 性能与资源预算 | 已实现未实机验证 | CI/L3 覆盖代码构建与自动检查；轻量缩略图反馈维持滚动优先级。 | 没有低配宿主机、真实大型图库、低速网络节点或长期 soak 的 P95/RSS 数据；自签名监听器资源 cap 待补。 |
| 用户体验与可访问性 | 已实现未实机验证 | 公开更新弹窗在 1280/390/320 宽度检查通过；公开页面报告 page errors=0。 | 该公开浏览器套件未单独验证图库新反馈、两种桌面主题及实体头像上传旅程；无读屏器或实体设备专项。 |
| 数据、配置与迁移 | 已验证 | 头像随账号备份；预览升级提示说明与 RC11 回退兼容边界；没有数据库 schema 迁移。 | 账号备份新增头像字段；回退到 RC11 前须调整 XP 壁纸，头像字段不识别时恢复默认图标。 |

## 自动门禁

- 定向测试及结果：候选 push CI run 39153842686 对精确 SHA 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15 成功；发布编排内 candidate-ci 同样成功。
- make verify-release 环境和结果：未在本地单独运行。GitHub-hosted L3 对冻结候选的 web、go、deploy lanes 全部通过。
- L3 外层入口 run ID、计划/脚本/bundle SHA-256、不可变 Runner ID、终态与证据目录：run 39154377686，L3 job 117754492639，attempt 1，Runner ID 1000003948；candidate=3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15、tree=96630f4075509631c29b72f15b2dfb6933ceb79b，identityUnchanged=true。Ubuntu 24 image 20261012.334.1；Go 1.27.2、Node 24.21.0、Docker 28.0.4；开始 2026-10-18T18:30:07Z，结束 18:42:50Z，status=passed。entry SHA-256 c286f7f2ad692ea17b8ec0537fe2a84e8d049d22adec63a2b00ceba39b165d57；source lanes web/go/deploy 均通过。报告与日志保存在 C:\GitHub\_release-evidence\v1.27.0-rc.12\publication-r1\l3。
- 候选 CI：push run 39153842686 对冻结提交成功；发布 workflow candidate-ci job 117754492796 成功。链接：https://github.com/kejilion/KPanel/actions/runs/39153842686。
- 主线 CI：workflow run 39154377686 的 promote job 117757534367 快进 main；main-ci job 117757576504 对同一冻结候选成功。
- Release workflow：https://github.com/kejilion/KPanel/actions/runs/39154377686，attempt 2 status=completed、conclusion=success；inputs、candidate CI、L3、promote、main CI、publish/release、public-acceptance 和 preview-completion 均成功。
- 首次公开验收：attempt 1 的 public-acceptance job 117764571250 因 GitHub REST 对 annotated tag object API 返回 HTTP 403 失败，导致 preview-completion 未执行。已保留原失败 job、snapshot-39154377686-1 与 publication-39154377686-1；对同一 frozen publication snapshot 只重跑 public-acceptance，attempt 2 job 117765100216 成功，没有重发 Release、附件或镜像。完整失败指标与恢复记录见“流程异常明细”。
- 发布说明门禁：RC12 Changelog/Release 正文检查通过；公开正文包含桌面主题、管理员头像、轻量节点自签名 TLS、图库反馈和可信网络风险提示；升级注意事项 3 条，明确加入预览不自动安装、退出不自动降级及 RC11 回退边界。
- 安全扫描、镜像契约、SBOM/provenance：Release job 与 L3 source lanes 成功，公开验收验证官方镜像版本、immutable digest、双架构和附件校验；单独的 security-advisories job 未作为本轮通过项，不据此声明额外扫描通过。

## 依赖与技术栈变化

- make dependency-report 生成时间及检测源完整性：未单独运行 make dependency-report；本候选未新增运行时依赖。
- 最近每日安全通告审计、EOL 复核状态及证据：未单独完成 EOL 专项复核；记录本次 CI 与 Release 已执行的漏洞/依赖门禁，不扩展为全依赖生命周期结论。
- 直接/基座行动项、传递依赖归属信号及首次完整检测后的启动/决策/处置期限：没有新增运行时依赖候选或待处理升级项。
- 本版采用的依赖、工具链、基础镜像、Action、扫描器或受管脚本候选：Go 1.27.2、Node 24.21.0、Docker 28.0.4；产品运行时依赖未新增；受管脚本基线保持不变。
- 版本/锁文件/Action SHA/镜像 digest/脚本提交与摘要：版本元数据更新至 1.27.0-rc.12；受管脚本 160924a498d9b8d071fed72f165d23c1b9e886de / SHA-256 39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1；preview index sha256:d2c2eac666f3dff33e5b4ae369c5a805f7de5d07a591b4a244dcba81da5130e2。
- 暂缓或拒绝候选、证据、负责人、复核日期和退出条件：自签名 TLS 监听器资源 cap medium，负责人 KPanel release maintainers，deadline 2026-11-01，backlog #40；完成并复核接收循环并发资源上限后退出可信网络限制。
- 升级后的兼容、安全、构建、性能资源和回滚结论：候选构建、CI 与公开产物门禁通过；无数据库 schema migration；账号备份字段需按 Release notes 提醒回退；Gallery 真实大库资源表现及 TLS 长时间连接预算未实机测量。

## 隔离真机与浏览器验收

- 主机/发行版/架构/运行时版本：GitHub-hosted Ubuntu 24 image 20261012.334.1；L3 Runner ID 1000003948；公开验收 Runner ID 1000003954；Docker 28.0.4；OCI 覆盖 linux/amd64 与 linux/arm64。
- 环境策略 ID 与允许用途：environment-policy.json 的 github-hosted；用途为 candidate-validation、browser-validation、failure-injection；未连接生产主机。
- 使用的精确候选或公开产物：候选 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15；公开 image index sha256:d2c2eac666f3dff33e5b4ae369c5a805f7de5d07a591b4a244dcba81da5130e2。
- 后台作业 ID、终态、退出码、超时、证据目录、命令规格路径及 SHA-256：workflow run 39154377686 attempt 2，public-acceptance job 117765100216 success，Runner ID 1000003954。browser-published 45 checks、browser-upgrade 31、container initial 44、restart 26，合计 146 checks，均 passed、errors=0。证据目录 C:\GitHub\_release-evidence\v1.27.0-rc.12\public-r2；成功 artifact public-39154377686-2，2,884,338 bytes；首次失败 artifact public-39154377686-1 与原始 snapshot 另行保留。
- 测试窗口/循环数及风险依据：公开验收在一次发布窗口完成，无多小时 soak；隔离容器和浏览器报告不代表真实用户主机安装。
- 受影响用户旅程、视口、100%/125%/200% 缩放、最小计算字号、主题、键盘/焦点、语言和失败态：公开报告核对 preview 版本、immutable digest、Release URL、升级摘要、更新状态及失败态；更新对话框 1280/390/320 宽度无溢出，报告 page errors=0。公开套件没有单独执行新 Gallery、XP/Win98 外观或真实头像上传视觉旅程；原生 200% 缩放、读屏和实体手机未验证。
- 宿主机写入、失败注入、重启恢复和回滚结果：容器初次启动 44 项、restart 26 项通过；browser-upgrade 31 项覆盖预览更新与失败关闭路径。目标为隔离测试容器，未写入生产主机或用户数据，也未执行生产回滚。
- 未执行场景及原因：真实 NAS/宿主机图库、大型图库 soak、真实自签名轻量节点配对、生产更新、读屏器、OpenRC 和原生 systemd 主机更新均未执行；本次授权范围为 preview。

## 发布产物与公开仓库复核

- 公开 Release 正文、面板发布信息接口与更新弹窗：均显示 v1.27.0-rc.12、preview channel、官方 immutable digest 与 Release URL；published/upgrade 报告 errors=0。
- GitHub Release 的 draft / prerelease / Latest 状态：https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.12，published 2026-10-18T19:13:36Z，draft=false、prerelease=true；GitHub Latest endpoint 仍为 v1.26.0。
- Docker 版本与通道 OCI index：docker.io/kjlion/kejilion-panel:1.27.0-rc.12 与 :preview 均核对为 sha256:d2c2eac666f3dff33e5b4ae369c5a805f7de5d07a591b4a244dcba81da5130e2；:latest 仍指向 stable v1.26.0，digest sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9。
- linux/amd64、linux/arm64 digest：amd64 sha256:66a1edad07594c23d6faf993260b81408034e99c06dbf852c21b2072af55f44c；arm64 sha256:b28e15381fbcfad088b6bdf8cc038a4d3a3724227c76e0965f1fb607538546e3。
- 附件及 SHA256SUMS：GitHub Release 有 15 个附件，SHA256SUMS 有 11 项；已下载复核，文件 SHA-256 为 1cd5fbd3343b508f1dbf094881952b307d801aef496ab6018d7f5b6c2262ece4。包含 Agent、轻量节点、MCP 多平台二进制、部署元数据、KPANEL_MODIFIED_COMPONENTS.json、LICENSE 与第三方声明。
- 公开镜像 image_e2e=pass：L3 镜像契约与公开初次启动/restart 报告通过；公开四份验收报告共 146 项检查、errors=0。已验证版本标签与预览摘要一致。
- kejilion/apps / kejilion.sh 契约结论：本版没有跨仓库脚本或应用市场变更；scriptLinkageState=not-required。

## 自更新通道验收

- 稳定来源只选择正式 GitHub Latest，预览来源选择规范稳定版或 RC，并校验唯一官方镜像 digest：public-release.json 确认 RC12 与 index sha256:d2c2eac666f3dff33e5b4ae369c5a805f7de5d07a591b4a244dcba81da5130e2 匹配；stable-before.json 确认 v1.26.0 digest 未变。
- 加入预览只切换来源并立即检查，没有自动安装：公开升级报告核验通过；无安装请求。
- 自动安装开关与一次性立即安装相互独立：验收未启用自动安装，加入预览没有触发安装。
- 旧状态默认迁移到 stable，重启后通道选择保持：隔离容器仍检查 v1.26.0 稳定来源，restart 后配置保留。
- 退出预览且稳定版较低时没有产生降级候选：公开浏览器报告通过，退出 preview 不自动降级。
- systemd 后台执行、更新前备份、失败恢复和失败版本隔离：隔离浏览器验证覆盖不支持/失败态的 fail-closed 与队列清理；未在真实生产 systemd 上更新。
- OpenRC 与轻量 Node 的当前边界已按 docs/release-channels.md 明确呈现：未在生产或 OpenRC 主机验证；轻量节点自签名 TLS 只建议可信网络中的预览验证。

## 生产部署安全核对

- 生产目标和部署授权范围：不适用（预览版禁止生产部署）。
- 验证/灰度环境：environment-policy.json 的 github-hosted 隔离验收；未使用 arena-154。
- 正式部署环境：未使用，未连接生产主机。
- prod-108：按项目策略禁用全部 KPanel 操作；本次未连接、未备份、未部署、未升级、未核对。
- 部署前版本、健康、备份位置及摘要：不适用，未执行生产部署。
- 部署命令/入口：不适用。
- 部署后版本、Panel/Agent 状态、重启、日志、数据完整性和公网入口：不适用。
- 生产已执行写操作：无。
- 仅在隔离真机执行、未在生产执行的场景：公开容器初次启动、重启、浏览器预览更新、失败态与通道验收。

## 回滚

- 源码/tag：RC12 tag v1.27.0-rc.12 对应 3d4cc7435ba3ff6d1204c8eac3800e02a80bfb15；上一预览 RC11 tag v1.27.0-rc.11 对应 78d74cefa760de2e874587352f7f9561ba988ebc；稳定 tag v1.26.0 保留。
- 镜像 digest：RC12 sha256:d2c2eac666f3dff33e5b4ae369c5a805f7de5d07a591b4a244dcba81da5130e2；RC11 sha256:ec869f3f8ed9d8f21ca2ff733f7ec4c59a886be52b2eed1b2a503bc1c7c7199b；RC10 sha256:1868915e2be7837ee087c3e4d13324b470aab3acefef3411e241b57a7028c9fe；stable sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9。
- 数据/配置备份：本次未写生产数据；头像进入账号备份，Release notes 说明 RC11 回退前调整 XP 壁纸及头像字段的兼容行为；没有数据库 schema migration。
- 回滚步骤和回滚后复核：预览用户可固定到 RC11/RC10 immutable digest，稳定通道保持 v1.26.0；未执行实际回滚。
- 回滚后生产实际版本与健康状态：不适用，未生产部署。
- GitHub Latest、Docker latest 与标准更新入口实际指向：GitHub Latest=v1.26.0；Docker latest stable digest=e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9；标准 stable 通道=v1.26.0。
- 公共默认更新通道决策：恢复上一稳定版本作为默认 stable；preview 仍需用户显式选择。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-19T01:40:14+08:00
- 候选冻结时间：2026-10-19T02:21:04+08:00
- 生产完成时间：未验证
- 提交到生产用时：未记录
- 是否回滚、紧急热修复或重复发布：否（无产品失败、回滚或热修复；首次公开验收失败后只重跑验收，未重复发布产物）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

- 预览公开时间：2026-10-18T19:13:36Z。
- 发布流程异常或无效证据拦截：首次公开验收的未认证 GitHub REST tag-object 请求在一个 hosted runner 收到 HTTP 403；Release 已发布但完成标记被 fail-closed。固定 snapshot、stable-before 和 authenticated gh api 结果核对后重跑同一 public-acceptance job 成功；没有改写 verifier 或覆盖公开产物。

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：1
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

### 流程异常明细

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "public-acceptance/github-rest/runner-403",
    "position": "before-production-write",
    "count": 1,
    "impact": "attempt 1 的 public-acceptance job 在读取 annotated tag object API 时收到 HTTP 403，阻止 preview-completion；公开 Release 与镜像已经成功发布，但生产写操作没有开始。",
    "recoveryEvidence": "GitHub Actions run 39154377686 attempt 1 job 117764571250 的原始日志；publication-39154377686-1 与 snapshot-39154377686-1；认证 gh api 确认 tag object 指向精确候选 SHA；attempt 2 job 117765100216 对同一 snapshot 通过，artifact public-39154377686-2。",
    "permanentAction": "本次未修改 verifier，不能把现场重试记作永久修复。KPanel release maintainers 在下一次预览发布前复核 GitHub tag-object API 的认证读取路径；若再次出现 403，改用受认证 API 并增加回归。退出条件：后续预览中的 tag-object 复核稳定通过，且本指纹不重复。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

## 遗留风险与后续准入

- 本地资源回收（按 docs/project-management.md 13.1）：没有回收候选和来源工作树；XP 来源 worktree 有 22 条状态变更，其余四个候选来源 worktree clean。原作者所有权未释放，因此保留所有本地证据与恢复位置。发布证据位于 C:\GitHub\_release-evidence\v1.27.0-rc.12；本记录 worktree 为 C:\GitHub\_release-docs\kpanel-v127-rc12。
- 未验证风险：轻量节点自签名 TLS 接收循环资源上限；真实轻量节点 TLS 配对；真实 NAS/大型图库性能；RC11 对头像与 XP 壁纸字段的回退兼容；原生 200% 缩放、读屏和长期运行。
- 已实现待实机准入：桌面 XP/Win98 外观、头像上传与备份、轻量节点固定公钥连接和图库缩略图反馈已进入 RC12；候选 CI、L3 与公开产物/更新通道验证通过，真实设备专项仍待执行。
- 不阻断本版的理由：用户明确要求发布预览；精确候选 CI、L3、主线 CI、Release、双架构镜像和公开浏览器/隔离容器验收通过；已知 medium TLS 资源上限风险已在 Release notes 披露并限制可信网络使用；GitHub Latest、Docker latest、稳定通道和生产环境未改变。
- 后续应进入的自动门禁或专项工作流：补齐自签名 TLS listener 并发/资源上限及回归；保留 15 个 missing security review 包组在 stable 发布前的既定复核节奏；增加真实轻量节点 TLS 配对与 Gallery 大库/窄视口专项，检查桌面主题和头像的备份恢复兼容性。
