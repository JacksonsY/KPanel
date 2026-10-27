# KPanel v1.28.0-rc.8 发布验收记录

日期：2026-10-27（Asia/Shanghai）

发布级别：L3。产物已发布，生产未部署。

候选提交 / 标签：3db23780805d66c0895d725947272f6b4d2c54d0 / v1.28.0-rc.8；releaseChannel=preview，releaseTrain=1.28.0。

公开 Release：https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.8；ID=424886231；draft=false、prerelease=true，非 Latest。

版本及 preview OCI index：sha256:218f9f15540d44d5a9b5ad6670ac7d4c1a63cc9c6ce52dce0810ddb0d7281e07。稳定默认仍为 v1.27.2 / sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10，GitHub Latest ID=420434596。

上一预览回滚点：v1.28.0-rc.7 / sha256:05514ee580e703bfb2abc8b0df457ce196ac6f025eb8d491a5ef1b34c12b50ca。

## 发布画像与精确范围

业务域：可选桌面浏览器、Windows轻量节点、网站桌面名称/图标编辑、虚拟机与网站监控窗口、终端链接/跟随和Win98/XP文件图标。涉及新平台、宿主机高权限写入/命令、可选容器、协议能力字段、展示与发布资产集合，按L3验收。Panel/Agent仍是Linux控制面；不包含Windows远程桌面或系统管理适配器。

| 来源分支 | 精确提交 | 归档引用 | 收尾 |
| --- | --- | --- | --- |
| feature/desktop-browser-chromedp | 58d913f8735b19e131e1bf7fa5bb45b94e1900d3 | refs/heads/archive/feature/desktop-browser-chromedp | 已验证；物理代码与证据保留 |
| feature/desktop-site-editor | bea059ee397d582502328e370f19f85e6a747eef | refs/heads/archive/feature/desktop-site-editor | 已验证；物理代码与证据保留 |
| feature/windows-command-node | 862ab1f895e427d59e51800d97d039f8b841cc1e | refs/heads/archive/feature/windows-command-node | 已验证；物理代码与证据保留 |
| fix/desktop-virtual-machine-entry | 22630e90d2c2e4507a7ef1c57ffef0cb3483ab64 | refs/heads/archive/fix/desktop-virtual-machine-entry | 已验证；物理代码与证据保留 |
| fix/site-monitor-window-spacing | ebeb7120704f90bf2e20842833e0a294fc336e33 | refs/heads/archive/fix/site-monitor-window-spacing | 已验证；物理代码与证据保留 |
| fix/terminal-bottom-follow | 09ef982ddff49160f07ad1f043f1aeb53bab4b61 | refs/heads/archive/fix/terminal-bottom-follow | 已验证；物理代码与证据保留 |
| fix/terminal-long-auth-links | 412491a86d5e3c6f2ad26d3cede94eb61aedd15a | refs/heads/archive/fix/terminal-long-auth-links | 已验证；物理代码与证据保留 |
| fix/windows-file-shortcut-artwork | 73563a78cbca1485edabab8d89c07572cc60b330 | refs/heads/archive/fix/windows-file-shortcut-artwork | 已验证；物理代码与证据保留 |
| feature/browser-image-publisher | ffd9a67ad264f82b6e2b25c5064b21aae17c481e | refs/heads/archive/feature/browser-image-publisher | 已验证；物理代码与证据保留 |
| fix/rc8-integration-acceptance | 3db23780805d66c0895d725947272f6b4d2c54d0 | refs/heads/archive/fix/rc8-integration-acceptance | 已验证；物理代码与证据保留 |

主线已有但尚未随RC7发布的偏好路由修复362e1feb一并纳入；浏览器镜像发布控制器和本次有界Windows安装/验收修复按归档表区分。八产品来源均是产品祖先，无squash映射。291个登记工作树完成本轮盘点，本地/远端同名去重；历史未合并不按名字/日期假定已上线。冻结后未追加新功能。

