# KPanel v1.26.0-rc.20 发布验收记录

日期：2026-10-15。发布级别L3，releaseChannel=preview，releaseTrain=1.26.0。批准基线81a8ce0ede23e93199817960c4e85d6a33799c11；冻结产品7723da7dfe991e8cd3d1d1341e5231afc947f4be。[v1.26.0-rc.20](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.20)于2026-10-14T17:31:08Z公开，draft=false、prerelease=true。产物已发布，生产未部署。

## 发布画像与候选处置

集群公开分享新增本地主题ZIP上传，上传后独立应用或删除，标记本地来源，不自动启用、不覆盖同名已安装主题。帮助页按语言动态读取Bilibili最新搜索或YouTube固定列表，最多9条真实标题和封面，提供共享缓存、失败重试和显式旧缓存提示。无新增Agent动作、宿主机权限或端口。

| 来源分支 | 精确tip | 处置 |
| --- | --- | --- |
| feature/share-theme-upload | 4f0a87c767b5700fa2c9ca2a81845b8c3e23250a | 普通merge纳入，为产品标签祖先。 |
| feature/guides-dynamic-videos | 84dd1a27e306661474fc37435d0acb81cdfda737 | 普通merge纳入，为产品标签祖先；旧依赖快照冲突保留RC19基线并重新资格化组合。 |

来源最近对话completed、桌面状态notLoaded，未把不可见调度器状态推断为idle。76个本地活动refs已逐项核对，branch-inventory.json记录精确tip、祖先关系和历史处置依据。56c507547ff18ed86ecda0c42741474d3cb3bbc9快捷命令已在RC18交付且为本轮祖先；systemd已随RC9交付；Shoutrrr和Issue29已在RC19等价重放交付。Issue30撤销净零、历史审计材料、废弃旧文件快捷方式图标及旧分页/文件页布局继续排除，没有重复合并。来源分支、作者工作树/预览服务保留所有权；恢复凭产品标签祖先及已验摘要bundle。预览train分支保留产品SHA，发布/验收检查点归档，最终refs在final-disposition.json。

## 业务、安全与数据

ZIP压缩6MiB、解压合计5MiB、最多40文件；校验路径、类型、重复和manifest，服务端派生文件摘要，私有能力目录、原子安装及失败清理沿用现有机制。上传和应用分离，冲突拒绝覆盖；认证/Origin/CSRF/审计保持。主题只接收白名单公共DTO，sandbox=allow-scripts、opaque origin、受限CSP，不执行宿主命令。教程固定两个上游来源、有界响应和8s共享获取，现有缩略图proxy与域名白名单；两缓存槽、1h TTL、1min失败退避、最多7day旧缓存，语言切换中止过期请求。

完整first-party源码SHA256 d0d8d7ea70e50f44bb8b6ebc0ee11f8dd19e2ab09a5ca9539f282323df9d4c6a，930个Go文件；selected graph 24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927；production closure 6977f65200880d9310e2046b843449ee6947fd55c8c9e340e388516f23a8b18d，110模块。仅现有Linux Panel闭包各加入tutorial_feed.go与upload.go；上游身份、模块版本、目标、许可证未变。36项既有reviewed findings观察2026-10-11、到期2026-10-25不延长；原始scanner非零项保留，按受跟踪基线资格化，不称零漏洞。

## 外部审计与独立复核

普通发布源码复核独立于两个功能实现者，无新增成立发现；其他provider CLI不可用，明确同provider回退，不称跨provider复核。OCR自由臂先于约束臂，25/25可审文件覆盖，5生成/证据文件排除，constrained-only=0。CF scoped/full本轮未执行，coverage decision=scoped-required；security-coverage-final.json保留未覆盖提交/文件和既有风险，稳定前必须补审，不把普通复核或浏览器隔离证据称CF审计通过。

## 跨仓库联动判定

