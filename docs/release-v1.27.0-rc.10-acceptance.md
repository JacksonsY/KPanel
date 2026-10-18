# KPanel v1.27.0-rc.10 发布验收记录

已验证：RC10 已公开为预览版；候选 CI、主线 CI、L3、Release workflow 和公开镜像浏览器/隔离容器验收均通过。生产未部署。

日期：2026-10-18

发布级别：L3

候选提交 / 标签：`cf032966f52e8df133ed29f369383a785547310f` / `v1.27.0-rc.10`

上一稳定版本 / 回滚点：`v1.26.0` / Docker index `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`

`releaseChannel`：`preview`

`releaseTrain`：`1.27.0`

候选分支与发布后处置：`release/v1.27.0-candidate` / 预览版保留

- 原分支 / 精确 tip / 处置分类：`release/v1.27.0-candidate` / `cf032966f52e8df133ed29f369383a785547310f` / 预览候选，按项目规则保留。
- 归档 ref 与 SHA / 远端复核结果：预览候选不归档；GitHub `main` 与候选分支在发布后均指向 `cf032966f52e8df133ed29f369383a785547310f`。Annotated tag object 为 `d1bdc95bac211fd758286e0263755a9ff210d41b`，peeled commit 与候选一致。
- 本次来源任务分支：`fix/posix-acl-agent-ipc`（本地 tip `766e9611`，纳入提交 `7e3c0e899197d30efe63d0e8bb09e812b537b036`）和 `fix/gallery-virtual-rendering`（tip `14a7f182f877168701b14c831bfd04908642a835`），均已纳入 RC10；原任务分支和 worktree 保持原状，未删除或迁移。
- 本地分支/upstream/worktree：候选 worktree `C:\GitHub\_release-candidates\kpanel-v127-rc1` 保持干净并留存；发布验收文档使用独立 docs-only worktree，不混入 RC10 产品标签。根主线 worktree 未参与构建，保持原状。
- 未完成归档项 / 责任人 / 下次复核触发条件：无候选归档项；两项来源任务的原 worktree 由其来源任务继续管理。预览候选在 RC 周期内保留，稳定版发布时按项目 10.2 复核处置。

归档不代表生产上线。本次产物已发布，生产未部署。

## 发布画像

- 业务域：媒体图库、部署与本机 Agent/Panel POSIX IPC。
- 变更面：图库展示/性能；Linux 宿主安装与 IPC 文件权限。
- 受影响用户旅程：在大图库中滚动浏览时间线、加载缩略图并打开查看器；Linux 安装或升级时建立安全的私有 IPC 路径与访问控制。
- 未变化契约：未修改公开 API、数据库 schema、端口、Compose 接口、Agent 权限模型、应用市场契约或内置 `kejilion.sh` 基线。
- 风险等级及理由：L3。图库改动涉及大量条目的渲染和视口行为；ACL 改动涉及 Linux 宿主文件权限与进程间通信边界。

## 发布范围与未纳入内容

- 用户可见更新：
  - 图库改为虚拟时间线渲染，降低大图库同时挂载的图块数量；保留缩略图缓存、查看器导航与键盘焦点行为。
  - POSIX IPC ACL 更新统一 Linux 主机上私有 IPC 路径的权限处理，并拒绝危险的链接路径情形。
- 精确提交清单：
  - ACL 源提交 `7e3c0e899197d30efe63d0e8bb09e812b537b036` → 候选 cherry-pick `08862e36be0274e17d1ffbccfdee2e27f233fdc1`。
  - 图库源提交 `14a7f182f877168701b14c831bfd04908642a835` → 候选 cherry-pick `db45b38d85110566f9a682edce40e69649097a70`。
  - 候选组装提交 `cf032966f52e8df133ed29f369383a785547310f`。
- 明确未纳入的事项：没有发布其他来源任务的未合并改动；没有部署生产环境；图库 390px 宽视口在 200% CSS 缩放下仍有横向溢出，列为后续 UX 修复。

## 外部审计与修复交付

