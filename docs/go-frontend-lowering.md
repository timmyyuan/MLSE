# Go Frontend Lowering Guide

这份文档给 `internal/gofrontend/formal_*` 提供一份稳定的阅读地图。

先说明一个边界：**当前仓库还没有真正独立的 `Go SSA -> GoIR` importer。**
现在这层实现更准确的形态是：

```text
Go source -> go/ast -> formal MLIR / go dialect bridge
```

当前实现已经补了一层可退化的最小 `go/types` 上下文，主要用来稳定：

- package selector 是否真的是 package 选择器
- 方法调用到底归属哪个 package
- 一部分直接调用和 selector 的函数签名 / 返回类型

但整体 lowering 仍然是 AST-driven，不是 SSA-driven，也还不是完全 typed-first。

另外，从当前这轮开始，Go `int` / `uint` / `uintptr` 已经按 target-sized 规则 lower，而不是固定 `i32`。
因此下面示例里如果出现旧的 `i32` 片段，应把它理解成“结构示意”；在 64 位 target 上，同类 `int` 位置现在会实际打印成 `i64`。

因此，下面所有“前后变化”的例子，描述的都是 **Go 源码 / AST 形态** 到 **formal MLIR** 的变化，不是 `x/tools/go/ssa` 指令逐条导入。

## 复现实例

所有例子都对应 `internal/gofrontend/testdata/` 里的真实 fixture，可以直接重跑：

```bash
artifacts/bin/mlse-go internal/gofrontend/testdata/default_simple_add.go
```

formal FileCheck 和 LLVM FileCheck 分别在：

- `go test ./internal/gofrontend -run TestCompileFileWithFileCheck -v`
- `go test ./internal/gofrontend -run TestCompileFileToLLVMIRWithFileCheck -v`

## 文件与函数族索引

当前 `formal_*` 辅助文件按语义前缀分组：

- `formal_core_*`：module/env/symbol/type inference 这类共享底座
- `formal_control_*`：`block/if/loop/range/returning` 这类结构化控制流辅助
- `formal_memory_*`：地址化、赋值、分配、聚合初始化
- `formal_call_*`：builtin、method、stdlib call 建模
- `formal_type_*`：常量 materialize、类型转换和 coercion

下面这张表按文件列出主要 lowering 入口、职责和建议先看的例子。