release/v1.28.0-candidate保留在3db23780805d66c0895d725947272f6b4d2c54d0，单次RC不归档列车。归档精确tip均已远端回读，本地活跃引用转archive并解除旧upstream；物理工作树、忽略文件和证据未删除。备份外部导入、旧终端测试及旧版验收文档三处历史dirty原样保留，不纳入产品。验收文档分支另在同SHA候选/主线CI成功后归档。

## 跨仓库联动与可选组件

scriptLinkageState=not-required：无需发布脚本（不适用）。内置已公开脚本614d58cee228007012a2ec602cbdc8dbaeb7950d，SHA-256=ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9；本轮未改变kejilion.sh协议、Linux安装/卸载或应用市场默认入口。Windows bootstrap/安装器属于同版本KPanel附件，不冒充另一个脚本仓库发布。

可选组件docker.io/kjlion/kpanel-browser:0.1.0-rc.2，固定index sha256:fbc00e5b7e5bb39c117507b1995cc0e4f85976eb6818db23322d46968f7d5697，最终来源58d913f8。原组件双架构、L3、匿名公开拉取和真实Panel/HTTPS集成原件单独保留；不是本次Mock或主KPanel公开镜像验收的替代。组件需单独配置、按需启用，不自动安装或更新；未配置时不可用，不暴露原始CDP、公网端口或Docker Socket。

## 六维质量与证据边界

| 维度 | 状态 | 事实与限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证（限定范围） | 新Windows轻量节点入网元数据、遥测/历史、文件/终端接口及Linux兼容测试；真实Windows SCM与TLS协议夹具成功。真实多主机Windows relay、public PowerShell downloader/installer全旅程未验证。 |
| 安全与供应链 | 已验证（门禁范围） | 来源普通包级盲审、独立复核、本次有界运行时修复普通盲审；完整CI/L3扫描、双架构SBOM/provenance和公开身份校验。未执行新CF或全产品无漏洞认证。 |
| 稳定与恢复 | 已验证（限定范围） | SYSTEM bootstrap、受限遥测账户与2个SYSTEM broker，签名遥测、SCM停止/启动、真实磁盘更新、注入health失败后的字节回滚、卸载/计划任务清除，原ConPTY15秒断言通过。OS重启、断电、所有Windows版本和原生ARM64未验。 |
| 性能与资源 | 已实现未实机验证（长期趋势） | 复用现有历史/任务/传输配额及背压，浏览器组件独立限额且按需启用；没有宣称低配RSS、soak、吞吐或所有网络拓扑已实测。 |
| 体验与可访问性 | 已验证（浏览器范围） | 精确候选Chrome/Mock27项，真实公开镜像150项。三种桌面外观、宽窄/等效200%视口、站点编辑浅深/CSS200、键盘/焦点及失败恢复。原生浏览器缩放、实体触屏/读屏未验。 |
| 数据、配置与兼容 | 已验证（限定范围） | Windows可选能力字段，旧Windows身份不复活，新授权才可管理；Linux旧配对保留。站点编辑仅桌面别名/图标，不改域名、站点文件或favicon。新19附件/15校验项；旧控制器冻结15/11仍按原身份严格恢复。 |

## 精确门禁、审查与公开产物

可信执行器5490d9ba在候选CI40409839046和同SHA主线CI40411351015成功后进入main，保持rc7版本；随后递增RC8与源码身份。冻结push CI40415118924完整成功。发布run40417067758/attempt1的候选CI和完整L3双成功后，原子main/annotated tag提升、显式mainCI、Windows原生校验和Release全部成功；执行控制器5490d9ba78648c8b5ffd1d28c8c61c86f9febd01与产品3db23780805d66c0895d725947272f6b4d2c54d0分层记录。

