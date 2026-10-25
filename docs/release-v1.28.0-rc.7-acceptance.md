# KPanel v1.28.0-rc.7 发布验收记录

日期：2026-10-26（Asia/Shanghai）

发布级别：L3。产物已发布，生产未部署。

候选提交 / 标签：f244152d9930bd65dc20241ce333ddf2d9626d97 / v1.28.0-rc.7；releaseChannel=preview，releaseTrain=1.28.0。

公开 Release：https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.7；Release ID=423561273；draft=false、prerelease=true，非 Latest。

版本及 preview OCI index：sha256:05514ee580e703bfb2abc8b0df457ce196ac6f025eb8d491a5ef1b34c12b50ca。

上一预览回滚点：v1.28.0-rc.6 / sha256:1c7e0f8c80cd9c09e15bc0139ce722bee358d8e97676625f5df8e947c2b37220；稳定默认 v1.27.2 / sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10，GitHub Latest ID=420434596。

## 发布画像与精确范围

业务域：文件传输显示、网站监控国家识别、虚拟机系统声明图标。展示与只读数据变化为主，虚拟机云创建新增已有 libvirt 元数据写入；版本发布为 L3。

| 来源分支 | 精确提交 | 归档引用 | 收尾 |
| --- | --- | --- | --- |
| fix/vm-system-icons | f354f0de45cc25498ca15970d79e78ec9912fdef | refs/heads/archive/fix/vm-system-icons | 已验证；保留源码工作树与证据 |
| fix/site-monitor-builtin-geodb | d647d5e0a4e9f43c32c711dad6749a22dfd6bc3d | refs/heads/archive/fix/site-monitor-builtin-geodb | 已验证；保留源码工作树与证据 |
| feature/transfer-rate-display | 14512f3a0cb65373bd6cb3b8b55dca02e6e1402d | refs/heads/archive/feature/transfer-rate-display | 已验证；保留源码工作树与证据 |
| fix/rc7-vm-icon-preview | 35b19d983e170e7ff8f79b9c69357d88ee413505 | refs/heads/archive/fix/rc7-vm-icon-preview | 已验证；保留源码工作树与证据 |

release/v1.28.0-candidate 保留在产品提交 f244152d9930bd65dc20241ce333ddf2d9626d97；本次单个 RC 不归档列车。来源提交均为产品祖先，无 squash 映射；归档引用逐一 SSH 回读验证，本地分支改为 archive/ 并解除原 upstream，物理工作树不删除。验收文档候选另在同 SHA 候选/主线 CI 成功后归档。

相对 RC6 主线1d1413fc，本次收录3项新增候选：实际字节速率与共用进度行；内置固定 DB-IP Lite 国家库；按声明显示虚拟机系统图标。scripts/mock-virtual-machines.mjs 的3行模拟数据差异单独形成可信前置35b19d98，通过候选 CI40213835058及主线 CI40214233475后纳入控制面，不在未信任候选执行新发布工具。8项预览工具测试通过。

279个登记工作树与上轮盘点比较，仅有上述3项新增未发布候选。备份外部导入、旧终端测试及旧版RC8验收记录的未提交内容原样保留，未纳入。既有来源端约60秒断流未修复；新需求不得加入冻结候选。

## 跨仓库联动判定

scriptLinkageState=not-required：无需发布脚本（不适用）。本版未修改脚本协议、固定版本、安装/卸载、应用市场默认入口或外联配置。内置已公开脚本 614d58cee228007012a2ec602cbdc8dbaeb7950d，SHA-256=ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9；Dockerfile及现有契约不变，双架构公开镜像标签均已核对。脚本候选与跨仓库变更集不适用。

## 多维质量与证据层级

