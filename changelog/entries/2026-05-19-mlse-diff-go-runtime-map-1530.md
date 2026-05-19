# Go symbolic-diff 简化 runtime 与 map 模型

| 字段 | 内容 |
| --- | --- |
| 状态 | 验证中 |
| 关联请求 | 将 Go 基本数据结构实现在简化 runtime 中，避免直接建模或编译 Go runtime |
| 开始时间 | 2026-05-19 15:30 CST |
| 结束时间 | 无 |

## 背景

当前 `go_llvm` KLEE harness 已能覆盖一批 bounded string / slice / pointer / error / helper 语义，但 map、对象图和外部 runtime 仍然主要以 case-specific stub 呈现。用户希望把 Go 基本数据结构收敛到一个简化 runtime 中，避免直接模拟 Go runtime 的 `hmap` 或编译完整 Go runtime。

## 设计约束

- 保持真实 `Go -> MLIR -> LLVM IR -> KLEE` 链路，不引入文本重写或 compatibility bridge。
- 简化 runtime 只承载 repo-owned bounded 语义模型；没有覆盖到的 Go 语义继续显式 unsupported。
- map 模型优先表达 Go 语义可观察行为，而不是 Go runtime 内存布局。

## 工作记录

- 2026-05-19 15:30：确认 `changelog/status.md` 为 `空闲`，从 `origin/main` 新建 `codex/mlse-diff-go-runtime-map` 分支。
- 2026-05-19 15:30：开始阅读仓库 README / spec / symbolic-diff 说明和 Obsidian GoIR 设计文档，定位现有 KLEE runtime stub 的边界。
- 2026-05-19 15:36：将 `go_llvm` KLEE runtime IR bundle 从 `scripts/mlse-diff-go-pipeline-probe.py` 拆到 `scripts/mlse_diff_go_runtime.py`，避免 probe 入口继续承载大量 runtime 细节。
- 2026-05-19 15:39：新增 bounded `map[string]string` 简化 runtime：old/new 分别获得同构 map，`runtime.store.index.map` 写入后通过 `__mlse_map_string_string_equal` 比较可观察副作用。
- 2026-05-19 15:40：把 `motus-mod19-foo1-foo2` 从 `klee_model_unavailable` 升级为 `go_llvm` supported case，并加入 CI supported case 清单。
- 2026-05-19 15:42：使用 fake KLEE 验证 `mod19` 和 `mod27` / `mod28` 相关 map stub 可以完成 harness assemble / link；随后验证 supported 等价子集没有因为 runtime 拆分退化。
- 2026-05-19 15:44：同步更新仓库 README / dev setup，以及 Obsidian `mlse设计/02-GoIR` 的构建和测试说明。
- 2026-05-19 15:44：本地通过 `python3 -m py_compile ...`、`python3 scripts/mlse-diff-smoke.py`、`go test ./cmd/... ./internal/...`、`staticcheck ./cmd/... ./internal/...`、`scripts/test-all.sh`、`scripts/lint.sh`。
- 2026-05-19 15:44：进入 PR / GitHub CI 验证阶段。

## 待更新

- 等待 PR 上的真实 GitHub Docker/KLEE CI 验证 `mod19`。
