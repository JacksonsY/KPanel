# KPanel v1.27.0-rc.9 发布验收记录

已验证：RC9 已公开发布，正式发布流程、精确候选 CI、L3 和公开镜像浏览器验收通过；生产未部署。

日期：2026-10-18。发布级别：L3；releaseChannel：preview；releaseTrain：1.27.0。

- 复核日期：`2026-10-18`
- 基线提交：`421b8bfe134277fdc8371af884a727cf00fcefca`
- 基线版本：`v1.26.0`

冻结产品提交：`a37ab4443990a05900dbba2d4e95c2802b5e23fd`。标签 `v1.27.0-rc.9` 是 annotated tag，tag object 为 `a6bf6e33c3b1e3622aae5d44845f13c1c9f38411`，peeled commit 与冻结提交一致。公开 Release 于 `2026-10-18T06:33:38Z` 发布。

上一稳定版及稳定回滚点：`v1.26.0` / `247ff67d98700c9af56d55a807cb9389fc8eedc1`，稳定镜像 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。上一预览版回滚点：`v1.27.0-rc.8` / `sha256:6202f24f70a1ee2bffa7aca8ff5e7461b94eb5f89ddb383aa30f38973ee88592`。

## 候选与分支处置

- `release/v1.27.0-candidate` / `a37ab4443990a05900dbba2d4e95c2802b5e23fd`：唯一预览列车候选。远端 `origin/main` 和 `origin/release/v1.27.0-candidate` 均核对为同一 SHA；RC 期间按项目 10.2 保留，不归档。
- 远端 tag 核验：`v1.27.0-rc.9` 的 tag object 为 `a6bf6e33c3b1e3622aae5d44845f13c1c9f38411`，产品 commit 为 `a37ab4443990a05900dbba2d4e95c2802b5e23fd`；GitHub API 读回 `draft=false`、`prerelease=true`。
- 来源分支 `feature/files-path-copy` / `2b66f55870d9104735cd99bc4c13301b4bcb94b4`：已纳入 RC9，精确 tip 是产品提交祖先。该分支没有对应的远端 feature ref，本地跟踪 `origin/main` 且落后两次主线提交；worktree `C:\GitHub\_codex-tasks\kpanel-files-path-copy` 干净。原功能任务已交付，但未找到明确释放分支所有权的声明；因此保留分支和 worktree，不创建归档 ref、不解除 upstream。由本发布负责人在原作者明确释放所有权后、最迟下一次稳定候选筛选时复核。
- 候选 worktree `C:\GitHub\_release-candidates\kpanel-v127-rc1` 保持在 RC9 精确 tip 且干净。本验收记录自身使用独立治理分支；文档候选/主线 CI、归档 ref 与最终 SHA 记录在证据根的 `docs-closeout.json`，不将文档提交混入产品 tag。

## 发布画像

- 业务域：Files 文件管理器的路径复制交互。
- 变更面：只读展示与前端交互；点击当前路径后聚焦路径输入框并全选，用户可用系统复制键复制完整路径；离开编辑时恢复面包屑，上级目录仍可导航；工具栏在窄窗口换行。
- 用户旅程：打开目录 → 点击路径 → 全选完整路径 → Ctrl+C → 粘贴到其他输入框；Escape、Enter、失焦和父目录跳转仍可用。
- 未变化契约：无 API、数据库、数据格式、端口、Compose、Agent 权限、宿主机写入、`kejilion.sh` 或应用市场协议变化。
- 风险等级 L3：按预览发布流程进行完整候选、L3、主线、公开产物和浏览器验收；业务改动本身为受限前端交互。

## 发布范围与未纳入内容

相对 RC8 基线仅纳入文件路径交互、响应式工具栏和本次 RC9 版本/扫描身份元数据。精确提交顺序：

1. `694e8c82ef87052b31769c56705a3016bf9bcb88` — feat(files): select full directory path for copying
2. `08ea94d7b3642ba9e68004fa6226c79421f7d207` — chore: correct recorded OCR tool version
3. `1301b9b7c73dd96c2ba2c80752cb2894422b397e` — fix(files): retain path input width in stacked toolbars
4. `4b2276b518ae686d7cfb2fd8bb18ed2e1ed175ce` — fix(files): wrap toolbar to keep the directory path usable
5. `2b66f55870d9104735cd99bc4c13301b4bcb94b4` — style(files): align path input with shared form styling
6. `12f21d071e30e9ca097e299c58d05648b43fbb8e` — release: assemble v1.27.0-rc.9 preview candidate
7. `a37ab4443990a05900dbba2d4e95c2802b5e23fd` — chore(security): rebind RC9 source inventory

