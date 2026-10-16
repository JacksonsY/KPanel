# KPanel v1.27.0-rc.2 发布验收记录

日期：2026-10-16。L3，`releaseChannel=preview`，`releaseTrain=1.27.0`。产物状态见下方回执；生产未部署。

冻结产品提交 `7c3441d8560b4727de634d1a15dde7b776030af8`；批准集成基线 `77c811a2a698d505906f282d3117408889638bf9`（前轮批准治理基线7cb71fe4）。上一预览 `v1.27.0-rc.1` / `3df93782f554f139a53d74e3cc02d163aaa05aaa` / `sha256:92ac0d50c95c427d460e9a9ff027c6d8029aa3af3f81725047be147684b278f9`。上一稳定 `v1.26.0` / `247ff67d98700c9af56d55a807cb9389fc8eedc1` / `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。

## 发布范围与来源

用户要求今天未汇入主线的候选全部跟预览发布，本轮纳入四个已列分支、三个功能组。原始语言分支完全包含于质量分支，不重复应用。rc.1 的终端和三个小修复继续保留。

| 来源分支 | 精确 tip | 本轮处置 |
| --- | --- | --- |
| claude/apps-local-source | `dfc9f1896a75df941f05cd453f2d554210318109` | 本地配置、图标与单流程入驻；10 个功能提交重放。 |
| claude/panel-access-https | `0e2478d3a9694d9f8465d3925784ce9b1f5e4b0a` | 同端口自签 HTTPS、域名归属验证、入口切换与离线恢复；5 个功能提交重放。 |
| feature/language-desktop-sync | `1a1272ffc62f8da3a900b715c1fdcb70b20564e6` | 包含于下项；不作为另一份功能重放。 |
| feature/language-desktop-sync-quality | `9071673eb20c440d7a9f61e8cc41edb245250f4d` | 服务端语言/桌面偏好、迁移、冲突、会话切换及失败重试；2 个功能提交重放。 |

17 个非空功能提交的实际加删行及重复计数与来源全部相同；两个三路重放 patch-ID 差异已逐项核实为上下文差异。旧来源 scan-scope 摘要由组合身份重新核验；跳过的空审查元数据不冒称功能被遗漏。精确映射在外部 `source-replay-equivalence.json`。单独修复工作树承担本轮修正，唯一列车候选 `release/v1.27.0-candidate` 接收快进。

整合修正：图标先验证尺寸再完整解码；域名成功响应必须带本实例独立随机凭据；新 Teleport 弹窗显式响应语言变化；语言存储说明、局部字号及旧脚本兼容提示。两个后端回归和两语言 Teleport 回归均有实际 fail-before/pass-after。未扩大为新功能、依赖升级或 CF 审计。

风险 L3：新增认证管理下的宿主配置写入、root 脚本契约、TLS listener 和用户持久化。保存配置只解析/检查，不执行用户脚本；应用操作沿用现有特权边界、Origin/CSRF、审计和能力校验；AI 工具拒绝代执行本地应用。

## 审查与依赖

发布接管者独立于来源作者审查。Claude CLI unavailable，对 Codex 语言来源采用独立会话、同提供方 fallback，原因在 `9470885e` 的 Independent-Review trailer。OCR 1.12.11 先自由臂再约束臂，组合最终覆盖 **66/66**，字体增量4/4、最终界面增量9/9；`blind=false`，`constrained-only=unreported`，不计算为工具独有发现。文档/锁文件按原选择规则排除，另核对版本与实际字节。

CF `not-run`；本轮不声明 CF 完成、覆盖通过或线索关闭。按当前主线 `PROJECT_RULES.md` 5.4，CF 为可选辅助，不需要补审或豁免。rc.1 的历史限期豁免记录保持历史事实，不据此伪造新审计结果。

固定 Go1.27.2 原始 vendor 453 文件摘要、六目标 actual/original 共12闭包核验通过。生产依赖仍110模块，selected graph=`24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927`，closure=`98ac00a5cdff85d5e3369149a0e5fc5f3c7e3ec2f73e8cb8ea50ff0469896cae`，首方 Go（含测试）=`2ad9e8027546439552e793117a585e5ea974bfa82fb6301b47320e2b3cf442ce`。既有36条 finding及2026-10-25到期日未延长，未将 selected/standalone 图说成零漏洞。

无第三方依赖、Action、扫描器或基础镜像升级。b1e中间候选 Dependency freshness [38893635786](https://github.com/kejilion/KPanel/actions/runs/38893635786) 实测失败，exit3；必需9类检测源均ok，CF为not-requested。TypeScript7、@types/node26 两项既有例外due仍待KPanel基座维护决策，不称所有工作流或新鲜度全绿；原始报告在`freshness-failed.log`。EOL复核原到期2026-10-28不改写。最终7c3441d8标签触发的Dependency freshness [38909301871](https://github.com/kejilion/KPanel/actions/runs/38909301871)仍以exit3报告相同两项due，9类必需源ok；当前检测到@types/node26.6.5，既有例外目标26.6.3未改写。原件freshness-final-r2.json和freshness-final-tag-failed.log保留；前轮77c主线38902748036及原件也保留；该结果未冒充发布CI成功或被静默忽略。

## 跨仓库联动及成对回滚

`scriptLinkageState=coupled`，变更集 `v1.27.0-rc.2-local-apps-protocol-v1`。配套官方 `kejilion/sh` 已公开 commit=`160924a498d9b8d071fed72f165d23c1b9e886de`，根脚本SHA-256=`39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`。本轮验证既有官方发布，不另推脚本。根/CN唯一允许的地区参数差异保留；两份 `bash -n` 和原始 local config smoke 通过，公开 immutable raw 字节与Git blob一致。

新协议 `KPANEL_LOCAL_APPS_PROTOCOL_VERSION=1` 与 `local:<token>` 独立入口。旧脚本拒绝新本地应用操作，UI解释需要更新；官方同名优先、原内置别名不受本地命名碰撞影响。配套新 Panel/Agent/script必须一致，不能只换前端。

隔离成对回滚 r2：真实 rc.1 Panel/Agent/旧内置脚本 `5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c` 初始化数据；备份副本升级新组合，写入偏好及本地配置；恢复旧组合及对应旧数据，核实旧外观和新配置不存在；再次恢复新组合及新数据，偏好与配置恢复。四阶段各5项检查通过，保存/回滚均未执行配置内 touch。r1 夹具未提交完整外观DTO，实际422保留，补齐字段后r2通过；不改产品掩盖失败。旧7f6镜像执行的后端专项对应 Go源码、构建配置和脚本原件与最终提交逐字节相同，不据此复用公开镜像身份或整套L3；最终L3和公开镜像验收仍分别执行。

应用市场官方 `kejilion/apps@25e7487ad5e2efce21fd57805bde9268c680a525` 的 `kpanel.conf` 与打包配置字节相同，SHA-256=`2389669ddbad7d58536ef76ad477e55a0718e2968611034cca7348170cbb41ea`，仍默认 `kjlion/kejilion-panel:latest`。本地市场工作区有他人/未知改动，未写入；不把它当公开真源。

## 自动门禁

唯一完整 L3入口 `scripts/run-release-l3.mjs`，`arena-154`。Runner=`kpanel-go127-security-runner:go1.27.2-node24.21.0` / `sha256:4edff59fa4c1441e3e97594593b255cc57a74e51dabbdd8db295e4ff51d0ca4f`。

最终 L3 r8 **已验证**：2026-10-16T12:57:22Z → 2026-10-16T13:20:16Z，exit0；12份原件摘要回收通过，handoff资格化通过。候选 CI [38907220157](https://github.com/kejilion/KPanel/actions/runs/38907220157)、主线 CI [38908245342](https://github.com/kejilion/KPanel/actions/runs/38908245342)、Release workflow [38909301855](https://github.com/kejilion/KPanel/actions/runs/38909301855) 均在精确产品SHA通过。草稿与公开正文均通过唯一门禁；正文含5条更新、4条升级提示，API/UI按既有8/3展示限额呈现5条更新、3条升级提示。

环境：arena-154，Debian13.7/trixie、kernel6.12.107+deb13-amd64、Docker29.6.2/API1.55 amd64；宿主Node24.18.0，固定Runner Go1.27.2/Node24.21.0；fixture OpenSSL3.5.7、Python3.13.5；浏览器Playwright1.62.1，固定镜像mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e。源码分组实测耗时web=715922ms、go=1030405ms、deploy=3285ms；并发分组总计1030468ms，不能将分组相加当墙钟耗时。远端完整L3门禁区间1374000ms。CI/Release API观察区间：38907220157: 2026-10-16T13:21:28Z→2026-10-16T13:29:01Z；38908245342: 2026-10-16T13:29:34Z→2026-10-16T13:36:57Z；38909301855: 2026-10-16T13:37:38Z→2026-10-16T13:52:58Z，updated_at是API终态更新时间，不冒充净执行耗时。阶段原始日志与身份见外部evidence及environment.json。

本轮 r1 因非稳定base-tag被入口拒绝；947候选r2因字号修正主动停止，exit137留存；7f6候选r3完整通过，但随后翻译修改使整套发布资格失效；245候选r4因并发main治理前移被入口拒绝；合并批准main后b1e候选r5完整通过，但候选CI38893635778发现新增Agent测试依赖真实/root目录，在非root Runner下失败。通过UID65532重现旧失败后，将缺失文件检查移到既有TempDir服务夹具，路由早期拒绝不读取宿主目录；两个修正测试均通过。只改变两测试文件及扫描身份记录，12份闭包/110生产模块/生产GoFiles不变；修正77c候选r6因退出141中断，未记通过。77c候选r7完整通过且12原件摘要校验通过、候选CI38901567749通过；主线CI38902748171随后暴露既有cluster文件转发测试的确认/响应排队顺序假设，body acceptance实际先收到response。生产client允许这类交错，测试现用handler同步信号和closeAll清理，全部原断言保留；Windows同步修正阶段（追加closeAll前）的用例10000次及整个cluster包通过；追加清理后的最终代码在固定Linux用例race10000次及完整L3通过。旧源码Windows1000次未复现，不声称本地fail-before。最终7c3441d8按同一官方入口r8持久运行，结果见本节回执。治理合并未改产品树；已通过的54项UI矩阵按精确Web树及原环境/工具复用，不跨SHA复用完整L3。没有删除失败、跳过检查或重写原始日志。

## 多维质量与隔离验收

| 维度 | 状态 | 证据与实际限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | Panel/Agent真实Linux CRUD、icon、版本409、无效422、匿名/Origin/CSRF拒绝、CLI改文件后回读及冲突；不证明所有用户自定义安装脚本。 |
| 网络入侵与供应链安全 | 已验证 | 自动扫描/固定供应链身份、图标解码预算回归、TLS与域名6边界；CF未运行，不声明全面安全。 |
| 稳定性、失败恢复与兼容 | 已验证 | Panel+Agent重启、证书指纹不变、offline CLI关闭、坏证书HTTPfallback；数据/二进制/script成对回滚；无生产systemd升级或WAN soak。 |
| 性能与资源预算 | 已验证 | 配置/图标/窗口/探针限额、失败重试有界及资源受限隔离旅程；浏览器采样和watchdog保留，无长时负载基准或原生ARM运行。 |
| 用户体验与可访问性 | 已验证 | 54项矩阵、附加6字号+2键盘检查；实际三语言文案、两主题，1280/640/390/195px及DPR2放大模拟；辅助>=13px、操作/错误>=14px，编辑器>=12px；非实体缩放/读屏证明。 |
| 数据、配置与迁移 | 已验证 | 偏好持久化、跨浏览器/刷新/重启恢复、409与无效DTO拒绝、备份恢复及命令不重执行；写文件不是跨文件事务承诺。 |

最终 clean acceptance mock预览 `http://127.0.0.1:4175`，manifest=`preview-relay-final/manifest.json`，绑定7c3441d8；只证明UI。54项原运行绑定245，Web树与b1e、77c和7c3441d8一致，身份比较在`merge-evidence-identity.json`及`source-final-binding-r4.json`。r3/r4旧矩阵仅命名语言/主题不充分，已明确作废；r5检查实际 html.lang、data-theme、标题和服务端theme。两个补充字号作业因翻译完成后中文定位器失效，失败保留；改语言无关定位后r3通过，不修改产品。

