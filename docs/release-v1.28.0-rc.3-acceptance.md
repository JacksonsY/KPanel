# KPanel v1.28.0-rc.3 发布验收记录

日期：2026-10-24

发布级别：L3；`releaseChannel=preview`；`releaseTrain=1.28.0`。

候选 / 标签：`a4c812f477af204f10dd484c2fd5d00899247d06` / `v1.28.0-rc.3`；tree `220cddea3abfcd254feb40fc1c2646a3d01fafaa`。

产物已发布，生产未部署。稳定 GitHub Latest / Docker latest 保持 v1.27.2；本版只更新 prerelease / preview。

## 发布范围与画像

本轮纳入已交付 WAF 消费适配 `c3e6a1322b668a7c15fa33f190c17d85ec030849`（最终代码 `45f0c6b51b192d5066d4a7504763f27cab366286`），批准主线 `1efc74ab3b78193b705300bf2a212e02a111d3cb`；组装版本、扫描身份和累计业务基线后冻结上述候选。候选 push 的实际 before 是 `37427b4cef550e9be3a45a61c67f8f89d0bdc021`，不冒充批准基线。

- 用户可见更新两项：统一使用已发布 WAF 配置脚本；状态明确区分拦截、关闭、检测、按站点、旧配置和待核验。未知值不显示正常拦截，旧布尔字段保持兼容。
- 宿主配置由既有固定 `waf-on/waf-off` 动作与同源工具负责；KPanel 仅增加可选 `wafMode` 和展示，不增加常驻 WAF 服务、权限入口、依赖或数据库迁移。
- 差异共15文件。组装提交 `2e21cf00`、业务基线 `fa329377`、复核检查点 `a4c812f4`；复核检查点不改变产品 tree。
- 本轮不纳入其他候选、稳定发布、生产部署、上游重复发布及无关依赖升级。首次启用仍要求兼容 ModSecurity/CRS 镜像和 Python3.11+；检测不执行拦截，新部署不承诺累计自动封禁，旧镜像不静默替换。
- 发布说明解析器实际核验2条更新、3条升级提示；公开正文、面板发布信息接口和更新弹窗的实际结果见下方公开报告。

## 跨仓库联动与来源

`scriptLinkageState=coupled`，变更集 `shared-waf-configuration-20261023`。上游已先发布，本轮没有对上游仓库写入。

- 内置 `kejilion/sh@614d58cee228007012a2ec602cbdc8dbaeb7950d`，根脚本 SHA-256 `ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9`。
- 固定 `kejilion/nginx@8773850871f991b027f62841fde2795121251620` 六资源；发布接管逐文件复算6/6摘要，并核对两个来源是已发布 main 的祖先。此前 Node 防降级补丁 `24f65f9f` 是本次内置脚本祖先，generation7仍在。
- 已发布兼容 Nginx 镜像 index `sha256:7827d4c2e19bfce579f6fb4ce94884a126b5ab8ea74943e5448fa4c2b3d7f894`；上游发布工作流39974763991成功。原生矩阵/正式服试点属于上游历史声明，不计为本轮 KPanel 生产或实机验证。
- 源候选文档声明代码45f0c6b5本地完整L2通过；接管未找到其完整原始本地回执，未将该声明冒充独立验证。本轮以下述冻结候选的实际云端 CI/L3 与公开产物验收重新资格化。

## 审查与供应链身份

组装独立复核 `PASS WITH FOLLOW-UP`，报告 SHA-256 `040ec244c0674a76668658c281bb6a2b84751d6a281337049a3a179c54a74e73`；其他提供商入口不可用，采用与实现分离的干净会话，trailer记载 `fallback=provider-unavailable`。后续要求的精确候选 CI/L3、源码扫描和预览均已按下文完成。

OCR1.12.11先保存自由臂，再圈选与解析规则，范围1efc74a..fa32937：11/11可评审路径、4个排除项人工核对；零成立问题。`blind=false`、`constrained-only=unreported`，不计为有效盲跑增量，不作工具有效性结论。自由臂误录时间原件与文件时间更正保留。

