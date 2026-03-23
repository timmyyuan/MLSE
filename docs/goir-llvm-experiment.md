# GoIR To LLVM IR Experiment

这份文档记录仓库内新增的**实验性** GoIR 到 LLVM IR 路径。

它不是正式后端，也不代表仓库已经具备 `Go -> LLVM IR` 的稳定支持；当前目标只是把“哪些 GoIR 形态已经足够落到 LLVM dialect MLIR / LLVM IR、哪些还只是占位文本”固定成可复现实验。

截至 `2026-03-20`，仓库里已经另外新增了一条**正式 `go` dialect bootstrap** 路线，见 [goir-dialect.md](goir-dialect.md)。两条路径的关系是：

- 正式 `go` dialect 路线：面向长期可维护的真实 MLIR 工程面
- 当前实验路径：继续承担覆盖回归和 blocker 摸底

## 当前链路

截至 `2026-03-20`，当前实验后端的最小链路已经改成：

```text
GoIR-like text
  -> LLVM dialect MLIR
  -> mlir-translate --mlir-to-llvmir
  -> LLVM IR
```

由于 `cmd/mlse-go` 默认已经切到正式 `go` dialect 输出，实验脚本和批跑现在会显式使用：

```bash
go run ./cmd/mlse-go -emit=goir-like <input.go>
```

CLI 默认仍输出 LLVM IR，但现在额外支持：

```bash
go run ./cmd/mlse-goir-llvm-exp -emit=llvm-dialect <input.goir>
go run ./cmd/mlse-goir-llvm-exp -emit=llvm-dialect -slice-model=cap <input.goir>
```

其中 `-slice-model=min|cap` 的当前语义为：

- `min`：默认最小 slice 表示 `{data,len}`
- `cap`：实验性 slice 表示 `{data,len,cap}`，并启用 `cap(xs)` lowering 与更真实的 sub-slice capacity 计算

## 新增内容

- `cmd/mlse-goir-llvm-exp`：一个独立的、最小可用的 GoIR 子集翻译器
- `testdata/goir-llvm-exp/`：从成功 GoIR 样本中提炼出来的稳定夹具
- `testdata/goir-llvm-exp/*.llvm.mlir`：稳定的 LLVM dialect 中间层 golden
- `testdata/goir-llvm-exp/successes/`：每个成功样本一个目录，保存成对的 GoIR 与 LLVM IR

## 设计边界

当前实验路径已经从“只能吃一小撮手工夹具”扩到“能容忍一批 placeholder-rich GoIR 文本”，但它依然不是语义保真的正式后端。

当前稳定覆盖的形态主要包括：

- 单返回值函数，以及通过 LLVM struct 打包的多返回值函数 / `return`
- `i1`、`i32`、`i64`
- 其它大多数 `!go.*` 类型先按 opaque `ptr` 处理
- 局部变量初始化与重赋值，经由 stack-slot lowering 处理
- `arith.addi` / `arith.subi` / `arith.muli` / `arith.divsi`
- `arith.cmpi_*`
- `mlse.if` / `mlse.for` 的 `i1`、整数和 pointer-truthiness 条件
- `mlse.for`
- `mlse.switch`，tag 只接受 `i1/i32/i64`，`case` 只接受单个字面量值，可带 `default`
- `mlse.call`
- `mlse.nil`
- `len(slice)`，以及在 `-slice-model=cap` 下的 `cap(slice)`
- 前端把简单 `range` 先降成 `len + index + mlse.for`
- `break` / `continue`
- `mlse.expr`
- `mlse.store_select` / `mlse.store_index` / `mlse.store_deref`
- `mlse.select` / `mlse.index` / `mlse.load` / `mlse.composite` / 字符串字面量的 **opaque fallback** lowering
- `mlse.slice` 的 pass-through 与 `a[i:]` / `a[:j]` / `a[i:j]` 子切片；在 `cap` 模式下会额外携带并更新 runtime `cap`
- 同名 external call 在不同签名下的 symbol 自动拆分

明确不支持：

- 对多返回值内部函数调用的真实返回值建模
- 需要真实 SSA phi-like join 的值合流
- `mlse.switch` 的 multi-case、fallthrough、非标量 tag 和其它更复杂形式
- 依赖真实容器/对象布局的精确语义
- 目前的 opaque fallback 只保证“能翻出 LLVM IR / verifier 尽量通过”，不保证语义等价

这里的“opaque fallback”是刻意加上的实验补丁：它把 `select/index/load/composite/string/store` 等高层形态先收敛成可验证的 LLVM dialect / LLVM IR 占位值或副作用占位，而不是假装已经完成了真实 lowering。

## Validation Strategy

