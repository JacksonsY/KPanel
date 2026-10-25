# KPanel v1.28.0-rc.5 发布验收记录

日期：2026-10-25；发布级别：L3；`releaseChannel=preview`；`releaseTrain=1.28.0`。

候选 / 标签：`140967a0c5fba18d3bc99567651d855e63df3618` / `v1.28.0-rc.5`；tree `45b32f03a68ee590e13887a99738bb9dabf07c5d`。

产物已发布，生产未部署。GitHub Latest / Docker latest 保持 v1.27.2，本版仅更新 prerelease / preview。

## 发布画像、范围与未纳入内容

- 三组批准功能：Win98/XP 表面、图标和第三方外框（16d9dd18）；集群全局/单机终端与文件开关、搜索和批量设置（8d684ed6）；桌面压缩包关闭/Escape 返回桌面且保留经典文件浏览目录（83d19409）。
- 前两组在 RC4 冻结前已有本地候选，但未被当轮盘点纳入；本轮补发，保留 RC4 历史原件，不把该遗漏解释为源代码未提交。
- 发布复核发现备份授权恢复缺陷，独立修复分支 bb3e3c5d 已纳入：同节点恢复以当前有效控制器密钥/权限为上界，已删除及 GC 后的旧配对不复活；拒绝歧义身份字段，清除大小写变体的一次性配对凭据。
- 变更面为前后端策略、既有 JSON 备份恢复和展示；新增 `cluster-remote-access.json` 有界配置并纳入备份，不修改系统 SSH、宿主机外联模板或已有节点的线路凭据协议。风险等级 L3。
- 普通升级保留现有配对，新开关默认开启。策略只约束当前中心；全局关闭优先，重新开启保留单机选择，不复活旧命令/传输。已执行操作不撤回，失联远端清理可待确认。
- 不纳入稳定发布、生产部署、未提交的外来备份导入实现、终端补充测试、旧 RC8 文档、其他进行中候选或默认轻量节点安装器修复。

## 来源、分支归档与本地处置

预览列车 `release/v1.28.0-candidate` 保留在 `140967a0c5fba18d3bc99567651d855e63df3618`；annotated tag 对象 `34f5a4a369db2cc41e0824a1a6398be07cb518f5` 的 peeled commit 与候选一致。

- `fix/windows-third-party-icon-size` / `16d9dd181de5e030a8082dc62a4537e230f6d649` → `archive/fix/windows-third-party-icon-size`；远端精确回读、本地 clean 与解除旧 upstream 完成，工作树保留。
- `feature/cluster-settings` / `8d684ed61b2824988f283b87dcb648240f65596a` → `archive/feature/cluster-settings`；远端精确回读、本地 clean 与解除旧 upstream 完成，工作树保留。
- `fix/desktop-archive-close` / `83d19409344e0aaecb3890a9d8c06cbdff1a859f` → `archive/fix/desktop-archive-close`；远端精确回读、本地 clean 与解除旧 upstream 完成，工作树保留。
- `fix/rc5-restore-revocation` / `bb3e3c5d4c799267014081cb9bc2a06a4673a323` → `archive/fix/rc5-restore-revocation`；远端精确回读、本地 clean 与解除旧 upstream 完成，工作树保留。

- 测试夹具前置：`fix/rc5-cluster-preview-fixture` / `ce23cccd954246d70dd8406fa4469afd6edd0f25` → `archive/fix/rc5-cluster-preview-fixture`；候选 CI 40174997938、主线 CI 40175545424 成功后已原子归档并回读，工作树保留。
- 用户授权跨项目清理：检查 14 个 Git 仓库，6 条无工作树占用且已进入旧版本的本地分支改名归档；停止 3 组旧预览共 6 个服务。另按本轮交付收尾停止压缩包来源预览及最终 RC5 预览，各 2 个服务，manifest 原件保留。未删除源码、证据或工作树，未移除未提交或未知归属内容，未把在用 Codex/浏览器进程当残留关闭。
- 归档不等于生产上线。未知与未提交内容继续保留待处置；未执行磁盘内容删除，不作释放空间收益声明。

## 外部审查与修复交付

