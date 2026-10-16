# KPanel v1.27.0-rc.4 发布验收记录

结论：RC4已公开发布，GitHub-hosted完整L3与公开产物/真实浏览器恢复验收通过，独立最终复核PASS。发布止于Docker Hub及公开产物，未部署生产。原正式run跳过公开验收的事实保留，后续只读恢复补齐；不将原run绿色状态冒充全链路验收。

## 范围、版本及授权

用户明确要求发布全链路迁移至GitHub并终止于Docker Hub，不提供自有服务器方案，实际发布下一预览版。本轮不部署生产、不提升stable/latest。RC4保留RC2已交付的本地应用、HTTPS、共享偏好和终端功能，仅改发布基础设施、经过实测复核的依赖例外及版本元数据。

- 原迁移基线654435896f14fb693bc94ce20aba9888d235290a，原迁移主线304c9a7e8ea3da73b8a2c0c8ffc62246dbcb67fa。
- RC3候选9b0575bfb131975caf74b98e4e7ca72b4a7693a1；tag对象05b3d97c71ef185f00824c9426933f4fe3b985e3。该尝试在Release输入校验失败，仅创建tag，未公开Release或镜像。原tag不改写，失败记入本轮事件。
- 修复主线7824838a8a3162154b6c2544f35232c6afce7ee1；RC4组装226698ee196ee136cecc22a7bc0f7ef129058190；最终候选e20f427d6ee14aac38c8656e39aa3d61e3934dc0与组装tree相同，末次为空的独立/OCR复核trailer留痕。
- RC4组装相对782只改VERSION、CHANGELOG.md、internal/version/version.go、web/package.json、web/package-lock.json和scripts/torrent-backport-scan-scope.json。唯一Go差异是发布版本字符串，未添加业务功能。

## GitHub发布链路

main的Publish preview接收冻结候选SHA，依次执行候选CI、完整L3、原子快进main与创建注释tag、同源main CI、Release附件与双架构Docker Hub镜像、独立只读公开验收。GITHUB_TOKEN push不会递归触发后继工作流，故使用workflow_call显式衔接。发布工作流、脚本、Makefile及测试夹具必须先集成main；冻结前核对候选tip、main基线、递增RC版本及相同SHA候选push CI。原有SSH/WSL发布执行入口退役，本轮未连接自有服务器。

