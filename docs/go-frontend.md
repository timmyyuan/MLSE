# Go Frontend MVP

这是 MLSE 当前最小可运行的 Go 前端入口。

当前工具是 `cmd/mlse-go`，它只输出正式 `go` dialect bridge，不再保留旧的文本实验模式。

运行示例：

```bash
go run ./cmd/mlse-go ./examples/go/simple_add.go
```

如果需要按 `internal/gofrontend/formal_*` 文件查看 lowering 入口、源码前后对照和测试夹具，请直接看 [docs/go-frontend-lowering.md](go-frontend-lowering.md)。

## 当前支持的最小子集

- 单文件 Go 源码
- package 级函数
- 参数和返回值中的 `int`
- 参数和返回值中的常见整数宽度映射：`int/int32`、`int64`、`byte/int8`、`int16`
- 条件位置和参数位置中的 `bool`
- `string`、`error`、指针、切片的最小类型映射
- 函数体中的：
  - `var` 零值初始化和 `:=` 定义单个局部变量
  - 标识符赋值
  - 受限的非标识符赋值：blank identifier 丢弃、field selector 赋值、index 赋值、`*ptr = v`
  - `+ - * /`、`&& / ||`、`!` 和整数比较二元表达式
  - 带显式类型上下文的 selector value 读取，包括 package selector 和字段 selector 比较上下文，例如 `x + pkg.GlobalValue`、`ctx.Owner != ""`
  - 受限的 `if` lowering：
    - `if { return ... } else { return ... }`
    - `if { return ... }` 后接顶层 `return ...`
    - 以 returning region 递归嵌套的 `if { return ... }`
    - 前面带 prefix 语句、后面进入 returning region 的 `if { return ... }`
    - 多返回值函数上的 early-return `if`
    - `if/else` 对单个或多个外层变量的简单 merge
  - 受限的计数循环 lowering：`for i < n { ...; i = i + 1 }`
  - 受限的 `range` lowering：基于 helper `len/index` 的 `for _, v := range xs`
  - 直接函数调用与 selector 调用
- 立即 method call，例如 `x.Len()`、`pb.Do2Bo()`
- 立即 method call 当前会统一命名成 `package.receiver.method`
  - method receiver 绑定成 formal 函数首参
  - 非捕获的 `func(...) { ... }` 字面量
  - 函数类型参数上的间接调用
  - `make([]T, len[, cap])`
  - 非 slice `make(...)` 的 typed helper 路径，例如 `make(map[K]V)`
  - 受限的 `CompositeLit`：typed helper 字面量，以及空 slice literal `[]T{}`
  - `nil` 和非 nilable opaque 类型上的 zero-value helper 兜底
  - 带明确目标类型的 helper-based coercion / conversion，例如 `int64(0)`、`string(x)` 和 placeholder 返回值的 typed return
  - `return`
  - 整数 / 字符串字面量

## 当前输出形态

前端输出正式 MLIR 文本，尽量复用标准 dialect：

- 结构：`func.func`、`return`
- 函数值：`func.constant`、`func.call_indirect`
- 标量计算：`arith`
- 控制流：受限子集的 `scf.if`、`scf.for`
- Go 特有值 / builtin / aggregate：`go.string_constant`、`go.nil`、`go.make_slice`、`go.len`、`go.cap`、`go.index`、`go.append`、`go.append_slice`
- Go 特有比较：`go.eq`、`go.neq`
- Go 特有地址/内存桥接：`go.elem_addr`、`go.field_addr`、`go.load`、`go.store`

当前 frontend 已开始在直接 `len/cap` builtin、string `IndexExpr`、`append`、`append(dst, src...)`、slice `IndexExpr`、一部分 Go 值比较，以及受限 `range` 绑定路径里直接产出 `go.len`、`go.cap`、`go.index`、`go.append`、`go.append_slice`、`go.eq`、`go.neq`、`go.elem_addr`、`go.load`、`go.store`；但还没有覆盖所有 builtin / aggregate 形态。

对于当前还没稳定 lower 的形态，前端会显式产出：

- `go.todo`
- `go.todo_value`

这样做的目的不是“假装已经支持这些语义”，而是先保证输出仍然是 parseable MLIR，便于把前端逐步迁到正式 dialect。

示例：

```mlir
module {
  func.func @demo.add(%a: i32, %b: i32) -> i32 {
    %bin1 = arith.addi %a, %b : i32
    return %bin1 : i32
  }
}
```

## 当前明确边界

