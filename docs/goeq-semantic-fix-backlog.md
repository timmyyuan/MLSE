# goeq semantic fix backlog

本文把当前 `gobench-eq / goeq-spec` 在 `mlse-go -emit=goir-like -> mlse-goir-llvm-exp -> mlir-translate -> opt verify` 路径上的问题整理成可执行 backlog。

目标不是单纯提升“可生成 LLVM IR”的覆盖率，而是提升：

1. LLVM IR 合法性
2. Go 语义保真度
3. 失败原因可观测性
4. 后续语义等价分析的可信度

---

## 当前基线

来自 `artifacts/goeq-llvm-bulk-probe/summary.json`：

- 总 Go 文件：`1235`
- 成功：`1191`
- 失败：`44`
- 成功率：`96.44%`

当前硬失败分布：

- `19`：LLVM dialect parse 失败
- `15`：`TypeSwitchStmt` 不支持
- `10`：函数路径 return 不完整（falls off the end）

当前软失败/语义降级主因：

- 多类表达式直接降级为 zero / nil
- 大量 Go 类型统一压成 `!llvm.ptr`
- unknown local / unknown expr / type mismatch 被静默吞成零值
- 字符串、slice、interface、composite 等高价值语义没有真实表示

---

# P0 — 先阻止“伪成功”

这组问题不先修，覆盖率数字会比真实语义支持更乐观。

## P0.1 禁止 unknown expression 默认回落为 zero

### 现状
`internal/goirllvmexp/emit.go` 中 `resolveTyped(...)` 的默认分支会把未识别值降为 `materializeZero(...)`。

### 风险
- 未支持语义被静默伪装成可编译程序
- probe 成功率被高估
- 后续语义等价判断不可信

### 修改建议
- 新增 lowering 模式：
  - `strict`
  - `permissive`
- `strict` 模式下：
  - 未识别表达式直接报错
- `permissive` 模式下：
  - 允许降级，但必须记录 semantic-loss

### 产出
- CLI 选项：`-lowering-mode=strict|permissive`
- 默认建议切到 `strict`
- 统一 semantic-loss 日志输出

### 预期收益
- 将“假成功”转成“真失败可见”
- 为后续修复排序提供可靠数据

---

## P0.2 禁止 unknown local `%foo` 静默变 zero

### 现状
`resolveTyped(...)` 中找不到局部 SSA 值时，直接回落成 zero。

### 风险
- SSA 断链不再暴露
- 语义错误被掩盖

### 修改建议
- unknown local 直接报错
- permissive 模式下记录：
  - 文件
  - 函数
  - 行号
  - value name
  - 期望类型

### 预期收益
- 把数据流问题前置暴露

---

## P0.3 禁止 type mismatch 用 zero 糊平

### 现状
`storeLocal(...)` / `coercedLocalLLVMType(...)` 遇到 slot type 不匹配时，存在用 zero 回填的路径。

### 风险
- 类型系统错误不再可见
- 程序行为被 silent rewrite

### 修改建议
- 引入显式 cast 规则：
  - `trunc`
  - `zext`
  - `sext`
  - `bitcast`
  - ptr/int 转换（仅在明确允许时）
- 不能合法转换时直接失败

### 预期收益
- 类型问题从“静默语义损失”改为“显式失败”

---

## P0.4 建 semantic-loss 审计报告

### 现状
当前 bulk probe 只统计 hard failure，不统计“编过但语义降级”。

### 修改建议
probe 为每个文件额外输出：

- `semantic_loss_count`
- `semantic_loss_kinds`
- 样例条目：
  - file
  - function
  - line
  - category
  - original_expr
  - lowered_as

建议 loss category 初版包括：

- `unknown_expr_to_zero`
- `unknown_local_to_zero`
- `type_mismatch_to_zero`
- `string_literal_to_zero`
- `index_to_zero`
- `slice_to_zero`
- `composite_to_zero`
- `typeassert_to_zero`
- `opaque_call_signature`
- `ptr_erasure`

### 预期收益
- 让“成功率”和“语义可信度”脱钩统计
- 为 P1/P2 排序提供依据

---

# P1 — 修最影响 goeq 的语义缺口

## P1.1 修复整数字面量宽度/符号问题

### 现状
当前有 `19` 个 parse 失败，至少一类明确是：

- `integer constant out of range for attribute`

### 根因
字面量在 lowering 时没有严格按源类型位宽归一化。

### 修改建议
- 在 GoIR/emit 交界处保留 typed literal 信息
- lowering 常量时根据目标类型做：
  - 位宽约束
  - signed/unsigned 区分
  - 必要的截断/封装
- 不允许把超宽整数字面量直接打印成目标位宽常量