当前实验链路的验证分成五层：

1. lowering：`cmd/mlse-goir-llvm-exp -emit=llvm-dialect` 是否成功把 GoIR 文本降成 LLVM dialect MLIR
2. MLIR parse：优先用 `mlir-opt` 检查中间层是否可被 MLIR 正常解析
3. translation：`mlir-translate --mlir-to-llvmir` 是否成功把 LLVM dialect MLIR 翻成 LLVM IR
4. verifier：优先用 `opt` verifier；如果没有 `opt`，再尝试 `llvm-as`
5. compile：独立记录 `clang -c` 是否能够把生成的 LLVM IR 编译成目标文件

这些状态会分别进入 `report.json` / `report.md`，不再把“能翻译”和“能编译”折叠成同一个 success。

## 当前实验结论

当前仓库已经验证了下面几类样本：

- `examples/go/simple_add.go`
- `examples/go/sign_if.go`
- `examples/go/choose_if_else.go`
- `examples/go/choose_merge.go`
- `examples/go/sum_for.go`
- `examples/go/switch_value.go`
- 成功 GoIR 样本里的 `mmapSize` 风格整数返回函数
- 成功 GoIR 样本里的 `prealloc*` 风格 opaque-pointer / 外部调用函数
- `ByteOrder` 风格的宽松条件样本现在可通过 opaque fallback 翻译
- `if` / `for` 体内新建局部变量现在可被 stack-slot lowering 接住
- `multi-case switch` 仍会被显式拒绝

`../gobench-eq/dataset/cases/goeq-spec-*` 的当次批跑结果已经固定到：

- `artifacts/goeq-spec-batch/summary.tsv`
- `artifacts/goeq-spec-batch/report.md`

截至 `2026-03-20`，最近一次 **完整批跑** 的结果为：

- frontend success: `78/78`
- lowering success: `78/78`
- MLIR parse success: `78/78`
- translation success: `78/78`
- verifier success: `78/78`
- 双边都 verifier-success 的 case: `39/39`

这次数字已经对应“先产 LLVM dialect MLIR，再走 `mlir-translate`”之后的最新完整批跑。`goeq-spec-*` 当前已经全量到达 LLVM IR / verifier-success。完整成功 case 名单固定在仓库产物：

- `artifacts/goeq-spec-batch/report.md`
- `artifacts/goeq-spec-batch/summary.tsv`

当次运行的日志和汇总报告默认写到：

- `artifacts/goir-llvm-exp/report.json`
- `artifacts/goir-llvm-exp/report.md`
- `artifacts/goir-llvm-exp/results.jsonl`

成功样本的持久化落盘默认写到：

- `testdata/goir-llvm-exp/successes/<sample>/goir.mlir`
- `testdata/goir-llvm-exp/successes/<sample>/llvm-dialect.mlir`
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

## 与正式 `go` dialect 的关系

当前实验链路继续保留，原因是它仍然是最快的覆盖率摸底入口。

但从架构上说，后续不应继续把这条路径无限扩成正式实现。更合理的分工是：

- `cmd/mlse-go` / `cmd/mlse-goir-llvm-exp`：继续承担实验和批跑
- `include/mlse/Go/IR`、`lib/Go/IR`、`tools/mlse-opt`：承接正式 `go` dialect 的长期实现

## 设计建议

这轮为了摸清 `gobench-eq` 的覆盖情况，已经临时引入了一部分 opaque fallback；当前完整 batch blocker 已经清零，但这些 fallback 仍然不等于语义保真。

下一步不应该继续无约束地把更多 `mlse.*` 占位语句硬翻成 LLVM dialect / LLVM IR，而应该把这些 fallback 逐步收敛成明确 contract。

更合理的顺序是：

1. 先定义一个 verifier-backed 的 `GoIR-L0` 子集契约。
2. 明确哪些 `cmd/mlse-go` 输出已经满足“标量 + 预声明局部 + 受限控制流”约束。
3. 让 `GoIR-L0` 先稳定落到 LLVM dialect，并复用标准 `mlir-translate` 导出 LLVM IR。
4. 把目前为覆盖率加入的 `!go.any` / pointer-like fallback，逐步替换成 verifier-backed 的 typed contract。
5. 在当前 `if/for/switch/range` 子集批跑稳定后，再把更真实的容器布局、值合流和更通用的调用/返回建模纳入契约。

## Obsidian 同步说明

按仓库约定，这项实验后续应同步到 vault `next` 下的 `mlse设计/02-GoIR/`。

本次会话已经同步更新了 vault `next/mlse设计/02-GoIR/` 里的相关说明，避免设计文档继续停留在“仓库内尚无任何实验实现”的旧状态。
