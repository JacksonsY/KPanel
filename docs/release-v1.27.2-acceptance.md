# KPanel v1.27.2 稳定版发布验收记录

日期：2026-10-22。发布级别：L3。`releaseChannel=stable`，`releaseTrain=1.27.2`。

**产物已发布，生产未部署。** [v1.27.2 Release](https://github.com/kejilion/KPanel/releases/tag/v1.27.2) ID `420434596`，2026-10-22T07:49:50Z 公开，draft=false、prerelease=false，GitHub Latest=v1.27.2。发布 [39737690219](https://github.com/kejilion/KPanel/actions/runs/39737690219) attempt 1 的 L3、release、公开验收和稳定候选归档全部成功；preview-completion 不适用于本稳定版。未重跑发布、未调用恢复入口、未覆盖已有版本产物。

精确产品候选 `bcae4f2a53afe4a24dd3f81b073af475abf6b557`，tree `8bd5e2579c1530e0c42990fdbc772ac3a83abd08`，annotated tag 对象 `790949c91cd8eb7bc705daf8e009f3005ac9c343`。验收文档属于发布后的事实收尾，不改变上述产品身份。用户授权本轮补丁整合、SSH 推送主线、标签、Release 与镜像发布；未授权生产部署。

## 发布范围、业务真源与回滚

本版为 1.27.1 的三项前端补丁，没有新增功能列车、依赖升级、后端协议、端口、Agent 权限、数据结构或迁移。宿主机资源与既有 `kejilion.sh`/Panel/Agent API 继续为业务真源。

| 来源候选 | 本次重放 | 用户可见行为 |
| --- | --- | --- |
| `219b502c`，`fix/terminal-batch-output-bottom` | `37463e64` | 单项/全部展开和迟到日志首次显示末尾；用户向上阅读时轮询不抢位置 |
| `dbc94dd6`，`fix/app-script-toolbar-remove` | `2fcb8b69` | 移除脚本终端重复提示和结束按钮；关闭需停止确认，最小化与刷新保留任务 |
| `dde6a248`，`fix/help-video-reliability` | `99212b49`，重放产品树相同的 `7ff8dd5d` | 内置教程与本地封面即时可用；后台更新失败、空列表、重试均保留内容，两平台隔离 |

`dde6a248` 与 `7ff8dd5d` 的完整产品树相同，后者用于避免带入已撤回的中转站实验；本版没有教程后端、应用市场中转站或定时任务改动。另含 `6db9be01` 版本/日志准备、`7c30c430` 关闭竞态修复及最终源码身份资格化。相对批准主线 `569d181dfb57fe015692c6b9b8c686ca291235f2` 共 15 文件，未纳入其他旧分支。

回滚点为 `v1.27.1` / `sha256:be8a8e16a484d536094e335307424a39968f321fa0a7091b5d2630aade36a23c`。升级保留配置与数据，无额外迁移；仍按既有生命周期流程备份，回滚使用配套数据备份，不仅替换镜像。本轮没有实演实体宿主机升级或回滚。

## 独立复核、OCR 与安全边界

代码候选由独立干净 Codex 上下文只读复核。其他提供商没有可用非交互入口，明确记录 `fallback=provider-unavailable`，没有将同提供商复核冒充跨提供商。首轮 `6db9be01` 发现 P2：任务发现仍在加载时先 abort，可能未停止已有任务即关闭窗口；该轮为 FAIL。`7c30c430` 将等待发现/创建置于 abort 之前；新增 inventory/jobs/action 在途关闭回归，旧实现两项失败、修复后相关 16 项全部通过。独立确定性交错复验确认：等待时窗口未关闭，返回任务后查询并取消一次。此问题不是安全漏洞或已部署事故。

最终独立复核为 PASS，绑定 `c93c02df`；最终 `bcae4f2a` 只增加审查 trailer，tree 相同。复核同时确认 1007 项第一方 Go 路径集合不变，唯一 Go 内容差异为版本常量；版本引起的来源摘要已最小重绑，依赖闭包、构建目标、37 项既有处置及到期日未变。

OCR 1.12.11 的初轮自由臂先落盘，再解析规则；最终 13/13 可评审文件已逐项覆盖，Markdown 和锁文件另行人工/版本元数据比对。H0/M0/L0，最终 `constrained-only=unreported`：后续独立评审发现的竞态不计作 OCR 收益，不能重新包装为盲测。稳定基线 `v1.27.1..bcae4f2a` 汇总有 4 条非 skipped trailer，其中 2 条数值有效、2 条 unreported；另有 2 条纯文档 skipped。数值记录仅作格式及历史观察计数，不推断工具有效或补写旧自由臂顺序；试用退出按下一治理周期处理。

稳定前安全补漏入口实际输出 `security_review=not-applicable groups=0 invalid_trailer=true`。本轮前端文件和版本常量不在敏感包自动审查路径内，invalid trailer 提醒保留，不解释为安全覆盖通过；未执行 CF 或全仓安全审计。1.27.1 的既有中风险待办和未审范围继续保留，本版不关闭这些风险。

`scriptLinkageState=not-required`，无需发布脚本（不适用）；无跨仓变更集或脚本候选。内置脚本 commit `160924a498d9b8d071fed72f165d23c1b9e886de` / SHA-256 `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`，两架构镜像标签、脚本契约和应用生命周期检查匹配。关闭继续使用既有结构化取消动作，不增加新脚本或主机操作契约。

## 自动门禁与供应链身份

- [候选 CI 39734249924](https://github.com/kejilion/KPanel/actions/runs/39734249924) 与 [主线 CI 39735745399](https://github.com/kejilion/KPanel/actions/runs/39735745399) 均绑定精确 `bcae4f2a`，完整成功。变更核验、核心 race、Go 漏洞调用链、npm audit、Trivy 源码/依赖/秘密/配置及脚本生命周期分别有原件。
- GitHub-hosted 完整 L3 于 07:02:06.081Z 开始、07:25:11.387Z 通过，环境 Ubuntu24 runner `20261012.334.1`，Go1.27.3、Node24.22.1、Docker28.0.4。权威 gate 摘要 `43e57f4fd321846204020d6cf27eb75961f0aefa1284a9187e786601704c2d40`。Web、Go、部署三分组退出码均为 0，source identity 未变；逐份日志 SHA-256 独立复算匹配。Linux 前端 314 文件、3105 测试通过、6 跳过；镜像 E2E 和安装生命周期通过。
- `l3-39737690219-1` 原始 ZIP SHA-256 `f2f6965d7ab086c2b7e1413d3d73856470eccd7951f1b5519259b0e7f531b88b` 与 GitHub artifact digest 匹配。原记录 `entrySha256` 指向 `run-release-gate.sh`，并非外层 JS 适配器；本轮取证纠正保留在流程异常中。
- 本地 Windows 全前端 3111 测试、类型检查、i18n 和生产构建通过。本地 `verify-change` 在 torrent race 前因无 CGO 退出，未记为完整通过；完整门禁由上述同 SHA Linux CI/L3 提供。草稿与最终发布说明均经实际运行时解析门禁，公开正文有 3 条修复和 3 条升级注意。
- 第一方 1007 文件的 `firstPartySourceSha256=0dbbab66d8e0a81903806221f2110132c6f615de0dbd619df36486ece685df09`；closure `f5b0d0988c5f7d37ed9cd9cee453c22586efa5d9d1bed42240599df5517eee07`；selected graph `24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927`。固定 Go1.27.3 对六平台 actual/original 闭包重新资格化；候选、主线与 L3 原始扫描匹配新源码身份。
- 三类扫描 `packageCount` 为 production110 / selected465 / standalone150，原始 exit 为 0/1/1、保留 finding 为 0/10/27。后两项是保留完整库存及既有已审处置，不是零告警。观察 2026-10-11、到期 2026-10-25 不变；未延长例外或新增 suppress。
- 候选新鲜度 [39734250031](https://github.com/kejilion/KPanel/actions/runs/39734250031) 与主线 39735745429 成功。已读候选报告原件，必需检测源 9/9；EOL current，截止 2026-10-28T23:59:59.999Z。本版没有升级依赖、工具链、基础镜像、Action 或扫描器，锁文件仅版本元数据变化。
- 最近可见 scheduled 安全审计 39565808978 于 2026-10-21 成功，绑定 RC22 `f0c039d8`，不是本候选；本版当前扫描另有上述同 SHA 原件。检测到新候选不等于采用，继续由既有依赖维护流程分级处置，不在本补丁扩大范围。

## 公开产物、镜像与浏览器

版本及 Docker `latest` OCI index 为 `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`。公开验收实际读取 registry、校验双架构来源/版本/许可证/脚本标签；本机独立复核原件和摘要，没有把本机直连 registry 的未完成探测写成成功。

- linux/amd64：`sha256:3abf6878e9e6bbd00cf4b41a5fbd4518ed8672a165007ede87714ed4b609d4ad`
- linux/arm64：`sha256:b93fd7881a3bb9486a7893825df0dae3a9ea9ec5b5daf9016b66cddb5806e22f`
- preview 保持 `1.27.0-rc.23` / `sha256:144cf771a1043dc584b342cd2cef15e3f0b9280a632706b0c1c04576a1ac0dd7`。

冻结快照 SHA-256 `9bcda539d26bcbc5a57909d4ae98868e9273ea2b1eae6a47ade7627cd3041666`，附件 manifest SHA-256 `6462a1bd27b381ebcff3a9dcbab41fbacb1e51c81f30a3df1637a14c784c60b3`，最终正文 SHA-256 `397ce58ec2678b25361a54e5d47785e92651c40034eb3216126fda95caf2192d`。公开 API 的 15 附件名称、大小、digest 与冻结清单逐项匹配，正文逐字节一致；两份原始 artifact 的 publication identity、快照与清单相同，已下载 modified-component provenance 摘要匹配。公开工具实际下载验证 15 附件、11 项 SHA256SUMS，并核对 SBOM/provenance。元数据归档不是可构建源码包，不包含 paneld 原生部署能力。

`public-39737690219-1` 原始 ZIP SHA-256 `bd865f6fc0a380ea92bc1a3e39a91278a50b977948c7d3914142bd0228c20295` 匹配 GitHub artifact digest。四份原始浏览器报告均 passed、mocked=false、errors=[]：initial44、restart26、published47、upgrade35。数字为各报告断言数，不能相加为互不重叠的独立测试数。真实 Panel/Agent 与 PTY 在隔离容器中执行，Docker inventory 和测试 CA 是声明的受控夹具，没有挂载真实宿主 Docker/systemd socket。

测试 Runner ID `sha256:6bf5cd11f98ddc97361540a0926a3de960084e97676ce2d03002fe2478953416`；产品与控制器均 `bcae4f2a`，Runner 原/实际 recipe SHA-256 均 `e2a026415568924488a35e53e5529aae40e2d57f99640d0a743444316f8a3926`，diff 为空、productImageUnchanged=true。

真实 API 验证覆盖本地应用/图标、偏好和桌面重启恢复、TLS/证书故障恢复、PTY 输出与续接/关闭、命令历史、版本发布信息及旧 RC23 后端发现 1.27.2 稳定更新。默认 stable/自动安装关闭；加入预览不会启用自动安装，取消加入保留 stable，退出不自动降级。无 systemd 的立即安装按预期受控失败并清理排队状态；不是实体主机更新、冷备份或回滚实演。

公开正文、真实发布信息接口和更新弹窗的版本/digest、3 条摘要及 3 条升级提示匹配。1280/390/320 无横向溢出的自动断言通过；人工复核 1280 和 320 截图，更新至 v1.27.2 按钮、三项修复和升级注意区域可见。宽图通知叠层遮住顶部局部版本文字，底部版本按钮仍可见；窄图需内部滚动查看全部升级注意，未声称截图展示所有隐藏内容。原生 100%/125%/200% 缩放、读屏、实体手机、Safari 和长时 soak 未专项验证。

本轮精确候选的本地 UI Mock 另覆盖九条教程在 HTTP502/空列表/重试时保留、五条英文 YouTube 教程隔离、恢复后九条动态夹具替换，以及批量单项/全部/迟到输出 bottomGap=0、PageUp 后位置经轮询仍为3293。脚本最小化/刷新保留任务，关闭确认后窗口消失并提示管理锁已释放。Mock 不证明真实宿主进程已停止。

## 多维质量、权限与遗留边界

| 维度 | 状态 | 已有证据与未验证范围 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 受影响前端回归、同 SHA 门禁和真实隔离 API/PTY；没有新脚本契约，实体混版本节点未验 |
| 网络入侵与供应链安全 | 已验证 | 当前规范扫描、库存、附件/provenance 与独立代码复核；不等于全仓安全，继承风险不关闭 |
| 稳定性、失败恢复与兼容 | 已实现未实机验证 | 关闭发现竞态回归和隔离重启/受控失败通过；实体宿主升级、进程生命周期、发布恢复入口未实演 |
| 性能与资源预算 | 已实现未实机验证 | 无新后端接口、端口或常驻服务；前端缓存限两平台且每份最多九条，ref/计时器延用既有回收；未跑低配吞吐/RSS/长时 soak |
| 用户体验与可访问性 | 已验证 | 三项组合 Mock、公开更新弹窗、键盘日志阅读及宽窄视口；原生缩放、读屏和实体手机未验 |
| 数据、配置与迁移 | 不适用 | 不改存储格式/配置，无迁移；升级和回滚仍保留配套备份 |

生产动作全部不适用：没有生产目标或部署授权，未连接、备份、升级、回滚或核对生产。`prod-108` 禁用全部 KPanel 操作，本次未连接。发布仅使用 `github-hosted`；公开容器验收不能写成生产部署。继承 OneDrive 真实账户/云端往返未验、自签名 TLS 端口仍接受明文 HTTP API 的边界；请限制到可信网络并强制 HTTPS。

## 归档、本地资源与交付事实

发布 workflow 的唯一 `archive-release-candidate.mjs` 已归档原远端 `release/v1.27.2-candidate` 精确 tip `bcae4f2a` 至 `archive/release/v1.27.2-candidate`；远端回读同 SHA，原活跃引用不存在。归档不等于部署。

来源仅本地候选没有额外推送远端功能分支。以上映射和正式 tag 保存纳入证据；原三个功能工作树/分支及其预览保留，未删除或接管未知资源，不再把它们当作尚未上线的新功能。发布产品工作树、发布后文档工作树和本轮原始证据保留以供追溯；当前任务自己的模拟预览已按 ownership manifest 停止，QA 代理在核对 PID 与命令行归属后停止，不涉及来源任务进程。没有执行本地磁盘删除，净释放空间不作推断。

本文档按纯记录候选完成检查、同 SHA 候选 Linux CI 和主线 CI后归档；该记录收尾不重建、不改写已发布标签或镜像。未新增知识库、工作流或永久治理；复用既有版本、审查、预览、发布与验收入口。

## 交付节奏与流程异常

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-22T10:00:48+08:00
- 候选冻结时间：2026-10-22T14:27:29+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：8
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

下列为使必需本地准备/取证无效而被拦截的八次，不是八次产品发布失败。可选网络探测、运行中日志不可下载及浏览器定位适配不计作门禁失败；旧代码回归失败用于证明修复，也不计为流程异常。正式候选/主线/L3/发布作业均首轮通过。工具原回执可追溯；未另存完整原始失败日志的项目只保留工具输出摘录及成功复验，不冒充完整原始日志。

已比较最近五份既有稳定验收记录。无当前加前四版窗口内的同指纹重复；v1.25.0 的 Go PATH 同因虽在窗口外，仍沿用原指纹并声明历史关联，不靠改名隐藏。历史三版计数“未记录”不解释为零；本轮执行修正不冒充已完成永久机制修复。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preparation/scan-inventories/missing-output-directory",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次canonical库存生成因仓外输出目录不存在而退出，未产生通过凭据。",
    "recoveryEvidence": "保留scan-inventory.log；创建独立scan-inventory-r2后Go1.27.3六平台资格化退出0。",
    "permanentAction": "复用项目6.2.2输入预检；此任务显式准备独立输出目录，不修改官方库存入口或放宽身份校验。",
    "historicalReleases": []
  },
  {
    "fingerprint": "validation/verify-change/windows-cgo-unavailable",
    "position": "before-production-write",
    "count": 1,
    "impact": "Windows变更门禁在torrent竞态入口因缺CGO失败，不能作为完整L1/L3通过。",
    "recoveryEvidence": "verify-change-r1.log保留；本地前端3111测试及构建独立通过；完整同SHA Linux候选CI39734249924成功。",
    "permanentAction": "沿用既有Windows前端/静态检查与GitHub-hosted竞态和发布L3分工；不新增跳过开关或将本地失败改记为通过。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/release-notes/go-not-on-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "首轮发布说明已生成，但运行时解析器检查因新Shell未带Go路径而失败。",
    "recoveryEvidence": "原工具输出保留；固定Go1.27.3后release-notes-check-r2.log退出0。",
    "permanentAction": "本任务显式初始化固定Go路径后重验，不宣称临时初始化是永久机制修复；沿用task-preflight资格化。v1.25.0同因在当前加前四版窗口之外，仍保留历史关联。",
    "historicalReleases": [
      "v1.25.0"
    ]
  },
  {
    "fingerprint": "qualification/task-preflight/missing-go-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "最终输入预检在未初始化Go的新Shell中拒绝缺少go/gofmt。",
    "recoveryEvidence": "原工具输出保留；复用固定环境初始化后preflight-ready-final.json通过。",
    "permanentAction": "与说明解析共用本任务的显式环境初始化；继续由权威task-preflight检查工具，而非凭先前Shell状态推断。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/task-preflight/receipt-script-quoting",
    "position": "before-production-write",
    "count": 1,
    "impact": "仓外回执整理的内联脚本引号错误，handoff因缺少一一对应证据而拒绝。",
    "recoveryEvidence": "handoff.json和原工具语法错误保留；改用独立文件脚本后完成实际日志绑定，handoff-r3.json通过。",
    "permanentAction": "复杂数据处理改用独立仓外脚本；权威handoff字段和证据散列要求保持不变。",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/github-job-logs/formatted-output-incomplete",
    "position": "before-production-write",
    "count": 1,
    "impact": "GitHub CLI格式化日志未包含长JSON库存和扫描结果，整理器拒绝零条scanner凭据，未冒充通过。",
    "recoveryEvidence": "保留candidate-ci.log与失败输出；GitHub连接器原始job日志4588190字符恢复三份扫描凭据，保存candidate-ci-raw.log和散列。",
    "permanentAction": "本次复用现有GitHub连接器下载解码原始job日志；格式化日志仅作辅助，扫描结论按原始JSON和精确SHA核对。",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/l3-receipt/entry-field-misinterpretation",
    "position": "before-production-write",
    "count": 1,
    "impact": "首轮仓外回执复核误把entrySha256与外层JS入口比较而拒绝；原L3和工件本身没有失败。",
    "recoveryEvidence": "按权威github-hosted入口定义改为核对run-release-gate.sh，摘要43e57f4f匹配；三lane日志及ZIP摘要独立通过，l3-verifier-r2.log保留。",
    "permanentAction": "复用权威回执字段语义，仓外核验脚本只读取固定门禁文件，不改共享回执格式或放宽匹配条件。",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/publication-manifest/asset-field-misinterpretation",
    "position": "before-production-write",
    "count": 1,
    "impact": "首轮仓外附件复核脚本误按sha256字段读取实际digest字段，报告未获接受；公开附件本身与冻结清单一致。",
    "recoveryEvidence": "按真实清单结构读取带sha256前缀的digest后，15项API名称/大小/摘要和已下载provenance、正文全部匹配；publication-verifier-r2.log保留。",
    "permanentAction": "本轮辅助脚本按已读取的权威manifest字段修正，保持原文件、算法和精确匹配；不修改发布清单协议，不将执行修正宣称为产品或永久治理修复。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