真实后端candidate r1：42+14项通过；Docker库存仅只读fixture，域名使用测试CA及6种受控服务，Panel/Agent API和TLS实现均为真实；fixture无宿主Docker socket，未安装/卸载任意第三方app。日志区分测试信任根与实际自签名证书下载/指纹/Secure cookies，不把ignoreHTTPSErrors用于域名服务端归属校验。

[公开预览Release](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.2) 已发布、非draft、prerelease；GitHub Latest仍v1.26.0。不可变版本及preview清单均为 sha256:207aa4614c703914df69dcd2e1a392ed55f4864ce31d971bd910e02ee667f659，双架构身份如下：

| 平台 | manifest digest |
| --- | --- |
| linux/amd64 | sha256:47973738a449a33eec525e752ed44ecdd2b7534b7cb6c4e6a33099355676230d |
| linux/arm64 | sha256:c168730b8929321b46ac7c361da3da8bab7f930328fcb4cebf0561b3ba98631a |

两平台OCI revision均为产品SHA，内置script commit/digest均匹配coupled身份。公开15附件下载/流摘要/磁盘回读/API摘要全部一致，SHA256SUMS11项全通过；保留SBOM/provenance与版本原件。

公开镜像官方image-e2e通过；真实本地应用、TLS与重启专项42+18项通过，其中active TLS证书损坏HTTP fallback单独注入验证；公开终端/通道旅程37项、旧rc.1后端配新Web的更新弹窗/真实503恢复31项通过，pageerrors均为0。三种弹窗宽度1280/390/320无横溢出，公开正文5+4，实际API/弹窗按限额5+3、digest和来源URL一致，加入预览不自动安装、退出不降级。混合版本仅用于升级失败UI验收，不作为正常部署组合；无systemd升级或生产验收。公开验收均使用冻结digest，不以可变preview标签作为执行目标。

