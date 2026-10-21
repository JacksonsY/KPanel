# KPanel v1.27.0 发布验收记录：发布失败，未公开

日期：2026-10-22。发布级别：L3。`releaseChannel=stable`，`releaseTrain=1.27.0`。

候选 `7ddeec2e81106ddcb65611c75108f8fa842a55d7`，tree `253dcdcb7678ca312ea7c351fe780f324f895782`；annotated tag `v1.27.0` 对象 `bfe428c5081ccbe0fea3da0c217d831373017669`。用户授权稳定版发布和受限恢复修复，未授权生产部署。

**最终事实：本次未发布稳定版。** 原发布 attempt1 在提升通道前失败；GitHub Release 保持 draft，Docker `latest`、GitHub Latest 和 `preview` 未提升，公开验收及候选归档均 skipped。本记录补齐已有标签的真实失败事实，不将标签或已生成镜像当作公开发布成功，也不降低验收覆盖门禁。

## 发布画像与范围

计划汇总 RC1–RC23：本地应用、OneDrive、桌面外观、移动终端、图库、后台批量终端、文件编辑器、systemd 独立挂载安装与安全边界维护。包含展示、只读和授权宿主机操作，风险 L3。精确范围为 `v1.26.0..v1.27.0`。稳定日志8项摘要、3项升级提示已生成并通过解析；这些说明仍不是公开 Release。

额外稳定准备包括异步 MCP 测试终态等待、稳定版公开验收入口和原 run/attempt、candidate/version/digest 绑定的受限恢复。没有生产部署或数据库迁移。恢复入口经静态复核和回归，未执行真实部分发布故障恢复。

## 跨仓库与回滚点

`scriptLinkageState=not-required`，无需发布脚本（不适用）。内置脚本 commit `160924a498d9b8d071fed72f165d23c1b9e886de`，SHA-256 `39837815ce0eb23021fd4d8e8b5898000f5309b709bdc48b2713e18f78f3eaf1`；跨仓变更集和脚本候选不适用。

上一稳定 `v1.26.0` / `247ff67d98700c9af56d55a807cb9389fc8eedc1`，Docker latest `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`。上一预览 `v1.27.0-rc.23`，preview `sha256:144cf771a1043dc584b342cd2cef15e3f0b9280a632706b0c1c04576a1ac0dd7`。本次没有执行生产回滚或公共通道回退，原稳定入口保持上述状态。

## 独立复核与安全补漏

恢复源代码、手册、runner上下文和冻结正文经独立只读 Codex 复核，最后 reviewed `d98b2278` 为 PASS，与最终候选 tree 相同；其他提供商没有可用非交互入口。OCR1.12.11：9文件中7可评审文件100%覆盖，另2份文档人工核对，自由臂0、H0/M0/L0，constrained-only=unreported。这些复核没有发现本次实际触发的许可证布局缺口，不能据此声称发布逻辑已无缺陷。

稳定前轻量盲审为 source-only，非CF。累计44组中34 recorded、10 missing，`invalid_trailer=true`。四个范围各有报告；cluster仅部分覆盖，`client_v2.go`、`light_platform.go`、`light_terminal.go`仍无明确审阅记录。原始H0/M2/L0；两条既有资源配额线索经独立静态核实维持medium，依赖可信高权限主体，未动态验证实际资源阈值。仓外待办#45/#46已有cluster/terminal维护者、触发条件和2026-11-04期限；备份导入中风险待办继续由backup维护者跟进。不公开未修复攻击细节，不宣称全仓审计通过。

## 自动门禁与实际发布结果

