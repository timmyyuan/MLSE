# 固化已通过 Motus symbolic-diff CI 测试

| 字段 | 值 |
| --- | --- |
| 日期 | 2026-05-19 |
| 状态 | 完成 |
| 范围 | CI、SymbolicDiff 测试清单、文档 |
| 关联请求 | 把已经通过的 mod* 变成 CI 测试用例 |

## 摘要

本轮把已经通过 KLEE symbolic-diff 的 Motus `mod*` case 从“全量 probe 中的隐式覆盖”提升为“显式 supported-case 测试清单”。

## 当前进展

- 已确认 changelog 工作锁为 `空闲`，本轮切换为 `正在执行`。
- 已阅读仓库 CI / symbolic-diff probe，以及 Obsidian GoIR 测试与集成说明。
- 已新增 supported Motus KLEE case 清单，并让 CI 通过 `--require-case-list` 显式断言这些 case。
- 已同步仓库 dev setup 和 Obsidian GoIR 测试 / 集成说明。

## 实现

- 新增 `test/SymbolicDiff/supported-klee-mod-cases.txt`，列出已通过完整 Go -> LLVM -> KLEE 路径的 Motus case。
- `scripts/mlse-diff-go-pipeline-probe.py` 新增 `--require-case-list`，会把清单中的 case 加入 `summary.required_cases`，并在 case 缺失、出现 blocker 或结果不等于 `expected_status` 时填充 `required_case_failures`。
- `.github/workflows/symbolic-diff.yml` 的 Docker KLEE smoke 继续跑全量 probe，同时要求 supported Motus 清单全部通过。

## 测试

- `python3 -m py_compile scripts/mlse-diff-go-pipeline-probe.py`
- `python3 linters/check_python_metrics.py --root . --include scripts,linters --exclude tmp,artifacts,.git --max-params 5 --max-function-lines 200 --max-file-lines 2000`
- `python3 scripts/mlse-diff-go-pipeline-probe.py --case motus-mod10-foo1-foo2 --require-case-list test/SymbolicDiff/supported-klee-mod-cases.txt --emit artifacts/ci-supported-mods-no-klee-probe --expect-status blocked`
- `python3 scripts/mlse-diff-go-pipeline-probe.py --emit artifacts/ci-supported-mods-full-probe`
- `go test ./cmd/... ./internal/...`
- `staticcheck ./cmd/... ./internal/...`
- `scripts/test-all.sh`
- `scripts/lint.sh`

## 剩余边界

- 本机仍未安装 KLEE；清单内 case 的真实 KLEE 通过性由 GitHub CI Docker job 验证。
