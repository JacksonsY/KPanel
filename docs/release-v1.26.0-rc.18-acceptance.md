# KPanel v1.26.0-rc.18 发布验收记录

发布级别：L3。releaseChannel=preview，releaseTrain=1.26.0。批准 main 基线 1fc67d3e55886ad0d28ee566c48f0c4741155e66；冻结产品 53ff5667f0ee9f12924d8930ef02fc305d0b0e23。[v1.26.0-rc.18](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.18)于 2026-10-13T17:49:18Z 公开，draft=false、prerelease=true、非 GitHub Latest。注释 tag 对象 89211bc06b76ee8bc7b8f93329ff66360993ce29 指向 53ff5667f0ee9f12924d8930ef02fc305d0b0e23。产物已发布，生产未部署。

## 发布画像与范围

三个新增来源均在引用任务全部排队对话结束后冻结；source-queue-completion.json 记录该任务 idle、8/8 turns completed。发布者独立完成集成与最终验收，不召回已交付作者。

| 来源分支 | 精确 tip | 内容 |
| --- | --- | --- |
| feature/desktop-terminal-shortcuts | 56c507547ff18ed86ecda0c42741474d3cb3bbc9 | 导入终端快捷命令、本机/集群目标、独立脚本规格窗口、主机标题、共享悬浮选择器、Escape 草稿保护。 |
| feature/guides-more-videos | 79797bcfda3bbbc88ecc48ea52d4fdcced8be3d0 | 三种语言的 B站最新视频和 YouTube 教程合集外链。 |
| fix/gallery-mobile-viewer-20261013 | 0e4abb03ae866b929090fcc01063f524350d6067 | 手机关闭栏常显、视频适配、缩略图条与视频控件独立占位。 |

系统中心 systemd 管理已随 RC9 发布，offers 为既有内容；旧文件快捷方式图标、分页、文件页旧布局继续排除。历史替代来源和未完成审计文档不当作新产品。唯一 release/v1.26.0-candidate 保留在上述产品 SHA。三个来源 tip 已在标签祖先中逐项核对，远端无对应活跃任务分支；可从不可变 tag 按表中 SHA 恢复。本地作者分支、工作树和预览保持作者所有权并保留；这些内容已交付，不再计待发布。收尾使用独立 docs/v126-rc18-acceptance 分支；测试夹具修复与验收事实分别提交，组合 L1 候选待同 SHA 主线 CI 通过后归档，最终远端回执留在本版外部证据。

## 业务、安全与数据边界

命令是既有桌面工作区中的惰性数据，最多8192 UTF-8字节，保存不执行。显式启动复用既有鉴权终端 API；启动时固定主机和命令快照，重连、最小化恢复及刷新恢复不自动重复提交。远端不存在、不可用或撤权时不降为本机。remote-command 类型阻止旧 UI 把远端命令当成本机命令；来源旧 UI 兼容回归保留，但不等于旧 Panel 能读取全部新工作区类型。回滚旧 Panel 前须备份并按需恢复更新前工作区，不能仅凭 schemaVersion 未变承诺回滚编辑兼容。

存量 URL/file/directory 记录、Agent 协议、端口、身份配对、权限和安装流程未改变。当前 Go 变更仅两个生产文件、两个测试文件及版本字符串；无新增第三方 import、生产 GoFiles、生成 protobuf、软链接、模块图、构建目标变化。独立发布复核更新 firstPartySourceSha256=5647d7edc0b37d0051e3c8bc7ab0a482e5f979f9bd3bd0b723be408b0560d045，918文件；完整 L3 重算 closureSha256=485cc7f1758b7186cf3305c9d5365667bc2116769593f6f6faaccfabc54785b8 一致，36项处置及2026-10-25到期不扩大。

CF覆盖检查 decision=scoped-required，104边界提交/220文件未覆盖，fullAgeDays=22。本轮未新运行 CF scoped/full，预览按规范记录，稳定前须补审；不得把自动扫描或 OCR 称为 CF 审计。来源代码与 Escape 独立复核/OCR记录保留，跨提供商不可用的 fallback 已在源提交明示。源码原始/实际上游扫描、选中图和生产闭包扫描通过既有处置验证；实际可达扫描未发现可达漏洞，不声称整个 require-only/standalone 图零发现。

## 跨仓库联动