| 文件 | 主要函数族 | 负责什么 | 建议先看 |
| --- | --- | --- | --- |
| `internal/gofrontend/compiler.go` | `CompileFile` `CompileFileFormal` `parseModule` | 读取单文件、排序函数、拼 module 外壳 | `default_simple_add.go` |
| `internal/gofrontend/formal.go` | `emitFormalFunc` `emitFormalFuncBody` `emitFormalStmt` `emitFormalExpr` | 顶层函数壳子、主语句分发、主表达式分发 | `default_simple_add.go` |
| `internal/gofrontend/formal_core_helpers.go` | `emitFormalHelperCall` `chooseFormalCommonType` `normalizeFormalType` `isFormalTypeExpr` | `formal_*` 共享 helper、占位类型规则、类型表达式识别 | `make_map.go` `assign_targets.go` |
| `internal/gofrontend/formalruntime/abi.go` | `AnyBoxSymbol` `MakeHelperSymbol` `CompositeHelperSymbol` `NewHelperSymbol` | 第一批真正拆出的子包：纯 runtime ABI 命名规则和稳定 helper 符号族 | `make_map.go` `string_call.go` |
| `internal/gofrontend/formal_runtime_abi.go` | `formalRuntimeSymbol` `formalRuntimeAnyBoxSymbol` `formalRuntimeMakeHelperSymbol` | `gofrontend` 包内对 `formalruntime/` 的薄适配层，继续负责类型后缀和 composite key 提取 | `make_map.go` `string_call.go` |
| `internal/gofrontend/formal_runtime.go` | `emitFormalRuntimeCall` `emitFormalRuntimePackAnyArgs` `emitFormalRuntimeNewObject` | frontend 侧统一 runtime ABI：`newobject`、`runtime.make.*`、`runtime.fmt.*`、`runtime.any.box.*` | `make_map.go` `string_call.go` `new_builtin.go` |
| `internal/gofrontend/formal_control_stmt.go` | `emitFormalReturnStmt` `emitFormalDeclStmt` `emitFormalIfStmt` `emitFormalForStmt` `emitFormalRangeStmt` | 语句级 lowering，包括 `if/for/range` 和局部声明 | `if_merge.go` `classic_for.go` `range_slice.go` |
| `internal/gofrontend/formal_control_return_dispatch.go` | `emitFormalTerminatingIfStmt` `emitFormalReturningIfStmt` `emitFormalReturningRegion` `emitFormalReturnValues` | returning-region 识别、`if` early-return merge、return 默认值组装 | `prefixed_returning_if.go` `multi_return_if.go` |
| `internal/gofrontend/formal_type_expr.go` | `emitFormalBinaryExpr` `emitFormalUnaryExpr` `emitFormalZeroValue` `emitFormalTodoValue` | 通用二元/一元表达式、`go.eq/go.neq`、整型 `%` / `&` / `|` / `^` 和一元 `+` / `^` 的直接 `arith.*` lowering、零值和 todo value | `default_simple_add.go` `selector_string_compare.go` `bitwise_binary.go` |
| `internal/gofrontend/formal_call_dispatch.go` | `emitFormalCallExpr` `emitFormalCallStmt` `emitFormalFuncLitExpr` `formalExprFuncSig` | 通用 call dispatch、函数值/`FuncLit`、以及 stdlib/runtime facade 接线 | `single_arg_call.go` `func_literal_callback.go` `string_call.go` |
| `internal/gofrontend/formal_memory_assign.go` | `emitFormalAssignStmt` `emitFormalExpandedAssignStmt` `emitFormalAssignTargetValue` | 标识符赋值、`_` 丢弃、selector/index/deref 赋值，以及 `string +=` 这类 compound assign 回写 | `assign_targets.go` `string_compound_assign.go` |
| `internal/gofrontend/formal_control_block.go` | `emitFormalFuncBlock` `emitFormalRegionBlock` | block 扫描、returning-region matcher 接线 | `prefixed_returning_if.go` `range_return.go` |
| `internal/gofrontend/formal_control_branch.go` | `emitFormalLoopBody` `emitFormalLoopBodyWithCarried` | loop body 中的 `continue` 形态，以及单/多 carried value 的 `scf.if` yield | `range_continue.go` `range_continue_prefix.go` `range_multi_iter_args.go` |
| `internal/gofrontend/formal_call_builtins.go` | `emitFormalBuiltinCall` `emitFormalLenCapBuiltinCall` `emitFormalAppendBuiltinCall` `emitFormalAppendSliceBuiltinCall` `emitFormalGoLenValue` `emitFormalIndexedReadValue` `emitFormalGoIndexValue` | `len/cap/index/append/append(dst, src...)` 的直接 `go.*` op 路径；slice 索引优先地址化 | `len_builtin.go` `cap_builtin.go` `index_builtin.go` `append_builtin.go` `append_spread_builtin.go` |
| `internal/gofrontend/formal_memory_ops.go` | `emitFormalIndexExpr` `emitFormalSelectorExpr` `emitFormalFieldAddr` `emitFormalElemAddr` `emitFormalLoad` `emitFormalStore` | selector/index 读写、最小地址化桥接、`go.field_addr` / `go.elem_addr` / `go.load` / `go.store` | `index_builtin.go` `selector_value.go` `assign_targets.go` |
| `internal/gofrontend/formal_memory_composite.go` | `emitFormalCompositeLitExpr` | 空 slice literal 和 typed helper composite literal | `empty_slice_literal.go` |
| `internal/gofrontend/formal_control_condition.go` | `emitFormalCondition` | 任意条件表达式到 `i1` 的 coercion | `helper_condition.go` |
| `internal/gofrontend/formal_type_convert.go` | `emitFormalStarExpr` `emitFormalTypeAssertExpr` `emitFormalCoerceValue` `emitFormalIntegerCast` | 解引用、类型断言、返回值 coercion、整数/浮点 cast | `type_assert_and_star.go` `int64_conversion.go` `return_coercion.go` `float_binary.go` |
| `internal/gofrontend/formal_control_if.go` | `emitFormalIfStmtWithInit` `emitFormalVoidReturningIfStmt` `emitFormalVoidReturningRegion` `emitFormalVoidBranchRegion` | `if init; cond` 和 void-returning `if` | `if_init.go` `void_returning_if.go` |
| `internal/gofrontend/formal_control_loop_return.go` | `emitFormalReturningLoopStmt` `emitFormalLoopReturnStmt` `emitFormalLoopReturnIfStmt` `emitFormalLoopBreakIfStmt` `emitFormalLoopReturnSequence` `emitFormalReturningLoopRegion` | loop body 内的函数级 early return / break | `range_return.go` `for_break_return.go` |
| `internal/gofrontend/formal_control_loop_return_stmt.go` | `emitFormalForLoopReturnState` `emitFormalRangeLoopReturnState` | `for/range` 上的 `scf.for iter_args(stop, done, ret...)` 形态 | `range_return.go` `for_break_return.go` |
| `internal/gofrontend/formal_call_methods.go` | `emitFormalMethodCallExpr` `emitFormalMethodCallStmt` | 立即 method call 到稳定的 `package.receiver.method` 符号调用 | `method_call.go` |
| `internal/gofrontend/formalstdlib/model.go` | `Lookup` `CallModel` `ResultKind` `ArgHintKind` | 第一批真正拆出的子包：声明式 stdlib registry，统一 `fmt/strings/errors` 的 runtime ABI 选择 | `string_call.go` `stdlib_alias_calls.go` |
| `internal/gofrontend/formal_call_stdlib.go` | `inferFormalStdlibCallResultType` `inferFormalStdlibCallArgHint` `emitFormalStdlibCall` `emitFormalStdlibCallStmt` | `gofrontend` 包内把 `formalstdlib/` 的声明式 model 接到真实 lowering，包括 `fmt` variadic/runtime ABI 和 `strings/errors` wrapper | `string_call.go` `stdlib_alias_calls.go` |
| `internal/gofrontend/formal_control_range.go` | `inferFormalRangeValueType` `inferFormalIdentUsageType` `inferFormalIdentContextType` `formalCallArgHint` | `range` 变量类型提示与上下文传播 | `range_slice.go` `selector_string_compare.go` |
| `internal/gofrontend/formal_control_returning.go` | `emitFormalReturnExprOperands` `emitFormalYieldLine` | 多返回值 return / yield operand 组装 | `multi_return_if.go` |
| `internal/gofrontend/formal_core_types.go` | `formalFuncSig` `formalFuncBodySpec` `formalHelperCallSpec` | core 共享数据结构和签名/函数体描述 | `func_literal_callback.go` |
| `internal/gofrontend/formal_core_env.go` | `formalEnv` `newFormalEnv` `syncFormalTempID` | 函数级 SSA/temp/local state | `selector_value.go` |
| `internal/gofrontend/formal_core_module.go` | `formalModuleContext` `newFormalModuleContext` `formalFuncSymbol` | module 级 extern/generated func 管理、receiver/top-level 命名 | `func_literal_callback.go` `selector_value.go` |
| `internal/gofrontend/formal_core_loc.go` | `emitFormalLinef` `annotateFormalStructuredOp` `formalLocationSuffix` | formal 文本里的 `loc(...)` 和 scope label 格式化 | `goeq_scope_locations.go` |
| `internal/gofrontend/formal_core_api.go` | `registerFormalExtern` `reserveFormalFuncLitSymbol` `lookupFormalDefinedFuncSig` | core facade：给 call/runtime/symbol 层提供较窄的 module 访问面 | `string_call.go` `func_literal_callback.go` |