| 维度 | 状态 | 证据与边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证（限定范围） | 当前精确候选真实 systemd Agent worker 安装、prepare、创建Debian客体、SSH登录、DNS/HTTPS出网、Docker共存、删除及磁盘清理9检查；未更改 kejilion.sh 业务协议。 |
| 网络入侵与供应链安全 | 已验证（门禁范围） | 同 SHA CI/L3、原始27条扫描/构建证据、SBOM/provenance、公开架构source标签；并非全产品无漏洞结论。 |
| 稳定、失败恢复与兼容 | 已验证（限定范围） | 原生工作线程、读取声明/失败回读测试、public restart与upgrade；来源端60秒超时及RC5一次未定位桌面恢复偶发仍 OPEN。 |
| 性能与资源预算 | 已实现未实机验证（长期预算） | 速率采样每任务最多64样本，过期重置且不新增API轮询；离线原始库增加8,348,509字节。未实测低配RSS、长时soak或吞吐提升。 |
| 用户体验与可访问性 | 已验证（浏览器范围） | 当前提交Mock28项；真实公开镜像150项。390/1280px、浅深主题、CSS100/125/200%、键盘/焦点、三语言与失败/空态；原生缩放、读屏和实体设备未验。 |
| 数据、配置与迁移 | 已验证（限定范围） | 可选osId，无持久化迁移；自定义MMDB优先、损坏库显式invalid、移除回退、无config目录也能离线识别。历史未知地区不回填；声明不能证明客体手动重装后的实际系统。 |

## 独立复核与审查范围

发布接手者与三项主要实现任务分离，实际提供商codex，精确模型未暴露/null；无其他提供商非交互入口，记录provider-unavailable降级，不虚构模型或跨提供商复核。OCR1.12.11组合范围35b19d98..15fdec3b，34/34文件；自由臂先落盘，free-form=0，constrained-only=0。来源国家库候选的constrained-only=unreported保留，不追改成0。

internal/contract 既有盲审记录无可信发现；组合范围安全覆盖为recorded。未执行CF，未执行新的全产品安全补漏；不把受限普通复核写成全面安全审计。原虚拟机命名空间P1在来源767044e6已修复，四份真实libvirt9.0 test-driver Debian/Ubuntu修复前后结果已核对；本轮另有精确候选原生KVM资格。

来源原件继续保留：VM首轮Linux L2曾发生terminalbatch 1秒清理时序失败，随后隔离复验通过，未把未定位偶发写为根因关闭；国家库作者的单次Linux L1调用因Docker socket权限退出1，后续只分阶段补齐，不能写成作者一次完整调用成功。当前组合重新通过同SHA托管CI/L3，来源Windows工具、Linux副本前提及浏览器定位/主题修正均与产品门禁分层，不将其错误原件覆盖。

## 自动门禁与公开产物

- 冻结候选push CI40214454933：success，同SHA f244152d9930bd65dc20241ce333ddf2d9626d97，有效Verification-Level:L2。
- 发布workflow 40215313371/attempt1：success；候选CI、完整L3、主线/tag提升、主线CI、Release、公开验收全部真实成功。Release内部L3 skipped复用本run同SHA外层L3，不代表省略。
- L3入口为GitHub-hosted scripts/run-release-l3.mjs --github-hosted；receipt、runner、工具、源码三分组结果与摘要独立验证。 Test Files  323 passed (323);       Tests  3198 passed | 6 skipped (3204)；image_e2e=pass。四个必需公开报告mocked=false、errors及assetFailures为空，共150检查。
- 六平台actual/original生产闭包均相等；1159份first-party Go文件与Git blob匹配；selected466/production111/standalone150模块不变；原37项处置及2026-10-25..2026-11-08观察窗口保持。完整CI/L3重新扫描，身份资格化不冒充扫描成功。
- GitHub Release15个附件、11条SHA256SUMS、两架构OCI/source/脚本标签和发布前稳定/上一预览快照全部核对。测试Runner实际ID：sha256:a0d2ae614f45544afd15ba79b47edb875e53a1486d1138d806245d458fc9c460。测试配方、sh/CJS夹具与可信控制面、冻结产品逐字节一致，无有效覆盖修改。
- 公开正文5条更新、5条完整升级提示；运行时接口与实际更新弹窗依现有限额显示5条摘要和前三条提示，包括来源端断流与新入网限制。草稿使用上一预览摘要作解析占位；最终公开正文由流水线生成并绑定本版真实摘要，未将草稿当成公开产物。
- 自更新通道的加入不安装、自动安装独立、重启保留及退出不降级由真实公开旅程覆盖；systemd生产安装/实际用户实例升级不适用。本轮未发布脚本、未修改应用市场latest默认入口。