scriptLinkageState=not-required（无需发布脚本，不适用）。本轮仅复用已有终端 API，无新脚本协议、运行时动作、宿主产物、内置字节或外联配置改变。内置 kejilion.sh revision1800d955f216aebd2776674368a8e469a671c489 / SHA-256 5778fdc9637c8614f5246f9eb13c1e549de51522fb91b3805067cbc0475b614c。kejilion/apps 远端 main6602ac36e0c329e35b74ff621721b23583e94f5f 的 kpanel.conf 与本候选 packaging blob均9d2d2efdc334db1ba014203370fdc8281f1a5870，默认 latest、不需提交；保留无关本地检出。RC17旧反代站点360s等待要求继续适用。

## 自动与组合浏览器门禁

唯一完整 L3 入口 run-release-l3.mjs，run ID v1.26.0-rc.18-53ff566-l3-r1，2026-10-13T16:47:12Z—17:11:46Z，status=passed、exit_code=0；12项原件落盘摘要一致。Runner sha256:4edff59fa4c1441e3e97594593b255cc57a74e51dabbdd8db295e4ff51d0ca4f，Go1.27.2、Node24.21.0。计划 a9573a78f17c87442e4048bc5becc8cec05a4b90ddd0027a9bd26a3787d48ce2、bundle 424785e9771519082155d4b6629f37ab4a58e85166ada41d44288f0bfa0a629f、远端脚本 21c08b11be3526a0fe766aa606a81d0e4047e8a4e89d79227da4e62914164979。源码门禁1065302ms、identity_unchanged=true；前端294文件、2785 passed/6 skipped，Go全量、核心race/vet、类型/语言/构建通过。Linux双架构、镜像非root/只读运行时、安装生命周期与更新/冷备份/中断恢复夹具通过。

