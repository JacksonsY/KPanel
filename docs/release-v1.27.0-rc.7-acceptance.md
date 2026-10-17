# KPanel v1.27.0-rc.7 发布验收记录

已验证：RC7于2026-10-17T16:11:57Z公开，正式GitHub run39045179405 attempt1终态success；生产未部署。

日期：2026-10-17（UTC；北京时间2026-10-18）；发布级别：L3；releaseChannel：preview；releaseTrain：1.27.0。

## 范围、授权与身份

用户决定直接发布1.27.0新预览并清理本轮1.26.1准备物。仅发布GitHub Release和Docker Hub；不部署生产，不提升stable/latest，不使用自有服务器或WSL发布。

业务组合及控制面B8为`b8df158134e4d57579a6f3f23ff0c9b2b958a59b`，批准主线基线为`791c03f1b7eec1f35437141afa50bd999ed30423`；冻结产品3D为`3d669b7f1414a140c069332b3c9adde4e7f4025c`，版本`v1.27.0-rc.7`。唯一列车`release/v1.27.0-candidate`保留3D；RC6及历史版本不可覆盖。正式root39045179405仅dispatch一次，attempt1，controller B8与product 3D分别记录。

纳入`fix/security-backlog-complete@f1d4a644aad3c26eba93b31fd9d4739abb638386`及独立核验后的集成修复：认证在途/账户预算、ASCII用户名与IPv6来源分组、Passkey限额、秘密脱敏、备份导入容量与租约恢复、外来备份完整预审和一次确认、备份固定父目录句柄及有界重名文件处理。原作者工作树未清理或冒作本轮所有。

撤回原候选的IPv4-only端口发布和会拒绝旧控制器写入的v1请求体签名，相关净差异恢复主线基线；既有入口及中继协议不变。相关风险保留，由发布维护者2026-10-23前复核兼容方案。备份恢复缺明细、旧导入标记或未完成清单时，确认不能替代预审；更新Agent并重新导入，Panel身份单独恢复不受宿主清单前置影响。

## 独立复核与候选安全盲审

源码独立复核覆盖59/59文件，后续测试夹具、版本组装和业务事实刷新各有独立增量复核。实现与复核提供商均codex，其他提供商CLI不可用，fallback=provider-unavailable；runtime模型不可见，不推测模型。OCR1.12.11先保存自由臂再约束臂，初始有效3项已修复或撤回，constrained-only为0；后续已读取规则的增量不计为新增盲测收益。

候选安全盲审七单元，原始H1/M3与修复验证增量M1共核实5项：2项修复，3项延期。不同代理发现与验证；不把CF当作本轮已执行，CF not-run，不宣称全仓全面安全。

备份恢复祖先目录竞态H已在本轮范围内关闭：固定直接父Root并校验SameFile，应用/回滚/清理均使用父句柄与basename，日志记录平台文件身份，恢复时拒绝缺失或替换身份；新旧容器启动前再次复核路径。不同验证者复核源码和确定性测试，Linux完整CI覆盖相应回归。另一修复为超长扩展名碰撞时有界UTF8截断，避免重名循环。

延期3项：复杂空格/YAML多行脱敏、编码JSON/URL字段名脱敏、Agent复核与Docker daemon解析宿主bind路径之间的非原子窗口。前两项有外部overlay复现和授权AI/MCP输出路径核验；第三项确认边界但未实测Linux Docker利用，不冒称已原子保证或动态利用成功。负责人均KPanel release maintainers，2026-10-23复核，原始线索及处置保留。

安全最终报告SHA256`0dc7caf2c424e04ce4fdfaeef6a031651dfebc4c0a8dc203ad24db1c1e39410d`；父目录独立验证R2为`f0bc7c4f4187e9e4548cf18ab72986b70536c717867321695e0c7071fcdc5a55`（实际文件摘要以仓库外原件回读为准）。初始验证FAIL和平台中断原件保留，不计为通过；同会话防御性续审及独立源码结论分别记录。

## 源码资格与发布前验证

B8完整Linux L2 run39042465835、B8主线CI39043688174、冻结3D push CI39044047959均success。首次业务CI39042051111因tar fixture写死root UID/GID导致普通runner chown EPERM失败；只修夹具为实际属主，不弱化生产权限校验。首次release CI39043693977被51提交业务事实新鲜度门禁拒绝；补六维实质复核及独立审查，50/20+8门槛未放宽。所有失败原件保留。

