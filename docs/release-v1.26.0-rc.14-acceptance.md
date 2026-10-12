# KPanel v1.26.0-rc.14 发布验收记录

发布级别：L3。`releaseChannel=preview`，`releaseTrain=1.26.0`。

批准的 `main` 基线为 `a690084eb4a135db6db986612d2c8ae8cd2df26c`；冻结产品提交为 `690b953ad0609b1cff56f80082cf9ae3cd907124`。注释标签 `v1.26.0-rc.14` 的 tag object 为 `b2e976ab3afd612ef8e803d7677701b14d08af4b`，指向上述产品提交。Release 工作流于 `2026-10-12T18:20:43Z` 公开，`prerelease=true`、`draft=false`。

回滚预览为 `v1.26.0-rc.13` / `63dc3afe50fc9354446cc0af490e475409700e6e` / `sha256:25ba652bc2f75e6292d405ff37c645a4092268c6ab48b6ed042ce10cf0e6dadc`。稳定版仍是 `v1.25.1`，Docker `latest` 仍为 `sha256:1894b5e3aa76edbd75efef2de1164f44d38734cb3a27dc11aa2f75c420c1b325`。本次未部署生产。

## 候选分支整理

| 候选分支 | 精确 tip | RC14 处置 |
| --- | --- | --- |
| `feature/terminal-split-workspace` | `5bb9b8d8c3f2bfc5b60ad224a2e448a84bb61b2d` | 唯一尚未发布的产品增量，已合入 RC14。 |
| `fix/issue-27-autofs-statfs` | `0f6d47c0c8b389bb60cdb99b76864b42b88d3c09` | 修复已在 RC13 产品中；RC14 相对 RC13 没有新的差异。 |
| `fix/system-packages-retry` | `c1fe7b755ccce8882edb711448c7186d982c0672` | 重试修复已在 RC13 产品中；RC14 相对 RC13 没有新的差异。 |
| `feature/systemd-services` | `10dcb1d5502a8b899bbdf644f9759cf74e79ebcd` | 系统中心的 systemd 服务管理已在 RC9 上线；本版不重复集成。 |
| `feature/systemd-services-backend` | `33c11668e6f8152c29823736ce2ac82e4812dddd` | 后端配套已随 RC9 上线；本版不重复集成。 |

版本更新内容只有交互终端分屏工作区：支持单屏、左右、上下和四宫格；布局切换、调整窗格、最大化及窄窗口降级期间保留现有 PTY 会话。窗格布局只影响当前页面会话，不需要迁移配置，也不改变主机与终端权限。该能力是前端增量，不新增服务端 API、数据库 schema 或 Agent/Node 协议。

旧桌面快捷方式图标、分页和文件页布局仍按用户要求归档，没有重新进入候选。`Offers` 是既有的广告专栏 `/offers`；RC14 没有改动该页，“新内容”提示属于浏览器本地状态，不是本次新增的优惠内容。

作者分支和工作树保持原样。发布候选分支 `release/v1.26.0-candidate` 仍指向冻结提交 `690b953a`；Release 工作流的候选归档步骤对预览版跳过，供后续 RC 沿用。

## 审阅与验证

