# KPanel v1.28.0-rc.1 预览版发布验收记录

日期：2026-10-22。发布级别：L3。`releaseChannel=preview`，`releaseTrain=1.28.0`。

**产物已发布，生产未部署。** [v1.28.0-rc.1](https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.1) ID `420835387`，2026-10-22T14:41:30Z 公开，draft=false、prerelease=true，15 个附件。Docker `preview` 指向本版；GitHub Latest / Docker `latest` 保持 v1.27.2。

产品候选 `85fe4b815e9a07f2c390ee49c2ba69f4f2a72fef`，tree `263ccb5383be7ad738a5941c92cfa5bbf7cd4a22`，annotated tag 对象 `1b196304ca317e1ac1022e9c690c3419054b5c45`。用户确认接管本地候选继续发布，授权本轮 SSH 推送、main、tag、Release 与镜像；不含生产部署。本文为发布后的事实记录，不改变产品身份。

[Publish preview 39790232673](https://github.com/kejilion/KPanel/actions/runs/39790232673) attempt1 的 L3、候选 CI、主线集成/CI、Release 发布成功；首次公开验收在入口校验处失败，原运行保留 failure。随后 [Verify published release 39798207805](https://github.com/kejilion/KPanel/actions/runs/39798207805) attempt1 以原 publication run/attempt 和冻结快照只读恢复，公开产物与真实浏览器验收成功。没有重建、覆盖版本镜像或重发 Release。

## 范围、业务画像与来源

业务真源仍是宿主文件、现有 Panel/Agent API 与认证契约。变更覆盖文件只读下载、既有归档宿主写入边界、前端布局与启动资源；风险 L3，归档目录身份涉及特权文件操作。没有增加端口、数据结构、迁移、Agent 权限、应用市场协议或生产部署动作。

| 本地来源 | 纳入提交 | 行为 |
| --- | --- | --- |
| `fix/desktop-archive-download` @ `6feef3a5559966ce40c6d702ade8245b3089c58a` | `29f1e8fa`、`cf4082f4`、`667d88e9`、`6feef3a5` | 桌面压缩包与快捷方式统一浏览；原包、单成员及多选 ZIP 流式下载；复用目录窗口并关闭过期内容 |
| `fix/primary-page-window-layout` @ `f245cbb7bade704b2fd1a0b478eb568f7e9d1ed4` | `b522fb91`、`f245cbb7`，包含上一组 | 概览/主要页面工具栏按窗口宽度换行；表格滚动与定位限制在自身区域；窄窗口按钮可读 |
| `fix/panel-memory-budget-20261022` @ `2a956b28f48010b45bf7f0a3b56c470421f02104` | `61142c4d`、`13e3e4fb`、`e51dd01d` | 默认未知账号 verifier 初始化不预跑 Argon2；保留实际校验成本、并发预算及自定义 hasher 行为 |

必要安全修复 `c5a7ce55`、`4b6f81af`；版本/说明准备 `887789e4`、`8e846cb8`，扫描身份 `308564b0`；预览夹具 `a17deb99`、`99f9cefb`、`bbecee49`，合并 `8a637620`；业务基线与审查 `014ab935`、`e566de56`、`85fe4b81`。精确批准基线为 `df9decb19d219404aa4bc29cbde3321559e6d1e2`，以上为该基线到产品候选的全部提交。

未纳入用户已撤回的教程后续候选 `archive/fix/help-video-reliability` @ `2038339b`。已稳定发布的 1.27.2 三项补丁不重复列为新候选。其余旧分支、未知归属工作树不自动纳入或删除。

## 独立复核、OCR 与安全处置

非作者独立 Codex 会话只读复核产品及夹具，结果 PASS WITH FOLLOW-UP；其他提供商 CLI 不可用，明确 `fallback=provider-unavailable`，未冒充跨提供商审查。复核发现 Mock 对 malformed JSON 可退出，已在 `99f9cefb` 修复，21 个正反例通过。业务基线另由该非作者复核为 PASS；最初夸大的专项公开验收表述已在 `e566de56` 更正。

本次执行普通安全盲审，不是 CF scoped/full 审计。`security-review-scope.mjs` 对上一稳定版到候选的六组均 recorded；继承旧格式 trailer 有 `invalid_trailer=true` 提醒，未将其误记为 missing，也不等于全仓安全结论。quality-audit 全仓补漏本次 not-run。

- `archive-existing-20261022-H1`：既有普通解压暂存目录竞态 HIGH；`c5a7ce55` 固定暂存、发布和清理目录身份。
- `archive-source-20261022-H1`：归档源目录身份竞态 HIGH；`4b6f81af` 固定已打开源目录与成员读取。旧实现三项目录替换回归失败，修复后 Linux 文件管理包成功，非作者复核确认。
- 上述两项已交付源码和本 RC，候选 CI/L3 与公开验收均有成功回执；尚未进入稳定版或生产。静态证实与回归不冒充生产漏洞实演。
- `archive-output-20261022-M1`：既有压缩输出目标目录竞态 MEDIUM，静态确认、未动态复现，本候选未引入；KPanel filemanager maintainers 于 2026-10-25 前独立修复或重评。既有 copy、thumbnail、auth 待办保留，不由本版关闭。

OCR 1.12.11 原自由臂先落盘，原产品 39 文件及后续两夹具文件有覆盖原件；后续扫描/文档 checkpoint 按实际范围留痕。`constrained-only=unreported`，不将独立安全审查的发现归功 OCR，也不从本次预览推断三个稳定周期的收益。原件位于仓库外 `C:/GitHub/_release-evidence/kpanel-v128-rc1`，含 `free-form.json`、`ocr-coverage.json`、`helper-ocr-coverage.json`、`security-dispositions.json`、两份 independent-review JSON。源码修复/RC/稳定/生产分层状态另已保留于私有 backlog。

## 跨仓库与供应链身份

`scriptLinkageState=not-required`（无需发布脚本，不适用）。本版原生 Go 归档、前端和认证不新增 `kejilion.sh` 协议；脚本候选/跨仓变更集/移除依赖范围不适用。实际内置脚本 commit `160924a498d9b8d071fed72f165d23c1b9e886de`，SHA-256 `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`，镜像双架构标签匹配。

Go1.27.3 对六平台 actual/original 闭包重新资格化，1009 个第一方 Go 文件、110 个 production modules。sourceManifest `3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580`，graph `24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927`，closure `11fe9afd659a796fc066c756d5f819cf887c532d1f323331a34202ca663961a8`，firstParty `324e2e3e7b56ba146a3fbe077640d4dd32392e58edab2b8d346f5679202d9859`。独立复算与最终 CI/L3 身份一致。

三类扫描 production110 / selected465 / standalone150，原始 exit0/1/1，保留 finding0/10/27；后两类为完整库存及既有处置，不能说全库存零漏洞。37 项既有告警观察日 2026-10-11、到期 2026-10-25 不变，未新增 suppress 或延长期限。

[dependency-report 39786281255](https://github.com/kejilion/KPanel/actions/runs/39786281255) 在 `8a637620` 成功，必需检测源9/9；最终85与8a的依赖输入精确一致，复用有效。EOL current，下次截止 2026-10-28T23:59:59.999Z。本次 push 的 scheduled 安全审计 job 为 skipped，最近可见 scheduled 成功仍是 2026-10-21 的39565808978；本候选安全事实来自同 SHA CI/L3 的实际扫描。本版未采用依赖、基础镜像、Action、工具链或扫描器升级；报告中的新候选由既有维护流程负责，不在 RC 夹带升级，2026-10-25 例外到期与上述 EOL 截止前复核。

## 自动门禁与证据层级

- 本地 Web 314 文件3120测试全部通过；Linux文件管理定向包成功。来源内存任务 L2 曾因 Docker socket 缺失失败，原件保留；最终云端完整 L2/L3 补齐，不改写来源失败。
- 夹具与业务基线相关治理各353测试通过。夹具先由候选 CI39786008072、main CI39787114041集成到可信主线；产品没有候选独有 `.github` 自动化。
- 产品 push CI [39787356042](https://github.com/kejilion/KPanel/actions/runs/39787356042) attempt1 成功，job119853974915，source85，实际 `VERIFY_BASE_REF=8a63762021a967d77d85dc34653300149b0d31a5`，强制完整L2。原始日志 SHA-256 `8d24375d5f52d589d6bf1ded498f11e8f26a23115e2da226b4cb770dc397b12d`。首轮8a的39786281308被业务基线过期拦截，产品测试尚未开始，刷新累计61提交/90文件六维业务复核后恢复，没有降低阈值。
- 发布控制面 `bbecee495988f84709077dcd5f1a9b863ec5e0d2`；外层39790232673/1内 L3 job119864151952成功，候选复核job119864152041成功；主线 CI job119873448741 source85/basebbecee成功，原始主线日志 SHA-256 `4154844106d812f53aa217000badf4db3a0cca02b53fad44456e92f9fa8495d1`。
- L3 只在 GitHub-hosted Ubuntu24 image20261018.339.1，由 `run-release-l3.mjs --github-hosted` 进入 `make verify-release`。Go1.27.3、Node24.22.1、Docker28.0.4；2026-10-22T13:49:48.167Z至14:03:07.599Z。源码Web/Go/deploy三组exit0，identityUnchanged=true，Linux Web3114pass/6skip（314文件），镜像 `image_e2e=pass`。六项 skip 与本地平台差异如实保留。
- L3回执 artifact12431856727 ZIP SHA-256 `cb448916aee6b168823193825a4d61e8629410c0a3c0d367aa01aaf413c5b3ad`；安全检查解包，三组日志摘要独立匹配。回执 `entrySha256=43e57f4fd321846204020d6cf27eb75961f0aefa1284a9187e786601704c2d40` 对应内层 `run-release-gate.sh`，不是外层JS。
- release job119881418218所有发布步骤成功；复用上游 L3，不把内部 skipped 的重复L3误判失败。草稿与最终正文均经过真实 selfupdate 解析器，最终4条摘要、3条升级注意，版本/digest匹配。

## 公开产物与只读恢复验收

首次public job119890044401在2026-10-22T14:41:56Z失败：复用 workflow继承外层 `workflow_dispatch`，入口误进独立dispatch校验，而 publication run/attempt未传，尚未开始公开业务断言；后续无artifact是同一前置失败的结果。原始日志 `public-acceptance-first-failed.log` 保留。自动入口尚未永久修复，KPanel release maintainers须在下一RC发车前且最晚2026-10-25前，以独立治理候选修复和回归；本次合法恢复不代替永久修复。

只读恢复39798207805/1、job119891109800成功，控制器/产品均85；输入明确绑定原publication39790232673/1、上述candidate/version与digest。下载原snapshot，不手工重建历史快照、不重新提升通道。恢复业务检查于2026-10-22T14:45:25.265Z完成，artifact于14:45:30Z生成。

冻结 stable-before SHA-256 `0fe725ed6cf76abbb8bbc91e9ea9528b95108b1d8042d262b4c3432c8bc18fe9`，附件manifest `423c6cdbc9b1034ad3fa45aa9a76693bde66f6f2eea20fbdb774a34252747c2d`，原最终正文 `572fe08995b2ba8213a1eeb9366dbef501a9e6552cd7b23c16ecfd038a2f7bfa`。snapshot artifact12433453838 ZIP `83b5fed72c414ab667f11e2331acf9b52cd599cab6d052d7d265ffd825a04431`，publication12435075032 ZIP `f34bb891ea25bcde90df96c5f638f5cfb58c0732263c46bd8f5e357f385f751e`，共同identity/三文件字节相同，15附件API名称/大小/digest和正文逐字节一致。

公开工具实际下载15附件、API digest与磁盘回读校验、11项SHA256SUMS成功。metadata tar不是可构建源码包。双架构source/version/脚本标签及SBOM/provenance通过；不可变版本与Docker preview OCI index均为 `sha256:3e933fe16dcdf49b51e7403d9c6ee25618be1c9582641ecad8f0d7e11f5442ae`。

- linux/amd64：`sha256:ea7500c011339a403c76c39d71132236f447bdf70ebf13057d046187a39fb9ab`
- linux/arm64：`sha256:6f8a73bbdc663498134c4b8f61abe842d1525f9859770b8856c9b5f064d74834`
- GitHub Latest仍v1.27.2；Docker latest仍 `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`。
- 发布前preview为1.27.0-rc.23 / `sha256:144cf771a1043dc584b342cd2cef15e3f0b9280a632706b0c1c04576a1ac0dd7`，本版只更新preview。

恢复artifact12435155611 `public-39798207805-1` ZIP SHA-256 `dd0bc8bf9a54f26c2ce127625adead590e8ce31d620a05aa0127e245ed12f841`，下载字节与API摘要匹配，经安全解包、逐项复核JSON。四份报告passed、mocked=false、errors=[]：initial44、restart26、published45、upgrade31（各报告断言数，不能累加为独立测试数）。Runner ID `sha256:5ffe6c35c423762f1643dfa41e426b408acffaa443e89137f494b7a3c7f52f2d`；原/实际recipe摘要同为 `e2a026415568924488a35e53e5529aae40e2d57f99640d0a743444316f8a3926`，diff空、productImageUnchanged=true。

真实Panel/Agent、PTY与浏览器覆盖本地应用/图标、偏好/桌面重启恢复、TLS证书故障/离线恢复、域名边界、终端续接/关闭及命令历史、真实发布信息API/更新弹窗。Docker inventory与测试CA是明确夹具；不挂载真实宿主Docker/systemd socket。watchdog、资源限额、cleanup回执均留存，短时场景不构成长时soak或实体宿主验收。

旧RC23后端真实发现本RC，API与更新弹窗绑定正确版本/digest、4条摘要及3条升级注意。1280/390/320无水平溢出断言通过，人工查看1280/320截图文字正常；宽屏全部更新和注意可见，窄屏需要内部滚动查看后续内容。默认stable且自动安装关闭；取消加入不改变stable，加入preview仅检查、不启用或请求安装，同版本不重装；新RC退出preview不降级。无systemd立即安装受控503并清理队列，前端无需刷新可切换来源，不能据此声称systemd后台安装/备份/回滚已实演。OpenRC/轻量Node边界仍按release-channels文档。

## 本地专项、六维结论与未验证范围

精确85候选本地Mock预览覆盖包浏览、多选下载链接、空状态、Esc关闭与焦点返回，console error0。最终 `web/src` 与布局来源f245相同，复用原功能预览；Mock不证明真实下载吞吐/实体宿主写入。原启动内存A/B证据保留，优化范围仅初始化；未证明登录后工作集即时归还或全程32MiB。

| 维度 | 状态 | 证据与边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 定向回归、完整L2/L3、真实隔离API/PTY；实体混合版本远程Agent未验 |
| 网络入侵与供应链安全 | 已验证 | 两HIGH修复回归/非作者复核、扫描、公开附件和镜像身份；既有MEDIUM仍待办，不等于全仓无漏洞 |
| 稳定性、恢复与兼容 | 已验证 | 隔离重启、证书故障、只读发布验收恢复、受控503；实体systemd更新/回滚未验 |
| 性能与资源预算 | 已实现未实机验证 | 启动初始化A/B和保留Verify成本回归；低配组合峰值、大归档/慢网络/长时soak未验 |
| 用户体验与可访问性 | 已验证 | 本地Mock、窄布局回归、公开更新界面/键盘焦点；原生125%/200%缩放、读屏、实体手机未验 |
| 数据、配置与迁移 | 不适用 | 不增加迁移或存储格式；隔离持久化恢复通过，用户升级仍须配套备份 |

继承OneDrive真实账号云端往返未验、自签名TLS端口仍接受明文HTTP API边界；限制可信网络并强制HTTPS。未验证项不因RC通过而关闭。当前准入理由为变更范围有限且必要门禁与公开产物验收完成，预览需要显式加入，不影响稳定默认。

## 生产、回滚、候选与资源处置

生产动作全部不适用（预览版禁止生产部署）。没有生产目标/部署授权；未连接、备份、部署、升级、回滚或核对生产。`prod-108`禁用全部KPanel操作，本次未连接。发布/验证环境只使用github-hosted，不以隔离容器冒充arena-154或其他生产证据。

源码/tag回滚点v1.27.2，镜像为上述固定稳定digest；本轮未执行回滚，没有新的实体数据备份。需要回滚时按既有生命周期停止/备份后以固定稳定镜像及配套数据备份恢复，再检查Panel/Agent、数据与健康；退出preview不会自动降级。GitHub Latest/Docker latest/标准stable更新入口保持原稳定版，无恢复默认通道动作。

`release/v1.28.0-candidate`精确tip85远端核对相同，按预览规则保留，不归档RC列车。夹具治理 `fix/v128-preview-fixtures`精确tip `bbecee495988f84709077dcd5f1a9b863ec5e0d2`已归档到 `archive/fix/v128-preview-fixtures`，远端同SHA、原引用按expected-SHA lease删除，本地branch同名归档且工作树保留。该归档不等于部署。

本验收记录在 `docs/release-v128-rc1-acceptance`专用文档工作树提交，按同SHA候选Linux CI→main快进→主线CI成功后归档到 `archive/docs/release-v128-rc1-acceptance`，不改变产品标签/镜像。产品/来源/安全修复/文档工作树与仓库外证据保留以供追溯；其他所有权不明资源不回收。当前任务4175模拟预览在移交后按manifest停止；没有删除磁盘文件，不推断净释放空间。未新增知识库、工作流或永久规范。

## 交付节奏与流程异常

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-22T15:38:11+08:00
- 候选冻结时间：2026-10-22T21:29:41+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：11
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

以上变更失败字段不适用于本轮流程异常：没有产品载荷引起的生产退化、回滚、紧急热修复或重复发布。公开验收流程发现时间2026-10-22T22:41:56+08:00，恢复时间2026-10-22T22:45:25.265+08:00；未逃逸公开验收门禁，原run失败保留，恢复入口绑定冻结产物完成断言，无生产写入。草稿曾将这段流程恢复填入变更失败字段，被本地指标校验拒绝，已按两类指标语义更正；这是文档编写校验，不计为另一次发布执行失败。

计数口径为本轮已记录的准备、资格化、夹具、候选门禁及取证/公开验收流程拦截，不是11次产品发布失败。开发中间断言调整、有效旧代码red回归和可选探测不机械计数。较早来源任务未知的全量操作不作补算。部分失败只保留工具原回执而非独立日志文件，下面明确区别；成功后不覆盖首轮记录。

<!-- kpanel-release-process-incidents:start -->
[
  {"fingerprint":"qualification/task-preflight/missing-go-path","position":"before-production-write","count":1,"impact":"Windows资格化时Go未在PATH，工具原回执保留","recoveryEvidence":"采用已核实Go1.27.3固定路径后预检/候选门禁通过，takeover-contract.json","permanentAction":"KPanel release maintainers：与v1.27.2重复；2026-10-25前或下一L3生产写前，修复唯一预检入口并补回归，本次固定PATH只是绕行","historicalReleases":["v1.27.2"]},
  {"fingerprint":"qualification/source-inventory/windows-esm-path","position":"before-production-write","count":1,"impact":"初次来源资格化遇Windows C: ESM路径错误","recoveryEvidence":"source-qualification.log失败、source-qualification-r2.log与result.json成功","permanentAction":"仓库外qualification脚本已修正Windows URL适配，保留修前日志；无产品规则改动","historicalReleases":[]},
  {"fingerprint":"validation/archive-regression/offline-module-cache","position":"before-production-write","count":1,"impact":"首次Linux回归准备无法离线取得模块，未构成产品测试结果","recoveryEvidence":"archive-regression-before.log；缓存/环境修正后filemanager-linux-final-r2.log及云端L3成功","permanentAction":"KPanel release maintainers于2026-10-25复核模块缓存资格化；退出条件为固定工具/依赖可用，保留离线边界，不将一次网络成功写成永久修复","historicalReleases":[]},
  {"fingerprint":"preview/archive-fixtures/malformed-json-exit","position":"before-production-write","count":1,"impact":"错误选择JSON可使必需Mock预览退出，独立复核拦截","recoveryEvidence":"takeover-independent-review.json、21例正反验证、helper两轮治理日志","permanentAction":"99f9cefb补齐输入校验；bbecee49同SHA候选/主线CI通过后集成","historicalReleases":[]},
  {"fingerprint":"candidate-ci/check-business-context-freshness/stale-baseline","position":"before-production-write","count":1,"impact":"39786281308在产品测试前因累计基线过期失败","recoveryEvidence":"candidate-ci-first-failed.log；014ab935/e566de56六维复核与独立PASS，39787356042成功","permanentAction":"刷新业务事实而不改门槛；现有freshness入口继续拦截，最终baseline8a、commits3","historicalReleases":[]},
  {"fingerprint":"evidence/github-run-log/unresponsive-transfer","position":"before-production-write","count":1,"impact":"CLI整run日志取证无输出而停滞，自有读取进程取消，0字节文件不作证据","recoveryEvidence":"candidate-ci.log为0字节；连接器取得4.49MB candidate-ci-job.log并绑定摘要","permanentAction":"本轮改用GitHub连接器原始job日志；KPanel release maintainers于2026-10-25复核CLI通道，恢复条件为有界读取与完整日志摘要可核验","historicalReleases":[]},
  {"fingerprint":"evidence/github-job-log/bounded-transfer-timeout","position":"before-production-write","count":1,"impact":"直接job日志下载60秒超时，未读取完整日志","recoveryEvidence":"candidate-ci-job.log.error.txt；连接器下载后摘要核对通过","permanentAction":"保留有限超时并改用连接器；KPanel release maintainers于2026-10-25复核上游传输，完整原件校验通过才退出例外","historicalReleases":[]},
  {"fingerprint":"evidence/github-run-watch/tls-handshake-timeout","position":"before-production-write","count":1,"impact":"CLI发布watch TLS握手超时，不能据此推断workflow终态","recoveryEvidence":"publication-watch.log；以run/jobs、原始job日志和artifact继续核验","permanentAction":"KPanel release maintainers于2026-10-25复核网络/CLI；本轮使用连接器有界查询，不盲目重发版本","historicalReleases":[]},
  {"fingerprint":"evidence/candidate-ci/base-ref-misinterpretation","position":"before-production-write","count":1,"impact":"临时契约将push CI base误记为bbecee，未据此发布新内容","recoveryEvidence":"candidate-ci-job.log实际env为8a；takeover-contract.json已按原日志更正","permanentAction":"本轮以实际job参数冻结身份，最终文档明确push/main不同base；不将资格回执冒充L3","historicalReleases":[]},
  {"fingerprint":"evidence/publication-artifact/zip-layout-assumption","position":"before-production-write","count":1,"impact":"辅助读取publication ZIP误猜flat路径，KeyError，取证未完成","recoveryEvidence":"工具原回执；按namelist实际嵌套路径读取，publication-artifact-verification.json成功","permanentAction":"仓库外verify-publication-artifacts.py按原布局验证并拒绝不安全条目；不更改公开artifact","historicalReleases":[]},
  {"fingerprint":"public-acceptance/validate-frozen-source/inherited-dispatch-event","position":"before-production-write","count":1,"impact":"复用工作流误入独立dispatch分支，run/attempt为空，公开业务断言尚未启动","recoveryEvidence":"public-acceptance-first-failed.log；39798207805/1从原39790232673/1 snapshot只读恢复成功，public-recovery-verification.json","permanentAction":"KPanel release maintainers在下一RC前且最晚2026-10-25前独立治理修复workflow_call事件判定并补回归；本次恢复不是永久处置","historicalReleases":[]}
]
<!-- kpanel-release-process-incidents:end -->

另有GitHub官方Actions Results处理延迟事故自2026-10-22T14:18:15Z起，现场观察step完成而job/后继状态延后，官方快照 `github-status-incident-initial.json`保留。它解释外部等待，不替代上述公开入口的项目根因；没有把等待额外算作产品失败。未声称该官方事故已恢复。