## 生产与回滚

不适用（预览版禁止生产部署）。本轮未连接、备份、部署、升级或核对prod-108；arena仅登记隔离测试，不改变其既有服务。稳定GitHub Latest、Docker latest和标准应用市场入口保持1.26.0。

恢复时从已保存数据备份整体选择rc.1 Panel/Agent/image/script，恢复对应Panel数据、本地配置及图标；验证版本、登录、健康和脚本身份。不能只换镜像而假定新增状态向后兼容。本轮成对副本演练不等于真实宿主systemd升级/回滚。旧脚本、不可变镜像和来源精确提交保留。

## 分支及资源处置

远端候选release/v1.27.0-candidate及main在产品发布时均为7c3441d8560b4727de634d1a15dde7b776030af8，rc.2注释tag指向同SHA；RC列车候选按规范保留。来源四分支精确tip已逐项复核并纳入本版本，作者活跃worktree/预览所有权尚未明确释放，分类“已纳入RC2、本地待处置”，不再算待发布候选；由后续稳定列车或明确所有权释放触发归档核对，不强删。

唯一integration工作树及4175验收预览保留，旧compiled4187和专属SSH tunnel已按命令身份停止，4177作者预览保留。验收文档由docs/release-v127-rc2-acceptance独立提交，按候选CI→main快进/CI→原子expected-SHA lease归档到archive/docs/release-v127-rc2-acceptance顺序收尾，实际最终回执外部docs-closeout.json，不改产品tag。

