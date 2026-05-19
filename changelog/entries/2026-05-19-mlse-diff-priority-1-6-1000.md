# 修复 mlse-diff 优先级 1-6 case

| 字段 | 值 |
| --- | --- |
| 日期 | 2026-05-19 |
| 状态 | 已完成 |
| 范围 | symbolic-diff KLEE model、Go frontend、CI、文档 |
| 关联请求 | 修复优先级 1 到 6 并补对应 CI |

## 摘要

本轮目标覆盖：

- `mod14`：全局变量符号输入。
- `mod38`：窄数字 `fmt.Sprintf`。
- `mod26`：小型 pointer / map 模型。
- `mod8` / `mod35`：pointer slice / 对象图差异。
- `mod27` / `mod28`：request struct、map、strings 小模型。
- `mod32` / `mod33` / `mod34`：frontend round-trip blocker。

## 当前进展

- 已确认 changelog 工作锁为 `空闲`，本轮切换为 `正在执行`。
- 已阅读仓库 README / spec / dev setup 和 Obsidian GoIR 测试文档。
- 已跑 priority 1-6 baseline probe，确认分层：
  - `mod14` / `mod38` / `mod26` / `mod8` / `mod35` / `mod27` / `mod28` 已可到 LLVM，缺 KLEE model。
  - `mod32` / `mod33` / `mod34` 卡在 formal MLIR round-trip。
- 已完成实现并进入验证：priority 1-5 新增 KLEE model，priority 6 修掉 frontend round-trip blocker。

## 实现

- Go frontend：
  - 普通函数调用现在按签名 hint 对实参做 coercion，`any` 形参会触发 `runtime.any.box.*`。
  - 用户自定义 `args ...any` 会在 call site 打包成 `!go.slice<!go.named<"any">>`，再按 callee 的三参数签名调用。
  - 新增 `direct_any_arg_box.go` 和 `variadic_any_call.go` FileCheck fixture。
- symbolic-diff KLEE probe：
  - `go_llvm` 新增 `i64`、`string_bool`、`slice_ptr`、`conf_ptr`、`mod27_req_ptr`、`mod28_req_ptr`、`opaque_ptr` 等测试 ABI。
  - 新增符号全局输入支持，用于 `mod14` 的 `GlobalInput`。
  - 扩展 `runtime.fmt.Sprintf` 的 equality-preserving `%d` / `%.1fK` 数字格式化模型，用于 `mod38`。
  - 增加测试所需的 map/string helper stub，用于 `mod27` / `mod28` 的 bounded harness。
- case / CI：
  - `mod8`、`mod14`、`mod26`、`mod27`、`mod28`、`mod35`、`mod38` 加入 `supported-klee-mod-cases.txt`。
  - `mod32` / `mod33` / `mod34` 的 expected blocker 从 round-trip failure 前移到 `klee_model_unavailable`，CI 会锁住“不再退回 round-trip 失败”。
  - `mod35` 当前 fixture 的 `success` 只返回 `CommonResp.Code=0`，实际可观测输出等价，因此把 `expected_status` 调整为 `equivalent` 并在模型 notes 里说明。
- 文档：
  - 更新 repo 文档中的 Go frontend variadic/any 边界和 symbolic-diff KLEE 支持面。
  - 同步 Obsidian `mlse设计/02-GoIR` 的方言、构建集成和测试说明。

## 测试

- `go test ./internal/gofrontend` 通过。
- `go build -o artifacts/bin/mlse-go ./cmd/mlse-go` 通过。
- priority 1-6 probe（无 KLEE）通过 Go -> MLIR -> LLVM bitcode 阶段；`mod32` / `mod33` / `mod34` round-trip blocker 已消失。
- 使用 `/usr/bin/true` 作为 fake KLEE 验证新增 7 个 supported case 的 harness `llvm-as` 和 `llvm-link` 阶段通过。
- 使用 fake KLEE 验证 `mod32` / `mod33` / `mod34` 当前 blocker 稳定为 `klee_model_unavailable`。

## 剩余边界

- 本机没有真实 KLEE，完整 KLEE 执行留给 GitHub Docker CI。
- `mod32` / `mod33` / `mod34` 目前只修复 frontend/LLVM 可达性，还没有纳入 supported KLEE 清单。
- map/string helper 仍是 repo-owned fixture 的 bounded KLEE 模型，不表示完整 Go map 或 stdlib 语义。