RC3失败[38925832702](https://github.com/kejilion/KPanel/actions/runs/38925832702)证实GitHub runner默认GITHUB_REF_NAME覆盖同名job env，虽然日志env列显示预览tag，实际shell解析main^{commit}。修复改为RELEASE_TAG贯通所有发布tag操作，CI/L3在shell中显式export冻结候选身份。新增回归执行实际workflow shell并模拟caller main，19项定向测试通过；独立R6核对所有替换及边界。

正式RC4运行[38933320001](https://github.com/kejilion/KPanel/actions/runs/38933320001)的head_sha为控制面782，产品checkout、L3、tag与OCI来源必须为e20，不能混用。版本镜像发布前必须权威确认未占用，已存在或查询不确定均拒绝覆盖；推送串行。公开验收为独立只读job，失败可重跑该job，不重建已存在的版本镜像。原始L3日志、分组结果、公开回执及截图保存为Actions artifacts。


公开验收跳过源于隐含success()受被跳过的祖先L3影响；原根运行的success不撤销也不冒充验收通过。后续943a96b3仅修复治理工作流：显式状态条件、preview-completion检查及单一只读release-public-verify.yml入口。独立恢复校验现有e20/RC4/d2镜像，禁止重推产品镜像或改写tag。恢复snapshot为依据既有稳定基线及原Release日志PREVIOUS_PREVIEW_VERSION/IMAGE重建的输入，非找回的原stable-before.json；恢复已实读当前stable/preview与公开资产并确认相符。

## 已完成资格化及正式门禁

| 精确提交/检查 | 实际结果 |
| --- | --- |
| 原迁移304候选CI38922029685、完整L338922029102、依赖新鲜度38922029088 | success；L3回执passed |
| 原迁移304主线CI38923996401、主线新鲜度38923996423 | success |
| RC3候选push CI38922919265、新鲜度38922919341 | success |
| RC3根运行38925832702 | 候选CI/L3/promote/mainCI成功，Release输入失败；无Release/镜像 |
| 修复782候选CI38931258721、完整L338931258389、新鲜度38931258751 | success；L3回执passed |
| 修复782主线CI38933309819、主线新鲜度38933310000 | success |
| RC4 e20候选push CI38931539310、新鲜度38931539357 | success |
| 公开验收治理修复943候选CI38941793410、完整L338941793715、新鲜度38941793798 | success；L3精确943回执passed，2026-10-16T17:43:17.331Z–17:55:46.790Z；原始ZIP摘要复核通过 |
| RC4只读恢复38943483548 | success；实际public assets/OCI及128项浏览器检查通过，未进行publication写入 |
| RC4正式运行38933320001 | candidateCI、L3、promote、mainCI、publish/release成功；整体success，但public-acceptance被跳过，不能据此宣布全链路验收通过 |

RC3实际L3为15:52:44.229Z–16:05:21.405Z，source三lanepassed、identityUnchanged=true；修复782 L3为16:21:36.393Z–16:36:15.974Z。均使用GitHub-hosted ubuntu24/20261012.334.1、Go1.27.2、Node24.21.0、Docker28.0.4。唯一release-gate入口摘要c286f7f2ad692ea17b8ec0537fe2a84e8d049d22adec63a2b00ceba39b165d57。上述证据分别绑定其实际提交，不能替代RC4 e20的完整L3。

RC4实际L3为2026-10-16T16:49:10.636Z–17:03:33.857Z，回执candidate=e20、status=passed。source三lane均passed，identityUnchanged=true；总554061ms，Go554038ms、Web307639ms、deploy2123ms，jobs=2、GOMAXPROCS=2。实际Ubuntu24 ImageVersion20261012.334.1、Go1.27.2、Node24.21.0、Docker28.0.4；三个lane日志摘要逐一复算匹配。原始日志提取12份actual/original target stdout，字节数、摘要与exit0逐一验证。正式mainCI为17:03:54Z–17:12:22Z成功；publication为17:12:25Z–17:28:51Z成功。公开验收原job在17:28:52Z被跳过，恢复验收38943483548为2026-10-16T17:56:13Z–17:57:49Z，全部步骤success。原发布run仍保留skipped事实，恢复未重推镜像。

## 源码、依赖及风险边界

RC4首方Go源码摘要be026eb8acd52159f33380f8ba72e7a0642415b1d97fe6d1958ab0b9ff862c70；selected graph24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927；closure98ac00a5cdff85d5e3369149a0e5fc5f3c7e3ec2f73e8cb8ea50ff0469896cae；vendor manifest3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580。110productionmodules、6targets/12actual-original closures、36既有findings及2026-10-25到期规则保持；RC4完整L3中canonical inventory已实测相符，12份原始target输出摘要已重新计算核对。scope的review.evidenceSha256绑定真实R6字节，R7独立核对版本组装及摘要；不据此省略最终canonical inventory或扫描器。

两项工具链例外经实测与独立复核续审至2026-10-23：TypeScript7.0.2配vue-tsc3.3.11及3.3.13均报ERR_PACKAGE_PATH_NOT_EXPORTED typescript/lib/tsc，原始安装锁及日志保留；官方Node index确认Node24为LTS、Node26为Current，保持当前LTS runtime及对应声明。未修改31天上限、检测源或每日审计。之前三次freshness exit3原件保留，不回写RC2历史结果。

无需发布脚本（不适用），scriptLinkageState=not-required。受管脚本kejilion/sh@160924a498d9b8d071fed72f165d23c1b9e886de，SHA-25639837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1；安装、数据、配置及应用市场契约未变。应用市场dirty工作树未修改。

CF not-run，未补审、未豁免、不声明CF覆盖完成；按现行规范为可选辅助。未验证原生ARM硬件、实体手机、真实systemd/宿主升级和长期负载。公开升级UI使用显式旧后端配新Web隔离夹具，不作为正常部署组合。

## 公开产物、浏览器及稳定通道

[RC4 Release](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.4)于2026-10-16T17:28:37Z公开，draft=false、prerelease=true。注释tag对象7739655f4832cd45d433adfec256599ad8d832f7，peeled=e20。Release正文及publication artifact给出image index sha256:d2d22e6b6e4320ae1f5541327f653b77c86b49316b4f1da497c3fa2d02fc3e12；独立只读[恢复验收38943483548](https://github.com/kejilion/KPanel/actions/runs/38943483548)已实读version/preview均为该digest；GitHub Latest v1.26.0及Docker latest E0保持，15附件逐字节摘要和11条SHA256SUMS均通过。两架构OCI version/revision以及受管script revision/hash均绑定e20和既有脚本。Buildx生成SBOM/provenance，本轮未额外独立逐blob复核。

发布说明预期2条摘要/3条升级注意项，已用实际Go运行时parser验证，公开恢复的实际API和弹窗报告再次确认2条摘要、3条升级注意项。上一已发布预览为[RC2](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.2)，源7c3441d8560b4727de634d1a15dde7b776030af8，配套回滚镜像sha256:207aa4614c703914df69dcd2e1a392ed55f4864ce31d971bd910e02ee667f659。GitHub Latest v1.26.0和Docker latest sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9须发布前后实读相同。


| linux平台 | manifest digest | config digest |
| --- | --- | --- |
| amd64 | sha256:bec7b3a6c706f9d12e4b5f709daf61bfed97d38ddd13280483bface5f2dcde2e | sha256:2e6084ef46d584b6efcae717b14e94e32b05bf8bf40666f23d5bd1faef0f97ac |
| arm64 | sha256:56bc7dc916c5ce9243a69bd9808d78f55a91dcf7d1ed6e8b94f9cdfed647071d | sha256:9740ceac93c9db54fdc91c0b5258942f1255c24fbbfc768012e35f51863a8ccb |

公开真实功能initial42项、restart18项、published browser37项、upgrade browser31项，共128检查均passed，四份report的errors=[]、mocked=false。覆盖本地配置与图标、域名TLS边界、自签名与Secure cookie、偏好持久化、重启后的桌面恢复、真实PTY续接/关闭、preview订阅及退出、无systemd时真实503后界面恢复；升级弹窗1280/390/320宽度无横向溢出。8张截图已保存，人工复核320更新弹窗及桌面终端续接。fixture的旧RC2名称只作为固定测试ID，实际releaseProbe、version、candidate和digest均为RC4。Dockerinventory与测试CA为显式隔离夹具，不证明宿主Docker或公共CA部署；upgrade为旧RC2后端与新RC4 Web的显式UI夹具，不冒实际宿主升级成功。

实际临时测试runner imageID sha256:70cc9880f1738419cbdda91a02b2333da3a04ae23bbbe1884f7fbf97583171af，Playwright1.62.1与固定MCR digest dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e。三组cleanup.txt均记录仅移除各自命名容器/网络；测试Panel/Agent不挂载宿主Docker/systemd socket，GitHub runner为一次性环境。

Actions原始artifact ZIP下载后再次校验服务端digest：publication sha256:d943be24b65c83f2518aa8121f602097d90335166ccb4c6f78b9e6036a9986c0（123038bytes），RC4 L3 sha256:77e4cba0689572d003237c9bd27bb18b879d8dad8ea81a5ec1f71f760efcc81f（593567bytes），公开验收 sha256:e86fbc8e6395897052bee318a0c16f02e57b37cda223f8ab9cd87157dea7e9f9（2317381bytes）。四份真实report另行复算SHA，见证据根rc4-public-report-integrity.json。Release成功job的大纯文本下载超时文件仅partial，未冒充完整原始日志。

## 独立复核与资源收尾

不同provider CLI不可用，使用同provider独立干净会话，fallback=provider-unavailable。自由复核R1发现版本镜像重推风险、L3文档触发器及前版硬编码，R2–R4确认修复；R5核对RC3组装，R6核对实际输入失败修复，R7核对RC4六文件组装。R8独立核对公开验收被跳过的根因与四文件修复9a05100d，PASS。R9独立实际回执核对PASS：live GitHub注释tag/Release、publicZIP及50个解包文件摘要、四份128项真实报告、15资产与11校验项、双架构及稳定通道均相符。R9记录见证据根independent-review-r9.json；主线CI随后实际完成，见下方收尾。

OCR1.12.11：原迁移31/31、RC3组装4/4为blind=false，constrained-only unreported；后续身份修复6/6和RC4组装4/4先落自由臂、blind=true，均constrained-only=0；公开恢复修复4/4同样先落自由臂、blind=true、constrained-only=0。人工按规则逐文件处理，不把圈选覆盖率冒充独立源码/发布通过。验收文档不含产品代码，OCR skipped reason=acceptance-and-documentation-only。

本人治理分支feature/github-release-pipeline在精确943候选CI38941793410、完整L338941793715和mainCI38943470499全部success后，以expected-SHA lease原子保存至archive/feature/github-release-pipeline=943a96b30e61a1c3fa8bda19a53168e745124b6d并删除活跃远端引用；远端实读确认。该最后治理修复纳入main，在RC4产品tag之后，不声称943属于已发布e20镜像。主线新鲜度38943470507同样success。clean且无忽略独有文件、嵌套Git、reparse或活跃进程后，已用git worktree remove回收本人迁移树并将本地分支改归档名、upstream指向归档；逻辑文件55084289bytes，C卷实测净空闲增62685184bytes，剩余82528800768bytes（空间变化含同期系统活动，不冒充精确物理删除量）。恢复从archive精确943建立新任务树。

本验收记录为纯文档分支docs/release-v127-rc4-acceptance，基线943；只改本文件和README入口说明，依次接受本地verify-change、精确候选Linux CI和mainCI，之后原子保存archive/docs/release-v127-rc4-acceptance并回收自身工作树。自身最终提交SHA不能写进该提交正文，其实际最终tip、CI IDs及归档/清理核对记录在同证据根closeout-final.json；未完成步骤不得宣称完成。release/v1.27.0-candidate保留为唯一RC列车；旧作者未释放所有权的分支、dirty工作树及4175/4177预览保留。本人验收工作树曾目录更名被Windows拒绝，原目录C:/GitHub/_codex-tasks/kpanel-v127-rc3-acceptance完整保留，新分支名已改为rc4；不强制搬迁。

证据根C:/GitHub/_release-evidence/v1.27.0-rc.3-github-migration同时保存迁移资格化、RC3失败、修复及RC4原件。阶段scope分别交付：原迁移304使用654基线契约；身份修复782使用9b基线六文件契约；RC4使用782基线版本契约，不扩大原允许路径掩盖继承差异。复用现有release-kpanel工作流并更新为GitHub-only，README简介同步纠正终点，不另建平行发布制度。

## 节奏与异常

下方已记录异常按可核对的执行/根因计数：GitHub迁移资格化10次、正式RC3输入失败1次及本地输入/资格化/状态文件等12次，加公开验收跳过、部分日志下载超时与本机registry探测失败3次，再加验收schema两次与文档树换行一次拦截，共29次，生产写操作后0次。正常开发期规则编辑、预期负向安全夹具及被新提交取代的cancelled run不冒充成功，也不作为公开版本故障。原始日志或状态文件路径逐项给出；仅保存在执行会话中的早期诊断明确注明，未补造日志。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-16T22:56:29+08:00
- 候选冻结时间：2026-10-17T00:23:05+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：29
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "migration/release-workflow/unsupported-runner-context",
    "position": "before-production-write",
    "count": 1,
    "impact": "Release38919912762在workflow解析阶段拒绝job.env中的runner.temp，没有开始发布写入。",
    "recoveryEvidence": "failure-workflow-parser原始metadata及官方解析错误；改为step写入GITHUB_ENV，后续完整运行核验。",
    "permanentAction": "仅在step上下文读取runner.temp，保留workflow解析失败原件。",
    "historicalReleases": []
  },
  {
    "fingerprint": "migration/verify-governance/stale-release-contract-assertions",
    "position": "before-production-write",
    "count": 3,
    "impact": "本地governance-r1与GitHubCI38919915086、L338919915133分别拦截旧GITHUB_SHA及metadata文档断言。",
    "recoveryEvidence": "governance-r1.log、failure-ci-r1/failed.log、github-l3-r1-failed.log；对应定向测试与最终完整L3成功。",
    "permanentAction": "契约断言绑定实际RELEASE_SHA与现行文档，不削弱来源校验。",
    "historicalReleases": []
  },
  {
    "fingerprint": "migration/verify-change/workflow-trailing-blank-line",
    "position": "before-production-write",
    "count": 2,
    "impact": "CI38920771370和L338920771392均被release.yml末尾空行的diffcheck拦截。",
    "recoveryEvidence": "failure-ci-r2/failed.log和failure-l3-r2/failed.log明确new blank line at EOF；7326732删除空行，最终完整门禁通过。",
    "permanentAction": "提交前复用git diff --check，不放宽格式门禁。",
    "historicalReleases": []
  },
  {
    "fingerprint": "migration/check-business-context/stale-reviewed-baseline",
    "position": "before-production-write",
    "count": 2,
    "impact": "CI38920933681与L338920934022拒绝过期业务事实基线。",
    "recoveryEvidence": "github-ci-r3-failed.log与failure-l3-stale-business-context；55742f7据已验收RC2真实事实更新基线并独立R3复核。",
    "permanentAction": "更新已核实业务事实，不改变50提交/8阈值。",
    "historicalReleases": []
  },
  {
    "fingerprint": "migration/dependency-freshness/due-toolchain-exceptions",
    "position": "before-production-write",
    "count": 3,
    "impact": "38919915085、38920771301、38920933583均以exit3报告TypeScript7和@types/node26例外到期，9类检测源正常。",
    "recoveryEvidence": "github-dependency-r1.log和failure-freshness-r2/r3/failed.log；实际TS7/Vue试验与官方Node索引、独立R4核验后续审至10月23日，38922029088及主线/RC3新鲜度success。",
    "permanentAction": "例外到期据真实兼容性和LTS依据续审，不删除来源或修改期限上限。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/collect-torrent-baseline/missing-evidence-directory",
    "position": "before-production-write",
    "count": 1,
    "impact": "inventory-migration-r1因指定证据目录未创建而ENOENT，没有形成有效inventory。",
    "recoveryEvidence": "inventory-migration-r1.log；随后明确创建各次唯一目录，最终GitHub L3执行完整闭包检查。",
    "permanentAction": "先创建唯一拥有的证据目录，原失败日志不可覆盖。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/collect-torrent-baseline/vendor-crlf-source-mismatch",
    "position": "before-production-write",
    "count": 2,
    "impact": "inventory-migration-r2发现本任务检出vendor .deepsource.toml换行摘要不一致。 本轮验收文档树verify-change-r2同样拦截CRLF换行原始字节不符。",
    "recoveryEvidence": "inventory-migration-r2.log；仅恢复本任务30个vendor文件为Gitblob原始字节，无vendorGitdiff，后续GitHub完整L3通过。 docs-vendor-byte-restoration.json记录仅恢复本人文档树30个既有Gitblob字节，无vendorGitdiff；rc4-acceptance-verify-change-r2.log保留失败，r3重新完整治理验证。",
    "permanentAction": "保持vendor原始字节及现有摘要校验，不更新基线掩盖换行差异。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/collect-torrent-baseline/local-go-proxy-timeout",
    "position": "before-production-write",
    "count": 1,
    "impact": "inventory-migration-r3本机Goproxy取selectedmoduleinfo超时，不能声明本机12closure成功。",
    "recoveryEvidence": "inventory-migration-r3.log；保留本机失败状态，改由授权GitHub-hosted完整L3真实执行，没有对同因进行盲重试。",
    "permanentAction": "网络失败和验证结果分开，证据只绑定实际环境。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/task-preflight/missing-fixed-go-path",
    "position": "before-production-write",
    "count": 2,
    "impact": "一次本机治理预检未加载固定Go工具路径，被前提检查拦截。 修复回归初次调用亦漏加载GoPATH，3个Go契约测试因此失败；两次均不能视作产品失败或通过。",
    "recoveryEvidence": "原始诊断在本地执行会话，未另存独立日志，不补造原件；加载C:/GitHub/_codex-deps/go1.27.2/go/bin后复核，最终GitHub完整L3使用固定Go1.27.2。 reserved-identity-tests-r2.log真实19/19通过；之后完整governance-r2真实323/323通过。",
    "permanentAction": "调用本地Go验证时显式加载已批准工具链，不能用跳过检测替代修复。",
    "historicalReleases": []
  },
  {
    "fingerprint": "publication/validate-release-inputs/reserved-github-ref-overrode-tag",
    "position": "before-production-write",
    "count": 1,
    "impact": "RC3根run38925832702在Release输入校验失败，shell解析main^{commit}；候选CI、L3、mainCI成功，main/tag已推进，Release/image均未产生。",
    "recoveryEvidence": "rc3-release-input-failed.log与rc3-publication-failure-r1原始metadata；eb44/782修复，实际shell回归19/19与独立R6通过，后续RC4实跑待记录。",
    "permanentAction": "发布使用RELEASE_TAG；CI/L3在shell显式export实际候选身份；不改写RC3tag，递增至RC4。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/release-identity-regression/incomplete-runner-fixture",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次新增发布shell回归夹具遗漏GITHUB_RUN_ID，set-u真实拒绝；同次另有GoPATH失败，分别按根因计数。",
    "recoveryEvidence": "原始stderr在本地执行会话，未另存r1日志；补完整valid runner fixture，reserved-identity-tests-r2.log19/19通过。",
    "permanentAction": "测试执行真实workflow shell时显式提供其必需runner上下文，不关闭set-u或弱化校验。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/verify-governance/windows-state-rename-eperm",
    "position": "before-production-write",
    "count": 1,
    "impact": "Windows完整governance-r1 322/323通过，后台测试状态文件atomic rename因EPERM失败，不能称完整通过。",
    "recoveryEvidence": "reserved-identity-governance-r1.log和windows-state-rename-failed.json保留；受影响定向5/5通过，随后完整governance-r2 323/323通过。",
    "permanentAction": "隔离原Windows共享文件失败证据，不修改无关产品/后台runner；限定一次完整复核，Linux同源门禁仍必须成功。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/assemble-preview/windows-esm-absolute-path",
    "position": "before-production-write",
    "count": 1,
    "impact": "RC4外部组装工具初次以Windows C:裸路径ESM import，初始化即拒绝，未修改版本文件。",
    "recoveryEvidence": "原始stderr在本地执行会话，修正外部脚本为file:///URL后六文件组装及独立R7确认成功。",
    "permanentAction": "Windows ESM模块导入使用file URL；组装必须确认实际Gitdiff和canonical source hash。",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/check-release-notes/unsupported-flag-invocation",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次RC4发布说明校验误用flags，实际入口需要四个位置参数，被版本格式检查拒绝。",
    "recoveryEvidence": "实际入口scripts/check-release-notes.sh/render-release-notes.sh usage核对；rc4-notes-check.log按四参数真实Go parser通过。",
    "permanentAction": "先读既有CLI接口，复用workflow实际argv；不得用未执行的参数结果代替证据。",
    "historicalReleases": []
  },
  {
    "fingerprint": "closeout/git-worktree/owned-path-move-denied",
    "position": "before-production-write",
    "count": 1,
    "impact": "本人新建验收工作树更名时Windows拒绝目录move；分支已改为docs/release-v127-rc4-acceptance，路径保持旧rc3名称，差异原样保留。",
    "recoveryEvidence": "git worktree list、rev-parse和status实读确认旧目录/metadata/README差异及本人未跟踪草稿完整，无新目录；保留原路径继续，不强制移动。",
    "permanentAction": "目录命名不作为正确性门禁；Windows有打开cwd或句柄时保留路径，未强制删除或覆盖。",
    "historicalReleases": []
  },
  {
    "fingerprint": "handoff/task-preflight/inherited-release-outside-migration-scope",
    "position": "before-production-write",
    "count": 1,
    "impact": "把修复后的782整体与原迁移654基线比较时，继承的RC3 CHANGELOG被原迁移契约正确拒绝，不能声称该组合资格通过。",
    "recoveryEvidence": "contract-handoff-migration-r2.json失败保留；原迁移304的contract-handoff-migration.json已通过。修复按既定9b基线六文件契约contract-handoff-reserved-identity.json单独交付，实际L3日志摘要验证通过。",
    "permanentAction": "不放宽原允许路径；迁移、版本组装及后续修复分别使用其真实基线与既定scope。",
    "historicalReleases": []
  },
  {
    "fingerprint": "migration/public-acceptance/implicit-success-skipped",
    "position": "before-production-write",
    "count": 1,
    "impact": "RC4根运行38933320001整体success，但public-acceptance因默认success()传播被跳过，公开发布未完成真实验收。",
    "recoveryEvidence": "rc4-publication-with-skipped-acceptance/jobs.json保留skipped事实；R8和21项定向/325项本地治理通过；943精确GitHub CI/L3success；只读恢复38943483548success、128项实际检查零错误，无重推。",
    "permanentAction": "显式状态函数并增加preview-completion实际成功断言；独立只读验证现有产物，不复写版本镜像。",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/release-job-log/download-timeout",
    "position": "before-production-write",
    "count": 1,
    "impact": "本机下载117097829437大体积纯文本日志60秒超时，rc4-release-complete.log仅含前缀，不是完整日志。",
    "recoveryEvidence": "保留原部分日志及采集器，metadata和publication artifact已独立取得；前缀含真实原发布PREVIOUS_PREVIEW_VERSION/IMAGE。",
    "permanentAction": "明确partial，不将命名或部分日志冒充完整证据；优先Actions artifacts及压缩日志。",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/docker-registry/local-dns-connect-timeout",
    "position": "before-production-write",
    "count": 1,
    "impact": "本机Node公开registry探测DNS解析到不可连接地址后超时；同轮GitHub Release/main/tag API成功，registry未得到验收结论。",
    "recoveryEvidence": "rc4-remote-state-r1保留4个API原件；GitHub-hosted38943483548实际验证version/preview/latest、2架构和15附件，public-release.json status=passed。",
    "permanentAction": "公开验收统一在GitHub-hosted执行，以实际digest/assets/browser回执确认，不重复本机失败路径。",
    "historicalReleases": []
  },
  {
    "fingerprint": "closeout/release-metrics/preview-in-stable-history",
    "position": "before-production-write",
    "count": 2,
    "impact": "验收草稿将RC4预览tag写入仅允许稳定版的historicalReleases字段，独立metrics校验和同轮verify-change分别拦截；文档尚未提交推送。",
    "recoveryEvidence": "原rc4-acceptance-verify-change.log保留失败；standalone错误保留执行会话。删除错误字段中的预览tag但保留impact中的RC4事实，完整verify-change-r2重新验证。",
    "permanentAction": "historicalReleases仅引用稳定版，当前预览事实写在impact和正文；首次schema错误应先修复再跑后续门禁。",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
