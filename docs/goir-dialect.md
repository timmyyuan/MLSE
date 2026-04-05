# Formal GoIR Dialect Bootstrap

这份文档记录仓库里**正式 `go` dialect** 的当前落地内容。

这里的正式 `go` dialect 可以理解为“当前 GoIR 方向在真实 MLIR 工程面上的最小 bootstrap 形态”。
它不是把早期文本原型按 `TableGen` 做一次一比一翻写，而是当前仓库为长期可维护实现选定的正式入口表示。
当前仓库已经移除了旧的文本实验链路，`cmd/mlse-go` 输出的 formal bridge 就是唯一保留的 Go 前端入口。

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
- `include/mlse/Go/Conversion/BootstrapLowering.h`
- `include/mlse/Go/IR/GoOps.td`
- `include/mlse/Go/IR/GoTypes.td`
- `include/mlse/Go/IR/GoDialect.h`
- `include/mlse/Go/IR/GoTypes.h`
- `lib/Go/Conversion/BootstrapLowering.cpp`
- `lib/Go/IR/GoDialect.cpp`
- `tools/mlse-opt/mlse-opt.cpp`
- `test/GoIR/ir/basic_types.mlir`
- `test/GoIR/ir/bootstrap_ops.mlir`
- `test/GoIR/ir/control_flow_bridge.mlir`
- `test/GoIR/ir/frontend_bridge.mlir`
- 顶层 `CMakeLists.txt` 以及 `include/`、`lib/`、`tools/` 子目录 CMake 入口

## 当前类型与 op 边界

目前正式 `go` dialect 已落下的类型有：

- `!go.string`
- `!go.error`
- `!go.named<"...">`
- `!go.ptr<T>`
- `!go.slice<T>`

当前已经落地的自定义 op 是：

- `go.string_constant`
- `go.nil`
- `go.make_slice`
- `go.len`
- `go.cap`
- `go.eq`
- `go.neq`
- `go.index`
- `go.append`
- `go.append_slice`
- `go.elem_addr`
- `go.field_addr`
- `go.load`
- `go.store`
- `go.todo`
- `go.todo_value`

除了 `go` dialect 自身的类型和 op，当前 formal bridge 还会带两层源码 metadata：

- module 级 `go.scope_table`：记录函数和当前已建模的 `if / for / range / funclit` scope
- 普通 MLIR `loc(...)`：直接挂在 `func.func` 和前端发射的 op 上，当前默认使用 `loc("scopeN"("path.go":line:col))`
- 调用方可通过 `MLSE_SOURCE_DISPLAY_PATH` 覆盖 metadata 里的展示路径，避免 staging / GOPATH 临时路径泄露到 formal dump

这两层都不是新的 `go` dialect op；它们只是 formal bridge 对现有 MLIR location / attribute 机制的最小使用，用来把源码文件、行列和近邻作用域穿过 frontend 与 bootstrap lowering

这批实体的目的不是“已经完整表达 Go”，而是先把最常见、最稳定、且不适合直接塞进标准 dialect 的 Go 语义骨架放进真正的 MLIR dialect 中，同时为 frontend 迁移提供一个可解析的过渡面。

其中：

- `go.string_constant`：承接 Go 字符串字面量
- `go.nil`：承接带类型的 `nil`
- `go.make_slice`：承接 `make([]T, ...)`
- `go.len` / `go.cap`：承接 `len` / `cap` 这类仍需保留 Go 聚合语义的 builtin
- `go.eq` / `go.neq`：承接当前已经有稳定语义边界的 Go 值比较，例如 string compare、pointer compare、`error == nil`、`slice == nil`
- `go.index`：承接对 `string` 这类值语义聚合的索引读取
- `go.append`：承接 `append` 对 slice 的增长语义
- `go.append_slice`：承接 `append(dst, src...)` 这类 slice spread 追加
- `go.elem_addr` / `go.load` / `go.store`：承接 slice 下标读写在 frontend 尚未下沉到 LLVM 级地址模型前的最小地址化桥接
- `go.field_addr` / `go.load` / `go.store`：承接 selector / deref 读写在 frontend 尚未下沉到 LLVM 级地址模型前的最小地址化桥接；当 frontend 已经能通过 `go/types + sizes` 算出静态字段偏移时，`go.field_addr` 还会携带一个 `offset` 属性
- `go.todo` / `go.todo_value`：承接 frontend 迁移阶段尚未完成的 statement / value lowering

