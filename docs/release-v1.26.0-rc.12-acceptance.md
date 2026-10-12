# KPanel v1.26.0-rc.12 发布验收记录

日期：2026-10-12。发布级别：L3。

候选提交 / 标签：`a6b8d2aac975e4708d216b989d8714f92f871083` / `v1.26.0-rc.12`。annotated tag object `fc37320c245c04344b4efa8c45aaffb696ff2837` peeled 到同一提交。

上一稳定版本 / 回滚点：`v1.25.1`，Docker `latest` digest `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。上一预览 `v1.26.0-rc.10` / `e889add5d28827d93b8d1f26310b5fe98021c93c` / `sha256:66ff8546cbeaa4f0d415393f8a6e0fe4794f9e082098354ef6934790352e9f85`。

`releaseChannel`：`preview`

`releaseTrain`：`1.26.0`

候选分支与发布后处置：`release/v1.26.0-candidate` / 预览版保留；验收文档独立交付，不改公开产品 tag。

## 发布画像与范围

批准 main 基线 `7ee53c50b8745e2954242f943016e48d7438129b`。五组功能先基于 `b79d47fe` 集成为 RC11，但其 Release 两次失败，未公开 Release 或镜像；失败标签保留不移动。本版新增 test-only 回收同步并升 RC12，整合五组 RC10 后的新候选：经典壁纸顶栏、概览操作按钮、移动监控图标、指南路由与桌面位置、图库窄窗口。展示与桌面偏好键校验变更；不涉及数据库 schema、Agent 权限、端口、Compose、脚本协议或应用市场默认更新入口。L3 用于公开预览产物；生产未部署。

| 来源分支 | 原始精确 tip | 内容与处置 |
| --- | --- | --- |
| `codex/classic-topbar-sidebar-sync` | `23c6b635e5314af03e894d4e6076c7e23532bc0b` | 减少透明度时顶栏静态填充与侧栏比例对齐并关闭实时模糊；高对比度维持实色；来源工作树保留 |
| `codex/overview-toolbar-button-size` | `93b4699644671c5c1926753a7842c85e67d65bbc` | 概览两项操作恢复标准 40px 按钮；来源工作树保留 |
| `codex/overview-monitor-icon-mobile` | `3a7b2bef786473758c7ddd04cee20c4d6726257b` | 实时监控图标固定 36px flex basis，防止窄屏压缩；来源工作树保留 |
| `fix/desktop-icon-layout-persistence` | `d67b1a5bd8c6f0e6345c0c3cb12e717995ef9b13` | 有界多段偏好键、独立 `/guides` 与旧地址跳转、设置与指南同窗口往返；来源工作树保留 |
| `fix/gallery-window-layout-20261012` | `e6810cfdf4f1ab397ed3c525bb5c7e424f0a7277` | ≤640px 桌面容器内封面及操作区单列排版；来源工作树保留 |

五组原始 tip 均经 RC11 组装的 merge 保留为 RC12 产品祖先，没有改写来源历史。来源未统一 rebase，但整合候选建立在上述批准 main 基线上。作者工作树所有权未释放，未知/活动预览内容保留；保留来源分支不表示功能仍未发布。

- 原组装版本与边界回归 `62ac1509`、扫描身份 `017958f4`、OCR `7ee53c50`；RC12 测试/版本 `6ca1d4cd`、扫描身份 `14c14e52`、OCR `a6b8d2aa`。偏好键每段 1–64 字符、整体最多 256 字节，禁止空段、遍历、查询、尾斜线及大写；它不是文件系统路径或授权规则。
- systemd 服务管理继承 RC9，RC10 指南、系统图标隐藏和 autofs 采样过滤保持。旧快捷方式图标、分页、文件页布局不重新引入。旧已交付来源按既有 RC10 处置，不把原始未 rebase 历史当成新功能。
- login 审计记录及 backup 路径分支的未完成审计原件不是新产品候选；冻结后新增功能不进入本版。

## 外部审计与修复交付

- 本版未启动新的 CF scoped/full 审计；offers 仅修复测试回收，生产 `Service.Close` 仍只取消而不等待，不宣称生产生命周期风险已全部修复；有界偏好键不增加权限入口或关键授权不变量，提交记录 deferred 理由。覆盖检查 `decision=scoped-required`、88 个待审提交；既有 BT/offers/终端及 run-34 systemd 未决项继续开放，预览只记录，不提升稳定版。
- RC10 指南 run-35 原件只覆盖其登记的来源与五个单元；本版不扩大该记录的覆盖声明。真实 systemd 写操作、主机组成员/token ACL 条件风险继续未核验。
- first-party 901 个 Go 文件，SHA-256 `497138842eb4bc58d755b4a00ff8191897467e788477bb3c4b33dcb95e4dfd2c`；closure `7d8cb6312aab83626856dbfc60ba4d2728d71ab6a0d46fd025680f913a2965d3`。新增源码内容和测试未变更 imports、文件清单、build tags、依赖声明或构建目标；登记闭包沿用已通过 RC11 L3 的六行，完整 L3 复核实际身份后才计为通过。
- OCR 1.12.11，集成区间 `b79d47f..017958f`，31 文件中 27 reviewable / 4 排除，27/27 直接审阅，free-form 先行；有效 H0/M0/L1，缺少偏好键长度回归已补齐，constrained-only=0。原始观察、规则、覆盖和资格化证据在外部目录，不公开内部临时上下文。

## 跨仓库联动判定

- `scriptLinkageState`：`not-required`（无需发布脚本（不适用））。变更集、脚本候选：不适用。
- 实际内置脚本基线 `1800d955f216aebd2776674368a8e469a671c489`，SHA-256 `5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c`。本版差异不改变脚本动作、安装/更新路径或镜像内脚本。
- apps SSH fetch main `6602ac36e0c329e35b74ff621721b23583e94f5f`；`kpanel.conf` 与候选 blob 均为 `9d2d2efdc334db1ba014203370fdc8281f1a5870`，无需 apps/sh 写入。

## 多维质量结论

| 维度 | 状态 | 证据与限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已实现未实机验证 | 自动门禁和集成 Mock；无新增真实宿主偏好保存或服务写操作验收 |
| 网络入侵与供应链安全 | 已实现未实机验证 | 登记闭包/backport 扫描策略；全局 CF 补审未完成，不表示全部上游依赖无漏洞 |
| 稳定性、失败恢复与兼容 | 已实现未实机验证 | 有界偏好键回归、旧 URL 跳转及隔离更新恢复夹具；无生产恢复演练 |
| 性能与资源预算 | 不适用 | 本版无性能改善结论；既有 GPU/WAN/依赖图性能限制保留 |
| 用户体验与可访问性 | 已实现未实机验证 | 390px DOM、截图和同窗口交互；OS 媒体偏好、125%/200% 缩放、实体手机及读屏未验证 |
| 数据、配置与迁移 | 已实现未实机验证 | 无 schema 迁移，旧指南地址重定向；无真实宿主升级证据 |

- RC12 OCR 1.12.11 增量 `7ee53c5..14c14e5`：7 文件中 5 reviewable / 2 排除，5/5 直接审阅，自由臂先行，无新增 findings、H0/M0/L0、constrained-only=0。原五组功能的上述 27/27 记录继续有效，未重新计作新发现。

## 自动门禁

- 完整 L3 `v1.26.0-rc.12-a6b8d2a-l3-r1`：`arena-154`，12:04:34Z–12:27:39Z，passed / exit 0，23 分 05 秒；source checks 984.239 秒、identity_unchanged=true。固定 Runner `kpanel-go127-security-runner:go1.27.2-node24.21.0` / `sha256:20667a9f1fad6590219ba8d8acdabc09a49f8683d4a877b847df17d28b83f8fb`。
- plan SHA-256 `71367884888be5fee0046bc8e204f676791fbea18ad3accaf1baf1a82f581c0e`；remote script `21c08b11be3526a0fe766aa606a81d0e4047e8a4e89d79227da4e62914164979`；bundle `b80bb33392faf0f45b67ccc4a5effaf538b342c82986cc79c19aedf2fa0fba39`。12 份回收证据摘要一致。
- Web 279 文件 / 2,658 通过、6 跳过，typecheck/build 通过；Panel/Auth/Docker race 223.173 / 8.610 / 63.958 秒；BT 定向 race 通过。源码/镜像扫描及契约、更新备份和失败恢复隔离夹具全部执行，最终 `app_conf_update_backup_parity=pass`。本轮 L3 首次通过，未跳过测试。生产闭包 106 外部模块、HIGH/CRITICAL 0；selected 460 模块保留 10 个漏洞，standalone 150 模块保留 24 个漏洞和 2 个配置项，实际 scanner exit 1 按已登记完整库存保留，不写成全图无漏洞。逐项策略有效至 2026-10-25，未知/新增/过期/生产交集或身份变化须重审。
- RC12 候选 CI `38249331422`、依赖检查 `38249331406` 均 success，绑定 `a6b8d2aa`。RC12 主线 CI `38250393603`（attempt 2）、依赖检查 `38250393572`（attempt 2）均 success，绑定同一产品 SHA。Release `38253559540` attempt 1 success；标签依赖检查 `38253559703` success，均绑定同一产品 SHA。

- 定向 race：两次失败场景各 30 次（135.544 秒），offers 全包 3 次（104.597 秒），均 exit 0。实际源码为 `6ca1d4cd`；至冻结 `a6b8d2aa` 的 cmd/internal 无差异，source hash 同为 `49713884...`，资格化证据保留。
- 测试夹具在取消刷新后持锁捕获 in-flight 完成通道，等到刷新及文件 pruning 结束才让 TempDir 清理；5 秒失败上限保留，不用睡眠碰运气、不跳过断言。4 个 Open 入口全部注册 cleanup，按 LIFO 先于临时目录清理执行；生产功能逻辑未修改，另有版本元数据更新。
- RC11 原候选 `7ee53c50` 的完整 L3 与候选/主线 CI 已成功；其 Release `38244200965` attempt 1 / job `114793929427` 在 `TestStaleCacheRefreshesInBackground` 的 TempDir 回收失败，attempt 2 / job `114796715577` 在 `TestViewHonoursPublishingWindow` 同类失败。两次都是 `directory not empty`，源码身份均未变化，web/deploy lane 通过；原始失败日志完整保留。停止重跑并修复测试，未移动或删除 `v1.26.0-rc.11` / tag object `a698b9be4d306ebd25e96de472932acac4907352`，RC11 未公开 Release/镜像。
- RC11 重跑连接器 403 后由本机现有 GitHub CLI 登录提交；不把该重跑写成成功。所有失败和修正均保留至 RC12 验收。RC12 主线依赖检查 `38250393572` 首轮 job `114814871375` 因 sum.golang.org 校验树请求 HTTP/2 INTERNAL_ERROR 退出 2（go-modules 数据缺口，非无升级候选），原日志保留，已提交同 SHA 失败作业重跑；重跑 attempt 2 已 success，检测源重新完整返回（不是忽略首轮错误）。
- RC12 主线 CI `38250393603` attempt 1 / job `114814871698` 的变更核验和核心 race 已通过；Go call-path 扫描本体 affected/imported 均 0，后续 original-version 基线读取 proxy.golang.org 上 `github.com/anacrolix/torrent@v1.61.0.mod` 时 502，作业退出 2。保留原日志，已提交同 SHA 完整失败作业重跑；重跑 attempt 2 已 success，完整核验全部执行。未改代理、关闭校验或跳过门禁。
- RC12 发布说明草稿检查使用固定 Runner：`TestRenderedReleaseNotesForPublication` exit 0；4 条修复、1 条升级提示。最终正文以 RC12 Release workflow 结果为准。
## 依赖与技术栈变化

无依赖、工具链、Action、扫描器、基础镜像或受管脚本升级。沿用固定 Go 1.27.2 / Node 24.21.0 Runner 与登记扫描策略，依赖维护期限和退出条件继承当前政策；不将 package 版本字段变化算作依赖升级。没有新性能基准或全依赖图无漏洞声明。

## 隔离真机与浏览器验收

- `arena-154` 执行 candidate-validation 和公开镜像隔离 E2E；禁止连接 prod-108/108，本版无生产用途。
- Mock acceptance `http://127.0.0.1:4176`，新 manifest 绑定 `a6b8d2aa`；390px 无横向溢出，监控图标 36×36，概览两按钮 40px / 14px。桌面 `desktop-window-1` 在设置→指南→设置各步保持一个窗口；`/settings/guides` 跳转 `/guides`。
- 390px 图库封面 338px，内容和操作 304px 单列，操作区完整包含在封面内；经典氛围浅/深色及图库、概览共 4 PNG 已实际查看。RC11 四张截图和交互记录按 web/src、web/public 及视觉/桌面领域文档无差异进行资格化复用，原件仍在 RC11 外部目录；RC12 新建预览并保存一张概览加载确认，已实际查看。窗口/导航操作仅改变 Mock，不证明真实 Go 偏好写入通过。
- 减少透明度、高对比度和无 blur 支持只覆盖静态 CSS/单元测试，未操纵 OS 媒体偏好；125%/200% 浏览器缩放、实体手机、原生 ARM、Safari/Firefox、读屏及长时 soak 未执行。Mock 既有 MCP/passkey 缺失 endpoint 不作为本版产品失败或通过证据。

