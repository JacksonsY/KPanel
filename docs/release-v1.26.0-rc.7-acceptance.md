# KPanel v1.26.0-rc.7 发布验收记录

日期：2026-10-11。发布级别：L3。发布通道：preview；发布列车：1.26.0。

**结论：RC7 已公开发布，精确产品门禁及发布后验收通过；未部署生产。** 原 Release 工作流失败历史保留，公开发布通过受审恢复步骤完成，不将恢复记为原 CI 成功。

- 产品提交：`6ee57f095268346b115f64c0f2c6fec6212157c8`，tree `7a2038148ced232cb22880ed0b22e7a174009581`。
- 标签：`v1.26.0-rc.7`，annotated tag `827795949ece0e205a862774700527822134038c`，peeled 指向上述产品提交。
- [公开 Release](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.7)：2026-10-11T11:19:26Z，draft=false、prerelease=true、非 Latest；15 附件/11 条 SHA256SUMS。
- version/preview 镜像：`docker.io/kjlion/kejilion-panel@sha256:968946e9dcc1e478fef355f84c355c11db7d5e35b89284c01f6f2649389fdaf4`，amd64/arm64。
- 上一预览回滚点：`v1.26.0-rc.6` / `sha256:83960073e7202ad6b73db193037e208cbe8d242c96bd4919fa9cf80ca3a2e52b`。
- 稳定 Latest/latest 保持 `v1.25.1` / `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`；仅本轮公开读回时点结论。
- 预览候选 `release/v1.26.0-candidate` 保留在产品提交；后续治理提交 `e564b6af9f290a54608397d5c2475bbd627728ce` 的候选/main CI 均通过并已归档清理，不属于已发布产品。

## 发布范围与用户旅程

| 范围 | 来源及本次结论 |
| --- | --- |
| 终端打开、首段输出、缺 Shell 提示 | `fix/terminal-open-latency` / `de7eb95f8766762bb1ada2dba03e0b21d7695dcd`；首条输入与首段输出、关闭重开、缺 Shell 场景通过 |
| BusyBox 光标与粘贴 | `fix/terminal-cursor-report` / `13360e870c150ab55afcf5ff9e6d8426d15f394c`；光标报告及最终 rollup 的原生 Ctrl+V 路径通过 |
| BT、容器多挂载版本、原生粘贴 rollup | `fix/bt-progress-stall-rc7` / 产品提交；部分载荷后 writer 唤醒及 keepalive 重启，Mounts 排序避免伪冲突，真实冲突保留 |

**边界：无 /bin/sh 的镜像不能打开交互 Shell。** RC7 提供准确错误，不给镜像安装 Shell；用户现场的 kejilion-panel 镜像属于这一情况。CAP_KILL 的旧安装迁移沿用 RC6，没有新增权限迁移。Panel/Agent 应配套更新。本轮不扩大 API、存储、端口、Compose 或宿主机写入协议。

BT 保留上游 v1.61.0，只回移两项官方补丁。定向回归不证明所有公网/DHT/长时停滞已覆盖。跨仓库 apps 只读观察无需同步提交；scriptLinkageState=not-required，内置脚本 commit `1800d955f216aebd2776674368a8e469a671c489` / SHA256 `5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c`。apps commit `6602ac36e0c329e35b74ff621721b23583e94f5f`，用户工作区修改保留。

## 精确提交的自动与独立验证

| 门禁 | 实际结果与范围 |
| --- | --- |
| 候选 CI / freshness | 38127368506 / 38127368498，产品提交，success |
| main CI / freshness | 38128280334 / 38128280323，产品提交，success |
| canonical L3 | runId=`v1.26.0-rc.7-6ee57f09-l3-r1`，08:21:21Z—08:44:03Z，22 分 42 秒，exit0；12 原件回收 |
| web | 274 文件，2568 通过/6 跳过；typecheck、production build 通过 |
| Go / BT / 构建 | 普通 BT 37.313s；panel race 224.295s、auth 8.358s、docker 65.447s；两项官方补丁 race 1.069s；vet/source/install_safety/scenepack/双架构 5 二进制通过 |
| watchdog | 690 采样、最大间隔 2.4s、0 干预；最低空闲磁盘 3.95GB、内存 2.259GB；不将预算与实际剩余值混用 |
| 终端后台浏览器 | 08:48:42Z—08:49:52Z，17 场景/18 PNG，8 会话关闭；root/uid1000、首条/合并输入、关闭重开、窄屏/浅色/resize、BusyBox CPR、原生粘贴通过 |
| 原生粘贴证明 | 真实 paste DOM → WS seq2/CR ack2 → SSE echo，未发送 0x16；无新增异步剪贴板权限读取 |
| 发布说明 parser | 实际 5 条摘要/3 条升级提示；未知分类/展示限额规则保持 |
| 发布后公开镜像 E2E | 11:22:21.718Z—11:22:37.755Z，精确 digest、port18080、exit0；前后自有容器/网络清单相同 |
| 公开说明与通道浏览器 | 11:36:47.628Z—11:37:26.998Z，8 场景/8 PNG，52 原件 hash 校验，退出0；7 自有容器/网络/私有 fixture 清理通过 |

终端 18 张和公开 8 张 PNG 均已实际查看。终端只接受预先审查的 7 项 registry 错误及预期缺 Shell 409，未知错误仍阻断。公开验收覆盖说明/API/升级提示、stable/preview 切来源不安装、加入后立即检测、显式检测、退出不自动降级、默认迁移/重启持久化、实际下载 HEAD/range、登录退出后404及两个 ZIP 字节内容。offers 实际 14 项/17 media；0 OOM、console/resource/install 错误。窄屏短时 toast 遮挡部分弹窗标题的视觉限制保留。

## 依赖、供应链与安全覆盖

| 扫描范围 | 实际告警与退出码 |
| --- | --- |
| 生产闭包 106 外部模块 | 0 HIGH/CRITICAL，exit0 |
| selected 460 外部模块 | 10 项漏洞，exit1，原件保留 |
| standalone 150 外部模块 | 24 项漏洞及 2 配置项，exit1，原件保留 |
| govuln / npm | govuln 0 affected、0 imported、1 require-only 警告；npm 0 |

36 条逐 ID/module/version/scope 处置有效至 **2026-10-25**；非生产范围判断不等于修复漏洞。新增/未知/过期/生产交集或源码、policy、构建身份变化均重审，不声称全图无漏洞或永久豁免。扫描补充的原 stream marker 交错解析失败保留；只修正外部 parser 后绑定同一原件，未改变扫描退出码或告警。

