# KPanel v1.28.0-rc.6 发布验收记录

日期：2026-10-25；发布级别：L3；`releaseChannel=preview`；`releaseTrain=1.28.0`。

候选 / 标签：`da7e266a5f6f716711961860d77e36f8130f3cb4` / `v1.28.0-rc.6`；tree `d369ea518d3c48ed7557248b6406ea6f586aac58`。

产物已发布，生产未部署。GitHub Latest / Docker latest 保持 v1.27.2；本版仅更新 prerelease / preview。

## 发布画像与范围

- 虚拟机 `4b711c0b`：基于现有 libvirt 发现、创建、启停、开机自启、ISO 与浏览器控制台；云镜像来源固定并校验，删除默认保留磁盘。
- 文件传输 `60b0e50e`：统一活动区、从持久任务恢复显示、减少重复状态，区分后台与依赖页面的操作，提供失败记录与诊断。
- 桌面网站监控 `b8a7522e`：补上桌面窗口路由、归组、标题和恢复识别，流量与安全在原网站窗口打开；不把此修复当作所有桌面恢复偶发问题已关闭。
- 接手修复 `98873aca` 仅调整 worker 测试权限环境：普通用户必须被拒绝且不调用工具；原 root 持久性断言完整保留并在原生验收明确执行。运行时 root 所有权要求未放宽。
- 风险为 L3：新增宿主机写入和原生虚拟机任务、展示与生命周期。libvirt 是资源真源，Panel 无特权，Agent 固定动作；没有第二套虚拟机库存、自编网络 XML 或防火墙模板。
- 未纳入稳定发布、生产、其他未完成/未知改动、默认轻量 Node 安装器修复，或旧文件后端超时/断点续传重构。

## 来源与分支收尾

预览列车 `release/v1.28.0-candidate` 保留 `da7e266a5f6f716711961860d77e36f8130f3cb4`；annotated tag 对象 `9f8d549127c9460ec873fefbf71c9a644d1f54e1` peeled commit 相同。

- `feature/virtual-machines` / `4b711c0b738815617501bf2fa03f5f9141805c65` → `archive/feature/virtual-machines`；远端精确回读、本地 clean 和解除旧 upstream 完成，工作树保留。
- `fix/file-transfer-status-clarity` / `60b0e50ebce753c58e81971b0a3f634e33263bda` → `archive/fix/file-transfer-status-clarity`；远端精确回读、本地 clean 和解除旧 upstream 完成，工作树保留。
- `fix/rc6-site-monitor-desktop` / `b8a7522e96b534b59b9db0c9a684541ffe254b58` → `archive/fix/rc6-site-monitor-desktop`；远端精确回读、本地 clean 和解除旧 upstream 完成，工作树保留。
- `fix/rc6-vm-worker-ownership` / `98873acafc6a63987d3441525d02283a30e186e3` → `archive/fix/rc6-vm-worker-ownership`；远端精确回读、本地 clean 和解除旧 upstream 完成，工作树保留。
- `fix/rc6-vm-preview-fixture` / `141b21204b72ba5afa6b09decdc65ff232c49216` → `archive/fix/rc6-vm-preview-fixture`；远端精确回读、本地 clean 和解除旧 upstream 完成，工作树保留。

- 工具候选 `141b21204b72ba5afa6b09decdc65ff232c49216`：候选 CI 40198391992、主线 CI 40199071344 通过；原生运行 40199079468 通过后与本轮来源一起归档回读。
- 本地代码、原始失败、首轮报告、回滚工作树与证据全部保留；本次只停止已核验归属的预览。未知端口/进程及未提交内容不作为可删除残留。没有磁盘删除或空间回收收益声明。
- 归档只移出候选队列，不代表生产上线；验收文档分支在其精确候选/主线 CI 完成后另行归档，发布列车继续保留。

## 原生虚拟机与来源硬条件

