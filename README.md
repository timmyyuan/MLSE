# MLSE

MLSE 是一个多语言到 MLIR 的编译基础设施项目。

项目目标是将 `C/C++/Python/Go` 程序转换为 `MLIR`，并支持将 `MLIR` 进一步 lowering 成 `LLVM IR`，形成统一的多语言编译与分析管线。

目前仓库处于“规划已建立、最小原型开始落地”的阶段。除了技术规划文档，仓库中已经补上了一个极小 Go 前端原型，并开始把它桥接到正式 `go` dialect。这份 README 的作用是说明项目目标、当前阶段和文档入口。

## 当前状态

- 状态：初始化中，已落下第一个可运行原型
- 代码：已提供 `cmd/mlse-go` 最小 Go 前端 MVP，以及第一批正式 `go` dialect C++/TableGen 骨架
- 文档：已补充技术规划版 spec、docs 索引、Go 前端说明和正式 GoIR dialect bootstrap 说明
- 目标：继续收敛到真实 frontend / MLIR 管线，并把当前 formal bridge 扩展成可维护实现

## 文档入口

- [仓库规格说明](docs/spec.md)
- [文档索引](docs/README.md)
- [开发环境说明](docs/dev-setup.md)
- [Go 前端说明](docs/go-frontend.md)
- [GoIR 方言 bootstrap 说明](docs/goir-dialect.md)

当前在本机已验证过两条最小可运行路径：

- `scripts/build.sh` + `scripts/test.sh`：构建并测试 Go 主线代码
- `scripts/test-all.sh`：运行仓库当前的统一测试入口，覆盖 Go、linters 和 repo-owned MLIR bridge 样例
- `scripts/build-mlir.sh`：配置并构建最小 `mlse-opt`，可解析 `test/GoIR/ir/` 下样例
- `scripts/lint.sh`：运行仓库当前的 Go/C++/Python 规范检查入口

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
├── linters/
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
- 支持 `int` 参数 / 返回值，以及条件/参数位置的 `bool`
- 支持局部 `var` / `:=`、标识符赋值，以及受限的 selector/index/deref 赋值、整数常量、字符串常量、标识符、`+ - * /`、整数比较、直接调用、`make([]T, ...)`、`nil`、`return`
- 已开始把简单 `if`、标准计数循环和受限 `range` 直接 lower 到 `scf.if` / `scf.for`
- 默认输出可被 `mlse-opt` 解析的正式 MLIR：以 `func/arith` 为主，直接 `len/cap/append`、`append(dst, src...)`、string `index`、以及 slice 下标地址化路径已开始产出 `go.len`、`go.cap`、`go.index`、`go.append`、`go.append_slice`、`go.elem_addr`、`go.load`、`go.store`，必要时补 `go.string_constant`、`go.nil`、`go.make_slice`、`go.todo`、`go.todo_value`

运行示例：

```bash
go run ./cmd/mlse-go ./examples/go/simple_add.go
```

示例输出：

```mlir
module {
  func.func @add(%a: i32, %b: i32) -> i32 {
    %bin1 = arith.addi %a, %b : i32
    return %bin1 : i32
  }
}
```

更多说明见：

- [docs/go-frontend.md](docs/go-frontend.md)

### 正式 GoIR bootstrap

仓库现在已经新增第一批正式 MLIR 工程骨架：

- `include/mlse/Go/IR/`：`go` dialect 的 TableGen 与头文件
- `include/mlse/Go/Conversion/`：Go 专属 lowering / conversion 头文件
- `lib/Go/IR/`：dialect 注册与类型定义实现
- `lib/Go/Conversion/`：Go bootstrap lowering 实现
- `tools/mlse-opt/`：最小 MLIR 驱动，只负责注册 dialect、解析输入并接线显式 lowering 入口
- `test/GoIR/ir/`：正式 GoIR 方向的最小 IR 样本

当前这条线已经落了第一批 `go` 类型和自定义 op：

- 类型：`!go.string`、`!go.error`、`!go.named<...>`、`!go.ptr<T>`、`!go.slice<T>`
- op：`go.string_constant`、`go.nil`、`go.make_slice`、`go.len`、`go.cap`、`go.index`、`go.append`、`go.append_slice`、`go.elem_addr`、`go.field_addr`、`go.load`、`go.store`、`go.todo`、`go.todo_value`

它还没有完整的 frontend / pass / lowering，但已经标志着仓库从“纯 Go 文本原型”进入“真实 MLIR dialect 工程面”；`cmd/mlse-go` 现在已经可以直接产出这套正式 dialect 的最小 parseable 子集。与此同时，`mlse-opt` 现在除了 `--lower-go-builtins` 之外，还新增了一个更完整的 `--lower-go-bootstrap` 入口，用来把当前 `!go.*` 类型和这批 bootstrap op lower 到可继续走 `mlir-opt -> mlir-translate -> LLVM IR` 的 LLVM-legal MLIR。这个 lowering 现在已经不再内嵌在 `tools/mlse-opt/` 下，而是作为可复用实现放进 `Go/Conversion`，`mlse-opt.cpp` 只保留驱动和命令行接线职责。

更多说明见：

- [docs/goir-dialect.md](docs/goir-dialect.md)

## 协作原则

- 文档先行：重要设计、边界和决策先写文档，再进入实现。
- 后端先行：先打通 `MLIR -> LLVM IR`，再叠加各语言前端。
- C/C++ 复用优先：优先集成 `ClangIR/CIR`，避免重复造轮子。
- 分层推进：优先支持可验证的语言子集，再逐步扩大覆盖面。
- 定期清理：周期性移除死代码、过期实验和无主产物，保持仓库可维护。
- 文档与代码一致：一旦代码落地，README 和 spec 需要同步更新。
- 优先可维护性：目录、命名和模块边界要为未来协作留出空间。

## 当前 lint 约定

仓库现在把代码规范检查集中到 `linters/`：

- Go：`gofmt -l`、`go vet`、代码规模阈值检查，以及“单次调用的纯转发 wrapper + 单次调用 callee”检查
- C++：代码规模阈值检查
- Python：`py_compile` 和代码规模阈值检查

默认阈值是：

- 参数个数不超过 `5`
- 函数长度不超过 `200` 行
- 文件长度不超过 `2000` 行

统一入口：

```bash
scripts/lint.sh
```

## 下一步建议

- 建立 Docker 开发环境和统一脚本入口。
- 继续把当前 Go formal bridge 扩展成真实的 frontend / pass 管线。
- 为 `C/C++` 集成启用 `CIR` 的 Clang/ClangIR。
- 定义统一的 frontend contract 和最小可支持语言子集。
- 创建编译驱动、dialect、pass、测试、lint 和 clean 骨架。
- 优先打通 `C/C++ -> CIR -> LLVM IR`，再逐步补 `Python` 和 `Go` 前端。
