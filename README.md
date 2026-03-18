# MLSE

MLSE 是一个多语言到 MLIR 的编译基础设施项目。

项目目标是将 `C/C++/Python/Go` 程序转换为 `MLIR`，并支持将 `MLIR` 进一步 lowering 成 `LLVM IR`，形成统一的多语言编译与分析管线。

目前仓库处于“规划已建立、最小原型开始落地”的阶段。除了技术规划文档，仓库中已经补上了一个极小 Go 前端原型，用于验证 `Go -> MLIR-like text` 的第一条链路。这份 README 的作用是说明项目目标、当前阶段和文档入口。

## 当前状态

- 状态：初始化中，已落下第一个可运行原型
- 代码：已提供 `cmd/mlse-go` 最小 Go 前端 MVP
- 文档：已补充技术规划版 spec、docs 索引和 Go 前端说明
- 目标：先打通可验证的最小链路，再逐步替换为真实 frontend / MLIR 管线

## 文档入口

- [仓库规格说明](docs/spec.md)
- [文档索引](docs/README.md)

## Agent 约定

- 后续 agent 写代码前，先看仓库根目录的 `AGENTS.md`。
- `AGENTS.md` 中说明了如何定位 Obsidian 里的 `mlse设计` 文档，以及哪些改动需要同步设计文档。

## 当前阶段目标

1. 明确多语言前端到 MLIR 的总体架构。
2. 明确 MLIR 到 LLVM IR 的后端路径与模块边界。
3. 制定可分阶段推进的语言支持策略和里程碑。
4. 用小而可运行的原型验证各语言前端的最小落地路径。

## 建议目录结构

下面的结构是推荐目标，而不是当前已经存在的内容：

```text
.
├── README.md
├── docs/
│   ├── README.md
│   └── spec.md
├── docker/
├── include/mlse/
├── lib/
├── tools/
├── test/
├── scripts/
└── examples/
```

## 当前可运行原型

### `cmd/mlse-go`

这是一个面向 Go 子集的最小前端原型。

当前能力：

- 读取单个 `.go` 文件
- 解析 package 级函数
- 支持 `int` 参数 / 返回值
- 支持局部 `:=`、整数常量、标识符、`+ - * /`、`return`
- 输出简化的 MLIR-like module 文本

运行示例：

```bash
go run ./cmd/mlse-go ./examples/go/simple_add.go
```

示例输出：

```mlir
module {
  func.func @add(%a: i32, %b: i32) -> i32 {
    %c = arith.addi %a, %b : i32
    return %c : i32
  }
}
```

更多说明见：

- [docs/go-frontend.md](docs/go-frontend.md)

## 协作原则

- 文档先行：重要设计、边界和决策先写文档，再进入实现。
- 后端先行：先打通 `MLIR -> LLVM IR`，再叠加各语言前端。
- C/C++ 复用优先：优先集成 `ClangIR/CIR`，避免重复造轮子。
- 分层推进：优先支持可验证的语言子集，再逐步扩大覆盖面。
- 定期清理：周期性移除死代码、过期实验和无主产物，保持仓库可维护。
- 文档与代码一致：一旦代码落地，README 和 spec 需要同步更新。
- 优先可维护性：目录、命名和模块边界要为未来协作留出空间。

## 下一步建议

- 建立 Docker 开发环境和统一脚本入口。
- 先实现基于手写 MLIR 的 `LLVM IR` 输出链路。
- 为 `C/C++` 集成启用 `CIR` 的 Clang/ClangIR。
- 定义统一的 frontend contract 和最小可支持语言子集。
- 创建编译驱动、dialect、pass、测试、lint 和 clean 骨架。
- 优先打通 `C/C++ -> CIR -> LLVM IR`，再逐步补 `Python` 和 `Go` 前端。