- 安全盲审补漏（quality-audit review 档）：本次未执行独立 blind quality-audit。Cloudflare CF boundary audit 判定不适用，范围结果为 `security_review=not-applicable groups=0`；GitHub 的 Go 漏洞、依赖与源码扫描及 L3 已通过。该结论不替代对未来稳定候选的独立审查。
- `node scripts/security-review-scope.mjs --base <上一稳定 tag>` 输出：本次保留记录为 `not-applicable groups=0`；未产生新的 CF audit package 组。
- finding fingerprint / 修复 commit / 独立复核与回归证据：本次 OCR 无已验证 H/M/L finding；源码测试、候选 CI 与 L3 均通过。未报告需延期的已证实 finding。
- 修复交付状态：两项功能已包含在 RC10 源码和公开预览镜像；未发布稳定 tag，未部署生产。
- OCR 观察区间 / 计数：OCR v1.12.11；审查基线 `0524bbcb70a577850f276bcb25c22a0a1f131f4e` 至组装前候选 `95a4c09a8e83b7a2b6ed2d020784f8c0fff45633`；25/25 reviewable files，H0/M0/L0，free-form=0，constrained-only=unreported。最终组装仅改发布元数据/回执 trailer，最终 tree `924f9b2ca3f4ad7934156c31ef0dab6aee456bf4` 与审查树一致。reviewer 在 fresh pass 前已知来源任务的重叠 OCR trailer，blind=false。
- 按 PROJECT_RULES.md 5.5 记录本稳定周期 OCR 观察结果；不足三个周期，不据此评价工具有效性或退出。

## 跨仓库联动判定

- `scriptLinkageState`：`not-required`。
- 变更集编号：不适用。
- KPanel 实际内置脚本基线 commit / SHA-256：`160924a498d9b8d071fed72f165d23c1b9e886de` / `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`。
- 脚本候选 commit / SHA-256：不适用。
- 状态判定依据与兼容性证据：图库和 POSIX IPC ACL 改动不要求跨仓库更新 `kejilion.sh`；Release image labels、受管脚本 hash 和版本均在公开验收中匹配。
- 本版发布决定：脚本不在范围。
- 阻断或移除的依赖范围：不适用。

## 多维质量结论

| 维度 | 状态 | 证据 | 未验证风险或不适用依据 |
| --- | --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 图库 8 个测试套件、99 项通过；候选 CI 和公开容器验收通过。 | 真实用户图库样本未公开验收；本地 2,000 项压力样本为 mock route。 |
| 网络入侵与供应链安全 | 已验证 | Go vulnerability scan、npm audit、源码/依赖/密钥/配置扫描、镜像扫描与 L3 均通过；发布包含 SBOM/provenance。 | 未执行独立 blind quality-audit；未来稳定候选仍须按项目规则复核。 |
| 稳定性、失败恢复与兼容 | 已验证 | L3 source/deploy lanes、`app_conf_update_backup_parity=pass`、公开真实容器初次启动与重启报告通过。 | 未在实际 NAS 和多发行版长期运行环境执行；本次没有生产升级/回滚。 |
| 性能与资源预算 | 已验证 | 2,000 项图库本地 mock 验收：初始挂载 32 项，滚动峰值 80 项，帧 p95 17.5ms、最大 26.9ms；缩略图请求与缓存复用检查通过。 | 390px × 844、200% CSS 缩放出现横向溢出。 |
| 用户体验与可访问性 | 已验证 | 查看器关闭后焦点恢复、空筛选恢复、主题切换、桌面滚动及桌面/平板/手机常用缩放检查通过；公开浏览器更新旅程通过。 | 未执行读屏器专项验证；图库 200% 手机缩放溢出待修。 |
| 数据、配置与迁移 | 不适用 | 不改数据库 schema 或用户数据格式；本次公开验收使用隔离容器。 | 生产配置和宿主机数据未操作。 |

## 自动门禁

