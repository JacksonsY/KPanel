# KPanel v1.26.0-rc.9 发布验收记录

日期：2026-10-12。发布级别：L3。

候选提交 / 标签：`c9e5063f028db9dc6efedc9c89e2c5f8cac8a855` / `v1.26.0-rc.9`。

上一稳定版本 / 回滚点：`v1.25.1` / commit `6ee57f095268346b115f64c0f2c6fec6212157c8`，Docker `latest` digest `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。上一预览 RC8 镜像 digest 为 `sha256:fbb6345bb34c6d035c0f60405f640e96e438d844ee6fa70c533a0dc3c80e34a1`。

`releaseChannel`：`preview`

`releaseTrain`：`1.26.0`

候选分支与发布后处置：`release/v1.26.0-candidate` / 预览版保留。

- 原分支 / 精确 tip / 处置分类：`release/v1.26.0-candidate` / `c9e5063f028db9dc6efedc9c89e2c5f8cac8a855` / 预览候选保留；远端 `main` 及候选分支均指向此 SHA。annotated tag object `67f793131493609402daafa3c75547f99440cc31` peeled 到同一 SHA。
- 归档 ref 与 SHA / 远端复核结果：预览候选按规则不归档；远端 refs、GitHub Release 和 tag 已复核。候选分支 tip 与公开 tag 一致。
- 本次来源任务分支：`feature/systemd-services-backend` tip `33c11668e6f8152c29823736ce2ac82e4812dddd`，仅有独有提交 `33c11668 feat(system): add bounded systemd service management`；`feature/systemd-services` tip `10dcb1d5502a8b899bbdf644f9759cf74e79ebcd`，独有提交 `a07b02e4`、`a4fcb0ce`、`01f043fa`、`640b951e`、`309fe93d`、`10dcb1d5`。两条分支均非 RC9 的提交祖先，工作树分别位于 `C:/GitHub/_codex-tasks/kpanel-systemd-backend` 与 `C:/GitHub/_codex-tasks/kpanel-systemd-services`，干净但各比 RC9 落后 63 个主线提交。
- 来源分支内容核对：后端分支 12 个文件的 systemd 实现与 RC9 内容一致，`internal/panel/server.go` 有 1 行差异；UI 分支 25 个文件的主体已体现在 RC9，`scripts/mock-app-market-api.mjs`、Overview 双语文案和 `web/src/lib/api.ts` 有差异。未把其独有 commit 误报为已合入，也未改动/归档这两个独立活动 worktree；所有权释放尚未确认，留待协调中心处置。
- 本地分支/upstream/worktree：本次候选 worktree 干净并保留在 `release/v1.26.0-candidate`；上述两个 systemd 来源 worktree 干净保留。应用市场本地工作树仍在 `feat/lanqin-email`（落后远端 main 3 个提交），本次未修改。
- 未完成归档项 / 责任人 / 下次复核触发条件：两条 systemd 来源分支尚未归档；由原任务所有者确认释放后，由协调中心按 `docs/project-management.md` 10.2 核对精确映射并处置。RC9 预览候选本身必须保留至 1.26.0 稳定版发布。

**结论：RC9 已公开发布为预览版；未部署生产。** GitHub prerelease 与 Docker `preview` 已验证，GitHub Latest 仍为 `v1.25.1`，Docker `latest` digest 未变。

## 发布画像

- 业务域：系统中心 / 日常维护 / systemd 服务管理。
- 变更面：展示、只读查询、宿主机写入、Agent API 与部署权限契约。
- 受影响用户旅程：在所选 Linux systemd 主机查看服务状态、自启状态、详情和最近日志；经确认后启动、停止、重启、重载服务或调整开机自启。
- 权限边界：仅在 Linux systemd 主机、Agent 以 root 运行且启用宿主机系统写入时开放写操作；其他环境返回不支持原因。应用配置包含 `CAP_KILL`，应用市场远端 `main` 的 `kpanel.conf` blob `9d2d2efdc334db1ba014203370fdc8281f1a5870` 与候选文件完全相同，无需另行同步提交。
- 未变化契约：无数据库 schema / 数据迁移；无端口和 Compose 契约变化；`kejilion.sh` 生命周期检查通过。新增 system-services API/Agent 契约，不兼容地扩大写入面的证据已由 L3 与 CI 检查。
- 风险等级及理由：L3；操作直接影响所选主机服务，误操作可能中断 SSH、网络或面板连接。预览仅供主动加入 preview 的用户评估。

## 发布范围与未纳入内容

- 用户可见更新：系统中心“日常维护”增加 systemd 服务管理，查看服务状态、自启、详情和最近日志；确认后可 start/stop/restart/reload 与 enable/disable。
- 精确提交清单（相对发布前 `main=198b493af6068f00d6918375b9d32daf9076e0bc`）：`2eeb15ee` 系统管理 UI；`6aca184a` 本地化与不支持状态；`bb14a308` 有界 systemd 管理；`62912014` 日志控件翻译；`e2968b30` Teleport 内容语言测试；`3ce59821` RC9 版本准备；`c9e5063f` 来源闭包身份元数据。
- 明确未纳入：两个系统管理 source worktree 的独有 commit 没有按 commit ancestry 纳入 tag；其功能差异如上节所述。实时 systemd 宿主机操作、生产部署和真实更新弹窗未验证。

## 外部审计与修复交付

- 安全审计 run：`run-34` / scoped / source-only / 状态 `incomplete`；精确源码基线 `4c0694aa8e02e46145a775707b8d5a0355f7ce10`，审计目标 `e2968b30962b2ac6b89c9c73f6f84dc699705558`（五个 systemd 提交）。使用 `gpt-6-luna` / `max`；12 次代理调用、2799 秒。由于验证器与 Phase 5 未完成且宿主 OS 沙箱约束不可用，此 run 不计入覆盖。
- 覆盖检查：`decision=scoped-required`，全局有 84 个待审提交；其中 1 个与本次审计范围相交，另 83 个在范围外。最新 full 基线为 `4c0694aa8e02e46145a775707b8d5a0355f7ce10`，距今 7 天；全局覆盖未完成。
- 未确认漏洞；候选 `agent-token-panel-group-membership` 只留在 ledger，`findings.json` 为 0 条。条件风险是应用市场安装复用已有 `kejilion-panel` 组时未检查 supplemental/primary GID 成员，若其中存在低信任主机账号，可能取得 root Agent token/socket 访问权。实际成员与 ACL 未观察，不能写成已确认漏洞，也不能写成已排除。
- 审计原件：`C:/GitHub/_release-evidence/v1.26.0-rc.9-systemd/security-audit/REPORT.md`、`coverage-ledger.json`、`run-metadata.json`；findings 校验通过（0 条），coverage ledger 校验通过（7 个单元）。这不是完整 CF audit pass。
- 来源身份独立复核：`C:/GitHub/_release-evidence/v1.26.0-rc.9-systemd/identity-check/independent-review.json`，SHA-256 `425a69eb000bb8cc6285c6811991f301075e2647e4ad5abd0fca472e06835941`。该复核覆盖源/闭包身份；不等于漏洞扫描或完整人工审计。
- OCR：版本 `1.12.11`，preview/rule/rules-check 结果为 28/28 reviewable、10 项按规则排除；范围文件 `scripts/torrent-backport-scan-scope.json` 的规则检查通过。提交 trailer 限定 `198b493..be782a6`，不宣称覆盖全 release。
- 修复交付状态：systemd 源码、RC9 和公开 Release 已发布；尚无稳定版 tag，未部署。CF scoped 补审、candidate 的部署事实核查和其他 83 项仍未完成。

## 跨仓库联动判定

- `scriptLinkageState`：`not-required`（无需发布脚本；本次未改变 `kejilion.sh` 脚本协议或源文件）。
- 变更集编号：不适用。
- KPanel 内置生命周期脚本基线：沿用 `1800d955f216aebd2776674368a8e469a671c489`；脚本提交与摘要：不适用。Release workflow 双 Shell lifecycle 检查通过。
- 状态判定依据：`kejilion.sh` lifecycle 脚本未变；`kpanel.conf` 只新增 `CAP_KILL` 配置。应用市场远端 `main=6602ac36e0c329e35b74ff621721b23583e94f5f` 的 `kpanel.conf` blob 与候选完全一致，故无需 apps 仓库提交。
- 本版发布决定：只发布 KPanel preview；不更改应用市场默认 stable / `latest` 更新入口。
- 阻断或移除的依赖范围：不适用。

## 多维质量结论

| 维度 | 状态 | 证据 | 未验证风险或不适用依据 |
| --- | --- | --- | --- |
| 业务正确性与双端互通 | 已实现未实机验证 | L3、候选/主线 CI 通过；systemd mock 用户旅程可打开服务列表、详情与日志；公开镜像 E2E 通过 | 未在真实 systemd 宿主机上执行服务写操作 |
| 网络入侵与供应链安全 | 已实现未实机验证 | govulncheck/Trivy/npm audit、镜像扫描和 runtime contract 自动门禁通过；来源闭包独立复核完成 | CF scoped audit incomplete；`agent-token-panel-group-membership` 的部署前提与 ACL 未核实 |
| 稳定性、失败恢复与兼容 | 已实现未实机验证 | 双架构构建、最小权限运行契约及 `image_e2e=pass` | 未做真实宿主机服务失败注入、Agent 重启恢复和回滚演练 |
| 性能与资源预算 | 不适用 | 本版未建立性能改善结论；Release 镜像按 runtime resource contract 验证 | 无新增性能基准或长时间资源趋势证据 |
| 用户体验与可访问性 | 已实现未实机验证 | 本地 mock 预览 HTTP 200；7 个模拟服务的列表、详情、日志以及相关翻译已查看 | 未完成真实服务数据、屏幕阅读器、实体手机及所有主题/缩放组合验收 |
| 数据、配置与迁移 | 已实现未实机验证 | 无数据库 schema 变化；公开容器健康与 bootstrap 检查通过 | 未执行真实安装升级、备份恢复或生产迁移 |

## 自动门禁

- `make verify-release`：在固定 `arena-154` Runner 上通过；覆盖 source checks、Go test/race/vet、Web typecheck/test/build、扫描、双架构构建及 runtime 镜像契约。
- L3 外层入口：`v1.26.0-rc.9-c9e5063-l3-r2`，target `arena-154`，终态 PASS；Runner `kpanel-go127-security-runner:go1.27.2-node24.21.0`，ID `sha256:20667a9f1fad6590219ba8d8acdabc09a49f8683d4a877b847df17d28b83f8fb`；plan SHA-256 `dae8d19e6fb04f95d8eb5609e81b73b8724153567afa50e6fe12b8518fd27dca`；remote script SHA-256 `21c08b11be3526a0fe766aa606a81d0e4047e8a4e89d79227da4e62914164979`；bundle SHA-256 `9c4a71c61578d18ee22fd9cfd39ae3800ac08ada66d9f2671138c1ec81cbae46`；证据目录 `C:/GitHub/_release-evidence/v1.26.0-rc.9-systemd/v1.26.0-rc.9-c9e5063-l3-r2`。
- 候选 CI：run `38202041109` 成功；Dependency freshness `38202041122` 成功。
- 主线 CI：run `38203046340` 成功；Dependency freshness `38203046353` 成功。tag Dependency freshness `38204016569` 成功。
- Release workflow：run `38204016586` 成功；预览候选归档步骤按规则跳过。
- 发布说明门禁：草稿与最终正文均通过；目标 `1.26.0-rc.9`，1 条新增说明、2 条升级注意事项；明确说明 systemd 写入前提与主机中断风险。
- 安全扫描、镜像契约、SBOM/provenance：Go 调用路径、npm、Trivy 源码与镜像扫描、容器 runtime contract、修改组件 provenance 步骤均通过。govulncheck 记录有一个依赖 advisory 未见代码调用路径；不得概括为依赖零漏洞。
- 首次 L3 尝试 `v1.26.0-rc.9-3ce5982-l3-r1` 被拦截：`torrent-backport-scan-scope.json` 仍钉住 RC8 的 source/closure SHA；未触发目标上的候选执行或生产写入。按独立身份复核更新两条 Linux scope row 后，最终 c9 候选的 L3 r2 通过。失败原件保留于 `C:/GitHub/_release-evidence/v1.26.0-rc.9-systemd/v1.26.0-rc.9-3ce5982-l3-r1`。

## 依赖与技术栈变化

- 依赖新鲜度：候选、main、tag 三次 workflow 均通过（`38202041122`、`38203046353`、`38204016569`）。
- 本版没有新增 Go module 或 Node 依赖；Go/Node 发行工具链与扫描器版本按锁定 workflow 执行。`go.mod` / `go.sum` 未变；前端 lockfile 只随版本号更新。
- EOL/上游长期维护状态：未在本 RC 单独验证；稳定版冻结前按依赖策略复核。
- 暂缓项：CF scoped audit 与 systemd 真实宿主机写操作验证列为稳定版前准入事项。

## 隔离真机与浏览器验收

- L3 环境：`arena-154`，Linux，固定 Runner ID 如上；仅用于候选验证，不是生产部署。
- 使用精确候选 `c9e5063f028db9dc6efedc9c89e2c5f8cac8a855` 完成 L3；公开 image E2E 使用 `1.26.0-rc.9`。
- 本地 mock acceptance 预览：`http://127.0.0.1:4176/overview`，HTTP 200，mode=mock；7 个模拟 systemd 服务、详情和日志可显示。证据目录 `C:/GitHub/_release-evidence/v1.26.0-rc.9-systemd/preview-3ce598219`。mock 不代表真实 systemd 主机行为。
- 本次没有对真实主机执行 start/stop/restart/reload/enable/disable，没有真实服务失败注入、重启恢复或长时间 soak；未验证屏幕阅读器、实体手机、所有缩放和主题组合。
- 后续应在明确登记的非生产 systemd 主机核对 Agent UID/权限、服务操作前确认、服务状态刷新、日志上限及失败后恢复。

