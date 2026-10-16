# KPanel v1.27.0-rc.5 发布验收记录

结论：RC5已公开发布，GitHub-hosted完整L3、公开产物和真实隔离浏览器只读恢复验收通过，独立复核PASS。正式root因测试环境失效失败的事实保留；已发布产品未重建，未部署生产。

日期：2026-10-17；发布级别：L3；releaseChannel：preview；releaseTrain：1.27.0。

## 范围、版本及授权

用户授权完善 GitHub 发布、集成主线、纳入成熟业务候选并继续预览。RC4 已公开不可覆盖，故顺延 RC5。
发布止于 GitHub Release 与 Docker Hub，不部署生产，不提升 stable/latest，不连接自有服务器。

- 产品提交：`c8ee07a82775549ab37aeefcc1a43982be24a3c0`；组装源码 `b120b4420bc8c0e11dae979ada90964b22f9f320`。
- 注释 tag：`v1.27.0-rc.5`；tag 对象 `5624949d4846d15731db8c86716d3b2e24944df8`，peeled 为 c8。
- 正式控制面：`b5eccb8baa3ad0c62950c918cb24a2a00ae04d75`；控制面 SHA 不是产品 SHA。
- 发布流程改进 `c3b8d089f1b73641e94e57c7469abaccf019cabb` 先经候选 CI38948162332、main CI38949060824和独立复核纳入主线。
- 业务组合 `b5eccb8b` 经精确候选完整 Linux L2 CI38951661165及 main CI38953787588通过后集成。

## 新业务与来源映射

| 来源分支 / 原始提交 | 组合重放提交 | 用户可见结果 |
| --- | --- | --- |
| claude/editor-split-panes / d451a2c0bd04d26cab0f260daf4de44ab444988b、b5d41676fe6fc1d98813a320b647f1ae9fce38a1 | 10bedc6ab83ad02b97c732747d7eb1c607747981、6ec330695b2256c5da17930f54c7e2cc44739b75 | 单屏、左右、上下和四宫格编辑；查找/跳转/保存作用于当前文件，隐藏文档保留草稿与撤销 |
| fix/desktop-cross-viewport-layout / 9edc2e1d695975ef8a2e315b88398833750e0e0f | f25768ab9c75d7898698e3c584c30e59871856c5 | 共享几何与视口呈现分离，窄屏会话不会压缩保存的桌面布局与分屏比例 |
| fix/terminal-auto-temporary / 85c55aaeed387278ad00d044ae1d58960dd75f26 | 340e70e280d73ae02f5e9d14ff94752d463295ee | 仅明确 terminal_retention_unavailable 才一次回退临时终端；通用502、网络不确定、权限或上限不自动重试 |

集成修复 `c8733822` 关闭独立发现的两项：通用终端失败时不得再次创建结果不确定的远端 PTY；
编辑器停放恢复时清空与关闭查找栏不一致的旧 query/高亮，保留草稿和撤销。`d7103aaf` 仅修正提案状态枚举。
原来源通过 cherry-pick -x 纳入，不将原作者 refs 误称为产品祖先；作者工作树和分支保持所有权。
此前 RC2/RC4 的本地应用、HTTPS、共享偏好和终端能力保留，不算本轮新增。

业务 Go 源码身份与 RC4 相同 `be026eb8acd52159f33380f8ba72e7a0642415b1d97fe6d1958ab0b9ff862c70`。
冻结六文件只涉及 VERSION、CHANGELOG、Go 开发版本字符串、Web package/lock 版本与扫描审查绑定。
无业务 Go、依赖、权限、数据 schema、安装、端口或应用市场契约变更。

## GitHub 链路及实际失败恢复