- 定向测试及结果：图库 8 suites / 99 tests 通过；web build/typecheck 通过；Linux ACL 的来源任务测试证据与正式 L3 结果保存在发布证据目录。
- `make verify-release`：本次未单独执行；权威 GitHub-hosted L3 入口 `node scripts/run-release-l3.mjs --github-hosted` 通过。L3 run 39109351379，receipt candidate/tree 匹配且 identityUnchanged=true。
- L3 外层入口 run ID、脚本与 Runner：run [39109351379](https://github.com/kejilion/KPanel/actions/runs/39109351379)，2026-10-18T07:53:55Z–08:09:11Z，status=passed；Runner ID `1000003918`，Ubuntu 24 image `20261012.334.1`，Go 1.27.2、Node 24.21.0、Docker 28.0.4。L3 entry SHA-256 `c286f7f2ad692ea17b8ec0537fe2a84e8d049d22adec63a2b00ceba39b165d57`。source lanes web/go/deploy 全部 passed；耗时分别 336357/581106/2980ms，jobs=2，GOMAXPROCS=2。原始 receipt 和日志保存在 `C:\GitHub\_release-evidence\v1.27.0-rc.10\published-final\l3`。
- 候选 CI：首轮精确候选 CI [39108498529](https://github.com/kejilion/KPanel/actions/runs/39108498529) 成功；最终发布编排内再次运行的候选 CI 同样成功。
- 主线 CI：发布编排 run 39109351379 的 `main-ci / verify` 成功。
- Release workflow：run 39109351379 全部必需任务通过，公开验收和 `preview-completion` 成功。
- 发布说明门禁：RC 草稿和最终正文校验通过；公开 report 记录摘要 2 项、升级提示 3 项。
- 安全扫描、镜像契约、SBOM/provenance：候选与 main 扫描通过；原生镜像运行契约、受管脚本 hash、基础容器权限和 healthcheck 验证通过；发布时生成 SBOM/provenance。
- 本机补充验证：`render-release-notes.sh` 成功；本地 Windows 缺少 Go，导致本地 `check-release-notes.sh` 的 Go parser 未执行。GitHub Release workflow 在 Go 1.27.2 runner 上完成了权威草稿和最终正文验证。

## 依赖与技术栈变化

- `make dependency-report` 生成时间及检测源完整性：本次未单独运行依赖报告；Release runner 使用 Go 1.27.2、Node 24.21.0、Docker 28.0.4。
- 最近每日安全通告审计、EOL 复核状态及证据：本次未单独执行 EOL 审核；CI 的 Go/Node 漏洞与依赖扫描通过。
- 直接/基座行动项、传递依赖归属信号及首次完整检测后的启动/决策/处置期限：未产生本次专属处置项；未独立复核基座之外的 EOL。
- 本版采用的依赖、工具链、基础镜像、Action、扫描器或受管脚本候选：产品改动未增加运行时依赖；`web/package.json` 与 lockfile 只更新 RC 版本号到 1.27.0-rc.10。
- 版本/锁文件/Action SHA/镜像 digest/脚本提交与摘要：Go 1.27.2、Node 24.21.0；构建 Action 与基础镜像按 workflow digest 固定；受管脚本为 commit `160924a498d9b8d071fed72f165d23c1b9e886de`，sha256 `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`。
- 暂缓或拒绝候选、证据、负责人、复核日期和退出条件：无。
- 升级后的兼容、安全、构建、性能资源和回滚结论：构建、扫描和公开验收通过；RC9 与稳定版回滚目标保留。未在生产执行升级。

## 隔离真机与浏览器验收

- 主机/发行版/架构/运行时版本：GitHub-hosted Ubuntu 24；L3 Docker 28.0.4；公开验收覆盖 linux/amd64 与 linux/arm64 镜像。
- 环境策略 ID 与允许用途：项目 Release L3 与 public-acceptance workflow；用途限隔离测试，不包含生产目标。
- 使用的精确候选或公开产物：candidate `cf032966f52e8df133ed29f369383a785547310f`；公开 index digest `sha256:1868915e2be7837ee087c3e4d13324b470aab3acefef3411e241b57a7028c9fe`。
- 后台作业 ID、终态、退出码、超时、证据目录：L3 run 39109351379 passed；公开验收 browser-published、browser-upgrade、real-container initial/restart reports 均为 passed，errors=0。原始证据见 `C:\GitHub\_release-evidence\v1.27.0-rc.10\published-final\public-verify`。
- 测试窗口/循环数及风险依据：单次发布级真实公开产物验收，无 soak；RC 预览不代表生产长期运行。
- 受影响旅程、视口、缩放、主题、键盘/焦点、语言和失败态：本地图库 mock 使用 2,000 项，检查筛选恢复、缩略图缓存、查看器焦点恢复、主题和桌面滚动；视口 1280×900、768×900、390×844，100%/200% 缩放。390px 视口 200% CSS zoom 横向溢出。公开浏览器验收同时检查 1280/390/320 下更新对话框、预览版本/digest 和失败态，报告通过。
- 宿主机写入、失败注入、重启恢复和回滚结果：公开容器 initial/restart 与应用更新备份/恢复 parity 通过；仅写隔离容器，未改宿主机或生产数据。
- 未执行场景及原因：生产宿主机、真实 NAS 图片库、读屏器和多小时 soak 未执行，因为本次是预览发布且无生产部署授权。

## 发布产物与公开仓库复核

- 公开 Release 正文、面板发布信息接口与更新弹窗：均指向 1.27.0-rc.10 和 index digest `sha256:1868915e2be7837ee087c3e4d13324b470aab3acefef3411e241b57a7028c9fe`；摘要 2 项、升级提示 3 项；真实浏览器更新旅程通过。
- GitHub Release：[`v1.27.0-rc.10`](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.10)，published 2026-10-18T08:33:07Z，draft=false、prerelease=true；GitHub Latest 仍为 `v1.26.0`。
- Docker OCI index：`docker.io/kjlion/kejilion-panel:1.27.0-rc.10` 与 `:preview` 均为 `sha256:1868915e2be7837ee087c3e4d13324b470aab3acefef3411e241b57a7028c9fe`。
- 稳定版 `:latest`：发布前后均为 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`；公开验收确认 stable 仍为 v1.26.0。上一预览 `v1.27.0-rc.9` 回滚 digest 为 `sha256:ae42c6e900b65bee7a07b048e57d00bccddbbe1e70b00a98d3e5d064f303f915`。
- OCI 子镜像：linux/amd64 `sha256:1512094c86693c3bb4b95489215f8c425f80ebd8e1a54da4c78dff56ec80e0b6`；linux/arm64 `sha256:66530da3f0bf45e6cb9e1dc750ad207202324a7f22402bcf0f97508f6ae9b252`。
- 附件及 `SHA256SUMS`：15 个 Release assets 全部 uploaded；11 个 checksum entries 与公开 Release API digest/size 通过验收，`KPANEL_MODIFIED_COMPONENTS.json` 已上传并校验。
- 公开镜像 `image_e2e=pass`：L3 和 public-acceptance 报告均通过。
- `kejilion/apps` / `kejilion.sh` 契约结论：应用生命周期门禁通过；未修改 `kejilion.sh` 内容，脚本基线与公开镜像标签匹配。

## 自更新通道验收

- 稳定来源选择 GitHub Latest；预览来源选择 RC 并校验唯一官方镜像 digest，公开 browser report passed。
- 加入预览只切换来源并立即检查，没有自动安装。
- 自动安装默认关闭；一次性立即安装与自动安装开关保持独立。
- stable/preview 通道选择、检查失败清队列、退出预览不降级和更新说明显示均通过公开浏览器验收。
- OpenRC、轻量 Node 与真实生产 systemd 部署未在生产环境验证；遵守 `docs/release-channels.md` 边界。

## 生产部署安全核对

- 生产目标和部署授权范围：不适用；本次授权为发布预览，不含生产部署。
- 验证/灰度环境：仅 GitHub-hosted 隔离测试；未使用 arena-154。
- 正式部署环境：未使用；未连接生产主机。
- `prod-108`：未连接、未备份、未部署、未升级、未核对；本次没有对其执行 KPanel 操作。
- 部署前版本、健康、备份位置及摘要：不适用，未执行生产部署。
- 部署命令/入口：不适用。
- 部署后版本、Panel/Agent 状态、重启、日志、数据完整性和公网入口：不适用。
- 生产已执行写操作：无。
- 仅在隔离真机执行、未在生产执行的场景：公开容器启动、重启、浏览器升级流程和失败恢复验证。

## 回滚

- 源码/tag：保留不可变标签 `v1.27.0-rc.9`、`v1.27.0-rc.10`；RC10 的 tag object/peeled commit 已核对。
- 镜像 digest：本版 `sha256:1868915e2be7837ee087c3e4d13324b470aab3acefef3411e241b57a7028c9fe`；上一 RC9 `sha256:ae42c6e900b65bee7a07b048e57d00bccddbbe1e70b00a98d3e5d064f303f915`；稳定回退 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。
- 数据/配置备份：不适用；未对生产数据或配置做写入。
- 回滚步骤和回滚后复核：预览消费者可固定到上一已发布 RC 或稳定 digest；本次没有执行回滚。
- 回滚后生产实际版本与健康状态：不适用。
- GitHub Latest、Docker `:latest` 与标准更新入口：仍指向稳定版 v1.26.0。
- 公共默认更新通道决策：恢复上一稳定版本作为默认 stable；预览仍需用户显式选择。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-18T10:58:12+08:00
- 候选冻结时间：2026-10-18T15:34:37+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（没有产品失败；未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：2
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "candidate-handoff/preflight/contract-shape",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次候选 handoff 预检因 gallery 本地化路径未列入 allowedPaths，且时间戳精度超过校验上限而失败；未发生远端写入。",
    "recoveryEvidence": "更新 allowedPaths 并将时间戳格式化到毫秒后，candidate-handoff-final.log 记录 status=passed、qualificationOnly=true、permissionsGranted=false；首轮失败和最终通过日志均保存在本次发布证据目录。",
    "permanentAction": "后续从候选实际 diff 生成 allowedPaths，并在预检前统一将 evidence 时间戳格式化到毫秒；保留首轮与最终回执，失败时不继续发布。",
    "historicalReleases": []
  },
  {
    "fingerprint": "release-dispatch/inputs-check/main-trust-gate",
    "position": "before-production-write",
    "count": 1,
    "impact": "首个 preview-release dispatch 39109246662 在 inputs 核验处因候选中的 ACL packaging/tests 尚未进入 main 而失败；L3、promote、tag 与 publish 阶段均未启动。",
    "recoveryEvidence": "按候选祖先关系将 main 快进到 ACL commit 08862e36be0274e17d1ffbccfdee2e27f233fdc1；随后 run 39109351379 的 inputs 检查、候选 CI、L3、main CI、发布和公开验收全部通过。",
    "permanentAction": "以后在 dispatch 前对比 main 与候选的 release automation 覆盖路径，并先将要求受信任的测试脚本纳入 main 祖先；本次保留门禁设计，没有绕过检查。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

## 遗留风险与后续准入

- 本地资源回收：本地预览服务及其子进程已停止；候选、L3、公开验收及失败预检证据保存在 `C:\GitHub\_release-evidence\v1.27.0-rc.10`；发布 worktree 保留用于 RC 周期复核。
- 未验证风险：390px、200% CSS 缩放仍横向溢出；未执行读屏器、真实 NAS 图库和多发行版长期运行；未执行生产升级；未执行独立 blind quality-audit；未单独运行 EOL 审核。
- 已实现待实机准入：大图库虚拟时间线和 Linux POSIX IPC ACL 已在公开 RC image 和 CI/L3 通过；真实用户宿主机和 NAS 环境仍待其各自环境准入。
- 不阻断本版的理由：本次发布为预览版，候选 CI、L3、main CI、镜像/安全门禁与公开验收全通过；稳定通道未改变，生产未部署。
- 后续应进入的自动门禁或专项工作流：修复并加入 390px × 200% zoom 的图库回归；稳定候选前复核 POSIX ACL 在目标发行版/初始化系统的实机适配，并完成所需独立安全审查与 EOL 复核。