- 覆盖检查：scoped-required；80 commits/196 files。本轮未新增 CF 审计；继承 deferred/inherited，run-31/32/33 incomplete/needs_validation，稳定版前须补审，不将未完成 run 计为完成。
- SSE 撤权、轻量节点撤权和 private magnet DHT needs_validation 保持；经典 GPU 拖动 +25.485% 超20%预算未关闭。
- offers 后台刷新/TempDir 竞态：2026-10-15 或稳定版前；GPU 准入：2026-10-16 或稳定版前。截止要求未延期。
- 纯文档收尾不改变安全边界；没有新增 CF scoped/full 审计或跨供应商审计结论。

## 公开产物与恢复发布

发布后读回 version/preview 同 digest、稳定 Latest/latest 保持、正文和 15 附件/11 sums；修改组件 metadata SHA256 为 `0d7bbb3482d9cdfc6a3f86f00c7f5bf67dee0c43b6f769d7995dec2efb969bbc`。453 个源原件与产品 Git blob 对应，双架构 manifest/config/labels 和镜像提取的 4 项二进制 build info 对应。原 SBOM/SLSA 保留；原 Git materials 缺失已披露。未重新下载全部10个独立二进制附件，不宣称所有资产都进行了逐字节下载。

Release Actions **38129259772 attempt1 仍为 failure**：step20 draft API 查询404，后续步骤跳过；freshness 38129259780 success。受审 REC6 使用既有资产、同一 tag/镜像，完成 provenance/metadata 及通道/Release 公开，没有重建、移动标签或覆盖已有版本产品字节。恢复回执为 recovered，不是原工作流 success。

Docker 29 将 .Image 暴露为 child/config 身份差异，恢复流程使用独立复核的兼容 collector；原 canonical collector 未直接通过。可移植性治理项须 **2026-10-25 或稳定版前**处理。后续治理提交只修复 draft Release databaseId 的正整数查询，19 行基础设施变更、实际 L1 300/300 通过；候选 CI/freshness 38132687041/38132687131、main CI/freshness 38136556753/38136556747 全部 success。它不能替代 Docker 29 可移植性修复，也未重发 RC7。

REC5 私有 classic daemon 误删共享 docker0 的实际失败、34 原件及恢复回执保留。已退出私有 daemon、移除其自有 runner，恢复桥 MAC/IP/MTU，既有 n8n/arena 容器 PID/端点未变，两个 gateway ping 和两个服务 HTTP200 通过。该证明不覆盖完整路由或防火墙审计；REC6 只使用共享 Docker29 单 daemon。所有历史失败均保留，不以最终成功抹除。

## 分支归档、本地清理与回滚

三个功能来源分别保存完整可验证只读 bundle 和精确 `archive/fix/*` 引用，活跃远端来源分支现均不存在：latency 在本次归档中移除，cursor/BT 在归档前已不存在、本次仅保存精确 archive 引用；所有权释放、干净状态、路径/process/引用检查后普通 worktree remove 和 branch-d 完成。cursor 来源额外仅清理已盘点生成目录 web/dist、web/node_modules。治理来源同样已完成精确归档及普通清理。预览候选保留。process 命令行检测不等于完整打开句柄检测。

六个 blocked 工作树、其他作者和用户未提交内容、共享 Docker 缓存/镜像/卷继续保留。C3、C4、旧RC1自有重复临时文件已按各原始回执清理，不重复删。本次文档自己的候选/main CI、归档清理，以及 C5/REC5 自有临时目录最终清理，由之后的实际外部回执证明；本提交不提前宣布。

回滚保留 v1.25.1/RC6 tag、immutable 镜像、成对 Panel/Agent 和配置备份。退出预览不会自动降级；实际降级/回滚需核对目标及当时通道。生产部署、prod-108 健康及生产回滚均不适用，本轮没有访问或部署 prod-108。

## 交付节奏与保留的异常

候选冻结 09:09:01.727Z 至公开 11:19:26Z 约 2 小时10分24秒；这段包含公开构建和恢复，不能解释成全程产品测试或生产交付前置时间。canonical L3 本身22分42秒。冻结前还经历未发布 BT/供应链/终端候选修复，因此该差值不是整个任务耗时。

封存的正常化台账有 **62 次流程异常、122 条事件历史、41 组处置、生产写后0**；静态拦截、工具读取笔误与产品缺陷分开，不将每条历史事件都算作运行失败。两个原始未确认 native 记录仍 retained-unresolved。七层候选原件、四份未 seal 草稿、原109事件及17份追加片段保留；成功原件不覆盖失败原件。原生 ARM、Safari/Firefox、实体手机/缩放、屏幕阅读器、真实 CA、长时WAN/soak及完整性能准入未新增验证。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-11T09:46:20+08:00
- 候选冻结时间：2026-10-11T09:09:01.727Z
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：未验证
- 若发生失败，发现时间、恢复时间和逃逸门禁：未验证
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：65
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

### 流程异常明细