- 候选 23 个安全组 recorded；历史无效 skip trailer 仍为告警，不作通过依据。新增交叠范围为 backup_data、backup_restore、cluster 路由和 mcp_cluster，按固定提示由与实现分离的只读快照会话审查。
- 本轮原始线索 H2/M1/L1；三项备份相关缺陷已核实修复，三份定向回归均有失败前与通过后记录。修订源码由独立审查者复核无剩余可信发现；修订复核明确不是全新盲审。
- 一项既有低级 MCP 元数据范围线索保留仓外，负责人 KPanel MCP file maintainers，复核期限 2026-11-04，触发为进一步修改文件授权根或稳定版质量复核。未修复细节不公开，不宣称全仓安全。
- 夹具固定矩阵独立审查 PASS WITH FOLLOW-UP，阻断/重要项 0，一般项 2：Mock 仅证明设置和列表，不能证明真实拒绝/清理或资源上限；维护者在扩展对应断言前补齐。
- OCR 1.12.11：先留自由臂，再取规则；最终 67/67 路径及 39 排除项逐项覆盖，夹具另 1/1。`blind=false`、`constrained-only=unreported`，不把独立安全发现算 OCR 收益。固定规则和阈值未变。
- 完整 CF 审计及稳定版前全范围补漏未执行；本次为 RC，不伪装成 CF 通过。源码已进入本 RC；稳定版和生产交付均不适用。

## 跨仓库联动判定

`scriptLinkageState=not-required`（无需发布脚本，不适用）。本轮不修改脚本协议、安装/更新路径或镜像内脚本；继续固定已发布兼容基线 `614d58cee228007012a2ec602cbdc8dbaeb7950d`，SHA-256 `ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9`。无脚本候选、无重复脚本发布、无永久规范/工作流修改。

## 多维质量结论

| 维度 | 状态 | 证据 | 边界 |
| --- | --- | --- | --- |
| 业务与双端互通 | 已验证 | 当前 Go、Linux L2/L3 与公开容器业务检查 | 不等同真实公网多主机完整互通 |
| 网络与供应链安全 | 已验证 | 独立审查、回归、当轮源码/镜像扫描、SBOM/provenance | CF 未运行，既有低级项保留，不作无漏洞结论 |
| 稳定、恢复与兼容 | 已验证 | 备份恢复回归、核心 race、公开重启/升级/故障恢复 | 未做长期 WAN soak 或全发行版灾备 |
| 性能与资源预算 | 已实现未实机验证 | 有界侧车、主机偏好、取消生命周期与 CAS 测试 | 未测低配 RSS 或长时负载 |
| 体验与可访问性 | 已验证 | 当前 Mock 22 项、公开真实浏览器 150 项 | 200% 为等效 CSS 视口/DPR，实体设备与读屏未测 |
| 数据、配置与迁移 | 已验证 | 默认开关、旧备份保留目的端策略、授权恢复上界 | 回退旧版会失去新策略；先备份，不靠换镜像恢复数据 |

## 自动门禁与精确证据

