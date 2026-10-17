# 安全审计状态

现行方法是 PROJECT_RULES.md 5.4 的安全盲审，执行入口为 `.codex-workflows/security-boundary-audit.workflow.yaml`。
本目录保存它的固定输入和 CF 时期的历史产物。

## 现行文件

- `boundary-policy.json` 的 `securityReview.paths`：自动参与的安全敏感路径前缀，按所在目录分组；
  `scripts/security-review-scope.mjs` 是唯一的范围与 trailer 计算入口。其余字段供 CF 历史覆盖查询使用。
- `review-prompt.md`：交给复核者的固定提示。修改它即修改审计方法。
- `review-benchmark.json`：审计器回放基准，13 条已修复、已公开的真实漏洞及 2026-10-17 基线结果。
- `remediation-status.md`：历史 CF 发现的修复交付追踪，原文保留。

盲审不入库运行产物。状态只记在候选提交的 `Security-Review:` trailer；未修复线索的细节只放仓库外。

## 回放基准

修改 `review-prompt.md`、默认做法或复核方式前执行：

1. 对每个 case 导出修复前快照，只含列出的路径和规范上下文，不含 Git 历史：
   `git archive <fix>^ -- <paths...> PROJECT_RULES.md docs/development-quality-standard.md | tar -x -C <dir>/<case>`。
2. 每个 case 交给一名新的复核者，提示正文只替换快照路径；复核者不得得知缺陷。
3. 在看到任何结果之前已写定判分口径（见 `scoring`）。按缺陷 ID 判命中，跨 case 命中计入对应缺陷。
4. 命中数不得低于 `baseline.hits`；记录耗时、调用数与 token，作为新基线与提示修改一起提交。

新修复的真实漏洞（尤其是盲审漏掉、后来由其他途径发现的）补入 `cases`，保留修复提交与路径即可，
不写攻击细节。2026-10-17 基线：命中 G1、G4、G5、G7、G8、G9、G12，共 7/13；同一版本上
CodeQL、Semgrep、gosec 命中 0/13。原始输出与扫描结果保存在仓库外证据目录，不入库。

## CF 历史

`run-*` 为 CF（cloudflare/security-audit-skill）时期按需入库的最小脱敏产物；历史 verdict、incomplete
状态和补审/豁免记录按当时事实保留，不改写为通过。`scripts/check-security-audit-coverage.mjs` 仍可只读查询
历史覆盖并集，输出 blocking=false，不进入任何门禁。用户明确要求深度审计时才可单独执行 CF，
其发现同样按 5.4 逐条核实；含攻击细节的 FINDINGS-DETAIL、findings.json 等仍受
`check-governance-consistency.mjs` 的披露检查约束。
