# 安全边界审计报告

## 结论

本次 security-boundary-audit scoped 审计已完成，采用 source-only 方法。覆盖账本中 5/5 个单元为 covered；没有确认发现、没有待验证候选，findings.json 为 []。这只代表本批选定提交的源码审查结论，不代表整条待审提交区间已覆盖。

## 冻结范围

- 仓库：kejilion-panel
- 源码 worktree：C:\GitHub\_codex-tasks\kejilion-panel-codex-settings-user-guides
- Source commit：c4ef4c5d82589ed8bf94dcce5c1c107c75be65da
- Source tree：9cf28dd2868fbdcd3152c0158d57e1ab027d2655
- Comparison base：4c0694aa8e02e46145a775707b8d5a0355f7ce10
- source_dirty: false（审计开始与结束时均复核）
- reviewed_commits: 仅上述功能提交；scope_complete: false
- 固定 skill：security-audit，commit c1c8a8c1471069fb0e188eeaff69b8e8db6564a8

范围展开覆盖了 24 个提交变更文件及直接路由、会话认证、视频 URL 解析、上游获取/解析、缓存和页面渲染调用链。未选择的其它待审提交仍未覆盖。

## 已核验控制

- 两个 tutorial 路由为精确 GET 路由；处理器在 URL 解析和上游工作前调用 requireSession。参见 internal/panel/server.go:453-456,1452-1464、internal/panel/tutorial_metadata.go:67-125。
- 用户输入被限制为支持的平台视频 URL 和格式化 ID；请求发往固定平台端点，拒绝重定向并设置 8 秒超时。参见 internal/panel/tutorial_metadata.go:128-189,237-305。
- JSON、YouTube 页面和图片响应的读取分别有 1 MiB、4 MiB、2 MiB 上限；图片来源 host/path、媒体类型和签名受检查。参见 internal/panel/tutorial_metadata.go:307-405,408-434。
- 元数据缓存是进程内缓存，TTL 为 6 小时、最多 128 项。并发相同 key miss 仍会各自访问上游，且没有全局 provider semaphore；这记录为加固建议，源码审查未证明无会话攻击路径或具体部署攻击者。
- 成功响应使用 private 缓存策略和摘要 ETag；API 默认 no-store（internal/panel/server.go:365-367）。页面将标题作为 Vue 插值文本渲染，视频链接来自固定目录。参见 internal/panel/tutorial_metadata.go:67-125,197-234、web/src/views/SettingsGuidesView.vue:121-150,196-235。

## 加固建议（不是 finding）

可为相同 key 的上游请求增加 singleflight，并为 provider fetch 增加全局或按会话并发限制。源码未校验 Origin/Fetch Metadata；会话 cookie 使用 SameSite=Strict（internal/panel/server.go:1686,1706），降低跨站请求风险，但是否存在可被攻击者控制的同站子域取决于部署，当前没有该部署证据，也没有将其提升为候选。

## 验证与限制

最终 WSL validators 均退出码 0：validate-coverage-ledger.cjs 输出 PASS: 5 coverage units valid，validate-findings.cjs 输出 PASS: 0 findings valid；原始输出保存在 validators/。

本机未建立同时隔离网络、限制资源并限定 scratch 写入的 OS sandbox。因此没有运行目标代码、测试、构建、fixture、浏览器或外部服务。应用代理/CDN/防火墙/主机连接限制及真实域名拓扑未验证。source-only 结论是高可信线索，不是人工安全复核结论。