[GitHub-hosted 原生验收 40199079468](https://github.com/kejilion/KPanel/actions/runs/40199079468) / attempt1，实际候选 `c274e8979e1b8553b3a27c4f2205b691ea23b39e`、控制器 `141b21204b72ba5afa6b09decdc65ff232c49216`。最终冻结仅增加事实文档，`cmd/internal/web` 与该实测提交逐字节一致；不能声称最终 OCI 镜像已在此宿主机安装。

- Ubuntu 24.04.5 / amd64、4 CPU、15989 MiB；真实 Agent 源码构建、真实 transient systemd worker 完成 install / prepare / create / delete。原生集成没有手动直接调用 RunJob。
- Debian 12 客体真实启动与公钥 SSH 登录，DNS 和 HTTPS 到官方 Debian 站点经 libvirt NAT 通过；虚拟机与所选磁盘由真实任务删除，资源回读确认移除。
- Docker 主机发布端口、同桥容器访问在操作前后均成功，客体 NAT 活跃时发布端口仍可用；FORWARD 前后均为 DROP，libvirt/Docker 规则同时存在。
- 默认 XML 来自 `libvirt-daemon-config-network 10.0.0-2ubuntu8.19`，SHA-256 `9da5b0ff5481a4b26940af229bc211746e8f3e9c247aeb7331a9d513904ee92b`。包记录、有效 XML、规则及包来源均保留，来源登记仅对实际范围合规，不外推全部发行版。
- 实际观测安装单元 MemoryMax=1073741824、TasksMax=128；Docker HTTP 夹具最终 Running=true / OOMKilled=false，64 MiB、0.25 CPU、32 PIDs、只读、无 capabilities。9 项原生检查及 root/nonroot 两种权限 fixture 均实际 PASS。
- 所有 1547 个 tracked Go 路径集合与 Git 树一致，原字节逐个匹配；其中 first-party1156 的 LF 身份为 `fa5e581e8da27894937a9aa8d6ffb06495c659b59a84d35d01a9ffc09825faa7`。另外391项（third_party388、deploy2、scripts1）没有排除。独立离线复核通过。
- 前两轮 40196293002 / 40197761168 分别因缺少 httpd applet、初始 HTTP reset 在 VM 安装前终止；保留失败，工具修订后才运行成功源。第三轮初始 reset 也被日志保留，20 秒重试窗口和单次2秒的有限 readiness 处理该前置条件；根因未确认，不倒写为首轮成功。

## 审查与跨仓库联动

- 候选相对受信任控制器的安全敏感组 recorded=5；沿用精确源码的原候选盲审并核对身份。所有权测试和工具、原生 artifact、事实登记分别有独立聚焦复核，结果 PASS；聚焦修订不是新盲审。
- 复核提供商 Codex，其他提供商未提供入口，按 provider-unavailable 留痕；实际模型未暴露时记录 null，不推测。CF 未运行，不宣称全产品安全或全范围审计完成。
- 既有低级 MCP 元数据范围线索继续留仓外，由 KPanel MCP file maintainers 在2026-11-04复核，触发为授权根变更或稳定版质量复核；未修复细节不公开。
- OCR1.12.11 覆盖57/57，按 Git blob 精确复用已审来源和集成接缝；自由臂先留痕，`blind=false`、`constrained-only=unreported`。工具另按单文件修订覆盖，不把独立发现或已有源审查计算为 OCR 新收益。
- `scriptLinkageState=not-required`（无需发布脚本，不适用）；不新增 `kejilion.sh` 协议或重复脚本发布。内置已发布脚本仍为 `614d58cee228007012a2ec602cbdc8dbaeb7950d` / `ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9`。

## 多维质量

| 维度 | 状态 | 证据 | 边界 |
| --- | --- | --- | --- |
| 业务与互通 | 已验证 | 原生9项、API/契约回归、L3与公开容器 | 非全发行版或完整浏览器至客体控制台链路 |
| 网络与供应链安全 | 已验证 | 候选审查、来源/原字节核验、当轮扫描与SBOM/provenance | CF未运行，既有低级项保留，不作无未知漏洞保证 |
| 稳定、恢复与兼容 | 已验证 | 原生幂等、持久任务、公开重启/升级/恢复 | RC5桌面恢复一次未定位偶发仍OPEN；无长期WAN/全平台证明 |
| 性能与长期资源 | 已实现未实机验证 | 单次限额和健康有实际观测 | 未测低配RSS、长期soak或吞吐；不把限额观测当整项性能通过 |
| 用户体验与可访问性 | 已验证 | 冻结Mock26项及公开真实浏览器150项 | CSS200%非原生缩放；读屏/实体设备未验 |
| 数据、配置与回退 | 已验证 | 真实资源回读、默认保留磁盘、数据与通道测试 | 先备份并处置在途任务，切换镜像不回退宿主机libvirt资源 |

## 自动门禁与精确回执

发布流水线 [40201778872](https://github.com/kejilion/KPanel/actions/runs/40201778872) / attempt1 / success，控制器 `141b21204b72ba5afa6b09decdc65ff232c49216`、产品 `da7e266a5f6f716711961860d77e36f8130f3cb4`。最终公开资格运行 [40201778872](https://github.com/kejilion/KPanel/actions/runs/40201778872) / attempt1 / success；是否只读恢复按原运行和流程明细区分，不覆盖旧版本。

| 阶段 | job ID | 结论 |
| --- | --- | --- |
| inputs | 121194286872 | success |
| l3 / l3 | 121194334138 | success |
| candidate-ci / verify | 121194334169 | success |
| promote | 121197384625 | success |
| main-ci / verify | 121197439814 | success |
| publish / release | 121200572703 | success |
| publish / l3 | 121200573195 | skipped |
| publish / public-acceptance / verify | 121203714274 | success |
| publish / preview-completion | 121204181393 | success |
| publish / stable-completion | 121204182253 | skipped |

- 冻结 push CI 40200359539 通过；完整 L3 只由 GitHub-hosted `run-release-l3.mjs --github-hosted` 执行。源码web/go/deploy、核心race、扫描、部署/应用生命周期和 `image_e2e=pass` 通过；表中publish/l3的skipped是受控复用外层同一SHA的已通过L3，不是省略该门禁。
- L3 的27份原始流 bytes / SHA / exitCode全部核对；1156第一方、六平台actual/original闭包及 selected466 / production111 / standalone150清单与冻结观察完全一致。37项保守处置和2026-10-25至2026-11-08原窗口不变，不等同已观察14天。
- Linux web321文件：3172通过、6跳过；跳过为既有仅win32执行的PowerShell batch用例。先前Windows相同前端来源的完整3178项原件保留，不把Linux跳过改成通过。
- 原始说明正文6条更新/6条升级注意，面板按既有上限显示6/3；前三条优先呈现VM环境、备份与轻量节点限制。草稿和最终正文经同一实际运行时解析器验证。
- 依赖新增noVNC1.7.0及相应许可/声明；Go模块、Go/Node、基座/Action、扫描器与脚本pin不升级。既有TypeScript7和Node26类型声明采纳例外的2026-10-23到期告警保留，未续期，不能称全部最新；维护者须在对应兼容资格化与稳定版复核前处理。
- 本轮证据目录 `C:/GitHub/_release-evidence/kpanel-v128-rc6`；原始runner/toolchain、入口SHA及各组耗时见receipt/source/result，不用计划或退出码替代业务事实。

- `native-vm-40199079468-1`：artifact 12601521261，ZIP `sha256:c884c47fcfc33d5cb44df0ba496aac3eab0563b4362d67cd2f6863cec0b5e4c2`。
- `l3-40201778872-1`：artifact 12602601715，ZIP `sha256:f1556ac04ec9403b799973769561432d50fbcd9e17bd915f4362c021070c8936`。
- `snapshot-40201778872-1`：artifact 12603361591，ZIP `sha256:bed59aa5e9c9a9e18370420298e92901a656b38a576a60714e0791e246d7eba2`。
- `publication-40201778872-1`：artifact 12603501418，ZIP `sha256:3a6261fe4caa4429bc63b37da481a8ffcb74d34995eb65b97acc6f3011228f18`。
- `public-40201778872-1`：artifact 12603806532，ZIP `sha256:421ec1fc5c949faa0edb486d4c1aadb8ec6bc6041f9b594994f9f138edd843f2`。

## 浏览器、公开产物与更新

- 冻结Mock26项：VM10、网站监控7、文件活动9；源 `da7e266a5f6f716711961860d77e36f8130f3cb4`。宽1280/窄390、浅深、英语/繁体、键盘、失败与恢复、CSS200%重排；任务区诊断可在有界滚动中查看，辅助13px/主要状态与按钮14px。
- 最终VM首轮误设起始主题导致的超时保留；修订只固定 AppearanceSettings 输入并核对当前失败任务ID，原始断言保留。未知合成传输错误码的浏览器测试不冒充实际source_changed映射；后者由已有精确源码单测及完整门禁证明。
- 所有远程验证为登记 `github-hosted`。公开真实Panel/Agent容器、固定Runner/浏览器、夹具库存与测试CA均明确；不挂载宿主Docker/systemd到被测Panel/Agent。四份 `mocked=false`、errors/assetFailures为空，总150项：

- `kpanel-real-40201778872-1/results/initial/report.json`：48 项，passed。
- `kpanel-real-40201778872-1/results/restart/report.json`：26 项，passed。
- `kpanel-browser-published-40201778872-1/results/report.json`：45 项，passed。
- `kpanel-browser-upgrade-40201778872-1/results/report.json`：31 项，passed。

- GitHub Release ID `423469919`，draft=false/prerelease=true、非Latest；15附件下载/API摘要/磁盘回读一致，11条SHA256SUMS匹配。正文、发布信息API和更新弹窗的版本/digest、6条摘要与3条升级提示实际显示已验证。
- 版本/preview OCI index：`sha256:1c7e0f8c80cd9c09e15bc0139ce722bee358d8e97676625f5df8e947c2b37220`；双架构标签/来源/摘要一致（不等同ARM64实体原生验收）：

- `amd64`：`sha256:8324d9ac704ea137a2e8d42f7c2225d8a3d215714a3bc2b401b058077c4eda27`。
- `arm64`：`sha256:24aa9630c01460019e00719d1cd2a069f7659ca2bd3f994673f9ef769498b788`。

- Runner/sh/CJS配方与冻结产品及受信控制器 `141b21204b72ba5afa6b09decdc65ff232c49216` 原字节一致，没有有效覆盖；Runner实际ID `sha256:548f8cb5b5a8f010164f39c251e446cd84c9c3897e0fe76b2ca50045f6c5a3d0`。
- 稳定仍1.27.2；加入预览、自动安装、一次立即安装相互独立，退出不自动降级。公开新装、重启和从旧稳定容器升级旅程通过；不冒充原生宿主机systemd自更新、OpenRC、默认全新Node或完整跨主机传输验收。

## 生产与回滚

生产动作不适用（预览版禁止生产部署）。未连接prod-108/108或其他生产主机，未做生产备份、检查、部署、升级或回滚；没有生产完成时间。

上一预览 `v1.28.0-rc.5` / `sha256:565fe676f32d915273ca45eec11d843c8546f5187009a7b54708df417dd12144`，公共稳定 `v1.27.2` / `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`。回退前备份Panel/Agent数据并处置在途任务；旧版不提供VM新界面，但libvirt资源仍以宿主机为准，不能靠换旧镜像撤销资源写入。

## 交付节奏与流程异常

发车前滚动14天统计：4个正式版本、3个发布日、单日最多2；有生产证据部署0次。最近20个正式版本验收覆盖100%、生产完成证据50%、已报告变更失败1个/比例5%。缺失不推断成功，RC不计正式发布频率。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-25T16:00:44+08:00
- 候选冻结时间：2026-10-25T14:02:10.214+00:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

资格化工具/验证通道失败单列，不等同生产退化；红绿回归、可选诊断不自动计流程异常，不以重试成功抹去首轮。

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：17
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preflight/release-contract/late-ready-qualification",
    "position": "before-production-write",
    "count": 1,
    "impact": "Dedicated writer and exact clean baseline were checked, but release JSON ready was not run until after the first approved VM source merge; no external source write had occurred.",
    "recoveryEvidence": "Original source merge and writer checkpoints retained; ready qualified before further integration, with all source paths and final required hosted CI/L3 checks unchanged.",
    "permanentAction": "Release executor uses the complete contract checkpoint for remaining phases; release tooling maintainer reviews sequence before next preview. This local ordering correction is not a permanent policy/code fix. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "validation/local-web/shared-dependency-path-denied",
    "position": "before-production-write",
    "count": 1,
    "impact": "Full local standalone web check had36 failed suites and2 failed assertions after83 targeted tests passed. Focused diagnostic establishes Vite Denied ID for raw SVG assets outside the worktree via a reused node_modules junction; country-flag-dependent tests also fail.",
    "recoveryEvidence": "Original verify-change.log and full failure-diagnostic.log retained. Integrated owned release worktree now has its own canonical npm-ci dependencies; full integrated web and hosted final L2/L3 required.",
    "permanentAction": "Use actual in-worktree dependencies when raw asset loading enforces Vite filesystem roots; preserve the original junction and failed evidence. No filesystem allowlist or product assertion weakening. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/dependency-junction/removal-rejected",
    "position": "before-production-write",
    "count": 1,
    "impact": "Automatic approval rejected Remove-Item for the task-created dependency junction; removal did not run.",
    "recoveryEvidence": "Junction retained; shifted validation to the already-owned integrated release worktree with its own dependency installation, avoiding directory removal.",
    "permanentAction": "No bypass of rejected deletion. Existing release validation covers all approved changes; user code/evidence untouched. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/fixture-handoff/timestamp-precision",
    "position": "before-production-write",
    "count": 1,
    "impact": "First helper handoff rejected six-digit fractional timestamp; validator accepts at most milliseconds.",
    "recoveryEvidence": "Original handoff.json and contract-before-time-fix.json retained; format-only precision correction preserves original times, handoff-r2.json passed.",
    "permanentAction": "Release evidence executor uses millisecond timestamps per canonical parser, without extending expiry or altering successful raw logs. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "publication/fixture-candidate/unconditional-after-handoff",
    "position": "before-production-write",
    "count": 1,
    "impact": "Helper candidate push was invoked after a failed handoff because dependent tool execution was not gated on exit status. Candidate branch only; main/tag/release unchanged.",
    "recoveryEvidence": "Handoff-r2.json now passes for the exact pushed1eb9fd4f; candidate CI40189018202 runs before any main update. Initial failure and branch publication retained.",
    "permanentAction": "All subsequent external source writes use explicit verified-success guards and exact expected-SHA leases. This sequence correction is not claimed as a permanent repository mechanism fix. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/business-context/stale-before-tooling-gate",
    "position": "before-production-write",
    "count": 1,
    "impact": "Trusted native helper first full governance preflight stopped at the existing50-commit business-context threshold.",
    "recoveryEvidence": "vm-fixture/governance-native-final.log; refreshed actual business facts, threshold unchanged.",
    "permanentAction": "No policy or assertion weakening; retain original failure and qualify the exact next inputs. Remaining hosted/native/public gates still required. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/business-context/preview-version-header-rejected",
    "position": "before-production-write",
    "count": 1,
    "impact": "First refreshed facts header used an RC semantic version although the business baseline parser requires stable tag semantics; parser rejected it.",
    "recoveryEvidence": "Original commit8ace773fa4547e8c2424d8fa89606b473b4a5498 retained as Git object; corrected to actual stablev1.27.2 without changing threshold or published history.",
    "permanentAction": "No policy or assertion weakening; retain original failure and qualify the exact next inputs. Remaining hosted/native/public gates still required. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "validation/linux-worker/root-only-unit-fixture",
    "position": "before-production-write",
    "count": 1,
    "impact": "First integrated hosted CI40193645642 failed because a root-owned worker unit fixture ran as unprivileged Linux user.",
    "recoveryEvidence": "native-candidate-first-ci-job.log; preserve all old assertions in explicit root native execution; add mandatory unprivileged no-tool rejection test. Runtime policy unchanged.",
    "permanentAction": "Source98873aca keeps original root assertions, adds unprivileged no-tool rejection, runtime unchanged; native controller141b2120 requires explicit PASS for both ownership fixtures plus actual native integration. Native40199079468 independently verified.",
    "historicalReleases": []
  },
  {
    "fingerprint": "validation/local-governance/background-worker-start-timeout",
    "position": "before-production-write",
    "count": 1,
    "impact": "Full local361-test governance had360 pass/1 fail: an existing background browser runner fixture did not reach terminal state within10s.",
    "recoveryEvidence": "vm-fixture/governance-ownership.log preserved. Affected5 tests passed isolated; one bounded canonical full re-execution passed361/361 in governance-ownership-r2.log. Root cause not proven; no test timeout or assertions changed.",
    "permanentAction": "OPEN: KPanel release-tool maintainers review by2026-10-28. Capture worker state and host scheduling if the10s fixture recurs; identify prerequisite before another long replay. Current isolated5 and one full361 re-execution pass, as do exact hosted CI; no timeout/assertion change and no permanent root-cause closure claimed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/local-governance/node-option-rejected",
    "position": "before-production-write",
    "count": 1,
    "impact": "Attempt to reduce test concurrency using NODE_OPTIONS was rejected by Node before any gate executed.",
    "recoveryEvidence": "vm-fixture/governance-ownership-serial.log retained; option removed; canonical unmodified gate executed successfully. No reduced concurrency claim.",
    "permanentAction": "No policy or assertion weakening; retain original failure and qualify the exact next inputs. Remaining hosted/native/public gates still required. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/local-toolchain/go-path-missing",
    "position": "before-production-write",
    "count": 1,
    "impact": "New tool ownership ready qualification correctly failed missing Go in that process PATH.",
    "recoveryEvidence": "vm-fixture/ready-ownership.json retained; explicit Go1.27.3 PATH restored; ready/handoff both must pass before push.",
    "permanentAction": "No policy or assertion weakening; retain original failure and qualify the exact next inputs. Remaining hosted/native/public gates still required. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "native-qualification/docker-http-fixture/missing-applet",
    "position": "before-production-write",
    "count": 1,
    "impact": "Native hosted run40196293002 stopped before VM installation because the pinned Bash image BusyBox has no httpd applet. Unprivileged ownership test passed; no native runtime result exists.",
    "recoveryEvidence": "native-40196293002-artifact/native-host.log and docker-http.log; all original artifact bytes and hashes retained. Repair uses the existing repository-pinned Node runtime, exact HTTP response and unchanged container resource isolation.",
    "permanentAction": "Committed c2c6701c reuses the already pinned Node image; main141b2120 and native40199079468 qualify actual HTTP service plus Docker coexistence. No product/gate change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "native-qualification/docker-http-fixture/initial-connection-reset",
    "position": "before-production-write",
    "count": 1,
    "impact": "Native run40197761168 initial HTTP baseline returned curl56. Docker server log was empty, container state was not captured; no VM install or native success. Root cause unconfirmed.",
    "recoveryEvidence": "native-40197761168-artifact with original hashes; retain error. Add bounded initial readiness and Docker inspection before disposal; post-install host/peer assertions and isolation limits unchanged.",
    "permanentAction": "Committed3ea3567e/141b2120 adds finite startup readiness and Docker state capture; final source collector and independent review require healthy non-OOM64MiB/.25CPU/32PID/readOnly state. Reset cause remains unconfirmed. KPanel release-tool maintainers review by2026-10-28 if it recurs; exit requires lifecycle evidence identifying cause; later probes never retry.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/optional-browser-fixture-extension/automatic-review-rejected",
    "position": "before-production-write",
    "count": 1,
    "impact": "Automatic review rejected an optional command that would correct a synthetic browser error code and add a specific source_changed browser assertion. The command did not execute; existing reports and fixture file remain unchanged.",
    "recoveryEvidence": "Used read-only source inspection instead: actual backend emits source_changed and existing FilesView.test.ts:2453 asserts the specific message. The exact source is covered by the3178-test combined suite and successful hosted CI. Existing26 browser checks prove their stated generic/unknown-code behavior only; no new specific-mapping browser pass is claimed.",
    "permanentAction": "Do not bypass the rejected operation or replay it through another tool. Retain existing browser evidence scope and source regression proof; required native and publication gates unchanged. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/native-go-manifest/first-party-versus-all-tracked",
    "position": "before-production-write",
    "count": 1,
    "impact": "Native run40199079468 succeeded; the first read-only collector rejected its1547-row tracked-Go manifest because it incorrectly assumed total tracked equals1156 first-party.",
    "recoveryEvidence": "collect-native-before-source-count-fix.py retained; collector now checks all1547 Git paths/raw bytes and separately1156 cmd/internal normalized identity. Independent offline ZIP verification passed; actual native run was not repeated.",
    "permanentAction": "Corrected collector checks all1547 tracked paths/raw bytes in one Git batch and separately1156 normalized cmd/internal identity. Independent offline review passes. Native run unchanged; no391 paths omitted or tests relaxed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/final-mock-preview-batch/automatic-review-rejected",
    "position": "before-production-write",
    "count": 1,
    "impact": "Automatic review rejected the bundled final-preview start/tests command before execution, returning only blocked by policy.",
    "recoveryEvidence": "Established the mock preview separately and inspected exact source/PIDs and actual4176 URL. The previously assumed4174 belongs outside current release and was left untouched. Separate controlled tests use the actual manifest target, not a replay of the rejected bundle.",
    "permanentAction": "Use qualified exact inputs and preserve original failures. Local driver/collector correction does not change product policy or release gate; final source CI/L3/public qualification remain required. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is a correctly ordered, exact-input preflight or the documented safe alternative on its next applicable use. This is not a claimed permanent policy change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "mock-ui/vm-theme-driver/initial-theme-assumption",
    "position": "before-production-write",
    "count": 1,
    "impact": "First final VM driver timed out after toggling an already-dark page while waiting for dark; the initial desktop-light.png actually shows dark. User-preferences theme override was not the AppearanceSettings contract.",
    "recoveryEvidence": "Original driver and partial final/vm screenshots retained. New driver fixes actual appearance GET to light, asserts initial light, retains dark-toggle/mobile checks and binds failure detail to this run request ID to avoid stale history. Final vm-r2 passed10 checks. No product change or weakened assertion.",
    "permanentAction": "Frozen R2 driver supplies actual AppearanceSettings light fixture and asserts light before exercising dark toggle; current failed-request ID must match dialog. Original driver/failed images retained; all10 VM cases pass on frozen source, no product changes.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

最近五个正式版本原始指纹比较见historical-incident-comparison.json，不改写旧记录；本次同指纹历史重现按实际数组填写。

## 未验证风险与后续准入

- 实际原生范围只为上述Ubuntu24.04.5/amd64与Debian12客体；其他发行版/防火墙组合、ARM64实体、Windows客体、完整浏览器至客体控制台链路、低配/长期负载仍需各自验证。
- 文件整理不提供断点续传；未提交批次依赖页面，既有慢网络/长耗时后端超时边界未改变。不能把Mock与单测当真实跨主机/长时传输已修复。
- 已配对节点无须为本轮重配；默认稳定轻量Node安装器仍不能消费新格式入网命令。节点维护者在2026-10-28和正式1.28默认入口互通前复核关闭。
- RC5桌面恢复未定位偶发保持OPEN；桌面/偏好与发布夹具维护者在2026-10-28、稳定1.28前获取时序及红绿重现，不因本轮通过关闭，复现同指纹时不得继续盲重试。
- CF/稳定版前全范围补漏、本轮未覆盖的实体设备与读屏不作通过声明。源/修复/工具代码与证据保留，仅停止已核验自有预览，无删除磁盘数据。
