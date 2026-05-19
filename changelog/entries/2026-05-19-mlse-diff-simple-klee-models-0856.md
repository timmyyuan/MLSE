# 扩展 mlse-diff 简单 KLEE model

| 字段 | 值 |
| --- | --- |
| 日期 | 2026-05-19 |
| 状态 | 完成 |
| 范围 | scripts、internal/symbolicdiff、SymbolicDiff fixture、文档 |
| 关联请求 | 先修简单的 Motus unsupported case |

## 摘要

本轮只扩展低风险、可边界化的 KLEE model：

- `[]int` nil / empty / full 的 strict 比较，用于 `mod10`。
- 常量 `error` 模型，用于 `mod21` / `mod25`。
- 简单 `*struct{ int field }` 返回值字段比较，用于 `mod39`。

不在本轮处理 context、map、JSON、外部 client、复杂对象图。

## 当前进展

- 已确认 changelog 工作锁为 `空闲`，本轮切换为 `正在执行`。
- 已阅读仓库 symbolic-diff 文档和 Obsidian GoIR symbolic-diff 边界。
- 已实现并绑定 `mod10`、`mod21`、`mod25`、`mod39` 的窄 KLEE model。
- 已同步 README、dev setup 和 Obsidian GoIR 测试 / 集成文档。

## 实现

- `slice_i64` harness 增加 `nil_empty_full` 输入模式和 strict 返回比较，用于捕获 nil slice 与非 nil 空 slice 的差异。
- `go_llvm` harness 增加常量 `error` 返回比较，覆盖 `errors.New` 和无格式参数的 `fmt.Errorf`。
- `go_llvm` harness 增加简单 `ptr_i64` 返回比较，用于比较返回结构体首个整数字段。
- `cmd/mlse-diff` 的自动模型选择允许 `error` 返回的简单签名，但不扩展复杂指针 / map / context 边界。

## 测试

- `go test ./internal/symbolicdiff`
- `python3 -m py_compile scripts/mlse-diff-go-pipeline-probe.py`
- `python3 linters/check_python_metrics.py --root . --include scripts,linters --exclude tmp,artifacts,.git --max-params 5 --max-function-lines 200 --max-file-lines 2000`
- `python3 scripts/mlse-diff-go-pipeline-probe.py --case motus-mod10-foo1-foo2 --case motus-mod21-foo1-foo2 --case motus-mod25-rollback1-rollback2 --case motus-mod39-foo-bar --emit artifacts/simple-klee-selected-probe`
- 对上述四个 case 额外执行本地 harness `llvm-as` / `llvm-link` smoke；本机未安装 KLEE，真实 KLEE 结果交给 CI。
- `python3 scripts/mlse-diff-go-pipeline-probe.py --emit artifacts/simple-klee-full-probe`
- `python3 scripts/mlse-diff-smoke.py --emit artifacts/simple-klee-smoke`
- `go test ./cmd/... ./internal/...`
- `staticcheck ./cmd/... ./internal/...`
- `scripts/test-all.sh`
- `scripts/lint.sh`

## 剩余边界

- `fmt.Errorf` 仍只支持无格式参数的常量错误；带参数格式化会显式进入 unsupported。
- `ptr_i64` 只覆盖返回指针非空时首个 `i64` 字段比较，不代表完整结构体或对象图等价。
- context、map 真实语义、JSON、外部 client 和复杂 stdlib model 继续保留为后续工作。
