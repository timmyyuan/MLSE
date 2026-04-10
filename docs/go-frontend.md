# Go Frontend MVP

这是 MLSE 当前最小可运行的 Go 前端入口。

当前默认按 target-sized 规则处理 `int` / `uint` / `uintptr`：

- 优先读取环境里的 `GOARCH`
- 没有显式 `GOARCH` 时退回当前运行机的 `runtime.GOARCH`
- 32 位目标收成 `i32`
- 64 位目标收成 `i64`

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
- 参数和返回值中的常见整数宽度映射：target-sized `int/uint/uintptr`、显式 `int32/uint32`、`int64/uint64`、`byte/int8`、`int16`
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
  - 可安全落到当前整数宽度的 typed package const，例如 `math.MaxInt32`
- 立即 method call，例如 `x.Len()`、`pb.Do2Bo()`
- 立即 method call 当前会统一命名成 `package.receiver.method`
  - method receiver 绑定成 formal 函数首参
  - 非捕获的 `func(...) { ... }` 字面量
  - 函数类型参数上的间接调用
  - `make([]T, len[, cap])`
  - 非 slice `make(...)` 的固定 runtime ABI 路径，例如 `runtime.make.map`
  - 受限的 `CompositeLit`：typed helper 字面量，以及空 slice literal `[]T{}`
  - 受限的取址聚合字面量：`&T{Field: v}` 和 `new(T)` 在当前静态布局已知时会优先走 `runtime.newobject(size, align)`，再用 `go.field_addr + go.store` 初始化具名字段；拿不到稳定布局的路径仍会回退到旧 helper
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
- 源码元数据：module 级 `go.scope_table`，以及 `func.func` / op 上的 `loc(...)`

当前 frontend 已开始在直接 `len/cap` builtin、string `IndexExpr`、`append`、`append(dst, src...)`、slice `IndexExpr`、一部分 Go 值比较，以及受限 `range` 绑定路径里直接产出 `go.len`、`go.cap`、`go.index`、`go.append`、`go.append_slice`、`go.eq`、`go.neq`、`go.elem_addr`、`go.load`、`go.store`；但还没有覆盖所有 builtin / aggregate 形态。

当前 formal 输出还会尽量保留源码定位：

- module 顶层会挂 `go.scope_table`，记录函数和当前已建模的 `if / for / range / funclit` scope
- `func.func` 会带 `go.scope = <id>` 属性，指回对应 scope
- 前端直接发射的 op 会带 `loc("scopeN"("path.go":line:col))`
- 如果调用方设置 `MLSE_SOURCE_DISPLAY_PATH`，frontend 会优先用它作为 metadata 里的展示路径；suite staging 场景靠这个把临时 GOPATH 路径还原成原始 case 路径

这层 metadata 的目标是让 frontend dump、`mlse-opt` 诊断和 bootstrap lowering 至少保住源码文件/行列与近邻作用域；它不是 LLVM debug info 的替代品

对于当前还没稳定 lower 的形态，前端会显式产出：

- `go.todo`
- `go.todo_value`

这样做的目的不是“假装已经支持这些语义”，而是先保证输出仍然是 parseable MLIR，便于把前端逐步迁到正式 dialect。

示例：

下面这段用 64 位 target 举例；如果你用 32 位 `GOARCH`，对应 `int` 会是 `i32`。

```mlir
module attributes {go.scope_table = [...]} {
  func.func @demo.add(%a: i64, %b: i64) -> i64 attributes {go.scope = 0 : i64} {
    %bin1 = arith.addi %a, %b : i64 loc("scope0"("testdata/default_simple_add.go":5:7))
    return %bin1 : i64 loc("scope0"("testdata/default_simple_add.go":6:2))
  } loc("scope0"("testdata/default_simple_add.go":4:1))
}
```

## 当前明确边界