## Core Dispatch

核心入口是 `CompileFileFormal -> emitFormalFunc -> emitFormalFuncBody -> emitFormalStmt / emitFormalExpr`。

示例 fixture：`internal/gofrontend/testdata/default_simple_add.go`

变化前：

```go
func add(a int, b int) int {
	c := a + b
	return c
}
```

变化后：

```mlir
module attributes {go.scope_table = [...]} {
  func.func @demo.add(%a: i32, %b: i32) -> i32 attributes {go.scope = 0 : i64} {
    %bin3 = arith.addi %a, %b : i32 loc("scope0"("testdata/default_simple_add.go":5:7))
    return %bin3 : i32 loc("scope0"("testdata/default_simple_add.go":6:2))
  } loc("scope0"("testdata/default_simple_add.go":4:1))
}
```

这个例子里可以直接对照：

- `emitFormalParams`：`a int, b int` 变成 `%a: i32, %b: i32`
- `emitFormalBinaryExpr`：`a + b` 变成 `arith.addi`
- `emitFormalReturnStmt`：`return c` 变成 `return %bin3 : i32`
- `formal_core_module.go + formal_core_loc.go`：module 会额外挂出 `go.scope_table`，函数和 op 带上 `go.scope` / `loc(...)`，必要时优先使用 `MLSE_SOURCE_DISPLAY_PATH` 作为展示路径

## If, Merge And Returning Regions

这一组入口主要是：

