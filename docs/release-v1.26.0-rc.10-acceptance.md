# KPanel v1.26.0-rc.10 发布验收记录

日期：2026-10-12。发布级别：L3。

候选提交 / 标签：`e889add5d28827d93b8d1f26310b5fe98021c93c` / `v1.26.0-rc.10`。annotated tag object `f0bb8dcf9ab90574507f48e04084c3d26c51af53` peeled 到同一提交。

上一稳定版本 / 回滚点：`v1.25.1`，Docker `latest` digest `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。上一预览 `v1.26.0-rc.9` / `c9e5063f028db9dc6efedc9c89e2c5f8cac8a855` / `sha256:7e2cd7cb16340ba3b473d1cb3d857269e2ca2962d4d3ddf6a95caed1bb284e19`。

`releaseChannel`：`preview`

`releaseTrain`：`1.26.0`

候选分支与发布后处置：`release/v1.26.0-candidate` / 预览版保留，产品 tip 为 `e889add5`；验收文档另行交付，不改写公开产品 tag。

## 发布画像与范围

批准 main 基线 `03650f985c4cf28b10c7949f91438c3af7c634b5`。新增用户指南、桌面系统图标隐藏及 autofs 磁盘采样修复；继承 RC9 已发布的系统中心 systemd 服务管理，不将旧来源分支再次当作未发布功能。无数据库 schema、Agent 权限、端口、Compose、脚本协议或应用市场默认更新入口变化。发布级别 L3，范围为公开预览产物，禁止生产部署。

| 来源分支 | 原始精确 tip | 纳入依据与处置 |
| --- | --- | --- |
| `feature/settings-user-guides` | `a951aef2660fafd365d91ae9e47781bdbe3238d8` | 合入原始 `46093860`、`c4ef4c5d` 和审计记录；原提交为产品祖先；来源预览/工作树保留 |
| `feature/desktop-system-icon-removal` | `d50fd55e2c12d1b0c60b09e4ca55996fcef23e15` | 合入 `fd528688` / `d50fd55e`，系统开始菜单不显示“隐藏”徽章；原提交为产品祖先；来源预览保留 |
| `fix/issue-27-autofs-statfs` | `0f6d47c0c8b389bb60cdb99b76864b42b88d3c09` | 原修复重放为 `cf122a16`；原分支保留，不能用不同 SHA 冒充原提交已 merge |
| `feature/systemd-services` | `10dcb1d5502a8b899bbdf644f9759cf74e79ebcd` | 主体已在 RC9 发布；原始分支未统一 rebase，独立来源工作树保留待所有权释放 |
| `feature/systemd-services-backend` | `33c11668e6f8152c29823736ce2ac82e4812dddd` | `git cherry` 表明实现补丁已在主线；来源工作树保留 |

- 组合修正：`d4974d3a` 修正指南注册中过时的 `removable` 字段和旧布局测试；`30acece9` 修正三语指南的系统图标文案；`45b6af9c` 绑定最终 source/closure 身份；`e889add5` 更新过期业务事实记录。无新增功能搭车进入冻结候选。
- 旧来源 `claude/share-theme-immersive`、`feature/latency-median-band`、`feature/office-light`、`feature/panel-login-notification`、`fix/system-packages-retry` 的独有补丁均与主线等价，归为已交付待本地处置。备份路径分支的实现/复核补丁已在主线，未完成审计文档保留本地；两个 login 审计记录分支不是新产品候选。
- 旧 `fix/offers-desktop-icon` tip `e3261e8af5952add5e2ff4c9ecc7512fcf43c22c` 与 RC9 资产一致，已推送并复核 `archive/fix/offers-desktop-icon`，删除原本地分支及干净工作树。旧文件快捷方式/分页/文件布局候选不重新纳入。
- 未确认释放所有权、正在运行预览或含未知修改的共享工作树保留，不自动 rebase、stash 或删除。保留不代表还有功能未发布；下一次所有权交接或发布生命周期边界再复核资源。

## 外部审计与修复交付

- 指南 scoped 审计原件因编号冲突从来源 run-34 原样导入 `.governance/security-audit/run-35`，9 个文件字节与来源一致；main 的未完成 systemd run-34 保留不覆盖。导入说明在 `imported-record.json`，不是重新审计。
- 指南登记审阅提交 `c4ef4c5d82589ed8bf94dcce5c1c107c75be65da` 为产品祖先；source-only，run complete、scope_complete=false、5 个覆盖单元、0 findings / needs_validation。15 次调用均为 `gpt-6-luna` / `max`。不将单个来源的完成记录写成整个 RC10 审计通过。
- 覆盖检查：`decision=scoped-required`，85 个待审提交；计数范围 86 个、覆盖 1 个。新边界包 `internal/bittorrent`、`internal/offers` 与既有未决项仍需补审；预览策略只记录，不据此提升稳定版。
- RC9 systemd run-34 incomplete；主机组成员/token ACL 条件风险未独立实机核实，不能写成漏洞已确认或已排除。真实服务写操作也未验证。
- 扫描身份：first-party 901 个 Go 文件，SHA-256 `286e7b4ba0d677bb237b2f2cd9d3b89e6091080b8bada41ce42b31fdf798b78c`；closure `7d8cb6312aab83626856dbfc60ba4d2728d71ab6a0d46fd025680f913a2965d3`。保留独立身份复核，不以身份复核代替安全审计。
- OCR 1.12.11，集成范围 `7246fcb..30acece`，31 个 reviewable 文件，8 个共享变更直接检查、23 个精确来源 blob 继承，处置覆盖 31/31，另 29 个排除项；free-form 原始问题与复核原件保留，有效 H0/M0/L3，constrained-only=0。`e889add5` 仅文档，skipped 理由已写 trailer；不宣称 OCR 覆盖全部 release 历史。

## 跨仓库联动判定

- `scriptLinkageState`：`not-required`（无需发布脚本）。变更集、脚本候选提交/摘要：不适用。
- 实际内置脚本基线 `1800d955f216aebd2776674368a8e469a671c489`，SHA-256 `5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c`。
- SSH fetch 后 apps main `6602ac36e0c329e35b74ff621721b23583e94f5f`；`kpanel.conf` 与候选 blob 均为 `9d2d2efdc334db1ba014203370fdc8281f1a5870`，无 apps/sh 写入。默认 stable / latest 不改变。

## 多维质量结论

| 维度 | 状态 | 证据与限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已实现未实机验证 | 完整 L3 与集成 Mock；未核验真实视频元数据提供方、真实 autofs 挂载和 systemd 写操作 |
| 网络入侵与供应链安全 | 已实现未实机验证 | 自动扫描按登记闭包/backport 策略通过；standalone inventory 保留已审阅条目，不表示上游依赖无漏洞；CF 全局补审未完成 |
| 稳定性、失败恢复与兼容 | 已实现未实机验证 | race、更新备份及恢复隔离夹具通过；无新增真实宿主升级/失败注入/回滚证据 |
| 性能与资源预算 | 不适用 | 本版未建立性能改善结论，既有 GPU/WAN/依赖图性能准入限制保留 |
| 用户体验与可访问性 | 已实现未实机验证 | 指南、主题、窄屏、图标恢复 Mock 已核对；截图、125%/200% 缩放、实体手机及屏幕阅读器未验证 |
| 数据、配置与迁移 | 已实现未实机验证 | 无 schema 迁移；生命周期隔离夹具通过；无生产升级/恢复验收 |

## 自动门禁与历次尝试

- 最终外层 L3 `v1.26.0-rc.10-e889add-l3-r4`：`arena-154`，06:54:04Z–07:17:48Z，passed / exit 0，23 分 44 秒；source checks 1,010.664 秒，identity_unchanged=true。固定 Runner `kpanel-go127-security-runner:go1.27.2-node24.21.0` / `sha256:20667a9f1fad6590219ba8d8acdabc09a49f8683d4a877b847df17d28b83f8fb`。
- plan SHA-256 `ef23ed599c23295db5460a8212f7ba3ad63e5417f97ba636c0c8c67e63337266`；remote script `21c08b11be3526a0fe766aa606a81d0e4047e8a4e89d79227da4e62914164979`；bundle `6bc57505cf348ce490934d72520f07fe5be12c048ab3abe2451e6eb332bb06a4`。12 份回收证据摘要无差异。
- Web 279 文件 / 2,656 用例通过、6 跳过；typecheck/build 通过；Panel/Auth/Docker race 分别 228.888 / 8.413 / 65.185 秒。源码、镜像和 lifecycle 门禁全部执行；最终 `app_conf_update_backup_parity=pass`。
- 候选 CI `38216999999`、依赖检查 `38216999993`；主线 CI `38218413787`、依赖检查 `38218413708`：全部成功，均绑定产品 SHA。
- 首轮 `51caf6e-l3-r1` 因旧 source identity 失败；`7246fcb-l3-r2` 源码/扫描/镜像通过，但最终更新夹具受磁盘不足拦截；同 SHA 隔离 lifecycle 重跑通过，不据此冒充整轮通过；`45b6af9-l3-r3` 在启动前因业务事实记录过期失败。原件保留，修正身份/业务记录并回收再生资源后，最终精确候选完整 r4 通过。
- Release workflow 与 tag 依赖检查：Release `38219539803` 成功；tag 依赖检查 `38219539791` 成功。。

## 浏览器与专项验收

- 集成 Mock `http://127.0.0.1:4179/settings/guides`，证据 `C:/GitHub/_release-evidence/v1.26.0-rc.10/assembly-preview-final`，manifest 绑定 `45b6af9c`；产品仅追加业务文档，`preview-doc-only-qualification.json` 核对界面/后端/版本源文件无差异。
- 中文 9 张 Bilibili 卡片、本地海报加载；英文 5 个 YouTube 链接及 `hl=en`；1149/390/1280 视口无横向溢出，最小 compact 标签 12px、正文/动作 14px；浅/深色 DOM 状态检查；控制台 JS errors `[]`。
- 隐藏 Overview 系统图标后桌面按钮消失，开始菜单入口保留且无隐藏徽章；打开窗口并从布局管理恢复后桌面按钮重新出现。只改 Mock 状态，没有宿主或 Provider 写入。
- IAB 截图两次 CDP 超时、独立 Chrome tab 创建不可用，未生成可交付截图；125%/200% 缩放未实际生效，明确未验证。不能用 DOM 状态替代像素或缩放验收。

