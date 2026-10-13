# KPanel v1.26.0-rc.17 发布验收记录

发布级别：L3。releaseChannel=preview，releaseTrain=1.26.0。批准 main 基线 c347edb19e0abf8decc94bde89754a088ef8cb7b；最终冻结候选 5116c36f73124141183dfc4b2d4b60834eec7d1c。[v1.26.0-rc.17](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.17)于2026-10-13T14:28:18Z（22:28:18+08:00）公开，draft=false、prerelease=true、非GitHub Latest。注释tag对象5bfc2b06f3180f5a36b3632ff665268dda2d2069指向最终冻结SHA。产物已发布，生产未部署。

## 发布画像与候选处置

业务域为文件分享、图库、手机桌面监控及终端入口。大文件匿名内容证明扩大资源预算，风险由完整 L3 和现有鉴权/内容绑定/超时/撤销回归覆盖。其他差异为前端加载生命周期和布局。没有 Schema、端口、配对协议或 Agent 权限变化。旧一次性上传仍为512MiB；新分享10GiB、五分钟校验、64KiB缓冲、匿名两路并发、大文件全局最多两次校验/分钟，最坏读盘20GiB/分钟。HEAD/Range/304同样先完整校验，不能保证无等待下载。

| 来源分支 | 精确 tip | 本次内容 |
| --- | --- | --- |
| fix/file-share-large-files | fcf3a2d63eedae473302c3adbd2e5914430e7ba6 | 10GiB内容绑定分享、独立旧上传限额、504可重试反馈。 |
| fix/gallery-mobile-image-recovery | bd671f2dded151ebfba33849d0f4116eead4b980 | 延迟地址/旧事件隔离，Blob解码失败恢复原生图片URL。 |
| fix/mobile-monitor-overflow | c51d894670d27f63479ac622afd05e8a6a9369dc | 手机负一屏长主机名、流量与指标换行增高。 |
| feature/terminal-split-workspace | a54a38e7ff77b96500c135cb2742f0a0db7637bc | 手机隐藏分屏布局入口；桌面分屏主体已随RC14/RC15发布。 |

systemd服务管理已随RC9交付，offers为既有功能。旧快捷方式图标、分页和文件页旧布局继续排除；旧独有分享主题、历史替代分支和未完成审计文档不作为新产品内容。来源工作树保留其独立预览和作者所有权，不擅自删除。唯一release/v1.26.0-candidate在RC公开后按策略保留。

## 跨仓库联动

scriptLinkageState=coupled，变更集file-share-large-files-20261013。官方Nginx模板2677b00aeb578a76ac0557413b4e9dd699e29050已先发布，公开URL字节SHA-256为9ce68093700f9c9afa99fba4c2403769cb53f516bbe30e6ffe042bcc2fd88430，本任务重新核对远端main与公开字节一致。模板将proxy_read_timeout设为360s，原模板回滚点05f5a2eac269967706f30dc3ff7985339e1f3ce4。既有站点需由管理员应用、nginx -t和重载，模板公开不自动改变旧站点。

kejilion.sh源码和协议未改变，内置revision1800d955f216aebd2776674368a8e469a671c489 / SHA-256 5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c。应用市场origin/main的kpanel.conf与候选packaging文件blob均9d2d2efdc334db1ba014203370fdc8281f1a5870，不需新提交，默认latest。

## 安全、源码身份与未验证范围

覆盖检查decision=scoped-required，102个边界提交、219个文件未覆盖；预览版记录不阻断，稳定版前须补审。本次未运行新的CF scoped/full。来源独立复核与OCR证据保留，不冒称CF审计。与已独立复核fcf3a2d6相比，候选Go源码只额外改变版本字符串；最终firstPartySourceSha256为5335d33a07555d96fa002a4fbe4863e841ac9a36717ac8971c37543e88390ee2，918个源文件；closureSha256仍485cc7f1758b7186cf3305c9d5365667bc2116769593f6f6faaccfabc54785b8。36条既有处置与2026-10-25到期日不扩大；完整L3须重新核验实际/原始上游闭包与所有扫描器。

