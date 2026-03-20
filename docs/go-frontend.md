# Go Frontend MVP

这是 MLSE 当前最小可运行的 Go 前端入口。

它现在承担两件事：

1. 继续保留旧的 `GoIR-like` 文本输出，给实验性 `goir-llvm-exp` 路径做覆盖回归。
2. 开始把同一份 Go AST 直接桥接到正式 `go` dialect，作为长期可维护实现的起点。

## 当前命令接口

当前工具是 `cmd/mlse-go`。

它支持两种输出模式：

- `-emit=formal`：默认模式，输出可被 `mlse-opt` 解析的正式 MLIR 子集
- `-emit=goir-like`：兼容模式，继续输出旧的 GoIR-like 文本

运行示例：

```bash
go run ./cmd/mlse-go ./examples/go/simple_add.go
go run ./cmd/mlse-go -emit=goir-like ./examples/go/simple_add.go
```

## 当前支持的最小子集

- 单文件 Go 源码
- package 级函数
- 参数和返回值中的 `int`
- 条件位置和参数位置中的 `bool`
- `string`、`error`、指针、切片的最小类型映射
- 函数体中的：
  - `var` 零值初始化和 `:=` 定义单个局部变量
  - 标识符赋值
  - `+ - * /` 和整数比较二元表达式
  - 受限的 `if` lowering：
    - `if { return ... } else { return ... }`
    - `if { return ... }` 后接顶层 `return ...`
    - `if/else` 对单个外层变量的简单 merge
  - 受限的计数循环 lowering：`for i < n { ...; i = i + 1 }`
  - 直接函数调用与 selector 调用
  - `make([]T, len[, cap])`
  - `nil`
  - `return`
  - 整数 / 字符串字面量

## Formal 模式

默认输出为正式 MLIR 文本，尽量复用标准 dialect：

- 结构：`func.func`、`return`
- 标量计算：`arith`
- 控制流：受限子集的 `scf.if`、`scf.for`
- Go 特有值：`go.string_constant`、`go.nil`、`go.make_slice`

对于当前还没稳定 lower 的形态，前端会显式产出：

- `go.todo`
- `go.todo_value`

这样做的目的不是“假装已经支持这些语义”，而是先保证输出仍然是 parseable MLIR，便于把前端逐步迁到正式 dialect。

示例：

```mlir
module {
  func.func @add(%a: i32, %b: i32) -> i32 {
    %bin1 = arith.addi %a, %b : i32
    return %bin1 : i32
  }
}
```

### 当前 formal 模式的明确边界

- 直线型标量代码支持最好
- 简单 `if` return / merge 形态已开始 lower 到 `scf.if`
- 简单计数循环已开始 lower 到 `scf.for`，当前会用 `arith.index_cast` 桥接 `Go int -> scf index`
- `make([]T, ...)`、字符串字面量、`nil` 已经有正式 `go.*` op 承接
- `switch / range / defer / go` 等结构当前还没有真正 lower 到稳定的标准/正式 IR；会先用 `go.todo` / `go.todo_value` 占位
- 更复杂的 `if / for` 形态，例如多变量 merge、`for init; cond; post`、嵌套控制流和 loop-exit 精确值，目前仍会回退到 `go.todo` / `go.todo_value`
- 这条路径的目标是“先保证正式 dialect 可生成、可解析、可测试”，不是立即追平旧实验链路的覆盖率

## GoIR-like 模式

`-emit=goir-like` 会保留旧的手写文本原型：

- `mlse.if`
- `mlse.for`
- `mlse.switch`
- `mlse.call`
- 以及其它实验性 `mlse.*` 占位文本

这条输出只用于当前实验性 `cmd/mlse-goir-llvm-exp` 路径和批量回归，不代表长期正式接口。

## 当前设计取舍

- 语义分析仍主要依赖 AST 结构，没有接 `go/types` / SSA
- formal 模式优先保证“parseable MLIR”，而不是完整语义保真
- 旧 `goir-like` 模式暂不删除，因为它仍承担 `gobench-eq` 覆盖率摸底和 blocker 定位

## 未来扩展点

1. 接 `go/types`，把名字解析和类型检查做实。
2. 接 SSA，为结构化控制流和更稳定的值建模打基础。
3. 继续把更复杂的 `if / for / switch / range` 从 `go.todo` 收敛成真正的 `func/scf/cf + go.*` contract。
4. 让 `cmd/mlse-go` 的 formal 模式逐步替代旧的 `goir-like` 文本路径。