- 直线型标量代码支持最好
- `int` / `uint` / `uintptr` 当前已经不再硬编码成 `i32`，而是按 target-sized 规则 lower；因此 `len/cap`、`make([]T, ...)`、`range` 绑定、以及 `math.MaxInt` 这类 package const 的 materialize 边界都会跟着目标字宽变化
- 简单 `if` return / merge 形态已开始 lower 到 `scf.if`
- 简单计数循环已开始 lower 到 `scf.for`，当前会用 `arith.index_cast` 桥接 `Go int -> scf index`
- 受限 `range` 现在也会直接 lower 到 `scf.for`；对 slice value 绑定优先走 `go.elem_addr + go.load`
- 具备可操作 underlying 的 named type，例如 named slice / string / pointer，当前会优先通过 `go/types` 还原成正式桥接类型；因此这类值现在在签名、局部显式类型、`make(T, ...)` 和 type conversion 上会优先收成 `!go.slice<...>`、`!go.string`、`!go.ptr<...>`，不再因为 `PlaybookList` 这类 named slice 一路退回 `runtime.range.len.*` / `runtime.index.*`
- 指向切片、切片元素仍为切片的类型，例如 `*[][]byte`，当前也会保留完整桥接类型，不再把 pointee 或元素错误压扁成 `!go.named<"element">`
- 当 selector value 明确出现在标量/返回值类型上下文里时，当前会先把它收成 extern value read，而不是直接退回 `go.todo_value`
- field selector 读写和 `*ptr` 读写在当前能形成 concrete pointer / named aggregate 边界时，会优先收成 `go.field_addr`、`go.load`、`go.store`；如果 `go/types + sizes` 已经能算出静态字节偏移，当前还会把它作为 `offset` 属性直接打进 `go.field_addr`
- slice 下标读写在当前会优先收成 `go.elem_addr`、`go.load`、`go.store`；`go.index` 主要保留给 string 这类值语义索引
- `map` index 更新、以及仍拿不到稳定地址模型的 selector / deref，当前还会保留 helper 路径
- 这些残留 helper family 当前统一并入 runtime ABI 命名：`&x` 的 fallback 会走 `runtime.addrof.*`，opaque conversion 会走 `runtime.convert.*.to.*`，而 `map` index 更新会走 `runtime.store.index.*`
- method receiver 当前会显式进入 formal 函数签名，避免在方法体里生成未声明 SSA
- `nil` 如果落在非 nilable opaque 类型上下文，当前会先 materialize 成 helper zero value，而不是生成非法 `go.nil`
- 非捕获 `FuncLit` 当前会先收成 module 内私有 helper `func.func`，并在值位置产出 `func.constant`，在函数值调用点产出 `func.call_indirect`
- 立即 method call 当前会优先收成稳定的 `package.receiver.method` 符号调用，不再一律退回 `indirect_call`
- 顶层函数当前会统一命名成 `package.func`
- 非捕获 `FuncLit` 当前会统一命名成 `enclosing.__litN`
- package selector value 当前会优先按 import path 产出稳定符号，而不是继续依赖源码里的 alias 名
- package selector 如果经 `go/types` 判定为 typed const，且值能安全落到当前 formal 宽度，当前会直接 materialize 成 `arith.constant` / `go.string_constant`，而不是继续走 helper call；超出当前宽度的 case 仍会保留 helper 路径
- 针对标准库 selector call 的建模当前已统一收口到独立的 stdlib/runtime 模块里；`fmt.Sprintf` / `fmt.Sprint` / `fmt.Errorf` / `fmt.Print*` 当前会优先 lower 到固定 ABI 的 `runtime.fmt.*`，并把 `...any` 显式打包成 `!go.slice<!go.named<"any">>` 后经 `runtime.any.box.*` 装箱；`strings.Contains` / `Split` / `ReplaceAll` / `Trim*` / `ToLower` / `ToUpper` 以及 `errors.New` 这类 selector call 也会优先收成稳定的 `runtime.*` wrapper，而不再按 call-site 签名分裂成 extern facade
- `CompositeLit` 当前至少已经覆盖 typed helper 字面量和空 slice literal；因此不再统一落回 `go.todo_value "CompositeLit"`
- `&T{Field: v}` 和 `new(T)` 这类静态布局已知的 heap 分配，当前会优先收成 `runtime.newobject(size, align)`；其中 `&T{Field: v}` 还会继续接 `go.field_addr` + `go.store` 初始化字段，不再退回带完整类型后缀的 `runtime.new.*` helper 名
- 多返回值函数上的 returning-`if` 当前也已经开始直接 lower 到多结果 `scf.if`
- `for / range` 循环体里的函数级 early-return，当前也开始通过 `scf.for iter_args(stop, done, ret...)` 和一部分 `scf.while(stop, done, ret, carried...)` 这类标准结构化形式 lower；简单 `break` 会直接把 loop-local `stop` 置位，不再落回 `go.todo "BranchStmt"`。这轮 counted-returning `for init; cond; post` 也开始直接走 `scf.for iter_args(stop, done, ret...)`；但 `init/post` 如果还是复杂 lvalue（例如多层 `index/selector` 回写），当前仍然显式留在 `go.todo "ForStmt"`
- 普通 `for / range` 上的 carried 外层变量当前也不再只限单个值；像 `continue` 之前后同时更新多个局部累计值的 case，现在会直接收成多结果 `scf.if` + 多结果 `scf.for iter_args(...)`
- `make([]T, ...)`、字符串字面量、`nil` 已经有正式 `go.*` op 承接；非 slice `make(...)` 当前会先走固定的 `runtime.make.<type>` ABI
- `string` 比较、pointer 比较，以及 `error == nil` / `error != nil`、`slice == nil` / `slice != nil` 这类当前语义边界已经明确的 case，会优先收成 `go.eq` / `go.neq`；其余 opaque compare 当前仍可能保留 helper 路径
- `string` 的 `+=` 当前也开始直接走正式路径：右侧如果还是 string 表达式，会先按现有 `string + string` 规则收成 `runtime.add.string`，再通过统一 assign-target 回写，不再退回 `go.todo "compound_assign"`
- `++ / --` 当前也不再只限局部标识符；`*ptr`、field/index assign-target，以及函数内的 nonlocal identifier shadow update，当前都会复用统一的 assign-target 路径，不再一律退回 `go.todo "IncDecStmt"` / `go.todo "incdec_non_local"`；其中 array-like index 的结果类型会优先吃 typed hint，避免再被误判成 `incdec_non_integer`
- classic `for` 的 counted 形态当前会直接 lower 到 `scf.for`；这轮补上的范围已经从原来的 `<` / `i++` 扩到 `< <= > >=`，以及 `i++ / i-- / i += 1 / i -= 1 / i = i +/- 1`，并统一改成“trip count + 实际 iv 映射”的 `scf.for` 方案。空 loop body、loop 之后继续使用 induction variable 的 exit-value 恢复、以及一部分 counted-returning `for` 都不再退回 `go.todo "ForStmt"` 或 `go.todo_value "loop_iv_exit"`；更宽的一部分不含当前层 `break/continue/label` 的通用 `for init; cond; post` 仍然继续走 `scf.while`
- builtin `panic(...)` 当前会被当成终止语句；因此“函数尾部直接 `panic`”以及 `if { return ... } ; panic(...)` 这类 returning-if suffix，当前也开始直接 lower 成正式 `scf.if -> (...)`，不再额外落回 `go.todo "IfStmt_returning_region"` / `go.todo "implicit_return_placeholder"`
- 受限的 `goto/label` 当前也开始正式进入结构化控制流路径：forward `goto` 的 noop / break 子集不再保留 `go.todo`，而 backward `goto/label` 现在会先归一化成 synthetic restart-loop，再收成标准 `scf.while`；这条路径已经覆盖顶层函数体和 nested block（例如 `if` body）里的 backward label，不再把这类 case 留给 `go.todo "BranchStmt"` / `go.todo "LabeledStmt"` / `go.todo "implicit_return_placeholder"`
- 控制流不允许再用 statement helper escape hatch 伪装成 extern call；像 `IfStmt_returning_region`、`IfStmt_condition`、复杂 `ForStmt` 这类还没结构化收敛的 case，当前会显式保留成 `go.todo`
- 这轮 scheme B 之后，repo-owned fixture 和 `mlse-opt --lower-go-bootstrap` 已经覆盖 `go.field_addr` / `go.load` / `go.store`；其中带静态 `offset` 的 `go.field_addr` 在 bootstrap lowering 会优先直接 lower 成 byte-offset `llvm.getelementptr`，只有拿不到稳定 offset 的路径才保留 `runtime.field.addr.<Field>` helper
- 外部 `goeq-spec-*` gobench probe 当前这轮已经达到全绿；最新结果以 `artifacts/go-gobench-mlir-suite/summary.{md,json}` 为准，当前是 `frontend_success = 249/249`、`mlse_opt_success = 249/249`、`llvm_eligible = 249/249`、`llvm_verify_success = 249/249`
- 外部 `goeq-dce-*` probe 当前也已经达到全绿；最新这轮在 `../gobench-eq/dataset/cases/goeq-dce-*` 的 `330` 个非 test Go 文件上做到了 `frontend_success = 330/330`、`mlse_opt_success = 330/330`、`llvm_eligible = 330/330`、`llvm_verify_success = 330/330`。这一轮最后补齐的控制流长尾是 backward `goto/label`：顶层和 nested block 里的 restart-loop 形态现在都能走正式 lowering，不再作为 residual blocker 留在 `go.todo`
- 类型断言、解引用，以及更复杂的 closure / 控制流长尾，当前仍可能通过 typed helper call 或 `go.todo` / `go.todo_value` 维持可解析性；但像 `float64(x)`、浮点常量、直接 `f32/f64` 二元算术、整型 `%` / `&` / `|` / `^`，以及一元 `+` / `^` 这类更小的 typed-aware 子集，当前已经开始直接 lower 到 `arith.sitofp` / `arith.fptosi` / `arith.{addf,subf,mulf,divf,cmpf,remsi,remui,andi,ori,xori}`；对 `uint*` 的 `/` / `%` / 比较当前也会优先选择 `ui` / `u*` 变体，而不再统一按 signed 语义发射
- `switch / defer / go` 等结构当前还没有真正 lower 到稳定的标准/正式 IR；会先用 `go.todo` / `go.todo_value` 占位
- 捕获外层变量的 `FuncLit` 当前仍然没有通用 closure object 表示；但“立即调用”的子集这轮又往前推了一步：capture 只读时会继续 lower 成 private generated `func.func` + 显式 capture 参数，而一部分“立即调用 + capture write-back + 末尾显式 return”的子集，当前也会直接在 call-site 收成多结果 `scf.execute_region`，把返回值和写回后的 capture 一起带出。另一个刚补上的边界是：如果 `FuncLit` 只引用 package-scope 对象，frontend 当前不会再把这些名字误当成 lexical capture。其余需要真正 closure value、逃逸闭包，或者更复杂 returning-region 的路径，当前仍会回退到 `go.todo_value`
- 更复杂的 `if / for` 形态当前不会再伪装成控制流 helper；后续继续逐步替换成更结构化的 lowering

