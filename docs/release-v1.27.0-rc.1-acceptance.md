# KPanel v1.27.0-rc.1 发布验收记录

日期：2026-10-16。发布级别：L3；releaseChannel=preview，releaseTrain=1.27.0。生产未部署。

冻结产品提交：`3df93782f554f139a53d74e3cc02d163aaa05aaa`；集成基线：`a1bbff0b5bdeb115890c2628255782944f55f15b`。上一稳定版及默认更新来源仍为 `v1.26.0` / `247ff67d98700c9af56d55a807cb9389fc8eedc1`，稳定镜像 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。上一预览 RC20 镜像 `sha256:ee0c159cedf525d3da3182b1ec2960f1b70408c648bb1bc411a06c85199938bb` 保留为预览回滚输入；降级前备份数据，不能保证新元数据向后兼容。

## 发布范围与画像

用户明确选择“全部成熟候选一并审查发布”，随后确认“按完整范围继续预览发布”。范围包含稳定版之后已经集成、尚未公开的终端保留及兼容修复，以及三个已交付界面修复。候选审查逐项核对差异和来源，不以分支 ahead 数判断待发布内容。

- 终端正常最小化保持运行，刷新恢复原会话，关闭终止；快捷命令恢复不重复提交。旧节点不支持保留时明确拒绝并提供临时入口，流恢复超时后在原会话走 v2。迟到启动意图绑定用户、host、shortcut 并有期限和数量限制。来源最终 `fa11134010437a8e661437ed01920924bd89e7f1`，之前包含 `b0be321389dbde8260bca8749b44eded3a9e4b95`、`aa8d96f15770c4aaeacaa91505f51e8fb8797c09`；相关产品源码与最终候选逐字节一致。
- 概览 History 图标置于文字左侧：来源 `a148459346eacf0589f99701b8d30ffabcddbec3` → 重放 `2dbfe854`。
- 安装启动失败后回取最新 automatic-update 状态，回取失败清除旧状态并保留原错误，避免立即切通道使用旧 resourceVersion：来源 `68903b42dd619abdbf281ce89e8198361f550aa1` → `873a62be`。
- ≤680px 弹窗与 toast 同时出现时预留顶部空间：来源 `8de5c1f8e6845430ecf610d22e5a7573e7d45f44` → `34b26b9d`。
- 发布元数据 `468998008d82734f886aa505bf9a17ec3ad959ed`；历史豁免 `0cfdabdaada31b0ec735682b2d7b7e1f44baad21`；完整范围说明 `3df93782`。三个重放 patch-ID 对照已核验。

风险为 L3：终端涉及进程生命周期、跨节点协议与恢复；界面涉及更新失败后状态并发控制。未新增 Agent 一般系统写授权、端口或 Compose/数据库迁移。内置脚本与应用市场契约不变。

未纳入：`claude/apps-local-source` / `4fd9ee3895cb40642221cd3919e664746dc95834` 依赖新脚本协议 `KPANEL_LOCAL_APPS_PROTOCOL_VERSION`、`local:<name>`，脚本候选 `160924a` 尚未发布，必须成对验收；`claude/panel-access-https` / `0e2478d3a9694d9f8465d3925784ce9b1f5e4b0a` 尚缺完整交付/所有权释放和 exact-tip HTTPS 启停、重启、域名切换与恢复隔离证据；历史主题候选没有本轮完整交付。不声明这些分支有已确认漏洞，也不擅自发布配套脚本。保留原分支和工作树。

## 跨仓库联动

`scriptLinkageState=not-required`，变更集及脚本候选不适用。实际内置 `kejilion.sh` commit=`1800d955f216aebd2776674368a8e469a671c489`，SHA-256=`5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c`；本次不修改脚本和应用市场默认更新入口。

## 审查、安全与依赖边界

- 覆盖检查：scoped-required，113 个未审计提交；用户限期豁免至 2026-10-23，仅适用于本次完整范围预览，CF 未完成。