Windows全Web2998项/309文件通过，Linux B8全Web2992项通过、6平台专属跳过。BackupCenter18项、前端构建、认证/存储/脱敏Go整包、Panel定向和备份确定性回归通过。Linux结果以各原run为准，不将Windows本地结果冒作Linux执行。

Go1.27.2，971首方Go文件，954生产引用，六平台actual/original闭包及四构建目标一致；selected graph SHA256`24c24be50e4477fc28044d24ae11c26ff0e6f75603d7f8ae91e798034fa81927`。冻结3D source SHA256`09eae06df3e2ba8fbf3c95be277a87e9b45f0d5bfb892333bcd9abfe65a42a84`，closure`94e74f223292dfbc13c1e7a59a621877b7e3aa63238e6f979b817417f5ddf963`。既有selected/standalone36项处置及2026-10-25期限不变；未升级依赖或构建目标。

脚本联动not-required：撤回端口和协议不兼容变化，沿用内置脚本commit`160924a498d9b8d071fed72f165d23c1b9e886de`、SHA256`39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`，没有外部应用市场同步需求。

## 本地预览与验证边界

B8 clean模拟数据预览9项通过、pageErrors0；覆盖浅深主题、390移动布局、CSS200%页脚、键盘确认、一次确认POST和503/null/缺明细时拒绝提交并提供升级/重新导入指引。正文14px、代码13px；9PNG中8张与逐张查看的验收原图字节一致，新增失败态图另行查看。

模拟数据不证明真实API/宿主资源；CSS缩放不替代原生browser zoom。读屏、实体手机、真实混合旧节点、完整异机恢复、宿主systemd安装/回滚、原生ARM性能与低配长期P95/RSS未在本轮重新实机验证。现有有界预算及自动回归不作为长期负载保证。

## 1.26.1准备物清理

本轮自有1.26.1候选树、维护流水线树、两个本地分支和外部准备证据目录已删除，共用修复与必要原始证据先迁入RC7。复查本地引用和目录均无残留，远端没有本轮1.26.1 refs、Release或workflow dispatch，本轮未发布1.26.1版本镜像。清理回执cleanup-1261.json记录释放409612288bytes；不清理未知所有权树或历史Git对象，不删除已发布版本。

本轮原始证据根：`C:\GitHub\_release-evidence\v1.27.0-rc.7`。产品风险与流程异常分别记录，失败不被成功重试覆盖。


## 正式GitHub L3与关键路径