前 41 组只投影已独立复核台账的七个 canonical 字段，第 42、43 组为封存后的文档 L0 及本地检出恢复失败追加记录；逐项原件/追加记录在封存台账中保留。historicalReleases 只接受稳定版标签；RC 重复指纹作独立观察，不伪造稳定历史。 逐组处置文字按封存时点原样保留；其中 G main integration 等 pending 文字属于当时历史，后续已完成事项以本记录正文和其列出的新回执为准，不回写封存原件。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "draft-parser/windows-entry/missing-go-runtime",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：Windows draft parser invocation failed because Go was unavailable; no public write had occurred.",
    "recoveryEvidence": "Predecessor draft-parser-r2-qualification.json binds actual fixed Linux Runner recovery. Old Windows execution remains failed.",
    "permanentAction": "Owner: release toolchain maintainer; review by 2026-10-18 or before next stable release preflight, whichever earlier. Exit condition: notes entry enforces required Go runtime/fixed Runner preflight. Current external fixed-runner recovery is retained; no shared repository repair is claimed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "canonical-l3/offers-fixture/background-cleanup-race",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：Old candidate complete canonical L3 r1 failed on historical asynchronous offers TempDir cleanup; web cancelled, deploy not run.",
    "recoveryEvidence": "原4e9 r1失败与同SHA r2恢复继续保留；本候选C5也实际完成canonical L3（08:21:21至08:44:03Z，exit0），仅证明本次完整运行通过，不证明旧异步fixture隐患已永久修复。",
    "permanentAction": "Owner: KPanel offers fixture maintainer; existing deadline 2026-10-15 or before promotion to stable, whichever earlier, is retained without extension. Exit condition: isolated reviewed fixture waits for background refresh completion and its required regression passes. Current C5 L3 recovery does not prove permanent repair of the old asynchronous fixture risk.",
    "historicalReleases": []
  },
  {
    "fingerprint": "diagnostics/ssh-remote-argv/docker-format-whitespace",
    "position": "before-production-write",
    "count": 2,
    "impact": "两次已计只读inventory调用的Docker --format空格在远端shell参数重构中丢失，均在本次公开写入前发生；原失败输出未用于门禁通过。",
    "recoveryEvidence": "draft-parser-r2-inspection-incident.json + actual draft-parser-r2-qualification.json; terminal-browser-prep/image-list-incident.json binds failure e1aef7 and corrected --format=json native595449. Read-only impact does not exempt these two events.",
    "permanentAction": "Owner: KPanel release maintainer; review by 2026-10-18 or before next L3 production write, whichever earlier. Reuses the exact v1.26.0-rc.2/rc.6 fingerprint. Exit condition: single shared remote inventory entry uses structured subprocess argv or unambiguous JSON format and passes required regression for spaced input. Existing external corrections are recovery only; no permanent shared repair is claimed. RC recurrences remain preview observations, not stable historicalReleases.",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/terminal-fixture/bootstrap-token-owner-permissions",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：Actual terminal browser r1 failed before any business assertion: drop-ALL root could not read Panel UID65532/0600 bootstrap token; browser-result EACCES, cases empty. R1 cleanup passed with existing Agent PID/container set retained.",
    "recoveryEvidence": "旧EACCES、后续独立产品失败和诊断原件保留。私有权限明确的夹具在C5最终实际浏览器验收完成17项/18图，cleanup passed；最终通过只属于C5，不改写旧r1/r2/r3结论。",
    "permanentAction": "Owner: KPanel browser-validation maintainer; review by 2026-10-18 or before next L3 production write, whichever earlier. Exit condition: shared fixture copies bootstrap material into private runtime with explicit required owner/mode and read-only mount, qualifies actual unprivileged-capability semantics, and the complete required browser suite passes. Current external fixture repair is not a permanent repository repair.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/read-only-file-inspection/unverified-path",
    "position": "before-production-write",
    "count": 16,
    "impact": "历史文件名、工作区或参数假设导致只读检索失败，本组当前共16次真实既有occurrence，全部ID和原观察记录保留。失败输出不作为成功证据；原记录中仅owner确认而无独立完整转录的范围不补造。普通新增读错没有借本组追加计数。",
    "recoveryEvidence": "incident-review-rg-failure-observation-r1.json preserves actual command, native chunk and exact visible error excerpt without fabricating a full raw file. Nativecc551b enumerated authoritative filenames; native761f22 read actual RC6 process-incidents.json completely. Parent native2bcb06 recovered through dfb99d; native7c976f recovered through a0155c; parent observation record preserves actual native references and clearly discloses that full raw argv/output were not supplied to this reviewer. Additional parent native282616 recovered through c17c75; incident-review-parent-rg-failures-observation-r2.json records those actual receipt IDs and limits.",
    "permanentAction": "Owner: release evidence reviewer; review by 2026-10-18 or before next L3 production write, whichever earlier. Exit condition: discover exact filenames before reading, put rg options before --, and require success before consuming output; shared inventory regression must cover absent paths/options. This corrected read is recovery, not a permanent shared-entry fix.",
    "historicalReleases": []
  },
  {
    "fingerprint": "release-l3/governance-disclosure/missing-origin-main",
    "position": "before-production-write",
    "count": 3,
    "impact": "本组历史共3次既有过程记录，缺失origin/main使规范披露比较不能完整执行；每次原warn/raw保持。外部精确基线补充校验关闭对应证据缺口，不把原warning擦成PASS。",
    "recoveryEvidence": "历史三次已计记录及其原warning保留，不把source任务的历史warning重复计入。C5 canonical L3原件与本候选governance-disclosure-supplement-result.json均已实际passed；补充记录明确比较基线及原warning，属于本候选证据补足，未声称canonical共享脚本已经永久修复。",
    "permanentAction": "Owner: canonical L3/governance maintainer; review by 2026-10-18 or before next L3 production write, whichever earlier. Exit condition: canonical kit supplies the approved comparison ref or checker takes an explicit frozen base, with required regression proving missing refs cannot silently skip disclosure. External exact-ref supplement closes this run evidence gap only; no permanent canonical-script repair is claimed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/static-review/source-hash-read-race",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：R1 static review read original push-preview source, then the parent modified it before hash capture; the finding described original code while inputs bound repaired hash. This invalid provenance required a separate correction; ordinary static defect discoveries are not additionally counted.",
    "recoveryEvidence": "release-controller-static-review-r1.json preserved unchanged; release-controller-static-review-r2.json provenanceCorrection binds original a44dc149... and corrected ee0e9860... hashes and explains read/hash race. Root explicitly accepted this as one invalid-evidence event.",
    "permanentAction": "Owner: release review maintainer; review by 2026-10-18 or before next L3 production write, whichever earlier. Exit condition: review immutable snapshots, compute hash from the same bytes used for review, and reject mutation before publication with a required regression. Append-only r2 correction is recovery only, not a permanent shared review-tool repair.",
    "historicalReleases": []
  },
  {
    "fingerprint": "validation/cursor-report-fixture/environment-mismatch",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：One focused test invocation failed three cases: Terminal.paste without an opened textarea, plus two jsdom non-file import.meta.url reads. Count 1 invocation, not 3 cases.",
    "recoveryEvidence": "历史focused-r1及fixture-only修正后的11文件/94case结果保留。C5完整canonical L3及17项终端浏览器验收已实际passed；完整浏览器证据含真实剪贴板和输入场景，未扩大到未执行的浏览器或设备。",
    "permanentAction": "Owner: cursor-report fixture maintainer; reviewed 2026-10-11. Actual fixture repair uses the Node environment and public Terminal.input semantics, with existing input/paste regressions retained. Exit condition: final exact-candidate L3 and actual browser acceptance complete; focused recovery alone does not qualify them.",
    "historicalReleases": []
  },
  {
    "fingerprint": "cleanup/owned-artifacts/mount-array-order",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：Cleanup r1 failed before deletion because Docker Mounts array order was compared directly. removed=[]; 18 containers and Agent identities were unchanged after mount-order normalization.",
    "recoveryEvidence": "owned-superseded-cleanup-r1-remote-receipt.json remains failed; r2 exact cleanup sorted mount lists without removing fields and actually passed, preserving protected evidence and Agent/container identities.",
    "permanentAction": "Owner: release cleanup maintainer; reviewed 2026-10-11. Exact external r2 sorts mounts while preserving duplicates, all identity fields and reference guards. Exit condition by 2026-10-18: shared cleanup entry adopts deterministic mount comparison with required regression; no permanent shared repository repair is claimed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/patch-expression/undefined-js-binding",
    "position": "before-production-write",
    "count": 1,
    "impact": "一次本地functions表达式在调用apply_patch前发生ReferenceError；未执行patch、产品门禁或远端操作。它是已保留的历史过程事件，不重新计数。",
    "recoveryEvidence": "delivery-referenceerror-observation-r1.json preserves exact reported error and ID limitation; immutable b730 preparation snapshot and four actual syntax receipt files support successful split String.raw recovery.",
    "permanentAction": "Owner: release delivery controller maintainer; review by 2026-10-18. Exit condition: literal patch construction avoids host-language interpolation and every generated controller passes its required syntax/static checks. Reissuing the patch is recovery, not a claimed shared-tool permanent fix.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/incident-generator/invalid-string-literal",
    "position": "before-production-write",
    "count": 2,
    "impact": "历史组影响（原失败仅按其实际候选成立）：Two actual local child invocations failed on the same missing closing quote: syntax check and generator parse. They remain two occurrences under one fingerprint.",
    "recoveryEvidence": "Native764c51 and failed original source retained; corrected source syntax parsed in native218db6 exit0. Subsequent preparation results remain independently recorded; no product/runtime gate PASS is claimed.",
    "permanentAction": "Owner: release incident preparer; reviewed 2026-10-11. Corrected the missing quote. Exit condition by 2026-10-18: shared preparation entry validates generated-source syntax before use; no permanent shared-entry repair is claimed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/dependent-entry/missing-fail-fast",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：One downstream draft-generator child ran after its prerequisite failed, so input-r1.json was absent and it failed ENOENT.",
    "recoveryEvidence": "Native764c51 retains the dependency failure. Subsequent preparation is now strictly sequential, waiting for each actual exit0 before continuing.",
    "permanentAction": "Owner: release incident preparer; reviewed 2026-10-11. Current calls are sequential and checked. Exit condition by 2026-10-18: shared orchestration stops on nonzero dependency exit and never invokes a missing-input consumer; no shared-script permanent fix is claimed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/preparation-input/concurrent-manifest-drift",
    "position": "before-production-write",
    "count": 1,
    "impact": "历史组影响（原失败仅按其实际候选成立）：One draft-builder invocation stopped because the current preparation manifest changed after its SHA was captured. The guard rejected different bytes; no input/ledger was generated.",
    "recoveryEvidence": "Native767f73 failure retained; exact preserved source-closeout-v2-preparation-r1.json was discovered and verified against b73055 in native075a15. Builder now binds the immutable original path; no hash guard was relaxed.",
    "permanentAction": "Owner: release evidence preparer; reviewed 2026-10-11. Bind immutable source revision paths, retaining the evolving current manifest separately. Exit condition by 2026-10-18: shared evidence handoff supplies immutable revision paths before consumers capture hashes; current path correction is recovery only.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/upstream-patch/windows-line-ending-byte-drift",
    "position": "before-production-write",
    "count": 1,
    "impact": "静态补丁字节等同性检查因 CRLF 转换中断；产品源码语义及运行行为未因此发生变更。",
    "recoveryEvidence": "static-patchcheck-r2.json 记录显式 LF 参数、两补丁 check/apply 退出 0，两个结果文件与官方原件逐字节一致；这里只是静态补丁检查恢复。",
    "permanentAction": "已实施：本次补丁回放调用显式 core.autocrlf=false/core.eol=lf，继续要求完整原件哈希一致。没有声称修改全局 Git 配置或完成通用永久修复。",
    "historicalReleases": []
  },
  {
    "fingerprint": "diagnostics/ssh-remote-argv/powershell-quoting",
    "position": "before-production-write",
    "count": 1,
    "impact": "一次 PowerShell SSH 参数解析在远端执行前失败，未修改远端状态。",
    "recoveryEvidence": "原 delivery 记录后续改用 Node argv 与 Python stdin，定向两阶段验证完成且原始证据回收；定向结果不代替最终候选 L3。",
    "permanentAction": "已实施：本轮后续远端控制器采用 argv 数组和 stdin 传递脚本。本项没有产品代码修复，也没有授权重新执行旧失败命令。",
    "historicalReleases": []
  },
  {
    "fingerprint": "diagnostics/dependency-discovery/missing-bounded-timeout",
    "position": "before-production-write",
    "count": 1,
    "impact": "一次只读 Go 依赖发现子进程无内置超时且无输出，实际 252.099 秒后由 owner 仅终止自有子进程；旧“超过十分钟”表述已保留并纠正。",
    "recoveryEvidence": "保留 stopped、correction 和 partial-source 报告；未取得该次完整发现成功结果。后续六环境生产包诊断另有真实完成回收证据，但不能把它当成旧工具已修复。",
    "permanentAction": "root 已明确采用本轮操作约束：本次发布后续停用旧无内置 timeout 的 Go reporter，改用有明确 timeout 的当前控制器。旧共享 reporter 未作通用修复，不标记为全局已修；原停止与修正记录继续保留。此处是本发布的调用约束提案，仍待 root 对本组最终事实逐项复核。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/controller-migration/replacement-string-corrupts-python",
    "position": "before-production-write",
    "count": 1,
    "impact": "一份生成的 Python 预览控制器因 JavaScript replacement-string 展开特殊符号而语法错误，未执行发布控制器或远端门禁。",
    "recoveryEvidence": "原 failure、错误脚本、callback 修正及29项语法恢复记录保留，C3/C4 静态审查仍按原候选引用。当前 C5 的51份发布控制器及10份 gate/common 已分别完成独立静态复核；纯静态结果不替代 C5 runtime。",
    "permanentAction": "已实施：字面模板替换使用 callback，生成后执行语法检查并独立绑定文件哈希；保留失败原件。此措施限本轮控制器准备，不声称产品运行修复。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/read-only-file-inspection/not-yet-created",
    "position": "before-production-write",
    "count": 1,
    "impact": "读取尚未创建的日志失败；该读操作失败本身不代表正在运行的 canonical gate 失败。",
    "recoveryEvidence": "原 owner-reported-process-observations 记载后续 native 6a0048 已读到真实日志；证据层级仍为 owner 对实际工具输出的记录，未伪造独立 raw transcript。",
    "permanentAction": "root 已明确采用：尚未生成的日志只表示 pending，不据此判 canonical gate 失败；读取动作与实际失败执行分开记载。原失败执行和 owner 观察原件保留，不补造独立执行转录或通用代码修复。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/apps-source-qualification/noncanonical-workspace-assumption",
    "position": "before-production-write",
    "count": 2,
    "impact": "两次只读资格化定位使用了非 canonical 工作区假设；未修改这些工作区。精确失败子命令退出码未单独保存，不补造退出码。",
    "recoveryEvidence": "原 apps-script-result 证明 canonical C:/GitHub/kejilion/apps；C3/C4 原观察保留。C5 又完成57个只读操作/114份 raw 的 source/apps/script 观察，root 对实际字节及退出状态独立复核，syncRequired=false；没有 apps 同步提交或运行验收结论。",
    "permanentAction": "已实施：本轮资格化绑定 canonical 仓库及实际 SHA/blob/hash，不按用户提及目录名推断真源；无需制造 apps 同步提交。",
    "historicalReleases": []
  },
  {
    "fingerprint": "candidate-validation/canonical-l3/baseline-declaration-schema",
    "position": "before-production-write",
    "count": 1,
    "impact": "51d canonical L3 因 helper 将缺省空 Replace 强制与 null 比较而中止；此前 Go/BT/race 结果仍仅属于其实际候选与已执行阶段，不冒称整轮通过。",
    "recoveryEvidence": "S4允许空Replace等价表达的最小修复、真实Go回放、定向验证、S5 qualification及C3/C4历史通过仍按各自候选保留。C5已取得自身canonical L3及publication-supplement实际passed，未借用历史PASS。其他声明与graph拒绝条件保持。",
    "permanentAction": "已实施的源码措施仅规范化允许的空Replace表达差异，其他声明和graph fail-closed保持；C5自身完整L3原件已绑定。历史失败、源码修复和最终候选运行通过分别记录，不主张额外的通用工具修复。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/static-evidence-review/misclassification",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地静态遍历把 root 所有的 watchdog 声明误分类，导致准备校验退出 1；watchdog、L3、产品均未在该调用执行。",
    "recoveryEvidence": "actual-controller-traversal-incident-r4 保留错误与修正；独立 r4 review 重新核验准确来源及 482 次哈希关系，无未关闭发现。",
    "permanentAction": "已实施：遍历同时识别由独立 root gate review 绑定的精确文件，不把它们冒充迁移器生成文件；保留原错误和独立复核记录。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/source-lineage-read-only-witness/fixture-path-rejection",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地源码见证器拒绝真实上游含空格/括号的两个 fixture 路径；Git 内容未变，未执行 BT 或发布门禁。",
    "recoveryEvidence": "C2 502、C3 503、C4 504产品见证全部保留原候选身份；当前 C5 对实际 Git 联合集合得到506产品见证/511变更路径，完整 source-lineage/provenance 与独立静态复核已完成。该只读见证不是产品运行或发布通过。",
    "permanentAction": "已实施：仅适配实际 Git 路径集合中的空格/括号，继续拒绝路径逃逸并按真实 union 逐文件核验；使用 execFile 参数而非 shell 拼接。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/static-evidence-traversal/hash-schema-misclassification",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地静态遍历错误地将 syntax 日志哈希与脚本 file 比较，在已经完成源码见证后退出 1；无发布或 BT 运行失败。",
    "recoveryEvidence": "原 traversal-schema-incident、C2 typed traversal r3 与 source-lineage 原件保留。C3/C4/C5 各按实际 files、syntax log 与 sourceSHA256 schema核对其候选文件；C5 typed handover 和独立静态报告已形成。普通临时文件读取错误不借本组追加计数。",
    "permanentAction": "已实施：分别处理 files:{file,sha256} 与 syntax:{file,log,sha256日志hash} 的真实 schema，未知形状停止，不猜测哈希所属对象。",
    "historicalReleases": []
  },
  {
    "fingerprint": "candidate-validation/baseline-manifest/windows-archive-line-endings",
    "position": "before-production-write",
    "count": 1,
    "impact": "S4 scanner 准备阶段的 Windows git archive 将 30 个认证源文件转 CRLF，validateBackport 在真实扫描开始前拒绝；并非新的 BT 运行回归。",
    "recoveryEvidence": "原Windows archive的30文件CRLF拒绝与453个源文件的LF字节恢复保留。当前C5 gate10保持显式core.autocrlf=false/core.eol=lf约束，C5本身canonical L3及其补充扫描均已通过；不改写原失败或放宽源哈希。",
    "permanentAction": "已实施：每次归档调用加 core.autocrlf=false/core.eol=lf 并执行精确源字节校验；不得放宽 manifest 哈希来迁就 CRLF。",
    "historicalReleases": []
  },
  {
    "fingerprint": "source-scan/trivy/tmpfs-exhausted",
    "position": "before-production-write",
    "count": 1,
    "impact": "S4 R2 的 Trivy DB 下载耗尽自有 128 MiB /tmp，扫描阶段失败；Go 阶段已完成，业务容器和 Agent 未变。",
    "recoveryEvidence": "R3任务私有disk bind越过/tmp空间阻断后遇到的真实漏洞阻断仍分开保留。C5本候选canonical L3与publication supplement已实际passed，完整扫描原件保留；不把后续恢复解释为旧128MiB配置可用或共享缓存可任意删除。",
    "permanentAction": "已实施：后续 scanner 使用精确自有 disk bind，原 CPU、内存、deadline 和磁盘/内存最低值不放宽；原缓存/报告保持证据隔离。没有清理共享缓存的新增授权。",
    "historicalReleases": []
  },
  {
    "fingerprint": "source-scan/trivy/source-scope-findings",
    "position": "before-production-write",
    "count": 1,
    "impact": "旧源扫描因真实 HIGH/CRITICAL 告警阻断。完整 selected 图包含 10 漏洞；上游 standalone 有 24 漏洞和 2 配置项，不能称为全图无漏洞或全部误报。",
    "recoveryEvidence": "C5本候选已实际完成canonical L3与独立补充资格化：production106外部模块0 HIGH/CRITICAL，selected460保留10漏洞，standalone150保留24漏洞和2配置项，真实exit0/1/1保留。六环境生产闭包和主模块源码身份受限期精确清单约束至2026-10-25；这不是全图无漏洞，也不是修复或永久豁免。",
    "permanentAction": "已实施：完整生产闭包、主模块源码/生成代码边界及精确 ID/module/version/scope 清单；未知、新增、过期、进入生产交集或未复核的身份变化均阻断。2026-10-25 截止与身份/范围变化再审保持，不延期、不称漏洞已修或永久豁免。C4更新firstParty身份，C5对Go/policy未再修改；原36条记录、closure和阈值不放宽。",
    "historicalReleases": []
  },
  {
    "fingerprint": "source-diagnostic/inventory/main-version-keyerror",
    "position": "before-production-write",
    "count": 1,
    "impact": "I1 在 selected scanner 退出 1 且输出十项漏洞后，由后处理读取合法 synthetic main 缺省 Version 触发 KeyError；另两 scope 未执行。只计这一调用一次。",
    "recoveryEvidence": "I2 后处理仅针对 synthetic main 的缺省字段适配，完整 expected inventory 保持；实际完成 selected/production/standalone，原 scanner 退出 1/0/1，32 raw 已回收。diagnostic-complete 不是 securityPass。",
    "permanentAction": "已实施：识别唯一 synthetic main 的合法缺省 Version，同时继续验证所有真实模块版本、完整包库存、JSON 与实际 scanner 状态；不通过吞退出码修复。",
    "historicalReleases": []
  },
  {
    "fingerprint": "terminal-browser/open/resource-conflict",
    "position": "before-production-write",
    "count": 1,
    "impact": "C3 terminal-browser-r1 实际完成 8 项内层用例后，在缺 Shell 场景收到 409 resource_conflict 而不是 container_shell_unavailable，整次执行失败并阻断发布；按一次失败计数。外层 cases=[] 是异常转录边界，不能解释为没有执行用例。",
    "recoveryEvidence": "C3实际8项后409失败、32次只读inspect和未保留旧夹具精确request/inspect对的限制均保留。C4最小Mounts排序修复曾恢复缺shell场景但整轮另因剪贴板失败；C5现在自身完整17项/18图通过，含missing-shell-message及root-after-error。无shell容器仍明确不能创建交互shell，旧CAP_KILL迁移未改。",
    "permanentAction": "已实施仅在版本计算中复制并排序Mounts，保留重复项/真实内容冲突与显示顺序；不增加自动重试或放宽断言。C5完整场景已通过；本轮无shell提示与既有CAP_KILL迁移范围保持。",
    "historicalReleases": []
  },
  {
    "fingerprint": "owned-cleanup/preflight/integer-precision",
    "position": "before-production-write",
    "count": 1,
    "impact": "一次真实自有清理控制器在删除前的 fresh inventory 比较处因 mtimeNs 整数经 JavaScript Number 序列化精度变化而停止；诊断记录 22,252 处差异且 deletionReached=false。它属于发布清理流程失败，不是 KPanel/BT 产品故障，也不是普通只读工具 schema 笔误。",
    "recoveryEvidence": "r2 独立授权后使用十进制字符串保留纳秒身份，重新核验备份、归属、进程、挂载及目标清单。实际 cleanup-receipt 记录恰好 7 个自有目标已移除、回收 951,377,920 字节，原证据、14 个业务容器和 Agent 未变；执行结果为 passed。root 已在 native 85d958 实际复核成功。r1 原失败仍完整保留。",
    "permanentAction": "已实施且限本轮清理控制器：整数身份按精确十进制字符串跨Python/JavaScript传递，逐字段不一致即拒绝删除，不设容差。C4已清对象与两旧RC1 bundle均仅保留缺席检查；C5清理仍只允许自己的三个精确目标，完整进程/挂载namespace/ignored树校验及真实收尾证据前提不变。当前仅静态准备，不清共享cache/image/volume、不称共享工具通用修复。",
    "historicalReleases": []
  },
  {
    "fingerprint": "terminal-browser/native-clipboard/paste-missing",
    "position": "before-production-write",
    "count": 1,
    "impact": "C4浏览器实际前15项通过后，plain Ctrl+V没有产生DOM paste而发送0x16，剪贴板验收超时并阻断发布。该一次失败仅计一次；同次neon镜像403鉴权被check_update转为502作为同一调用的附带外部条件保留，不再多计。",
    "recoveryEvidence": "C4前15项后plain Ctrl+V未触发DOM paste的真实失败保留。C5窄Vue分支的源码交付与独审后，当前C5实际终端浏览器17项/18图通过，含clipboard-paste及关闭后焦点，cleanup passed；native paste断言保持。已知neon registry鉴权失败按精确路径/status/code/detail/容器三元组保留，不宣称已修复外部鉴权。",
    "permanentAction": "已实施plain Ctrl+V窄分支返回false且不取消浏览器事件；保留capture paste/canPaste/其他快捷键和真实DOM paste断言，未改用其他快捷键或异步剪贴板读取。当前运行恢复仅覆盖隔离headless Chromium；精确neon允许项不能扩成允许所有502。",
    "historicalReleases": []
  },
  {
    "fingerprint": "normalization/draft/predecessor-mismatch",
    "position": "before-production-write",
    "count": 1,
    "impact": "本轮draft-r1实际退出0但输出predecessorCandidate错误地指向C3，形成一次无效候选证据；在seal/发布前发现。仅计一次证据生产流程事件，不记BT产品故障或伪称子进程退出失败。",
    "recoveryEvidence": "r1执行receipt的精确argv/exit/stdout与draft原件证明错误产出。r2仅将输出predecessorCandidate:H改为D，实际重新生成正确C4前驱；106事件与所有旧组未变。独立r1报告确认此修正；r3沿用同一r2 builder，仅append本次事件及RC1清理事实，实际51过程/108事件，9stages仍空、未seal。 最新r4实际52过程/109事件/32组仍沿用同一r2 builder；新增observe事件不改旧108事件/31组，最终owner模板只增加真实L3/terminal两阶段绑定。",
    "permanentAction": "已实施且限本次候选：使用已独审r2 builder，交接前显式核对输出candidate/source/immediate predecessor、全部旧事件/组等同性和pending状态。保留r1错误原件；后续最终owner输入仍须真实9stage和独立完整审定。未声称额外通用框架或新测试已实现。",
    "historicalReleases": []
  },
  {
    "fingerprint": "supplement/observe/stream-marker-interleave",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地发布补充证据observe实际退出1，阻断该汇总记录生成。产品C5、canonical L3原件与扫描真实退出码未变化；仅计一次外部汇总控制器过程失败，不记BT产品或扫描器故障。",
    "recoveryEvidence": "真实observe7a9c8f失败、旧helper和同一不可变L3日志保留；C5/C4纯解析回放、独立复核、root提升及实际observe恢复已绑定。后续C5 publication-supplement-result.json已于09:03:25.678Z完成独立资格化并passed。纯回放的旧parser预期拒绝不另计；失败精确start/finish仍未返回，不从回执observedAt反推。",
    "permanentAction": "已实施且限当前辅助器：严格校验12个标记、scope顺序与真实exit，仅接受正常顺序或production完整JSON尾部的已知标记交错；交错间隔必须为空白、所有正文保留，未知标记或混入正文拒绝。verifyScanReport/policy原字节及实际exit/完整库存校验保持。旧原件保留；后续出现其他交错须另行诊断，不能扩大忽略规则。",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/provenance/draft-tag-api",
    "position": "before-production-write",
    "count": 1,
    "impact": "Original tag Release failed at draft provenance lookup; original14 assets and immutable image remained; public preview was RC6.",
    "recoveryEvidence": "Actual reviewed REC6 apply recovered the original failed Release continuation: published 2026-10-11T11:19:26Z, 15assets/11sums, unchanged original14 assets, version/preview exact immutable image, stable/latest still1.25.1. Original Actions remains failure. G candidate CI/freshness passed; main integration is a separately pending closeout action.",
    "permanentAction": "Release owner: integrate G only after actual product qualification and exact main CI, then archive its branch; original failed Actions must remain failed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "recovery/download/timeout-budget",
    "position": "before-production-write",
    "count": 1,
    "impact": "R1 download timed out at90s with partial owned downloads; no publication write.",
    "recoveryEvidence": "R2 download completed exit0 under explicit20min budget. REC6 freshly revalidated all14 reused assets,11 sums and unchanged fresh draft.",
    "permanentAction": "The reviewed recovery controller reuses only exact verified original assets; future downloads must have bounded explicit adequate budgets.",
    "historicalReleases": []
  },
  {
    "fingerprint": "governance/verify-change/source-identity",
    "position": "before-production-write",
    "count": 2,
    "impact": "R1 had two failed regressions, R2 one: derived workflow scan hash and Windows physical vendor bytes. Neither failed candidate was published.",
    "recoveryEvidence": "Exact C5 Git blobs restored physically in owned GOV worktree, zero committed vendor difference, derived workflow scan hash synchronized; R3 actual300/300 and exact G candidate CI passed.",
    "permanentAction": "Preserve453 committed LF source identities and derive scan-policy hash from exact workflow bytes; never normalize the approved source manifest to physical CRLF.",
    "historicalReleases": []
  },
  {
    "fingerprint": "recovery/legal/checkout-crlf",
    "position": "before-production-write",
    "count": 1,
    "impact": "R2 legal comparison incorrectly compared downloaded LICENSE to physical Windows CRLF although all14 downloaded assets already matched GitHub.",
    "recoveryEvidence": "REC6 uses exact C5 git-show LICENSE/NOTICE blobs; both downloaded original legal files matched and original asset bytes were preserved.",
    "permanentAction": "Legal provenance must compare exact committed bytes, with physical checkout conversion excluded from identity checks.",
    "historicalReleases": []
  },
  {
    "fingerprint": "recovery/generator/readonly-mountpoint",
    "position": "before-production-write",
    "count": 1,
    "impact": "R3 OCI startup failed before Node because readonly source had no nested output mountpoint.",
    "recoveryEvidence": "REC6 created the uniquely owned empty host source/release mountpoint, mounted independent output, then verified empty/nonlink and removed it; actual generation succeeded.",
    "permanentAction": "Create only exact owned empty mountpoints before readonly nested bind mounts and validate/remove them after container cleanup.",
    "historicalReleases": []
  },
  {
    "fingerprint": "recovery/generator/docker-image-identity",
    "position": "before-production-write",
    "count": 1,
    "impact": "R4 rejected Docker29 containerd manifest .Image semantics. The originally planned R5 classic-daemon remedy subsequently failed and is not claimed as a successful fix.",
    "recoveryEvidence": "REC6 actual external collector succeeded with descriptor/digest/size/mediaType plus downloaded manifest/config binding; both architectures/four binary metadata generated; actual collector SHA is separately disclosed.",
    "permanentAction": "Release owner exception through2026-10-25 or before next stable release: canonical C5 remains unchanged; permanent Docker29 portability must receive normal reviewed candidate regression/CI before removing this exception.",
    "historicalReleases": []
  },
  {
    "fingerprint": "recovery/generator/shared-bridge-side-effect",
    "position": "before-production-write",
    "count": 1,
    "impact": "R5 private daemon initialization removed shared defaultdocker0 and prevented runner start; unchanged business PIDs did not prove network unchanged.",
    "recoveryEvidence": "Private daemon stopped and owned runner removed. Exact original bridge identity/gateway/MAC restored without container/service restart. Both affected gateway probes and published HTTP endpoints200; REC6 used only shared daemon and exact selected environment before/after equality.",
    "permanentAction": "Do not execute parallel dockerd on shared arena154. Reviewed REC6 removes that approach and checks selected network identities. Release owner retains full routing/firewall/external traffic verification as unperformed; no full network audit claimed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-provenance/recovery/evidence-allowlist",
    "position": "before-production-write",
    "count": 1,
    "impact": "Remote public verifier passed exit0; outer local mapping failed exit1 because frozen remote evidence allowlist had19 entries, new verifier21. Including two verification/public originals, total mappings are21→23. Original failure retained.",
    "recoveryEvidence": "Append-only reviewed Docker29 mapping binds all21 original evidence files plus2 original receipts unchanged, strict index/manifest/config/descriptor/arch/nonrunning/imageID checks passed; root actual recover succeeded nativee4b83d.",
    "permanentAction": "Release owner retains frozen original controllers and the reviewed compatibility mapping. Canonical Docker29 portability remains a dated owner exception until2026-10-25 or before stable; no public rebuild/republication to repair local mapping.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-browser/fixture/unix-socket-path-length",
    "position": "before-production-write",
    "count": 1,
    "impact": "Original headless browser fixture Unix socket pathname exceeded108bytes before any test container/browser creation; R1 failed and cleaned its created network/private directory.",
    "recoveryEvidence": "Actual R4 headless eight cases passed exit0 at2026-10-11T11:37:26.998Z, all52 raw originals recovered with SHA verification,8PNG actually inspected by root, zero console/resource/install errors and all owned fixtures/resources cleaned. R1 failure retained; R2/R3 never executed.",
    "permanentAction": "Use exact short owned /run fixture directory with independent browser /fixtures:ro mount and <108bytes socket guard. No test assertion, artifact identity, timeout or resource budget relaxed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "docs-l0/business-context/missing-canonical-metadata",
    "position": "before-production-write",
    "count": 1,
    "impact": "公开发布后，文档候选fb53be4的canonical L0在business context freshness处真实退出1：复核日期缺严格反引号格式，且新头部未保留基线提交/基线版本机器字段。未推送文档、未改变RC7产物，不是BT或产品运行失败。",
    "recoveryEvidence": "docs-closeout-r2/result.json及017-canonical-docs-l0.stdout.log/stderr.log保留原失败；按既有checker恢复三项canonical元数据，新文档仍须完整L0/精确metrics和候选/main CI。此记录形成时未提前声明新门禁已通过，后续成功以独立实际回执为准。",
    "permanentAction": "发布负责人：直接遵守extractBusinessContextMetadata严格三行语法；绑定真实C5祖先及其nearestStableVersion=v1.25.1，独立核新Git blob，完整L0通过后推送。不修改校验器、不放宽阈值或绕过门禁；本次只修正文档。",
    "historicalReleases": []
  },
  {
    "fingerprint": "docs-l0/windows-checkout/upstream-crlf-drift",
    "position": "before-production-write",
    "count": 2,
    "impact": "公开后的文档候选e9d049c的canonical L0真实299/300通过、1失败：Windows检出的30个上游文件被转换CRLF，触发Backport source digest mismatch。随后首次本地字节恢复在Git stat缓存仍报修改的后置断言真实退出1。两次流程失败均保留，不是已发布BT运行失败，未推送文档、未改变公开产物。",
    "recoveryEvidence": "docs-closeout-r3原失败result及017 stdout/stderr；docs-checkout-crlf-inspection-r1逐项核453个提交blob均匹配原清单、30个工作副本仅CRLF差异。恢复30文件的提交字节并保留原字节备份；04ed64的显式诊断refresh非零作为观察保留，不另计门禁失败；404c37精确git add刷新stat后index tree/HEAD不变、工作树clean、453 SHA全部一致。新完整L0/CI仍以实际新回执为准，未提前声明通过。",
    "permanentAction": "发布负责人：字节认证源码的Windows临时检出先验证实际453 SHA，检出时使用显式LF策略；发现仅换行转换后恢复精确Git blob并核index tree与commit不变。不修改产品源清单或放宽校验器。后续统一固定Linux发布环境；公开Docker29兼容限期仍保留。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

### 封存后的文档资格化失败

文档候选 `fb53be421245a3c14ca48a004ee69d175a4e8c46` 的 canonical L0 在复核日期格式处退出1，原 stdout/stderr/result 全部保留。按现有 checker 的严格语法恢复日期、产品基线和对应稳定版字段；未改变产品、公开 tag 或镜像。本记录当前总计65次/43组，原产品 seal 的62次/122事件/41组保持不变；新候选实际恢复以之后的完整L0和CI回执为准。追加记录 `post-public-docs-incident-extension-r1.json` SHA256 `88d9d1c578feaa1a4c783a34fad451947d24cf94f7256931c0188f4b92c8484c`。

另有文档候选 `e9d049cb9c7417d1b239b7d0d9fb9bbbe947bcd9` 的完整L0因本地Windows上游源码换行转换而真实299/300通过、1失败，首次恢复后置stat断言也真实失败；共追加2次。已将30个工作副本恢复为与Git提交和453项原始清单一致的字节，备份保留，HEAD/index tree未变且clean。完整新L0及CI仍须实际通过。追加记录 `post-public-docs-incident-extension-r2.json` SHA256 `63257f486d528f9cc95ce329b1ca18e67ca1b27e14e57007a0810aefeef2f64e`，本地字节恢复回执 `docs-checkout-crlf-repair-r1/receipt.json` SHA256 `2af71d784b88e53d64921208c4436b02691c9861de54cba3265c1a2ee1fc4968`。

## 证据索引

以下文件名属于本轮外部冻结证据集 `v1.26.0-rc.7-candidate-6ee57f09`；原件在本地证据与隔离154原始归档中保留，不将本记录当作二进制资产替代品。

| 回执 | SHA256 |
| --- | --- |
| closeout-r1/qualification.json | 2e8c7bb0e206878651b7919bfbd3cf81d12e999ae62408ecd3d6a2846f20d12c |
| public-closeout-final-independent-review-r1.json | 4244f77bc85418f1e8335f179dd937392af4e52bec8ac4b13ff56c4649673942 |
| public-after-r1/result.json | 1421fdde34566cf1dbd7313a05cb6d9bda5495e3e1a92fb21d557e8ef5fea96e |
| public-after-r1/modified-component-local-verification.json | cf896d0826bfb18198cd5e500eeac1a24dede22a26fbbb3b7986145132be469f |
| public-image-e2e.json | 85d91905504ed6cecf1835088011d16513dbfce269899e871d78aa7535c8f44f |
| public-browser-result.json | 232c3ffdd0eb58f2a10e558e567ab89488ae74f906ea9f85317df231eb81d5eb |
| public-notes-visual-review.json | 773ea12f1bed98f3810cce7678192547e78f149417d187777bed15a5adf7856a |
| terminal-browser-qualified.json | bc3df6214b7b4ddfd1838a230fe906822d9dce02eced75b36e1d86bb784af585 |
| tag-release-recovery-r6/apply/receipt.json | 7864d3e8b3763daf1d827c75ab9aa9feb263558a833d8241052cdf0ed1ddaec7 |
| apps-script-incident-evidence-r1/incident-normalization-r5/normalized.json | b81e3a027d4957ea1cc4cfef53cac8048f891f79b3c8cd7a72919d01b5ed2829 |
| apps-script-incident-evidence-r1/incident-normalization-r5/normalization-receipt.json | b576342ed89ec33a625513a73022e7bfecee64023af5cbda66dc67ecd2c16f13 |
| governance-e564b6af/governance-integration.json | 6ee0a0d4207c38e9dafc317699b892448b3340a157606074a194fa1c648acee3 |
| governance-e564b6af/archive-r1/result.json | 7b6c69f75147952f0c9e7d9cd1121ce315621904067ff8ef1cb3cd8cbdfcdc38 |
| governance-e564b6af/cleanup-r1/result.json | 0941f28e8e834e299b784933c4fb4713e4c8197bdc374eb814e332ce03cad5d9 |
| source-latency-archive-r1/result.json | 02c5ddb7d8e2f938a98207a5c1146164a75fd5bdaa6c63f9805a34c6e67b7346 |
| source-latency-cleanup-r1/result.json | 66f61fce04dca4324c84892b098e03a6b4d592f0a255d1499e73a1ff0a2e74e5 |
| source-cursor-archive-r1/result.json | 76a5100f823194744f0002d834d2149164bdf4506b8671fab9cb8c54990d6020 |
| source-cursor-cleanup-r1/result.json | 9d60f0525b6f4051614517e23dc750259697076fbcd2c91350cf8a28df22a458 |
| source-bt-archive-r1/result.json | f80594c30248ac2756f368b4410a2c49d46ad20b9ba1cb2ee455fb07d7f41869 |
| source-bt-cleanup-r1/result.json | 1ebc304aa78ca0ea6316ae5aa5fd7238b11694b0e0509e35b8f2d96e773de455 |

上述公开资格只针对产品提交；文档候选仍须自己的 L0、精确 blob 事实审查、候选/main CI 及归档清理回执。此处未提前填入未来提交 SHA 或声称未来操作成功。