与此同时，frontend bridge 已经开始直接复用标准控制流 dialect：

- 简单 return-`if` 和单变量 merge-`if` 会 lower 到 `scf.if`
- 简单计数循环会 lower 到 `scf.for`
- `go` dialect 当前仍主要保留 Go 特有值和迁移期 placeholder，而不是接管所有控制流结构
- 这些新增 op 当前已经进入正式 dialect 并有 round-trip 样例；frontend 也已经开始在直接 `len/cap` builtin、`IndexExpr`、`append` 和受限 `range` 路径里直接产出它们，但还没有覆盖所有 aggregate / builtin 形态

## 当前没有做的事

这条正式路线还没有实现：

- `GoIR -> MLSE canonical IR` lowering pass
- `SSA -> GoIR` 导入器
- `lit/FileCheck` 测试基建
- 完整的 `mlse-opt` pass pipeline 入口

当前 `mlse-opt` 默认仍以解析 / round-trip 为主，用来验证 dialect 注册，以及类型 / op 的 parser/printer；但现在已经有两个显式 lowering 入口：

- `--lower-go-builtins`：只把 `go.len` / `go.cap` / `go.index` / `go.append` / `go.append_slice` lower 成 runtime helper call
- `--lower-go-bootstrap`：把当前 `!go.*` 类型，以及 `go.string_constant` / `go.nil` / `go.make_slice` / `go.len` / `go.cap` / `go.eq` / `go.neq` / `go.index` / `go.append` / `go.append_slice` / `go.elem_addr` / `go.field_addr` / `go.load` / `go.store` 这批 bootstrap 实体 lower 成 LLVM-legal MLIR；其中布局已知的路径会优先直接 lower，例如 `go.string_constant` 会收成 `llvm.mlir.global` + `llvm.mlir.addressof` + `{ptr, len}`，pointer compare 与 `error == nil` 当前会优先直接收成 `llvm.icmp`，`slice == nil` / `slice != nil` 当前会优先展开成对 `{ptr, len, cap}` 零值的显式判定，string compare 当前会收成固定 ABI 的 `runtime.eq.string` / `runtime.neq.string`，`go.make_slice` 当前会收成固定 ABI 的 `runtime.makeslice`，slice/string 的地址化路径则优先直接 lower 成 `llvm.extractvalue` / `llvm.getelementptr` / `llvm.load` / `llvm.store`；`go.field_addr` 在带静态 `offset` 时当前也会优先直接 lower 成 byte-offset `llvm.getelementptr`，只有拿不到稳定 offset 的路径才保留 `runtime.field.addr.<Field>` helper，而静态布局已知的 `new(T)` / `&T{...}` 当前则统一保留为固定 ABI 的 `runtime.newobject(size, align)` helper call

这两条 lowering 现在都实现为 `Go/Conversion` 下的可复用库，`tools/mlse-opt/mlse-opt.cpp` 只负责驱动层接线。它仍然不是完整 pass runner，但已经足够支撑当前仓库的 `Go -> formal MLIR -> mlse-opt --lower-go-bootstrap -> mlir-opt -> mlir-translate -> LLVM IR` 最小链路。

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

如果要直接查看当前 bootstrap lowering，可以运行：

```bash
build/mlir/tools/mlse-opt/mlse-opt --lower-go-bootstrap test/GoIR/ir/bootstrap_ops.mlir
```

## 00 -> 07：一个 Go 函数的生命周期

如果只看 `artifacts/go-gobench-mlir-suite/files/...` 里的文件名，很难直观看出每一步到底在回答什么问题。
下面用一个真实样例把整条链路串起来，风格上更接近一篇 “Life of a Go Function” 说明。

样例使用：

```text
artifacts/go-gobench-mlir-suite/files/goeq-spec-0003/prog_a/origin.go/
```

源代码是：

```go
func Target(pl []int) []int {
    var res []int
    res = append(res, pl[0])
    return res
}
```

