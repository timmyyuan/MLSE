# 扩展 mlse-diff KLEE ABI/model 支持

| 字段 | 值 |
| --- | --- |
| 日期 | 2026-05-19 |
| 状态 | 已完成 |
| 范围 | internal/symbolicdiff、scripts、symbolic-diff 测试 |
| 关联请求 | 按“LLVM lowering 负责稳定 ABI，MLSE KLEE 层负责 harness/model/runtime stub”的方案实现 |

## 摘要

本轮目标不是修改 KLEE 本体，而是在 MLSE 侧补齐 symbolic-diff 的 KLEE 输入/输出模型与 harness 生成能力。

优先级：

- 先扩展可稳定映射到 LLVM ABI 的标量、字符串、slice 模型。
- 再让 probe 能用同一组符号输入调用 old/new 并比较可观察输出。
- 对 context、map、外部 client 等复杂 runtime 语义继续保持显式 unsupported，避免伪装成已证明等价。

## 当前进展

- 已确认 changelog 工作锁为 `空闲`，本轮已切换为 `正在执行`。
- 已开始阅读仓库文档与 GoIR / symbolic-diff 相关实现。
- 已新增 `go_llvm` KLEE model，用于 bounded `string` / `[]string` 输入输出。
- 已让 probe 对 old/new 两侧整个 `diffcase.*` 模块符号按 side 重命名，避免 helper 函数在 `llvm-link` 时重名。
- 已同步仓库文档与 Obsidian `mlse设计/02-GoIR` 中的 symbolic-diff 边界。

## 实现

本轮新增的支持边界：

- `internal/symbolicdiff` 生成 `case.json` 时，除原有 scalar `c_model` 和 `slice_i64` 外，会为 `string -> string`、`[]string -> []string` 等签名生成 `go_llvm` model。
- `scripts/mlse-diff-go-pipeline-probe.py` 会根据 `go_llvm` 生成 LLVM IR harness，按 Go lowering ABI 构造 `{ptr,len}` string 和 `{ptr,len,cap}` slice。
- `[]string` 输入包含 nil、非 nil empty、固定长度 symbolic 三种 bounded 形态，用来覆盖 nil/empty 相关分支。
- harness 内提供测试所需 runtime stub：`runtime.add.string`、`runtime.any.box.*`、`runtime.fmt.Sprintf` 的 `%s` 子集、`runtime.makeslice`、`runtime.growslice`、`runtime.newobject`、`runtime.composite.map`、`runtime.panic.index`。
- 数字格式化、context、map 真实语义、外部 client 仍然保持 unsupported / inconclusive，不在本轮伪装成已证明等价。

## 测试

已完成：

```bash
go test ./internal/symbolicdiff
go test ./cmd/... ./internal/...
staticcheck ./cmd/... ./internal/...
python3 -m py_compile scripts/mlse-diff-go-pipeline-probe.py
python3 linters/check_python_metrics.py --root . --include scripts,linters --exclude tmp,artifacts,.git --max-params 5 --max-function-lines 200 --max-file-lines 2000
python3 scripts/mlse-diff-smoke.py --emit artifacts/klee-go-abi-smoke
python3 scripts/mlse-diff-go-pipeline-probe.py --case motus-mod11-normnil1-normnil2 --case motus-mod16-foo1-foo2 --case motus-mod17-foo1-foo2 --emit artifacts/klee-go-abi-selected-probe
python3 scripts/mlse-diff-go-pipeline-probe.py --emit artifacts/klee-go-abi-full-probe
scripts/test-all.sh
scripts/lint.sh
```

本地没有 `klee`，所以 full probe 预期停在 `klee_run_not_requested`；但全量 old/new 均通过 `llvm_as_status=success`，没有 Go/MLIR/LLVM 阶段失败。

还用本地 `llvm-as` / `llvm-link` 对 11 个已有 model case 做了 linked bitcode smoke，覆盖原有 scalar、`[]int` 和新增 `go_llvm` case。

## 剩余边界

- 本机未安装 KLEE，真实 KLEE 探索需要交给 symbolic-diff CI 容器。
- `fmt.Sprintf` 只支持 `%s` 子集；`%d`、`%.1f` 等数字格式化会保持 inconclusive，避免误判。
- `map`、`context.Context`、外部 client 和完整 Go runtime 行为仍不进入本轮模型。