候选 CI [38433177764](https://github.com/kejilion/KPanel/actions/runs/38433177764)、依赖 [38433177645](https://github.com/kejilion/KPanel/actions/runs/38433177645)，同 SHA 主线 CI [38437070276](https://github.com/kejilion/KPanel/actions/runs/38437070276)、依赖 [38437070274](https://github.com/kejilion/KPanel/actions/runs/38437070274)，Release [38439451284](https://github.com/kejilion/KPanel/actions/runs/38439451284)全部成功。发布说明在冻结前及公开后使用面板真实解析器验证；4项用户摘要、3项升级注意事项，无绕过展示限额。

acceptance visual-composition Mock预览绑定精确产品，arena-154独立非root/只读1GiB/1CPU浏览器容器经仅回环SSH隧道访问。Chromium完成4组语言/主题/320至1280px视频入口检查及导入/保存/鼠标展开Escape保留草稿/双击启动/解码后恰好一次输入确认；实际截图已复核，默认窗口与脚本规格一致。WebKit完成4组视频入口、键盘表单/目标选择/启动及解码输入确认，r8额外等待window--open和opacity1并复核实际绘制的终端PNG。r7早期透明窗口截图不能作为视觉通过证据。r5/r6 WebKit菜单鼠标/触摸稳定性超时仍明确未验证，不用键盘通过替代指针结论。

图库Chromium/WebKit各15组几何布局检查，手机桌面窗口关闭/键盘退出/焦点返回、Chromium媒体失败退出通过。WebKit H264实际播放未确认：一张捕获帧后来显示不支持媒体提示；几何与关闭检查不当作播放证明。实体iPhone、原生缩放、读屏、真实远端命令执行、真实WAN及live更新弹窗未验证。来源视频入口页面blob与组合候选完全一致，其7组实际视觉证据可按范围复用；YouTube列表公共可用性未确认。

## 多维质量

| 维度 | 状态 | 证据与限制 |
| --- | --- | --- |
| 业务正确性 | 已验证 | 工作区/目标保留回归、命令一次提交和组合Mock；真实远端执行未实机验证。 |
| 安全与供应链 | 已验证 | 全部自动门禁及源码身份复核；CF scoped稳定前待补，既有条件风险保留。 |
| 稳定与恢复 | 已验证 | 迟到会话/失败关闭、目标不可用不回落、重连不重复、更新备份恢复夹具。 |
| 性能与资源 | 已验证 | 8KiB命令限额、既有会话预算和只读非root容器；无新长期压力结论。 |
| 体验与可访问性 | 已验证 | Chromium完整路径、WebKit键盘及绘制、响应式/三语言；WebKit指针/H264、实体机/读屏限制如上。 |
| 数据与迁移 | 已验证 | 新类型持久化及旧UI远端屏障回归；旧Panel读取新类型不保证，回滚需工作区备份。 |

## 发布后收尾测试夹具修复

原文档提交037ab4c6的候选CI38442727775成功，主线CI38443829388在既有MCP撤权测试失败，原始记录不改写。executeMCPOperation持久化claim并最多等待100ms后可返回executing；原测试错误要求立即得到failed。专用test提交ee92d730加入受控channel阻塞预检，使测试明确覆盖异步返回，再以8秒上界轮询持久化终态，检查authorization_revoked_before_dispatch和零Agent写入。受控旧测试两种路径均失败，修复后20轮通过；Linux race及组合最终CI以本版外部最终回执为准。

生产Go源码与已独立复核的RC18逐字节相同，没有新运行时修复、重新发布或镜像变更。当前main的一方全量源摘要因一个_test.go变化而资格化为cb1e63fa0d8bde0641ccc4156f62edd65611b60f2a5e9a28eecf7ed2907c4805；发布标签的原摘要5647d7ed仍保持正确。生产闭包、依赖、36项发现处置及到期日均不改变。源码资格化保留原生产独立复核，不把维护者自测称为独立审计。

## 公开产物与收尾

版本镜像与preview均 sha256:b139b32d3ae8f7e60ee6da0e8cfc7102e833a184b3658ee862de6a752f41c8bb；双架构实际 Registry version/revision 匹配。GitHub Latest仍v1.25.1、Docker latest仍sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325。15附件已在arena-154下载、从磁盘重读大小与API SHA256全通过，SHA256SUMS11项通过，许可/修改组件元数据保留。公开不可变镜像由冻结源码 image-e2e.sh 验证 image_e2e=pass；E2E独立容器/网络及18080已清理，不等于生产验证。

本任务Mock、浏览器和SSH隧道已停止。只回收本任务可再生克隆、node_modules、L3工作副本及新浏览器缓存；原始成功/失败、脚本/spec、截图、锁文件、固定bundle、可信Runner归档和公开附件原件保留。自有清理卷观察净释放 3916640256 字节，详见owned-scratch-cleanup.json；不清理作者工作树或无关项目。证据根 C:/GitHub/_release-evidence/v1.26.0-rc.18/，公开附件原件 /root/kpanel-release-evidence/v1.26.0-rc.18-public-assets/。

prod-108/108禁用全部KPanel操作，本次未连接、未部署、未核对。生产未部署；本次没有生产写入、回滚、紧急热修复或重发已公开版本。上一预览回滚点v1.26.0-rc.17 / 5116c36f73124141183dfc4b2d4b60834eec7d1c / sha256:52104ff51d76f06eff973ddbf86f89b29c19742c50828fee803fe3b7b2cab80a；稳定默认v1.25.1保持。复用现有流程，没有新增工作流或知识文档。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-13T21:42:13+08:00
- 候选冻结时间：2026-10-14T00:45:12+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：13
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

均为生产写前的独立失败步骤或无效证据，不是产品变更失败或公开发布次数。原始失败和受限结论保持；未确认根因不宣称已永久修复。原始工具输出及恢复回执在本版证据目录，以下按稳定指纹列明；RC17也有WebKit指针/绘制不足的相近现象，但工具与入口不相同，不推断单一同根因；稳定提升前继续复核。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "readiness/check-collaboration-state/owned-candidate-behind-main",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial role checkpoint rejected previous RC17 product branch before docs fast-forward",
    "recoveryEvidence": "Owned clean candidate fast-forwarded to exact approved1fc67d3e; ready/handoff passed",
    "permanentAction": "Perform approved baseline fast-forward before writer qualification; no gate weakened",
    "historicalReleases": []
  },
  {
    "fingerprint": "metadata/dockerhub/unqualified-windows-node-fetch",
    "position": "before-production-write",
    "count": 1,
    "impact": "Direct Node fetch failed connection while reading channel baseline",
    "recoveryEvidence": "Existing PowerShell Invoke-RestMethod succeeded; channel-baseline.json retained",
    "permanentAction": "Use bounded existing PowerShell/Linux curl transport; no claim of global DNS repair",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/desktop-loading-placeholder",
    "position": "before-production-write",
    "count": 2,
    "impact": "r1/r2 operated the async desktop loading placeholder before taskbar existed",
    "recoveryEvidence": "r3 onward wait for actual taskbar and icon readiness; Chromium full journey passed",
    "permanentAction": "Freeze UI readiness selectors in release-owned spec; retain both failed runs",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/webkit-gallery-pointer",
    "position": "before-production-write",
    "count": 1,
    "impact": "r1 WebKit click timed out during stability check",
    "recoveryEvidence": "r3 touch path passed15layout cases; source/full traces retained",
    "permanentAction": "Use qualified touch input and reduced motion; physical Safari playback remains unverified",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/input-base64-assertion",
    "position": "before-production-write",
    "count": 1,
    "impact": "r3 inspected JSON request text rather than decoding the existing base64 data field",
    "recoveryEvidence": "r5 Chromium and r7/r8 WebKit decode data and await200 accepted input response; exactly once assertion passes",
    "permanentAction": "Bind assertion to actual API encoding and response, not screenshot timing",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/reused-mock-shortcut-name",
    "position": "before-production-write",
    "count": 1,
    "impact": "r4 reused same mock name, leaving2matching desktop icons; pending response rejection obscured strict lookup",
    "recoveryEvidence": "Mock inventory independently confirmed two identical names; r5 onward unique run names and Promise.all manage both waits",
    "permanentAction": "Unique sample names per run and failure.json capture; no author data removed",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/webkit-menu-pointer",
    "position": "before-production-write",
    "count": 2,
    "impact": "r5 click and r6 tap timed out while menu actionability waited for stability",
    "recoveryEvidence": "r7/r8 keyboard activation passed with actual command submission; mouse/tap limitation remains explicit",
    "permanentAction": "Do not label pointer path passed; retain diagnostics and require follow-up before claiming Safari support",
    "historicalReleases": []
  },
  {
    "fingerprint": "browser/background-browser-test/window-paint-readiness",
    "position": "before-production-write",
    "count": 1,
    "impact": "r7 DOM assertions passed but screenshot showed a transparent not-yet-open terminal window",
    "recoveryEvidence": "r8 brings page forward, waits for desktop-window--open and opacity1, then captures actual terminal; PNG independently viewed",
    "permanentAction": "Require window-open/opacity readiness and visual inspection; original r7 screenshot remains invalid visual proof",
    "historicalReleases": []
  },
  {
    "fingerprint": "cleanup/owned-browser-scratch/crlf-final-command",
    "position": "before-production-write",
    "count": 1,
    "impact": "Cleanup mutations and receipt succeeded; final shell cat command carried a CR from Windows Set-Content and returned path-not-found. Subsequent scp succeeded; this is not a clean initial shell result.",
    "recoveryEvidence": "Original cleanup-browser.sh preserved. Independent exact SSH cat and local browser-cleanup.json agree; removed clone/node_modules absence confirmed; observedNetReleased3558674432 bytes.",
    "permanentAction": "Subsequent Linux helper files are written with explicit LF using Node; do not rerun the completed delete operation. No claim of global adapter repair.",
    "historicalReleases": []
  },
  {
    "fingerprint": "closeout/ci-race/mcp-async-preflight-terminal-state",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial documentation main CI38443829388 failed TestMCPRevocationDuringPreflightPreventsDispatch after the bounded100msAPIwait returned a durable executing claim; writes0. Runtime guard code unchanged.",
    "recoveryEvidence": "First failed JSON/log retained. Controlled channel barrier reproduces the old false failure for both local and delegated paths. Corrected test waits bounded terminal state, checks exact revocation error and zero writes;20targeted repeats passed. Final combined CI receipts retained separately.",
    "permanentAction": "Dedicated test-only commit ee92d730c8e71bc24537ff09b458ddf34a632c63 adds controlled preflight release and bounded terminal-state poll; no production Go path/hash changes versus published RC18. Do not weaken revocation/writes assertions or rewrite published artifacts.",
    "historicalReleases": []
  },
  {
    "fingerprint": "diagnostic/go-overlay/wrong-worktree-binding",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial controlled-red overlay mapped the dedicated fix worktree while Go ran from the docs worktree, so the intended barrier was not applied; successful baseline result is invalid red evidence.",
    "recoveryEvidence": "Original red.log/tool failure retained; red-r2.log runs from mapped fix worktree and reproduces expected executing/writes0 failure for both subcases. Corrected green and frozen20-repeat logs pass.",
    "permanentAction": "Bind overlay replacement absolute path and command cwd to same qualified worktree; controlled red expected failure is separate from publication gate failures.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
