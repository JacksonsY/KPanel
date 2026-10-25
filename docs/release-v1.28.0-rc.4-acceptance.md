# KPanel v1.28.0-rc.4 发布验收记录

日期：2026-10-25

发布级别：L3；`releaseChannel=preview`；`releaseTrain=1.28.0`。

候选 / 标签：`f673068989deb5da1e0d2e7ca03a35db8c03d89b` / `v1.28.0-rc.4`；tree `1b774646cc9c8229b1e7646c9a186ea33f04cc30`。

产物已发布，生产未部署。GitHub Latest / Docker latest 保持 v1.27.2，本版仅更新 prerelease / preview。

## 发布范围与画像

纳入安全交付7b38aa797bcc77d35372f5ae8d91e40b402fb088与网站流量监控04d06654dea28431d4be76e59efb4b3ce4fcf960。批准基线及可信controller为8b9a0d58c210530fb225a217e55b9741d13a9090；最终候选push实际before为a9b4be9adf3da484299236bd0334d2870e540dcf，两者分别记录。

- 三条更新：新增网站流量/WAF监控（趋势、世界地图、国家、路径、来源及筛选，默认关闭）；修复认证、恢复、文件暂存/权限、轻量节点接入及AI/MCP输出边界；修复undici开发依赖安全通告。
- 监控仅在Panel保存SQLite；Agent读取原生Nginx访问/审计/限流/错误日志与`.1`轮转文件，不回填gzip；每批16源、1MiB、2048记录，访问行16KiB/WAF行64KiB，普通10秒/积压1秒采集。
- 可选本地MaxMind MMDB不超过64MiB，无在线下载；数据库128MiB上限，聚合30天，近期事件7天/5000条，独立有界盐化去重。无原始IP、请求体、请求头、query或规则匹配载荷落库，不增加WAF写操作。
- 风险集中于认证撤销、文件边界、节点受保护凭证传递和新历史数据库；监控真实生产日志兼容、高吞吐与跨发行版长期运行未专项验证。
- 历史安全台账87条唯一记录中72修复、13按已验证既有契约关闭、2被替代；不是87个独立漏洞，也不构成全仓安全保证。新来源盲审H1/M4中4成立并修复，1按明确授权文件数据契约驳回。
- 未纳入稳定发布、生产部署、无关候选、上游脚本重复发布、工具链大版本采纳或门禁放宽。累计业务基线fbdcfacb8923655621d368900350940cc196f2c4；50/20+8阈值不变。

## 升级与兼容边界

正文、接口和更新弹窗实际核验3条更新、3条升级提示。以下限制属于公开预览已知边界：

1. 主动加入预览通道后方可更新，稳定默认仍1.27.2；监控只读取现有本机源站日志，地域需要自行提供兼容离线MMDB，不等同浏览器访客统计。暂停保留历史，清空不删除原始日志及WAF配置。
2. **默认复制命令不能直接用于全新轻量节点的预览版安装。** 新命令使用v2 FD3/hash本地引用，稳定Node显式拒绝；固定安装脚本仍默认GitHub Latest稳定1.27.2。须匹配同版本Node资产，不能把完整带凭证命令嵌入bash-c/ssh参数。候选Node与原安装脚本的同版本受控夹具已验；默认公开稳定下载后的全新接入未通过，不声称可用。负责人KPanel发布/轻量节点维护者，2026-10-28复核，稳定1.28宣称默认安装可用前必须用真实默认脚本完成全新接入和受保护凭证/恢复验收；详见仓外preview-compatibility-followups.json。
3. 升级前备份Panel/Agent，先完成或安全处置在途归档/恢复任务再回退RC3；新恢复身份、任务记录及监控数据库不能只换旧镜像自动回退。历史位于`<DataDir>/site-traffic/analytics.sqlite`，独立于配置备份，需停Panel并备份整个site-traffic及WAL/SHM。

## 跨仓库联动

`scriptLinkageState=not-required`，变更集不适用。本轮不发布上游脚本。保持已发布`kejilion/sh@614d58cee228007012a2ec602cbdc8dbaeb7950d`，SHA-256 `ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9`；Node防降级24f65f9f为祖先。Nginx资源pin8773850871f991b027f62841fde2795121251620不变。FD3引用由现有脚本透传，不要求新增shell协议；默认稳定下载的分发限制单独记录，不能视为脚本联动已验全新预览安装。