### 阶段表

| 阶段 | 文件 | 生产者 | 主要作用 |
| --- | --- | --- | --- |
| `00` | `00-source__origin.go` | suite 拷贝 | 固定这次 probe 实际看的源码快照 |
| `01` | `01-ssa__main.Target.txt` | `cmd/mlse-go-ssa-dump` | 看 `x/tools/go/ssa` 眼里的 `main.Target` 是什么形状 |
| `02` | `02-formal.mlir` | `cmd/mlse-go` | 看 Go frontend 直接产出的 formal bridge / bootstrap `go` dialect |
| `03` | `03-roundtrip.mlir` | `mlse-opt` | 确认 `02` 能被 parser/printer 接受，并观察 round-trip 后的规范化文本 |
| `04` | `04-go-bootstrap-lowered.mlir` | `mlse-opt --lower-go-bootstrap` | 把当前 `!go.*` 类型和 `go.*` bootstrap op 收成 LLVM-legal MLIR |
| `05` | `05-llvm-dialect.mlir` | `mlir-opt` | 把 `func/arith/scf/...` 继续 lower 到 `llvm` dialect |
| `06` | `06-module.ll` | `mlir-translate --mlir-to-llvmir` | 生成最终 LLVM IR 文本 |
| `07` | `07-module.Oz.ll` | `opt -Oz -S` | 在 LLVM `opt` 下验证 `06`，并输出 `-Oz` 后的 LLVM IR 文本 |

### 00：源码快照

`00-source__origin.go` 的作用不是“重复保存一份源码”，而是把这次 probe 的输入固定下来。  
后面你看到的 `01 -> 07` 全部都应该理解成“这是针对这份具体源码快照的产物”。

### 01：SSA 看到的世界

在这一步，suite 不再问“MLIR 怎么写”，而是先问：Go 自己的 SSA 视角觉得这段代码是什么。

样例里的 `01-ssa__main.Target.txt` 大致是：

```text
t0 = &pl[0:int]
t1 = *t0
t2 = new [1]int (varargs)
t3 = &t2[0:int]
*t3 = t1
t4 = slice t2[:]
t5 = append(nil:[]int, t4...)
return t5
```

这里你可以看到两件事：

- `pl[0]` 已经变成显式的取地址和解引用
- `append(res, pl[0])` 在 SSA 里更接近 “构造一个临时单元素切片，再 append”

这一步的价值是：它告诉你 Go 工具链自身已经把哪些高层语义拆开了。

### 02：formal Go bridge

`02-formal.mlir` 回到 MLSE 自己的前端视角。
当前 `cmd/mlse-go` 不直接照搬 SSA，而是优先保留一层更接近 Go 语义的 bootstrap 表达：

```mlir
func.func @Target(%pl: !go.slice<i32>) -> !go.slice<i32> {
  %nil4 = go.nil : !go.slice<i32>
  %const8 = arith.constant 0 : i32
  %elem9 = go.elem_addr %pl, %const8 : (!go.slice<i32>, i32) -> !go.ptr<i32>
  %load10 = go.load %elem9 : !go.ptr<i32> -> i32
  %append11 = go.append %nil4, %load10 : (!go.slice<i32>, i32) -> !go.slice<i32>
  return %append11 : !go.slice<i32>
}
```

这一步回答的问题是：

- frontend 有没有把 Go 源码理解成我们当前认可的 Go bridge contract
- 哪些语义还保留在 `!go.*` 类型和 `go.*` op 中

### 03：round-trip 之后的规范化文本

`03-roundtrip.mlir` 是把 `02` 再喂给 `mlse-opt` 解析并重新打印后的结果。
它的目标不是继续 lowering，而是确认这份 MLIR 真能被仓库当前的 parser/printer 接受。

样例里你会看到：

```mlir
%c0_i32 = arith.constant 0 : i32
```

而不是 `02` 里的 `%const8`。  
这类差异通常是**打印层的规范化**，不是语义变化。

所以这一步最适合看：

- parser/printer 是否吃得下这份 formal MLIR
- 文本形式有没有被 MLIR printer 收成更标准的写法

### 04：go bootstrap lowering

这是整条链里最关键的一步。

