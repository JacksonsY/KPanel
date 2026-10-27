# KPanel v1.28.0-rc.9 发布验收记录

日期：2026-10-27（Asia/Shanghai）。发布级别：L3。产物已发布，生产未部署。

候选提交／标签：`534935794de50e1d11665627886ea2232b0c53f3` / `v1.28.0-rc.9`；`releaseChannel=preview`，`releaseTrain=1.28.0`。发布控制器为 `c1c1685c03053525aaf2d568f5e7689c18761ff3`。

公开 [Release](https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.9)：ID `425236090`，`draft=false`、`prerelease=true`，19 个附件。版本及预览镜像 index：`sha256:620aa6096f0f1018efd9e08b8619de55cf9f61a97e9adafee9b295e5484e1aca`。正式 Latest 仍为 `v1.27.2`，发布前 stable 快照为 `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`。上一预览回滚点 `v1.28.0-rc.8` / `sha256:218f9f15540d44d5a9b5ad6670ac7d4c1a63cc9c6ce52dce0810ddb0d7281e07`。

## 范围与来源

本次为文件传输和中继、桌面恢复、Windows 节点兼容、分享与监控安全修复、批量接入撤销及可选浏览器组件设置。Panel/Agent 需配套升级；慢消费者隔离需中心和节点双方支持。预览候选 `release/v1.28.0-candidate` 保留，以下来源 tip 均是发布提交的祖先：

| 来源分支 | 精确 tip | 处置 |
| --- | --- | --- |
| `fix/transfer-desktop-recovery-candidate` | `705348c5ca957f61fb2996de9bd5ed91f4b28449` | 已纳入，活跃引用保留 |
| `fix/windows-lifecycle-legacy-safe` | `02033f9ed7276d1274cc8b5cdf571261654052f2` | 已纳入，活跃引用保留 |
| `fix/file-relay-backpressure-20261027` | `ecb42e514529623c3f9de9b7dcbf4a46cd40f730` | 已纳入，活跃引用保留 |
| `fix/security-existing-leads-20261027` | `9e7c58ea3384215757d0da53e10331c3e6879643` | 已纳入，活跃引用保留 |
| `fix/batch-enrollment-removal-20261027` | `0678c4f45d0c76265e3953864e06292f30e1a4d1` | 已纳入，活跃引用保留 |
| `feature/browser-service-settings` | `8428c72ff89f6cd9a705c2b3f9c65dca3df6c6bc` | 已纳入，本地引用和工作树保留 |
| `fix/win-browser-tabs` | `98b02c1a7024fa9e41f27a4200393958e66cc9da` | 已纳入，本地引用和工作树保留 |
| `fix/rc9-preview-test-helpers-20261027` | `c1c1685c03053525aaf2d568f5e7689c18761ff3` | 发布前进入主线，活跃引用保留 |

来源引用尚未归档，物理工作树及证据均保留；未删除原引用。后续按项目 10.2 核对所有权和精确引用后处理。此前未完成的 RC9 本地组装不作为本版发布身份。

## 跨仓库联动与质量边界

`scriptLinkageState=not-required`（无需发布脚本，不适用）：内置脚本保持已公开的 `614d58cee228007012a2ec602cbdc8dbaeb7950d`，SHA-256 `ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9`；浏览器为 KPanel 原生可选组件，不引入新脚本协议。未发布脚本或部署生产。

| 维度 | 状态与证据边界 |
| --- | --- |
| 业务和互通 | 已验证（限定范围）：来源候选测试、最终候选与 L3；真实跨物理主机、全部旧节点组合仍未实机验证。旧版慢下载使用原中继方式。 |
| 安全与供应链 | 已验证（门禁范围）：组合安全范围工具记录 15 组，候选／主线 CI、L3 和公开镜像扫描通过；来源中未确认线索仍保留，不声称全产品无漏洞。 |
| 稳定和恢复 | 已验证（限定范围）：Windows SCM 安装、服务、更新和回滚测试；前置验收读取首败及有界重试保留，首败缺具体错误码，根因未完全确认。原 RC5 桌面恢复偶发未因本版局部修复被宣称全部关闭。 |
| 性能与资源 | 已实现未实机验证：中继和浏览器容器使用既有限额、超时与清理策略；未验证低配长期 RSS、真实多主机吞吐和长期 soak。 |
| 体验与可访问性 | 已验证（限定范围）：L3 前端 329 文件、3296 项通过、6 项跳过；公开浏览器／隔离容器四组 45、31、48、26 项检查通过。实体触屏、读屏和所有缩放未验证。 |
| 数据和兼容 | 已验证（限定范围）：删除主机撤销新旧接入尝试；升级前已删除主机缺撤销标记时不能与中断尝试区分。回退前备份配置、节点状态及在途任务；退出预览不自动降级。 |

此前来源浏览器重装后访问外网页面探测失败，错误细节不足以确认根因；本版添加的托管实际组件测试在候选 CI 与 L3 中完成安装、停止、启动、重启、卸载、重装，以及两次安装后的联网探测。该结果不等于所有网络拓扑或完整真实 Panel→Agent→浏览器旅程均已验证。公开测试使用声明的 Docker 库存和测试 CA 夹具；Windows OS 重启、原生 ARM64 和公开安装器全旅程未验证。

## 门禁和公开产物

候选 push CI [40470480144](https://github.com/kejilion/KPanel/actions/runs/40470480144)、预览发布 [40474123733](https://github.com/kejilion/KPanel/actions/runs/40474123733) attempt 1 成功：候选 CI、GitHub-hosted L3、主线／标签提升、主线 CI、Windows 附件、Release 和公开验收全部通过。L3 原始回执绑定候选与 tree、Go 1.27.3、Node 24.22.1、ubuntu24 runner 20261023.334.3；Web、Go、部署三组日志摘要回读一致。L3 浏览器组件测试包实际运行约 85 秒；公开验收绑定相同版本与镜像摘要。

公开验收核对 Release、19 个附件及双架构镜像、发布前 stable 快照与浏览器／隔离容器结果；不可将隔离环境解释为生产部署。安全扫描、SBOM 和 provenance 以工作流原件为准。此前业务现状新鲜度拦截、Windows 验收结果读取失败及被替代的 CI 运行均保留；Windows 有界重试后通过，不将首败抹除。完整流程异常总数未完成历史去重，保持未报告。

原始回执和公开验收文件保留在 `C:/GitHub/_release-evidence/kpanel-v128-rc9-full`，包括 `l3-40474123733`、`publication-40474123733`、`snapshot-40474123733`、`public-40474123733` 和 CI 首败日志。本版未新增永久工作流。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-27T08:07:05+08:00
- 候选冻结时间：2026-10-27T07:59:27Z
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