[正式发布run39045179405](https://github.com/kejilion/KPanel/actions/runs/39045179405)绑定controller B8/product 3D，所有必需阶段success。可复用release内L3按既有预览路径skipped，外层同一root完整L3实际通过，不是省略。

候选CI 2026-10-17T15:28:30Z—2026-10-17T15:41:26Z，L3 job 2026-10-17T15:28:30Z—2026-10-17T15:40:12Z，实际重叠702秒；promote于2026-10-17T15:41:29Z在双成功后启动，3D主线CI于2026-10-17T15:55:49Z成功。不以跨run总历时或本地返工归因于并行，不宣称总runner成本下降。

完整L3实际窗口2026-10-17T15:28:47.327Z—2026-10-17T15:40:06.733Z；go version go1.27.2 linux/amd64、Node v24.21.0、Docker 28.0.4、GitHub-hosted ubuntu24/ImageVersion20261012.334.1。三lane日志摘要重算一致，source tree 2a0953b06fd83ecde7331ffbacfad558eb2752f0、identityUnchanged=true、源码总433930ms；web:215209ms，go:433905ms，deploy:1843ms。27份rawEvidence摘要/bytes/exit、12份actual/original闭包和971首方源文件身份回读通过。

正式扫描110 production modules exit0/0finding；selected exit1保留10、standalone exit1保留26。既有36项处置及到期日不变，不能写成全依赖图无漏洞。L3原artifact12106400064，ZIP591620bytes，SHA2565b07b29ddf995b2784ca017a73f137509e2dfe9319a58a50015c415a3373027e，CRC/6文件核验通过。

## 公开产物与真实浏览器

[RC7 Release](https://github.com/kejilion/KPanel/releases/tag/v1.27.0-rc.7)为prerelease=true、draft=false、非Latest；注释tag对象a0f064be36b5a81fcacab4181a55497c6a7b37a3，peeled=3D。Docker version/preview index为`sha256:872959cc726dd4c85eae5213abef1065af36c2d561d9cbd47021bdef7975222d`；amd64 sha256:3dfc8581346b37dd158f75244796a75833d2e3e5c52392ab7ea8592fe299982a；arm64 sha256:226a82a10c1388d240a0c9cfd23e244bbe35648d8baa9a48c1f92183a4bb8126。

15个附件逐项API digest/下载/磁盘回读、11项SHA256SUMS一致；Release正文、面板更新API和实际弹窗均核对4项摘要/3条升级提示及同一index。SBOM、SLSA和修改组件原件一致，17份provenance rawUTF8摘要/大小重算通过，两架构config source/version及statement subject/build args核对通过。原SLSA Git materials缺项保持absent-in-original，不补造。

四阶段真实浏览器报告全部passed/mocked=false/errors=[]，合计146项（published:45、upgrade:31、initial:44、restart:26），10张PNG逐张查看，3份cleanup仅清理测试自有容器/网络。真实PTY、刷新续接、关闭后404、命令历史持久化/删除/清空、更新预览加入/退出及窄屏弹窗按原报告核验。实际无systemd的503为预期失败注入，验证失败闭合与恢复，不冒作宿主安装成功。测试配方绑定B8/3D，recipeSHA256 8e07c6dda1815cf80973a4311cdb60eeb037bb7d6ebbc03c8da499801c93e77f，受控Docker库存与测试CA为明确fixtures；测试Panel/Agent不挂宿主Docker/systemd socket。

发布前snapshot身份绑定原root/attempt/controller/product，stable-before SHA256 83e8cb3570930ee5ed9b0a076562d05b5ea32ca2d0f319b8a88c60da9d991c3c，与publication/public字节一致；原snapshot上传早于镜像推送与公开。GitHub Latest仍v1.26.0，Dockerlatest仍sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9，原快照回读未变化。上一预览1.27.0-rc.6/sha256:eb9ee91fd9df911dd1b0dd0163507efbdc5a543909c59593f7d059135296b24d保留回滚输入；未执行回滚或生产部署，本轮没有重复dispatch、只读恢复或版本覆盖。

## 六维质量与收尾

| 维度 | 状态 | 证据与限制 |
| --- | --- | --- |
| 业务正确性与互通 | 已验证 | 独立源码、精确Linux L3、真实公开API/PTY；完整混合旧节点及异机恢复未重验 |
| 安全与供应链 | 已验证 | 候选盲审2项修复、3项限期跟进，正式扫描及产物身份；CF未执行，不宣称全面无漏洞 |
| 稳定性与恢复 | 已验证 | 固定父句柄、失败关闭、journal与正常rollback回归，公开503恢复；跨daemon原子绑定未保证 |
| 性能与资源 | 已实现未实机验证 | 有界认证/导入/命名预算和回归；低配长期P95/RSS未验 |
| 体验与可访问性 | 已验证 | Mock9项及公开146项/10PNG、窄屏/主题/键盘；原生zoom、读屏、实体手机未验 |
| 数据与兼容 | 已验证 | 完整预审、来源私钥排除与身份journal；旧Agent须配套升级重导入，未完成旧日志不自动迁移 |

owned业务分支以原子双expected-SHA lease保存B8到archive/fix/v127-rc7-security-integration并移除活跃远端。4176预览已停止，clean自有业务工作树移除，释放297876464bytes；原作者工作树与其他预览不动，唯一RC列车保留3D。验收文档候选仅修改本记录、当前业务事实和既有提案；文档自身CI/mainCI及分支归档终态记仓库外最终交付回执，不提前写未来结果。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-17T16:08:55+08:00
- 候选冻结时间：2026-10-17T23:12:37+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（预览未部署生产）
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：10
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->

产品审查发现与流程异常分开，首次失败和中断原件保留；后续成功不倒写首次成功。

<!-- kpanel-release-process-incidents:start -->
[
  {
    "permanentAction": "依赖源码冻结后再运行组合测试",
    "count": 1,
    "position": "before-production-write",
    "impact": "Panel定向编译撞上hostbackup作者未完成的编辑",
    "fingerprint": "local/panel-test/concurrent-hostbackup-edit",
    "recoveryEvidence": "panel-targeted-r2.log passed",
    "historicalReleases": []
  },
  {
    "permanentAction": "先确认精确CI终态与原始日志，再形成handoff回执",
    "count": 1,
    "position": "before-production-write",
    "impact": "Linux CI前调用handoff，被candidate/evidence门禁正确拒绝",
    "fingerprint": "local/handoff/premature-evidence",
    "recoveryEvidence": "handoff-pre-review.log; contract-integration-handoff.json and handoff-passed.json bind B8 and passed CI39042465835",
    "historicalReleases": []
  },
  {
    "permanentAction": "同一次命令显式固定Go和Make PATH",
    "count": 1,
    "position": "before-production-write",
    "impact": "PATH未注入固定Go，native和repo-bash探针失败",
    "fingerprint": "local/governance/missing-pinned-go-path",
    "recoveryEvidence": "governance-final-r2.log passed",
    "historicalReleases": []
  },
  {
    "permanentAction": "仅在CRLF归一字节等于HEAD时恢复精确HEAD并核对cached diff为空",
    "count": 1,
    "position": "before-production-write",
    "impact": "唯一release树30 vendor文件检出CRLF，原始字节摘要门禁拒绝",
    "fingerprint": "local/release-vendor/crlf-checkout",
    "recoveryEvidence": "governance-release-r2.log; vendor-byte-repair-release.json; independent state-note.json",
    "historicalReleases": []
  },
  {
    "permanentAction": "fixture使用tar.FileInfoHeader实际属主；生产权限校验保持严格",
    "count": 1,
    "position": "before-production-write",
    "impact": "首轮CI39042051111人工tar fixture默认为root属主，普通runner chown EPERM",
    "fingerprint": "github/ci/nonroot-setid-fixture",
    "recoveryEvidence": "Original failed run/log ZIP retained; independent fixture PASS; full CI39042465835 and frozen3D CI39044047959 success",
    "historicalReleases": []
  },
  {
    "permanentAction": "使用已有跨平台archive/tar.FileInfoHeader",
    "count": 1,
    "position": "before-production-write",
    "impact": "首次fixture修补引用只存在Unix的fileOwnership helper，Windows编译拒绝",
    "fingerprint": "local/fixture-helper/windows-unavailable-symbol",
    "recoveryEvidence": "fixture-owner-windows-r2.log passed; 独立fixture复核",
    "historicalReleases": []
  },
  {
    "permanentAction": "仅要求防御性代码/测试核验；不要求攻击利用细节，不绕过过滤",
    "count": 1,
    "position": "before-production-write",
    "impact": "独立修复报告首次终态被平台过滤，无报告不计PASS",
    "fingerprint": "review/platform-filter/repair-report-interruption",
    "recoveryEvidence": "security-blind原中断记录及同会话防御性r2报告passed",
    "historicalReleases": []
  },
  {
    "count": 1,
    "fingerprint": "evidence/gh-log/buffered-download-no-bytes",
    "recoveryEvidence": "integration-log-download-cancel.json; 原始ZIP1262172bytes CRC通过",
    "permanentAction": "使用已有有界stream ZIP helper与HTTP2关闭设置，保留原空文件及取消回执",
    "position": "before-production-write",
    "impact": "gh run view --log缓冲下载长时间零字节；只取消精确自有gh进程，未重跑CI",
    "historicalReleases": []
  },
  {
    "count": 1,
    "fingerprint": "github/candidate/business-context-51-commit-threshold",
    "recoveryEvidence": "Original failed run/log retained; canonical six-dimensional context review independently PASS e9fd5835; frozen3D CI39044047959 success; thresholds unchanged",
    "permanentAction": "版本最终冻结后运行既有business-context-freshness入口；不调整阈值、不将基线移动替代实质业务复核",
    "position": "before-production-write",
    "impact": "RC7 push CI39043693977在测试前被51>=50业务事实门禁正确拒绝",
    "historicalReleases": []
  },
  {
    "fingerprint": "acceptance/metrics/missing-history-field",
    "position": "before-production-write",
    "count": 1,
    "impact": "首次验收metrics校验拒绝9条缺historicalReleases的JSON，不涉及已公开产物",
    "recoveryEvidence": "首次失败保留工具输出及acceptance-before-canonical-fields.md；补canonical字段后见acceptance-metrics-r2.log",
    "permanentAction": "生成验收异常JSON时使用report-release-metrics权威字段定义并执行本地校验",
    "historicalReleases": []
  }
]
<!-- kpanel-release-process-incidents:end -->
