# KPanel v1.26.0-rc.8 发布验收记录

日期：2026-10-12。发布级别：L3。releaseChannel：preview；releaseTrain：1.26.0。

**结论：RC8 已公开发布；未部署生产。** GitHub prerelease、双架构版本镜像及 `preview` 通道已核验；`latest` 仍指向原稳定镜像。

- 产品提交：`279adb4de44345663010389a31c9f205e3e9152f`，tree `857ba20e8baa47d017fd515066d2320be4e8f5a5`。
- 标签：`v1.26.0-rc.8`，annotated tag `9d5fb2aca474ca1a2c36dbce6a338f64bfb14ccd`，peeled 指向上述产品提交。
- [公开 Release](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.8)：2026-10-12T01:02:29Z，draft=false、prerelease=true；Latest 仍为 `v1.25.1`；15 个附件。
- 版本镜像与 `preview`：`docker.io/kjlion/kejilion-panel:1.26.0-rc.8` / `:preview`，OCI index 均为 `sha256:fbb6345bb34c6d035c0f60405f640e96e438d844ee6fa70c533a0dc3c80e34a1`；`linux/amd64` child 为 `sha256:c6a79d2db9a75bcc1d419f7d5b5a6db0e7bdb04128dd62d123a5ebc79ca06389`，`linux/arm64` child 为 `sha256:40dcb71402cd4d21984b369f56e94570b3307a3308b53158ebdba32b4bc72d8a`。
- Docker `latest` 仍为 `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`，双架构；上一预览回滚点为 `v1.26.0-rc.7` / `sha256:968946e9dcc1e478fef355f84c355c11db7d5e35b89284c01f6f2649389fdaf4`。
- 公开版本摘要列出了文件终端执行、Dockerfile/Compose 主机操作、运行时检测及相应权限和升级注意事项。预览版不自动安装，不默认进入生产环境。

## 候选分支与提交范围

- 远端 `release/v1.26.0-candidate` 与 `main` 在发布前均指向 `279adb4de44345663010389a31c9f205e3e9152f`；候选分支按 `docs/release-channels.md` 预览规则保留。Release workflow 的稳定版归档步骤因 RC 条件跳过，这是预期状态。
- 本次有 29 个提交纳入 RC8。两项发布前修复均已包含于 tag：`33173a01947729a70d0d540ff193c23143764abe`（更新 torrent 扫描来源身份审阅证据；findings 36 项不变）和 `279adb4de44345663010389a31c9f205e3e9152f`（扩大进程树测试的启动/外层超时）。
- 未发现远端 `fix/v126-rc8-*` 分支。两个本地来源分支在各自独立 worktree 中 Git 状态干净：`fix/v126-rc8-scan-identity-20261012` 位于 `C:\GitHub\_codex-tasks\kpanel-v126-rc8-scan-identity`；`fix/v126-rc8-timeout-test-20261012` 位于 `C:\GitHub\_codex-tasks\kpanel-v126-rc8-timeout-testfix`。两者提交均已进入 RC8；本次保留这些工作树与引用，没有删除其他任务的工作区。

## 发布画像与范围

- 业务域：文件管理/编辑器、主机终端、Docker 操作、软件包运行环境检测。
- 变更面：展示与状态反馈、向所选主机终端写入命令、Docker build/Compose 部署操作；风险为 L3，涉及宿主机状态变更。
- 用户旅程：从文件管理器、编辑器或桌面快捷方式将脚本交给所选主机终端执行；从 Dockerfile 启动构建；从 Compose 文件启动服务；查看 Agent 当前 PATH 可发现的 Python、Node.js、Java、Go 运行时。
- 执行前需核对主机和文件内容。Compose 操作可能创建/重建并启动容器。运行时检测不覆盖用户私有环境或容器内运行时。
- 未单独验证数据库/配置迁移、公网 KPanel 更新弹窗及真实终端主机的 RC 升级旅程；没有生产部署或回滚操作。

## 外部审计与安全覆盖

- 标准 L3 安全门禁已通过：`govulncheck ./...` 无漏洞；npm audit 无高危项；源代码、依赖、密钥/配置及原生镜像扫描通过；镜像 runtime contract 通过。
- 一方源码身份复核覆盖 891 个 Go 源文件、106 个生产模块及 6 个变化的生产 Go 文件。Linux amd64/arm64 闭包均含这 6 个文件；Darwin/Windows 闭包各仅含 `internal/version/version.go`，其他五个文件不在这些入口闭包中。review 摘要 SHA-256：`9e9e11349af242129f8b916cdaf543b6eeb7b7ed1626c5294dcd3eaea289e217`；独立闭包身份复核文件见 `C:\GitHub\_release-evidence\v1.26.0-rc.8-final\first-party-source-review\independent-closure-identity-verification.json`。审阅没有宣称源码扫描覆盖等于人工安全审计。
- `node scripts/check-security-audit-coverage.mjs --target HEAD --format=json`：`decision=scoped-required`，`comparison_base=4c0694aa8e02e46145a775707b8d5a0355f7ce10`，`source_ref=279adb4de44345663010389a31c9f205e3e9152f`；1 个待审计提交，最早待审 7 天，最近 full 为 20 天，触发新边界包 `internal/bittorrent`、`internal/offers`。
- 本轮没有完成 CF scoped/full 安全审计，也不把该状态写成通过。项目规则允许 preview 记录该门禁而不阻断预览发布；稳定版前须完成对应 scoped 审计或按规则取得用户豁免。
- `file-execution-resource-version-path-reopen` 仍为 `needs_validation`。run-35 不完整，未取得非生产 Linux LightNode 的真实 broker 状态、目标/父路径权限 ACL、挂载及低信任账号有效写权限证据；run-36 覆盖的是不同 Runtime PATH finding；run-37 只有进行中元数据且零 invocation，均不关闭此项。