- OCR 固定版本 `1.12.11`（`a758d9cbfb689937c7857ad64b2dd66adb58c0c2`），范围 `63dc3afe..4e0dd84b`：15 个可审文件全部复核；自由审阅与规则复核均无有效 H/M/L 发现、无仅受限规则发现。手工重点复核会话标识、窗格会话分配、活动窗格命令目标、关闭重试，以及零尺寸/隐藏窗格的 PTY resize 行为。
- 独立 first-party 源身份审阅确认 Go 代码差异只涉及 `internal/version/version.go` 版本元数据；变更不涉及 imports、执行流程、权限、网络路径或数据处理。审阅 JSON SHA-256 为 `510EC20749E147BF1A6D14275BE9D651DD9FA7F8D8BF18B3EA3103419DD1C1FF`；第一方源摘要为 `26beea06609ecf2f6e9c0717bdbf1ce9f952f789e2f322e5b59508052323143d`。
- 候选 CI [38284730757](https://github.com/kejilion/KPanel/actions/runs/38284730757) 与依赖新鲜度 [38284730763](https://github.com/kejilion/KPanel/actions/runs/38284730763) 通过。相同产品 SHA 快进 `main` 后，主线 CI [38287526417](https://github.com/kejilion/KPanel/actions/runs/38287526417) 与依赖新鲜度 [38287526305](https://github.com/kejilion/KPanel/actions/runs/38287526305) 通过。
- 固定 runner L3 `v1.26.0-rc.14-690b953-l3-r1` 通过，exit 0；候选 SHA 精确为 `690b953ad0609b1cff56f80082cf9ae3cd907124`，runner 为 `kpanel-go127-security-runner:go1.27.2-node24.21.0` / `sha256:20667a9f1fad6590219ba8d8acdabc09a49f8683d4a877b847df17d28b83f8fb`。`status.txt` 和完整 evidence SHA-256 清单已逐项核对。
- L3 全量 Go 测试、race、`go vet`、前端类型检查与测试、双架构发布二进制及镜像构建、生命周期和更新回滚夹具通过。`govulncheck` 未发现可达漏洞，另有 1 项 require-only 信号；`npm audit` 为 0。Trivy production 闭包 106 个包未保留发现；selected 460 包仍记录 10 项漏洞，standalone 150 包仍记录 24 项漏洞及 2 项配置发现。后两类是现有策略保留的非 production-closure 清单，不代表依赖图全局无风险，稳定版前仍需按既有准入要求复核。
- Release 工作流 [38288601355](https://github.com/kejilion/KPanel/actions/runs/38288601355) 的必需步骤全部成功；预览版的候选分支归档步骤按条件跳过。标签依赖新鲜度 [38288601205](https://github.com/kejilion/KPanel/actions/runs/38288601205) 通过。
- 发布后下载 15 个 GitHub Release 附件，逐个核验 GitHub API digest 和文件长度，15/15 匹配；`SHA256SUMS` 的 11 项也全部匹配。明细保存在外部 `release-asset-verification.json`。
- Docker Hub 的版本标签 `1.26.0-rc.14` 与 `preview` 指向同一个 OCI index：`sha256:2b816c66fcbc60590e1afcc0517617b8db21dd79cb91b72d3219b5d52e7fbe95`。amd64 manifest `sha256:ffa755e7520df806faa334ae02db97e9c5886806c301ebd499f35c0b133bf1f3` 与 arm64 manifest `sha256:87561a190b0553f529428f3a1496d4ee42262cae4ebb41a59b04a07f647678bf` 的 OCI version/revision 均为 `1.26.0-rc.14` / `690b953ad0609b1cff56f80082cf9ae3cd907124`。
- `arena-154` 显式拉取公开不可变 index digest 后运行 canonical `packaging/tests/image-e2e.sh`，`image_e2e=pass`、exit 0；临时容器、网络均清零，18080 端口释放。E2E 只证明公开镜像的隔离启动、健康、代理头、静态资源和 bootstrap 契约，不证明真实宿主 PTY 或生产升级。
- 本地 Mock 预览 `http://127.0.0.1:4184` 绑定冻结 SHA，四个模拟终端窗格可见且浏览器 console errors 为 0。Mock 不执行真实宿主命令；Codex 侧栏宽度限制使桌面宽度下的手工视觉确认不完整，紧凑宽度行为以组件和 CI 检查为准。

## 发布过程异常与保留限制

初始候选 `db8ab3920570f39521dfd1f95e3e6a2827503c5f` 的候选 CI 和首轮 L3 都拦截了同一问题：RC14 版本元数据更新后 first-party source 摘要仍是 RC13 值。随后统一更新到 `26beea...`，并完成独立源身份审阅；没有把两次拦截重复计为两个根因。初始失败证据与修复后 L3 原件保留于 `C:/GitHub/_release-evidence/v1.26.0-rc.14/`。该异常发生在生产写入前；本次生产写入为 0。

本机 Windows 的 `scripts/tests/source-checks.test.mjs` 另有 vendored `third_party/torrent/.deepsource.toml` 行尾摘要不匹配；没有修改第三方文件。精确 source-scope 检查、Linux 候选/主线 CI 与 L3 均通过，Windows 本地结果不冒充发布 gate 通过。

本轮没有新的 Cloudflare scoped/full 安全审计；既有 CF 覆盖状态与稳定前审计要求继续开放。真实 Linux PTY、真实服务管理写操作、实体手机、读屏、生产升级/回滚均未由本次 Mock 或容器测试证明。生产未部署，stable `latest` 未改变。

## 发布证据

证据根目录：`C:/GitHub/_release-evidence/v1.26.0-rc.14/`，包含 OCR、源身份审阅、L3 原件、Release 附件核验、公开镜像摘要和 E2E 日志。GitHub Release：[v1.26.0-rc.14](https://github.com/kejilion/KPanel/releases/tag/v1.26.0-rc.14)。

<!-- kpanel-release-metrics:start -->
- 首个纳入提交时间：`2026-10-13T01:15:16+08:00`
- 候选冻结时间：`2026-10-13T01:30:46+08:00`
- 生产完成时间：不适用
- 提交到生产用时：不适用
- 是否回滚、紧急热修复或重复发布：否（无生产部署）
- 生产写入后异常次数：0
<!-- kpanel-release-metrics:end -->

<!-- kpanel-release-process-metrics:start -->
- 本版发布流程异常根因数：1
- 其中生产写操作开始后异常次数：0
<!-- kpanel-release-process-metrics:end -->
