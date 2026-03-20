# GoIR To LLVM IR Experiment

这份文档记录仓库内新增的**实验性** GoIR 到 LLVM IR 路径。

它不是正式后端，也不代表仓库已经具备 `Go -> LLVM IR` 的稳定支持；当前目标只是把“哪些 GoIR 形态已经足够落到 LLVM IR、哪些还只是占位文本”固定成可复现实验。

## 新增内容

- `cmd/mlse-goir-llvm-exp`：一个独立的、最小可用的 GoIR 子集翻译器
- `scripts/goir-llvm-experiment.sh`：运行端到端实验，生成当次运行报告，并把成功样本持久化到仓库内
- `testdata/goir-llvm-exp/`：从成功 GoIR 样本中提炼出来的稳定夹具
- `testdata/goir-llvm-exp/successes/`：每个成功样本一个目录，保存成对的 GoIR 与 LLVM IR

## 设计边界

当前实验路径只支持非常小的一组 GoIR 形态：

- 单返回值函数
- `i1`、`i32`、`i64`
- 其它大多数 `!go.*` 类型先按 opaque `ptr` 处理
- 顶层局部变量初始化与重赋值，经由保守的 stack-slot lowering 处理
- `return`
- `arith.addi` / `arith.subi` / `arith.muli` / `arith.divsi`
- `arith.cmpi_*`
- `mlse.if %cond : i1`
- `mlse.if true/false : i1`
- `mlse.if arith.cmpi_* ...`
- 分支后通过**已预声明局部变量**完成的值合流
- `mlse.for`，条件只接受 `i1` 或内联 `arith.cmpi_*`
- `mlse.switch`，tag 只接受 `i1/i32/i64`，`case` 只接受单个字面量值，可带 `default`
- `mlse.call`
- `mlse.nil`

明确不支持：

- `mlse.if` / `mlse.for` 的其它条件表达式形态
- 在 `if / for / switch` 控制流体内首次定义新局部变量
- 需要真正 SSA phi-like join 的分支值合流
- `mlse.range`
- `mlse.switch` 的 multi-case、fallthrough、非标量 tag 和其它更复杂形式
- `mlse.select`、`mlse.composite`、`mlse.funclit`
- 多返回值
- `break` / `continue` / 其它显式 branch 占位语句

这组限制是刻意保守的。它的用途是确认当前 GoIR 文本里已经有哪些片段能稳定导出 LLVM IR，而不是去掩盖大面积未定义语义。

## Validation Strategy

实验脚本现在会把验证拆成三层，并优先选择更强的 dedicated verifier：

1. translation：`cmd/mlse-goir-llvm-exp` 是否成功把 GoIR 文本翻成 LLVM IR 文本
2. verifier：优先用 `opt` verifier；如果没有 `opt`，再尝试 `llvm-as`
3. compile：独立记录 `clang -c` 是否能够把生成的 LLVM IR 编译成目标文件

这三层状态会分别进入 `report.json` / `report.md`，不再把“能翻译”和“能编译”折叠成同一个 success。

关于 MLIR 工具：

- 脚本会探测本地 `mlir-opt` / `mlir-translate`
- 但当前实验路径直接产出 LLVM IR 文本，不经过 LLVM dialect MLIR
- 因此 `mlir-*` 目前只进入工具可用性报告，不参与主动验证链路

## 当前实验结论

通过 `scripts/goir-llvm-experiment.sh`，当前仓库已经验证了下面几类样本：

- `examples/go/simple_add.go`
- `examples/go/sign_if.go`
- `examples/go/choose_if_else.go`
- `examples/go/choose_merge.go`
- `examples/go/sum_for.go`
- `examples/go/switch_value.go`
- 成功 GoIR 样本里的 `mmapSize` 风格整数返回函数
- 成功 GoIR 样本里的 `prealloc*` 风格 opaque-pointer / 外部调用函数
- `ByteOrder` 风格的 `mlse.if` 控制流样本仍会被明确拒绝，因为条件仍然是 `mlse.select ... : !go.any`
- `if` / `for` / `switch` 体内新建局部变量、以及 multi-case `switch`，也会被显式拒绝

当次运行的日志和汇总报告默认写到：

- `artifacts/goir-llvm-exp/report.json`
- `artifacts/goir-llvm-exp/report.md`
- `artifacts/goir-llvm-exp/results.jsonl`

成功样本的持久化落盘默认写到：

- `testdata/goir-llvm-exp/successes/<sample>/goir.mlir`
- `testdata/goir-llvm-exp/successes/<sample>/llvm.ll`
- `testdata/goir-llvm-exp/successes/index.json`
- `testdata/goir-llvm-exp/successes/index.md`

这里刻意把 `artifacts/` 和 `testdata/` 分开：

- `artifacts/` 继续承载当次实验日志、失败样本日志和校验输出，属于可清理的瞬时目录
- `testdata/goir-llvm-exp/successes/` 承载可提交、可复查的成功样本对照结果，命名以样本 ID 为准，保留样本身份

报告里会额外固定以下信息：

- 本机 `opt` / `llvm-as` / `mlir-opt` / `mlir-translate` / `clang` 的探测结果
- 当前样本是 translation success、verifier success 还是 compile success
- dedicated verifier 覆盖率，以及多少样本只能退化到 compile-only 检查

## 设计建议

下一步不应该直接把更多 `mlse.*` 占位语句硬翻成 LLVM IR。

更合理的顺序是：

1. 先定义一个 verifier-backed 的 `GoIR-L0` 子集契约。
2. 明确哪些 `cmd/mlse-go` 输出已经满足“标量 + 预声明局部 + 受限控制流”约束。
3. 让 `GoIR-L0` 先稳定落到 LLVM IR 或 MLIR LLVM dialect。
4. 在当前 `if/for/switch` 子集稳定后，再逐步把 `range`、`break/continue`、更通用的值合流和 `select` 等结构纳入契约。

## Obsidian 同步说明

按仓库约定，这项实验后续应同步到 vault `next` 下的 `mlse设计/02-GoIR/`。

本次会话已经定位到 vault `next`，但当前沙箱只允许写仓库工作区，不能直接修改 vault `next/mlse设计/02-GoIR/`。因此这里先把结论落在仓库文档中；后续需要在可写环境里把同样的支持边界补同步到 Obsidian。