- `emitFormalIfStmt`
- `emitFormalIfStmtWithInit`
- `emitFormalTerminatingIfStmt`
- `emitFormalReturningIfStmt`
- `emitFormalReturningRegion`
- `emitFormalFuncBlock`
- `emitFormalRegionBlock`

### 变量 merge

fixture：`internal/gofrontend/testdata/if_merge.go`

变化前：

```go
func choose(b bool) int {
	var x int
	if b {
		x = 1
	} else {
		x = 2
	}
	return x
}
```

变化后：

```mlir
%if8 = scf.if %b -> (i32) {
    %const10 = arith.constant 1 : i32
    scf.yield %const10 : i32
} else {
    %const10 = arith.constant 2 : i32
    scf.yield %const10 : i32
}
return %if8 : i32
```

### `if init; cond`

fixture：`internal/gofrontend/testdata/if_init.go`

变化前：

```go
if y := x + 1; y > 0 {
	x = y
}
```

变化后：

```mlir
%bin14 = arith.addi %x, %const13 : i32
%bin16 = arith.cmpi sgt, %bin14, %const15 : i32
%if17 = scf.if %bin16 -> (i32) {
    scf.yield %bin14 : i32
} else {
    scf.yield %x : i32
}
```

### early return

fixture：`internal/gofrontend/testdata/prefixed_returning_if.go`

变化前：

```go
y := x + 1
if y > 3 {
	return y
}
return x
```

变化后：

```mlir
%bin8 = arith.cmpi sgt, %bin4, %const7 : i32
%ifret9 = scf.if %bin8 -> (i32) {
    scf.yield %bin4 : i32
} else {
    scf.yield %x : i32
}
return %ifret9 : i32
```

多返回值版本看 `internal/gofrontend/testdata/multi_return_if.go`；递归 nested returning-if 看 `internal/gofrontend/testdata/nested_returning_if.go`。
这轮还额外补上了 builtin `panic(...)` 作为 terminating suffix 的路径：像 `if { return x } ; panic("boom")` 这类形状，当前也会直接收成 `scf.if -> (...)`，不再退回 `IfStmt_returning_region` / `implicit_return_placeholder`。

## Loops, Continue And Loop-Body Return

这一组入口主要是：

- `emitFormalForStmt`
- `emitFormalRangeStmt`
- `emitFormalLoopBody`
- `emitFormalReturningLoopStmt`
- `emitFormalForLoopReturnState`
- `emitFormalRangeLoopReturnState`

### 经典 counted `for`

fixture：`internal/gofrontend/testdata/classic_for.go`

变化前：

```go
for i := 0; i < len(xs); i++ {
	_ = xs[i]
}
```

变化后：

```mlir
%len4 = go.len %xs : !go.slice<!go.string> -> i32
%idx5 = arith.index_cast %const3 : i32 to index
%idx6 = arith.index_cast %len4 : i32 to index
scf.for %i_iv8 = %idx5 to %idx6 step %const7 {
    %i_body9 = arith.index_cast %i_iv8 : index to i32
    %elem10 = go.elem_addr %xs, %i_body9 : (!go.slice<!go.string>, i32) -> !go.ptr<!go.string>
    %load11 = go.load %elem10 : !go.ptr<!go.string> -> !go.string
}
```

这一层当前还有两点补充：

- 空 body 的 counted `for` 不再因为 `len(body) == 0` 直接落回 `go.todo "ForStmt"`
- 对当前已经支持的 `<` / `+1` counted `for`，如果 loop 之后还继续使用 induction variable，frontend 会直接发 `arith.cmpi + arith.select` 恢复 exit value，而不是再塞一个 `go.todo_value "loop_iv_exit"`
- counted `for` 当前已经从 `< / i++` 扩到 `< <= > >=`，以及 `i++ / i-- / i += 1 / i -= 1 / i = i +/- 1`；实现上不再直接把 loop IV 当成 `scf.for` induction variable，而是统一先算 trip count，再在 loop body 里把实际 IV 重建出来
- 对一部分不含当前层 `break/continue/label` 的通用 `for init; cond; post`，frontend 也开始直接 lower 到 `scf.while`；这轮主要覆盖 `>=` / `<=` / `==` 条件、`+=` / `-=` 这类 post/update 形态，以及一部分 loop body 里直接 `return` 的 carried-local 子集
- counted-returning `for init; cond; post` 当前也开始复用同一套 trip-count lowering，直接 lower 到 `scf.for iter_args(stop, done, ret...)`
- 普通 loop body 里的 nested returning loop 当前也开始复用 returning-loop matcher，不再一律因为“内层 loop 含 return”而退回 `go.todo "ForStmt"`
- returning-loop 这条 `scf.while(stop, done, ret, carried...)` 路径当前还带一条显式边界：如果 `init` / `post` 自身就是复杂 lvalue 回写（例如多层 `index/selector`），frontend 仍然会保守退回 `go.todo "ForStmt"`，而不是继续生成可能不合法的 nested store SSA
- `emitFormalIncDecStmt` 当前也已经和 `emitFormalAssignTargetValue` 接上：`ident++` 之外，`(*p)++`、`x[i]++`、`x.f--` 以及函数内的 nonlocal identifier `g--`，都会先读出当前值、做 `arith.addi/subi`，再走统一 assign-target 回写；对 array-like index，这一步还会优先用 typed result hint 把 `runtime.index.value__sig*` 直接收成整数结果，避免再误落 `go.todo "incdec_non_integer"`

