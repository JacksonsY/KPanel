# KPanel v1.27.0-rc.6 发布验收记录

已验证：RC6 于 2026-10-17T05:52:31Z 公开，正式 run39004865035 attempt1 于05:54:26Z完成，全部必需阶段 success。产物已发布，生产未部署。

日期：2026-10-17；发布级别：L3；releaseChannel：preview；releaseTrain：1.27.0。

## 范围、授权与版本身份

用户要求发布本轮全部五条候选，并明确授权保留持久化命令历史、调整对应存储规则。发布只到 GitHub Release 与 Docker Hub，不部署生产，不提升 stable/latest，不使用自有服务器或 WSL。

- 业务组合：`e8599eedbf5f6a4576636b3b808430bbf5c7ab41`，基线 `80bd333cec29eb3772aae8bcc4064efbb89c7919`。
- 冻结产品：`a1cbaa44bca5b527911e39f97fdc1d37862dfa10`；版本组装 `7dcbbbf40377124fa1f67ebc51196a08733b2f69`。
- 注释 tag：`v1.27.0-rc.6`；tag 对象 `c4c68ada8019a81953ca6ed18a93a9dddd3ccd54`，peeled 为 A1。
- 正式控制面：E859；[发布 run39004865035](https://github.com/kejilion/KPanel/actions/runs/39004865035)，attempt1。控制面 SHA 与产品 SHA 分别记录。
- 唯一候选 `release/v1.27.0-candidate` 保留 A1；RC5/tag/版本镜像不覆盖。

## 发布画像与来源映射

涉及终端前后端协议、兼容性、查看器生命周期和 Panel 命令历史数据。需要 Linux L2、精确发布 SHA 的 L3、受影响 Mock 和真实公开镜像浏览器验收。未增加依赖、端口、安装协议、宿主目录挂载或 Agent 权限。

| 来源分支 / 精确 tip | 纳入提交或关系 | 用户可见结果 |
| --- | --- | --- |
| fix/terminal-retention-preflight / 2ee3805aa643e7650dc78548d91165aad36eef5e | 46822292 | 先探测续接支持再创建终端，不因能力拒绝误建旧节点 PTY |
| fix/shortcut-terminal-auto-temporary / 0612d620757b4a69a22171a4dd5270bbc0af6fea | 734396c5；依赖上一条 | 保存的快捷命令仅在明确不支持续接时自动回退临时会话 |
| fix/site-terminal-retention / 3894440e442d98635dbc896b9646bac67781d69d | 6fd1038a；包含前两条 | 网站管理终端固定 k web、恢复同会话、启动意图防重复和关闭失败保留 |
| refactor/terminal-shared-core / 6bf20b53dea5cf3bbcb070b9267564dd40462ffb | fe984946/9cb3aa71/82bd018c；包含前三条 | 主机与任务共用有界传输/输入/尺寸控制，切换身份丢弃迟到回调，多查看器分别跟踪输出位置 |
| claude/terminal-command-history / abc42b8d50ae37a80377d490671b0d6a33be1b74 | 867d066f/6f7b2bfe/b467a9ed/149b33c2/3c57925d | 全 Panel 命令历史、回填/搜索/删除/清空、抽屉页签记忆、浅色可读性 |

五条分支实为“终端核心组合”和“命令历史”两组，重叠部分由发布任务重放融合，未重复纳入前三条。额外集成修复包括历史接入共用核心、Mock 网站启动兼容、隐私提示、批量空格排除及 psql 提示符排除。已发布的编辑器、桌面、本地应用和 HTTPS 原作者分支不重复计作本轮新增；原作者工作树及本地引用保留所有权。

## 存储与隐私边界

按用户授权同步上位规范与 storage-strategy 1.6 的窄例外。只保存终端页面识别的命令及显式批量命令，不保存 PTY 原始流或回放。全 Panel 一份，最多 200 条去重命令、单条 8 KiB、文件 4 MiB；按最近执行淘汰，保留至淘汰或管理员删除/清空。目录0700、文件0600、原子替换；损坏停用并保留原件，由显式清空恢复。

历史正文不写审计/日志、不进入面板备份；删除/清空仅审计意图、结果和条数。命令可能含秘密，界面和升级提示说明本机保存、空格排除与删除/清空。交互及批量路径统一拒绝前导空白记录；常见菜单/密码与 psql 默认提示符已覆盖。任意自定义提示符不能保证区分 shell 与程序，不把启发式当完整秘密检测。旧版回滚忽略历史文件，不迁移或删除。

## 独立复核、OCR 与跨仓库联动

- 独立存储/源码 R1 发现批量 trim 绕过空格排除、psql 被当作 shell，以及上位规范引用歧义；修复后 R2 PASS，原失败和复现保留。
- 发布浏览器 R3 首轮发现不存在的 small 状态选择器和隐藏 batch 重复 toolbar；修复为连接状态 class 与可见控件后 R3 PASS。脚本复核不冒充实跑。
- 版本组装 R1 PASS：仅六文件，唯一 Go 差异为开发版本字符串，四项更新与三条升级提示准确。其他提供商 CLI 无可用入口，按规则使用独立干净 Codex 会话并记录 fallback，不以工具品牌代替独立性。
- R2 原件SHA256：97e48f71c43f9dddf25c8731da1253f4380ead7320bc55363652be8007d7c462；R3：04dc7607a331887845cccb2897be1073741acab3a2f9b9de76ddce8e1f2dbda9；组装R1：ed8d914fbc9d1971468f69d526a669bcf9408b0b7d8cafb5f78fc1f89d0859fc。
- OCR1.12.11：业务/发布测试共62/62文件，另3份文档人工复核；组装4/4及2份排除文件人工覆盖。先自由臂再约束臂，无新增 constrained-only；独立复核发现不计 OCR 收益。元数据提交不改已绑定源码；不宣称完整稳定周期收益。
- CF not-run，为可选辅助；未补审、未豁免、不作为发布前置，历史状态不改写。
- scriptLinkageState=not-required（无需发布脚本（不适用））：沿用既有 k web 与终端契约。内置脚本 commit160924a498d9b8d071fed72f165d23c1b9e886de，SHA25639837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1；无脚本候选或应用市场改动。

## 自动门禁、依赖与原始证据

- 本地终端核心119项、组合91项、隐私修复后定向46项通过；修复前 Windows 全Web2987通过，不冒充最终组合的全量结果。最终 Linux 全Web2986通过、6平台专属跳过，总2992；前端构建通过。
- terminalhistory全Go包与Panel精确定向通过。首次宽泛正则误选 AIHistory 分配测试并遗漏历史包测试，原失败保留；精确选集后通过，不将其写为首轮成功。
- 本地完整治理328/328及尾项通过；后来事实基线刷新单独检查通过，最终精确 Linux CI 执行全部后置门禁。
- 业务候选CI39003777788与主线CI39004579629均success；主线治理回执明确绑定原候选CI。
- 冻结候选push CI39003815944与依赖新鲜度39003815919均success。正式流水线candidate CI、L3、promote和A1的main CI均success；main CI于05:36:34Z完成。Release、公开产物/真实浏览器及preview-completion均success；可复用release内的L3 job按预览路径skipped，实际完整L3已在外层同轮通过，不是省略L3。
- 完整L3：2026-10-17T05:04:10.221Z–05:18:35.439Z，candidate=A1、status=passed，GitHub-hosted ubuntu24/ImageVersion20261012.334.1、Go1.27.2、Node24.21.0、Docker28.0.4。
- 权威入口SHA256 c286f7f2ad692ea17b8ec0537fe2a84e8d049d22adec63a2b00ceba39b165d57；source tree6640c7afc3fb620692479447a7e4f011577c5c70，identityUnchanged=true。三lane均passed，source总555732ms；Go555710、Web323335、deploy1955，jobs2/GOMAXPROCS2。
- L3 artifact12090947231，ZIP595302bytes，服务器SHA25670a6630da7f544ec029ee88f59fc81ad428c0f161da2151549cf84822e798ee6；CRC与6个解包文件核实。发布者回读核验27份rawEvidence摘要/bytes/exit及12份actual/original闭包，三lane日志摘要重算匹配。
- 首方Go SHA25670172a6c3fa4c043e90f2a100f77c08c1d799b196793f197495804c60b09c454（957文件）；graph24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927；closure37d5d5bf81705ac2eb8d8c71eed2ea045c3bb8e83cc7189d1b8bbec58a756095；vendor3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580。
- 110 production modules扫描exit0无finding；selected exit1保留10项、standalone exit1保留26项，共36既有finding及2026-10-25到期规则不变，不能宣称全依赖图无漏洞。既有TypeScript/Node维护例外及2026-10-23复核期限不变。无依赖、构建工具或基础镜像升级。
- 原始证据根：C:\GitHub\_release-evidence\v1.27.0-rc.6。日志下载最初60秒限时不足，原失败/部分ZIP保留；确认持续传输后使用有界300秒流式下载并核验CRC，不重跑远端门禁。

## 浏览器与多维质量

组合Mock绑定clean E859：17项通过、页面错误0，记录历史采集/回填、空格排除、刷新单一会话、网站终端恢复、浅深主题、中英及390/768/640视口；新历史主文字14px、元数据13px。1280下CSS zoom2的全局裁切截图不计为浏览器缩放通过，640仅为1280在200%的布局等效；原生zoom、读屏和实体手机未验证。首次无效状态选择器超时保留；修复后实跑通过。预览为模拟数据，不证明真实宿主机或脚本互通。

真实公开产物浏览器已验证146项（published45、upgrade31、initial44、restart26），四份原report均passed/mocked=false/errors=[]，窗口05:53:42.729Z–05:54:15.070Z。真实PTY执行、刷新同session、关闭后输入404，以及新增历史采集/隐私提示/仅回填/清空、Panel重启持久化/单条删除/清空读回均通过。RC5后端配RC6 Web实际读取RC6版本/index/更新摘要/升级提示，1280/390/320弹窗无横向溢出，真实无systemd的503失败关闭队列并允许无需刷新切回稳定；不把预期失败注入当作真实安装成功。

原始public artifact12091103612，ZIP2916846bytes，SHA2566eff69e52915961725e36184650d56dbacbeced1bd2e985d8fd6fbceb4d506ee；CRC、53文件/3643546展开字节核验。三份清理回执与权威脚本一致，测试只清理自有容器/网络。10张PNG逐张查看：历史真实内容/回填/隐私提示、PTY、版本弹窗三个宽度、预期失败及桌面恢复；桌面截图处于文件列表加载中，只证明桌面窗口/语言恢复，不能证明列表加载成功。公开browser对比使用RC5旧后端，不声称所有混合节点兼容。

固定测试配方绑定controller E859与product A1，recipeSHA2568e07c6dda1815cf80973a4311cdb60eeb037bb7d6ebbc03c8da499801c93e77f；实际Runner ID保存在test-runner.json，受控Docker库存/测试CA fixtures明确标注，Panel/Agent未挂宿主Docker/systemd socket。真实混合版本节点、完整LDNMP网站入口、应用任务多视图长期负载、宿主systemd更新/回滚、原生ARM等未在本轮重新实机验证。性能资源仅对现有限额、失败与生命周期自动回归负责，不宣称低配长期P95/RSS验收。

| 维度 | 状态 | 证据与边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 精确L3与真实PTY/历史/公开API；混合远端节点及完整LDNMP未验证 |
| 网络入侵与供应链安全 | 已验证 | 独立隐私复核、认证/Origin/CSRF回归、110production modules零finding；36既有selected/standalone finding及CF未运行不隐藏 |
| 稳定性、失败恢复与兼容 | 已验证 | 页面续接、关闭确认、Panel历史重启、TLS故障/离线恢复和503恢复；宿主systemd/所有旧节点未重验 |
| 性能与资源预算 | 已实现未实机验证 | 有界输入/输出/历史及回归通过；无低配长期P95/RSS或负载验收 |
| 用户体验与可访问性 | 已验证 | Mock17项与公开146项/10PNG、窄屏与浅深中英；原生zoom、读屏、实体手机未验证 |
| 数据、配置与迁移 | 已验证 | 有界历史原子存储、损坏恢复回归、真实重启和删除/清空；无原始PTY落盘、历史不入面板备份 |

## 公开产物、通道、归档与回滚

[RC6 Release](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.6) 已公开，draft=false、prerelease=true、非Latest。version/preview OCI index为`sha256:eb9ee91fd9df911dd1b0dd0163507efbdc5a543909c59593f7d059135296b24d`；linux/amd64为`sha256:b3ab04725e2408f9bb5ccf2327283b1f1a8d2b9520de73b1d5a572ebabae2a9d`，linux/arm64为`sha256:e1760da979c8aaea28ebbc2c56f57f5fe8c5a03648aa4c34a04da39fe4316dc2`。

15附件下载/API digest/磁盘回读、11项SHA256SUMS、两架构config/source/version/内置脚本标签一致。公开Release正文与面板API/实际弹窗均核对4项更新摘要、3条升级提示和同一镜像index。SBOM与SLSA原件保留；发布者回读17份provenance rawUTF8重算摘要/大小并复用权威校验器核对两架构subject/build args。原SLSA Git materials为absent-in-original，身份绑定为原始build args与runtime config标签，不补造Git材料。修改组件附件SHA2565a5df599855dd5a3f47c0d21806e3ee194603b4696be3bf2682936f2ba598e8f；与publication/public原件字节一致。

发布前snapshot artifact12091576658于05:46:49Z上传，早于镜像推送及05:52:31Z公开；ZIP709bytes，SHA25635f20cc1c5b82426727ae731de671855937993df72447fa5eb734acd6756208b。publication-identity绑定A1/E859/run39004865035/attempt1/before-publication，stable-before字节SHA256126c8f577b80143589ac260066641c1934d539571637e63d5228fc7f11575c0a；与publication/public两份原件一致。publication artifact12091985331 SHA256706ffccc975bc72a973306f825f462b1f572404e79d344c8bac94f2716c297e1；本轮无需只读恢复或重复发布。

生产动作不适用（预览版禁止生产部署）；本轮未连接、备份、部署或核对任何自有服务器。稳定 GitHub Latest v1.26.0、Dockerlatest sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9 已由公开验收对发布前原快照重新核实未变化。加入预览不启用自动安装、不请求安装；同版本不提供重装；退出不提供降级，均已实测。宿主systemd更新/备份/回滚本轮未运行。

回滚输入为RC5/c8ee07a82775549ab37aeefcc1a43982be24a3c0、镜像sha256:8e00585d30dc94eea361fdc0d4d362b292496b51ca1922f82b51f65ad51a2a94。回滚部署未执行；退出预览不自动降级。需要回滚时使用配套版本、先备份面板配置；历史不在面板备份中，旧版忽略文件，删除需求应在新版显式清空。

owned业务分支`feature/v127-rc6-integration`原tip E859已以原子双端expected-SHA lease保存到`archive/feature/v127-rc6-integration`并移除活跃远端，读回复核一致。local upstream已切到archive；本地clean工作树及4174 Mock仍运行，供体验保留，不继续写归档。五条原作者refs/工作树保持所有权，已纳入映射见上，不强删或当作新增待发布；待原作者释放后再处置。唯一release候选保留A1，未归档RC列车。

验收文档候选只改本记录、当前业务事实和既有提案；自身候选/main CI及文档分支归档终态放仓库外最终交付回执，避免提前写未来结果。文档树无前端依赖安装；本地C卷可用约66.6GB，预览/冻结候选/原始证据与未知所有权资源保留，本轮未清理，净释放0bytes。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-17T12:32:38+08:00
- 候选冻结时间：2026-10-17T12:46:12+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（预览未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：17
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

正常独立审查发现的产品隐私缺陷单列质量证据，不计为流程异常；GitHub正式root全程成功。下列本地准备/取证异常及旧候选失败保留；只读探索命令的路径/字段猜测不冒充门禁执行或计作重复发布。helper引号风格首次定位失败也保留在原工具会话，不计为必需发布步骤无效。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "qualification/go-targeted/overbroad-regex",
    "position": "before-production-write",
    "count": 1,
    "impact": "宽正则误选无关AIHistory分配测试，且历史包无匹配测试。",
    "recoveryEvidence": "go-targeted-r1.log；精确r2与terminalhistory全包通过。",
    "permanentAction": "仅使用精确目标测试名，最终Linux完整门禁另行核实。",
    "historicalReleases": []
  },
  {
    "fingerprint": "integration/helper/default-gbk",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地融合helper因默认GBK解码中止，冲突标记进入中间候选，未公开。",
    "recoveryEvidence": "原工具会话；fc609修复后node --check及后续完整门禁通过。",
    "permanentAction": "文本helper显式UTF-8；保留中间提交，禁止当作已验证候选。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/history-regex/unicode-quote-escape",
    "position": "before-production-write",
    "count": 1,
    "impact": "Unicode正则引号转义导致首次测试SyntaxError。",
    "recoveryEvidence": "history-fixes-r2.log；8a修复后history-fixes-r3.log46项通过。",
    "permanentAction": "修复无效转义并保留定向回归。",
    "historicalReleases": []
  },
  {
    "fingerprint": "observation/go-module/proxy-connectivity",
    "position": "before-production-write",
    "count": 1,
    "impact": "本地缺失module.info且proxy.golang.org连接失败，首次闭包观察无效。",
    "recoveryEvidence": "observe-source-r1.log；r2及source-observation.json成功。",
    "permanentAction": "本地有界重试使用GOPROXY=https://goproxy.cn,direct；未修改仓库依赖或GitHub入口。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/governance/vendor-checkout-bytes",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次治理被30个vendor CRLF字节差异拦截。",
    "recoveryEvidence": "governance-r1.log；vendor-byte-repair.json按HEAD及pin恢复；r2治理328项和尾项通过。",
    "permanentAction": "只按已核验HEAD blob恢复并刷新stat；源码差异为空，不更改pin。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/browser/nonexistent-status-selector",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次Mock浏览器等待不存在small连接状态元素超时。",
    "recoveryEvidence": "browser-local-r1.log；修复为实际状态class及可见toolbar，独立R3和Mockr3通过。",
    "permanentAction": "连接状态使用实际DOM class，排除隐藏重复toolbar。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/browser/css-zoom-clipping",
    "position": "before-production-write",
    "count": 1,
    "impact": "1280视口CSS zoom2截图裁切，不能作为原生200%缩放有效证据。",
    "recoveryEvidence": "早期browser-local截图保留并排除；r3使用640布局等效且明确原生zoom未验证。",
    "permanentAction": "验收显式区分CSS/布局等效与浏览器原生zoom，不补写通过。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/review-trailer/separate-paragraphs",
    "position": "before-production-write",
    "count": 1,
    "impact": "各trailer分段造成OCR状态提醒stale；旧候选CI主动取消。",
    "recoveryEvidence": "原工具会话；39003575818保留cancelled；E859连续canonical trailer recorded。",
    "permanentAction": "提交前核对interpret-trailers输出，不改写已推送SHA。",
    "historicalReleases": []
  },
  {
    "fingerprint": "qualification/business-context/stale-baseline",
    "position": "before-production-write",
    "count": 3,
    "impact": "旧候选GitHubCI被50提交基线过期拦截；本地两次误将RC用作稳定基线版本，被parser拒绝。",
    "recoveryEvidence": "integration-ci-r1-failed.log和原工具会话；恢复v1.26.0稳定基线并刷新精确复核提交；39003777788/39004579629通过。",
    "permanentAction": "保持稳定版本字段语义，仅刷新已实际复核的commit和日期。",
    "historicalReleases": []
  },
  {
    "fingerprint": "collection/github-logs/local-timeout",
    "position": "before-production-write",
    "count": 4,
    "impact": "本机三次60秒日志取证超时，不能据此判断远端门禁失败。 完整root日志持续传输3879534bytes后300秒超时，原partial及receipt保留。",
    "recoveryEvidence": "原工具会话保留下载失败；integration-ci-logs-r4.zip及receipt、CRC和SHA回读通过。 rc6-root-logs-r1.zip.receipt.json；后续900秒有界下载，最终回执核对CRC。",
    "permanentAction": "小run使用300秒、完整多job run使用900秒有界流式下载，统一限制100MiB；不重跑远端；诊断探测不计必需步骤。",
    "historicalReleases": []
  },
  {
    "fingerprint": "collection/public-evidence/windows-esm-import",
    "position": "before-production-write",
    "count": 1,
    "impact": "发布者回读helper首次用Windows绝对路径ESM import，未进入断言。",
    "recoveryEvidence": "原工具会话保留ERR_UNSUPPORTED_ESM_URL_SCHEME；file:///修正后verify-public-evidence-r3.log通过。",
    "permanentAction": "Windows ESM使用file:/// URL；无远端门禁重跑或产品修改。",
    "historicalReleases": []
  },
  {
    "fingerprint": "collection/public-evidence/cleanup-text-assumption",
    "position": "before-production-write",
    "count": 1,
    "impact": "第二次回读helper错误假设清理回执含clean，而原回执为removed owned containers only。",
    "recoveryEvidence": "原工具会话保留AssertionError；核对两个权威shell脚本及三份原回执后精确匹配，r3通过。",
    "permanentAction": "按原脚本定义校验回执，不凭关键词猜格式；保留失败，不将其当远端清理失败。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