scriptLinkageState=not-required（无需发布脚本（不适用）），变更集编号/脚本候选不适用。Panel原生主题数据、既有公共分享协议和固定视频来源，不改变脚本协议、运行时动作、宿主产物、安装或存储布局。实际内置脚本1800d955f216aebd2776674368a8e469a671c489 / SHA256 5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c保持；沿用RC19应用市场8b8d0c452924a2b11f57d3e5836c8e2decc805e9，kpanel.conf blob cb3740a4319c19b2820586390bf407451c374544。本轮不重复发布脚本/apps；继承RC17 Panel/Agent配套及旧反代proxy_read_timeout360s要求。

## 自动门禁与依赖

唯一入口scripts/run-release-l3.mjs，run ID v1.26.0-rc.20-7723da7-l3-r1，arena-154，Runner kpanel-go127-security-runner:go1.27.2-node24.21.0 / sha256:4edff59fa4c1441e3e97594593b255cc57a74e51dabbdd8db295e4ff51d0ca4f。完整verify-release、安装生命周期通过；12原始evidence文件、计划/script/bundle摘要、evidence.sha256/status在l3-r1/remote-evidence，额外source lanes三份日志摘要匹配。源码/核心race/torrent回归、扫描、跨架构构建、镜像契约、容器/脚本/自更新、SBOM/provenance均按入口执行。

候选CI 38598075503、产品主线CI 38602533667、Release workflow 38604797651均success，head=7723da7dfe991e8cd3d1d1341e5231afc947f4be；依赖新鲜度单独保存。草稿与实际公开正文均经过实际Panel解析器TestRenderedReleaseNotesForPublication，3更新摘要、3升级提示。Go1.27.2、Node24.21.0、基础镜像/Actions/scanners未升级；最近Dependency freshness作为依赖检测源，未延长旧观察/风险期限，未知或EOL项不凭门禁成功推断不存在。

## 隔离环境与浏览器

arena-154登记candidate/browser validation。Windows原生Panel与本地预览启动整条命令被自动审批拒绝，仅返回blocked by policy，未执行；未拆分或重试同等原生启动。改为非root65532、只读、cap-drop ALL、no-new-privileges、受限内存/CPU/PID、仅owned data且无Agent的真实镜像后端，127.0.0.1:18120绑定。浏览器同样受限、后台作业有900s上界及持久终态。镜像身份通过L3冻结source计划和新构建immutable image ID绑定；verify镜像revision标签未设置时不冒称已验证SHA标签，公开镜像另行核对完整revision。

真实后端浏览器165项通过，Bilibili实际HTTP502、0条（video_list_unavailable，未验证该隔离主机上的成功列表），YouTube实际HTTP200、9条；上传/不自动应用/显式应用/重复拒绝/删除恢复默认/旧能力撤销，以及opaque origin、父文档拒绝、CSP联网拒绝、公共DTO与白名单动作，详见browser-results/report.json。三语言×明暗×1280/1024/640/390/320共30组真实页面：英语验证YouTube链接/真实封面，中文验证真实上游失败提示/重试/更多入口，全部验证实际主题及无横溢，125%/200%为桌面等效CSS视口，非原生缩放。已独立查看4张代表性实际PNG（非全部36张）；无Agent，不把本轮验收当宿主服务管理或真实跨主机运行证据。外部平台限流/接口变化、实体设备、读屏、原生ARM运行和长时压力未验证。受控故障夹具与来源自动测试不称真实平台故障证据。

| 维度 | 状态 | 证据与限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 主题真实Panel交互、YouTube成功/Bilibili不可用反馈、现有脚本兼容；无新增脚本协议。 |
| 网络入侵与供应链安全 | 已验证 | 完整扫描/闭包/ZIP/CSP普通复核；CF待补，不表示无风险。 |
| 稳定性、失败恢复与兼容 | 已验证 | 核心race、冲突/失败清理、缓存/过期响应夹具；平台变化不可保证。 |
| 性能与资源预算 | 已验证 | 压缩/解压/文件数及缓存时限上界；无长期压力结论。 |
| 用户体验与可访问性 | 已验证 | Chromium三语言/明暗/视口及真实旅程；原生缩放/读屏未验证。 |
| 数据、配置与迁移 | 已验证 | 现有原子存储、本地来源标记及删除恢复；降级需备份自定义主题和Panel数据。 |

