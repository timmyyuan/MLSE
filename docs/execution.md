# Execution Layer Design

这份文档描述 MLSE 计划中的 IR 执行层。

当前仓库已经具备：

- `Go source -> formal MLIR / go dialect bridge`
- `formal MLIR -> --lower-go-bootstrap -> LLVM-legal MLIR`
- `LLVM-legal MLIR -> LLVM IR`

仓库现在已经开始落地这条执行链，但能力仍处在很窄的 MVP 阶段。

这里的执行层不以“重新实现一套 Go 或 Python 解释器”为目标，而是定义一个共享的 IR 执行框架：

- 一个语言无关的执行内核
- 若干输入 importer
- 若干语言专属 runtime bundle

当前计划中的工具名是：

- `mlse-run`

## 当前状态

仓库当前已经落下一版 `mlse-run` MVP：

- 当前输入：LLVM-dialect MLIR
- 当前测试入口：`scripts/go-exec-diff-suite.py`
- 当前样例：`test/GoExec/cases/goexec-spec-*`
- 当前 helper 子集：`runtime.makeslice`、`runtime.growslice`、`runtime.any.box.i64`、`runtime.any.box.string`、`runtime.eq.string`、`runtime.neq.string`、`runtime.fmt.Print`、`runtime.fmt.Println`、`runtime.panic.index`

还没有落地的是：

- `.ll` / LLVM IR importer
- 任意外部 LLVM IR 执行
- 更大的 Go runtime helper 子集
- Python runtime bundle

## 1. 目标

`mlse-run` 的目标是：

- 执行 MLSE 当前生成的 MLIR / LLVM IR 子集
- 为 frontend / lowering 提供可观测语义验证手段
- 为后续 Go、Python 等多语言执行提供共享基础
- 以 stdout / stderr 差分为主，支持类“编译器回归”风格测试

它的目标不是：

- 完整解释任意 Go 程序
- 完整解释任意 Python 程序
- 完整执行任意 LLVM IR
- 在第一阶段替代系统 linker、loader 或 native runtime

## 2. 设计原则

### 2.1 一个 runner，多个语言 bundle

`mlse-run` 不按语言拆成多个工具。

推荐形态是：

```text
mlse-run
  = Execution Core
  + Importers
  + Language Runtime Bundles
```

其中：

- `Execution Core`：共享解释执行引擎
- `Importers`：把不同输入格式导入成统一执行 IR
- `Language Runtime Bundles`：承接 `runtime.*` 或未来 Python runtime ABI

这样 Go 和 Python 可以共存：

- 共享内存模型
- 共享 block / terminator / call 栈执行
- 共享 stdout / stderr 捕获
- 共享差分测试框架

语言差异只停留在 runtime ABI 和 importer 上。

### 2.2 不把语言语义写进执行器主循环

执行器主循环不直接知道：

- Go slice
- Go string compare
- Python object model
- Python import/init 细节

这些语言语义都通过 runtime bundle 进入。

对于 Go，当前仓库已经有一套稳定的 runtime ABI 命名规则：

- `runtime.panic.index`
- `runtime.growslice`
- `runtime.makeslice`
- `runtime.eq.string`
- `runtime.neq.string`
- `runtime.newobject`
- `runtime.make.*`
- `runtime.any.box.*`
- `runtime.fmt.*`
- `runtime.strings.*`
- `runtime.errors.New`

见：

- [goir-dialect.md](goir-dialect.md)
- [go-frontend.md](go-frontend.md)
- `internal/gofrontend/formalruntime/abi.go`

`mlse-run` 会直接消费这层 ABI，而不是再发明一套新的 helper contract。

### 2.3 先支持“MLSE 当前生成的 IR”，不承诺全量 LLVM

第一阶段只承诺执行：

- `--lower-go-bootstrap` 之后的 LLVM-legal MLIR
- 从它翻译出来的 LLVM IR 子集

不承诺：

- 任意手写 LLVM IR
- 任意外部编译器生成的 LLVM IR

这条边界非常重要。否则解释器范围会直接失控。

## 3. 执行输入边界

### 3.1 第一阶段首选输入

最推荐的执行入口是：

- `04-go-bootstrap-lowered.mlir`
- `05-llvm-dialect.mlir`

原因：

- `02-formal.mlir` 还保留较多 Go 方言语义
- `06-module.ll` 已经更接近通用 LLVM IR，覆盖面会迅速膨胀
- `04/05` 正好是“Go 语义基本已收口、IR 还没复杂到不可控”的平衡点

### 3.2 用户可见输入形式

长期对外可以允许：

- `mlse-run input.mlir`
- `mlse-run input.ll`

当前仓库实际已落地的是第一条：

- `mlse-run input.mlir`

后续目标仍然是统一 normalize：

- `02-formal.mlir` / `03-roundtrip.mlir`
  - 先走已有 lowering，再导入执行层
- `04-go-bootstrap-lowered.mlir`
  - 直接导入执行层
- `05-llvm-dialect.mlir`
  - 直接导入执行层
- `06-module.ll`
  - 通过 LLVM IR importer 导入执行层

也就是说，用户看到的是一个 runner，但内部并不是两套完全不同的解释器。

## 4. 内部架构

### 4.1 目录建议

