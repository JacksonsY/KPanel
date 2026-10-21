# KPanel v1.27.1 稳定版发布验收记录

日期：2026-10-22。发布级别：L3。`releaseChannel=stable`，`releaseTrain=1.27.1`。

**产物已发布，生产未部署。** [公开 Release](https://github.com/kejilion/KPanel/releases/tag/v1.27.1) ID419899509，2026-10-21T18:20:52Z公开，draft=false、prerelease=false、GitHub Latest=v1.27.1。原始发布 [39646965427](https://github.com/kejilion/KPanel/actions/runs/39646965427) attempt1全部必需作业成功，没有重跑或执行恢复入口。

精确候选 `ee73500f9511006aad79ef38af7bb9a4dad60a5a`，tree `15dcb36940f309d311b765d232e3bfa3438d300e`；annotated tag对象 `373b21781f04f04d599ce0027d891d9b2cc65e85`。用户授权候选整合、推送主线及稳定发布，未授权生产部署。

## 发布范围与来源

稳定日志重新归纳 `1.27.0-rc.1` 至 `rc.23` 的累计更新：本地应用、OneDrive备份、XP/98桌面外观、移动终端、图库、后台批量终端、文件编辑器、systemd独立挂载安装器和安全边界维护。各RC完整条目继续保留。精确产品范围 `v1.26.0..v1.27.1`，涉及展示、只读和授权宿主机操作，风险L3。

同时纳入用户指定的月度维护候选 `b078b88a02c8c0cb7cab04938970ef5d04299a5b`：Node24.22.1、DOMPurify3.4.17、Vite8.3.5、Vitest5.0.5及官方场景重打包。`deefd94b`保留原提交历史合入；源文件及生成场景与原候选核对，锁文件只另更新本次1.27.1版本元数据。Go1.27.3和既有供应链基线继续使用。

本次修正附件冻结的真实法律文件布局、草稿可见查询和精确Release ID绑定，并保留稳定公开验收及受限恢复边界。`docs/release-v1.27.0-acceptance.md`真实记录上一列车失败，已随产品候选进入主线，解除月度维护缺少该记录的门禁缺口。原1.27.0标签、draft、版本镜像和失败证据保持原状；新补丁独立发布。

## 独立复核、OCR与安全边界

完整组合候选经非作者只读Codex复核：a778135c为PASS WITH FOLLOW-UP，最终ee73500f为相同tree。实现/复核均为codex，其他提供商没有可用非交互入口，明确采用隔离只读复核的同提供商回退。正常稳定发布没有阻断项，恢复权限和收尾限制见下文。

OCR1.12.11在 `7ddeec2..a778135` 区间自由臂先行，33个变更文件中22个可评审文件全部覆盖，H0/M0/L0、free-form=0、constrained-only=unreported。4份不支持的Markdown人工核对；6份生成产物与原维护候选绑定，锁文件按解析图核对。其余适用周期不据此推断工具有效性。

稳定周期安全补漏复用仓外原始报告：相对v1.26.0，44包组中34 recorded、10 missing，`invalid_trailer=true`；四个轻量source-only报告中cluster仅部分覆盖，余下范围未补成已审。原始H0/M2/L0，经不同代理静态核实仍为两项既有资源配额中风险；cluster/terminal维护者跟进仓外待办#45/#46，期限2026-11-04，备份导入既有中风险继续由backup维护者跟进。未公开未修复利用细节，未宣称全仓安全审计通过。本次没有执行CF scoped/full审计。

## 自动门禁与工件完整性

- 同一ee73500f候选CI [39642777266](https://github.com/kejilion/KPanel/actions/runs/39642777266) attempt1、主线CI [39644976592](https://github.com/kejilion/KPanel/actions/runs/39644976592) attempt1均success；原生工作流解析通过。候选/主线freshness39642777467/39644976587均success，必需来源9/9，可选CF未请求。
- 本地定向回归：发布22/22、源码范围21/21；runner pin、治理一致性、依赖策略、失败验收记录、覆盖、YAML上下文及空白检查通过。Windows本地Node24.17.0与云端24.22.1分别记录，未混用工具版本证据。
- 原始L3 job119384971241 success：GitHub-hosted Ubuntu24，Runner image20261012.334.1，Go1.27.3、Node24.22.1、Docker28.0.4；入口 `scripts/run-release-gate.sh` SHA-256 `43e57f4fd321846204020d6cf27eb75961f0aefa1284a9187e786601704c2d40`。完整 `make verify-release`、web/go/deploy三lane、生命周期和 `image_e2e=pass`；source tree与候选一致，identityUnchanged=true。前端314文件、3096项通过、6项跳过，三个原始lane日志摘要已独立复算。
- L3工件12368273994，ZIP SHA-256 `aec280c38bc7266121a5dfd5c6e51bc00f0a187850c8c01e5e759920002169ff`，下载字节数、API摘要、ZIP路径及解压内容均核验。
- release job119393207168 success：源码、Go调用路径、npm、Trivy源码/镜像、安装生命周期、运行时契约、双架构构建、modified-component及最终正文全部通过。2026-10-21T18:20:44Z完整15附件身份冻结成功，snapshot上传、通道提升及GitHub公开依次成功，原法律文件/草稿查询缺口本次正常路径已实际验证。
- snapshot工件12369207796 SHA-256 `a195bf344b1736c9bdae74b6e1986ca389bf92d25ba837c6df533954865e677a`；publication工件12369887359 SHA-256 `5022bd1f3cd55ae9277c61dfa11b485b2394731e441ea10cce7355d09f819708`。两份原件的快照、身份、附件清单字节相同；冻结Release ID419899509。
- 公开正文SHA-256 `5ba38fe4300b80f2dc6d10831d6d6cd590fc2e6a8cca7d79bc5b351afd1b9e3e`，与原始最终正文一致；附件清单SHA-256 `3b9d5105203667165e6e0af44b807e54e8f07a66b0fa3c843df46db4ceb318e5`。15个公开附件全部独立下载并按API大小/摘要核对，11项SHA256SUMS全部匹配。
- public-acceptance job119402199965 success；public工件12369827374 SHA-256 `7b6bab451b0f736f6df71cf148dc2c7a84ae74a4100c0ad9d21510e2d51d4f27`。stable-completion job119403114388 success，远端归档复核通过。

## 公开镜像、说明和双端契约

`docker.io/kjlion/kejilion-panel:1.27.1` 与 `latest` 的OCI index均为 `sha256:be8a8e16a484d536094e335307424a39968f321fa0a7091b5d2630aade36a23c`。GitHub-hosted公开验证实际读取registry并核对版本、来源SHA、双架构、许可证和受管脚本；本机另核对Release随附的13份OCI原始记录摘要，不将本机直连registry未完成说成成功。

- linux/amd64：`sha256:9fe12536f52d0c4a284a8a6c73d2d4d71364aca95536ccacaf5b6ed78d47f7eb`
- linux/arm64：`sha256:0e77a9209397632c54e750e5318b7ffd55f502b65c455f1d1aa165536f0f6efe`
- preview保持v1.27.0-rc.23 / `sha256:144cf771a1043dc584b342cd2cef15e3f0b9280a632706b0c1c04576a1ac0dd7`，未提升到稳定镜像。

公开正文有10条主要变更、3条升级提示；面板真实发布信息接口按既有上限返回8条摘要、3条升级提示，版本/digest匹配。systemd和月度维护升级均位于实际返回摘要内。旧RC完整更新仍可从Changelog读取。

`scriptLinkageState=not-required`，无需发布脚本（不适用）；跨仓变更集/脚本候选不适用。内置 `kejilion.sh` commit `160924a498d9b8d071fed72f165d23c1b9e886de`，SHA-256 `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`；应用配置生命周期、许可证和镜像标签契约实际通过。

## 隔离浏览器、自更新与验证边界

环境策略为 `github-hosted` 临时隔离容器，唯一权威入口仍为 `release-public-verify.yml`。测试Runner ID `sha256:2264eddd967f6d143bf546bfb090d336e804033f39b6fa774ebee27b1f4c00f6`，控制器和产品均ee73500f，Runner配方原件/overlay SHA-256均 `e2a026415568924488a35e53e5529aae40e2d57f99640d0a743444316f8a3926`，没有用后续main覆盖产品源树。

四份报告均passed、mocked=false、errors=[]：initial44项、restart26项、published47项、upgrade35项；逐份摘要和版本/digest绑定已独立核验。它们是各报告断言数，不视为互不重叠的独立测试总数。真实Panel/Agent执行与受控Docker inventory、测试CA夹具并存，未宣称整台真实宿主机环境。

验证了真实API、本地应用和图标、偏好持久化、TLS/命令历史/桌面重启恢复、隔离PTY输出、公开版本说明及旧RC23后端发现1.27.1稳定候选。选择预览不启用自动安装，拒绝加入保留stable；默认stable且自动安装关闭。无systemd的立即安装返回503、清理排队状态，随后保持stable/未自动安装；这仅是受控失败路径，不证明宿主机更新、备份和回滚已实演。

1280/390/320宽度无横向溢出的自动断言通过。320截图人工确认更新弹窗、目标1.27.1、更新内容和升级提示实际显示；1280截图当时未显弹窗，桌面弹窗的版本/摘要仅有自动DOM断言证据，不声称该截图已完成视觉验收。截图时刻差异原因未确认，由发布测试维护者在下一次浏览器取证修改时跟进，复核日2026-10-29。本次不放宽门禁或清除原件。100%/125%/200%原生缩放、全部主题、读屏、实体手机、Safari和长时间soak未验；本次重点为公开产物身份和关键发布旅程，未据短时隔离验证推断这些结论。

## 供应链与多维质量

六平台actual/original闭包按Go1.27.3重算，第一方1007文件；sourceManifest `3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580`，firstPartySource `ff0cdd4eb981e0547941980126422c6fd56d98e6cf9537d22f6c834b40c62946`，closure `f5b0d0988c5f7d37ed9cd9cee453c22586efa5d9d1bed42240599df5517eee07`。云端候选源码扫描的身份与本地canonical inventory一致。

扫描器packageCount为production110/selected465/standalone150，原始exit为0/1/1，保留finding为0/10/27；后两项是按原有独立复核处置保留的完整清单，不是零告警。37项与盲审44包组口径不同。观察日期2026-10-11、处置到期2026-10-25未延长；每日安全审计39565808978为既有RC22证据，本次实际候选/发布扫描另有同SHA原件，未冒称该每日作业运行于本候选。EOL2026-07-28复核有效至2026-10-28。

月度维护之外的新鲜度信号继续由基座维护任务按dependency-policy分级处理；本记录不编造逐项首次检测时间和处置期限，9/9完整来源报告原件保留。

| 维度 | 状态 | 证据及限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 同SHA CI/L3及真实公开产物四报告；真实微软云端和混版本实体节点未验 |
| 网络入侵与供应链安全 | 已验证 | 规范扫描、inventory、SBOM/provenance、独立复核；保留中风险及未审范围，无全仓安全结论 |
| 稳定性、失败恢复与兼容 | 已实现未实机验证 | 隔离重启、受控失败和安装生命周期通过；实体systemd/OpenRC升级回滚及发布恢复未实演 |
| 性能与资源预算 | 已实现未实机验证 | 有界实现与既有RC证据保留，无真实硬件吞吐/FPS/长期soak结论 |
| 用户体验与可访问性 | 已验证 | 公开浏览器、接口与移动弹窗；桌面截图差异、原生缩放及实体辅助技术未验 |
| 数据、配置与迁移 | 已验证 | 本地持久化及重启通过，无数据库迁移；OneDrive真实账户往返和备份恢复未验 |

受限发布恢复只接受原始精确run/attempt、candidate/version/digest及“promote成功、publish失败”，本次未执行。目标tag工作流与后续默认main不兼容时，GITHUB_TOKEN可能受Workflows权限限制，应失败关闭；不增通用凭据。恢复公开后还需按现有归档入口人工完成精确候选归档，恢复工作流本身不负责归档。后续真正使用恢复入口前，由发布维护者核对这些限制和原始工件，不宣称已完成真实故障注入或任意main恢复。

## 候选归档、资源与后续准入

`release/v1.27.1-candidate` 原tip ee73500f已自动保存为 `archive/release/v1.27.1-candidate` / ee73500f，原远端引用已移除，重新读取远端确认。正式tag保留同一精确提交。产品候选/主线CI与稳定公开验收成功后才归档。

月度维护b078b88a已纳入稳定版，原作者 `fix/security-dependencies-20261021` 及其工作树留原任务维护，不修改或清理。失败列车 `release/v1.27.0-candidate` /7ddeec2e保留故障与恢复证据，不称其稳定发布成功。`fix/v1271-stable-assets` 本地发布工作树作为当前稳定回溯保留；旧发布排错与验收工作树、仓外原始日志/工件一并保留。本轮没有资源删除，净释放0字节，不把远端归档视为本地已回收。

本验收记录在真实公开验收后形成纯文档候选；记录候选/主线CI、归档精确SHA和远端复核以其交付证据为准。本次产品tag始终固定ee73500f，后续验收记录提交不重写tag。记录必须进入main，不能仅留文档分支而再次造成后继维护缺少验收记录。

证据根 `C:/GitHub/_release-evidence/v1.27.1-stable`；原1.27.0失败原件根 `C:/GitHub/_release-evidence/v1.27.0-stable-recovery`。保留已知失败及工具不可用记录；未取得actionlint可执行版本，不称其通过。流程历史总数不完整，两项机器指标均“未记录”；原1.27.0法律文件布局故障及原run见其独立失败记录，不改写历史。

## 生产与回滚

生产未部署。生产目标、备份、灰度和生产写操作均不适用；prod-108未连接、未核对、未备份、未部署、未升级。公开隔离容器不替代实体服务器证据。真实OneDrive、实体systemd/OpenRC、轻量Node系统集成、手机/Safari及大图库弱网留待专项准入。自签名TLS端口仍接受明文HTTP API，需限制可信网络并强制HTTPS。

上一稳定/回滚点 `v1.26.0` / `247ff67d98700c9af56d55a807cb9389fc8eedc1`，镜像 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。本次没有生产配置/数据备份或回滚演练；实际回滚前须按对应生命周期备份数据、核对版本兼容并验收Panel/Agent。公共默认入口已指向1.27.1，未执行公共通道回退；preview仍为RC23。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-15T14:30:29+08:00
- 候选冻结时间：2026-10-22T01:18:22+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：未记录
- 其中生产写操作开始后异常次数：未记录
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[]
<!-- kpanel-release-process-incidents:end -->