发布任务独立于原作者审阅组合差异，沿用现有 API/认证/Origin/CSRF 与失败关闭契约；本机 Claude CLI 不可用，同提供方独立会话 fallback 留存于发布提交 trailer。来源 OCR：安装失败修复 1.12.11 覆盖 2/2 文件；两个不足 30 行的 L1 界面修复按规则 skipped；终端最终来源 OCR 21/21 文件。发布元数据 OCR skipped，不将 skipped 计有效；不从不完整历史推算整个稳定周期总量。

CF 覆盖检查绑定最终 `3df93782`，decision=`scoped-required`，113 个未审计提交、239 个机器边界文件，新增包 bittorrent/offers，fullAgeDays=25。本轮没有启动新的 CF scoped/full。历史 run-40 仍 incomplete，15 条 needs_validation、4 条 rejected、没有 confirmed；结构校验通过不等于审计通过。用户原话“授权限期豁免，继续预览发布”，随后授权更正至完整终端及三修复范围；截止 2026-10-23，责任为发布维护者。历史追加在公开之后，不倒写发布前授权、不关闭线索、不授权新稳定版或生产。稳定前须按 PROJECT_RULES.md 5.4 补审。

无依赖/锁文件语义、工具链、Action SHA、基础镜像或例外日期升级。依赖新鲜度完整检测 10/10 来源，两个既有例外已到期：TypeScript 7、`@types/node` 26，检查实际 exit 3；不宣称全部工作流成功。TypeScript 7 原 recheck 存在 `ERR_PACKAGE_PATH_NOT_EXPORTED`；Node LTS 24.22、vue-tsc 3.3.13 等检测候选本版不采用。维护者须重新决策到期例外，不能把旧例外写为仍有效。EOL 记录有效至 2026-10-28。

首方清单 935 文件，SHA-256=`d2f122ecb06fe0911962fca2635aad7ff33bc2165ef025938d0c9aa5603e38b5`；生产闭包 110 模块。canonical 源码/依赖/秘密/配置扫描及政策匹配通过，selected/standalone 既有发现仍按限期清单保留至 2026-10-25，不称全部依赖图零漏洞。

## 自动门禁

唯一 L3 入口 `scripts/run-release-l3.mjs`，arena-154，Runner=`kpanel-go127-security-runner:go1.27.2-node24.21.0` / `sha256:4edff59fa4c1441e3e97594593b255cc57a74e51dabbdd8db295e4ff51d0ca4f`。

| 运行 | UTC 时间 | 真实结果 |
| --- | --- | --- |
| v1.27.0-rc.1-4689980-l3-r1 | 06:01:05–06:01:14 | 历史 CF pending 无限期豁免，发布指标门禁拒绝；产品测试未执行。 |
| v1.27.0-rc.1-0cfdabd-l3-r2 | 06:05:33–06:29:41 | 完整通过，但随后完整范围文档更正使冻结 SHA 改变，保留为旧候选证据。 |
| v1.27.0-rc.1-3df9378-l3-r3 | 06:39:50–07:03:06 | 最终精确 SHA 完整通过，exit 0；12 份证据摘要回读一致。 |

r3 Web 297 文件、2827 通过/6 跳过，3719 词条/25 catalogs，类型与构建通过；Go/race、官方 torrent race、vet、部署门禁、双架构构建、镜像运行契约和安装/更新/恢复生命周期通过。没有删除失败、降低测试或将旧 SHA 结果冒充最终通过。原始日志 SHA-256=`e90368c02ff734dea29820b724a7d745fcd27596c8df4d5403b5067e2346df76`；bundle=`bdae78e17b2338582cfe187b0ce09ffb4fb6609a6a83c6238405a736b051f5bc`，plan=`726338a09bf2682e622f5c5010b63a78aa20c1f226a841c8ea32daf8ba83ab48`，remote script=`7b7013fe0eadbb71ae765a126c869e59f510fc540e359e182eb42af5bea4a4b4`。