来源已有10GiB稀疏文件身份、HEAD、续传、304、同大小改写拒绝与隔离Nginx静默65秒证据；不是实际非稀疏慢盘/WAN性能。实体iPhone、真实远端PTY、旧反代代理链、真实宿主systemd写入、生产升级/回滚和live更新弹窗未验证。无生产部署，未连接prod-108。

## 自动与浏览器门禁

L3 外层唯一入口 run-release-l3.mjs，run ID v1.26.0-rc.17-5116c36-l3-r2，于2026-10-13T13:26:58Z—13:51:14Z执行，status=passed、exit_code=0，12项远端原件与本地SHA-256逐项匹配。计划a7dfb99b1e8a1cb48db01b65fed5e6e6957d19454a6cadb87455c3199b021def、bundle2169ce295f1a47f41b553b9c39860aa7510899cfc3934a29686581e7a84b21c2、执行脚本21c08b11be3526a0fe766aa606a81d0e4047e8a4e89d79227da4e62914164979。Runner ID sha256:4edff59fa4c1441e3e97594593b255cc57a74e51dabbdd8db295e4ff51d0ca4f，Go1.27.2、Node24.21.0。

源码并行总耗时1042715ms。Go全量、核心race/vet，前端290文件/2752通过/6跳过、类型、语言资源与构建通过；Linux双架构、非root只读镜像契约、安装安全、应用生命周期与更新/备份/中断恢复夹具通过。实际源码与原始上游govulncheck无可达漏洞，require-only信号保留；生产闭包106模块无保留发现。Trivy selected/standalone原有发现继续按原观察处置保留，不能声称全部依赖图零风险。最终镜像扫描通过。

