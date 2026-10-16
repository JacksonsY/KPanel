# 信任边界安全审计状态

本目录保留 PROJECT_RULES.md 5.4 的可选 CF 辅助产物，执行入口为
.codex-workflows/security-boundary-audit.workflow.yaml。它不是候选、集成、RC 或稳定发布的门禁。
历史补审期限、豁免和试用退出描述保留当时事实，当前行为只遵守 5.4；不自动恢复历史 incomplete。

- run-<N>/ 保存按需入库的最小脱敏元数据、账本、findings 和报告，不要求每次运行都完整入库。
- 新尝试先在仓库外冻结精确源码、固定上游 pin、范围、预算和真实终态。未知成本写 null。
- run-1 至 run-3 沿用原形状；run-4 起记录 scope_mode、run_status、精确 source_ref、source_dirty=false；
  scoped 另含 comparison_base 与 scope_complete。历史 dirty 局限保持，不追溯补造证据。
- 完成覆盖仍按真实已完成 run 的并集计算。分批 reviewed_commits 只计完整核验的精确 SHA，
  scope_complete=false；未完成批次和没有完整声明的 partial 不计覆盖。
- scripts/check-security-audit-coverage.mjs 仅显式查询；14/30 天和新包产生 recommended 建议，
  blocking=false，旧 --require 不再强制补审。--validate 在审计产物入库时显式执行，普通治理不依赖全注册表。
- 入库前复核结构与披露边界；未修复问题的可复用攻击细节留在仓库外或不推送的本地候选。

有价值线索按需进入独立候选修复任务；沿用 [remediation-status.md](remediation-status.md) 追踪具体修复，
无需为原始线索创建空分支。未验证不等于风险关闭；已证实紧急/高风险继续按产品漏洞响应处理。
修复、独立复核、RC、稳定版和部署分别记录，历史 verdict 保持不变。没有新运行就不追加观察样本。