### backward `goto/label`

fixture：

- `internal/gofrontend/testdata/goto_backward_return.go`
- `internal/gofrontend/testdata/goto_backward_nested_if.go`

变化前：

```go
if y {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
}
return x
```

以前这类 case 容易在两处退化：

- backward `goto` 直接留成 `go.todo "BranchStmt"` / `go.todo "LabeledStmt"`
- synthetic restart-loop 虽然被识别出来，但函数 exit tail 还留在 loop 内，最后又补出 `go.todo "implicit_return_placeholder"`

现在这条路径会先做 AST 归一化：

- backward label region 改写成 synthetic restart flag
- 函数 exit tail 从 restart loop 里切出来，保留在 loop 外
- 这个归一化不只跑在函数体顶层，也会递归处理 nested block / `if` body / labeled stmt

变化后：

```mlir
%ifret36 = scf.if %y -> (i64) {
    %loop30:3 = scf.while (%loop_stop_iter21 = %loopstop20, %__mlse_goto_restart_lbl_iter23 = %const19, %x_iter25 = %x) : (i1, i1, i64) -> (i1, i1, i64) {
      ...
      %loopcont35:3 = scf.if %bin32 -> (i1, i1, i64) {
          scf.yield %loopstop34, %const33, %inc30 : i1, i1, i64
      } else {
          scf.yield %loopstop33, %const28, %inc30 : i1, i1, i64
      }
      ...
    }
    scf.yield %loop30#2 : i64
} else {
    scf.yield %x : i64
}
return %ifret36 : i64
```

这一步的关键不是“把 `goto` 伪装成 helper call”，而是：

- 保持在 frontend 里完成控制流结构化
- 让 backward restart-loop 最终落到标准 `scf.while`
- 明确把 loop-exit tail 留在 loop 外，避免再污染 return/SSA env

### 立即调用的 captured `func lit`

fixture：

- `internal/gofrontend/testdata/captured_immediate_call.go`
- `internal/gofrontend/testdata/captured_immediate_mutate_local.go`
- `internal/gofrontend/testdata/captured_immediate_mutate_param.go`

变化前：

```go
return func(z int) int {
	return x + z
}(3)
```

会先落成 `go.todo_value "FuncLit_capture"`，再接一个 `func.call_indirect`。

现在分两档：

- 如果 `func lit` 是立即调用
- 并且 capture 只读，不会在 `func lit` 里对 captured name 做重新赋值

frontend 会直接：

- 生成一个 private `func.func`
- 把 captured value 追加到参数列表前面
- 在 callsite 发 `func.call @...`，不再走 closure placeholder / indirect call

如果 `func lit` 也是立即调用，但它会对 captured local / param 做直接赋值或 `++/--`，并且当前还能收成“末尾显式 `return`”这一类简单 body，frontend 会改走另一条更窄的路径：

- 不再生成 private helper
- 直接在 callsite 发一个多结果 `scf.execute_region`
- region 结果会同时带出“正常返回值 + 写回后的 capture 值”
- callsite 再把这些写回值重绑回外层 env

这一步仍然不是通用 closure lowering。需要真正 closure object、逃逸闭包，或者更复杂 returning-region / control-flow 的 capture write-back，当前仍然保留 `go.todo_value "FuncLit_capture"`。不过如果 `func lit` 只引用 package-scope 对象，frontend 当前会直接把它当成“无 lexical capture”的立即调用子集处理，不再把 package-global 名字误记进 capture 集合。

### `range` + `continue`