## 发布产物与公开仓库复核

- [公开 Release](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.10)：2026-10-12T07:57:23Z 公开，draft=false / prerelease=true；正文非空且 version/digest 与公开镜像一致。GitHub Latest 仍 v1.25.1。
- 版本与 preview OCI index 均为 `sha256:66ff8546cbeaa4f0d415393f8a6e0fe4794f9e082098354ef6934790352e9f85`。
- linux/amd64 `sha256:c688db7bce4be9f7a90166555295f2fef9892cccf9c6abe309fafdd6fdc21a0b`；linux/arm64 `sha256:d2d95e572b3c44d906fb077dcbf7edd9b0851d0c502f39684c784a635a270a3a`；两者 OCI version/source revision 均为 1.26.0-rc.10 / e889add5。ARM 只核验构建和身份，不宣称原生运行通过。
- Docker latest 仍 `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。apps/sh 契约无需写入。
- 15 个附件均流式下载校验：11 个发行文件与 SHA256SUMS 匹配，其余与 GitHub asset digest 匹配；包含 Linux Agent/轻量节点、MCP 各平台、metadata、modified-component provenance 和许可记录。下载校验不等于各原生平台执行通过。
- `image_e2e=pass`：arena-154 显式拉取公开 rc.10 tag 后，运行仓库 canonical `packaging/tests/image-e2e.sh`；脚本 SHA-256 `1378218f9d4ac0fdd82d66ac502c5f7e0d82a13fca8d60429079936ed527edf4`，所需两张 public asset 与产品源文件摘要匹配。临时容器/自动网络/数据由脚本清理，复查无残留。最初本地 WSL 有一层传输过慢，中止拉取，未开始 E2E；只把 arena 终态计为通过，没有更换镜像或修改测试脚本。
- 外部证据根目录 `C:/GitHub/_release-evidence/v1.26.0-rc.10`：`public-proof.json`、`public-asset-checksums.json`、`arena-public-image-e2e.log`；原 WSL 拉取记录仍在 `public-image-e2e.log`。公开 registry JSON 首次解析未得到 platform 时停止，改用结构化 JSON 读取后双架构标签核对通过；该次解析失败不写成镜像缺失。
- live 面板发布信息接口和更新弹窗未验证，公开正文核对不能代替界面验收。

## 自更新通道验收

继承受管 stable/preview 来源、加入预览只切换检查、退出不自动降级及失败恢复契约；本版无新增 live 更新弹窗、自动安装、OpenRC/轻量 Node 真实旅程验收，不以 Mock 或单元测试写成已实机通过。公开 Release 正文和 registry 的核对不等于面板更新弹窗已验证。

## 生产部署安全核对与回滚

生产部署、升级、备份、生产恢复与公网核对：不适用（预览版禁止生产部署）。验证目标使用登记 `arena-154`；本地 WSL 镜像拉取未完成即中止，不计通过；`prod-108`/`108` 禁用全部 KPanel 操作，本次未连接。产物状态以公开复核章节为准，不把隔离夹具写成生产上线。

源码/镜像回滚点使用上述 RC9 或 v1.25.1；需要部署时按既有应用市场备份和回滚入口执行，本轮未执行生产回滚。GitHub Latest、Docker latest 和标准默认入口保持 v1.25.1。

## 资源回收

验证机深清先删除 30 个无容器引用的旧 KPanel digest 镜像，再回收闲置构建/Go/npm 缓存；发布后删除已完成 r4 构建副本、release Go 缓存与旧 Trivy 缓存。两阶段实测分别回收 7,033,798,656 / 3,237,687,296 字节，本轮累计 10,271,485,952 字节（9.566 GiB），清理结束时可用 12.908 GiB，公开镜像 E2E 拉取后复测可用 12.817 GiB；此前任务还回收过 1,498,238,976 字节。运行服务、数据卷、备份、固定 Runner、当前/上一稳定及预览回滚资源和审计/失败原件保留。本地批量依赖目录清理两次被自动审批拒绝，未绕过；仅移除本任务已完成、clean 且无进程使用的 scan-identity 工作树及分支，实测回收 60,125,184 字节，原 tip 7246fcb5 可从公开 RC10 tag 恢复。其余活动/未知所有权工作树保留；旧 RC9 Mock 已停止，当前集成 Mock 保留。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-12T11:22:37+08:00
- 候选冻结时间：2026-10-12T14:52:36+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（RC10；无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：未记录
- 其中生产写操作开始后异常次数：未记录
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[]
<!-- kpanel-release-process-incidents:end -->

验收文档首轮 CI `38222130076` 因多余 EOF 空行失败，公开产品和镜像不受影响；移除空行后对新文档提交重跑 CI。已知 L3、浏览器提供方及本地清理拦截逐项如上保留；未系统登记所有工具试错的统一计数，因此不推断为零。

## 遗留风险与后续准入

稳定版前完成要求的 CF 全局补审；验证真实视频元数据、autofs/服务管理边界与失败恢复；补齐截图、缩放、辅助技术和既有性能准入。来源分支后续归档必须先释放所有权；未知/活动内容保持原样。生产写入仍需单独授权。