最终候选改动 8 个文件，100 insertions / 13 deletions。第一版候选的扫描源指纹在 Go 版本从 RC8 更新至 RC9 后过期；CI 按策略拒绝后，在最终候选重绑扫描库存，产品 UI 代码未再改变。未纳入其他分支、跨仓库脚本改动、稳定提升或生产部署。

## 外部审计与修复交付

- 安全盲审补漏：本候选没有触及 `.governance/security-audit/boundary-policy.json` 的 `securityReview.paths`，故本候选不触发 candidate 档盲审；本记录不据此宣称全面安全，也未执行 CF scoped/full 审计。
- OCR 行审：工具版本 1.12.11；代码观察区间为候选代码提交范围 `421b8bf..47ad861`，5/5 文件覆盖，3 个代码路径 / 80 行，free-form=0、有效 H/M/L=0/0/0、constrained-only=0。最终候选相对已审代码仅变更 `scripts/torrent-backport-scan-scope.json` 的扫描身份元数据。
- `node scripts/security-review-scope.mjs --base v1.26.0` 累计输出 35 个审查分组，其中 17 recorded、18 missing；输出含 `invalid_trailer=true`。本次未列出 stale 分组，故不单独判定 stale 数为零。该稳定基线遗留覆盖不由 RC9 的 UI 差异引入；下一稳定发布前由 KPanel release maintainer 按 5.4 review 档补漏并逐项处置，未核实线索不作为漏洞或安全通过结论。
- 修复交付状态：路径复制和窄屏工具栏源码及 RC9 产物已交付；尚未进入稳定版，也未部署生产。
- OCR 与安全范围的原始摘要保存在 `C:\GitHub\_release-evidence\v1.27.0-rc.9\ocr` 及证据根内；详细 L3 / CI 原件见后续证据索引。

## 跨仓库联动判定

- `scriptLinkageState`：`not-required`。
- 变更集、脚本候选：不适用；无脚本仓库提交，亦未改变应用市场依赖。
- 镜像标签核对的内置脚本基线：commit `160924a498d9b8d071fed72f165d23c1b9e886de`，SHA-256 `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`。
- 发布决定：仅发布 KPanel 预览候选；脚本与应用市场契约保持不变。

## 多维质量结论

| 维度 | 状态 | 证据 | 未验证风险或不适用依据 |
| --- | --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 文件路径输入、复制、失焦恢复、父级导航与精确 SHA 浏览器验收；候选 Linux CI 与 L3 通过 | 本次无 Agent/API 交互；没有新增跨节点协议 |
| 网络入侵与供应链安全 | 已验证 | 精确候选依赖 freshness、安全通告任务、公开镜像 digest、附件和 release workflow 验收通过 | 本候选未触及安全盲审边界；稳定基线仍有 18 个 missing 审查分组，不代表全仓无漏洞 |
| 稳定性、失败恢复与兼容 | 已验证 | 公开镜像 E2E、146 项非 mock 浏览器检查、失败的 systemd dispatch 闭合检查通过 | 真实宿主机安装和生产恢复未执行 |
| 性能与资源预算 | 不适用 | 本候选只改变文件路径 UI 呈现与 CSS 布局 | 未进行专门性能压测；没有新算法或后台资源 |
| 用户体验与可访问性 | 已验证 | 路径全选/复制、键盘退出、窄屏布局及 320/390/1280 视口检查 | 本地 200% 浏览器缩放、读屏和实体手机未验证 |
| 数据、配置与迁移 | 不适用 | 没有数据库、配置或持久化格式变化 | 无迁移步骤 |

## 自动门禁

