# KPanel v1.26.0-rc.19 发布验收记录

发布级别：L3。releaseChannel=preview，releaseTrain=1.26.0。批准main基线a06ab4ce25464f7632e7a26160ff10e88a36af0c，冻结产品34666bb4225b1aaefb270108eb856ba06b74dbe7。[v1.26.0-rc.19](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.19)于2026-10-14T14:57:43Z公开，draft=false、prerelease=true，产物已发布，生产未部署。

## 发布画像与范围

通知为Panel原生能力，新增邮件、Bark、ntfy、Gotify、Discord、Slack，保留Telegram、飞书、钉钉、企业微信；各渠道独立启停、测试、投递和重试，历史展示逐渠道状态。界面保留草稿、协调渠道/规则保存、解释错误与恢复。安装许可继承失败时仅允许交互用户明确接受许可，非交互继续拒绝隐式授权。无新宿主机权限、端口或Agent动作。

| 来源 | 精确tip | 处置 |
| --- | --- | --- |
| feature/shoutrrr-notifications | 876dd1766346ef479080a167aabd40906e6e3e47 | 完整runtime树纳入，测试假Slack样例改为TTEST/BTEST/TEST。 |
| fix/issue-29-license-inheritance | 384e729f6f4dece17a23cfaba385c590146adde0 | 两个packaging文件逐字节纳入。 |
| fix/issue-30-allinssl-desktop-trash | 393c70c26ac7566281dc538b436358277d76ae7e | 作者撤销，净差异为空，不纳入。 |

来源任务冻结前均idle、最新对话completed。GitHub push protection拒绝原历史中的测试假Webhook，未绕过扫描；独立发布工作树以批准基线重放最终树。原历史保存于push-protection-original.bundle，SHA256 863a17c6a97e94b3e38165830d2279ace67432cfc1c99773303ed2aab5f43a39。两来源tip不因重放而宣称标签祖先，按文件blob等价与bundle恢复证明交付。本地作者分支、工作树和预览保留所有权；已交付等价内容不再计新候选。预览候选分支保留；旧拒推集成历史单独归档，不覆盖作者内容。远端最终refs及本地处置见final-disposition.json。

systemd已在RC9交付，offers为既有内容，旧文件快捷方式图标、旧分页/文件页布局仍排除；既有历史替代分支与未完成审计文档不重新作为新功能。

## 业务、安全与数据边界

复用既有原子0600配置、有界通知历史、审计、规则和集群来源；每平台一个配置，最多10渠道。首次编辑惰性迁移，读取不写入；旧凭证损坏可显式替换恢复。每轮最多8次发送、最多3个渠道并行，失败最多24小时重试，已成功渠道不重复；服务检查意图按来源ID进入同一有界历史。短时worker使用固定当前可执行文件，以stdin传凭证、空环境、丢弃输出、父进程超时；只允许6个适配器、HTTPS/隐式TLS及有界HTTP响应，SMTP执行时限由父进程限制。完整源身份4d776b3a594b368046bbd69efdc2cf304f8a2d00db1b026392cc17612921e07e，生产闭包60198f6b9f1d8ac9c6583094cdc6d67c5f7ad06c906655c862468cc3ff42b3f7；保留既有36项处置与2026-10-25到期日。

CF覆盖decision=scoped-required，fullAgeDays=23，未审计105提交/229文件，精确范围见security-coverage-final.json。预览只记录，本轮未执行CF scoped/full，稳定提升前待补，不能称CF审计通过。OCR1.12.11自由臂先于规则臂，42/42路径已reviewed，源候选精确文件复用作者证据，发布者复核边界/组合及假样例修复；无成立的新发现，不将其他独立复核归功于OCR。源码实现与最终发布验收由不同任务承担，同provider使用已声明provider-unavailable回退。

## 跨仓库联动判定

