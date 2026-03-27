# Agent Guide

本仓库的代码实现与设计文档需要一起看。

在开始写代码前，先阅读两类内容：

1. 仓库内文档
2. Obsidian 中的 `mlse设计`

## 1. 必读顺序

建议按下面顺序建立上下文：

1. `README.md`
2. `docs/spec.md`
3. Obsidian vault `next` 中的 `mlse设计/00-MLSE设计文档.md`
4. 本次修改直接相关的语言文档：
   - `mlse设计/01-PythonIR/`
   - `mlse设计/02-GoIR/`

## 2. 如何定位 Obsidian 文档

不要在仓库文档里写绝对路径。

如果当前环境里不知道 Obsidian vault 的具体位置，先从用户主目录搜索：

```bash
find ~ -path '*/next/mlse设计/00-MLSE设计文档.md' 2>/dev/null | head -n 1
```

找到总入口后，再进入对应语言目录继续阅读。

## 3. 修改代码时的要求

- 如果改动涉及架构、前端边界、方言、pass、运行时或测试策略，先看 Obsidian 设计文档再动手。
- 如果实现与设计不一致，先判断是设计落后于实现，还是实现偏离设计。
- 如果改动改变了既有设计约束，代码完成后要同步更新 Obsidian 中的 `mlse设计`。
- GoIR 到 LLVM 的链路禁止引入“兼容性桥接”式的绕过手段。不要通过额外文本重写、opaque-handle 替换或临时 helper 注入，把仍然含有 unresolved `go` dialect 语法的 MLIR 伪装成可继续下游的输入。
- 允许进入 `MLIR -> LLVM IR` 流程的输入，必须是已经过 `mlse-opt` round-trip，且不再含有 unresolved `go` dialect 语法的 MLIR。换句话说，只允许真正的 `MLIR -> LLVM IR`，不允许用 compatibility lowering 之类的中间层掩盖前端或 dialect 尚未完成的语义。
- Go frontend 的控制流 lowering 禁止使用 helper-based escape hatch。`if / for / range / branch` 这类结构要么下沉到标准控制流 dialect（如 `scf` / `cf` / `func`），要么显式保留为 `go.todo` / `go.todo_value`，不要把未完成的控制流语义伪装成 `func.call @__mlse_stmt_*` 之类的 extern helper。

## 4. 文档同步要求

以下变更默认需要同步文档：

- 新增或删除前端阶段
- 新增或删除 dialect / op / pass
- 修改 `PythonIR` 或 `GoIR` 的支持边界
- 修改 Docker、test、lint、clean 等工程约束
- 修改默认命令接口、输出产物或目录约定

## 5. 默认开发流程

除非用户明确要求只做分析或只改文档，默认按下面顺序工作：

1. 先阅读 `AGENTS.md`、相关仓库文档，以及必要的 `mlse设计`。
2. 先做局部、针对性的修改，不要一开始就大面积重构。
3. 改完后先跑**与本次改动直接相关**的检查。

常见局部检查：

- 改 Go frontend：`go test ./internal/gofrontend`
- 改 lint 逻辑：`go test ./linters`
- 改 `cmd/mlse-go`：`go build -o artifacts/bin/mlse-go ./cmd/mlse-go`
- 改 formal bridge / LLVM 可达性：`python3 scripts/go-gobench-mlir-suite.py --skip-build`

4. 局部检查通过后，再跑仓库主线检查：

- `go test ./cmd/... ./internal/...`
- `staticcheck ./cmd/... ./internal/...`

5. 如果改动涉及测试入口、MLIR bridge、dialect/build 集成，或者你需要跑当前仓库内“统一完整测试面”，再跑：

- `scripts/test-all.sh`

这条入口当前会覆盖：

- `scripts/test.sh`
- `go test ./linters`
- `scripts/build.sh`
- `scripts/build-mlir.sh`
- `mlse-opt` 解析 `test/GoIR/ir/*.mlir`
- `cmd/mlse-go` 桥接验证部分 `examples/go/*.go`

6. 最后跑 lint：

- `scripts/lint.sh`

如果某一步失败，先修掉失败，再继续后面的步骤，不要跳过并假装链路已经验证完。

## 6. 风格约束

- 不要在仓库文件中写绝对路径。
- 文档中引用 Obsidian 内容时，使用 vault 名和相对逻辑路径。
- 如果需要告诉后续 agent 如何找到文档，优先提供搜索命令，而不是硬编码本机路径。