- 直线型标量代码支持最好
- 简单 `if` return / merge 形态已开始 lower 到 `scf.if`
- 简单计数循环已开始 lower 到 `scf.for`，当前会用 `arith.index_cast` 桥接 `Go int -> scf index`
- 受限 `range` 现在也会直接 lower 到 `scf.for`；对 slice value 绑定优先走 `go.elem_addr + go.load`
- 具备可操作 underlying 的 named type，例如 named slice / string / pointer，当前会优先通过 `go/types` 还原成正式桥接类型；因此这类值现在在签名、局部显式类型、`make(T, ...)` 和 type conversion 上会优先收成 `!go.slice<...>`、`!go.string`、`!go.ptr<...>`，不再因为 `PlaybookList` 这类 named slice 一路退回 `__mlse_range_len__...` / `__mlse_index__...`
- 指向切片、切片元素仍为切片的类型，例如 `*[][]byte`，当前也会保留完整桥接类型，不再把 pointee 或元素错误压扁成 `!go.named<"element">`
- 当 selector value 明确出现在标量/返回值类型上下文里时，当前会先把它收成 extern value read，而不是直接退回 `go.todo_value`
- field selector 读写和 `*ptr` 读写在当前能形成 concrete pointer / named aggregate 边界时，会优先收成 `go.field_addr`、`go.load`、`go.store`
- slice 下标读写在当前会优先收成 `go.elem_addr`、`go.load`、`go.store`；`go.index` 主要保留给 string 这类值语义索引
- `map` index 更新、以及仍拿不到稳定地址模型的 selector / deref，当前还会保留 helper 路径
- method receiver 当前会显式进入 formal 函数签名，避免在方法体里生成未声明 SSA
- `nil` 如果落在非 nilable opaque 类型上下文，当前会先 materialize 成 helper zero value，而不是生成非法 `go.nil`
- 非捕获 `FuncLit` 当前会先收成 module 内私有 helper `func.func`，并在值位置产出 `func.constant`，在函数值调用点产出 `func.call_indirect`
- 立即 method call 当前会优先收成稳定的 `package.receiver.method` 符号调用，不再一律退回 `indirect_call`
- 顶层函数当前会统一命名成 `package.func`
- 非捕获 `FuncLit` 当前会统一命名成 `enclosing.__litN`
- package selector value 当前会优先按 import path 产出稳定符号，而不是继续依赖源码里的 alias 名
- `CompositeLit` 当前至少已经覆盖 typed helper 字面量和空 slice literal；因此不再统一落回 `go.todo_value "CompositeLit"`
- 多返回值函数上的 returning-`if` 当前也已经开始直接 lower 到多结果 `scf.if`
- `for / range` 循环体里的函数级 early-return，当前也开始通过 `scf.for iter_args(stop, done, ret...)` 这类标准结构化形式 lower；简单 `break` 会直接把 loop-local `stop` 置位，不再落回 `go.todo "BranchStmt"`
- `make([]T, ...)`、字符串字面量、`nil` 已经有正式 `go.*` op 承接；非 slice `make(...)` 当前会先走 typed helper call
- `string` 比较、pointer 比较，以及 `error == nil` / `error != nil`、`slice == nil` / `slice != nil` 这类当前语义边界已经明确的 case，会优先收成 `go.eq` / `go.neq`；其余 opaque compare 当前仍可能保留 helper 路径
- classic `for` 的简单 counted 形态当前也开始直接 lower 到 `scf.for`
- 控制流不允许再用 statement helper escape hatch 伪装成 extern call；像 `IfStmt_returning_region`、`IfStmt_condition`、复杂 `ForStmt` 这类还没结构化收敛的 case，当前会显式保留成 `go.todo`
- 这轮 scheme B 之后，repo-owned fixture 和 `mlse-opt --lower-go-bootstrap` 已经覆盖 `go.field_addr` / `go.load` / `go.store`
- 外部 gobench probe 当前这轮已经达到全绿；最新结果以 `artifacts/go-gobench-mlir-suite/summary.{md,json}` 为准，当前是 `frontend_success = 249/249`、`mlse_opt_success = 249/249`、`llvm_eligible = 249/249`、`llvm_verify_success = 249/249`
- 类型断言、解引用、浮点/非整数算术这类值语义长尾，当前仍可能通过 typed helper call 维持可解析性；但这不再扩展到控制流结构
- `switch / defer / go` 等结构当前还没有真正 lower 到稳定的标准/正式 IR；会先用 `go.todo` / `go.todo_value` 占位
- 捕获外层变量的 `FuncLit` 当前仍然没有 closure 表示，仍会回退到 `go.todo_value`
- 更复杂的 `if / for` 形态当前不会再伪装成控制流 helper；后续继续逐步替换成更结构化的 lowering

## 当前设计取舍

- 语义分析仍主要依赖 AST 结构，但当前已经接入了一层可退化的最小 `go/types` 上下文，用来改善 package selector、方法归属和调用签名；整体还不是 typed-first frontend，更没有接 SSA
- 当前实现优先保证“parseable MLIR”，而不是完整语义保真
- 旧的文本实验已经移除，后续演进都围绕 formal bridge 继续收敛

## 未来扩展点

1. 继续把这层最小 `go/types` 上下文扩成真正可依赖的名字解析和类型检查层。
2. 接 SSA，为结构化控制流和更稳定的值建模打基础。
3. 继续把更复杂的 `if / for / switch / range` 从 `go.todo` 收敛成真正的 `func/scf/cf + go.*` contract。