候选 CI：38864929881 success，精确产品 SHA；依赖新鲜度 38864930060 为上述两例外 due 的 failure，单独保留。主线 CI、Release 和公开产物回执在最终收尾追加。

## 界面与隔离验收

最终 Mock acceptance 预览绑定 clean `3df93782`，`http://127.0.0.1:4175`，manifest=`preview-complete/manifest.json`，visual-composition；它只证明界面，不代表宿主或真实节点。三个修复与来源测试字节对照一致。

- 通知/弹窗 30 组合 Chromium/WebKit：320px 等视口、明暗主题及 100%/125%/200% 模拟缩放，最小字号 14px；截图实际检查。原生系统缩放、实体手机与读屏未验证。
- 安装失败 Mock r2：503 后 GET 一次、安装一次；成功刷新后使用新 resourceVersion 切通道 PUT 200；GET 失败清旧状态、隐藏不可用操作，PUT 0、不盲目重试。首次夹具误期望失败后仍显示 toggle，保留失败后纠正夹具，不修改产品掩盖失败。
- 概览图标 first SVG 与键盘 Enter 转 monitoring 通过。
- 终端来源 Mock 验证最小化/前台刷新恢复同 ID、快捷命令一次、关闭移除；真实混合版本节点、长时负载仍未验证。

公开镜像隔离实测另在发布后补录，不将预先准备的脚本当结果。

## 生产与回滚

多维结论按证据分开：

| 维度 | 状态 | 依据与限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 来源回归、最终 L3；真实混合版本远端节点仍未验证。 |
| 网络入侵与供应链安全 | 已实现未实机验证 | 既有控制和自动扫描通过；CF incomplete 与未决线索保持，不能声明全面安全通过。 |
| 稳定性、失败恢复与兼容 | 已验证 | 有界生命周期、失败态和 L3；不证明本版 systemd 升级、内核重启与长时 WAN。 |
| 性能与资源预算 | 已实现未实机验证 | 会话上限 16、每用户 4、1MiB buffer、8h、5min intent；没有长时 benchmark/soak。 |
| 用户体验与可访问性 | 已验证 | 定向布局、键盘、主题与模拟缩放；原生缩放、实体手机、读屏未验证。 |
| 数据、配置与迁移 | 已验证 | 无数据库/安装协议迁移，自动门禁与失败关闭通过；不扩大为所有旧节点/旧数据实机证明。 |

不适用（预览版禁止生产部署）。本轮未连接、备份、部署、升级或核对 prod-108，也未对 arena 执行生产部署。arena-154 仅用于登记的隔离候选/浏览器验证。

回滚输入为上述 RC20 或稳定 1.26.0 不可变镜像；先备份实例数据，再按现有生命周期恢复原镜像及对应数据，核对 Panel/Agent/版本与健康。未在本轮执行真实 systemd 更新/实例重启/故障回滚；L3 生命周期和旧稳定版 WSL 证据不代替本版真实宿主证明。无生产实际版本/健康声明。

## 证据与资源

本地证据根 `C:/GitHub/_release-evidence/v1.27.0-rc.1`；L3 原件位于 arena-154 `/root/kpanel-release-evidence/v1.27.0-rc.1-3df9378-l3-r3`，候选 kit/bundle 和首轮失败保留。发布候选 `release/v1.27.0-candidate` 在 RC 期间保留；未知所有权工作树、共享 Docker 数据和来源预览不清理。最终来源分支处置、公开证据与本轮资源收尾另在下节追加。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-15T19:09:24+08:00
- 候选冻结时间：2026-10-16T14:38:06+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：未记录
<!-- kpanel-release-metrics:end -->

首个时间取本版最早产品提交 `8e7ffb77` 的提交时间；本轮三个修复的首次重放是 2026-10-16T13:55:53+08:00，不以它覆盖继承主线的更早终端提交。各准备异常和首次失败均保留，但未取得每个事件精确发现/恢复时刻，以上不捏造时间。


