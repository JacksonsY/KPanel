# KPanel v1.26.0 发布验收记录

日期：2026-10-15。发布级别：L3 流程的**降级执行**（见“未执行的门禁”），releaseChannel=stable，releaseTrain=1.26.0。

候选提交 / 标签：`247ff67d98700c9af56d55a807cb9389fc8eedc1` / [v1.26.0](https://github.com/kejilion/KPanel/releases/tag/v1.26.0)，2026-10-15T06:11:57Z 公开，draft=false、prerelease=false，已成为 GitHub Latest。产物已发布，生产未部署。

上一稳定版本 / 回滚点：`v1.25.1` / 镜像 `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。

## 发布范围

内容为 `1.26.0-rc.1` 至 `rc.20` 的全部变更，源码与 `v1.26.0-rc.20` 产品提交 `7723da7d` 相同，稳定版仅追加版本号与发布材料：`VERSION`、`web/package*.json`、`internal/version/version.go`、`CHANGELOG.md`、`scripts/torrent-backport-scan-scope.json`（重算 `firstPartySourceSha256`，方法对 rc.20 提交复现出其记录的哈希）。用户可见更新见 [CHANGELOG](../CHANGELOG.md) 的 1.26.0 条目。

候选分支：`release/v1.26.0-candidate` 已由发布工作流归档为 `archive/release/v1.26.0-candidate`（`247ff67d`），活跃候选分支已移除，远端复核通过。

## 发布过程与异常

同一标签共触发 3 次 Release 工作流，前两次均在发布前检查阶段失败，未产生任何公开产物（失败时 GitHub 无该 Release、Docker Hub 无 `1.26.0` 标签）：

| 运行 | 失败步骤 | 原因 | 处置 |
| --- | --- | --- | --- |
| 38687487392 | 校验发布说明 | CHANGELOG 1.26.0 条目缺少用户可见条目 | 补条目，本地用 `golang:1.27.2` 复现通过 |
| 38687999926 | 扫描（torrent 基线） | 改 `version.go` 使首方源码哈希变化，未同步 scan-scope | 重算哈希，`source-checks` 21 项通过 |
| 38689801385 | — | 全部步骤成功 | 公开 |

两次失败后均在无已发布产物的前提下将 `v1.26.0` 标签删除并指向修复后的提交；该做法不适用于已有公开产物的版本。

## 自动门禁与公开产物复核

| 项 | 结果 | 证据 |
| --- | --- | --- |
| Release 工作流（测试、漏洞扫描、双架构构建、镜像运行契约、摘要一致性、最终发布说明校验） | 已验证 | 运行 38689801385 全步骤 success |
| Go/npm 依赖漏洞扫描 | 已验证 | 运行 38687999926 日志：go.mod 0、package-lock.json 0、两个 Dockerfile 配置 0 |
| 公开镜像 | 已验证 | `1.26.0` 与 `latest` 同为 `sha256:e0fc7776d1b55754e597d73aab25c519a70c8ef0402752e1e70896397f1e96f9`，含 linux/amd64、linux/arm64；`preview` 保持 rc.20 摘要不变 |
| 附件 | 已验证 | 15 个附件；下载后 `SHA256SUMS` 的 11 项全部 OK |
| GitHub Latest | 已验证 | `releases/latest` 返回 `v1.26.0` |
| 发布说明 | 已验证 | 8 条更新、3 条升级注意事项，经运行时解析器 `TestRenderedReleaseNotesForPublication`；发布后正文曾按此重写一次 |

## 未执行的门禁（降级执行，风险由发布决策人承担）

- **未执行规范的完整 L3**：`scripts/run-release-l3.mjs` 的可信 Runner 与 arena-154 验收环境按发布决策未使用，本机无 Go 和该 Runner 镜像。仅依赖工作流内置门禁。
- **未补 CF 安全审计**：rc.20 验收记录写明“稳定前必须补审”，本次未补，仍为遗留风险；不称无风险。
- **未做隔离真机与浏览器验收**：本次未重新运行浏览器验收；沿用 rc.20 的 165 项浏览器结果，仅对应同源码。Bilibili 列表在该隔离环境返回 HTTP 502，成功路径仍未验证。
- 未验证：live Panel 更新 API 与更新弹窗、稳定通道自更新实机旅程、原生 ARM 运行、长时压力。

## 自更新与回滚

稳定通道读取 GitHub Latest（现为 v1.26.0），已在稳定通道的实例将在每日更新窗口自动检查；本次未在实机验证自更新旅程。回滚：恢复 `v1.25.1` 镜像 `sha256:1894b5e3…c325`；降级前备份 Panel 数据、自定义主题和通知配置，旧版本不保证读取新元数据。

## 生产部署

不适用，本次未连接、未部署任何生产环境。

## 遗留风险与后续准入

1. 补做 CF 安全审计（scoped/full）并记录结果。
2. 在登记的隔离环境对 v1.26.0 公开镜像补做 L3 与浏览器验收。
3. 在真实稳定通道实例验证自更新到 v1.26.0 的旅程。
4. 已知项：Bilibili 教程列表成功路径未验证；外部平台限流或接口变化不可保证。
