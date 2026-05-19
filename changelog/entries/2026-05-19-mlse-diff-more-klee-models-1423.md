# 扩展 Motus symbolic-diff KLEE model

| 字段 | 内容 |
| --- | --- |
| 状态 | 验证中 |
| 关联请求 | 先解决当前优先级较高的 KLEE unsupported Motus case |
| 开始时间 | 2026-05-19 14:23 CST |
| 结束时间 | 未完成 |

## 背景

上一轮全量 probe 显示 `mod12`、`mod13`、`mod15`、`mod18`、`mod19`、`mod20`、`mod22`、`mod23`、`mod24`、`mod29`、`mod30`、`mod31`、`mod32`、`mod33`、`mod34`、`mod36`、`mod37` 都已经能走到 Go -> MLIR -> LLVM bitcode，但尚未进入真实 KLEE 证明。阻塞点是 `klee_model_unavailable`，即 case 缺少 KLEE 输入/返回值/运行时模型。

本轮先处理低成本集合：

- `mod20`：无返回值，主要观察 panic / 模型错误即可。
- `mod18`：无返回值，额外需要支持 `any` 装箱本地 struct 指针。
- `mod34`：`(bool, error)` 返回、`any -> bool` type assertion、简单全局指针。
- `mod29` / `mod30`：AddOwners 简化版，需要 request/plugin 字段、绑定/响应/log no-op 和 `[]string` 返回比较。
- `mod12`：`context + string + []string -> []string`，需要 dal / set / PSM 字段和 slice helper 的 bounded 模型。

## 设计约束

- 继续只允许真实 `Go -> MLIR -> LLVM IR -> KLEE` 链路，不引入文本重写或兼容性桥接。
- KLEE model 必须是 repo-owned fixture 的 bounded 语义模型；没有建模到的 runtime 语义继续保持显式 unsupported。
- 新增 supported case 后必须加入 CI 清单，避免回退到 `expected_blocker`。

## 工作记录

- 2026-05-19 14:23：确认 `changelog/status.md` 为 `空闲`，从 `origin/main` 新建 `codex/mlse-diff-more-klee-models` 分支。
- 2026-05-19 14:23：阅读仓库 README / spec，以及 `mlse设计/02-GoIR` 的测试和集成文档，确认本轮属于 symbolic-diff KLEE harness 边界扩展。
- 2026-05-19 14:36：为 `go_llvm` KLEE harness 增加 `void` 返回、`(bool,error)` 返回、strict `[]string` 返回比较，以及 string-slice helper。
- 2026-05-19 14:36：为 `mod12`、`mod18`、`mod20`、`mod29`、`mod30`、`mod34` 补 repo-owned bounded runtime stubs 和 `case.json` KLEE model，并加入 `supported-klee-mod-cases.txt`。
- 2026-05-19 14:37：fake-KLEE 验证 `mod12`、`mod18`、`mod20`、`mod29`、`mod34` 可以构造 harness、`llvm-as`、`llvm-link` 并得到 fake equivalent；`mod30` 也完成 harness/链接，但 fake KLEE 不会生成 `assert.err`，因此真实 counterexample 留给 Docker CI 验证。
- 2026-05-19 14:38：同步更新仓库 README / dev setup，以及 Obsidian `mlse设计/02-GoIR` 的构建和测试说明。
- 2026-05-19 14:43：本地通过 `python3 -m py_compile scripts/mlse-diff-go-pipeline-probe.py scripts/mlse-diff-smoke.py scripts/mlse-diff-fuzz-smoke.py`、`go test ./cmd/... ./internal/...`、`staticcheck ./cmd/... ./internal/...`、`scripts/test-all.sh`、`scripts/lint.sh`。