fixture：`internal/gofrontend/testdata/range_continue.go`

变化前：

```go
for _, x := range xs {
	if x == 0 {
		continue
	}
	out = append(out, x)
}
```

变化后：

```mlir
%range11 = scf.for %range_iv10 = %idx7 to %idx8 step %const9 iter_args(%out_iter = %nil3) -> (!go.slice<i32>) {
    %rangeaddr12 = go.elem_addr %xs, %range_iv10 : (!go.slice<i32>, index) -> !go.ptr<i32>
    %rangeval13 = go.load %rangeaddr12 : !go.ptr<i32> -> i32
    %bin14 = arith.cmpi eq, %rangeval13, %const13 : i32
    %loopcont16 = scf.if %bin14 -> (!go.slice<i32>) {
        scf.yield %out_iter : !go.slice<i32>
    } else {
        %append15 = go.append %out_iter, %rangeval13 : (!go.slice<i32>, i32) -> !go.slice<i32>
        scf.yield %append15 : !go.slice<i32>
    }
    scf.yield %loopcont16 : !go.slice<i32>
}
```

### loop body 里的函数级 early return

fixture：`internal/gofrontend/testdata/range_return.go`

变化前：

```go
for _, x := range xs {
	if x > 0 {
		return x
	}
}
return 0
```

变化后：

```mlir
%range11:3 = scf.for ... iter_args(%loopret_stop_iter8 = %stop1, %loopret_done_iter9 = %done2, %loopret0_iter10 = %const3) -> (i1, i1, i32) {
    %rangestep16:3 = scf.if %loopret_stop_iter8 -> (i1, i1, i32) {
        scf.yield %loopret_stop_iter8, %loopret_done_iter9, %loopret0_iter10 : i1, i1, i32
    } else {
      %rangeaddr12 = go.elem_addr %xs, %range_iv7 : (!go.slice<i32>, index) -> !go.ptr<i32>
      %rangeval13 = go.load %rangeaddr12 : !go.ptr<i32> -> i32
      %bin14 = arith.cmpi sgt, %rangeval13, %const13 : i32
      %loopif15:3 = scf.if %bin14 -> (i1, i1, i32) {
          %stop15 = arith.constant true
          %done16 = arith.constant true
          scf.yield %stop15, %done16, %rangeval13 : i1, i1, i32
      } else {
          scf.yield %loopret_stop_iter8, %loopret_done_iter9, %loopret0_iter10 : i1, i1, i32
      }
      scf.yield %loopif15#0, %loopif15#1, %loopif15#2 : i1, i1, i32
    }
    scf.yield %rangestep16#0, %rangestep16#1, %rangestep16#2 : i1, i1, i32
}
%loopret17 = scf.if %range11#1 -> (i32) {
    scf.yield %range11#2 : i32
} else {
    %const19 = arith.constant 0 : i32
    scf.yield %const19 : i32
}
return %loopret17 : i32
```

### loop body 里的 `break + return`

fixture：`internal/gofrontend/testdata/for_break_return.go`

变化前：

```go
for i := 0; i < len(xs); i++ {
	if i >= limit {
		break
	}
	if xs[i] > 0 {
		return xs[i]
	}
}
return 0
```

变化后：

```mlir
%loopret14:3 = scf.for ... iter_args(%loopret_stop_iter10 = %stop1, %loopret_done_iter11 = %done2, %loopret0_iter12 = %const3) -> (i1, i1, i32) {
    %loopstep22:3 = scf.if %loopret_stop_iter10 -> (i1, i1, i32) {
        scf.yield %loopret_stop_iter10, %loopret_done_iter11, %loopret0_iter12 : i1, i1, i32
    } else {
      %bin15 = arith.cmpi sge, %i_body14, %limit : i32
      %loopif16:3 = scf.if %bin15 -> (i1, i1, i32) {
          %stop16 = arith.constant true
          scf.yield %stop16, %loopret_done_iter11, %loopret0_iter12 : i1, i1, i32
      } else {
          ...
      }
      scf.yield %loopif16#0, %loopif16#1, %loopif16#2 : i1, i1, i32
    }
    scf.yield %loopstep22#0, %loopstep22#1, %loopstep22#2 : i1, i1, i32
}
```

## Assignments And Rebinding

这一组入口主要是：

- `emitFormalAssignStmt`
- `emitFormalExpandedAssignStmt`
- `emitFormalAssignTargetValue`

fixture：`internal/gofrontend/testdata/assign_targets.go`

变化前：