### 任务拆分
1. 给 literal 增加显式类型归一化辅助函数
2. 补针对 `uint64/int64 -> i32/i16/i8` 的测试
3. 重跑 `goeq-llvm-bulk-probe`

### 预期收益
- 直接消除大部分 `llvm_dialect_parse` 失败

---

## P1.2 修复 return path 完整性

### 现状
当前有 `10` 个失败是：
- function falls off the end without returning

### 根因
CFG/terminator 覆盖不完整，某些路径没有显式 return 或 unreachable。

### 修改建议
- 在 GoIR lowering 前增加 must-return 分析
- 对语义上必不返回的路径输出：
  - `llvm.unreachable`
- 对 `if/switch/loop` 合流路径做完整性检查

### 任务拆分
1. 增加函数级 CFG 完整性检查
2. 针对 `if/else`、`switch`、`break/continue/goto` 补测试
3. 为“panic-like / impossible path”预留 unreachable 建模

### 预期收益
- 消除剩余 hard failure 中的 return 类问题
- 同时提升 verifier 稳定性

---

## P1.3 实现 `mlse.index` 的真实 lowering

### 现状
`mlse.index` 当前直接 zero fallback。

### 影响
- 数组/切片/字符串/多维索引的读取语义基本缺失
- goeq case 中大量局部行为被抹平

### 修改建议
按对象类型分别 lower：

- array index
  - aggregate extract / GEP + load
- slice index
  - 先解构 `{ptr,len,cap}`，再 GEP + load
- string index
  - data ptr + length 边界内访问

### 任务拆分
1. 在 GoIR 层区分 index target kind
2. 先支持 `array` / `slice` / `string`
3. 暂不支持 map index，先显式报 unsupported

### 预期收益
- 对 goeq 实际程序语义帮助非常大

---

## P1.4 实现 `mlse.composite` 的真实 lowering

### 现状
`mlse.composite` 当前直接 zero fallback。

### 影响
- 复合字面量全部退化成零值
- 数组/struct 初始化语义丢失

### 修改建议
- array literal -> aggregate constant / stack allocation + stores
- struct literal -> aggregate build
- slice literal -> backing array + `{ptr,len,cap}`

### 任务拆分
1. 先支持定长 array literal
2. 再支持 struct literal
3. 最后支持 slice literal

### 预期收益
- 可恢复大量 case 中的初始化语义

---

## P1.5 修复字符串字面量表示

### 现状
quoted string literal 当前走 zero fallback。

### 影响
- string 常量不是 string 常量
- 涉及 split/compare/len/IO 的程序语义严重不可信

### 修改建议
建立 string runtime layout：

```text
!mlse.go_string := { ptr, len }
```

lowering：
- 生成全局字节数组常量
- 生成指向首元素的指针
- 组合成 `{data,len}`

### 任务拆分
1. 定义 string lowering helper
2. 接入 quoted string literal
3. 补 `len(string)` / compare / pass-through 测试

### 预期收益
- 对文本类 goeq case 的语义提升非常关键

---

## P1.6 实现 `mlse.slice` 的真实 lowering

### 现状
`mlse.slice` 当前直接 zero fallback。

### 影响
- 子切片、长度、容量语义全部丢失

### 修改建议
建立 slice 表示：

```text
!mlse.go_slice<T> := { data_ptr, len, cap }
```

支持：
- `a[i:j]`
- `a[i:]`
- `a[:j]`

### 预期收益
- 让数组/字符串/切片相关程序首次具备基本可解释语义

---

# P2 — 修类型系统与运行时表示

## P2.1 不再把大多数 Go 类型统一压成 `!llvm.ptr`

### 现状
`mustLLVMType(...)` 目前除少量整数/布尔外，大量类型统一映射到 `!llvm.ptr`。

### 影响
- string/slice/interface/array/struct/function 全部语义挤压

### 修改建议
分层建模：

第一批：
- string
- slice
- array
- struct

第二批：
- interface / any
- func value / closure

第三批：
- map / channel（按需求）

### 预期收益
- 从根本上减少“指针壳化”语义损失

---

## P2.2 实现 `typeassert`

### 现状
`mlse.typeassert` 当前直接 zero fallback。

### 修改建议
前提是先有 interface layout：

```text
{ type_id, data_ptr }
```

lowering 时：
- 比较动态类型 tag
- success path 提取 data
- failure path 返回 zero value 或 ok=false（取决于语法形态）

### 预期收益
- 为 type switch 做准备

---

## P2.3 实现 `TypeSwitchStmt`

### 现状
当前 hard failure：`15`

### 修改建议
- 基于 interface runtime tag lower 成多分支 compare
- 先支持：
  - `switch x := v.(type)`
  - case concrete named type
  - default