## 组装、复核与可信自动化

原冻结a9b4be9a在发布前静态检查发现6个来源可执行工具未进入可信main，未调度或发布。保留原候选及CI40157107713/job121060722142成功证据。独立工具候选8b9a0d58只9路径：6个交付脚本原字节、实际未变主线1022源码扫描身份、undici锁定补丁及既有观察文档；不包含产品Go/UI/版本/工作流变化。固定六项矩阵/六维独立复核原FAIL及后续PASS均保留，PASS报告SHA-256 adc6f7c3aae4c82abe6e1720c2000671092e38eb9faa98eed1e0b4267df26cb9。候选CI40158857317/job121065942127实际完成358项治理后SSH快进可信main，同SHA主线CI40159781103/job121068721853再完成358项治理后归档archive/fix/rc4-trusted-source-tools；原工作树保留clean。

随后按可信main重新冻结f6730689，产品Go/Web/依赖及可执行脚本与原a9完全相同，只4个文档/扫描身份元数据路径变化；全部受信自动化与controller相同，唯一允许差异scan-scope JSON按实际1125源码绑定。未为绿色状态修改扫描策略、测试、超时或例外。

OCR1.12.11先保存自由臂，再执行规则覆盖；最终区间8b9a0d58..6e0eb19a为251/251可评审路径、22排除项逐项核对。原区间257/257及23排除项亦保留。`blind=false`、`constrained-only=unreported`，不作工具有效性结论。安全审查26组recorded：仅internal/agent及internal/panel/server.go组装交叠由两个独立无历史快照会话新审，均无可信安全发现；其他范围只在源码blob完全一致时复用原独立原始报告。历史无效skip trailer保留为告警，非通过依据。CF审计not-run。

低功能跟进：诊断collection拒绝RawQuery，因此未使用的projection=model-v1 collection分支不可达；真实MCP按task-ID详情/输出路径，现有回归通过，无原始隐私输出回退。负责人诊断维护者，2026-10-30复核；不冒充新增安全发现或OCR增量。

## 自动门禁与供应链

最终push CI40160682535/job121071388784成功，原日志证明实际beforea9、明确L2、治理358项与Web317文件/3137通过/6跳过，以及Go、race、govuln、npm/source扫描、生命周期检查。不是仅凭状态图标或空元数据提交跳过。