## 当前设计取舍

- 语义分析仍主要依赖 AST 结构，但当前已经接入了一层可退化的最小 `go/types` 上下文，用来改善 package selector、方法归属和调用签名；整体还不是 typed-first frontend，更没有接 SSA
- `internal/gofrontend/` 里的 `formal_*` 辅助文件当前已按 `core/control/memory/call/type` 五类前缀整理命名；这一步只收敛实现组织，不改变 formal bridge 的输出 contract
- `core` 这层当前也不再只是一整个 `formal_core_state.go`：共享底座已经拆成 `formal_core_types.go`、`formal_core_env.go`、`formal_core_module.go` 和 `formal_core_api.go`，后续要真的搬成子目录时，最核心的共享状态和 facade 边界已经先收出来了
- frontend 侧新增了一层显式的 runtime ABI registry，统一维护 `runtime.newobject`、`runtime.make.*`、`runtime.fmt.*`、`runtime.any.box.*` 以及 `runtime.strings.* / runtime.errors.New` 这批 helper 符号；这层纯 ABI 规则当前已经先落到独立子包 `internal/gofrontend/formalruntime/`
- 标准库 selector call 的声明式 model 也已经从 lowering 分支里抽出来，当前放在独立子包 `internal/gofrontend/formalstdlib/`；`formal_call_stdlib.go` 主要负责把这批 model 接到真实 lowering，而不再自己持有整张 registry 表
- 当前实现优先保证“parseable MLIR”，而不是完整语义保真
- 旧的文本实验已经移除，后续演进都围绕 formal bridge 继续收敛

## 未来扩展点

1. 继续把这层最小 `go/types` 上下文扩成真正可依赖的名字解析和类型检查层。
2. 接 SSA，为结构化控制流和更稳定的值建模打基础。
3. 继续把更复杂的 `if / for / switch / range` 从 `go.todo` 收敛成真正的 `func/scf/cf + go.*` contract。