[正式 run38956027316](https://github.com/kejilion/KPanel/actions/runs/38956027316) 由 main/B5 控制面冻结产品 C8。
candidate CI、完整 L3、promote、main CI、Release/Docker Hub 均 success。内层重复 L3 按设计 skipped；
外层 L3 已真实执行。公开资产/OCI 检查 passed，但公开浏览器在测试 runner 构建阶段失败，
Alpine 已将固定 docker-cli29.8.2-r0 / buildx0.34.1-r1 替换为 r1/r2；APK exit2，浏览器未启动。
原 root conclusion=failure、public-acceptance=failure、preview-completion=skipped 原样保留。

恢复修复 `f615c5d32673d7cbdf17e1e370d65996e9ca6f9d` 的产品 Go/Web、tag、镜像均未改变。
仅更新两项固定测试 APK 修订版，并从可信 main 控制面固定路径取 runner Dockerfile：覆盖前产品树 clean，
覆盖后只允许这一个测试配方差异，HEAD/VERSION/EXPECTED_SHA 不变；回执保存原新配方全文、SHA256、
精确 diff、产品/控制面/run/attempt。此项是测试环境恢复，不声称覆盖后的冻结工作树 clean 或原 L3 环境未变。
[只读恢复38964462298](https://github.com/kejilion/KPanel/actions/runs/38964462298)实际success，2026-10-16T20:44:57Z–20:46:36Z。控制面F615、产品仍C8/index8e00；全部身份、资产、配方、浏览器及归档步骤成功。public artifact ID12074511660，2307607bytes，服务器ZIP SHA256为4c04334395b7e36ecc9163568f29cf8c586ecfa0e0af8ba70cefcf3061b4d42f；CRC与51个解包文件逐字节一致。恢复使用上述原snapshot，不重新冻结当前通道或重推产品。

本轮候选 CI 与 L3 同在2026-10-16T19:35:39Z启动；CI结束19:51:26Z（947秒），L3结束19:50:31Z（892秒），
实际重叠892秒；promote于19:51:30Z在双成功后开始。同轮资格化span947秒，而串行相加1839秒。
不同运行的 CI floor 与负载不等，不直接拿 RC4 总耗时归因，也不声称 runner 成本下降。
流程提案继续试行，本轮为3次预览观察窗口中的第1次，后续2次继续观察。

发布前原件 `snapshot-38956027316-1` ID12071529703，2026-10-16T20:13:00Z已保存，早于镜像推送和公开。
原 ZIP SHA256 `e7c05aa2796079ea9c52787bb2e65cf090b4e09582583ed83daebbec9017384f`；
stable-before 原字节 SHA256 `67a0c8b59fe201c8a0005f11347085e66cb6e72414262daf5b1eb8d5ce3f7fe7`。
identity 绑定 C8产品/B5控制面/run38956027316/attempt1/before-publication。
snapshot、publication 和 public artifact 的快照逐字节相同；原件留存与恢复均无事后重构输入。

## 自动门禁与独立复核

- 业务定向 Web50/50、真实 Git trailer/shell11/11、本地 Windows Web2900/2900、前端构建通过。
- Linux Web实际2894 passed +6 Windows-only skipped，不将本地结果冒充Linux通过数。
- 本地完整L2先缺Make，随后既有Linux专属Go测试前提失败，不能记为通过；最终采用精确GitHub/Linux完整L2及全部后置门禁。
- 本地治理首轮326/327（超时状态分类失败），独立5/5及完整第二轮测试327/327；第二轮尾项及旧128aff候选CI又被提案状态枚举拦截。d710一行修复、30提案状态校验与最终Linux完整治理通过。原失败均保留。
- RC5精确候选push CI38954124646、依赖新鲜度38954124695全部success；正式root候选/main CI全success。
- 恢复修复定向12/12（实际工作流shell、真实Git六场景），本地完整治理328/328及尾项通过；精确Linux候选CI38962276343及全部后置门禁success。修复主线CI38964445691与本文档自身的CI/归档终态由外部最终交付回执记录，不提前写未来结果。
- 独立源码、版本绑定、L3、公开产物、流程试行分别复核。原件保留：business-independent-review-r2/r3、preview-r4、qualification-r5；rc5-independent-source-r1、binding-r2、l3-r3、publication-r4；pipeline-independent-trial-r2、runner-recovery-independent-source-r1。恢复独立R5为PASS，原件SHA256为46b5ac414c080a4c0f843dbd0568384e9781b094d64860ef9599346c365c43cb。复核者与实现者为独立会话；其他提供商不可用，按规则明确fallback，不将工具品牌当作独立性证明。
- OCR1.12.11：流程6/6+3排除文档人工覆盖；业务20/20+6人工，2项自由臂发现已关闭；RC5组装4/4+2人工；恢复3/3+1人工。各精确范围无新增constrained-only，不混为CF审计或完整稳定周期收益。
- CF not-run：按当前政策为可选辅助；未补审、未豁免、不作为前置门禁。历史CF状态不改写。

RC5完整L3原始receipt为2026-10-16T19:36:00.044Z–19:50:25.532Z，candidate=C8/status=passed。
GitHub-hosted Ubuntu24/ImageVersion20261012.334.1、Go1.27.2、Node24.21.0、Docker28.0.4。
唯一gate入口 SHA256 `c286f7f2ad692ea17b8ec0537fe2a84e8d049d22adec63a2b00ceba39b165d57`。
source tree03c535aad3dc1ad7214eda595de76579517aae1a、identityUnchanged=true、三lane均passed；
总555864ms，Go555842、Web310341、deploy2437，jobs2/GOMAXPROCS2。三个lane日志摘要重算一致，
27份rawEvidence字节/摘要/exit及12份六平台actual/original输出独立核实，不用工具退出码替代业务成功。

## 源码、依赖及脚本联动

首方Go source SHA256 `ccdbae73585ab94a5c0e70c49030af9492d364368ebc2cc61909b26007ba6ac8`（950文件）；
graph24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927、
closure98ac00a5cdff85d5e3369149a0e5fc5f3c7e3ec2f73e8cb8ea50ff0469896cae、
vendor3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580。
110 productionmodules、6targets/12actual-original闭包一致；生产扫描exit0无finding。
selected exit1保留10项、standalone exit1保留26项，共36既有finding，2026-10-25到期规则不变；
不能宣称全依赖图无漏洞。scope.review绑定原R1实际字节摘要227c1923c6b5cf57c4acaf00159980ece618f6ff53d162e7d4271dcf84a42083，R2仅核review元数据delta。
两项TypeScript/Node维护例外沿用RC4实际复核与2026-10-23复核期限，未降低升级检测或到期告警。
本轮依赖新鲜度成功不等于升级采用；唯一新环境依赖是上述测试runner两项APK修订版，未进入产品镜像。

无需发布脚本（不适用），scriptLinkageState=not-required；内置kejilion/sh提交
160924a498d9b8d071fed72f165d23c1b9e886de，SHA25639837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1。
已有契约兼容、未引入新的脚本动作或宿主产物；本轮不另发脚本，不改应用市场dirty目录。

## 隔离浏览器与多维质量

组合Mock预览绑定clean B5：7项主题/中英/窄屏/停放/终端边界，另2项当前pane正确PUT保存与桌面共享几何往返，
共9项通过、0记录错误、context清理。浅深与语言通过受控Mock响应及实际DOM断言；125%为视口等效，
200%为CSS zoom，非原生浏览器缩放；桌面状态使用实际browser store与1440→390→1440视口，非实体第二设备。
错误监听并非覆盖全部页面，200%主要截图检查，不将定向证据扩张为全部可访问性通过。
本地验收预览http://127.0.0.1:4174保留供体验，属于B5 Mock，不能误称RC5公开镜像真机。
公开恢复使用GitHub-hosted一次性隔离容器和真实浏览器：初始42、重启18、已发布PTY/通道37、旧预览后端加新Web更新提示31，共128项passed、errors0；8PNG逐张独立查看。Panel使用公开镜像，Agent从同镜像提取并在隔离runner执行真实PTY；Docker清单与测试CA为受控fixtures，不称真实宿主Docker或混合版本节点。3份cleanup回执、0资源guard失败；记录容器退出及清理receipt，不伪称runner终止后的live Docker无残留清单。实际runner镜像inspect与配方回执均在public artifact。

| 维度 | 状态 | 证据与边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 定向/全Web、组合Mock与公开隔离旅程；真实混合版本节点未验证 |
| 网络入侵与供应链安全 | 已验证 | race/安全扫描、精确双架构身份与附件；36既有发现及CF未完成事实保留 |
| 稳定性、失败恢复与兼容 | 已验证 | 明确续接拒绝一次回退、generic失败不重试、park/remount草稿与撤销；公开恢复不改tag/image |
| 性能与资源预算 | 已实现未实机验证 | 测试有CPU/内存/PID/超时和清理限额，未做低配长期负载或原生ARM实机 |
| 用户体验与可访问性 | 已验证 | 受影响主题、语言、窄屏、键盘及截图；原生zoom/读屏/实体手机未验证 |
| 数据、配置与迁移 | 不适用 | 未变更schema/端口/安装契约；真实systemd安装升级/回滚未补验 |

## 公开产物、通道与回滚

- [公开RC5](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.5)：2026-10-16T20:19:43Z发布，draft=false、prerelease=true、非Latest。
- Docker版本1.27.0-rc.5与preview同index：`sha256:8e00585d30dc94eea361fdc0d4d362b292496b51ca1922f82b51f65ad51a2a94`。
- linux/amd64：`sha256:74946c061e0bfbe1cbdcdbe4e4dc2eb6decaf84adfb566fba43a01f31bbfe3b9`。
- linux/arm64：`sha256:d202ccd722068e30247cc2c31fe676214e7971bd82368ec8ae095823023784ae`。
- 15附件、11项SHA256SUMS、OCI source=C8/version=RC5/script身份均一致；5条更新及3条升级提示通过最终解析与公开读取。
- 原public artifact保存13个OCI/SPDX/SLSA原对象并独立核验；SLSA Git materials absent如实保留，源码身份同时由OCI标签、build配置、GitHub tag及冻结回执交叉验证，不虚构materials。
- 版本/通道、不自动安装、重启保持、退出预览不降级及实际更新提示以本次公开浏览器实际报告为准；宿主systemd更新/备份/回滚不由容器验收证明。
- 原始证据根：C:\GitHub\_release-evidence\v1.27.0-next-github-quality。ZIP按服务器SHA256核验，原失败、snapshot、L3、public和独立报告均保留。
- GitHub Latest v1.26.0、Dockerlatest `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`不变。
- 预览回滚输入为RC4/e20f427d6ee14aac38c8656e39aa3d61e3934dc0，镜像`sha256:d2d22e6b6e4320ae1f5541327f653b77c86b49316b4f1da497c3fa2d02fc3e12`；实际回滚部署未执行，需另行授权和数据备份。

## 生产与来源收尾

生产动作均不适用（预览版禁止生产部署）；未连接、备份、部署或核对prod-108及其他自有服务器。
release/v1.27.0-candidate保持C8，预览列车保留。流程分支已归档archive/feature/github-release-quality=C3，
远端原活动分支已移除，本地WT在归档/所有权/clean/无独有文件核验后安全回收，实测卷净释放63246336bytes。
本轮业务feature/v127-business-next=B5已原子归档至archive/feature/v127-business-next，远端原活动ref移除且readback精确B5；恢复fix/github-public-runner-recovery=F615在其主线CI及收尾完成后按10.2归档；
最终远端readback、验收文档自身CI与归档结果记外部终态回执，不提前写自身未来结果。
业务本地树因活跃4174预览保留，upstream收尾指向只读archive；原三组作者分支/WT与既有preview进程保留，不擅自清理。
恢复与本文档owned WT仅在提交可从main/archive恢复、clean、无独有ignored文件/嵌套Git/进程后回收。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-17T01:37:37+08:00
- 候选冻结时间：2026-10-17T03:19:21+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（预览未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：14
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "qualification/task-preflight/management-as-writer",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次ready在管理树执行，writer角色校验按预期拒绝；未创建或修改仓库内容。",
    "recoveryEvidence": "原工具会话保留失败；按已填写契约从精确9b创建专用writer后ready passed。",
    "permanentAction": "ready需对已建立的隔离writer运行；不从管理main执行writer预检。",
    "historicalReleases": []
  },
  {
    "fingerprint": "review/editor-query-repro/windows-esm-import",
    "position": "before-production-write",
    "count": 1,
    "impact": "独立CodeMirror复现首次使用Windows C: ESM import而失败，没有运行断言。",
    "recoveryEvidence": "business-independent-review.reproduction.firstAttempt明确保留工具会话原件边界；file:///修正后的editor-query-retention-repro-r2.log exit0。",
    "permanentAction": "Windows外部ESM复现使用file:/// URL；首次诊断不冒有效验证。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/business-l2/missing-make",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地 L2 首次因缺少 Make 在预检失败。",
    "recoveryEvidence": "business-l2-r1.log; native-make-identity.json; native-make-dry-run.log",
    "permanentAction": "使用固定校验身份的 native Make。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/native-make/adapter-env-rejected",
    "position": "before-production-write",
    "count": 1,
    "impact": "run-repo-bash 正确拒绝非允许 SHELL --env；未执行 Make。",
    "recoveryEvidence": "首次工具会话；后续 PowerShell 环境变量配置正常，native-make-dry-run.log。",
    "permanentAction": "SHELL 在调用进程环境设置，不绕过 adapter 白名单。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/business-l2/windows-linux-prerequisites",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地完整 Go 测试依赖 Linux 绝对路径和系统管理后端而失败；没有源码 Go 改动。",
    "recoveryEvidence": "business-l2-r2.log; final exact B5 LinuxL2CI38951661165 and mainCI38953787588 passed; Windows failure not relabeled",
    "permanentAction": "GitHub 读取精确候选 trailer，运行现有 make verify-change 完整 L2；不修改无关 Go 测试。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/governance/background-timeout-classification",
    "count": 1,
    "impact": "治理首轮326/327，后台超时状态failed而非timed_out；精确原因未确定。",
    "recoveryEvidence": "business-governance-r1.log；独立重跑5/5；完整r2测试327/327。",
    "permanentAction": "保留首失败，不猜测根因；不改无关测试，以实际LinuxL2最终核验。",
    "position": "before-production-write",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/governance/proposal-status-prefix",
    "count": 2,
    "impact": "root将试行写为主线试行，不符合既有枚举；本地r2治理尾项及旧候选GitHubCI失败。",
    "recoveryEvidence": "business-governance-r2.log；business-obsolete-ci-final/run.json；d710仅1行修复；business-governance-health-r3.log；独立r3PASS。",
    "permanentAction": "提案状态保持规范试行枚举；main采用事实放括号，更新事实后运行现有状态校验。",
    "position": "before-production-write",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/browser/mock-preferences-overrode-localstorage",
    "count": 1,
    "impact": "初版浏览器截图文件名浅深/语言不等于实际配置；只可复用其他交互断言，不作为主题/英文证明。",
    "recoveryEvidence": "browser-business-128aff7c/result.json保留边界；final受控mock appearance/preferences，实际DOM data-theme/lang断言和新7casepassed。",
    "permanentAction": "覆盖参数必须由实际DOM断言确认，不凭截图文件名或localStorage设置。",
    "position": "before-production-write",
    "historicalReleases": []
  },
  {
    "fingerprint": "public-acceptance/test-runner/alpine-fixed-revision-removed",
    "position": "before-production-write",
    "count": 1,
    "impact": "RC5 product successfully published; root failed before browser execution because Alpine replaced pinned docker-cli29.8.2-r0/buildx0.34.1-r1 with r1/r2. Asset/OCI checks passed; browser checks not run.",
    "recoveryEvidence": "rc5-root-final and original failed log retained; repairf615 candidate full LinuxL2CI38962276343 and independentR2PASS; readonly recovery38964462298 success with original snapshot and unchanged productC8/digest8e00",
    "permanentAction": "Refresh exact test APK revisions; bind the only test-runner Dockerfile to trusted main controller with product/controller/recipe receipt; recover readonly, do not republish immutable artifacts.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/runner-recovery/vendor-stat-cache",
    "position": "before-production-write",
    "count": 2,
    "impact": "New repair writer ready rejected twice after exact vendor HEAD bytes were restored while Git index stat cache still reported30 unchanged-content paths; no source edits authorized until ready passed.",
    "recoveryEvidence": "Tool transcript; blobhash/rawbyte verification and git add identical HEAD bytes refreshed stat cache with empty staged diff; final ready passed.",
    "permanentAction": "After byte restoration verify every HEAD blob and refresh index stat before writerready; preserve vendor content exactly.",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/runner-recovery/independent-trailer-format",
    "position": "before-production-write",
    "count": 1,
    "impact": "First empty review metadata used noncanonical Independent-Review fields; writer emitted advisory nonconforming. Source review remained valid; old a0 CI intentionally cancelled to avoid redundant qualification.",
    "recoveryEvidence": "Tool transcript advisory; canonical f615 trailer recorded; exact fullLinuxCI38962276343 and independent qualificationR2passed; old a0CI38962236885 cancelled intentionally",
    "permanentAction": "Use canonical Independent-Review field syntax before final candidate CI; preserve metadata commits and old cancelled run, never label advisory as review PASS.",
    "historicalReleases": []
  },
  {
    "fingerprint": "recovery/evidence-collector/windows-apps-python-shim",
    "position": "before-production-write",
    "count": 1,
    "impact": "Independent recovery collector first selected WindowsApps python shim; ZIP safety preflight exited1 before target creation. No GHA/browser/product rerun or duplicate artifact download.",
    "recoveryEvidence": "Collector tool transcript and rc5-independent-runtime-r5.json preserve failed-entry boundary; official bundled Python selected, safe ZIP/CRC/51files byte verification passed.",
    "permanentAction": "Use explicit official bundled Python runtime for ZIP safety collection; never infer availability from WindowsApps shim path.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
