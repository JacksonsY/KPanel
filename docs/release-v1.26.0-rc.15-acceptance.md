# KPanel v1.26.0-rc.15 发布验收记录

发布级别：L3。releaseChannel=preview，releaseTrain=1.26.0。

批准的 main 基线为 77341dba755ddf931931ebef67d55373a6765f59；冻结产品提交为 5aebde092020b7d5bb9bc3d3adda96d1c3a682a9。注释标签 v1.26.0-rc.15 的 tag object 为 8c929ec115939fd83d524559878be0ac9f89552f，指向上述提交。Release 于 2026-10-13T00:45:38Z（08:45:38+08:00）公开，prerelease=true、draft=false、isLatest=false。Release 工作流 [38327504136](https://github.com/kejilion/KPanel/actions/runs/38327504136) 通过。

发布时，远端 main 和 release/v1.26.0-candidate 都精确指向 5aebde092020b7d5bb9bc3d3adda96d1c3a682a9。预览版候选归档步骤按策略跳过，candidate 分支保留在冻结 SHA。稳定版仍为 v1.25.1，GitHub Latest 未变；Docker latest 仍指向 sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325。本次生产部署为 0。

## 候选分支处置

| 分支 | tip / 内容 | 处置 |
| --- | --- | --- |
| feature/terminal-split-workspace | 378ebe1e35e95b4cbebfca6c2184857e67ab6bce；终端布局菜单、窗格顺序及活动会话同步 | 已纳入 RC15。 |
| fix/issue-27-autofs-statfs | 0f6d47c0c8b389bb60cdb99b76864b42b88d3c09；跳过 autofs 挂载采样 | 已在 RC13 发布；RC15 无新增差异。 |
| fix/system-packages-retry | c1fe7b755ccce8882edb711448c7186d982c0672；恢复系统包读取及有限重试 | 已在 RC13 发布；RC15 无新增差异。 |
| feature/systemd-services / feature/systemd-services-backend | 10dcb1d5502a8b899bbdf644f9759cf74e79ebcd / 33c11668e6f8152c29823736ce2ac82e4812dddd | 系统中心 systemd 服务管理及后端已随 RC9 上线；已核对 RC9 标签树中的界面、API、类型、模拟数据和测试，本次不重复集成。 |
| feat/offers-posters / claude/offers-column | 4884b5afa78a311b76dc243986292a58ddba3ff8 / db81b33c7cddf90f61c991f6a275587fc48b4674 | 两个 tip 均已包含在当前 main；RC14 到 RC15 的 offers 页面和 internal/offers 没有差异。offers 不是本次新功能。 |
| fix/backup-archive-data-path | 96d97075a449f7681c383f400094b2b1643ac930 | 产品补丁 0c28072a6f6cd04eff2653344012311105ebf9f2 已有等价补丁进入 main；未对齐内容为未完成的 run-17 审计记录，不作为本次产品候选。 |
| docs/security-audit-login-20261008 / docs/security-audit-login-validation-20261008 | 2c314d79358ccadf0042596b8b443ee74a9049c3 / fa5a54b9c4f275e85712908575b89efd840c0310 | 仅安全审计记录；验证延续材料未并入产品发布，本次未运行新的 Cloudflare scoped/full 审计。 |

旧桌面文件快捷方式图标、分页和文件页布局继续按用户要求归档；没有重新纳入本候选。候选之间未产生额外产品 API、数据库 schema 或 Agent/Node 协议变更。

本版相对 RC14 的产品变更只有终端布局交互：用图标菜单选择布局，修正窗格顺序；新建终端填入空窗格并聚焦新会话；窄布局变化后，活动会话和快捷命令目标同步到第一个可见窗格。现有 PTY 会话不会因切换布局而关闭，无需迁移配置。

## 审阅与验证

- 独立候选代码审阅 r3 未发现确认问题。Go 第一方源差异只包含版本常量从 1.26.0-rc.14-dev 更新为 1.26.0-rc.15-dev；功能差异是终端 UI。OCR 覆盖 12 个变更代码路径、249 行；自由审阅和规则复核均无有效发现。
- 候选 CI [38325222376](https://github.com/kejilion/KPanel/actions/runs/38325222376) 与依赖新鲜度 [38325222418](https://github.com/kejilion/KPanel/actions/runs/38325222418) 通过。相同 SHA 快进 main 后，主线 CI [38326428278](https://github.com/kejilion/KPanel/actions/runs/38326428278) 与依赖新鲜度 [38326428277](https://github.com/kejilion/KPanel/actions/runs/38326428277) 通过。标签依赖新鲜度 [38327504104](https://github.com/kejilion/KPanel/actions/runs/38327504104) 通过。
- 固定 Runner arena-154 的 L3 run v1.26.0-rc.15-5aebde0-l3-r1 通过，exit 0。候选 SHA 为 5aebde092020b7d5bb9bc3d3adda96d1c3a682a9；Runner 镜像为 kpanel-go127-security-runner:go1.27.2-node24.21.0，ID 为 sha256:20667a9f1fad6590219ba8d8acdabc09a49f8683d4a877b847df17d28b83f8fb。状态和 12 项 evidence SHA-256 清单已核验。
- L3 的 Go 全量测试、race、go vet、前端类型检查与 Vitest、双架构构建、生命周期、更新与备份回滚夹具均通过。govulncheck 没有可达漏洞，另有 1 项 require-only 信号；npm audit 为 0。Trivy production closure 无保留发现；selected scope 保留 10 项漏洞，standalone scope 保留 24 项漏洞及 2 项配置发现，这些范围不能表述为全依赖图零风险。
- 没有执行新的 Cloudflare scoped/full 审计。安全覆盖记录为 scoped-required，列出的包边界包含 internal/bittorrent 和 internal/offers；稳定版提升前仍须完成既有审计准入。未把该覆盖状态描述为安全审计全绿。
- Release 工作流的源码、依赖、配置扫描、应用生命周期、二进制/镜像构建与运行契约检查、provenance、Release notes、镜像 channel 提升及 GitHub 发布步骤均通过；候选归档步骤按预览条件跳过。
- GitHub Release 共 15 个附件；API digest 与下载文件 SHA-256、长度逐项匹配，15/15。SHA256SUMS 中 11 项逐项匹配，11/11。
- Docker Hub 的 1.26.0-rc.15 和 preview 同指 OCI index sha256:b871bddd99328cf3bbb05a6c8cbaa2a755789d2ecbfd6714cc9e3142aa1baccd。amd64 manifest 为 sha256:8e241fc901020a5021f5f0285ba5298c48d0eafbc5dd913d01b43a230b68a0ab；arm64 manifest 为 sha256:8a93071b87d36d370a991452d0a0f34cbfad51ea6a595fa6f872e59a11cae86e。Registry 配置 blob 中两个平台的 version/revision 均与 1.26.0-rc.15 和冻结 SHA 相符。latest 摘要与 RC14 记录的稳定摘要一致。
- arena-154 按公开不可变 index digest 运行 packaging/tests/image-e2e.sh，image_e2e=pass、exit 0。测试后临时容器、网络及 18080 端口均清零；远端测试 staging 文件已删除。该测试证明公开镜像隔离启动、健康、代理头、静态资源和 bootstrap 契约，不证明真实宿主 PTY 或生产升级。
- 本地 Mock 预览 http://127.0.0.1:4183 绑定冻结 SHA，覆盖终端布局、焦点与快捷命令流程。手工检查受 Mock 终端会话上限影响，未手工看到第四个可见会话；390px 视口未手工核验。Mock 不执行真实主机操作。系统中心 systemd 页面此前以 Mock 查看状态与日志；真实 systemd 写操作未测试。

Windows 本机 source-check 不能代替 Linux 门禁：本机未配置 Go，且第三方 torrent 配置的行尾摘要检查在 Windows 失败；没有修改第三方文件。Linux 候选/main CI 和固定 Runner L3 均通过。真实 Linux PTY、真实服务管理写入、实体手机、读屏、生产升级/回滚均未由本次验证证明。

## 发布证据

证据根目录：C:/GitHub/_release-evidence/v1.26.0-rc.15/，包含候选审阅、OCR、L3 原件、附件核验、公开镜像 metadata 和 image E2E 日志。GitHub Release：[v1.26.0-rc.15](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.15)。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-13T07:07:40+08:00
- 候选冻结时间：2026-10-13T08:29:29+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：0
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[]
<!-- kpanel-release-process-incidents:end -->