```go
m[key] = value
xs[0] = value
h.X = value
*defaultValue = value
```

变化后：

```mlir
%store7 = func.call @runtime.store.index.map(%m, %key, %value) : (!go.named<"map">, !go.string, i32) -> !go.named<"map">
%const10 = arith.constant 0 : i32
%elem11 = go.elem_addr %xs, %const10 : (!go.slice<i32>, i32) -> !go.ptr<i32>
go.store %value, %elem11 : i32 to !go.ptr<i32>
%field14 = go.field_addr %h, "X" {offset = 0 : i64} : !go.ptr<!go.named<"holder">> -> !go.ptr<i32>
go.store %value, %field14 : i32 to !go.ptr<i32>
%global17 = func.call @defaultValue() : () -> !go.ptr<i32>
go.store %value, %global17 : i32 to !go.ptr<i32>
```

当前策略现在分成两层：

1. 对 concrete pointer / named aggregate 上的 selector 写入、slice 下标写入、以及 `*ptr = v`，优先产出 `go.elem_addr` / `go.field_addr` / `go.store`
2. 对当前还拿不到稳定地址模型的目标，例如 `map` index 更新，仍保留 helper 路径；但这类 helper 当前已经统一并入 `runtime.store.index.*`

## Builtins, Aggregate Ops And Composite Values

这一组入口主要是：

- `emitFormalBuiltinCall`
- `emitFormalLenCapBuiltinCall`
- `emitFormalAppendBuiltinCall`
- `emitFormalGoLenValue`
- `emitFormalIndexedReadValue`
- `emitFormalGoIndexValue`
- `emitFormalCompositeLitExpr`
- `emitFormalMakeCall`
- `emitFormalZeroValue`

### `len/cap/index/append`

fixtures：

- `internal/gofrontend/testdata/len_builtin.go`
- `internal/gofrontend/testdata/cap_builtin.go`
- `internal/gofrontend/testdata/index_builtin.go`
- `internal/gofrontend/testdata/append_builtin.go`

变化后分别会落成：

```mlir
%len4 = go.len %xs : !go.slice<i32> -> i32
%cap3 = go.cap %xs : !go.slice<i32> -> i32
%elem4 = go.elem_addr %xs, %const3 : (!go.slice<i32>, i32) -> !go.ptr<i32>
%load5 = go.load %elem4 : !go.ptr<i32> -> i32
%append4 = go.append %xs, %const3 : (!go.slice<i32>, i32) -> !go.slice<i32>
```

`append(dst, src...)` 会单独走 `go.append_slice`：

```mlir
%append_slice4 = go.append_slice %dst, %src : (!go.slice<i32>, !go.slice<i32>) -> !go.slice<i32>
```

这里现在有一个明确分工：

- slice 下标读取优先发 `go.elem_addr + go.load`
- string 下标读取才继续发 `go.index`

### 空 slice literal

fixture：`internal/gofrontend/testdata/empty_slice_literal.go`

变化前：

```go
return []int{}
```

变化后：

```mlir
%const3 = arith.constant 0 : i32
%make4 = go.make_slice %const3, %const3 : i32 to !go.slice<i32>
return %make4 : !go.slice<i32>
```

### 非 slice `make`

fixture：`internal/gofrontend/testdata/make_map.go`

变化前：

```go
m := make(map[string]bool)
```

变化后：

```mlir
%make3 = func.call @runtime.make.map() : () -> !go.named<"map">
```

### `fmt.Sprintf` 与 `...any`

fixture：`internal/gofrontend/testdata/string_call.go`

变化前：

```go
return fmt.Sprintf("hello %s", name)
```

变化后：

```mlir
%argc3 = arith.constant 1 : i64
%args4 = go.make_slice %argc3, %argc3 : i64 to !go.slice<!go.named<"any">>
%any5 = func.call @runtime.any.box.string(%name) : (!go.string) -> !go.named<"any">
%argi6 = arith.constant 0 : i64
%slot7 = go.elem_addr %args4, %argi6 : (!go.slice<!go.named<"any">>, i64) -> !go.ptr<!go.named<"any">>
go.store %any5, %slot7 : !go.named<"any">, !go.ptr<!go.named<"any">>
%call8 = func.call @runtime.fmt.Sprintf(%str2, %args4) : (!go.string, !go.slice<!go.named<"any">>) -> !go.string
```

## Methods And Function Values

这一组入口主要是：

- `emitFormalMethodCallExpr`
- `emitFormalMethodCallStmt`
- `emitFormalFuncLitExpr`