发布[run40161392425/attempt1](https://github.com/kejilion/KPanel/actions/runs/40161392425)的候选CI/L3/主线CI/产物发布成功，整体failure来自首次公开验收旧传输夹具；原失败与preview-completion条件skip保留。可信发布controller8b9a0d58：

| 阶段 | job ID | 结果 |
| --- | --- | --- |
| inputs | 121073526214 | success |
| l3 / l3 | 121073561598 | success |
| candidate-ci / verify | 121073561691 | success |
| promote | 121076567782 | success |
| main-ci / verify | 121076609299 | success |
| publish / release | 121078749064 | success |
| publish / l3 | 121078749704 | skipped |
| publish / public-acceptance / verify | 121080843837 | failure |
| publish / preview-completion | 121081240864 | skipped |
| publish / stable-completion | 121081240963 | skipped |

外层L3唯一入口run-release-l3.mjs --github-hosted，实际make verify-release、web/go/deploy三组退出0，候选/tree身份未变，image_e2e=pass；receipt、分组stdout哈希和27条rawEvidence的原始字节/hash/exitCode逐条复算。固定Go1.27.3/Node24.22.1/Docker28.0.4，Runner ubuntu24/20261023.334.3，入口SHA-256 43e57f4fd321846204020d6cf27eb75961f0aefa1284a9187e786601704c2d40；receipt及分组哈希保留l3-artifact-verification.json，不借用旧服务器ID。发布链路全部GitHub-hosted。

1125个first-party Go blob、六目标actual/original及依赖库存与canonical L3完全匹配：firstPartySource bef27786b0b0c3c31340a07c266dbad60fc218ff855f552bb399b05c185387d4；closure d35c81e186d23b6b440e2aaa187cd0c96b86441d97d31d5e2e547a37c9b518f8；selected graph 145130563ab26d84307387f7fd8e71f2d41d7a1152a808c5fd6e8759faa9f818；生产111模块、选中466、standalone150；六目标672/671/239/238/241/240。新生产模块maxminddb-golang/v2 v2.8.0。37项有界保守处置保留，不称37个不同CVE；新10/25..11/08窗口由来源真实新DB及原始报告资格化，DB updated10/24T19:09:13Z/download10/25T01:07:32Z，生产0/selected10/standalone27、scanner退出0/1/1原件保留；最终同SHA扫描另通过，不是仅改到期日。

## 依赖与技术栈

采用undici8.11.2→8.12.3开发传递依赖安全补丁及上述本地MMDB读取器，未改工具链、基础镜像、Action或脚本pin。undici官方GHSA-xjj6-f33q-929c拒绝服务通告及同包告警由实际npm扫描回归验证。最近已核对的旧主线定时安全任务40074702002/job120815467772（2026-10-24，e577e277）因undici高危告警退出2，原日志保留，不改记通过。最终push依赖工作流40160682509的security-advisories/job121071389650按仅定时条件skip；本版实际安全结论来自同SHA候选/主线CI、L3及发布任务的独立扫描，不将skip当作每日审计成功。

6次自动依赖报告（逐项run/job见dependency-disposition.json）均failure，均完整9/9、emergency-security0，以exit3报告两项既有到期维护例外。最终产品330信号=33直接/基座+297传递归属；EOL current至2026-10-28T23:59:59.999Z。TypeScript6.0.3→7.0.2与@types/node24.19.0→26.6.5由KPanel base maintenance负责，reviewDate2026-10-23保持due；分别需Vue支持TS7全套检查及批准Node26LTS运行时，不为本版兼容性贸然升级、不续期、不把报告记为成功。其余未采纳信号按14.1原责任/期限独立处置。最终完整产品CI/L3安全门禁通过，与维护报表状态分别记录。

## 多维质量与功能预览

| 维度 | 状态 | 证据与边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 同SHA全量门禁及真实公开容器旅程；默认全新轻量Node预览接入存在上文限制 |
| 网络入侵与供应链安全 | 已验证 | 原安全交付4组原始回执、独立复核、新组装审查、当前扫描和冻结产物；不作全仓无漏洞保证 |
| 稳定、失败恢复与兼容 | 已验证 | race、SQLite/去重/重试夹具及公开容器重启；跨发行版和实体宿主恢复未专项验证 |
| 性能与资源预算 | 已实现未实机验证 | 上述批次/行/数据库边界及回归；无真实生产吞吐、RSS、长期soak结论 |
| 用户体验与可访问性 | 已验证 | 同SHA Mock21项、暗/亮/WAF/390/720+DPR2及英文/繁中、筛选/暂停/清空/503重试；真实读屏/实体手机/原生缩放未验 |
| 数据、配置与迁移 | 已验证 | SQLite独立历史和配置契约回归；历史需单独备份，退版不自动迁移 |

本地预览rc4-site-monitoring-refrozen-1792901242948-877df2绑定最终f673，Chrome155.0.8059.85，21项通过、errors[]。合成日志/testMMDB和MockUI只证明对应夹具；清空动作在浏览器上下文拦截，实际清空/checkpoint由Go测试证明。未把Mock当真实生产日志，未连接生产采集。原UI harness初始化错误及改正后的相同断言原件保留；未改产品或过滤错误。

## 公开产物与真实浏览器

[公开Release](https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.4)，ID423156713，draft=false、prerelease=true；版本/preview index `sha256:850f4c5c56dbbc0c6f450bee672bad577b1b0a0b9ffd79ca4ae537c2e0311211`。tag对象/peel与冻结候选按source-archive.json远端核对。

- `amd64`：`sha256:1081219705f53a935a5c2b4a2149d3793024381dd828274f810dbc9341bfb489`。
- `arm64`：`sha256:b0caa2ce0d14b29a19cb93684c9fb41f759ef95e224479291aca81fc20af0add`。

15附件名称/大小/API digest/真实下载及磁盘回读、11项SHA256SUMS、OCI source/version/脚本pin、SBOM/provenance均通过。snapshot和publication的三份冻结JSON字节相同，最终正文摘要与公开Release完全一致。

首次公开job121080843837完成15附件/11校验和/两架构身份后，在初始化旧HTTP请求处失败，browser checks0；ZIP12587497913 sha256467a92e4c14b2473d41f6d7ed8fe0fab682e6ca2b96d7de92fcd338c141f95b5与原日志保留。实际HTTPS PublicURL按新安全契约正确返回426，不把旧夹具失败当产品允许明文的理由。

只读验收修复 `390593921d92f2012c8fca0c35686cd4dea62dee`（代码2db4c74e）仅5文件：固定workflow、real-feature shell及其CJS companion、两份既有回归。唯一受信overlay从2个增为3个精确命名资源，各自原文/hash/diff分开wx记录，先解析全部来源后才写，dirty/untrusted/head/version/index检查保留，不允许目录级overlay。显式HTTP初始模式经真实UI启用TLS，实际重放旧Cookie要求HTTPS401，再重新认证；HTTP session426，证书损坏仍health200但session/login426，仅显式离线CLI恢复后HTTP匿名401。原断言和超时保留，未改产品源码/tag/image/15附件。

固定6矩阵/6维独立初审FAIL发现仅匿名401不足以证明旧会话撤销，2db4c74e补齐真实旧Cookie内存重放后PASS；不是产品撤销漏洞。初审SHA935859f0ffc9ead678a511b1c3d8ec12ceded742337b086da3e925b25e82e31a，最终SHA4e9cbde1eab4fc59707831edbca25ae48eaf7a4908a5dfbd39ab750f098bdb89，fallback=provider-unavailable。OCR最终5/5、blind=false/unreported；原34项定向及语法通过。helper精确候选CI40164856896与同SHA主线CI40165488375原日志均证明完整359项治理后归档。首次只读恢复40166176233/job121087575054通过31检查后，真实HTTPS导航使旧响应body被浏览器释放，夹具response.json报错；ZIP12588885817 sha25643d1f252599c44094f0e9ce3ce1db508b22f599aaad43e5c463666b12b41f598及原日志保留。随后单CJS058c25e8仅2增1删：启用UI响应仍要求200，启用状态与指纹继续由实际认证HTTPS接口断言，其他旧Cookie/426/证书/恢复断言与超时不变，无workflow或守卫改动；这是root复核的普通小型L1修正，不冒称新独立/CF审计。候选CI40166533446、同SHA主线CI40167070890成功并归档后，执行原run40161392425/1 snapshot的[只读恢复40167684516](https://github.com/kejilion/KPanel/actions/runs/40167684516)，终态success；未重发publication或改写两次原失败。

固定公开Runner实际ID `sha256:32e767a0097915a8402d50e72f1267ed0ac3e79d2847eeb831f38a1e5870ac43`，recipe SHA-256 `92d81f0410dd2a05b28b2b6554a0c865696263149306e6ef6d6a2923ff92b48f`，shell SHA-256 `d82f39b46463fe7dca2fdab2a2a65e377d64a2d80c6d4d01f1fe53dac91e543a`，CJS SHA-256 `89a943186734c3f78f612817649e7163006e2925a2e518132290680e038594bd`；三份收据原文/hash/diff分别与冻结candidate及可信controller058c25e8 Git blob逐字节核对。Runner配方不变，仅两份固定夹具overlay，冻结产品完全不变。四组共150项检查通过，报告mocked=false，errors[]、assetFailures[]，digest一致：

- `kpanel-real-40167684516-1/results/initial/report.json`：48项，passed；完成2026-10-25T06:04:02.003Z。
- `kpanel-real-40167684516-1/results/restart/report.json`：26项，passed；完成2026-10-25T06:04:08.176Z。
- `kpanel-browser-published-40167684516-1/results/report.json`：45项，passed；完成2026-10-25T06:04:22.542Z。
- `kpanel-browser-upgrade-40167684516-1/results/report.json`：31项，passed；完成2026-10-25T06:04:33.556Z。

覆盖实际应用、偏好/桌面/终端恢复、HTTPS/证书故障、通道切换、发布接口与更新弹窗。预览加入只切来源并检查，不自动安装；自动安装开关独立；退出较高预览不产生降级；旧状态默认stable并保持选择。应用库存和CA为测试夹具；不替代真实OneDrive、systemd/OpenRC宿主安装、WAF真实拦截、全新Node默认脚本或长期soak。

GitHub Latest ID420434596仍v1.27.2，Docker latest `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`与独立发布前快照相同。上一预览RC3 `sha256:1f112898aeec3cda3474a5a5d22a3c6c007493d7733fd6670869e8ba6c742eb6`保留作为回滚点。

## 原始证据与候选处置

- `l3-40161392425-1`：artifact 12586902966，ZIP `sha256:3593a68f4e7a896b68de4b6817fd6d7d280c9c27aeec9ec49587ff777470d547`。
- `snapshot-40161392425-1`：artifact 12587577449，ZIP `sha256:466f9e3682997b9ede5033794ea3c91e9cdc9eb9fa3fc24c6346f6f774c74d8c`。
- `publication-40161392425-1`：artifact 12587307874，ZIP `sha256:4c7387e09bddbc6ddcc98bea2d347f15a1b0045b46cf41311d9c4f4bca207861`。
- `public-40167684516-1`：artifact 12588469993，ZIP `sha256:32932b3b610005c6dd540a62af0fd0cd5b2a88c19694faeb1a967529b6ee89df`。

仓外证据C:/GitHub/_release-evidence/kpanel-v128-rc4。ZIP先核对API大小/digest、重复/危险路径/符号链接/解压大小，再读取和解压；扫描JSON、分组stdout、公开报告和清单均复算，失败原件全部保留。

- `fix/security-ledger-closure-20261025` / `7b38aa797bcc77d35372f5ae8d91e40b402fb088` → `archive/fix/security-ledger-closure-20261025`；远端归档回读及本地clean核验通过，保留工作树、解除旧upstream。
- `feature/site-world-traffic` / `04d06654dea28431d4be76e59efb4b3ce4fcf960` → `archive/feature/site-world-traffic`；远端归档回读及本地clean核验通过，保留工作树、解除旧upstream。

唯一RC列车release/v1.28.0-candidate保留在f6730689；不归档预览列车。工具archive/fix/rc4-trusted-source-tools精确8b9a0d58、archive/fix/rc4-public-transport精确39059392及archive/fix/rc4-public-response精确058c25e8保持。只归档两份已交付聚合源候选，不擅自清理原子任务其他分支或缓存。

本验收记录仅docs/release-v128-rc4-acceptance，须同SHA候选CI→SSH快进main→同SHA主线CI→归档；最终提交/CI/引用由交付回执补齐，不生成新tag/image。管理工作树本地main0524bbcb保持不变，所有原工作树和证据保留。已停止本发布自己启动的预览，其他源任务预览不处理。本轮不新增永久工作流或知识文档。

## 生产与回滚

生产动作不适用（预览版禁止生产部署）。未连接任何生产目标，prod-108/108未连接、未备份、未部署、未升级、未核对。隔离容器结果不计生产状态。

回滚点为RC3及稳定1.27.2上述固定digest；实际回滚前需要Panel/Agent数据、宿主WAF配置和独立site-traffic/WAL/SHM备份，选择固定版本后复核健康与兼容；退出preview不自动降级。本次未实际生产回滚、备份或作生产健康结论。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-24T16:36:10+08:00
- 候选冻结时间：2026-10-25T04:07:22.619Z
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：18
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "qualification/version-check/wrong-interpreter",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial supplementary version check mistakenly invoked a Bash file through Node and stopped with SyntaxError before script execution. No product or publication action.",
    "recoveryEvidence": "version-check-first-failure.txt retains original command and error; correct existing run-repo-bash adapter reruns the same version check with fixed Go PATH.",
    "permanentAction": "Release owner uses repository run-repo-bash adapter for all Bash checks; no script or acceptance threshold changed.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/ocr-coverage/incomplete-file-classification",
    "position": "before-production-write",
    "count": 2,
    "impact": "Two local coverage helper attempts rejected missing explicit VERSION classification and known MMDB binary exclusion; neither produced accepted coverage.",
    "recoveryEvidence": "ocr-coverage-first-failure.txt and second-failure.txt retained. All257 reviewable files and23 exclusions now classified; exact single binary matches delivered Git blob. Coverage1, no missing test defaults or claimed blind gain.",
    "permanentAction": "Release-owned helper explicitly classifies assembly metadata and permits only exact reviewed MMDB binary. Canonical OCR rules, source and thresholds unchanged.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preview/playwright/init-before-document",
    "position": "before-production-write",
    "count": 1,
    "impact": "Current UI driver completed interactions but final pageerror assertion rejected4 errors caused by its own new-document theme injection reading documentElement before creation. Product source unchanged.",
    "recoveryEvidence": "Original verify-ui.cjs, ui-validation.log and preview/browser/ui-failure.txt/screenshots/trace retained. Second driver waits DOMContentLoaded for theme initialization; all original assertions and55s timeout retained, outputs isolated in browser-r2.",
    "permanentAction": "Task-owned UI harness performs DOM-dependent initialization after document creation; no product change or error filtering. Exact same candidate revalidated.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/release-automation/unintegrated-source-tools",
    "position": "before-production-write",
    "count": 1,
    "impact": "Static pre-dispatch automation equality check found6 delivered executable script changes absent from trusted main; original product candidate not dispatched or published.",
    "recoveryEvidence": "Original a9 frozen/CI evidence retained. Trusted8b9 exact candidateCI40158857317/mainCI40159781103 complete358 governance and archived; f673 refreeze currentCI40160682535 and L340161392425 passed with exact product identity. Guard unchanged.",
    "permanentAction": "Release owner integrates executable source tools as a separate exact-SHA trusted prerequisite before existing preview dispatch; never push product to main early or relax automation equality.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/governance/windows-atomic-state-rename",
    "position": "before-production-write",
    "count": 1,
    "impact": "Local full governance357/358; unchanged background runner fixture failed Windows atomic state replacement with explicitEPERM, leaving queued state and failed tmp receipt. No product or remote write.",
    "recoveryEvidence": "helper/governance-local.log and background-first-state/spec/temp-state preserved. Same04220 revalidated offline in registered isolated Linux:358/358; final loga167818d and receipt retained.",
    "permanentAction": "Release owner qualifies OS-specific background worker semantics in registered Linux, preserves Windows failure, does not edit unrelated runner or expand timeout; production and officialCI/L3 remain GitHub-only.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/governance/inherited-git-transport-wrapper",
    "position": "before-production-write",
    "count": 1,
    "impact": "First offline Linux governance336/358 used inherited managed Git wrapper that rewrote local fixture SSH transport to real HTTPS, producing22 fixture failures. Not accepted as product or governance success.",
    "recoveryEvidence": "helper/governance-linux.log preserved. Existing reviewed private wrapper forwarding exact args to/usr/bin/git with global/system config isolation prepended to PATH; same04220 then358/358 and clean source. No fixture,assertion or policy change.",
    "permanentAction": "Release owner binds actual Git executable and isolation wrapper hash in grouped qualification; do not rely on ambient Git URL rewriting. Raw failures remain archived.",
    "historicalReleases": []
  },
  {
    "fingerprint": "maintenance/dependency-report/expired-toolchain-exceptions",
    "position": "before-production-write",
    "count": 6,
    "impact": "Complete9/9 freshness report exits3 because two existing TypeScript/Node-type major-upgrade exceptions are due10/23. No missing detector or emergency-security signal; not represented as passed maintenance.",
    "recoveryEvidence": "All4 automatically triggered complete reports failed exit3 with9/9 sources, emergency-security0 and same2 due exceptions:40157107690/121060722086,40158857339/121065942242,40159781075/121068721536,40160682509/121071388650. Original logs retained; no manual rerun or deadline renewal. Fifth automatic report40164857128/job121083709629 also complete9/9/emergency0/same2 due/exit3; original log retained. Sixth automatic main report40165488349/job121085551536 also complete9/9/emergency0/same2 due/exit3; original log retained.",
    "permanentAction": "KPanel base maintenance must review existing major-toolchain adoption evidence through14.1; ordinary release isolates unrelated maintenance debt and still requires actual product security/CI/L3. Do not reset deadlines or upgrade majors for green status.",
    "historicalReleases": []
  },
  {
    "impact": "Original public40161392425/job121080843837 completed public asset identity, but HTTPS-configured product correctly returned426 to legacy plaintext bootstrap. Browser initial checks0; no functional browser pass claimed.",
    "fingerprint": "public/real-feature-fixture/stale-transport-contract",
    "permanentAction": "KPanel release-tools maintainer updates only fixed verifier resources to actual secure-transport/session-revocation contract; no product bypass, assertion removal or timeout extension. Independent5.3 review and exact helper candidate/main CI before readonly recovery.",
    "recoveryEvidence": "Original public ZIP12587497913 sha256467a92e4 and failed log retained. Independent fixed6 matrix390 passed after actual-cookie replay correction, candidateCI40164856896/mainCI40165488375 complete359. Later058 response-only fix qualified; readonly40167684516 passed48/26/45/31 with samef673/850 digest and original40161392425 snapshot. No product republish.",
    "count": 1,
    "historicalReleases": [],
    "position": "before-production-write"
  },
  {
    "impact": "Task-owned syntax-check command passed -n as repository launcher script path and stopped before Bash execution. Subsequent targeted tests independently passed; failed syntax invocation not accepted.",
    "fingerprint": "qualification/shell-syntax/launcher-script-argument",
    "permanentAction": "Release owner uses fixed launcher API only for script execution and explicit resolved Bash for shell syntax checks. No shared launcher change or claimed permanent canonical repair.",
    "recoveryEvidence": "public-helper-syntax-first-failure.json retains exact message. Existing resolveBash() invoked as native Bash with -n and exact script path passed.",
    "count": 1,
    "historicalReleases": [],
    "position": "before-production-write"
  },
  {
    "impact": "Unpublished0e3 review checkpoint placed OCR-Review in a separate paragraph before Verification-Level. Git trailer parser reported ocr_line_review=missing; it was not accepted as coverage record.",
    "permanentAction": "Release owner writes one message file with adjacent final OCR/Independent/Verification trailers and confirms actual Git trailers with canonical role check; no parser or advisory rule changes.",
    "recoveryEvidence": "Original0e3 retained with public-helper-first-role-check.txt. Continuous-trailer390 checkpoint has identical reviewed2db tree; canonical final-role-check reports OCR recorded and independent recorded/cross_provider=false before initialpush. Exact candidate/mainCI then passed359; no parser change.",
    "historicalReleases": [],
    "position": "before-production-write",
    "fingerprint": "qualification/review-record/nonterminal-trailer-paragraph",
    "count": 1
  },
  {
    "fingerprint": "evidence/workflow-run/terminal-metadata-lag",
    "impact": "Archive guard rejected saved workflow run in_progress/null while its job had completedsuccess. No remote archive or readonly recovery dispatch executed.",
    "recoveryEvidence": "public-helper-main-ci-before-terminal.json and public-helper-archive-first-rejection.txt retained; fresh no-cache run40165488375 now completed/success at exact39059392, original joblog359 passed. No workflow rerun.",
    "count": 1,
    "historicalReleases": [],
    "position": "before-production-write",
    "permanentAction": "Release owner refreshes and verifies both workflow-run terminal status and job terminal proof before archive/recovery; keep strict guard and original stale observation. No parser, threshold or product change."
  },
  {
    "permanentAction": "Release tools maintainer reads status from navigation response and actual enabled state from subsequent authenticated HTTPS API; avoid reading disposed page-response bodies. Keep all original security assertions, artifacts and bounded workflow entry.",
    "historicalReleases": [],
    "fingerprint": "public/browser-fixture/navigation-response-lifetime",
    "count": 1,
    "recoveryEvidence": "public-recovery-first-run.json/public-recovery-first-failed-job.log/ZIP12588885817 sha25643d1f252 retained and all3 source receipts independently checked. SingleCJS058 keeps200 and actual TLSenabled/fingerprint assertions; candidateCI40166533446/mainCI40167070890 success. Fresh original-snapshot readonly40167684516 passed150 checks, errors[]/assetFailures[], immutablef673/850 and assets unchanged.",
    "impact": "Readonly40166176233/job121087575054 passed31 checks, then real intended HTTPS navigation released UI response body before fixture response.json. No product or secure-transport failure claimed; original failed artifact retained.",
    "position": "before-production-write"
  }
]
<!-- kpanel-release-process-incidents:end -->

已读本轮原始失败日志，并比较最近5个正式版本v1.27.2、v1.27.1、v1.27.0、v1.26.0、v1.25.1异常明细，已记录可识别指纹中无同名根因复发；旧版总量未记录的部分不推断为零，也不将不同浏览器根因强行合并。6个独立自动维护报表失败分别计数，不合并降低指标。原a9未发布、refreeze不算重复产品发布；只读恢复不重发产品。本版无产品重发、回滚或紧急热修复。

滚动14天正式发布4次、生产完成证据0次，不将缺失生产证据推断为实际零部署。最近20正式版本验收覆盖100%，生产完成及用时证据覆盖50%，中位提交到生产5.45小时、冻结到生产0.7小时，变更失败1/20=5%，有恢复记录1项；17版本报告流程指标、共294异常/31生产写后，19项历史重复已声明、未声明0。完整口径与历史记录另存release-metrics-before-closeout.json及release-metrics-final.json；预览不混入生产交付频率、覆盖率或用时。