## 发布产物与公开仓库复核

- GitHub [v1.26.0-rc.12](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.12) 于 2026-10-12T13:24:19Z 公开；draft=false、prerelease=true，正文版本与镜像 digest 一致。
- Docker `1.26.0-rc.12` 与 `preview` index 同为 `sha256:57de86879881985d166bbc68165d2a669bf709458f496c06c305709e949d5643`。amd64 `sha256:1483ba3d4a4b8e86f36bcf1049101760007c6081e03fecb1a456d55942b7488f`；arm64 `sha256:a610a4791c26992e608fdf3bda156c92e6e7feea83f946794e82ee03667c5f80`。两平台配置的版本与 revision 均匹配 RC12 / `a6b8d2aa`；未执行原生 ARM 运行旅程。
- 15 个公开附件全部实际流式下载，SHA-256 与 GitHub asset digest 一致，其中 11 个 SHA256SUMS 条目一致；Agent/Node 双架构、MCP 六平台、元数据及许可文件均存在。
- `arena-154` 从 DockerHub 拉取公开 RC12（非本地验证镜像）后，以 `KPANEL_EXPECTED_VERSION=1.26.0-rc.12` 执行 canonical `packaging/tests/image-e2e.sh`；`image_e2e=pass` / exit 0，2026-10-12T13:25:40Z 完成。健康/版本、Host/proxy 与真实静态文件字节匹配通过；临时容器和网络清理后查询为空。
- GitHub Latest 仍 `v1.25.1`，Docker `latest` 仍上述稳定 digest。原始公开校验、逐附件摘要及 E2E 日志保留在外部证据根目录。任务 handoff 对精确候选、参数及原始日志摘要资格化通过；先前缺少收据的提前检查不作门禁通过证据。