相对批准基线 `security_review=not-applicable groups=0`；本轮CF安全盲审为not-run，不是全仓或上游WAF审计，不关闭历史安全待办。此前安全修复已进入RC2，稳定版及生产交付状态不因此改变。

1022个 first-party Go 源文件，仅可选结构字段和版本文字变化；没有新Go路径/import/第三方模块。canonical L3实际重算六目标 actual/original，27条rawEvidence逐条核对原始bytes/hash/exitCode，inventory与本地身份观察吻合：

- firstPartySource：`f1069d1fa8b965175b4d4d5851644a70df443289da0b3973b43d56cf716601db`。
- closure：`f7c89d5e862888205deb3af9331784042142e11eaec2d3e420b349f750e8ff27`；生产110模块，选中图与闭包不变。
- Dockerfile：`2dcc56a68487566a9a7273b2269cf371b5dc0d9b6d5c08ed7cca304afba8eb38`，仅内置脚本pin改变。
- 37项历史处置、原2026-10-11观察与2026-10-25到期均未延长；源码身份观察不冒充扫描成功，实际扫描以同SHA CI/L3回执为准。

## 自动门禁与云端回执

候选push CI [39996547851](https://github.com/kejilion/KPanel/actions/runs/39996547851)，job120561442680：已验证success。原日志证明实际before37427b4c及`level=2`，Go1.27.3 / Node24.22.1；Web314文件、3115通过、6跳过。全量测试、构建、race、govuln、npm/source扫描与脚本生命周期通过，不用空复核提交规避L2。

主发布 [run39998970817/attempt1](https://github.com/kejilion/KPanel/actions/runs/39998970817)，可信controller `1efc74ab3b78193b705300bf2a212e02a111d3cb`：L3、候选CI、主线CI及产物发布success；原运行整体failure仅因首次公开验收夹具，失败原件保留，未重跑publication或覆盖产物：

| 阶段 | job ID | 结果 |
| --- | --- | --- |
| inputs | 120569788913 | 已验证：success |
| l3 / l3 | 120569906091 | 已验证：success |
| candidate-ci / verify | 120569906122 | 已验证：success |
| promote | 120578189106 | 已验证：success |
| main-ci / verify | 120578311014 | 已验证：success |
| publish / release | 120586214497 | 已验证：success |
| publish / l3 | 120586216225 | 外层L3已成功，内部重复L3按配置skip |
| publish / public-acceptance / verify | 120593134389 | 原失败保留，随后只读恢复 |
| publish / preview-completion | 120593537361 | 原公开验收失败后按条件skip；恢复后人工完成收尾 |
| publish / stable-completion | 120593538415 | 预览版不执行稳定候选归档 |

L3唯一外层 `node scripts/run-release-l3.mjs --github-hosted`，实际`make verify-release`及web/go/deploy三组均退出0、身份未变，镜像`image_e2e=pass`。入口摘要 `43e57f4fd321846204020d6cf27eb75961f0aefa1284a9187e786601704c2d40`；runner `ubuntu24/20261012.334.1`，`go version go1.27.3 linux/amd64`、Node `v24.22.1`、Docker `28.0.4`。没有使用自有服务器或WSL。

## 依赖与技术栈

同候选依赖报告39996547912/job120561442184成功，2026-10-23T17:38Z完整9/9；32直接/基座信号（8兼容patch、19minor、5major/base），295传递归属信号，emergency-security0。每日安全工作流39996818698/job120562395749在同一冻结SHA成功；govuln当前调用路径及两套npm审计通过，源码三范围按既有处置策略扫描。

本轮只采用上述已发布受管脚本，不采用其他Go/Web依赖、工具链、基础镜像、Action或扫描器候选。EOL current至2026-10-28T23:59:59.999Z；TypeScript/@types/node现有例外reviewDate2026-10-23在报告UTC日期内active，本轮不续期。未采用信号继续由基础维护负责人按既有期限独立处置，不以本轮报告重置首次检测时间或维护期限。

## 多维质量与本地预览

| 维度 | 状态 | 证据和边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 同SHA全量测试、结构协议兼容及公开容器旅程；真实WAF宿主迁移/混合节点长期互通未专项验证 |
| 网络与供应链安全 | 已验证 | 来源哈希、同SHA扫描、冻结资产/provenance；历史处置与安全待办保留，不作无漏洞保证 |
| 稳定与失败恢复 | 已验证 | 并发竞态、错误重试、公开容器重启与故障恢复；不等同实体宿主回滚 |
| 性能与资源预算 | 已实现未实机验证 | 消费层只增状态字段/标签计算；无新常驻进程，不宣称低配RSS、吞吐或长期soak结论 |
| 体验与可访问性 | 已验证 | 15项Mock交互、1280/390宽度、键盘/错误重试，公开浏览器另验；读屏/实体手机/原生缩放未验 |
| 数据与配置迁移 | 不适用 | 无数据库迁移；宿主WAF配置由同源工具管理，回退Panel不会自动回退它 |

模拟数据acceptance/profile interaction绑定 `a4c812f477af204f10dd484c2fd5d00899247d06`：6模式、constructor未知值、旧true/false、徽章最小计算字号12px、390无横向页面溢出、键盘焦点及HTTP503→错误反馈→键盘重试恢复，共15项通过，pageErrors0/unexpectedConsoleErrors0，故意503另计1。Driver1.62.1使用已有Chromium revision1228；不声称浏览器版本匹配或以此替代项目固定Runner的公开验收。初始窄屏转场截图、扩展失败用例中的歧义按钮定位错误与更正原件均保留；产品源码未为此改变。

## 公开产物、真实浏览器与更新通道

[公开 Release](https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.3)，ID422190087，draft=false、prerelease=true。版本/preview index `sha256:1f112898aeec3cda3474a5a5d22a3c6c007493d7733fd6670869e8ba6c742eb6`；annotated tag对象 `57936cb4b39847ba990145ba4b58b1adb12855de`，peel/source核对到冻结candidate。

- `linux/amd64`：`sha256:175ef7676087d5ffe1d4c0be53414ebfa7f605e00e1a74aff0fc04ccb59fc230`。
- `linux/arm64`：`sha256:6ee5b2458b9f050700114fbf9c9140ba88b13ac5d6b4c06145558850752b0e95`。

两架构OCI标签的source/version/脚本revision及hash一致。15附件名称、大小、API digest、实际下载和磁盘回读全部通过，11项SHA256SUMS通过；冻结snapshot、manifest和最终正文摘要逐字节关联，正文2条更新/3条升级提示。SBOM/provenance与官方镜像身份及modified-component附件一致。

首次公开验收job120593134389完成15附件/11校验和及两架构身份后，在创建测试容器前被写死旧脚本摘要398378的夹具拒绝；不是产品运行或业务断言失败。原ZIP12523643242和日志保留，未改写为成功。

验收工具修复 `59b3d8f55f0e1c1f6e2bc6468942d5e91931ad4e`（运行代码6971db63、测试补齐fc9e7e4a）仅4路径：现有canonical managed-script contract从冻结candidate Dockerfile验证标签和实际脚本bytes；可信controller可覆盖的验收文件固定为Runner配方和单个real-feature夹具，并保存每个原文/hash/diff。独立5.3六维/六项固定矩阵PASS，原报告SHA-256 `f852bee2e308abc15f740fe4cf0d380507f3d74ccc30944edf389aa608ea06b3`、限界补充报告 `9ff43d3d9ce464c52ce6d4bc469e7b141cd7f406b5f9048bb07bb2a8eb1b48d2`。首次helper CI40007372538因既有回归沙盒缺第二固定文件失败，原件保留；只补11行测试准备/回执断言，不改运行代码或删拒绝场景。37项定向、358项本地完整治理和独立5项边界模拟通过。随后40008183325虽绿色但test-only增量路由未跑治理；负责人在主线已快进后从原日志发现证据不完整，保留偏差并暂停恢复。另以明确a4→59b3的资格化ref运行完整4路径的同SHA CI40009333179，与主线CI40009164804都实际成功且原日志证明完整治理后，才执行只读恢复。

首次只读恢复40010543716/1实际脚本身份通过、initial44项通过、restart16项API/持久化检查通过后，语言资源请求出现`net::ERR_NETWORK_CHANGED`导致白屏和桌面等待超时。Panel健康、无OOM、资源充足，截图及失败报告原件保留；据此只进行一次同配置只读重试，没有修改产品/断言、扩超时或重发publication。

只读恢复 [run40011086634/attempt1](https://github.com/kejilion/KPanel/actions/runs/40011086634)，controller `59b3d8f55f0e1c1f6e2bc6468942d5e91931ad4e`，使用原run39998970817/1 snapshot；终态success。原产品candidate/tag/digest、正文及15资产未变；没有重发版本或镜像。项目固定公开Runner实际ID `sha256:c4271f51af4f7d53fa16f6357d0c8ac02bec40f3422bc42d2ee068765bf7634a`，recipe SHA-256 `92d81f0410dd2a05b28b2b6554a0c865696263149306e6ef6d6a2923ff92b48f`；Runner配方与candidate相同。仅夹具覆盖SHA-256 `2ed52184626242e5cc6c273a3abdbb1f34d0edb9cf5ee3736dabf50dabd48e95`，原文及controller Git对象逐字节核对。公开报告`mocked=false`、页面错误和assetFailures均为空，明确应用库存与测试CA为夹具：

- `kpanel-real-40011086634-1/results/initial/report.json`：44 项，passed，错误为空。
- `kpanel-real-40011086634-1/results/restart/report.json`：26 项，passed，错误为空。
- `kpanel-browser-published-40011086634-1/results/report.json`：45 项，passed，错误为空。
- `kpanel-browser-upgrade-40011086634-1/results/report.json`：31 项，passed，错误为空。

公开容器覆盖本地应用、偏好/桌面/终端恢复、HTTPS/证书故障、通道切换、真实发布信息接口和更新弹窗。加入预览只切来源并检查、不自动安装；退出预览且stable较低不产生降级候选，通道和自动安装开关独立。上述报告不替代真实宿主systemd/OpenRC安装、WAF请求拦截、跨发行版迁移、长期soak或真实OneDrive账户。

GitHub Latest ID420434596仍为v1.27.2，Docker latest `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`与发布前快照相同；上一preview确认为RC2固定digest。

## 原始产物证据

- `l3-39998970817-1`：artifact 12522060834，ZIP `sha256:0510dfaeb06bd80c57aaf26d68dac6d438c4e7fc58343d9c257299bb3005ca47`。
- `snapshot-39998970817-1`：artifact 12523499187，ZIP `sha256:d72adde2ad41e0c7118df45944a418660bd38c7d138639e6639f5e7076c4d728`。
- `publication-39998970817-1`：artifact 12524016816，ZIP `sha256:d7af600879b98bfaa5c76d50e89ba21c112cd89a1d49f560d3fa1ff54330973a`。
- `public-40011086634-1`：artifact 12526401882，ZIP `sha256:8f42d09918cc696a9c0c577de77a7b2a9e158ed074c1fc5763c632cdf6fbf9d9`。

仓外证据目录 `C:/GitHub/_release-evidence/kpanel-v128-rc3`。下载ZIP先核对API大小/digest和路径/符号链接边界，再读取/解压。L3三组日志摘要、27原始JSON证据、sourceinventory、公开报告与冻结资产清单已复算；未只凭绿色图标或任务自述。

## 候选与分支处置

- `release/v1.28.0-candidate` 保留唯一RC列车，tip `a4c812f477af204f10dd484c2fd5d00899247d06`；本次预览不归档列车。
- 源 `codex/waf-standardization-consumer`，精确tip `c3e6a1322b668a7c15fa33f190c17d85ec030849` 已纳入本版；保存为 `archive/codex/waf-standardization-consumer` 并以双端expected-SHA lease原子移除活跃远端ref，远端回读通过。
- 源本地工作树clean且tip未变后，分支改为归档名并解除旧upstream；保留工作树与全部证据，不删除文件。上游sh/nginx/docker任务不在本次KPanel归档范围。
- 工具修复 `fix/rc3-public-script-pin` 在同SHA候选/主线CI及公开恢复后归档为 `archive/fix/rc3-public-script-pin`，精确tip `59b3d8f55f0e1c1f6e2bc6468942d5e91931ad4e`；仅治理/验收工具进入可信main，原RC3产品tag不改写。
- 同SHA资格化ref `fix/rc3-public-script-pin-qualified` 同时保存为 `archive/fix/rc3-public-script-pin-qualified`，tip相同；a4基线脚手架CI40009320352按计划被后续59b3事件取消，不冒充验证成功。
- 本验收记录走独立 `docs/release-v128-rc3-acceptance` 候选；仅在同SHA候选CI、主线快进和主线CI成功后按既有规则归档，最终提交/CI/引用由交付记录补齐，不制造新版本或重发产物。
- 管理工作树原有main0524bbcb保持不变。本轮未新增永久工作流/知识文档，仅更新必需业务基线和本验收记录。

## 生产与回滚

生产动作不适用（预览版禁止生产部署）。无生产授权、未连接任何生产目标，`prod-108`/108未连接、未备份、未检查、未部署。隔离容器结果不计生产状态。

上一预览回滚点v1.28.0-rc.2 / `sha256:ef5f55851a6b42036d4a863767081671e0afcb099385ed4333c3c6b59d4412e2`；稳定v1.27.2 / `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`。实际回滚需备份Panel/Agent数据及宿主WAF配置，选择固定镜像后复核健康；退出preview不会自动降级，回退Panel不会自动回退宿主WAF配置。本次没有实际生产回滚、备份或生产健康结论。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-23T20:32:04+08:00
- 候选冻结时间：2026-10-24T01:35:23+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：9
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

流程异常原件保留，不改变产品变更失败/生产回滚指标。详情：

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "qualification/task-preflight/incomplete-validation-contract",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial RC3 ready contract omitted required validations; the read-only preflight rejected it before product work or test execution.",
    "recoveryEvidence": "preflight-first-failed.json and original tool receipt retained; populated canonical candidate CI tool/environment/parameters/argv before retry.",
    "permanentAction": "Release maintainer uses the existing required contract validations schema; existing preflight remains fail-closed. No product or governance threshold was relaxed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/dependency-report/local-tool-and-network",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local dependency report could not discover six required sources: missing Go PATH and failed external fetches. No complete report or scanner pass claimed.",
    "recoveryEvidence": "Original local dependency-report.json/log retained; exact frozen-candidate GitHub report39996547912/job120561442184 succeeded with9/9 sources.",
    "permanentAction": "Release maintainer uses existing fixed-toolchain GitHub-hosted dependency workflow, and explicit Go PATH for local qualification; no partial-source acceptance or policy extension.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/ocr-free-form/timestamp-entry",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial free-form record contained an incorrectly typed future completedAt literal; it is not accepted as the stage clock.",
    "recoveryEvidence": "Original free-form.json preserved; free-form-timestamp-correction.json records observed LastWriteTimeUtc before delegate preview; blind=false and constrained-only=unreported retained.",
    "permanentAction": "Release maintainer uses observed machine timestamps for all later receipts and preserves corrected evidence without overwriting the original.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preview/playwright-retry/ambiguous-button-locator",
    "position": "before-production-write",
    "count": 1,
    "impact": "Extended mock failure-recovery check stopped because a retry selector matched both retry and report buttons. No product failure or host action.",
    "recoveryEvidence": "preview/ui-failure-*.json original retained; exact observed retry accessible name used; final ui-result.json passes15 checks on unchanged candidatea4c812f4.",
    "permanentAction": "Task-owned browser spec selects retry by exact accessible name; no product source or test assertion weakened.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public/real-feature-fixture/stale-script-digest",
    "position": "before-production-write",
    "count": 1,
    "impact": "Original publication39998970817/1 publicly released RC3 and passed15 asset/two-architecture identity checks, but fixture rejected new managed-script digest using hardcoded old398378 value before creating any tested container.",
    "recoveryEvidence": "Original failed job120593134389 and ZIP12523643242 retained. Independently reviewed helper59b3d8f5 passed complete same-SHA candidateCI40009333179 and mainCI40009164804; readonly recovery40011086634 succeeded. Immutable product/index1f112898 unchanged.",
    "permanentAction": "Release-owned helper replaces hardcoded expectation with existing canonical frozen-source label/byte check and records a bounded controller-owned fixture overlay. Independent review, targeted14 tests and exact candidate/main CI required; no gate removed or immutable release overwritten.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/governance-test/missing-verifier-fixture",
    "position": "before-production-write",
    "count": 1,
    "impact": "First helper CI40007372538/job120598600008 failed the existing inline-workflow regression sandbox because it prepared only the former Runner path; updated workflow correctly requires its second fixed verifier resource.",
    "recoveryEvidence": "helper-first-ci-failed-job.log retained; test-only11-line fc9e7e4a preserves6 existing cases,37 targeted and358 local governance checks pass; independent supplement passed. Full same-SHA candidateCI40009333179 and mainCI40009164804 logs prove358 checks and Governance verification passed.",
    "permanentAction": "Extend the existing sandbox to both bounded verifier files without removing negative assertions; independent supplemental review and full canonical governance/candidate/main CI required before original-snapshot recovery.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/candidate-ci/partial-change-routing",
    "position": "before-production-write",
    "count": 1,
    "impact": "Retry CI40008183325 succeeded but actual beforea567 meant the test-only delta skipped governance (tools=none). Release owner discovered this in original log after helper-only mainFF; full candidate governance evidence was not yet complete. No read-only recovery or product publication was dispatched on this partial proof.",
    "recoveryEvidence": "Partial log/helper-handoff retained and superseded. Helper-only mainFF occurred before complete candidate governance proof and is explicitly recorded. Explicit a4-to59 qualificationCI40009333179 and mainCI40009164804 both completed full358 governance checks successfully before readonly recovery dispatch.",
    "permanentAction": "Release owner must bind actual VERIFY_BASE_REF and mandatory Governance verification passed/358 test evidence to the existing handoff receipt before acceptance execution; status-only success is insufficient. Qualification scaffold base run intentionally superseded, immutable product unchanged.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public/browser-runtime/network-change",
    "position": "before-production-write",
    "count": 1,
    "impact": "Read-only recovery40010543716/job120609494168 passed exact script label/bytes, initial44 and restart16 API/state checks, then blank-page desktop wait timed out while i18n asset request reported Chromium net::ERR_NETWORK_CHANGED. Product container healthy, no OOM or pageerror, ample memory/disk.",
    "recoveryEvidence": "First failure run/log/ZIP12525723655 retained. One same-configuration readonly retry40011086634 succeeded:44/26/45/31 checks, errors[] and assetFailures[]; same immutable product, digest and Runner. Temporary exception closed by verified full suite2026-10-23T19:25:41Z.",
    "permanentAction": "Release maintainer owns transient-runtime exception, review2026-10-25; close when same immutable source/digest and fixed runner pass the complete public suite with no asset failures. Retry limited to once; repeated failure requires fresh diagnosis rather than further blind retries.",
    "historicalReleases": []
  },
  {
    "fingerprint": "release-docs/release-metrics/decorated-unverified-marker",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial acceptance metrics validation rejected explanatory suffix on production completion marker. No production operation occurred. Same cause previously recorded in v1.25.1, explicitly retained as a repeat.",
    "recoveryEvidence": "docs-metrics-first-failure.txt preserves original validator output. Task-owned acceptance generator now emits literal 不适用; explanation stays outside machine block. Same canonical validator must pass before docs commit.",
    "permanentAction": "Owner KPanel release maintainer; corrected this task-owned generate-acceptance.py to emit exact existing template marker, without changing canonical parser or policy. This is not a shared script repair claim. Repeat requires shared tooling regression before any next L3 production write; no production write in preview. Review2026-10-25.",
    "historicalReleases": [
      "v1.25.1"
    ]
  }
]
<!-- kpanel-release-process-incidents:end -->

流程复核已读取本轮失败原始日志，并比较最近5个正式版本验收记录（v1.27.2、v1.27.1、v1.27.0、v1.26.0、v1.25.1）的异常明细；本轮前8个指纹没有同名历史复发；第9项文档固定标记错误与v1.25.1同名复发，已保留历史关联和永久处置要求，不以改名规避复发。网络瞬时例外在同配置完整公开测试通过后关闭，原失败与复核日期继续保留。该比较不将相似但不同根因强行合并。