### 立即 method call

fixture：`internal/gofrontend/testdata/method_call.go`

变化前：

```go
func size(s *StringSet) int {
	return s.Len()
}
```

变化后：

```mlir
%call3 = func.call @demo.ptr.StringSet.Len(%s) : (!go.ptr<!go.named<"StringSet">>) -> i32
return %call3 : i32
```

### 非捕获 `FuncLit`

fixture：`internal/gofrontend/testdata/func_literal_callback.go`

变化前：

```go
run(func() {
	ping()
})
```

变化后：

```mlir
%funclit1 = func.constant @demo.main.__lit0 : () -> ()
func.call @demo.run(%funclit1) : (() -> ()) -> ()

func.func private @demo.main.__lit0() {
  func.call @demo.ping() : () -> ()
  return
}
```

## Conversions, Selectors And Opaque Values

这一组入口主要是：

- `emitFormalSelectorExpr`
- `emitFormalCondition`
- `emitFormalStarExpr`
- `emitFormalTypeAssertExpr`
- `emitFormalCoerceValue`
- `emitFormalIntegerCast`

### selector value

fixture：`internal/gofrontend/testdata/selector_value.go`

变化前：

```go
return x + commonpkg.GlobalInput
```

变化后：

```mlir
%sel3 = func.call @example.com.common.GlobalInput() : () -> i32
%bin4 = arith.addi %x, %sel3 : i32
```

### 条件 coercion

fixture：`internal/gofrontend/testdata/helper_condition.go`

变化前：

```go
if x {
	return 1
}
```

变化后：

```mlir
%conv2 = func.call @runtime.convert.any.to.bool(%x) : (!go.named<"any">) -> i1
%ifret3 = scf.if %conv2 -> (i32) {
    ...
}
```

### 类型断言和解引用

fixture：`internal/gofrontend/testdata/type_assert_and_star.go`

变化前：

```go
v, _ := x.(bool)
_ = *p
```

变化后：

```mlir
%typeassert3 = func.call @runtime.type.assert.any.to.bool(%x) : (!go.named<"any">) -> i1
%load16 = go.load %p : !go.ptr<i32> -> i32
```

### 返回值 coercion 和整数 cast

fixtures：

- `internal/gofrontend/testdata/return_coercion.go`
- `internal/gofrontend/testdata/int64_conversion.go`

变化后示意：

```mlir
%conv17 = func.call @runtime.convert.value.to.string(%index12) : (!go.named<"value">) -> !go.string
%const3 = arith.constant 0 : i64
```

### opaque nil

fixture：`internal/gofrontend/testdata/opaque_nil.go`

变化前：

```go
return v == nil
```

变化后：

```mlir
%zero3 = func.call @runtime.zero.Result() : () -> !go.named<"Result">
%cmp4 = func.call @runtime.eq.Result(%v, %zero3) : (!go.named<"Result">, !go.named<"Result">) -> i1
```

## State, Externs And Generated Functions

`formal_core_env.go` / `formal_core_module.go` / `formal_core_api.go` 不直接 lowering AST 节点，但决定了输出是否可继续走后续 pipeline：

- `formalEnv`
  - 记录局部变量当前 SSA 名、推断类型和临时值编号
- `formalModuleContext`
  - 记录 module 内已有函数签名、extern declaration 和自动生成的 `FuncLit` helper

最值得看的两个例子：

- `internal/gofrontend/testdata/func_literal_callback.go`
  - `formalModuleContext.generated` 最终会追加 `@demo.main.__lit0`
- `internal/gofrontend/testdata/selector_value.go`
  - `formalModuleContext.externByKey` 最终会补 `func.func private @example.com.common.GlobalInput() -> i32`

## 阅读顺序建议

如果你是第一次进 `internal/gofrontend/`，建议按这个顺序读：

1. `compiler.go`
2. `formal.go`
3. `formal_core_env.go`
4. `formal_core_module.go`
5. `formal_core_api.go`
6. `formal_control_block.go`
7. `formal_control_if.go`
8. `formal_control_loop_return.go`
9. `formal_control_loop_return_stmt.go`
10. `formal_memory_assign.go`
11. `formal_call_dispatch.go`
10. `formal_call_builtins.go`
11. `formal_type_convert.go`
12. `formal_call_methods.go`
13. `formal_memory_composite.go`
14. `formal_control_range.go`

这样可以先建立“主调度器 -> block/region -> 特例 lowering -> helper/value family”的心智模型，再去追具体长尾语义。