- 精确候选 CI [39631194336](https://github.com/kejilion/KPanel/actions/runs/39631194336) attempt1、主线 CI [39632406687](https://github.com/kejilion/KPanel/actions/runs/39632406687) attempt1 均success；原生workflow解析通过。候选/主线freshness39631194386/39632406598成功，必需来源9/9，可选CF未请求。
- 原稳定发布 [39633562698](https://github.com/kejilion/KPanel/actions/runs/39633562698) attempt1：`l3 / l3` job119337890491成功，`release` job119346123892失败。所有证据均绑定同一候选，不替换原失败。
- 完整 `make verify-release`、生命周期和 `image_e2e=pass`：GitHub-hosted Ubuntu24，image20261012.334.1，Go1.27.3、Node24.21.0、Docker28.0.4。入口SHA-256 `a2406ede4695ad76b509bad1389e543b54e6d1e9b1af17f5f6111b6e582ef6f7`。web/go/deploy三lane退出0，原始日志摘要独立回算一致，identityUnchanged=true；前端314文件、3096项通过、6项跳过。
- L3工件12361441327，ZIP大小602837字节，API/实际SHA-256均 `0b8941974e463fe5d2f971ecb5f0161643682b4ca2143bb773de8776eb51f247`。
- release作业的源码、安全、安装生命周期、本机镜像扫描及运行时契约成功，双架构版本镜像已生成：`sha256:eb772e5528fa51e13e96bd76fb4b55982d9f9173da7df4794a8c157de7f23c89`。这仅证明版本产物已生成，不证明stable已发布或公开镜像E2E已完成。
- 2026-10-21T16:51:21Z，`Freeze exact release asset identity before channel promotion` 报 `ENOENT release/LICENSE`。新增冻结逻辑假定所有15附件在release目录，但原上传入口从仓库根上传LICENSE/THIRD_PARTY_NOTICES.md。完整 `release-assets.json` 尚未产生，snapshot工件上传、通道提升、GitHub公开、公开验收和归档均跳过。
- 原publication工件12363392005，ZIP126737字节，API/实际SHA-256均 `79147b253387c1d430bca354b5ff0183965375334b440baa3fccbeedebdee5d4`；含原说明、modified-components、发布前快照和未完成绑定的身份，缺少15附件清单，不能冒充完整恢复输入。
- 后续只读核对：draft Release ID419811240有15附件；REST tag endpoint对该draft返回404，按ID和草稿可见查询可以读取。冻结与恢复还存在草稿查询契约缺口，需独立修复。
- 原本已授权的恢复只接受“promote成功、publish失败”。本次两步均skipped且身份工件不完整，入口应拒绝；不扩大该恢复边界，不重写tag或重建覆盖1.27.0镜像。另以新补丁候选发布，本记录不预记其结果。

## 供应链与依赖

六平台actual/original闭包按固定Go1.27.3重算，第一方1007文件，sourceManifest `3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580`、firstPartySource `b2a9f9ead1aa54ea71c63e49361c7384b90aaf65c5bbf8382e1f66c0f8d02641`、closure `f5b0d0988c5f7d37ed9cd9cee453c22586efa5d9d1bed42240599df5517eee07`。

扫描器packageCount：production110/selected465/standalone150；原始exit分别0/1/1，保留finding分别0/10/27。后两项为保留已审finding的完整清单，不是零告警。37项与44个盲审包组不是同一计数体系。源码/镜像扫描、modified-component及双架构SBOM/provenance原件保留；没有公开产物门禁通过结论。每日安全审计39565808978成功，EOL2026-07-28复核有效至2026-10-28，原扫描处置期限未延长。

## 多维质量与未验证范围

| 维度 | 状态 | 证据与边界 |
| --- | --- | --- |
| 业务正确性与双端互通 | 已验证 | 同SHA CI/L3；正式公开验收未运行，真实微软云端未验 |
| 网络入侵与供应链安全 | 已验证 | 规范扫描、原始inventory和复核；遗留中风险及未审范围保留，无全仓安全结论 |
| 稳定性、失败恢复与兼容 | 已实现未实机验证 | L3生命周期通过；实际发布失败，恢复入口未真实执行 |
| 性能与资源预算 | 已实现未实机验证 | 沿用RC有界资源证据，无真实硬件吞吐/FPS/soak结论 |
| 用户体验与可访问性 | 已实现未实机验证 | 本次公开浏览器未运行；实体手机、Safari、读屏和原生缩放未验 |
| 数据、配置与迁移 | 已验证 | 无数据库迁移；真实OneDrive往返和恢复未验 |

公开Release正文、更新接口/弹窗、15附件公开下载与11校验、公开镜像/浏览器和自更新通道：本次均未验收。不得用RC23的公开成功替代本次。OneDrive真实账户、实体服务器systemd/OpenRC升级重启回退、实体手机/Safari及大图库弱网仍未验。自签名TLS端口仍支持明文HTTP API，应仅用于可信网络并强制HTTPS。

## 生产、候选处置与资源

产物未公开，生产未部署。生产目标、备份、灰度、部署和回滚均不适用；prod-108未连接、未核对、未备份、未部署、未升级。隔离容器不是生产或实体服务器安装证据。

`release/v1.27.0-candidate` tip7ddeec2e保留为失败列车，不自动归档。原tag、draft、不可变版本镜像及失败证据保留；本地发布修复和文档工作树仅按所有权处理，不清理其他作者资源。后续修复将此失败记录纳入主线，解除后继候选缺少验收记录的覆盖缺口，不把失败结果改写为成功。

证据根 `C:/GitHub/_release-evidence/v1.27.0-stable-recovery`；补漏原件 `C:/GitHub/_release-evidence/v1.27.0`。旧workflow上下文错误、范围身份失配、inventory失败和工具下载不可用原件均保留；actionlint未取得可运行版本，不称其通过。已知流程异常含 `publication/freeze-release-assets/legal-file-layout`，在生产写前被门禁拦截；历史总数不完整，不推断为零。

本次可确认单项如下；它不是完整历史累计数。机器指标依照模板将不完整总数记为“未记录”，不将已知1次误写成全部异常总数或零。

| 已确认指纹 | 次数与位置 | 影响、恢复证据与后续处置 |
| --- | --- | --- |
| publication/freeze-release-assets/legal-file-layout | 1，before-production-write | 2026-10-21T16:51:21Z原attempt1在冻结入口失败；原日志和publication工件保留，latest提升及公开均skipped。发布维护者在本地34a28579修正真实布局并补回归，组合候选另做独立复核、同SHA CI/L3后才可发布；不将未完成验证记为永久修复通过。后续复核触发为新候选发布前。 |

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-15T14:30:29+08:00
- 候选冻结时间：2026-10-21T23:58:38+08:00
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否
- 若发生失败，发现时间、恢复时间和逃逸门禁：不适用
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 已记录发布流程异常或无效证据拦截次数：未记录
- 其中生产写操作开始后异常次数：未记录
<!-- kpanel-release-process-metrics:end -->

<!-- kpanel-release-process-incidents:start -->
[]
<!-- kpanel-release-process-incidents:end -->