<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：14
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {"fingerprint":"preparation/workflow-cli/python-shim-unavailable","position":"before-production-write","count":1,"impact":"工作流清单首次调用失败。","recoveryEvidence":"bundled Python 执行 release-kpanel 后的 release-workflow.txt","permanentAction":"本轮固定 bundled Python；维护者 2026-10-23 复核机器 PATH，恢复入口可用后退出例外。","historicalReleases":[]},
  {"fingerprint":"preview/local-feature-preview/invalid-journey-arguments","position":"before-production-write","count":2,"impact":"两次预览启动参数不符合 journey JSON/分隔要求。","recoveryEvidence":"preview-complete/manifest.json 最终 ready 且绑定 3df93782","permanentAction":"本轮改用保存的完整 argv；维护者 2026-10-23 复核启动参数，不降低入口校验。","historicalReleases":[]},
  {"fingerprint":"preparation/run-repo-bash/invalid-syntax-check-invocation","position":"before-production-write","count":1,"impact":"把 -n 当作脚本路径，语法检查首次无效。","recoveryEvidence":"resolveBash 后用 bash -n 的成功回执","permanentAction":"本轮按工具接口分离脚本执行和 bash 语法检查；维护者 2026-10-23 复核 saved argv 后结束。","historicalReleases":[]},
  {"fingerprint":"preparation/external-tool-generation/powershell-interpolation","position":"before-production-write","count":1,"impact":"内联 Node 生成器被 PowerShell 的变量语法拒绝，无产品修改。","recoveryEvidence":"prepare-public-complete.mjs 及保存脚本与远端 SHA 对照","permanentAction":"本轮使用保存的 JS 文件，跨 shell 不内联复杂文本；维护者后续复用此固定入口。","historicalReleases":[]},
  {"fingerprint":"l3/run-release-l3/history-audit-gap","position":"before-production-write","count":1,"impact":"r1 在历史 CF pending 门禁被拒绝，产品测试未执行。","recoveryEvidence":"l3-r1 原始日志、用户限期豁免及最终 r3 passed","permanentAction":"维护者 2026-10-23 或下一稳定版前补审；incomplete 不计通过，豁免不续期。","historicalReleases":[]},
  {"fingerprint":"scope/release-selection/unpublished-main-omitted","position":"before-production-write","count":1,"impact":"初始范围说明漏列未公开终端功能；已通过 r2 被新冻结 SHA 取代。","recoveryEvidence":"用户完整范围确认、3df93782、candidate-selection-complete.json、r3 L3","permanentAction":"本轮以稳定标签到候选的差异核对完整范围；维护者下一轮在冻结前复核标签差集。","historicalReleases":[]},
  {"fingerprint":"browser/background-browser-test/stale-toggle-expectation","position":"before-production-write","count":1,"impact":"首次 Mock 夹具误期望 GET 失败清状态后仍显示 toggle，作业失败。","recoveryEvidence":"ui-failure-job 首次失败与 ui-failure-job-r2 passed；产品源码未改","permanentAction":"本轮夹具验证失败清状态/PUT 0，saved r2 脚本保留；维护者下一轮复用对应契约。","historicalReleases":[]},
  {"fingerprint":"distribution/registry-read/local-dns-resolution","position":"before-production-write","count":1,"impact":"本机 Registry 只读预检连接失败，未据此声明镜像不存在。","recoveryEvidence":"arena-154 的 image-preflight.json 和后续远端原始 Registry 回执","permanentAction":"本轮固定登记隔离环境读取 Registry；维护者 2026-10-23 复核本机 DNS 后退出绕行。","historicalReleases":[]},
  {"fingerprint":"browser/runtime-preflight/reused-directory-missing","position":"before-production-write","count":1,"impact":"旧 Playwright runtime 路径已被前次收尾删除，公开验收尚未启动。","recoveryEvidence":"prepare-browser-runtime.sh、browser-runtime-package-lock.json、browser-runtime-version.txt；新版 shell f580e29b","permanentAction":"本轮使用私有固定 1.62.1 依赖和 lock；维护者下轮预检具体 runtime 文件，禁止只依赖旧目录自述。","historicalReleases":[]},
  {"fingerprint":"preparation/docker-inspect/unquoted-go-template","position":"before-production-write","count":2,"impact":"两次 image inspect 参数被 PowerShell scriptblock 转换，预检未取得身份。","recoveryEvidence":"随后使用整体单引号 --format 参数，两张 pin 镜像 ID 精确匹配","permanentAction":"本轮保存跨 Shell 固定 argv，Go template 参数完整引用；维护者下轮复用经过预检的命令。","historicalReleases":[]},
  {"fingerprint":"docs/verify-governance/local-go-unavailable","position":"before-production-write","count":1,"impact":"本地文档门禁扩展到 Go 发布说明测试，三个用例因 go 缺失失败；不计完整通过。","recoveryEvidence":"docs-verify-change.log；同一文档提交的 Linux 候选 CI 完整门禁","permanentAction":"本轮以同 SHA Linux CI 资格化；维护者后续在运行完整治理门禁前预检 Go，不降低测试或伪造本地通过。","historicalReleases":[]},
  {"fingerprint":"docs/verify-governance/vendor-checkout-line-endings","position":"before-production-write","count":1,"impact":"Windows checkout 的 CRLF 使 vendor 字节摘要测试失败；没有 vendor 产品修改。","recoveryEvidence":"docs-checkout-readback.json 证实归一化磁盘字节与 Git blob 的 573eeadf 摘要相同；Linux 候选 CI","permanentAction":"本轮使用规范 Linux checkout 完整门禁；维护者下一轮预检原始字节，不更改 vendor manifest/规则或已发布源码来掩盖失败。","historicalReleases":[]}
]