scriptLinkageState=not-required（无需发布脚本（不适用））。没有新增脚本协议、运行时动作、宿主机布局或镜像内脚本内容；安装变更修复既有许可继承失败恢复，不依赖新脚本能力。实际内置脚本1800d955f216aebd2776674368a8e469a671c489 / SHA256 5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c保持。应用市场已有独立提交8b8d0c452924a2b11f57d3e5836c8e2decc805e9，与本候选kpanel.conf blob均cb3740a4319c19b2820586390bf407451c374544；本轮没有修改脚本或重复推送apps。

## 自动门禁与依赖

唯一L3入口run-release-l3.mjs，run ID v1.26.0-rc.19-34666bb-l3-r2，arena-154，Runner kpanel-go127-security-runner:go1.27.2-node24.21.0 / sha256:4edff59fa4c1441e3e97594593b255cc57a74e51dabbdd8db295e4ff51d0ca4f。完整verify-release通过，raw终态、计划/script/bundle摘要、evidence.sha256及逐项核验位于本版l3-r2/remote-evidence。原中断r1不当作通过。

候选CI38576206855、主线CI38580480405、Release workflow38582819200均success，精确head为34666bb4225b1aaefb270108eb856ba06b74dbe7；依赖检查单独保存。源码检查、核心race、Go/Node依赖扫描、容器/脚本/自更新场景、安装生命周期、镜像契约、SBOM/provenance按完整入口执行，不能用局部测试替代。草稿和实际公开Release正文均通过TestRenderedReleaseNotesForPublication，4项更新摘要、3项升级提示；live面板更新API和弹窗未验证。

新增生产依赖Shoutrrr v0.8.0、fatih/color v1.19.0、go-colorable v0.1.14及既有go-isatty v0.0.24进入闭包；6平台闭包/上游checksums独立观察与L3资格化保留。Go1.27.2、Node24.21.0、基础镜像/Actions/scanners未升级。每日依赖报告以本轮Dependency freshness与发布日志为证据；未知/EOL行动项不凭未报错推断不存在，旧风险/复核到期日不延长。

## 隔离环境、浏览器与多维质量

environment-policy arena-154，允许candidate/browser validation；正式服务未部署。两个后台browser作业均passed、exitCode0，绑定精确产品SHA，spec哈希及始终态保留。Playwright1.62.1/Chromium151.0.7922.34；十渠道、独立保存/暂停、错误保留输入、草稿切换、键盘Tab/Escape通过。三语言×两主题×1280/1024/640/390/320视口共30组无横溢、输入按钮最小14px，125%/200%为桌面等效CSS视口，非原生缩放。实际PNG已独立查看。额外195px窄屏仅为探索性几何记录，不能当作手机200%视觉舒适证明。真实平台联通、实体手机、WebKit/读屏、原生ARM运行、长时间压力和真实WAN未验证。SMTP仅隐式TLS。

| 维度 | 状态 | 证据与限制 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 自动契约/Mock独立渠道与apps blob；真实平台连通未实机验证。 |
| 网络入侵与供应链安全 | 已验证 | 完整自动扫描、源码与闭包资格化；CF scoped待补。 |
| 稳定性、失败恢复与兼容 | 已验证 | 核心race、有界失败重试与恢复夹具；不保证旧读取器接受新通知字段。 |
| 性能与资源预算 | 已验证 | 10渠道/8批次/3并发/24小时及worker上界；无长期压力结论。 |
| 用户体验与可访问性 | 已验证 | Chromium核心交互及30组布局；原生缩放、读屏未验证。 |
| 数据、配置与迁移 | 已验证 | 惰性迁移/只读读取/损坏恢复；降级备份与恢复升级前配置历史。 |

## 公开产物、自更新通道与回滚