只删除本轮明确拥有且不再被运行容器挂载的work/inbox/cache/fixture和测试数据，全部bundle已验证且原始证据、失败日志、截图、不可变镜像与runtime保留。远端清理实际可用空间14350979072→19799822336字节，观测净增加5448843264字节；不以预估代替结果。

本地证据根 `C:/GitHub/_release-evidence/v1.27.0-rc.2`；最终L3远端 `/root/kpanel-release-evidence/v1.27.0-rc.2-7c3441d8-l3-r8`。来源引用逐项实测（仅本地应用有远端分支，其余三个远端引用查无结果、本地精确引用保留）：

| 来源 | 本地精确tip | 远端复核 | 工作树 | 处置与恢复 |
| --- | --- | --- | --- | --- |
| claude/apps-local-source | dfc9f1896a75df941f05cd453f2d554210318109 | present-exact-tip | clean，所有权未释放 | 已纳入RC2；精确本地tip及回放映射可恢复，待所有权释放 |
| claude/panel-access-https | 0e2478d3a9694d9f8465d3925784ce9b1f5e4b0a | absence-verified | clean，所有权未释放 | 已纳入RC2；精确本地tip及回放映射可恢复，待所有权释放 |
| feature/language-desktop-sync | 1a1272ffc62f8da3a900b715c1fdcb70b20564e6 | absence-verified | clean，所有权未释放 | 已纳入RC2；精确本地tip及回放映射可恢复，待所有权释放 |
| feature/language-desktop-sync-quality | 9071673eb20c440d7a9f61e8cc41edb245250f4d | absence-verified | clean，所有权未释放 | 已纳入RC2；精确本地tip及回放映射可恢复，待所有权释放 |