发布流水线：[40177032954](https://github.com/kejilion/KPanel/actions/runs/40177032954)，attempt 1，conclusion=failure；其发布作业成功，原公开浏览器作业失败。只读公开复验 [40180170285](https://github.com/kejilion/KPanel/actions/runs/40180170285) / attempt 1，conclusion=success，未重建或重发产品。

| 阶段 | job ID | 结论 |
| --- | --- | --- |
| inputs | 121119829434 | success |
| l3 / l3 | 121119871948 | success |
| candidate-ci / verify | 121119871981 | success |
| promote | 121122222452 | success |
| main-ci / verify | 121122278448 | success |
| publish / release | 121124740403 | success |
| publish / l3 | 121124741083 | skipped |
| publish / public-acceptance / verify | 121127279336 | failure |
| publish / stable-completion | 121127666212 | skipped |
| publish / preview-completion | 121127666288 | skipped |

- 当前 push CI 40175726303 成功，显式 L2 floor；最初相同产品树 a1f74f7a 的 CI 40173363329 亦保留，不跨 SHA 替代当前完整门禁。
- L3 唯一外层为 `run-release-l3.mjs --github-hosted`；源码 web/go/deploy 三组、核心 race、安全、native 镜像与安装生命周期、`image_e2e=pass` 均通过；27 份原始流逐字节摘要核对。
- 本轮 Go1.27.3 六目标 actual/original 模块闭包、1139 个第一方 Go 文件与冻结 Git 字节一致；selected/production/standalone 模块清单及 37 项保守处置保留，原观察/到期窗口不延长。
- 草稿及最终发布说明通过同一实际运行时解析器：6 条摘要、3 条升级提示，包含预览边界和默认全新 Node 安装限制。
- 首次 dispatch 40174337534 在输入处因未集成 Mock 夹具失败，L3/main/tag/镜像均未开始；原日志保留。夹具完成独立审查、359 项本地治理及同 SHA 候选/主线 CI 后重新冻结，产品树逐字节不变；重新执行当前 CI/L3，未覆盖既有版本。
- 仓外原件：`C:/GitHub/_release-evidence/kpanel-v128-rc5`。原始 runner image/version、Go/Node、脚本摘要及各分组耗时以当轮 receipt/source/result.json 为准。

- `l3-40177032954-1`：artifact 12592832666，ZIP `sha256:b0e5ec8f48f6198cff9ab40accd1eea65c7e3aa85b5d83e85aee41e10a534301`。
- `snapshot-40177032954-1`：artifact 12593701090，ZIP `sha256:e607205a341f8e510c2ac991dd12f1089991611c98f032f61a33fc8603d5a152`。
- `publication-40177032954-1`：artifact 12593391564，ZIP `sha256:69dc372cf13d831de6b7c2be07eca52cbb89ef1ee214fd35762a18d15ad46f39`。
- `public-40180170285-1`：artifact 12593602733，ZIP `sha256:fe5a8aef621ae31bb9c63559b12fe11c89b3b868ec12707ddb53adcb3999ff4e`。

公开初轮已有 initial48 / restart26 通过，随后浏览器主动更新检查收到官方 GitHub preview endpoint HTTP403；报告未保留上游响应头，具体限流原因未确认，不能当作已证实限流。保留原 ZIP 和完整 job 原始日志，在新 hosted runner 使用原始 run/attempt 的 snapshot 做只读复验。原失败结论不改写，最终公开资格以独立复验回执为准。第一次只读复验 40179816935 在重启桌面恢复处 30 秒超时：服务端偏好仍含英文和一个窗口，截图却为 classic 壳层，尚未确认是产品恢复还是夹具时序问题。原失败报告/截图保留，进一步 40180170285 新 hosted runner 完整复验；后续绿灯不关闭此偶发项。桌面偏好和发布夹具维护者须在正式 1.28 前调查，2026-10-28 复核，要求网络/偏好时序及失败前/通过后重现；相同指纹再现不得继续盲重试。

GitHub CLI 日志下载的 HTTP2 CANCEL 亦保留，使用现有连接器取得完整原件；未放宽断言或产品证书校验。

## 依赖与技术栈

本轮未升级依赖、Go/Node、基础镜像、Action、扫描器或受管脚本；root package 仅同步版本。实际当轮扫描在上述 CI/L3 和 Release job，源码范围与固定构建身份重新核对，不改 37 项保守处置或观察窗口。

依赖报告 40173363188 因 TypeScript 7 与 Node 26 类型声明两项既有采纳例外在 2026-10-23 到期而失败；保留原失败，不延长期限，不把它说成全部最新或一次安全扫描通过。维护责任 KPanel base maintenance，需在 Vue/TS7 与获批 Node26 LTS 路径兼容资格化后处理；不纳入本次产品范围。主线关联运行 40174133840 的 security-advisories 通过，但 report 为 skipped，仅证明当时公告审计，不能抵消上述时效报告失败。实际终态保存在同一仓外证据目录。

## 隔离浏览器、公开产物与更新通道

- 环境为登记的 `github-hosted`，未使用自有主机、WSL 发布或生产。临时真实 Panel/Agent 容器使用固定测试 Runner/浏览器配方；夹具库存、测试 CA 明确标注，不挂载宿主 Docker/systemd 到被测 Panel/Agent。
- 当前 Mock UI 22 项：集群 11、英语/繁体 2、视觉/压缩包 9；宽 1280、390/320 窄屏、100%/200% 等效缩放，浅/深主题、键盘、409/503/部分保存恢复；桌面文字样本最小 12px，集群弹窗样本最小 13px。
- 压缩包实际内层关闭按钮与 Escape 均等待外层窗口退出后确认无文件管理窗口；经典文件管理器内关闭仍保留目录。Mock 不作为真实访问控制证据。
- 公开层四份报告 `mocked=false`、errors/assetFailures 为空，总 150 项：

- `kpanel-real-40180170285-1/results/initial/report.json`：48 项，passed。
- `kpanel-real-40180170285-1/results/restart/report.json`：26 项，passed。
- `kpanel-browser-published-40180170285-1/results/report.json`：45 项，passed。
- `kpanel-browser-upgrade-40180170285-1/results/report.json`：31 项，passed。

- 公开 Release ID `423276117`，draft=false、prerelease=true，非 GitHub Latest；15 个附件下载/API digest/磁盘回读与冻结清单一致，11 项 SHA256SUMS 通过。正文、实际发布信息 API 和更新弹窗均绑定 RC5 与同一摘要，6 条摘要/3 条升级提示真实显示。
- 版本与 preview OCI index：`sha256:565fe676f32d915273ca45eec11d843c8546f5187009a7b54708df417dd12144`；双架构 source/version 与固定脚本标签匹配：

- `amd64`：`sha256:fc7278a67233749b6181c7959561b615b7c815bb1e0a03cc90d0ede9c5c54bee`。
- `arm64`：`sha256:51d3480e6b2222f3164e0b0fc89b32225a1c08834a4754725e79f63de09f46e8`。

- Runner/sh/CJS 三份配方均与冻结产品及受信任 controller `140967a0c5fba18d3bc99567651d855e63df3618` 逐字节匹配，无有效覆盖；测试 Runner 实际 ID `sha256:abf1eaff5370e9981e95e3eea11bfb16aa99c7a55f4d4ecfc2b040cff13a784c`。保持产品镜像不可变。
- 稳定来源仍返回 1.27.2；加入预览、自动安装、立即安装相互独立；退出不自动降级。公开新装、重启与从旧稳定镜像升级旅程通过；不冒充真实宿主机 PID1/systemd 安装、OpenRC 自动更新或全新 Node 默认入口测试。

## 生产安全核对与回滚

生产动作不适用（预览版禁止生产部署）。未连接 prod-108/108 或其他生产主机，未做生产备份、健康检查、部署、升级或回滚；没有生产完成时间。

回滚点为 v1.28.0-rc.4 / `sha256:850f4c5c56dbbc0c6f450bee672bad577b1b0a0b9ffd79ca4ae537c2e0311211`。升级前保存 Panel/Agent 数据，处置在途任务后显式选旧镜像并验证数据恢复；旧版不识别新策略，不能指望旧版维持新访问限制。默认稳定入口仍 v1.27.2 / `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`，不自动降级。

## 交付节奏与流程异常

滚动 14 天：4 个正式版本、3 个发布日，单日最多 2；有生产证据的部署 0 次。最近 20 个正式版本验收覆盖 100%，生产完成证据 50%，已报告变更失败 1 个、比例 5%。缺失生产记录不推断为成功；RC 不计正式发布频率。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-24T09:04:39+08:00
- 候选冻结时间：2026-10-25T08:12:32.525Z
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

本 RC 补齐前轮遗漏并纳入新增批准内容；未重发或覆盖已公开的不可变版本。正常前置审查中的红/绿回归不计流程异常；工具和验证入口失败如实单列，不计为生产退化。

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：12
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preparation/tool-orchestration/javascript-syntax",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial batched read-only orchestration rejected before executing any nested tool.",
    "recoveryEvidence": "Corrected JavaScript invocation then completed source/worktree reads; no repository or publication mutation occurred.",
    "permanentAction": "Release executor uses validated object argument syntax; no gate or policy changed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/contract-reader/default-encoding",
    "position": "before-production-write",
    "count": 1,
    "impact": "First contract-copy read used Windows default GBK and failed before producing a contract.",
    "recoveryEvidence": "Explicit PYTHONUTF8=1 used for all subsequent contract and evidence reads/writes; ready/handoff passed.",
    "permanentAction": "UTF-8 is explicit in this release executor, preserving original failed attempt and successful qualified contract.",
    "historicalReleases": []
  },
  {
    "fingerprint": "mock-ui/visual-archive-driver/desktop-entry-mode",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local driver prerequisite or locator/lifecycle assertion failed; product bytes unchanged.",
    "recoveryEvidence": "Original visual-archive-browser r1/r2/r3/r4 logs and reports retained; r5 uses observed application entry/ARIA controls and waits for actual window detachment, preserving all behavior assertions;9checks passed.",
    "permanentAction": "Current release-owned driver uses canonical desktop mode, observed ARIA labels and condition-based lifecycle waits. No product or assertion threshold weakened.",
    "historicalReleases": []
  },
  {
    "fingerprint": "mock-ui/visual-archive-driver/shortcut-accessible-label",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local driver prerequisite or locator/lifecycle assertion failed; product bytes unchanged.",
    "recoveryEvidence": "Original visual-archive-browser r1/r2/r3/r4 logs and reports retained; r5 uses observed application entry/ARIA controls and waits for actual window detachment, preserving all behavior assertions;9checks passed.",
    "permanentAction": "Current release-owned driver uses canonical desktop mode, observed ARIA labels and condition-based lifecycle waits. No product or assertion threshold weakened.",
    "historicalReleases": []
  },
  {
    "fingerprint": "mock-ui/visual-archive-driver/dialog-accessible-label",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local driver prerequisite or locator/lifecycle assertion failed; product bytes unchanged.",
    "recoveryEvidence": "Original visual-archive-browser r1/r2/r3/r4 logs and reports retained; r5 uses observed application entry/ARIA controls and waits for actual window detachment, preserving all behavior assertions;9checks passed.",
    "permanentAction": "Current release-owned driver uses canonical desktop mode, observed ARIA labels and condition-based lifecycle waits. No product or assertion threshold weakened.",
    "historicalReleases": []
  },
  {
    "fingerprint": "mock-ui/visual-archive-driver/asynchronous-window-close",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local driver prerequisite or locator/lifecycle assertion failed; product bytes unchanged.",
    "recoveryEvidence": "Original visual-archive-browser r1/r2/r3/r4 logs and reports retained; r5 uses observed application entry/ARIA controls and waits for actual window detachment, preserving all behavior assertions;9checks passed.",
    "permanentAction": "Current release-owned driver uses canonical desktop mode, observed ARIA labels and condition-based lifecycle waits. No product or assertion threshold weakened.",
    "historicalReleases": []
  },
  {
    "fingerprint": "dispatch/github-preview-release/untrusted-mock-fixture",
    "position": "before-production-write",
    "count": 1,
    "impact": "Publication40174337534 stopped in inputs; no L3/main/tag/product artifact. Additive UI mock fixture differed from trusted main.",
    "recoveryEvidence": "publication-inputs-failure.log retained; one-file exact mock blob independently reviewed, local governance passed; helper CI/main qualification and product refreeze required.",
    "permanentAction": "Integrate only exact qualified fixture and run canonical automation tree comparison before redispatch. Existing fail-closed publish guard remains unchanged.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/refreeze-helper/canonical-function-binding",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local canonical tree comparator initially omitted its ref function argument and rejected before changing freeze metadata or pushing a candidate.",
    "recoveryEvidence": "Corrected explicit git/ref binding; old f83f29ae/a1f74f7a negative control rejects and ce23cccd/140967a0 positive comparison passes in refrozen-source-qualification.json.",
    "permanentAction": "Release-owned pre-dispatch comparison executes only the unchanged published canonical selection body with an ls-tree-only Git adapter and explicit parameters; original fail-closed publish guard unchanged.",
    "historicalReleases": []
  },
  {
    "fingerprint": "closeout/release-metrics/unsupported-cli-option",
    "position": "before-production-write",
    "count": 1,
    "impact": "First local metrics collection used unsupported --json and produced no report; no repository/publication change.",
    "recoveryEvidence": "Canonical parser inspected; --format json succeeds and raw output retained in release-metrics-before-closeout.json.",
    "permanentAction": "Use the existing explicit --format interface in this release executor; no product or gate change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-browser/github-release-source/upstream-http-403",
    "position": "before-production-write",
    "count": 1,
    "impact": "Published-browser manual release check received GitHub preview endpoint HTTP403 after direct preview and stable metadata retrieval had succeeded. Initial48/restart26 checks passed; published journey stopped at39; upgrade journey did not run.",
    "recoveryEvidence": "Original public-40177032954-1.zip and public-initial-job-raw.log retained; report.checked.lastError explicitly identifies upstream403. No response headers retained, so quota/rate-limit cause unconfirmed. Bounded read-only recovery40179816935 uses same immutable product and original snapshot.",
    "permanentAction": "Use existing read-only recovery workflow once on a fresh hosted runner. KPanel release-tool maintainers review by2026-10-27; exit only with complete public qualification and continued official endpoint availability, or investigate recurrence without blind outer retries. No product, fixture or assertion change.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/github-cli-log/http2-stream-cancel",
    "position": "before-production-write",
    "count": 1,
    "impact": "GitHub CLI failed-log retrieval returned HTTP2 stream CANCEL; its empty output is not evidence.",
    "recoveryEvidence": "GitHub connector fetch_workflow_job_logs returned the complete166202-character original public job log, retained as public-initial-job-raw.log; underlying public artifact independently downloaded and hashed.",
    "permanentAction": "Use existing decoded job-log connector for this failed transport. Release tooling maintainer reviews transport stability by2026-10-27; exit when required raw logs are retained with exact job identity. No product/gate changes or assertion weakening.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-browser/desktop-restoration/intermittent-ui-timeout",
    "position": "before-production-write",
    "count": 1,
    "impact": "Read-only recovery40179816935 passed initial48 checks but restart journey timed out waiting30s for desktop-window after16 checks. Persisted preference API returned expected locale and one window; screenshot shows classic shell. No pageerror reported. Distinct from first upstream403.",
    "recoveryEvidence": "Original recovery ZIP, exact job log and recovery-restart-failure.png retained. Root cause unconfirmed; source preference hydration and fixture ordering inspected. One further fresh-host replay40180170285 collects full end-to-end evidence; any success does not close this intermittent risk.",
    "permanentAction": "OPEN: KPanel desktop/preferences and release-fixture maintainers investigate before stable1.28, review by2026-10-28. Exit requires captured preference/network ordering and a targeted failing-before/passing-after reproducer. If same fingerprint recurs, stop blind retries and repair only the identified product or fixture cause. Product and assertions unchanged.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

最近五个正式版本原始指纹比较保存在 historical-incident-comparison.json；不批量改写历史记录，不把重试成功抹去首轮失败。

## 遗留风险与后续准入

- 重启桌面恢复有一次未定位的偶发 UI 超时，保留为 OPEN，不因后续复验通过而关闭；正式版前须按上文重现和修复。
- 默认稳定 Node 安装器不能直接消费新格式入网命令；全新或重新入网仍需隔离资格化。已有配对不因本次策略功能要求重建。节点维护者须在正式 1.28 默认入口互通验收前关闭此项，沿用 RC4 的 2026-10-28 复核要求。
- 一项既有低级元数据线索、Mock 的两项一般局限、真实公网多主机/长时负载、实体手机、读屏和原生浏览器缩放未完成；不据 Mock 或源码审查宣称生产全覆盖。
- 稳定版前继续执行固定质量补漏与各已登记兼容准入；本轮无剩余 confirmed 高/中缺陷，无新增脚本联动要求，预览允许保留明确边界。
- 本地来源、修复、夹具及验收工作树保留代码/证据；仅停止本任务不再需要的预览服务。无删除内容的磁盘净释放指标不适用。