## 自更新通道验收

受管 stable/preview 来源、加入预览只切换检查、退出不自动降级及失败恢复契约继承；本版无新增 live 更新弹窗、自动安装、OpenRC/轻量 Node 真实旅程。不以 Mock 或公开 Release 正文代替这些界面的实机验收。

## 生产部署安全核对与回滚

产物已发布后，生产仍未部署。没有生产变更前后状态、备份恢复点或生产回滚演练；不连接 prod-108/108。公开预览需要回退时选择 RC10 不可变 tag/digest，恢复用户环境须另获部署授权；本版不触碰稳定 `Latest` / `latest`。

## 分支与本地资源处置

- 唯一预览候选 `release/v1.26.0-candidate` 保留在产品 `a6b8d2aa`。验收文档在独立 `docs/release-v1.26.0-rc.12-acceptance` 交付；候选和主线文档 CI 均成功后，按 expected-SHA lease 保存到 `archive/docs/release-v1.26.0-rc.12-acceptance` 并移除活跃文档分支，最终引用与 CI 记录在外部收尾回执。本记录提交时尚未宣称该后续步骤完成。
- 五组来源 tip 均已纳入本版，但作者所有权未释放，保留其分支及工作树，不清除活动预览或未知改动。原始 tip 与路径见来源处置证据。
- 本轮已删除自有 RC11 L3 source 495,661,642 字节，以及 RC12 L3 source 495,665,233 字节和 targeted fixture source 110,272,104 字节，合计 1,101,598,979 字节（约 1.03 GiB 的已核对对象占用）。删除前 realpath、精确 HEAD/源码包摘要、无残留 Runner 容器及证据回收摘要均已检查；该合计不作为文件系统净空闲增量测量。原始 L3 证据、恢复 bundle、固定 Runner、服务数据和卷保留。
- 本地 acceptance Mock `http://127.0.0.1:4176` 保留供用户验收；manifest 仍绑定公开产品提交，验收文档不改变运行源码。未重试此前被自动审核拒绝的批量 node_modules 清理。

## 证据与限制

外部证据根目录 `C:/GitHub/_release-evidence/v1.26.0-rc.12`（RC11 失败和原始界面证据保留于相邻 rc.11 目录），包括 candidate contract、OCR、source identity、L3、browser acceptance、公开产物校验和公开镜像 E2E。验收文档自身 CI 与归档终态由后续外部回执记录，不能提前写成已通过。未新增知识或工作流。
## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：未记录
- 候选冻结时间：2026-10-12T19:59:41+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（RC12；RC11 失败标签保留，未公开产物；无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：未记录
- 其中生产写操作开始后异常次数：未记录
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[]
<!-- kpanel-release-process-incidents:end -->

上述“失败”指标针对生产变更；RC11 测试、RC12 上游网络重试和交付资格化缺少收据的提前检查均已保留原始记录，不推断为零。尚未系统登记全部工具试错计数。

## 遗留风险与后续准入

稳定版前完成所需 CF 补审、真实 systemd 写操作与主机组成员/token ACL 条件验证；补齐 OS 媒体偏好、缩放及既有性能准入。作者分支须释放所有权后再归档；生产部署另行授权。