L3严格核验原artifact大小/digest、入口摘要、tree、三源码lane及27份原始扫描/构建证据，image_e2e=pass；前端328文件、3285通过、6跳过，未把skip写成通过。Go1.27.3/Node24.22.1，GitHub-hosted ubuntu24实际runner版本20261023.334.3。1220第一方Go文件、六目标actual/original闭包、469selected/111production/150standalone均与精确冻结观测相等；保留原37处置和2026-10-25..11-08窗口，没有用身份资格化伪造新扫描日期。

首次公开验收在已发布浏览器旅程的更新API请求上失败，原run整体结论failure，不写成一次完整成功。该run另一真实Panel重启旅程已返回200及正确RC8信息，失败请求未保留HTTP状态/响应体，根因未确认。随后仅通过既有只读恢复入口run40422821013/attempt1重验同一不可变产品，恢复原run40417067758/attempt1的发布前snapshot，未重建/覆盖镜像、版本或正文；150项公开检查成功。原preview-completion跳过，规定的Git归档和记录由本次负责人补齐，不伪写该job已运行。

最终公开验收核对19附件名称/大小/API digest/下载磁盘回读、15条SHA256SUMS、最终正文及11条全文更新/运行时8条摘要、4条全文升级说明/运行时3条；两个OCI平台、固定源码/版本/脚本标签、SBOM/provenance、原发布前stable/previous-preview快照和不可变发布身份均通过。真实公开Panel/Agent容器的初次/重启、已发布及升级浏览器报告全部成功；测试CA/域名/应用库存等夹具已声明，不等于生产宿主机安装或跨平台实机证明。

原Windows受保护原子提交与任务XML编码是真实产品问题，经门禁发现后有界修复；管理员秘密读取权限未放宽。测试SYSTEM私有plan消费、原生包并发隔离、布局分类和一行提案枚举修正各有首败和成功原件。新Windows执行器delta经不同干净上下文独立只读复核；同提供商降级provider-unavailable，工具未暴露精确模型，不伪填其他模型/提供商。OCR1.12.11按各精确修复范围保留自由臂/约束臂；接触过规则的结果保留constrained-only=unreported，不伪作盲测。

## 通道、回滚、生产与遗留

稳定来源只选择正式Latest，preview选择规范RC并固定官方digest；加入预览不等于自动安装，退出不自动降级，真实公开升级对话框与通道策略已按实际报告核验。原窗口/终端恢复历史偶发不因当前成功关闭。

不适用（预览版禁止生产部署）。本次只使用GitHub-hosted，不连接生产/自有服务器或WSL，不读取/备份/升级生产，不把隔离容器与Windows临时runner当生产证明。prod-108未连接；稳定默认不提升。

回滚保留v1.28.0-rc.7及sha256:05514ee580e703bfb2abc8b0df457ce196ac6f025eb8d491a5ef1b34c12b50ca；不覆盖不可变tag/版本镜像。回退前备份Panel/Agent、节点配置和站点图标，核对在途文件/命令；旧中心可能拒绝新字段或清理Windows身份。代码回滚不能撤销已执行宿主命令或文件写入。新/重新Linux Node默认稳定安装器限制、来源端约60秒断流、RC5桌面恢复偶发保留，已有Linux配对不因本轮重新配对。

Windows原4条中风险线索仍是待核实/产品语义待定，不是4个新确认漏洞；本次普通审查保留既有互斥对象与文件中继线索，期限2026-11-02或1.28稳定Windows准入前先到者，仓外原件及责任人保留。RC5恢复与新Node入口按原2026-10-28/稳定前触发复核，MCP元数据低风险线索按原2026-11-04；均未续期。TS7/@typesNode26旧采用例外2026-10-23已到期，依赖发现job如实exit3，未采用/续期，不声称依赖全部最新；实际运行依赖审计和L3另有成功原件。

物理代码、证据、忽略缓存和工作树全部保留，没有后台磁盘删除；只停止本任务ownership marker匹配的Mock预览，来源预览未接管。资源收尾详见resource-closeout.json，不把磁盘可用空间变化冒充清理释放量。复用既有工作流，未新增知识库或永久发布规则。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-26T09:34:47+08:00
- 候选冻结时间：2026-10-26T22:47:21.775Z
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：17
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