## 发布产物与公开仓库复核

- GitHub Release：[`v1.26.0-rc.9`](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.9)，2026-10-12T04:45:48Z 发布，`draft=false`、`prerelease=true`；Latest 仍为 `v1.25.1`。
- Docker `kjlion/kejilion-panel:1.26.0-rc.9` 与 `:preview` OCI index 完全相同：`sha256:7e2cd7cb16340ba3b473d1cb3d857269e2ca2962d4d3ddf6a95caed1bb284e19`。
- 双架构 child digest：linux/amd64 `sha256:07fd183f61584df345777a7beb87dacc1692ea5a6d85c7274c35d0d0c860b670`；linux/arm64 `sha256:6291ec735eb67e46f338bc413dd10608ec779414d5e997a83a1743c3c89aa3e5`。`unknown/unknown` 项为 provenance attestation。
- Docker `latest` 仍为双架构 digest `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`；未被本次修改。
- 15 个 Release 附件包含 Agent/轻节点 Linux 双架构、MCP 二进制、meta archive、`KPANEL_MODIFIED_COMPONENTS.json`、`SHA256SUMS`、许可和 third-party notices。`SHA256SUMS` asset digest `sha256:791930ffbaac4605471c27904fc33e43f9906b74356230dab8af7a4c8236c454`。
- 公开镜像验收：WSL Docker root 拉取公开版本镜像并执行 `packaging/tests/image-e2e.sh`，结果 `image_e2e=pass`。脚本检查版本 health、Host/Forwarded headers、静态资产字节、bootstrap 安全 cookie 与 healthy 状态；退出后无遗留测试容器或网络。
- 应用市场 / `kejilion.sh`：`kpanel.conf` blob 与公开 apps `main` 一致，无额外 apps 提交；生命周期自动检查通过。应用市场默认入口仍为 stable / `latest`。