- 定向与本地检查：来源任务最终验证覆盖 309 个文件 / 2,999 项通过，构建通过；精确发布候选版本一致性和 source-check 为 21/21，writer 检查和 `task-preflight ready/handoff` 通过。来源任务的一个独立 FilesView 定向命令有 152 项断言通过，但进程因测试 teardown 的异步组件加载错误退出 1；原日志保留，不能把该命令写成全通过。最终精确候选 Linux CI 和 L3 通过。
- `make verify-release`：本地 Windows 环境没有可用 Go 命令，未完成本机 release-note 独立检查；正式 GitHub Release gate 的 `Validate release notes` 已通过。未把本机环境限制写成产品通过。
- L3 外层入口：run `39102152731` / attempt 1，GitHub-hosted `ubuntu24` image `20261012.334.1`；Go 1.27.2、Node 24.21.0、Docker 28.0.4；2026-10-18 05:54:51Z 至 06:09:38Z，status=passed。entry SHA-256 `c286f7f2ad692ea17b8ec0537fe2a84e8d049d22adec63a2b00ceba39b165d57`；L3 result SHA-256 `49ed059b89751bac7f0d4bdcca4801b46d9df1f1e8dfa0e544a77dfc34119949`，identityUnchanged=true，web/go/deploy 三 lane 均通过。该工作流回执未提供不可变 Runner ID；记录已披露实际 runner image/version。
- 候选 CI：最终精确提交 run `39101767116` success。首次 `12f21d0` 的 run `39100789367` 在安全源指纹门禁失败；不是流程异常，未发布。最终指纹提交后候选 CI 通过。
- 主线 CI：预览工作流 `39102152731` 的 `main-ci / verify` job `117603129580` 对最终产品提交通过。
- Release workflow：run `39102152731` attempt 1 success；candidate CI、L3、promote、main CI、publish、public acceptance、preview completion 均成功。嵌套的 `publish / l3` 按外层 L3 已通过而跳过，不表示 L3 缺失。
- 发布说明：最终正文 2 条摘要、3 条升级提示；版本与公开 digest 匹配，Release note gate 通过。
- 安全扫描与镜像契约：依赖 freshness run `39101878568` 的 report 与 security-advisories 均成功；公开镜像 `image_e2e=pass`；两架构标签、config、组件清单和公开产物身份由 Release / public acceptance 校验。

## 依赖与技术栈变化

- 依赖 freshness：run `39101878568` 于 2026-10-18T05:49:31Z—05:51:26Z 执行，report 与 security-advisories 均成功；没有因本候选新增依赖、升级工具链、Action 或基础镜像。
- 版本与锁文件变化只同步应用版本为 RC9；Go module、npm 依赖版本、Action SHA 与基础镜像 digest 未升级。镜像报告中的 Go / Node / Alpine base digest沿用发布输入。
- 本版未新增暂缓/拒绝依赖候选、例外期限或行动项；EOL 复核没有在本记录中单独完成，不以 freshness 成功代替 EOL 审查。
- 扫描源指纹更新是候选源码身份重绑，不是依赖升级。该提交修正被首次候选 CI 捕获，最终候选重新资格化。

## 隔离真机与浏览器验收

- 本地环境：Windows 10 / Chrome；exact candidate `a37ab4443990a05900dbba2d4e95c2802b5e23fd` 的 mock preview 仍在运行，入口 `http://127.0.0.1:4179/files?path=%2Fhome`，浏览器证据 `preview-final/browser-acceptance.json`。
- 本地验收通过：页面 HTTP 200；点击当前路径后只读输入框聚焦并全选 `/home`（selectionStart=0、selectionEnd=5）；Escape 退出后焦点回到导航；点击父级跳转至根目录并刷新列表。视口 1159×884，浏览器 warning/error 为 0。
- 公开隔离验收：精确 RC9 镜像 index digest；published 45、upgrade 31、initial 44、restart 26，共 146 项，均 `passed`、`mocked=false`、errors=[]。公开升级旅程含稳定版来源、RC 预览摘要、opt-in 不自动安装、320/390/1280 无横向溢出和无 systemd 时 dispatch 返回 503 且清空失败队列。4 份 report 与截图保存在外部 `public-acceptance-39102152731`。
- 测试 runner 通过隔离容器、测试 CA 与受控 Docker inventory fixture 验收；这些是自动化 fixture，不是生产主机。没有进行长时间 soak。
- 视口、键盘、焦点和失败态已验证；本地 200% browser zoom、读屏、实际异机升级和生产主机写入未执行。

## 发布产物与公开仓库复核

- [GitHub Release v1.27.0-rc.9](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.9)：`draft=false`、`prerelease=true`，不是 Latest；15 个附件，`SHA256SUMS` 含 11 条。
- Docker 预览 tag `preview` index：`sha256:ae42c6e900b65bee7a07b048e57d00bccddbbe1e70b00a98d3e5d064f303f915`。
- `linux/amd64`：`sha256:8d92c5a7b07f1d5b98b2df14330d3f02b6e41969f6d9f614390b04a2c1281caa`；`linux/arm64`：`sha256:41b9cd890873ce422923e2bf0c027715407ccef124c9d6a2a55fc70c0821b541`。
- GitHub 最新稳定 Release 仍是 `v1.26.0`；Docker stable `latest` snapshot 仍为 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。这是本次公开读回时点状态。
- public acceptance 核对了官方预览版本、唯一 digest、Release URL、非空正文与升级提示；组件清单、附件摘要和双架构镜像身份匹配。镜像 E2E 通过。
- `kejilion/apps` 与 `kejilion.sh` 契约：无改动，版本标签继续携带已发布脚本基线，不要求配套脚本发布。