### 流程异常明细

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preflight/business-context/stale-source-baseline",
    "position": "before-production-write",
    "count": 1,
    "impact": "Combined candidate CI rejected the old business review baseline after eight sources were integrated.",
    "recoveryEvidence": "ci-40395096265-linux-first.log; actual six-dimension facts refreshed without changing freshness limits.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/page-root-contract/missing-workspace-classification",
    "position": "before-production-write",
    "count": 1,
    "impact": "The route-driven layout contract did not register the existing full-height browser workspace; the check rejected the combination.",
    "recoveryEvidence": "combined-layout-first-failure.json; owned fix registered existing workspace semantics,12 files/282 tests passed without browser runtime padding changes.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/layout-regression/wrong-worktree",
    "position": "before-production-write",
    "count": 1,
    "impact": "A layout recheck used the release checkout before the owned fix was integrated; that result was not accepted as the repaired source.",
    "recoveryEvidence": "layout-after-fix.log retained; later source checks and preview drivers bind exact HEAD/worktree.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "verification/windows-scm/sealed-plan-reuse",
    "position": "before-production-write",
    "count": 1,
    "impact": "Second hosted updater step tried to replace a SYSTEM-only test plan as administrator and correctly received access denied.",
    "recoveryEvidence": "ci-40406854930-windows-task-encoding.log;285eb274 makes SYSTEM consume its bounded validated test plan. No production ACL changed.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "verification/windows-runtime/package-concurrency",
    "position": "before-production-write",
    "count": 2,
    "impact": "SCM/nested-build and terminal package tests ran concurrently; two cloud runs timed out first-shell assertions. Internal shell cause was not separately proven.",
    "recoveryEvidence": "ci-40406854930-windows-task-encoding.log and ci-40409160889-windows-plan-consume.log; independently reviewed5490d9ba serializes packages, retaining all tests and15-second assertions. Repeated exact hosted native success follows.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "review/ocr-delegate/overbroad-rule-equality",
    "position": "before-production-write",
    "count": 1,
    "impact": "The author comparison included different per-range file lists instead of comparing the resolved Go rule body, causing a false mismatch.",
    "recoveryEvidence": "Execution transcript retained; plan-consume-ocr-rules.json rule body compared equal to task-encoding rules; plan-consume-ocr-coverage.json records1/1.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/artifact-verifier/shell-quoting",
    "position": "before-production-write",
    "count": 1,
    "impact": "An inline PowerShell/JavaScript preparation command was parsed incorrectly; no prepared verifier evidence was accepted.",
    "recoveryEvidence": "Execution transcript retained; prepare-artifact-verifiers.mjs now carries exact structured file content, producing strict1220/19/15 and full/clipped note assertions.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/governance-health/nonexistent-entrypoint",
    "position": "before-production-write",
    "count": 1,
    "impact": "An initial local enum-fix verification called nonexistent check-governance-health.mjs.",
    "recoveryEvidence": "Execution transcript retained; existing verify-governance.sh points to report-governance-health.mjs --validate; proposal-enum-health.log passes34 proposals.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/release-contract/incomplete-handoff-inputs",
    "position": "before-production-write",
    "count": 1,
    "impact": "A premature read-only handoff check lacked Go PATH, exact candidateCommit and finished CI/L3 receipts; it failed closed.",
    "recoveryEvidence": "frozen-handoff-first-unqualified.json retained. Final handoff must bind completed exact receipts and verified Go1.27.3 PATH; no permissions inferred.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/proposal-state/noncanonical-status-prefix",
    "position": "before-production-write",
    "count": 1,
    "impact": "Frozen d68a candidate CI rejected proposal state beginning with an unsupported prefix.",
    "recoveryEvidence": "ci-40414607820-linux-first.log;3db23780 changes one field to the existing trial enum prefix; no gate/decision/date changed. Canonical health and frozen CI40415118924 passed.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/mock-ui/ambiguous-dialog-selector",
    "position": "before-production-write",
    "count": 1,
    "impact": "The combined driver used a generic dialog selector matching both desktop window and modal.",
    "recoveryEvidence": "combined-ui-5490d9ba/result.json; scoped to the actual accessible modal name.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/mock-ui/wrong-command-fixture",
    "position": "before-production-write",
    "count": 1,
    "impact": "The combined driver expected an installer-style word in a deliberately harmless Mock enrollment command.",
    "recoveryEvidence": "combined-ui-5490d9ba-r2/result.json; assertions now match actual harmless Windows/Linux fixture strings, not real enrollment.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/mock-ui/unsettled-animation",
    "position": "before-production-write",
    "count": 1,
    "impact": "The driver sampled overflow before the asynchronous desktop viewport/resize state settled.",
    "recoveryEvidence": "combined-ui-5490d9ba-r3/result.json and screenshot; bounded actual layout settlement plus window bounds assertion retained.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/mock-ui/invalid-zoom-evidence",
    "position": "before-production-write",
    "count": 1,
    "impact": "Automated CSS200 assertions passed, but screenshot review showed fixed-window coordinates clipped; that200-percent proof was rejected.",
    "recoveryEvidence": "combined-ui-5490d9ba-r4 and ui-driver-incidents.json retained; equivalent720x500 viewport and actual fitted bounds passed in r5/final. Native browser zoom remains unverified.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/mock-ui/retained-window-fixture",
    "position": "before-production-write",
    "count": 1,
    "impact": "The source site-editor driver assumed a fresh desktop, while a preceding combined Mock journey had persisted a VM window covering its icon.",
    "recoveryEvidence": "site-editor-frozen/browser-result-r2.json retained; final driver explicitly binds an empty-window Mock starting state, then13 exact3db checks pass. No product change.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/published-browser/release-api-non-success",
    "position": "before-production-write",
    "count": 1,
    "impact": "Original publication succeeded, but its published-browser journey failed a release-preview API request after24 checks. The same run separate real Panel restart suite returned200 and correct RC8 identity; the failed fixture did not retain status/body, so cause remains unconfirmed.",
    "recoveryEvidence": "public-40417067758-first-failure.log and original public ZIP retained. Existing read-only verification40422821013 restored the original snapshot and verified the unchanged immutable image with150 successful checks. No rebuild, channel write or republish. Review release-response diagnostics by2026-10-28/next preview; if it recurs, capture bounded HTTP status/body before further retry. Not a claimed permanent product fix.",
    "permanentAction": "Existing canonical gates stay unchanged. Owned fixture/executor or evidence-driver corrections are named above; remaining invocation/evidence corrections are not claimed as permanent repository mechanisms. KPanel release-tool maintainer reviews by2026-10-28 or next preview, whichever first; exit is correct canonical input ordering and stable exact-source evidence on next use.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

以上为生产写前的实际执行/取证问题；产品缺陷被正常质量门禁拦截单列，不作为生产变更失败。原始失败/无效截图保留，不以成功复验抹零。最近5个正式记录逐指纹核对见historical-incident-comparison.json；RC不计入正式发布频率，滚动指标见release-metrics-before.txt，未知生产数据不补零。

## 仓外原件

C:/GitHub/_release-evidence/kpanel-v128-rc8：frozen.json、candidate-inventory.json、freeze-contract/ready、冻结六目标inventory/source qualification与source binding、原普通安全审查/triage、Windows真实生命周期各首败与成功日志、ui-qualification、L3原ZIP及严格验证、原publication/snapshot/public ZIP与验证、公开browser报告、精确来源归档/恢复引用、流程异常、资源收尾与最终通道回读。证据只对精确源码、工具、环境及参数有效。