## 精确原生验收

native run40214464526/attempt1：success；controller=35b19d983e170e7ff8f79b9c69357d88ee413505，source=f244152d9930bd65dc20241ce333ddf2d9626d97。Ubuntu24.04.5 amd64，libvirt10.0.0-2ubuntu8.19、真实KVM及systemd临时worker，manualRunJob=false；9检查通过。1550份全部tracked Go原始blob与前后manifest逐一匹配；运行代码含本轮元数据写入。worker MemoryMax1GiB、TasksMax128；Docker夹具64MiB/0.25CPU/32PID/只读/cap-dropALL。

FORWARD DROP前后不变，Docker host/peer端点前后正常，libvirt/Docker规则共存。默认网络XML与发行版包归属保留。客体与其磁盘通过真实任务删除；其他Docker/测试资源按精确名称清理，临时runner销毁为最终隔离边界，不扩大为所有发行版/防火墙兼容保证。

本轮未验证物理ARM64、Windows客体、全部发行版/防火墙组合、宿主机重启与完整浏览器到客体控制台链路、长期性能。内置国家库Lite精度/覆盖有限；概览、小组件与集群既有外部公网信息来源不受影响。

## 生产、回滚与遗留

不适用（预览版禁止生产部署）；本次未连接任何生产/自有服务器，包括prod-108，未备份、部署、升级、读取生产状态。GitHub Latest及Docker latest保持v1.27.2。公开浏览器夹具不等于生产安装或systemd更新演练。

回滚保留v1.28.0-rc.6及sha256:1c7e0f8c80cd9c09e15bc0139ce722bee358d8e97676625f5df8e947c2b37220；不得覆盖不可变标签/版本镜像，切换通道不自动降级。回退前独立备份数据并核对任务；libvirt资源不会随旧面板移除。已有配对无需重配，默认稳定轻量Node安装器不能消费新格式入网命令的限制仍保留。

RC5桌面恢复偶发、Node新入网默认入口由对应维护者于2026-10-28/1.28稳定前复核；本轮通过不关闭其根因。既有低级MCP元数据范围线索仍在仓外，2026-11-04复核。TypeScript7/@typesNode26旧采用例外2026-10-23已到期，本轮未续期、未采用，不声称依赖全部最新；当前运行依赖安全审计与L3另有实际成功证据。

发布复用既有工作流，未新增知识库或发布工作流。源分支及前置工具已归档，源码、原始失败、截图、日志和恢复点保留；停止本轮持有且ownership marker匹配的预览，不物理删除工作树、代码或证据。C盘空间实测见仓外resource-closeout.json；未把净变化宣称为删除释放量。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-25T21:48:36+08:00
- 候选冻结时间：2026-10-25T17:01:15.236Z
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：10
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

### 流程异常明细