## 公开产物、自更新与回滚

版本/preview OCI index=sha256:ee0c159cedf525d3da3182b1ec2960f1b70408c648bb1bc411a06c85199938bb；Registry双平台version/revision与冻结SHA一致，amd64/arm64 manifest在public-image-metadata.txt。15附件磁盘重读大小/API摘要、11项SHA256SUMS通过；公开immutable镜像image_e2e=pass。GitHub Latest仍v1.25.1、Docker latest仍sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325。实际Release正文解析通过；live Panel更新API与弹窗未验证，不能以解析器或正文存在替代旅程。

既有自更新自动验收涵盖稳定/预览来源、加入仅检查、独立安装开关、通道保持、退出不自动降级、备份/失败恢复和隔离；OpenRC/轻量Node边界按release-channels.md。上一预览回滚点v1.26.0-rc.19 / 34666bb4225b1aaefb270108eb856ba06b74dbe7 / sha256:56a596329f1489c7195d104facb8686f1caacbe70b3efa1fa40ed4891e82fc36；降级前备份Panel数据与自定义主题，旧读取器不保证理解uploaded元数据，必要时恢复升级前数据。稳定回滚点v1.25.1及原latest摘要保持。

## 生产部署与清理

不适用（预览版禁止生产部署）。prod-108/108禁用全部KPanel操作，本次未连接、未备份、未部署、未核对；生产写入0、无生产回滚或紧急热修复，未覆盖已发布版本。仅回收自有L3 clone、临时数据/浏览器依赖和本轮缓存，保留原始失败/成功、scripts/spec/截图/锁、bundle、可信Runner及公开资产。作者工作树/预览不动；此前自动审批拒删的Windows临时目录继续保留。卷观察净释放3989270528字节，不宣称Windows C盘净释放。证据根C:/GitHub/_release-evidence/v1.26.0-rc.20；公开资产/root/kpanel-release-evidence/v1.26.0-rc.20-public-assets/。复用既有流程，没有新增知识或工作流。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-14T22:23:44+08:00
- 候选冻结时间：2026-10-15T00:25:20+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：3
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preview/native-panel-launch/auto-review",
    "position": "before-production-write",
    "count": 1,
    "impact": "Automatic approval rejected native Windows Panel and local integration preview startup, returning only blocked by policy; entire command did not execute.",
    "recoveryEvidence": "Use registered arena-154 with isolated nonroot/read-only containers, bounded resources and owned data only. Original rejection retained; no equivalent native launch retry.",
    "permanentAction": "Prefer registered isolated-container integration validation; do not bypass rejection by splitting the native startup command.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preview/external-bilibili-unavailable/rc20",
    "position": "before-production-write",
    "count": 1,
    "impact": "First live browser job timed out waiting for videos. Diagnostic r2 confirmed Bilibili backend 502 video_list_unavailable; YouTube returned 9 live videos.",
    "recoveryEvidence": "Original r1 state/log/report retained. Later jobs classify actual upstream failures and verify retry/more links; no Mock substituted for live source.",
    "permanentAction": "Keep external-provider availability as explicit release limitation; do not claim Bilibili live success from arena.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preview/theme-preference-fixture/rc20",
    "position": "before-production-write",
    "count": 1,
    "impact": "r2 fixture set localStorage directly, but existing authenticated appearance sync restored server theme; dark assertion failed.",
    "recoveryEvidence": "r2 failure retained. r3 uses existing visible theme toggle and asserts actual html data-theme, preserving true UI semantics.",
    "permanentAction": "Use actual UI for server-synchronized preferences; do not infer rendered mode from storage.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