## 自更新通道验收

- 稳定来源只返回 GitHub Latest `v1.26.0`；预览来源返回公开 RC9 及固定 digest，稳定和预览身份相符。
- 加入预览为显式 opt-in；报告确认 opt-in 不启用自动安装，也不请求安装。
- 自动安装保持关闭；无 systemd 的单次 dispatch 失败关闭，返回 503 并清理排队任务。成功安装与自动计划执行没有在真实宿主机上测试。
- 默认状态为 stable 且 auto install off；restart 报告通过。退出预览时低版本不降级场景未在本记录中单独确认。
- OpenRC 与轻量 Node 的既有限制及显示边界未在本候选中修改；未通过生产主机验证。

## 生产部署安全核对

- 本版本为 preview，生产部署不适用且禁止；本任务仅获预览发布授权，没有生产部署授权。
- 不存在验证/灰度或正式部署目标。环境策略中 `prod-108` 禁止全部 KPanel 操作；确认本次未连接、未备份、未部署、未升级、未核对。
- 生产写操作：0。隔离 runner 和 mock preview 结果不冒充生产证据。

## 回滚

- 源码/tag：RC9 `a37ab4443990a05900dbba2d4e95c2802b5e23fd` / `v1.27.0-rc.9`；上一预览回滚标签 `v1.27.0-rc.8`。
- 镜像回滚：RC8 `sha256:6202f24f70a1ee2bffa7aca8ff5e7461b94eb5f89ddb383aa30f38973ee88592`；稳定回滚输入 `v1.26.0` / `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。
- 本候选没有数据或配置迁移；标准做法是先备份，再按精确 digest 回切并复核 Panel/Agent。没有执行实际生产回滚。
- GitHub Latest 与 Docker stable latest 仍指向 v1.26.0；默认更新通道保持 stable。未更改公共默认通道。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-18T12:16:59+08:00
- 候选冻结时间：2026-10-18T13:43:59+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

候选 CI 首轮失败及恢复时间、拦截点见“自动门禁”；该候选问题在生产写入前被门禁拦截，不计为生产变更失败或重复公开发布。

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：1
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

### 流程异常明细

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "local-preflight/role-flag/missing-required-role",
    "position": "before-production-write",
    "count": 1,
    "impact": "一次本地候选 writer 检查漏传必需的 --role 参数而非零退出，阻断本次本地 ready 证据；不涉及产品行为或生产写入。",
    "recoveryEvidence": "candidate-local-final.log 保留缺参调用失败；随后显式传入 --role writer 的 candidate-local-check.log 与 candidate-local-qualified.log 通过，最终 task-preflight ready 和 handoff 均通过。",
    "permanentAction": "本发布负责人已把 --role writer 纳入本候选固定检查参数并保留校验输出；下一发布继续复用该显式参数并检查 writer 身份与终态后再生成 ready/handoff 收据。根因是调用漏参，不修改校验器或放宽校验。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

### 遗留风险与后续准入

- 本地资源回收：没有删除唯一原件。RC9 候选分支与 worktree 按预览策略保留；来源功能分支因所有权未明确释放而保留；正在运行的本地预览继续供用户查看。完整 L3、公开验收和首轮失败证据保存在 `C:\GitHub\_release-evidence\v1.27.0-rc.9`，本轮未做资源清理。
- 未验证风险：本地 200% zoom、读屏、真实宿主机升级和生产恢复未验证；稳定基线有 18 个待补安全审查分组并报告 invalid_trailer=true；EOL 未单独复核；source branch 归档待所有权释放。
- 已实现待实机准入：路径复制与响应式工具栏已通过预览浏览器和公开自动验收；实际宿主文件列表与真实用户长期使用仍待后续实机反馈。
- 不阻断本版的理由：本次功能未改服务端、安全边界、数据、配置或部署；正式候选、L3、主线 CI、公开 Release 和隔离浏览器验收均通过。稳定推广须先处理上述审查覆盖待办。
- 后续准入：稳定候选前复核安全审查 missing 组，确认或保留来源分支的明确处置；有实机用户反馈时补 200% zoom 与辅助技术验收。