<!-- kpanel-release-process-incidents:start -->
[
  {
    "fingerprint": "preflight/release-contract/late-ready-qualification",
    "position": "before-production-write",
    "count": 1,
    "impact": "Writer clean/baseline checks preceded integration, but full JSON ready qualification followed initial source merges. No external product main/tag write occurred before qualification.",
    "recoveryEvidence": "ready-corrected.json and original writer checkpoints; source integrations isolated in owned release tree.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/workflow-cli/python-alias-unavailable",
    "position": "before-production-write",
    "count": 1,
    "impact": "Initial workflow invocation used Windows python alias and returned exit1 without workflow output.",
    "recoveryEvidence": "Bundled Python executable resolved through workspace runtime; existing workflow rendered to rendered-workflow.md.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/local-inventory/unreported-shell-exit",
    "position": "before-production-write",
    "count": 1,
    "impact": "First PowerShell candidate inventory invocation exited1 without output; no inventory evidence was accepted. Root cause not conclusively established.",
    "recoveryEvidence": "inventory.mjs independently completed the complete279-worktree inventory; candidate-inventory.json retained.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/release-contract/unsupported-tool-name",
    "position": "before-production-write",
    "count": 1,
    "impact": "First ready rejected gh in requiredTools; canonical schema permits only enumerated build tools.",
    "recoveryEvidence": "ready-first-input-failure.json preserved; removed unsupported entry and verified gh separately; ready-corrected.json passed.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preflight/local-toolchain/go-path-missing",
    "position": "before-production-write",
    "count": 1,
    "impact": "First draft notes check could not find Go in repository Bash PATH.",
    "recoveryEvidence": "Notes check repeated with existing verified Go1.27.3 SDK on PATH; notes-check.log passed. No product source or gate weakened.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/release-metrics/unsupported-argument",
    "position": "before-production-write",
    "count": 1,
    "impact": "First rolling-metrics invocation used unsupported --limit.",
    "recoveryEvidence": "Read canonical --help, used --releases20; release-metrics-before.txt contains actual metrics.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/review-trailers/noncontiguous-footer",
    "position": "before-production-write",
    "count": 2,
    "impact": "Two candidate checks reported OCR stale because separate message paragraphs were outside final trailer block. No pushed frozen product depended on those invalid trailer interpretations.",
    "recoveryEvidence": "Final source review saved both arms; contiguous footer in final-candidate-message.txt; final frozen check reports OCR recorded and L2 is parsed.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "preparation/review-facts/blank-eof",
    "position": "before-production-write",
    "count": 1,
    "impact": "First local review record introduced a trailing blank and diff--check warned before local commit; commit was corrected prior to frozen candidate publication.",
    "recoveryEvidence": "finalize-candidate.mjs removes stray EOF and records meaningful review facts; amended only unpublished local commit; frozen diff check clean.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  },
  {
    "fingerprint": "evidence/mock-ui/unsettled-animation",
    "position": "before-production-write",
    "count": 1,
    "impact": "Functional assertions passed but initial screenshots captured sidebar resize and modal entry transitions; those visual images were rejected.",
    "recoveryEvidence": "Original images/results retained; stabilized driver additionally awaits finite animations and closed mobile sidebar. Replacement13 checks and manual screenshots passed without changing product.",
    "permanentAction": "Release executor uses verified runtime/schema/CLI and contiguous trailer file, clean checkpoints and animation-settled screenshots for remaining phases. This is an execution correction, not a claimed permanent repository mechanism. KPanel release-tool maintainer reviews by2026-10-28/next preview; exit is correctly ordered applicable preflight and stable evidence on next use.",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->

以上均发生于生产写前，原始失败/无效证据保留，不把成功复验抹作零。本次与最近五个稳定v1.27.2/1.27.1/1.27.0/1.26.0/1.25.1原记录逐指纹比较，匹配结果见historical-incident-comparison.json；RC6同类执行问题单独保留，不误作稳定版。没有产品生产退化或生产恢复事实，不推断为已部署成功。

滚动14天/最近20正式版：4个正式标签、3个自然日；窗口内有证据的生产完成0，记录覆盖20/20，生产时间覆盖10/20。首提交到生产中位5.45小时、冻结到生产0.7小时，均10/20有数据；历史变更失败率5.0%，历史流程异常294/17份有数据记录，生产写后31，滚动5正式版重复19项且均已声明。RC不并入正式频率；详情保留release-metrics-before.txt，未知事实不补零。

## 仓外证据位置

本轮证据根：C:/GitHub/_release-evidence/kpanel-v128-rc7。关键原件：frozen.json、contract.json、candidate-inventory.json、source-qualification.json、ocr-coverage.json、native-verification.json及原artifact、ui-qualification.json、l3-artifact-verification.json及原ZIP、publication-artifact-verification.json、public-verification.json、public-browser-reports.json、source-archive.json、process-incidents.json、resource-closeout.json与最终通道回读。证据绑定精确提交/工具/环境，不作为未执行场景的保证。