## 自更新通道验收

- GitHub prerelease、Docker `preview` 与稳定 `latest` 标签复核通过；版本镜像与 preview 的 digest 相同，latest 未变。
- 实际登录 Panel 后对 `/api/v1/settings/kpanel-release` 的 authenticated 响应与更新弹窗未验证；本地 mock 预览不作为 live API 证据。
- 加入/退出 preview、来源保持、自动安装/一次性安装隔离、后台更新失败恢复等旅程本次未在真实 Panel 实例验证。RC9 更新说明提示只有主动加入预览计划者安装，退出 preview 不会自动降级。
- OpenRC 与轻量 Node 的 systemd 服务管理边界：不适用，systemd 写操作仅在受支持 Linux 主机开放。

## 生产部署安全核对

- 生产动作：不适用（预览版禁止生产部署）。
- 验证环境：L3 使用 `arena-154`；公开镜像 E2E 在本地 WSL 容器执行。
- 正式部署环境：不适用。本次未连接、未备份、未部署、未升级或检查任何生产 Panel。
- `prod-108`：按环境策略禁用全部 KPanel 操作；本次未连接、未备份、未部署、未升级、未核对。
- 生产已执行写操作：无。
- 仅在隔离环境执行：L3 runner 的候选检查；本地 mock UI 和公开镜像容器 E2E。