`04-go-bootstrap-lowered.mlir` 不再允许 unresolved `go` bootstrap 实体继续往后流。
对于样例，它会把：

- `!go.slice<i32>` 收成 `!llvm.struct<(ptr, i64, i64)>`
- `go.nil` 收成 `llvm.mlir.zero`
- `go.elem_addr` / `go.load` 直接收成 `scf.execute_region` 包裹的 bounds check、`cf.cond_br`、`llvm.getelementptr`、`llvm.load`，以及 panic 分支上的 `llvm.unreachable`
- `go.append` 优先收成更像 Go 编译器的路径：长度/容量计算、必要时调用 `runtime.growslice`，然后直接 `llvm.getelementptr` + `llvm.store`
- `go.append_slice` 优先收成 `runtime.growslice` + `llvm.intr.memmove`，对应 `append(dst, src...)` 的 bulk-copy 路径

结果会像这样：

```mlir
func.func @Target(%arg0: !llvm.struct<(ptr, i64, i64)>) -> !llvm.struct<(ptr, i64, i64)> {
  %slice0 = llvm.mlir.zero : !llvm.struct<(ptr, i64, i64)>
  %dataNil = llvm.extractvalue %slice0[0] : !llvm.struct<(ptr, i64, i64)>
  %c0_i32 = arith.constant 0 : i32
  %c1_i64 = arith.constant 1 : i64
  %c4_i64 = arith.constant 4 : i64
  %len1 = llvm.extractvalue %arg0[1] : !llvm.struct<(ptr, i64, i64)>
  %cap2 = llvm.extractvalue %arg0[2] : !llvm.struct<(ptr, i64, i64)>
  %data3 = llvm.extractvalue %arg0[0] : !llvm.struct<(ptr, i64, i64)>
  %idx4 = arith.extsi %c0_i32 : i32 to i64
  %addr5 = scf.execute_region -> !llvm.ptr {
    %inBounds = arith.cmpi ult, %idx4, %len1 : i64
    cf.cond_br %inBounds, ^bb1, ^bb2
  ^bb1:
    %ptr = llvm.getelementptr %data3[%idx4] : (!llvm.ptr, i64) -> !llvm.ptr, i32
    cf.br ^bb3(%ptr : !llvm.ptr)
  ^bb2:
    func.call @runtime.panic.index(%idx4, %len1) : (i64, i64) -> ()
    llvm.unreachable
  ^bb3(%ptr_out: !llvm.ptr):
    scf.yield %ptr_out : !llvm.ptr
  }
  %elt6 = llvm.load %addr5 : !llvm.ptr -> i32
  %newLen7 = arith.addi %c1_i64, %c1_i64 : i64
  %needGrow8 = arith.cmpi ugt, %newLen7, %cap2 : i64
  %slice9 = scf.if %needGrow8 -> (!llvm.struct<(ptr, i64, i64)>) {
    %grown = func.call @runtime.growslice(%dataNil, %newLen7, %cap2, %c1_i64, %c4_i64) : (!llvm.ptr, i64, i64, i64, i64) -> !llvm.struct<(ptr, i64, i64)>
    scf.yield %grown : !llvm.struct<(ptr, i64, i64)>
  } else {
    ...
  }
  %data10 = llvm.extractvalue %slice9[0] : !llvm.struct<(ptr, i64, i64)>
  %slot11 = llvm.getelementptr %data10[%c1_i64] : (!llvm.ptr, i64) -> !llvm.ptr, i32
  llvm.store %elt6, %slot11 : i32, !llvm.ptr
  return %slice9 : !llvm.struct<(ptr, i64, i64)>
}
```

这一步回答的问题是：

- 这份 MLIR 还能不能继续走“纯 MLIR -> LLVM IR”链路
- Go 专属 bootstrap contract 是否已经被成功消解

### 05：LLVM dialect

`05-llvm-dialect.mlir` 是标准 MLIR lowering 之后的结果。
到这里，控制流、调用、返回都已经换成 `llvm.func` / `llvm.call` / `llvm.return` 这类 LLVM dialect 表达：