## 跨仓库联动判定

- `scriptLinkageState`：`not-required`。本次 KPanel 差异未改动跨仓库 `apps` 或 `kejilion.sh` 源文件；release workflow 的 `kejilion.sh` 生命周期检查通过。
- 变更集编号、脚本候选 commit/SHA、脚本阻断范围：不适用。没有据此宣称跨仓库发布或真实应用市场验证通过。

## 多维质量结论

| 维度 | 状态 | 证据 | 未验证风险或不适用依据 |
| --- | --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | L3 `make verify-release`、Go/Web 检查及精确候选浏览器旅程通过 | mock 旅程不能代替真实用户主机部署 |
| 网络入侵与供应链安全 | 已实现未实机验证 | govulncheck、npm audit、Trivy、镜像扫描及身份/闭包复核通过 | CF coverage 为 scoped-required；FileExecution finding 仍 needs_validation |
| 稳定性、失败恢复与兼容 | 已验证 | 双架构构建、镜像运行契约及 lifecycle 检查通过 | 未进行生产升级、真实回滚或长时公网 soak |
| 性能与资源预算 | 已实现未实机验证 | 本次未新增性能预算结论 | 未进行真实部署负载和长时资源观测 |
| 用户体验与可访问性 | 已验证 | classic/desktop overview 9 个场景通过；保存并执行 mock 旅程通过 | 未完成屏幕阅读器、实体手机及跨浏览器验收 |
| 数据、配置与迁移 | 已实现未实机验证 | 发布自动化和源代码检查通过 | 未执行真实安装升级、备份恢复或生产迁移 |

## 自动门禁与隔离验收

- 最终 L3：`v1.26.0-rc.8-279adb4d-l3-r1`，target `arena-154`；2026-10-11T23:52:22Z—2026-10-12T00:14:44Z，exit 0。Runner image `kpanel-go127-security-runner:go1.27.2-node24.21.0`，不可变 ID `sha256:20667a9f1fad6590219ba8d8acdabc09a49f8683d4a877b847df17d28b83f8fb`。计划 SHA-256 `d49d8f452bdf95b28e69411780ecd65c1bca06849f9b21088667d008064970c1`；远端脚本 SHA-256 `21c08b11be3526a0fe766aa606a81d0e4047e8a4e89d79227da4e62914164979`；bundle SHA-256 `d84615505d7a7e02c6c9d6bf1f0696983096d755e0f9e46305dac15e922857ae`。远端证据已复制到 `C:\GitHub\_release-evidence\v1.26.0-rc.8-279adb4d-l3-r1\remote-evidence` 并逐文件通过 `evidence.sha256` 校验。
- `make verify-release` 的源检查通过，300 项脚本测试、Web typecheck/test/build、Go test/race/vet、安全扫描、安装和部署检查通过；本地 `source_checks=pass`，约 949,890 ms。
- 候选 CI `38187622700` success；候选 Dependency freshness `38187622672` success；main CI `38188559838` success（13m53s）；main Dependency freshness `38188559833` success；tag Dependency freshness `38189492472` success；Release workflow `38189492524` success。
- 浏览器概览：`C:\GitHub\_release-evidence\v1.26.0-rc.8-279adb4d-browser-overview\summary.json`，9 个场景通过、无错误/回归、cleanup=true。保存/执行：`C:\GitHub\_release-evidence\v1.26.0-rc.8-279adb4d-browser-save-execute\result.json`，一次保存、仅一次 `./demo.sh` 命令、无外部请求。
- 本地 acceptance 预览：`http://127.0.0.1:4174`，HTTP 200，精确绑定 `279adb4d`，mode=mock；只证明 mock 用户旅程，不证明真实服务端行为。浏览器截图和布局结果在上述证据目录。

## 依赖与技术栈变化

- 候选、main、tag 的 Dependency freshness workflow 均成功。该结论不替代逐项上游 EOL 或稳定候选采纳审查；本记录没有单独登记新的依赖候选采纳。
- Go/npm lockfile 外的上游长期维护状态：未验证。稳定版冻结前应沿 dependency-policy 的检测结果复核。

## 发布产物与更新通道