候选CI [38407250207](https://github.com/kejilion/KPanel/actions/runs/38407250207)与依赖检查[38407249063](https://github.com/kejilion/KPanel/actions/runs/38407249063)通过。同一SHA快进main后，主线CI [38409207771](https://github.com/kejilion/KPanel/actions/runs/38409207771)与依赖检查[38409208385](https://github.com/kejilion/KPanel/actions/runs/38409208385)通过。。Release工作流[38411407750](https://github.com/kejilion/KPanel/actions/runs/38411407750)及标签依赖检查[38411407883](https://github.com/kejilion/KPanel/actions/runs/38411407883)全部通过。。

草稿说明经check-release-notes.sh使用实际面板解析器验证，5项用户摘要、3项升级注意事项，未扩大展示上限。VERSION、version.go开发版本与两个npm元数据保持1.26.0-rc.17一致；最终正文仍由Release门禁检查。

## 组合浏览器与视觉证据

本地acceptance visual-composition预览绑定冻结SHA，Mock端口4180。arena-154的非root、只读根、cap-drop ALL、1GiB/1CPU/512PID容器通过仅回环SSH隧道读取该Mock，不访问真实Panel/Agent数据。r2的monitor子作业14场景、terminal子作业7场景退出0、各自passed报告与清理证据保留；r2整组因其他夹具失败仍为failed，没有改写总状态。手机监控覆盖320/375/430px、中英文、浅深色、等效100%/125%/200%缩放、503错误态和键盘焦点。终端覆盖经典/桌面、两主题、窄屏、窗格顺序、布局缩放、配额及关闭失败后重试，未执行真实PTY命令。

分享独立后台share-r3为passed、exit0，1280/390px验证504提示、重试、创建、停止分享和键盘退出。现代图库后台job为passed、exit0，spec SHA-256 d233c6f00e180379e0c52aeae80e11630ad964a44601e68e50f7a3a3c5794a73；Playwright1.62.1、镜像sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e。WebKit和Chromium在390×844下分别覆盖触摸打开、受控延迟无空src、损坏Blob后的原生URL恢复、真实内容失败后重试、JPEG/PNG、前后切图和关闭。两引擎主图矩形均为屏内390×292.5，截图中心平均亮度约127，实际PNG已复核，避免仅凭DOM解码通过接受未绘制主图。页面errors为空。

早期Playwright1.55.0、非资格化原图/时限、隐式语言、glob路径、瞬态加载窗口与沉浸工具栏交互造成的失败或无效证据完整保留；不把原失败改写为通过。最终结论来自上述明确通过的精确候选与工具组合，不宣称旧引擎整组成功。截图复核涵盖手机监控、分享超时/成功、手机终端、桌面窗格与现代图库。实体iPhone、实际浏览器原生125%/200%缩放、读屏、真实代理链和长期负载仍未验证。

## 多维质量状态

| 维度 | 状态 | 已覆盖证据 | 限制 |
| --- | --- | --- | --- |
| 业务正确性 | 已验证 | 文件分享内容绑定/失效、重试与界面旅程；现有协议兼容 | 10GiB稀疏正确性不能替代慢盘/WAN实测 |
| 安全 | 已验证 | 自动扫描、现有鉴权与内容绑定回归、独立源码复核 | CF scoped仍待稳定版前补审；既有条件风险未关闭 |
| 稳定性 | 已验证 | 超时/取消、重试、备份中断恢复与生命周期夹具 | 无生产长期运行结论 |
| 资源预算 | 已验证 | 缓冲/并发/期限/读盘预算有界 | 无真实非稀疏10GiB吞吐结论 |
| 用户体验 | 已验证 | 精确组合Mock、触摸/键盘、语言/主题/等效缩放、截图绘制 | 无实体手机和读屏结论 |
| 数据迁移 | 不适用 | 无Schema/协议迁移，Panel与Agent配套更新要求写入说明 | 旧反代站点需人工应用360s等待窗口 |

## 公开产物与资源收尾

版本镜像与preview均为sha256:52104ff51d76f06eff973ddbf86f89b29c19742c50828fee803fe3b7b2cab80a。linux/amd64 manifest为sha256:394182bc0221680f9386841c5935a49c0cbd8e9e06fb64dd4a247ed1f4faa873；linux/arm64为sha256:09bf9007317edb27f9a155887093ecffd33d62bc583d514d5f6d12eb546f4c3d。两个Registry配置blob的version=1.26.0-rc.17、revision=5116c36f73124141183dfc4b2d4b60834eec7d1c逐项匹配。GitHub Latest仍v1.25.1，Docker latest保持下述稳定基线。公开正文实际字节再次交给面板解析器测试，非空摘要与三项升级提示通过；未执行live Agent发布信息接口/更新弹窗，不能以正文或测试代替。

arena-154显式拉取上述不可变公开index，冻结源码的packaging/tests/image-e2e.sh执行image_e2e=pass、exit0，覆盖公开版本/健康、代理头、静态资源字节与bootstrap。结束后E2E容器/网络均0、18080无监听。非root/只读根/权限和资源契约还由Release固定运行时门禁覆盖；没有宿主真实管理员写入或生产升级证明。

公开15个附件已在arena-154逐项下载并从落盘文件重新读取SHA-256；15/15长度与GitHub API摘要匹配，SHA256SUMS的11项全部匹配。验证脚本限定官方版本URL、文件名和大小，下载三路并发、有界读取并保留失败文件。receipt为release-asset-verification.json、sha256sums-verification.json；原件在/root/kpanel-release-evidence/v1.26.0-rc.17-public-assets/，许可与修改组件元数据也已保存本地证据。

四个来源tip已逐项确认是RC17标签祖先（4/4），远端无对应活跃来源引用；可由不可变tag按表中SHA恢复。本地作者工作树及其预览不删除，保持独立所有权，已纳入内容不再计待发布。唯一release/v1.26.0-candidate按预览策略保留在5116c36f。纯验收记录另走docs/v126-rc17-acceptance候选/主线CI后归档，不改公开tag。自有fix/v126-rc17-business-review已在同SHA主线CI通过后本地保存到archive/fix/v126-rc17-business-review=5116c36f，其clean且无忽略/嵌套内容的工作树已通过git worktree remove回收，正式tag与main可恢复。

本任务Mock预览、隔离浏览器与仅回环SSH隧道已停止；当前E2E资源清理已确认。仅回收本任务可再生的L3源码/产物副本、浏览器clone、两套node_modules与本次新拉取的1.62.1浏览器镜像缓存。原始失败、脚本/spec、截图、报告、npm锁文件、固定bundle、可信Runner归档和公开附件原件全部保留；未清理其他项目、作者工作树或生产实例。arena-154卷空闲空间实测净增4130930688字节，清理后可用32873971712字节；详见owned-scratch-cleanup.json。

证据根目录C:/GitHub/_release-evidence/v1.26.0-rc.17/。上一预览回滚点v1.26.0-rc.16 / 77cdfb4b5b0dd510f7610e822f8e33663c0ba24a / sha256:3f79dc7686e8c1ad015b4b8698535f0425e819e901e659f42eec907427605a17。稳定默认v1.25.1 / sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325。


<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-13T17:39:22+08:00
- 候选冻结时间：2026-10-13T21:25:20+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：15
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

流程计数按独立失败步骤或无效证据分类，并非公开发布次数；全部发生于生产写入前。本轮未重发任何已公开版本，原始失败、首次根因与恢复证据均保留。部分旧浏览器失效的单一根因未确认，记录中不作确定归因。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "l3/run-release-l3/stale-business-context",
    "position": "before-production-write",
    "count": 1,
    "impact": "offline bundle preflight refused stale business baseline at62commits;candidate tests not started",
    "recoveryEvidence": "source-prepare.json retained;5116c36f refresh reviewed62commits/101files;freshness passes at reviewed78c87bed baseline",
    "permanentAction": "Canonical context freshness gate retained;current candidate baseline and publicRC16 facts refreshed without changing thresholds",
    "historicalReleases": []
  },
  {
    "position": "before-production-write",
    "impact": "草稿说明首次校验因Windows PATH未包含已有Go而退出127，未创建标签或产物；历史RC13有同类Go PATH根因，RC不是historicalReleases字段要求的稳定标签，故在本字段明示。",
    "historicalReleases": [],
    "fingerprint": "release-notes/check-release-notes/missing-local-go-path",
    "count": 1,
    "permanentAction": "RC13存在同类Go PATH根因（不同入口），本轮再次明确固定既有Go1.27.2路径和本机说明解析能力；不隐瞒复发。预览无生产写入；后续生产L3前须将工具资格归并到唯一适配入口并补回归，不能把本轮恢复称为项目级永久修复。",
    "recoveryEvidence": "既有C:/GitHub/_codex-deps/go1.27.2/go/bin加入当前命令PATH；同一check-release-notes入口退出0"
  },
  {
    "position": "before-production-write",
    "impact": "本地后台浏览器启动命令被自动审批拒绝，未执行；工具仅返回blocked by policy",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/local-start-policy-rejected",
    "count": 1,
    "permanentAction": "本次浏览器执行方案固定登记arena-154，不新增或绕过环境策略",
    "recoveryEvidence": "改用仓库登记arena-154环境与隔离浏览器容器的原始background-browser-test入口，state与spec保留"
  },
  {
    "permanentAction": "本次发布冻结已资格化的浏览器镜像、夹具、输入、可见/绘制断言和后台spec；原失效证据保留。通用仓库入口沉淀及旧引擎兼容性作为稳定提升前跟进，不放宽门禁或声称旧轮通过。",
    "recoveryEvidence": "r2保留全部断言、使用远端时限，14+7场景通过",
    "position": "before-production-write",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/remote-time-budget",
    "impact": "r1 monitor55s与terminal120s分别终止",
    "count": 2
  },
  {
    "permanentAction": "本次发布冻结已资格化的浏览器镜像、夹具、输入、可见/绘制断言和后台spec；原失效证据保留。通用仓库入口沉淀及旧引擎兼容性作为稳定提升前跟进，不放宽门禁或声称旧轮通过。",
    "recoveryEvidence": "独立share-r3显式zh-CN，两个视口全部通过",
    "position": "before-production-write",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/implicit-locale",
    "impact": "r1/r2分享复用脚本依赖系统中文，远端默认英文导致中文按钮定位失败",
    "count": 2
  },
  {
    "permanentAction": "本次发布冻结已资格化的浏览器镜像、夹具、输入、可见/绘制断言和后台spec；原失效证据保留。通用仓库入口沉淀及旧引擎兼容性作为稳定提升前跟进，不放宽门禁或声称旧轮通过。",
    "recoveryEvidence": "明确路径正则且强制requests非空，后续现代引擎实际执行故障注入",
    "position": "before-production-write",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/legacy-glob-matching",
    "impact": "r1/r2旧Playwright的files**未覆盖内容路径，故障注入未命中",
    "count": 2
  },
  {
    "permanentAction": "本次发布冻结已资格化的浏览器镜像、夹具、输入、可见/绘制断言和后台spec；原失效证据保留。通用仓库入口沉淀及旧引擎兼容性作为稳定提升前跟进，不放宽门禁或声称旧轮通过。",
    "recoveryEvidence": "使用受控Promise释放响应，spinner/无空src断言先完成再放行",
    "position": "before-production-write",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/transient-loading-assertion",
    "impact": "r3固定350ms加载窗口在远端不能稳定捕获spinner",
    "count": 1
  },
  {
    "permanentAction": "本次发布冻结已资格化的浏览器镜像、夹具、输入、可见/绘制断言和后台spec；原失效证据保留。通用仓库入口沉淀及旧引擎兼容性作为稳定提升前跟进，不放宽门禁或声称旧轮通过。",
    "recoveryEvidence": "原件保留；显式1.62.1镜像、1440px夹具和可见区域/绘制断言在两引擎通过，不以旧轮结果宣称Safari成功",
    "position": "before-production-write",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/unqualified-legacy-webkit",
    "impact": "r4/r5/r7旧WebKit出现指针稳定性或页面关闭，未确认单一产品根因",
    "count": 3
  },
  {
    "permanentAction": "本次发布冻结已资格化的浏览器镜像、夹具、输入、可见/绘制断言和后台spec；原失效证据保留。通用仓库入口沉淀及旧引擎兼容性作为稳定提升前跟进，不放宽门禁或声称旧轮通过。",
    "recoveryEvidence": "按既有Tab唤回交互再点击；现代两引擎完成关闭、重试和切图",
    "position": "before-production-write",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/idle-control-not-revealed",
    "impact": "r6查看器沉浸隐藏工具栏后脚本直接点关闭，被stage拦截",
    "count": 1
  },
  {
    "permanentAction": "本次发布冻结已资格化的浏览器镜像、夹具、输入、可见/绘制断言和后台spec；原失效证据保留。通用仓库入口沉淀及旧引擎兼容性作为稳定提升前跟进，不放宽门禁或声称旧轮通过。",
    "recoveryEvidence": "现代两引擎增加实际截图像素亮度与屏内边界断言，主图亮度约127并实际复核PNG",
    "position": "before-production-write",
    "historicalReleases": [],
    "fingerprint": "browser/background-browser-test/screenshot-paint-proof-gap",
    "impact": "r4 Chromium DOM断言通过但截图主图未绘制，视觉复核拒绝该证据",
    "count": 1
  }
]

<!-- kpanel-release-process-incidents:end -->