```mlir
llvm.func @Target(%arg0: !llvm.struct<(ptr, i64, i64)>) -> !llvm.struct<(ptr, i64, i64)> {
  %0 = llvm.mlir.zero : !llvm.struct<(ptr, i64, i64)>
  %1 = llvm.mlir.constant(0 : i32) : i32
  %2 = llvm.extractvalue %arg0[1] : !llvm.struct<(ptr, i64, i64)>
  %3 = llvm.extractvalue %arg0[0] : !llvm.struct<(ptr, i64, i64)>
  %4 = llvm.sext %1 : i32 to i64
  %5 = llvm.icmp "ult" %4, %2 : i64
  llvm.cond_br %5, ^bb1, ^bb2
^bb2:
  llvm.call @runtime.panic.index(%4, %2) : (i64, i64) -> ()
  llvm.unreachable
  ...
}
```

这一步最适合看：

- 标准 MLIR lowering 是否成功
- 还有没有高层 dialect 残留

### 06：LLVM IR

`06-module.ll` 就是 `mlir-translate` 之后的最终 LLVM IR 文本：

```llvm
define { ptr, i64, i64 } @Target({ ptr, i64, i64 } %0) {
  %2 = extractvalue { ptr, i64, i64 } %0, 1
  %3 = extractvalue { ptr, i64, i64 } %0, 0
  %4 = icmp ult i64 0, %2
  br i1 %4, label %5, label %7
7:
  call void @runtime.panic.index(i64 0, i64 %2)
  unreachable
  ...
}
```

到这里你看到的已经不是 Go bridge，也不是 MLIR dialect，而是 LLVM 真正要消费的文本。

### 07：LLVM opt 校验与收口

最后 `07-module.Oz.ll` 由 `opt -Oz -S` 生成。

- 如果失败，`07-opt.stderr` 会留下 LLVM 的报错
- 如果成功，这里会得到一份新的 `-Oz` 后 LLVM IR

因此 `07` 既在表达“LLVM 能不能接受 `06`”，也在表达：

- `06` 这份 LLVM IR 到底是不是一份有效输入

### 怎么用这 8 步定位问题

可以把这条链理解成一个分层问题定位器：

- `00 -> 01`：确认源码和 Go SSA 语义
- `01 -> 02`：确认 frontend 有没有把 Go 语义映射到当前 formal bridge
- `02 -> 03`：确认 parser/printer 是否接受这份 MLIR
- `03 -> 04`：确认 `go` bootstrap contract 是否已经被消解
- `04 -> 05`：确认标准 MLIR lowering 是否成功
- `05 -> 06`：确认 LLVM IR 文本是否能导出
- `06 -> 07`：确认 LLVM 工具链是否接受最终结果

如果把整个过程类比成 Chromium 那篇 “Life of a Pixel”，那么这里更像：

- `00` 是你要画的原始场景
- `01` 是 Go 编译器自己的内部描述
- `02` 是 MLSE 的第一版 Go 视角
- `03` 是这份视角经过 MLIR parser/printer 认证后的标准写法
- `04` 是把 Go 专属部分收成后端能懂的契约
- `05` 是进入 LLVM dialect
- `06` 是真正的 LLVM IR
- `07` 是让 LLVM 回答“这东西到底能不能吃”

## 设计取舍

这条正式路线当前刻意把 Go 专属内容收敛到“类型 + 少量必要 op”，而不是一开始就把所有控制流和算术都塞进 `go` dialect。

原因是：

- 仓库 spec 的长期方向是“标准 dialect 为主，自定义最小化”
- `func/arith/scf/cf` 已经足够承载大量结构化语义
- `go` dialect 应主要保留 Go 无法自然下沉到标准 dialect 的部分
- Go 专属 lowering 不应继续塞在工具目录里；更合理的边界是 `Go/IR` 负责 dialect 定义，`Go/Conversion` 负责可复用 lowering，`tools/` 只保留 CLI

因此，下一步更合理的顺序是：

1. 继续补最关键的 Go 类型和属性
2. 继续把 `cmd/mlse-go` 的 formal 输出从 `go.todo` 收敛到真正的结构化 lowering
3. 建立 `SSA -> go dialect` 的 golden import
4. 再实现 `go -> canonical` lowering 和 verifier