版本与preview OCI index均sha256:56a596329f1489c7195d104facb8686f1caacbe70b3efa1fa40ed4891e82fc36；双架构Registry实际version/revision与冻结提交一致，各manifest见public-image-metadata.txt。GitHub Latest仍v1.25.1，Docker latest仍sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325。15附件落盘重读大小/API摘要通过，SHA256SUMS11项通过；公开不可变镜像E2E为image_e2e=pass。回滚点RC18 / 53ff5667f0ee9f12924d8930ef02fc305d0b0e23 / sha256:b139b32d3ae8f7e60ee6da0e8cfc7102e833a184b3658ee862de6a752f41c8bb；旧Panel不保证读取新multi-channel状态，回滚前须备份，必要时恢复升级前通知配置与历史。

既有自更新自动测试覆盖稳定/预览来源、加入仅检查、安装独立开关、通道保持、退出不自动降级、更新备份与失败隔离；公开正文解析通过不等于live更新弹窗验证。继承RC17 Panel/Agent配套与旧反代proxy_read_timeout360s要求；OpenRC/轻量Node边界按release-channels.md。

## 生产部署与清理

不适用（预览版禁止生产部署）。prod-108/108禁用全部KPanel操作，本次未连接、未备份、未部署、未核对。没有生产写入、回滚、紧急热修复或重发已公开版本。自有Mock/浏览器/隧道/E2E已清理；清理范围仅含可再生自有副本/依赖/新浏览器缓存，保留原始失败/成功、脚本/spec/截图/锁、bundle、可信Runner和公开资产；作者工作树/预览不动。自动审批拒绝删除的中断r1本地临时目录保留，没有换用其他删除方式。卷观察净释放3893841920字节，非Windows C盘净释放声明。证据根C:/GitHub/_release-evidence/v1.26.0-rc.19，公开附件原件/root/kpanel-release-evidence/v1.26.0-rc.19-public-assets/。复用既有流程，没有新增工作流或知识文档。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-14T19:18:40+08:00
- 候选冻结时间：2026-10-14T21:57:08+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：9
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "release/ready-before-clean/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "read-only contract rejected dirty intermediate version edits; rerun passed",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Use qualified explicit paths and standalone artifacts; no gate bypass or claim of global repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/windows-esm-path/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "qualification helper import required relative module URL; source hash repair completed before freeze",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Use qualified explicit paths and standalone artifacts; no gate bypass or claim of global repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/local-go-path/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "release parser initially failed absent Go PATH; explicit installed Go path then passed",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Use qualified explicit paths and standalone artifacts; no gate bypass or claim of global repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/github-push-protection/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "fake Slack test URL blocked old author history; new dedicated tree replay with noncredential fixture; no bypass",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Use qualified explicit paths and standalone artifacts; no gate bypass or claim of global repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/preview-dependency-missing/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "new worktree npm ci required and completed before preview",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Use qualified explicit paths and standalone artifacts; no gate bypass or claim of global repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/invalidated-r1/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "intermediate L3 interrupted before remote execution after push rejection; full frozen r2 replaces it",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Use qualified explicit paths and standalone artifacts; no gate bypass or claim of global repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/browser-script-auto-review/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "Long shell-embedded browser helper rejected by automatic approval without specific reason; standalone artifact written and executed successfully",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Use qualified explicit paths and standalone artifacts; no gate bypass or claim of global repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/scratch-cleanup-auto-review/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "Native recursive deletion of interrupted own temp tree rejected by automatic approval without specific reason; target retained with original complete recovery bundle, no alternate delete attempted",
    "recoveryEvidence": "Original tool output retained; exact final qualification and CI/browser/L3 receipts preserved in RC19 evidence",
    "permanentAction": "Retain blocked directory; deletion not necessary for release",
    "historicalReleases": []
  },
  {
    "fingerprint": "release/archive-upstream-absent/rc19",
    "position": "before-production-write",
    "count": 1,
    "impact": "Archive helper requested unset-upstream on an owned branch which already had no upstream; Git returned a fatal diagnostic and made no change.",
    "recoveryEvidence": "Exact rejected integration SHA preserved in local archive and qualified bundle; canonical candidate/archive update succeeded atomically. Final upstream inventory confirms no old upstream.",
    "permanentAction": "Read upstream before optional unset; no retry or alternative mutation required.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