- 暂不支持复杂接口嵌套匹配

### 预期收益
- 直接清掉一大类 hard failure
- 提升动态类型场景的语义可信度

---

# P3 — 修调用与存储语义

## P3.1 为 runtime/builtin/known extern 建正式 ABI

### 现状
当前调用经常通过 mangled extern 方式强行变成“类型上可通过”的调用。

### 风险
- 参数/返回语义被 ptr 化
- side effect 与调用约定缺乏约束

### 修改建议
把 call 分三类：

1. internal function
   - 必须严格匹配签名
2. known runtime/builtin
   - 使用正式 ABI
3. unknown extern
   - strict 下报错
   - permissive 下标记为 `opaque_call`

### 预期收益
- 调用语义更可分析

---

## P3.2 修复 `addr/load/store` 的对象语义

### 现状
当前 `mlse.addr` / `mlse.load` / `store_*` 在很多对象上只是 pointer shell 操作。

### 修改建议
- 对 aggregate/object type 建真实内存布局
- `addr` 必须指向真实对象
- `load/store` 必须使用对象 element type
- 不允许在未知对象上默认 ptr 化继续走

### 预期收益
- 修复指针相关 case 的核心语义缺口

---

# P4 — 提升测试与统计体系

## P4.1 为每个语义族补最小回归测试

建议新增测试目录，例如：

```text
testdata/goirllvmexp/
  literals/
  returns/
  composite/
  index/
  strings/
  slices/
  interface/
  typeswitch/
```

每个测试至少验证：
- LLVM dialect 可 parse
- 可 translate 到 LLVM IR
- `opt verify` 通过
- 若可行，增加 FileCheck 断言关键语义结构

---

## P4.2 bulk probe 报告拆成两套指标

建议报告同时输出：

### legality metrics
- frontend success
- lowering success
- llvm dialect parse success
- translation success
- verifier success

### semantic fidelity metrics
- files with semantic loss
- semantic loss count per file
- top semantic loss categories
- strict-mode success rate
- permissive-mode success rate

### 预期收益
- 避免“96% 成功率”掩盖真实语义问题

---

# 建议的执行顺序

## 第一周目标（先把数据变真实）
1. P0.1 unknown expr 不再默认 zero
2. P0.2 unknown local 直接报错
3. P0.3 type mismatch 不再 zero 糊平
4. P0.4 semantic-loss 报告落地
5. P1.1 literal 宽度/符号修复
6. P1.2 return path 完整性修复

### 成功标准
- strict mode 可以跑完整个 bulk probe
- 报告中区分 hard failure / semantic loss
- parse/return 类错误明显下降

## 第二周目标（恢复高价值值语义）
1. P1.5 string literal
2. P1.3 index
3. P1.4 composite
4. P1.6 slice

### 成功标准
- 文本/数组/切片类 case 的 strict success rate 明显上升
- semantic loss 中 `index/composite/slice/string_literal_to_zero` 显著下降

## 第三周目标（动态类型与调用）
1. P2.1 核心类型表示
2. P2.2 typeassert
3. P2.3 TypeSwitchStmt
4. P3.1 call ABI 规范化
5. P3.2 addr/load/store 对象化

---

# 建议新增 issue/任务卡

建议直接拆成以下任务卡：

1. `goirllvmexp: add strict/permissive lowering mode`
2. `goirllvmexp: emit semantic-loss audit report`
3. `goirllvmexp: reject unknown locals instead of zero fallback`
4. `goirllvmexp: remove type-mismatch-to-zero behavior`
5. `goirllvmexp: normalize typed integer literals before LLVM emission`
6. `goirllvmexp: ensure all function CFG paths terminate`
7. `goirllvmexp: lower string literals to global bytes + len`
8. `goirllvmexp: lower array/slice/string indexing`
9. `goirllvmexp: lower composite literals`
10. `goirllvmexp: introduce slice runtime representation`
11. `goirllvmexp: introduce string runtime representation`
12. `goirllvmexp: model interface/any runtime representation`
13. `goirllvmexp: implement type assertions`
14. `goirllvmexp: implement type switch lowering`
15. `goirllvmexp: distinguish known ABI calls from opaque externs`
16. `goeq probe: report legality metrics and semantic fidelity metrics separately`

---

# 最终目标

bulk probe 未来应该至少有三条数：

1. **合法率**：LLVM IR 是否能生成并通过 verifier
2. **严格成功率**：不依赖语义降级的真实成功率
3. **语义保真率**：无 semantic-loss 或 semantic-loss 低于阈值的成功率

如果只看第 1 条，容易得到一个乐观但不可信的结论。
对 goeq 来说，真正重要的是第 2 条和第 3 条。