原始失败、bundle、计划、源码回放、摘要、截图、资源采样及回滚报告保留。归档与生产状态分别记录，验收文档由独立docs候选CI→主线CI后归档，不改产品tag。

## 交付节奏和异常

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-15T19:09:24+08:00
- 候选冻结时间：2026-10-16T20:55:22+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（未部署生产；隔离副本回滚演练另记）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

本轮无生产变更失败；流程事件时间另记：非root夹具失败发现2026-10-16T11:27:20Z；修正及恢复验证见nonroot-regression-r1和最终候选CI。未发生已确认生产故障；其余时间未测不反推。

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：38
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

早期外部事件记录的二段指纹仅补充实际入口，原名映射见外部incident-fingerprint-canonicalization.json；原件及计数保留。以下为规范机器可读原件：

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preparation/task-preflight/incorrect-phase-position",
    "position": "before-production-write",
    "count": 1,
    "impact": "Ready 首次使用不存在的 --phase 参数，未获得有效资格化。",
    "recoveryEvidence": "ready-r1.json 正确以 ready 位置参数重跑",
    "permanentAction": "本轮按入口 usage 固定 argv；维护者后续复用同一接口。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/script-identity/insufficient-output-buffer",
    "position": "before-production-write",
    "count": 1,
    "impact": "脚本原始 Git blob 超过 Node 默认 1MiB 输出限制，读取被 ENOBUFS 拦截，未取得摘要。",
    "recoveryEvidence": "verify-script-source.mjs 调整为有界 8MiB 后重新比对官方 raw/根目录/CN 原件",
    "permanentAction": "本轮固定有界容量覆盖当前约1MiB脚本；维护者下次脚本更新复核大小与限长。",
    "historicalReleases": []
  },
  {
    "fingerprint": "script-identity/verify-script-source/windows-network-reset",
    "position": "before-production-write",
    "count": 1,
    "impact": "Windows raw Github ECONNRESET; no digest obtained",
    "recoveryEvidence": "Arena download and Git blob byte identity passed",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "script-identity/verify-script-source/regional-byte-difference",
    "position": "before-production-write",
    "count": 1,
    "impact": "Strict root/CN byte equality rejected real regional canshu setting",
    "recoveryEvidence": "Only default/CN regional difference allowed; semantic and published bytes verified",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "regression/docker-run/environment-noexec-tmp-budget",
    "position": "before-production-write",
    "count": 1,
    "impact": "First owned Runner /tmp noexec and undersized tmpfs prevented regression execution",
    "recoveryEvidence": "Preserved failure; disk-backed allowed tmp then two expected fail-before and pass-after",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "inventory/git-archive/native-archive-eol",
    "position": "before-production-write",
    "count": 2,
    "impact": "Native Windows archive CRLF invalidated vendor byte digests",
    "recoveryEvidence": "Explicit core.eol=lf and autocrlf=false; all 453 fixed vendor hashes and canonical inventory passed",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/node-import/windows-esm-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "Bare C: import URL rejected",
    "recoveryEvidence": "Existing file:/// import convention used",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "handoff/task-preflight/premature-evidence",
    "position": "before-production-write",
    "count": 1,
    "impact": "Handoff called before candidate/evidence qualification complete",
    "recoveryEvidence": "Retained handoff-r1.json; final exact L3 receipt required",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "l3/run-release-l3/nonstable-base-tag",
    "position": "before-production-write",
    "count": 1,
    "impact": "RC supplied as base; canonical entry rejected before product checks",
    "recoveryEvidence": "Stable v1.26.0 base supplied",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "l3/run-release-l3/superseded-typography",
    "position": "before-production-write",
    "count": 1,
    "impact": "9470885e running L3 withdrawn for observed new UI font issues",
    "recoveryEvidence": "Only owned Runner stopped; exit137 retained; typography fixed",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "l3/run-release-l3/superseded-localization",
    "position": "before-production-write",
    "count": 1,
    "impact": "7f6a4b9a full L3 passed but later UI localization fixes invalidated release SHA",
    "recoveryEvidence": "Old passing evidence retained; backend-only byte identity reused; final full L3 rerun",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/playwright/local-binary-unavailable",
    "position": "before-production-write",
    "count": 1,
    "impact": "Bundled Playwright browser missing locally",
    "recoveryEvidence": "Installed Chrome used for short scout; long tests use pinned registered Playwright image",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/compiled-preview/owned-port-conflict",
    "position": "before-production-write",
    "count": 1,
    "impact": "4177 already owned by another preview",
    "recoveryEvidence": "Existing process preserved; owned compiled helper moved to4187",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/start-process/automatic-approval-block",
    "position": "before-production-write",
    "count": 1,
    "impact": "Combined PowerShell background startup blocked by automatic policy review",
    "recoveryEvidence": "Used tracked foreground process successfully; no bypass",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/resource-exhaustion",
    "position": "before-production-write",
    "count": 2,
    "impact": "Mock r1/r2 context/target crashed, resource evidence retained",
    "recoveryEvidence": "Close prior context before desktop refresh, bounded2GiB browser and memory watchdog; r3 onward passed",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/insufficient-locale-theme-evidence",
    "position": "before-production-write",
    "count": 1,
    "impact": "r3/r4 labels alone did not assert actual locale/theme; screenshots exposed untranslated Teleport content",
    "recoveryEvidence": "Product explicit phrase rendering plus fail-before/pass-after; r5 server theme writes and actual language/theme/heading assertions, 54checks passed",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "frontend/check-page-i18n/stale-dynamic-phrase",
    "position": "before-production-write",
    "count": 1,
    "impact": "build-r4 rejected catalog phrase found only in Go reason",
    "recoveryEvidence": "Conditional old-script UI notice added with same actual reason; build-r5 passed",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "l3/run-release-l3/concurrent-main-advance",
    "position": "before-production-write",
    "count": 1,
    "impact": "245518bc entry rejected because approved origin/main advanced three governance commits",
    "recoveryEvidence": "Reviewed exact governance diff, merged7cb71fe4 without product changes, exact final b1e7782a fullL3 started",
    "permanentAction": "Follow current project-management6.2.2; preserve originals, fix prerequisites before affected-layer retry; no skipped checks",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/local-card-fixture/localized-selector",
    "count": 2,
    "impact": "Source/category/card font assertions passed in r1; later Chinese-only locator timed out after legitimate server locale hydration. r2 corrected only one localized locator and retained another; no complete pass claimed.",
    "recoveryEvidence": "local-card-job-r3: all source selectors made language-independent; 6 font checks plus focus/keyboard checks passed",
    "position": "before-production-write",
    "permanentAction": "Use observed inputs and stable selectors; fix all same-cause prerequisites before retry; preserve each original attempt.",
    "historicalReleases": []
  },
  {
    "fingerprint": "paired-rollback/api-fixture/incomplete-appearance-dto",
    "count": 1,
    "impact": "Old appearance PUT rejected422 because fixture omitted required wallpaper/classicLevel.",
    "recoveryEvidence": "Reviewed actual DTO and preserved all required fields; paired-rollback-job-r2 four phases passed",
    "position": "before-production-write",
    "permanentAction": "Use observed inputs and stable selectors; fix all same-cause prerequisites before retry; preserve each original attempt.",
    "historicalReleases": []
  },
  {
    "fingerprint": "review/coverage-qualification/missing-inventory-receipt-source",
    "count": 1,
    "impact": "Coverage merger correctly rejected scan-scope file absent from early64-file receipt; no complete coverage emitted.",
    "recoveryEvidence": "Used existing final65-file receipt containing separately reviewed inventory plus final9-file UI receipt; ocr-coverage-merged.json65/65",
    "position": "before-production-write",
    "permanentAction": "Use observed inputs and stable selectors; fix all same-cause prerequisites before retry; preserve each original attempt.",
    "historicalReleases": []
  },
  {
    "fingerprint": "app-market/contract-input/stale-local-path",
    "count": 1,
    "impact": "Requested local root kpanel.conf absent; discovered separate dirty source checkout and did not use it as publication truth.",
    "recoveryEvidence": "Read exact official GitHub main commit25e7487a and immutable kpanel.conf blob; packaged bytes equal; app-market-contract.json",
    "position": "before-production-write",
    "permanentAction": "Use observed inputs and stable selectors; fix all same-cause prerequisites before retry; preserve each original attempt.",
    "historicalReleases": []
  },
  {
    "fingerprint": "candidate-ci/test-fixture/production-root-permission-assumption",
    "count": 1,
    "impact": "Exact b1e CI38893635778 failed: missing-file checks accessed /root/apps-local as non-root, returning503 instead of404. Root L3 could not expose the assumption.",
    "recoveryEvidence": "UID65532 binary reproduces both errors before; corrected early route tests and temporary-root missing-file tests pass. Canonical12 closures unchanged, source digest rebound. New77c fullL3/CI required.",
    "position": "before-production-write",
    "permanentAction": "Use actual schema and isolated fixtures; preserve original failures. Run long canonical entry with durable logs; remote exact-source status and hashes determine completion.",
    "historicalReleases": []
  },
  {
    "fingerprint": "freshness/read-interface/unsupported-json-field",
    "count": 1,
    "impact": "gh run view --json artifacts was rejected; no run state obtained.",
    "recoveryEvidence": "Read documented run/artifact API endpoints; actual freshness failure and two due items preserved.",
    "position": "before-production-write",
    "permanentAction": "Use actual schema and isolated fixtures; preserve original failures. Run long canonical entry with durable logs; remote exact-source status and hashes determine completion.",
    "historicalReleases": []
  },
  {
    "fingerprint": "inventory/binding-parser/unverified-review-field",
    "count": 1,
    "impact": "Binding helper assumed qualification field; actual schema uses review. Failed before product file mutation.",
    "recoveryEvidence": "Inspected actual top-level and review keys; updated verified review.summary/evidence and exact new source hash; validateScanScope passed.",
    "position": "before-production-write",
    "permanentAction": "Use actual schema and isolated fixtures; preserve original failures. Run long canonical entry with durable logs; remote exact-source status and hashes determine completion.",
    "historicalReleases": []
  },
  {
    "fingerprint": "l3/session-lifetime/incomplete-exit-141",
    "count": 1,
    "impact": "r6 remote ended141 at2026-10-16T12:00:09Z; original terminal session disappeared and final Go/Web lane receipts were not captured. No complete gate pass claimed.",
    "recoveryEvidence": "Container already exited, originals retained. Lost stdout pipe inferred, exact parent termination reason unconfirmed. Same canonical entry launched persistently with independent stdout/stderr logs and newr7runID, no gate/source changes.",
    "position": "before-production-write",
    "permanentAction": "Use actual schema and isolated fixtures; preserve original failures. Run long canonical entry with durable logs; remote exact-source status and hashes determine completion.",
    "historicalReleases": []
  },
  {
    "fingerprint": "main-ci/test-fixture/relay-response-ack-order-assumption",
    "position": "before-production-write",
    "count": 1,
    "impact": "Main CI38902748171 failed existing relay test at2026-10-16T12:48:29Z: handler response can queue before body acknowledgement. Candidate CI and r7 L3 had passed, exposing scheduling-sensitive fixture.",
    "recoveryEvidence": "main-ci-failed-r1.log retained; deterministic handler synchronization and t.Cleanup(relay.closeAll); Windows10000 repetitions/package and final fixed-Linux race10000 passed. Complete exact-source r8 L3 and CI required.",
    "permanentAction": "Keep event ordering explicit in fixture; no sleeps, ignored failure or production relay modifications. Preserve every previous receipt and rebind source inventory.",
    "historicalReleases": []
  },
  {
    "fingerprint": "inspection/windows-rg/literal-wildcard-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "One rg call used internal/cluster/*test* as a literal Windows path and failed; no conclusion derived from missing output.",
    "recoveryEvidence": "Read exact test/production files and actual client event loop; subsequent searches use directories with -g filters.",
    "permanentAction": "Use rg directory roots and -g filename filters on Windows.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-tools/preflight/shared-runtime-copy-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "Pre-execution check found copied browser harness referenced a non-existent per-copy runtime. No browser job used the invalid binding.",
    "recoveryEvidence": "runtime-preflight.json confirms canonical Playwright1.62.1 exists and copied runtime is absent; mount changed to canonical runtime. Batch public-tools-preflight.sh passed shell/Node syntax, runtime, fixed images and exact clean source; preflight-harness.sha256 retained.",
    "permanentAction": "Keep shared runtime explicit when rebinding tool directories; preflight every actual harness/runtime binding before starting an expensive job.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-tools/preflight/ssh-node-e-argument-boundary",
    "position": "before-production-write",
    "count": 1,
    "impact": "Inline node -e over PowerShell/SSH lost remote argument quoting and failed with a shell syntax error; no runtime state was obtained.",
    "recoveryEvidence": "Saved the exact inventory as an external .mjs, copied it, then invoked its path; real results retained in runtime-preflight.json.",
    "permanentAction": "Use saved scripts and literal parameter arrays for cross-shell execution, following project-management6.2.2.",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-validation/expectation/release-upgrade-display-limit",
    "position": "before-production-write",
    "count": 1,
    "impact": "Public real-feature r1 initial42 checks passed; restart failed at release-info assertion requiring4 upgrade notes. PROJECT_RULES6.1 and actual source deliberately cap API/UI display at3, while public body keeps4. The test expectation was wrong; no product behavior change required.",
    "recoveryEvidence": "Failed r1 report/log/script retained. Source maxUpgradeNotes=3 and publication parser test inspected; both public harnesses now expect3 and retain actual payload; dialog additionally asserts3 visible upgrade items. Preflight-r2 passed; exact-image real-feature r2 and subsequent public UI results must pass.",
    "permanentAction": "Derive visible count from existing8-summary/3-upgrade/280-rune contract; distinguish raw body items from displayed API/UI items. Preserve full public notes and original failure.",
    "historicalReleases": []
  },
  {
    "fingerprint": "inspection/release-api/assumed-source-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial search named two non-existent guessed source paths; no result derived from missing output.",
    "recoveryEvidence": "Located route with rg over actual internal/panel and internal/selfupdate directories; inspected source.go and release_notes_test.go.",
    "permanentAction": "Search existing directories for route/type first and read returned paths; do not guess source filenames.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence-transfer/github-log/http2-peer-stream-cancel",
    "position": "before-production-write",
    "count": 1,
    "impact": "First full Release log read failed with stream ID1 CANCEL received from peer; its zero/partial output was not used as successful evidence.",
    "recoveryEvidence": "Fetched exact run38909301855 log archive via gh api with per-process HTTP2 disabled. ZIP1,378,504 bytes SHA2561813494fd755cf76c82e08136d14e9309880171ee1079f78468e0e27658ac93e; all33 entries CRC verified, uncompressed9,325,465 bytes. Original transfer failure retained; qualified workflow success remains independently recorded.",
    "permanentAction": "Use bounded original archive transfer for large logs, validate ZIP/CRC and digest, and retain failed attempts. Peer cancellation reason beyond transport error remains unconfirmed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "acceptance/report-release-metrics/missing-canonical-incidents-json",
    "position": "before-production-write",
    "count": 1,
    "impact": "First local docs verify-change rejected missing machine-readable incident JSON despite metrics/table being present. Early external fingerprints also omitted the canonical entry segment.",
    "recoveryEvidence": "Read actual report-release-metrics constants/parser and template; include canonical JSON block and normalize only missing entry segments with explicit original-to-canonical mapping. Original docs-verify-change.log retained. Corrected parser and full docs verify-change must pass before commit.",
    "permanentAction": "Validate the actual canonical acceptance schema before candidate push; preserve original incident records and explicit fingerprint normalization mapping rather than changing counts or hiding failures.",
    "historicalReleases": []
  },
  {
    "fingerprint": "acceptance/report-release-metrics/preview-process-versus-change-failure",
    "position": "before-production-write",
    "count": 1,
    "impact": "Second schema validation rejected process incident recovery text inside the production change-failure recovery field, while this preview has no production failure.",
    "recoveryEvidence": "Read actual null/failed-state semantics in validateAcceptanceMetrics. Set production recovery field to not-applicable; preserve all known process timestamps outside that metrics block and in canonical incident records. Corrected parser and docs verification required before commit.",
    "permanentAction": "Keep preview process incidents separate from production change-failure metrics, following existing template; do not fabricate a failed production state to satisfy the parser.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

流程异常全部位于生产写前；不把内部主动撤回、夹具/环境失败、被拦截的证据与线上故障混同。耗时仅引用真实阶段时间，未测净节约不外推收益。