<!-- kpanel-release-process-incidents:end -->
## 公开产物与实测收尾

[v1.27.0-rc.1](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.1) 于 2026-10-16T07:41:34Z 公开，draft=false、prerelease=true。注释标签对象 `17d4a9c4c01f4370e504305b234221e3efeb78c8` 剥离为产品 `3df93782`。候选 CI [38864929881](https://github.com/kejilion/KPanel/actions/runs/38864929881)、主线 CI [38865650273](https://github.com/kejilion/KPanel/actions/runs/38865650273)、Release [38867052002](https://github.com/kejilion/KPanel/actions/runs/38867052002) 全部 success，均对应精确产品提交。

- `1.27.0-rc.1` / `preview` OCI index：`sha256:92ac0d50c95c427d460e9a9ff027c6d8029aa3af3f81725047be147684b278f9`。
- linux/amd64：`sha256:c354d8358b1e2f35ac662747995acc779b7b6ccf2eb2b5b033969f6a25a075af`；linux/arm64：`sha256:b9067ef2860a2e65c6b94402ca85f2db88271454b5a8a9027b2783a5455be1aa`。逐架构 config 中 version、revision 和内置脚本 pin 均与冻结身份一致；不将双架构构建称为原生 ARM 实机运行。
- 发布正文及真实 `/settings/kpanel-release?channel=preview` 返回 5 条非空更新、3 条升级注意事项、同一官方 URL/版本/摘要，弹窗实际显示一致。15 个附件逐项下载、大小/API digest/磁盘回读 SHA-256 全通过，`SHA256SUMS` 11 项通过。SBOM/provenance 与修改组件清单由成功 Release 保留，不宣称独立完成全部来源语义审计。
- 公开镜像 E2E：07:42:35–07:42:53Z，精确新摘要运行原版 `packaging/tests/image-e2e.sh`，`image_e2e=pass`、exit 0；实际健康版本、未特权静态资源、bootstrap、cookie/网络契约通过，专有容器和网络清理后容器名单与原先一致。
- 当日安全通告审计 schedule [38867580590](https://github.com/kejilion/KPanel/actions/runs/38867580590)，同产品 SHA，security-advisories job success；该次 report job skipped，不是对两项到期维护例外的关闭。push/tag 依赖新鲜度 due 的原始失败保留。

公开后的两个浏览器作业均通过权威 `scripts/background-browser-test.mjs` 执行，environment=arena-154、candidateRef=`3df93782`、timeout=900s（远端 timeout 840s）、exit 0。Playwright 1.62.1，镜像 pin=`mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e`；本轮私有 runtime lock SHA-256=`dd12e4a7795821a5d1298d57c630f3828e5076eca77702aacc07f19d5595097f`。固定 shell=`f580e29b1ddcb9e05def06a660fc57cb344c0cc10a617d1fdb936d1d4cffc32b`，journey=`b3575ee52c4e1a6aca61843e211ec663f30196d3c91b04bc7ca5d717d163ccff`。

| 作业 | 终态窗口 UTC / 命令 spec SHA-256 | 结果及边界 |
| --- | --- | --- |
| public-published-job-r1 / arena-154-26348 | 07:43:11.229–07:43:36.380；`74f1808d34fa7a0c4e110e4e7ea70a0e1bd081f0d26011ac7960501139999052` | 37 项检查通过。新公开 Panel、从同镜像提取的 Agent 二进制在固定 bash Runner 内提供真实 PTY，验证分段 printf 的执行输出、刷新后相同保留会话、单一页签、resize、通过 UI 关闭确认、会话列表移除、后续输入 404；新版本无同版本重装候选，加入/拒绝/退出预览均不自动安装或降级。 |
| public-upgrade-job-r1 / arena-154-29500 | 07:43:50.047–07:44:18.406；`40faa7ffb1414535a62c9924020414cb1f6b5674b9ce2f484c490dadcca7803c` | 30 项检查通过。RC20 旧 Panel/Agent 搭配新公开 Web，在实际更新 API 发现新 RC/摘要/说明，1280/390/320px 无横向溢出，实际无 systemd 的安装请求 503 `self_update_start_failed` 后清队列、自动安装关闭；新前端无需刷新切 stable 的 PUT 200，无第二次安装。此组合用于复现升级候选和失败态，不冒充新后端完整部署或成功 systemd 升级。 |

两组无 Mock API、没有宿主 filesystem/Docker/systemd socket；只读容器、cap-drop、资源上限、回环监听、私有数据和 watchdog 均保留回执。7 张截图已实际检查，手机弹窗标题与 toast 分离；pageerror=0，不扩展为所有 console/network 零错误。36 份报告/截图/运行回执逐项 SHA-256 回读一致。源码 Mock 的 GET 失败路径和新旧节点回归与上述真实 503 路径分别记录，不互相替代。

未验证：真实混合版本远端节点、实体 VPS/手机、原生 ARM、原生缩放/读屏、长时 WAN/负载、完整性能准入、真实新版本 systemd 更新/宿主重启/故障回滚、OpenRC/轻量 Node 安装路径及 Bilibili 成功路径。现有有界自动门禁与隔离旅程支持主动选择的 RC 公开，不能据此直接提升稳定版。

GitHub Latest 仍 `v1.26.0`，Docker `latest` 仍 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`；稳定通道从正式 Latest 选取，应用市场默认入口和脚本不变。未部署生产。

## 分支处置与资源回收

`release/v1.27.0-candidate` 远端继续保留 `3df93782`，这是 1.27.0 序列唯一候选，单次 RC 不归档。下列来源全部已经纳入本版，不能因活跃引用仍在而重复作为待发布内容：

| 来源分支 | 精确 tip | 纳入与处置 |
| --- | --- | --- |
| fix/overview-action-icon-order | `a148459346eacf0589f99701b8d30ffabcddbec3` | 三修复的重放映射见前文；远端活跃引用不存在，clean 本地工作树保留，upstream=origin/main 原样保留。 |
| fix/ui-resource-version-after-install-failure | `68903b42dd619abdbf281ce89e8198361f550aa1` | 远端活跃引用不存在，clean 本地工作树保留，无 upstream。 |
| fix/mobile-toast-overlap | `8de5c1f8e6845430ecf610d22e5a7573e7d45f44` | 远端活跃引用不存在，clean 本地工作树及来源预览保留，无 upstream。 |
| fix/terminal-background-integration-20261015 | `b0be321389dbde8260bca8749b44eded3a9e4b95` | 标签祖先，远端 tip 核对一致；本地 clean、upstream 指向同名远端。 |
| integration/terminal-background-acceptance-20261015 | `b0be321389dbde8260bca8749b44eded3a9e4b95` | 标签祖先，远端 tip 核对一致。 |
| fix/terminal-close-refresh-semantics | `aa8d96f15770c4aaeacaa91505f51e8fb8797c09` | 标签祖先，远端 tip 核对一致；本地 clean、同名 upstream 保留。 |
| fix/terminal-retention-compatibility | `fa11134010437a8e661437ed01920924bd89e7f1` | 标签祖先，远端 tip 核对一致；本地 clean、同名 upstream 和已交付预览保留。 |

来源任务已完成交付，但原件未明确授权回收作者预览、工作树或释放全部活跃分支归档所有权；本轮不据 clean/祖先关系推定该授权，尚未删除上述引用。它们分类为“已纳入预览、保留待所有权释放”，不是下轮新功能。待收尾负责人为发布维护者；下次稳定发车或收到明确释放时，重读精确 tip 和无新增提交，再按第 10.2 节以 expected-SHA lease 原子保存到 `archive/<原分支>` 并移除活跃引用。归档冲突/前移时保留，恢复从精确原 tip 或发布 tag 重放到新分支；本地 upstream/工作树另核对，不做全仓清理。

本页在产品标签冻结后由独立 `docs/release-v1.27.0-rc.1-acceptance` 收尾，先自身候选 CI、再快进主线、再主线 CI；该自有文档分支按同 SHA 成功后归档，最终引用回执见外部 `docs-closeout.json`，不为补录重发产品或改指标签。

文档本机 `verify-change` 的元数据、diff、业务基线、验收指标与覆盖格式检查通过；扩展治理套件实测 308 项，304 通过/4 失败，不记整体通过。其中三个因缺失 Go，另一个为 Windows vendor 检出 CRLF 的原始字节摘要不同：`third_party/torrent/.deepsource.toml` 的 Git blob SHA-256=`573eeadf8fec68e3f33a1ae4f1b2b32fadb4a40ad05a689c478b66cd1d89c376`，磁盘归一化 LF 后完全一致且无该路径修改。原日志和 `docs-checkout-readback.json` 保留；同一文档提交必须由 Linux 候选 CI 补齐完整门禁，通过后才快进主线，未删测试或修改产品摘要清单。

2026-10-16T07:46:36Z 完成本轮专有远端资源回收：先核对三个本地 L3 bundle SHA 和 `git bundle verify` 完整历史可恢复，所有候选源码/报告已保存；远端逐项解析绝对路径、检查无容器挂载，再移除三个已终态 L3 的 work/inbox 共六个目录及两个浏览器私有 data 目录。没有 Docker prune 或删除证据。磁盘可用量观察从 19,369,111,552 增至 20,909,121,536 字节，窗口净增加 1,540,009,984 字节，不声称是 Windows C 盘释放量或无其他进程影响。原始成功/失败日志、报告、截图、远端 evidence、固定 runtime、共享镜像、本地三份 kit/bundle、当前候选与 ready 的 4175 预览均保留。回执 `remote-cleanup-r1/`；没有回收其他任务或用户数据。