## 回滚

- 源码/tag：上一预览 `v1.26.0-rc.8`，commit `279adb4de44345663010389a31c9f205e3e9152f`；稳定回退点 `v1.25.1` / `6ee57f095268346b115f64c0f2c6fec6212157c8`。
- 镜像 digest：上一 preview `sha256:fbb6345bb34c6d035c0f60405f640e96e438d844ee6fa70c533a0dc3c80e34a1`；稳定 latest `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。
- 数据/配置备份、回滚执行和回滚后生产状态：不适用；未进行生产部署或生产回滚。
- 公共默认更新通道：保持 v1.25.1 / `latest`，未回退；RC9 只在 GitHub prerelease 与 Docker `preview`。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-12T09:56:10+08:00
- 候选冻结时间：2026-10-12T11:32:09+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（RC9 首次公开发布；未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：1
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

### 流程异常明细

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "candidate-gate/torrent-scan-scope/stale-source-hash",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次 L3 对 3ce5982 候选发现 torrent-backport-scan-scope.json 仍固定 RC8 source/closure 哈希；门禁在候选目标执行和公开发布前停止，本事件未造成产品生产影响。",
    "recoveryEvidence": "失败 run v1.26.0-rc.9-3ce5982-l3-r1 原件保留；独立 source/closure 身份复核确认 Linux 两行变化；c9e5063 更新元数据后 v1.26.0-rc.9-c9e5063-l3-r2 在 arena-154 通过。",
    "permanentAction": "按新候选更新 firstPartySourceSha256 与 closureSha256；保留独立身份复核 SHA-256 425a69eb000bb8cc6285c6811991f301075e2647e4ad5abd0fca472e06835941，并对最终精确提交重跑完整 L3。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

## 遗留风险与后续准入

- 本地资源：公开 E2E 容器、网络和临时目录均由测试脚本清理；本机只留下可再拉取的 RC9 镜像缓存。保留 mock acceptance 预览 `http://127.0.0.1:4176/overview` 供用户核验；证据目录保留在 `C:/GitHub/_release-evidence/v1.26.0-rc.9-systemd`。
- 未验证风险：CF scoped audit incomplete；`agent-token-panel-group-membership` 的真实 group membership/ACL 未验证；未在真实 systemd 主机进行写入；live release API 与更新弹窗未验证；未做生产升级/回滚、长时间 soak、屏幕阅读器验收。
- 不阻断本预览的理由：项目预览策略允许记录 `scoped-required` 后继续；完整 L3、候选/主线/tag CI、Release、公开镜像双架构和 image E2E 均通过，且没有生产部署。
- 后续准入：稳定版前完成要求的 CF scoped/full 覆盖审计并按状态核查 candidate；在登记隔离 Linux systemd 主机验证受支持/不支持边界、权限、服务操作与失败恢复；验证 live release API 和更新弹窗；复核两条 systemd 来源 worktree 的所有者与精确处置；稳定版生产写仍需独立授权与 L3。
