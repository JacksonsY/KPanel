# KPanel v1.28.0-rc.2 预览版发布验收记录

日期：2026-10-23。发布级别：L3。`releaseChannel=preview`，`releaseTrain=1.28.0`。

**产物已发布，生产未部署。** [v1.28.0-rc.2](https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.2) ID `421531436`，2026-10-23T06:55:05Z公开，draft=false、prerelease=true，共15附件。Docker `preview` 指向本版；GitHub Latest / Docker `latest` 保持 v1.27.2。

[Publish preview39909530213](https://github.com/kejilion/KPanel/actions/runs/39909530213) attempt1的L3、候选CI、main/tag、主线CI和产物发布成功，首次公开验收因验收环境工具pin不可用失败，原运行failure保留。[只读恢复39918362315](https://github.com/kejilion/KPanel/actions/runs/39918362315) attempt1绑定原发布快照和不可变产物成功完成公开/浏览器验收；没有重发版本、重建或覆盖产品镜像。

产品候选 `37427b4cef550e9be3a45a61c67f8f89d0bdc021`，tree `c79290baa0f19c34b192382c40df68f3a7bdac91`。用户明确授权一并发布配套脚本和 rc.2，包含 SSH 推送、main、tag、Release 和镜像，不含稳定版或生产部署。批准业务基线 `59c7c47cbcd5b37340b49b0792b49b5092156a43`，可信发布控制面 `88bedbe170e0418efc58e0a72646b22a6b7f90ec`。本文是发布后的事实记录，不改变产品标签或镜像。

## 发布范围与业务画像

风险 L3：既有文件宿主写入、终端进程回收、集群会话和轻量节点升级边界加固，另有教程展示修正。业务真源仍为宿主文件、原有 Panel/Agent 认证和协议；没有增加端口、数据库迁移、Agent 权限或应用市场协议。

| 来源候选 | 精确提交 | 纳入行为 |
| --- | --- | --- |
| `fix/security-resource-bounds-20261023` | `d55ef103692fde4794672f242ecf098ac04e8235` | 安全闭环候选的前置资源边界修复；包含于下一行 |
| `fix/security-boundary-closure-20261023` | `cc6807f3c5b1f5bd397c57909f7d16d292e68191` | Linux fileRoot 固定目录身份与 no-follow；按实际字节限额；解压隐式父目录/每个目标 root 对象预算、Office ZIP 解析前限额；终端 pending/active 准入、取消及失败启动回收；relay 配额、回放按 principal/protocol 隔离、撤销关闭 v2 和旧轮询；Node generation7 防降级；宿主污染测试隔离 |
| `fix/guides-fallback-after-fetch` | `53d140eba16ee0a843ed4422350b2f03bb28d8c1` | 初次等待平台列表，失败/空列表才用内置教程；刷新保留已有成功内容；6 个回归测试 |

安全来源相对批准基线 48 文件、24 提交；教程 5 文件。组装 `1977a577`，版本/说明/扫描身份 `8b25e87a`，可信发布修复合并 `42e9d3dd`，连续审查 trailer checkpoint `37427b4c`；最终总差异 60 文件。未纳入已撤回 `archive/fix/help-video-reliability@2038339b`，不自动纳入其他历史或未知分支，不夹带无关依赖升级。

## 独立审查、OCR 与安全处置

本轮是普通安全盲审与候选非作者复核，不是 CF scoped/full 审计；quality-audit 全仓补漏本次 not-run。安全来源复用精确原件和原非作者报告，不重开已交付任务。原独立结论 PASS WITH FOLLOW-UP 保留，组装 11 文件非作者复核 PASS、发布入口修复 2 文件非作者复核 PASS；其他模型提供商 CLI 不可用，记录 `fallback=provider-unavailable`，未冒充跨提供商审查。

`security-review-scope` 相对批准基线 recorded14 组；另对冻结产品相对上一稳定版 `v1.27.2` 执行，19组均recorded、没有missing/stale输出，原件 `security-scope-since-stable.log`。继承旧格式 trailer 有 `invalid_trailer=true` 提醒，不等于 missing 或全仓安全结论。`move-existing-20261023-H1` 已静态证实并在 `206f3e13` / `f388bde8` 等修复，有 Linux 安全临时根回归和非作者复核；不声称已完成动态特权利用或生产实演。归档/Office/终端回收/relay 回放等来源发现对应修复已包含于 cc6807f3，较早独立 FAIL 与修后通过原件均保留。

`legacy-export-revoke-20261023-M1` 仍为 needs-validation：已授权的旧单次二进制导出在途不与 revoke 同步取消；fresh requests 拒绝、Close 释放槽。负责人 KPanel cluster maintainers，2026-10-30 前复核；不是 confirmed HIGH，不写成已修复或所有传输立即取消。其他历史 MEDIUM backlog 不由本记录关闭。源码 / RC / 稳定 / 生产交付分层：已交付源码和本RC；尚未进入稳定版或生产。旧RC1两项HIGH修复也继续包含，既有公开原件不改写。

OCR1.12.11 自由臂先落盘，组装53/53文件 reviewed，7个排除项（4文档、2删除的平台实现、1锁文件）另行审查；精确逐路径差异相同才复用原件。当前自由臂 `blind=false`，`constrained-only=unreported`，不将历史独立 finding 冒充 OCR 新收益，不从一次 RC 推断三个稳定周期收益。仓库外原件根 `C:/GitHub/_release-evidence/kpanel-v128-rc2`，含 assembly/helper 的独立审查、free-form、rules 和 coverage JSON；来源原件位于 `C:/GitHub/_codex-evidence/kpanel-security-inventory-20261023` 等已绑定路径。

## 跨仓库联动与供应链身份

`scriptLinkageState=coupled`，changeSetId `necessary-security-20261023`。用户追加授权后，先通过 SSH 将 `kejilion/sh` main 从 `bbda9e9c8c4a48154b59d59de44e8c4042cd09dc` 快进到 [24f65f9f](https://github.com/kejilion/sh/commit/24f65f9fb8cbf779d03451d528f038b69627ab14)，再冻结 KPanel。共4文件，根脚本 SHA-256 `c34c2ca3a04018a0c6b22628be63f290944c0ef429b4d3a9f06a582bd3143364`，中英文归一后相同，双脚本语法、37项隔离 updater 测试和安装 smoke 有精确来源成功证据。内嵌 Node runtime source.json pin该提交，7模板逐一摘要及 canonical sync --check 通过。同版本修复和失败升级事务恢复保留；不主动发起版本回退。

Docker 受管整脚本基线仍 `160924a498d9b8d071fed72f165d23c1b9e886de` / `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`，不承担 Node binary 内嵌升级运行时；不扩大到无关基座升级。sh 管理树已有 firewall/未跟踪内容未改动；发布使用逐命令既有授权 SSH identity，没有读取/展示私钥或改变共享 Git 设置。

Go1.27.3 原 cc6807f3 的六平台 actual/original 闭包经独立复算1022文件后，确认仅 Go 版本 literal 改为 rc.2，路径/import/module/build 输入不变；更新 firstPartySource `78ae6f27d3347ffbbd3034b9eaa347c5fc46835ba6cc83a722da06901ca02a25`，closure `f7c89d5e862888205deb3af9331784042142e11eaec2d3e420b349f750e8ff27`，sourceManifest `3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580`，graph `24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927`，production110 modules。37项库存既有处置观察日2026-10-11、到期2026-10-25未延长。最终L3库存为 production110 / selected465 / standalone150，实际原始 exit0/1/1、告警0/10/25；保留原始输出，不将本次扫描数量变化解释为源代码已修复或全库存零漏洞。L3另逐项核对27份rawEvidence的字节数与SHA-256通过。

## 自动门禁与发布入口修复

- 安全来源 CI [39902948812](https://github.com/kejilion/KPanel/actions/runs/39902948812) 成功。来源早期 L2/测试夹具迭代失败保留，最终云端通过不改写历史失败。
- 精确冻结产品 push CI [39907760937](https://github.com/kejilion/KPanel/actions/runs/39907760937) attempt1/job120263515065成功，实际 `VERIFY_BASE_REF=85fe4b815e9a07f2c390ee49c2ba69f4f2a72fef`，完整 L2、治理354、Go/Web/构建、race、Go/npm/Trivy及脚本生命周期通过。Web314文件，3114通过/6既有skip，共3120。ready/handoff资格化不是L3。
- RC1首次公开验收的 inherited workflow_dispatch 入口误判，在 `e42dbf78` 改以实际 snapshot/run/attempt 区分内联与只读恢复，维持身份/权限失败关闭；新增2回归在旧代码2fail，修后治理354通过。连续审查 checkpoint `88bedbe1` 产品树相同；独立复核PASS。空diff的39906525443不作治理证据，另建精确同SHA资格ref的 [39907111959](https://github.com/kejilion/KPanel/actions/runs/39907111959) 实际base=origin/main、354通过，main [39907920353](https://github.com/kejilion/KPanel/actions/runs/39907920353) 同SHA实际354通过后归档。未篡改RC1原始失败。

权威L3为39909530213/1的job120269097354，2026-10-23T06:07:10.995Z至06:17:28.235Z passed。GitHub-hosted ubuntu24 / Runner image20261012.334.1，Go1.27.3、Node24.22.1、Docker28.0.4。源码web/go/deploy三lane均exit0，candidate/tree与冻结值一致，identityUnchanged=true；镜像 `image_e2e=pass`。`run-release-gate.sh` entry SHA-256 `43e57f4fd321846204020d6cf27eb75961f0aefa1284a9187e786601704c2d40`，发布计划SHA-256 `9714eb936a8750a8d242da356be2219b5f7732bf689d8dae2f10717383a3643a`；artifact12482945979 ZIP SHA-256 `328d77e241527e85df7d74ee01935f5f2ddfb68ad894caaf58c141257d14b733`，原receipt、result及三lane日志摘要逐一验证于 `l3-artifact-verification.json`。后续publish/l3按preview设计skipped，不冒充另一轮执行；完整L3证据是上述实际作业。

同一发布运行内 candidate-ci job120269097403、main-ci job120274440399均success，实际base88bedbe1（区别于push CI的85fe4b81）。promote job120274381374原子快进main并创建annotated tag，release job120279730327 success。草稿和最终发布说明均经真实selfupdate解析器门禁：5条更新、3条升级提示；最终公开正文SHA-256 `a77e7ab0fcc78ff60f3925160cca609f6490ac00f850b99edec0407b41b08613`与原冻结正文匹配。源码扫描、native镜像扫描/契约及SBOM/provenance步骤均通过；完整原始日志保留于仓库外证据根。

首次公开验收job120284867229的冻结源码、内联snapshot恢复、15附件下载/API digest/磁盘回读、11项SHA256SUMS、双架构与正文校验均通过，记录 `first-public-assets-verification.json`；RC1入口修复至此验证闭环。但固定Runner的Alpine包docker-cli29.8.2-r1/buildx0.34.1-r2已不可用，apk exit2，浏览器断言未启动。`public-acceptance-first-failed.log` SHA-256 `4ffb5b5f0190a846895c6b1529963c99cfae2af63465ab399954d4965bd95cde`及失败artifact12484063488保留，不能把首次job记为success。

仅验收工具修复 `2f9fdda2` / 连续审查checkpoint `0e14533742cfa7a96d69e61c2f2f3b0a12b5d6c6`，1文件2行pin改为docker-cli29.9.2-r0/buildx0.38.1-r0。官方v3.24 community x86_64 APKINDEX归档SHA-256 `93e9eacbbaeae9be4ae7b87e52414e15149e058c98005f392ba4df4b2b2e00a0`，非作者独立解析版本/依赖并复核4/4既有Runner契约测试，报告SHA-256 `7bef3b9f7c953e832f133504ce7f91b5a69bb513b93f9bdde5f56b3b95e5f89b`。两行L1工具版本变更未达30行OCR阈值，记录skipped；不改变产品安全包、基座digest、Go身份或浏览器断言。

精确0e145337候选[39915406563](https://github.com/kejilion/KPanel/actions/runs/39915406563)/job120287760902，实际base=origin/main37427b4c；SSH expected37427b4c lease快进main后，同SHA主线[39916818633](https://github.com/kejilion/KPanel/actions/runs/39916818633)/job120292062438，实际base37427b4c，均完成Linux功能、竞态、安全扫描和应用生命周期success，再dispatch只读恢复。没有因为查询接口返回滞后状态而提前推进；按精确attempt和新鲜回读确认终态，GitHub官方当时未报告未解决事故，不推断等待根因。

恢复39918362315/1 job120297286522 success，controller0e145337、产品candidate37427b4c。从原39909530213/1下载snapshot并校验身份，未手工重造发布前快照。controller-owned recipe overlay仅改验收Runner Dockerfile两项工具pin；原产品HEAD/VERSION及镜像不变，`productImageUnchanged=true`，原/新recipe逐字节与对应Git blob一致。新recipe SHA-256 `92d81f0410dd2a05b28b2b6554a0c865696263149306e6ef6d6a2923ff92b48f`，实际固定测试Runner ID `sha256:2c03bf93ee7b933bc2b6f3c28fef33f5722622aa2ef6f1b55b828c5c23252d85`；Runner实际构建和工具执行成功，不只依赖静态索引。

公开容器/真实浏览器四报告：初始44、重启26、published45、upgrade31，共146项，全部passed、errors=[]、mocked=false；最终更新信息为1.28.0-rc.2、5条摘要/3条升级提示、同一公开digest。包括真实Panel/Agent API、偏好/语言/桌面恢复、HTTPS与证书故障恢复、域名边界、PTY续接/关闭、通道选择与旧RC1发现当前RC2更新弹窗；1280/390/320宽度无横溢出。已人工核对390px真实更新弹窗，截图与JSON保留。

测试运行于GitHub-hosted Linux隔离容器；Docker库存与测试CA明确为fixture，不声称生产安装/systemd或真实混合节点互通。测试Runner可访问测试宿主Docker用于编排，被测Panel/Agent未挂载宿主Docker/systemd socket或根目录。三组cleanup回执确认只移除自有容器/网络。每组timeout15m、workflow50m；本轮为短时受影响旅程验收，无长时soak/低配RSS/跨发行版实机结论。恢复ZIP artifact12485762823 SHA-256 `699919468f5952d3b05e7280b6e126a0637ea8b3c141bf6353480bba4965f475`，实际ZIP大小/摘要、安全条目、四报告、recipe及冻结资产逐项验证于 `public-verification.json`。

## 依赖、技术栈与质量边界

精确候选 dependency-report [39907760933](https://github.com/kejilion/KPanel/actions/runs/39907760933) report job120263515243成功，9/9必需源、EOL current；该push安全通告job按配置skip。另 [39906149855](https://github.com/kejilion/KPanel/actions/runs/39906149855) 在基线59的每日安全源审计job120258458682实际成功，不冒充候选SHA扫描；EOL截止2026-10-28T23:59:59.999Z。typescript/@types/node例外reviewDate2026-10-23 active，仅当日有效，不延长。产品除配套Node runtime未采用依赖、镜像、Go/Node工具链、Actions或扫描器升级；独立的公开验收Runner在发布后将两项不可用工具pin更新为docker-cli29.9.2-r0/buildx0.38.1-r0，固定基座digest及其他包不变，详见公开验收恢复段。现有维护负责人在既定期限复核。

| 质量维度 | 状态 | 证据与边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 定向/全量测试、隔离容器公开旅程；未等同真实混合节点长期互通 |
| 网络与供应链安全 | 已验证 | 精确来源审查、CI/L3扫描、冻结附件/镜像与provenance；保留37库存处置及needs-validation，不作无漏洞保证 |
| 稳定与兼容恢复 | 已验证 | 准入、取消、回收、失败升级及公开容器重启/恢复断言；实体跨发行版升级回滚未实演 |
| 性能与资源预算 | 已验证 | 有界预算和错误路径回归；不宣称低配RSS改善、实测吞吐或长期soak结论 |
| 体验与可访问性 | 已验证 | 教程10项Mock交互、窄屏与键盘；公开真实浏览器另核验。读屏/实体手机/原生缩放矩阵未专项执行 |
| 数据与配置 | 不适用 | 无数据结构迁移；既有配置/通道持久化由公开容器断言覆盖 |

教程本地 acceptance/profile interaction 绑定最终37427b4c，`http://127.0.0.1:4174/settings/guides`：初次等待、502回退9卡、键盘重试、成功列表、刷新与空刷新保留、fresh空列表回退、390px无横溢出，共10项通过，pageErrors0/unexpectedConsoleErrors0（故意502另计1）。1280×900和390×844截图、manifest/ui-result保留。Driver1.62.1初次缺预期Chromium1234，失败原件保留；显式使用已有revision1228/Chromium149.0.7827.55验证Mock，不声称版本匹配或以此替代固定Runner公开验收。6项组件测试及typecheck/build通过。

## 公开身份、自更新与回滚

annotated tag对象 `3c7f2f506f753749bf7de46078ac81fc0fe41ce9`，peel/source为37427b4c；版本/preview OCI index `sha256:ef5f55851a6b42036d4a863767081671e0afcb099385ed4333c3c6b59d4412e2`。linux/amd64 `sha256:fa74c58678cbf59915bfe5c6b93f74f62771e26d5836640245f3af04bd7b98f3`，linux/arm64 `sha256:168b603a7cd62d231eaf5f91d3e0045a257f65cc62bb1132534d750fb05a5166`，架构镜像标签source/version/受管脚本pin一致。GitHub Latest ID420434596仍为v1.27.2，Docker latest摘要与发布前快照相同。

snapshot artifact12484425987 SHA-256 `388e17be99181acf7fdbc708025215bbefd0e1a7ce440b74175198587d7a19d8`，publication artifact12484176792 SHA-256 `3603a52bd9cbdb15b22bf832183f25d2b4afb7dd869b1cb6705bc202d54bad2f`；根snapshot与publication嵌套目录的三个JSON逐字节一致。公开ReleaseID/15附件名称、大小、digest及正文匹配冻结清单，恢复再实际下载与磁盘回读15项、核对11项SHA256SUMS。`KPANEL_MODIFIED_COMPONENTS.json` digest `sha256:75d15900248b84e3b6ba24ca1f381725baa4517fd0b98cfab625c438b92b425b`，双架构SBOM/provenance与官方镜像关联通过。详见 `publication-artifact-verification.json` 与公开回执，Panel真实发布信息接口和更新弹窗同样已验证。`kejilion/apps` / `kejilion.sh`生命周期为隔离rootfs夹具通过，不替代真实宿主安装。

稳定来源只读取正式Latest；预览只切来源并立即检查，不自动安装；自动安装开关与单次安装独立。退出preview且stable更低时不产生降级候选。真实更新弹窗/通道切换与持久化以公开浏览器报告为准。容器夹具不代替真实systemd/OpenRC宿主安装、宿主备份或实体回滚。轻量Node本轮generation门禁不改变其通道适用边界。

上一预览回滚点 v1.28.0-rc.1 / `sha256:3e933fe16dcdf49b51e7403d9c6ee25618be1c9582641ecad8f0d7e11f5442ae`，稳定 v1.27.2 / `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`。需要实际回滚时应先备份数据/配置再选择固定版本镜像并复核健康，退出预览不自动降级。本次未回滚，未新增生产备份，生产实际版本/健康不适用。

## 生产、分支与资源收尾

生产所有动作：不适用（预览版禁止生产部署）。本次未连接、未备份、未部署、未升级、未核对 prod-108；未连接其他生产目标。GitHub-hosted Linux隔离验收不构成arena-154或其他真实宿主准入。

本次三条已交付来源均核对精确提交为产品tag祖先、clean且所有权已移交后归档，远端逐项回读成功；本地分支改archive并解除旧upstream，原工作树保留。

| 原分支 | 归档ref及精确SHA | 处置 |
| --- | --- | --- |
| `fix/security-resource-bounds-20261023` | `archive/fix/security-resource-bounds-20261023` @ `d55ef103692fde4794672f242ecf098ac04e8235` | 原为本地未推候选，直接保存精确归档；已纳入rc.2 |
| `fix/security-boundary-closure-20261023` | `archive/fix/security-boundary-closure-20261023` @ `cc6807f3c5b1f5bd397c57909f7d16d292e68191` | 原远端活跃ref以expected-SHA lease移除；已纳入rc.2 |
| `fix/guides-fallback-after-fetch` | `archive/fix/guides-fallback-after-fetch` @ `53d140eba16ee0a843ed4422350b2f03bb28d8c1` | 原为本地未推候选，直接保存精确归档；已纳入rc.2 |

原始tip也可从v1.28.0-rc.2恢复。另验收Runner修复archive/fix/rc2-runner-packages精确0e145337，同SHA候选/main CI与恢复验收通过后expected-SHA原子保存/移除原活跃ref，远端回读成功，本地改archive、旧upstream解除，工作树clean保留。

既有 helper archive/fix/rc2-public-acceptance-mode、archive/fix/rc2-public-acceptance-qualified 均精确88bedbe1，已回读并以expected-SHA lease删除原活跃ref，本地改archive且工作树clean保留。RC列车release/v1.28.0-candidate保留37427b4c。验收文档单独候选，同SHA Linux候选CI、main快进/CI成功后归档到archive/docs/release-v128-rc2-acceptance，不重打产品版本。来源及证据恢复点保留；sh来源分支保留作为跨仓恢复证据，不将其当下一KPanel候选。

本任务4174预览已按精确manifest停止，2026-10-23T07:36:06.470Z mockApi/web均stopped；证据保留。管理树及未知内容未改动。没有执行磁盘文件删除，不推断净释放字节；当前候选、首轮失败、审查和回滚证据仍需保留。不新增知识库或工作流；修正的是已有公开验收工作流。

## 交付节奏与流程异常

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-23T08:43:00+08:00
- 候选冻结时间：2026-10-23T13:46:51+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：8
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

上述为本次接管已记录的发布流程异常，不是产品发布失败次数；没有产品载荷引起的生产退化、回滚、紧急热修复或重复发布。来源开发阶段有效旧代码red回归、断言迭代、可选探测与正常外部等待不机械计入。部分首次失败只有工具原回执，以下明确区别，后续成功不覆盖原始失败。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preparation/sh-remote-state/https-connect-timeout",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial authoritative sh origin query over its existing HTTPS remote failed to connect; no remote write occurred.",
    "recoveryEvidence": "Tool transcript excerpt: github.com port443 connect timeout after21107ms; subsequent SSH ls-remote verified bbda9e9c and published 24f65f9f; paired-script-publication.json. No separate full original log file.",
    "permanentAction": "Release maintainers: use the authorized SSH publication path and verify its repository identity before writes; recheck by2026-10-25. The dirty shared sh checkout and its remote configuration were not changed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "publication/sh-ssh/missing-repository-identity",
    "position": "before-production-write",
    "count": 1,
    "impact": "First SSH query from the sh source worktree was denied publickey because it lacked KPanel's per-repository GitHub SSH identity selection; no writes occurred.",
    "recoveryEvidence": "Tool transcript and paired-script-publication.json; explicitly reused the existing authorized GitHub identity via per-command SSH options, then verified the exact script main tip.",
    "permanentAction": "Release maintainers to verify an explicit repository-authorized identity before the next paired publication and review by2026-10-25; do not treat SSH availability in one repo as another repo's configuration or alter shared dirty checkout settings.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/task-preflight/dirty-checkpoint",
    "position": "before-production-write",
    "count": 1,
    "impact": "Ready qualification was invoked while five version/notes files were still being edited and rejected the dirty checkpoint.",
    "recoveryEvidence": "Tool transcript of failed ready check; after source qualification and commit8b25e87a, ready passed; frozen37427b4c ready also passed. No separate complete first-failure log file.",
    "permanentAction": "Release maintainer follows clean checkpoint before ready/handoff; mandatory existing preflight remains unchanged. Qualification is not CI/L3 success.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/task-preflight/missing-go-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "Same initial ready invocation also reported missing Go because the known fixed Go path had not been applied to this fresh shell.",
    "recoveryEvidence": "Tool transcript; explicit Go1.27.3 path and GOTOOLCHAIN=local yielded passed ready and release-notes check. Original failed observation is retained here rather than declared passed.",
    "permanentAction": "KPanel release maintainers: recurring local entry problem also seen in RC1 and v1.27.2; by2026-10-25 and before the next applicable production-write gate, qualify/fix the unique Go discovery entry and regress it. This turn's explicit PATH is a workaround, no permanent repository fix claimed.",
    "historicalReleases": [
      "v1.27.2"
    ]
  },
  {
    "fingerprint": "qualification/review-trailers/noncontiguous-block",
    "position": "before-production-write",
    "count": 1,
    "impact": "Separate commit-message paragraphs meant only the final Security-Review field was parsed; OCR advisory reported missing despite the raw reviews existing. First helper CI39906425483 was superseded/canceled by branch concurrency.",
    "recoveryEvidence": "Original8e6ab70d kept; contiguous trailer checkpoint88bedbe1 recognized OCR and independent review. helper-review-message.txt, helper-independent-review.json and helper-ocr-coverage.json preserved.",
    "permanentAction": "Use an exact commit-message file containing one contiguous trailer block and verify with git interpret-trailers before push; no rewriting published history. The inherited invalid-trailer advisory is not concealed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/governance-ci/empty-review-diff",
    "position": "before-production-write",
    "count": 1,
    "impact": "Metadata-only checkpoint CI39906525443 succeeded but its changed-file step did not execute governance; it was not used as proof of Linux governance.",
    "recoveryEvidence": "Same88bedbe1 was qualified on fresh fix/rc2-public-acceptance-qualified ref against origin/main59c7c47c. CI39907111959 raw helper-candidate-ci-job.log confirms354 tests; mainCI39907920353 explicitly accepted that qualifying run and also ran354 tests.",
    "permanentAction": "Release maintainer must inspect actual base/steps and bind required checks to the complete candidate delta after empty review checkpoints. Both exact-tip qualification refs archived only after same-SHA main CI success; no skipped test or successful empty-diff run is represented as executed governance.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/browser-preview/driver-browser-mismatch",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local preview driver1.62.1 expected Chromium revision1234, which was not installed, so browser launch failed before UI assertions.",
    "recoveryEvidence": "tutorial-browser-first-failed.log and final-preview/ui-failure.json preserved; explicit installed revision1228/Chromium149.0.7827.55 used for10 passed mock UI checks, with driver/browser versions recorded in ui-result.json.",
    "permanentAction": "Release maintainers review by2026-10-25: probe the selected executable and record driver/browser versions before QA. This local mock uses an explicitly declared available browser; canonical public acceptance separately requires the project's pinned runner. No claim that a matching1234 bundle was installed or that the local browser proves public acceptance.",
    "historicalReleases": []
  },
  {
    "permanentAction": "Two-line runner pin fix2f9fdda2, reviewed checkpoint0e145337, integrated only after independent PASS and exact candidateCI; same-SHA mainCI and actual Runner rebuild/public browser recovery passed. Source branch archived to archive/fix/rc2-runner-packages at0e145337 after verification. Release maintainers review direct package availability before next preview, by2026-10-25; fixed pins still depend on upstream retention, with no floating versions or skipped assertions.",
    "position": "before-production-write",
    "recoveryEvidence": "Original public-acceptance-first-failed.log retained (sha2564ffb5b5f0190a846895c6b1529963c99cfae2af63465ab399954d4965bd95cde). Official runner-package-qualification.json and independent review verified replacement pins. CandidateCI39915406563 and mainCI39916818633 at0e145337 passed; read-only recovery39918362315/1 from original39909530213/1 snapshot passed15 assets/11 checksums/146 real browser and container checks. public-verification.json binds artifact12485762823 SHA256699919468f5952d3b05e7280b6e126a0637ea8b3c141bf6353480bba4965f475. Product tag/image unchanged.",
    "impact": "Publication39909530213/1 exposed rc.2 and passed frozen source, inline snapshot and15 public asset/multiarch verification, but browser assertions did not start because Alpine stopped offering docker-cli29.8.2-r1 and docker-cli-buildx0.34.1-r2. Offered versions29.9.2-r0 and0.38.1-r0 were subsequently verified against official APKINDEX.",
    "count": 1,
    "historicalReleases": [],
    "fingerprint": "public-acceptance/release-runner/unavailable-apk-pins"
  }
]
<!-- kpanel-release-process-incidents:end -->

遗留准入由相应维护者按原期限处理：legacy export 2026-10-30；库存处置2026-10-25；Go发现入口重复问题在下一适用生产写门禁前修复。没有在本次RC夹带额外治理，未将一次现场绕行写成永久根治。
