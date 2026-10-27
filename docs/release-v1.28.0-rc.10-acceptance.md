# KPanel v1.28.0-rc.10 发布验收记录

日期：2026-10-27（Asia/Shanghai）。发布级别：L3。预览产物已发布，未部署生产。

候选提交／标签：`93c4acb9d7cdacf2a20f022b8a662af59b309082` / `v1.28.0-rc.10`；预览 Release [425446422](https://github.com/kejilion/KPanel/releases/tag/v1.28.0-rc.10) 为非草稿 prerelease，19 个附件，发布时间 2026-10-27T12:45:20Z。版本镜像：`docker.io/kjlion/kejilion-panel@sha256:184d4ba2c80db45869340bafb66239515b34f6728dae8de61bb3c1198cb561a7`。正式 Latest 仍为 `v1.27.2`，发布前 stable 摘要 `sha256:5369dc9a2b8151314181d306e8d490939e63607d017b07c3985a0f4afb65aa10`；上一预览回滚点 `v1.28.0-rc.9` / `sha256:620aa6096f0f1018efd9e08b8619de55cf9f61a97e9adafee9b295e5484e1aca`。

## 范围与来源

| 来源分支 | 精确 tip | 纳入内容 |
| --- | --- | --- |
| `feature/browser-one-click-install` | `68dac59912551cbbc806a1d761bd839b87dd821d` | 浏览器组件一键安装、面板容器重建后连接恢复 |
| `fix/enrollment-platform-order` | `6542f1ef7270a0b158bfd661b1b4ba332c2f65c3` | 单节点接入先选择系统，再生成命令 |
| `fix/windows-archive-state-root` | `2edefbf985449ff3788f582871b7a9ef947be1f8` | 修复 Windows 卷根目录下归档状态初始化 |

来源 tip 均为发布候选祖先；引用及工作树保留，尚未归档。候选另含 Windows 回滚测试及恢复阶段错误上下文增强；它们没有改变回滚行为。`scriptLinkageState=not-required`，未发布外部脚本。浏览器组件使用固定镜像，KPanel 更新不会自动更新组件镜像。

## 验证和限制

[候选 CI 40497686756](https://github.com/kejilion/KPanel/actions/runs/40497686756) 通过；[预览发布流程 40498898477](https://github.com/kejilion/KPanel/actions/runs/40498898477) 第 2 次尝试通过：候选 CI、GitHub-hosted L3、主线及标签提升、主线 CI、Windows 附件构建、Release 和公开浏览器及隔离容器验收成功。L3 为第 1 次尝试通过并复用同一候选证据；首轮 Linux 源码扫描因漏洞数据库镜像 404 失败，重跑扫描成功。公开 Release 正文、19 个附件数量、预览标记、镜像摘要及 latest 指向已读取；面板发布信息接口和登记验收环境的更新弹窗未单独验证，不以公开浏览器验收替代。

Windows SCM 更新回滚验收在早先两次发布复验中失败；第二次明确返回 `update failed (injected new-binary health failure), recovery pending: Access is denied`，第一次原始错误未完整输出。后续候选、主线和发行阶段 Windows 检查通过，另有十五轮连续真实更新与回滚通过；两次首败具体根因仍未确认，不能称为已修复。错误上下文修改保留原错误并便于未来定位。真实 OS 重启、浏览器完整自动／手动更新和失败回滚端到端尚未验证；公开验收不代表所有网络拓扑或真实生产部署。Windows 归档修复的真实 D 盘及原用户环境未复测。

依赖时效独立检查曾因既有 TypeScript 7 和 @types/node 26 升级例外过期而失败，未把它伪报为通过。来源安全范围及扫描结果有边界，不代表全产品已完成安全盲审。回退前备份配置、节点状态和在途任务；退出预览不会自动降级。原始工作流证据和失败日志保留在 `C:/GitHub/_release-evidence/kpanel-v128-rc10`，发布工作流的 artifacts 是权威原件。

## 交付节奏数据

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：2026-10-27T17:36:58+08:00
- 候选冻结时间：2026-10-27T11:38:42Z
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
