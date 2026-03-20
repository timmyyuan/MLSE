# Formal GoIR Dialect Bootstrap

这份文档记录仓库里**正式 `go` dialect** 的当前落地内容。

它和 [goir-llvm-experiment.md](goir-llvm-experiment.md) 是并行关系：

- `goir-dialect.md` 记录长期可维护的正式 MLIR 方向
- `goir-llvm-experiment.md` 记录当前纯 Go 实验链路和覆盖率摸底结果

当前仓库不会立刻删除实验路径；实验路径继续承担覆盖回归和边界摸底，而正式 dialect 逐步接手结构化 IR 的长期实现。

## 当前目标

当前这条路线的目标分成两层：

- 在仓库里建立真实的 `CMake + TableGen + lib + tools` MLIR 工程骨架
- 用 `go` dialect 承载第一批 Go 专属类型与最小必要 op
- 复用 `func/arith/scf/cf` 这类标准 dialect 承载大部分结构
- 提供一个最小驱动，证明自定义 `!go.*` 类型 / `go.*` op 可以被 MLIR 正常解析和 round-trip
- 让 `cmd/mlse-go` 开始直接产出正式 `go` dialect 的最小 parseable 子集

## 当前已落地内容

当前已新增：

- `include/mlse/Go/IR/GoBase.td`
- `include/mlse/Go/IR/GoTypes.td`
- `include/mlse/Go/IR/GoDialect.h`
- `include/mlse/Go/IR/GoTypes.h`
- `lib/Go/IR/GoDialect.cpp`
- `tools/mlse-opt/mlse-opt.cpp`
- `test/GoIR/ir/basic_types.mlir`
- `test/GoIR/ir/bootstrap_ops.mlir`
- `test/GoIR/ir/frontend_bridge.mlir`
- 顶层 `CMakeLists.txt` 以及 `include/`、`lib/`、`tools/` 子目录 CMake 入口

## 当前类型与 op 边界

目前正式 `go` dialect` 已落下的类型有：

- `!go.string`
- `!go.error`
- `!go.named<"...">`
- `!go.ptr<T>`
- `!go.slice<T>`

当前已经落地的自定义 op 是：

- `go.string_constant`
- `go.nil`
- `go.make_slice`
- `go.todo`
- `go.todo_value`

这批实体的目的不是“已经完整表达 Go”，而是先把最常见、最稳定、且不适合直接塞进标准 dialect 的 Go 语义骨架放进真正的 MLIR dialect 中，同时为 frontend 迁移提供一个可解析的过渡面。

其中：

- `go.string_constant`：承接 Go 字符串字面量
- `go.nil`：承接带类型的 `nil`
- `go.make_slice`：承接 `make([]T, ...)`
- `go.todo` / `go.todo_value`：承接 frontend 迁移阶段尚未完成的 statement / value lowering

与此同时，frontend bridge 已经开始直接复用标准控制流 dialect：

- 简单 return-`if` 和单变量 merge-`if` 会 lower 到 `scf.if`
- 简单计数循环会 lower 到 `scf.for`
- `go` dialect 当前仍主要保留 Go 特有值和迁移期 placeholder，而不是接管所有控制流结构

## 当前没有做的事

这条正式路线还没有实现：

- `GoIR -> MLSE canonical IR` lowering pass
- `SSA -> GoIR` 导入器
- `lit/FileCheck` 测试基建
- `mlse-opt` 的 pass pipeline 入口

当前 `mlse-opt` 只是最小解析/回显驱动，用来验证 dialect 注册，以及类型 / op 的 parser/printer；它还不是完整 pass runner。

## 构建方式

在已安装 LLVM/MLIR 的环境中，可以用下面方式构建：

```bash
cmake -S . -B build/mlir -G Ninja \
  -DMLIR_DIR="$(brew --prefix llvm@20)/lib/cmake/mlir"
cmake --build build/mlir --target mlse-opt
```

构建后可以用最小样本做 round-trip：

```bash
build/mlir/tools/mlse-opt/mlse-opt test/GoIR/ir/basic_types.mlir
build/mlir/tools/mlse-opt/mlse-opt test/GoIR/ir/bootstrap_ops.mlir
build/mlir/tools/mlse-opt/mlse-opt test/GoIR/ir/frontend_bridge.mlir
build/mlir/tools/mlse-opt/mlse-opt test/GoIR/ir/control_flow_bridge.mlir
```

也可以直接验证前端默认输出：

```bash
go run ./cmd/mlse-go ./examples/go/simple_add.go > /tmp/simple_add.formal.mlir
build/mlir/tools/mlse-opt/mlse-opt /tmp/simple_add.formal.mlir
```

## 设计取舍

这条正式路线当前刻意把 Go 专属内容收敛到“类型 + 少量必要 op”，而不是一开始就把所有控制流和算术都塞进 `go` dialect。

原因是：

- 仓库 spec 的长期方向是“标准 dialect 为主，自定义最小化”
- `func/arith/scf/cf` 已经足够承载大量结构化语义
- `go` dialect 应主要保留 Go 无法自然下沉到标准 dialect 的部分

因此，下一步更合理的顺序是：

1. 继续补最关键的 Go 类型和属性
2. 继续把 `cmd/mlse-go` 的 formal 输出从 `go.todo` 收敛到真正的结构化 lowering
3. 建立 `SSA -> go dialect` 的 golden import
4. 再实现 `go -> canonical` lowering 和 verifier