- GitHub Release `draft=false`、`prerelease=true`；GitHub Latest 仍为稳定版 `v1.25.1`。15 个附件包含 `SHA256SUMS`、Linux Agent/轻量节点二进制、MCP 二进制、metadata archive 与 `KPANEL_MODIFIED_COMPONENTS.json`；Release workflow 的 provenance、最终说明及产物校验通过。
- Docker Hub 公开读回确认版本 OCI index 与 `preview` digest 完全一致；双架构 child digest 如本记录顶部所列。`latest` 摘要未改变。
- 设置页加入预览版、只切换来源并立即检查、自动安装与立即安装分离、退出预览不降级、systemd 失败恢复等 live KPanel 更新旅程：已实现未实机验证。本次没有安装 RC8 到真实 Panel，也没有访问应用市场发布接口。
- `kejilion.sh` 生命周期容器检查通过；`kejilion/apps` 实际生产入口与公网上的更新信息 API：未验证。

## 生产部署安全核对

- 预览版生产部署：不适用（预览版禁止生产部署）。
- 本次只在策略允许的隔离 L3 目标 `arena-154` 执行发布验证；没有连接、备份、部署、升级或核对 `prod-108`，没有生产写操作。
- 公共默认更新通道仍为 stable；用户需主动加入 preview。退出预览不会自动降级。

## 回滚

- 本 RC tag 与镜像均不可覆盖；若需退出本次预览，使用上一预览 `v1.26.0-rc.7` / `sha256:968946e9dcc1e478fef355f84c355c11db7d5e35b89284c01f6f2649389fdaf4`，或按稳定通道回到 `v1.25.1` / `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。实际目标须在安装主机核对版本和 digest 后选择。
- 本次无生产数据/配置备份、无回滚执行；生产恢复状态不适用。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-11T19:34:25+08:00
- 候选冻结时间：2026-10-12T08:20:24+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无已发布变更失败；候选 L3 门禁失败另计）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：3
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

### 流程异常明细

三次失败均发生在生产写操作前：一次 first-party source observation hash 不匹配；两次由 500ms 子进程窗口导致同一测试断言未等到 descendant 启动。失败原件保留在 `arena-154:/root/kpanel-release-evidence/`，对应 run 为 `v1.26.0-rc.8-45cfc08a-l3-r1`、`v1.26.0-rc.8-33173a01-l3-r1`、`v1.26.0-rc.8-33173a01-l3-r2`。修复证据摘要后，最终 `279adb4d` L3 通过；测试等待窗口由 500ms 改为 3000ms，外层 timeout 由 10s 改为 15s。没有忽略失败或覆盖原件。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "candidate-gate/l3/source-observation-hash-mismatch",
    "position": "before-production-write",
    "count": 1,
    "impact": "RC8 的首次 L3 在 source-check 中发现 first-party source observation 与候选闭包身份不匹配，阻止了该候选继续发布。",
    "recoveryEvidence": "arena-154 run v1.26.0-rc.8-45cfc08a-l3-r1 status=failed；review 摘要经独立闭包身份复核后在 33173a01947729a70d0d540ff193c23143764abe 修正，findings 36 项不变；最终 run v1.26.0-rc.8-279adb4d-l3-r1 pass。",
    "permanentAction": "修正 reviewer evidence 中的目标闭包描述并固定 summary/closure/source hashes；最终 L3 对精确 commit 重跑并保留远端证据哈希。",
    "historicalReleases": []
  },
  {
    "fingerprint": "candidate-gate/source-checks/child-start-timeout-window",
    "position": "before-production-write",
    "count": 2,
    "impact": "同一 source-check 测试在两次 L3 运行中都因 500ms 超时窗口未等到子进程启动而失败；未进入发布写操作。",
    "recoveryEvidence": "arena-154 runs v1.26.0-rc.8-33173a01-l3-r1 与 -l3-r2 均记录 descendant must actually run before cancellation；commit 279adb4de44345663010389a31c9f205e3e9152f 将窗口改为 3000ms/外层 15s，最终 L3、候选 CI、main CI 和 Release workflow 均通过。",
    "permanentAction": "扩大进程树测试启动等待与外层 timeout；以最终候选重新执行完整 L3、候选 CI、main CI 和 Release workflow。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

## 遗留风险与后续准入

- 本地资源回收：保留 L3 和浏览器验收证据目录；`http://127.0.0.1:4174` mock acceptance 预览仍运行供用户核验。未删除两个来源 worktree，也未清理共享资源。
- 未验证风险：CF scoped audit、FileExecution finding 的 LightNode broker/路径权限实测、公网更新信息 API、真实用户主机安装升级与回滚、屏幕阅读器、实体设备和长时 WAN soak。
- 不阻断本预览的理由：发布通道规范允许 RC 仅记录 coverage decision；无生产部署；所有自动 L3、CI、依赖、镜像和公开产物门禁通过。
- 后续准入：稳定版冻结前完成 scoped audit 并关闭/明确处理 needs_validation；执行登记的隔离真实主机升级、失败恢复与更新通道验收；稳定版生产写入仍需独立 L3 与授权。