```text
include/mlse/Execution/
lib/Execution/Core/
lib/Execution/Import/
lib/Go/Execution/
lib/Python/Execution/
tools/mlse-run/
test/GoExec/
test/PythonExec/
```

### 4.2 Execution Core

共享执行内核负责：

- 基本块调度
- SSA 值表
- 调用栈和栈帧
- terminator 执行
- 全局和堆内存
- stdout / stderr 捕获
- panic / trap 状态

第一阶段建议内部执行 IR 使用：

- 显式 block
- 显式 terminator
- 显式 load/store
- 显式 aggregate extract/insert

不要直接在执行期保留过多 MLIR/LLVM parser 细节。

### 4.3 Importers

Importer 负责把不同输入导成统一执行 IR：

- MLIR importer
- LLVM IR importer

它们的职责是：

- 识别当前可执行子集
- 映射函数、基本块和指令
- 把外部类型映射到内部值模型

不负责语言语义。

### 4.4 Language Runtime Bundles

Go runtime bundle 负责实现：

- `runtime.*` helper 的行为
- Go 特定的 panic 文本规则
- 字符串 / slice / `any` / map 等语义

Python runtime bundle 则负责：

- Python runtime ABI
- 对象 / list / dict / string 等行为

共享执行器只看到：

- 调用一个 extern/runtime symbol
- 由注册过的 language bundle 接管

## 5. 值与内存模型

### 5.1 值类型

第一阶段建议支持：

- `i1/i8/i16/i32/i64/index`
- pointer
- struct / aggregate
- function pointer

Go 侧常见值可直接通过 aggregate 表达：

- string：`{ptr, len}`
- slice：`{ptr, len, cap}`

### 5.2 指针模型

不要直接使用宿主机裸指针。

建议使用受控地址模型，例如：

- `(object_id, offset)`

对象可以分成：

- global object
- stack slot
- heap object

这样更容易：

- 做越界检查
- 复现 `runtime.panic.index`
- 保持不同语言 runtime 共用一套内存抽象

## 6. Go Runtime Bundle

### 6.1 首批必须实现的 helper

第一阶段至少实现：

- `runtime.panic.index`
- `runtime.growslice`
- `runtime.makeslice`
- `runtime.eq.string`
- `runtime.neq.string`
- `runtime.newobject`
- `runtime.make.*`
- `runtime.any.box.*`
- `runtime.fmt.*`
- `runtime.strings.*`
- `runtime.errors.New`
- `runtime.store.index.*`

其余 helper 可以按 repo-owned fixture 和执行测试逐步扩展。

### 6.2 不在第一阶段优先支持

- goroutine
- channel
- `select`
- `reflect`
- `cgo`

这些本来就不在当前 Go MVP 边界里。

## 7. 测试策略

### 7.1 核心思路

`mlse-run` 的正确性不主要通过“手写期望 IR 执行状态”验证，而是通过差分测试验证：

- 原生 Go 执行
- `mlse-run` 执行同一程序的 IR
- 对比两边 stdout / stderr

这套方式更接近编译器测试，而不是解释器单步教学 demo。

### 7.2 Go 执行差分 suite

建议单独建一套：

```text
test/GoExec/
scripts/go-exec-diff-suite.py
```

组织方式模仿 `goeq-spec-*`：

- `common/`
- `prog_a/`
- `prog_b/`

但目标不同：

- `goeq-spec-*` 更偏工程可达性和中间产物探针
- `GoExec` 更偏真实执行语义比对

### 7.3 输出协议

用户建议优先直接对比 stdout / stderr，这个方向是可行的。

因此测试样例应设计成：

- `main()` 明确把关键状态打印到 stdout
- panic 走统一 wrapper，收成稳定 stderr 文本
- 避免依赖非稳定结果，例如地址值、map 迭代顺序、时间和随机数

### 7.4 差分流程

建议脚本流程：

1. 原生 `go run` 执行源码
2. `mlse-go` 生成 formal MLIR
3. `mlse-opt --lower-go-bootstrap`
4. 继续生成执行输入
5. `mlse-run` 执行
6. 对比 stdout / stderr

必要时补：

- `--refresh`
- `--case-glob`
- `--limit`

### 7.5 分层验证

建议至少分三层：

- repo-owned 小 fixture：纯函数、slice、string、panic
- `GoExec` 差分 case：完整程序级 stdout/stderr 对比
- 阶段一致性：`04` / `05` / `06` 执行结果一致

## 8. 与 Python 共存

这个设计专门要求：

- `mlse-run` 不把 Go slice/string/panic 写死在 core
- runtime/external symbol 走注册式 dispatch
- importer 与 runtime bundle 解耦

因此未来 Python 进入时：

- 不需要再做一个 `python-mlse-run`
- 只需要补 Python importer / runtime bundle / 差分 suite

这也是为什么当前设计坚持“共享 core + 语言 bundle”，而不是“Go 一个执行器、Python 一个执行器”。

## 9. 当前结论

推荐的执行层路线是：

1. 先实现 `mlse-run`
2. 先吃 `04/05` 这类 LLVM-legal 输入
3. 复用现有 `runtime.*` ABI 作为 Go 语义边界
4. 用 stdout / stderr 差分测试验证 Go 原生执行与 `mlse-run`
5. 保持执行 core 语言无关，为后续 Python 共存预留稳定抽象
