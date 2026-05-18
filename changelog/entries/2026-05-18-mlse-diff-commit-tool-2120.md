---
date: 2026-05-18
status: 已完成
owner: Codex
scope: cmd/mlse-diff, symbolic-diff repo commit analysis, docs, tests
---

# 新增 commit 间函数级 diff 等价性工具

## 摘要

本轮按用户要求新增一个命令行工具，分析目标 Go 代码仓库两个 commit 之间的函数级改动，生成 symbolic-diff case，并调用现有 Go/KLEE pipeline 以 same-input harness 验证 old/new 输出是否一致。

## 状态流转

- 2026-05-18 21:20 CST：`正在执行`，取得工作锁，开始阅读 symbolic-diff、执行层与 GoIR 相关文档和脚本。
- 2026-05-18 21:39 CST：`已完成`，实现、文档和本地验证完成，准备提交 PR。

## 变更

### 用户可见

- 新增 `cmd/mlse-diff`：输入目标 Git 仓库、old commit 和 new commit，生成现有 symbolic-diff probe 可消费的 `old.go` / `new.go` / `case.json`。
- 默认调用 `scripts/mlse-diff-go-pipeline-probe.py`；不带 `-run-klee` 时验证 old/new 两侧能到 LLVM bitcode，带 `-run-klee` 时会要求 probe summary 为 `ok`。
- 支持两类函数级改动：
  - 同名入口函数内部修改。
  - 入口函数拆分 / 合并 helper；同 package 的其它变更 Go 文件会被合并进生成 case。入口改名时可用 `-old-function` / `-new-function` 生成同签名 wrapper。

### 实现边界

- 自动 KLEE model 当前覆盖 `int` / `int64` 标量参数返回值，以及一个 `[]int -> []int` 的 `slice_i64` model。
- 暂不把 unsupported 签名伪装成等价证明；生成的 case 会记录 `klee_model_unavailable` blocker。
- `scripts/mlse-diff-go-pipeline-probe.py` 现在允许 `cases-root` 位于仓库外部，外部路径会按原路径记录。

### 文件

- 新增 `cmd/mlse-diff/main.go`。
- 新增 `internal/symbolicdiff/` 的 commit diff 分析、case 生成、pipeline 调用与单元测试。
- 更新 `scripts/build.sh`、`scripts/test-all.sh`，把 `mlse-diff` 纳入构建和统一检查。
- 更新 `README.md`、`docs/dev-setup.md`、`docs/spec.md`。
- 按仓库约定同步 Obsidian `mlse设计/02-GoIR/06-构建与集成.md` 的当前工具接口与 symbolic-diff 集成说明。

## 验证

- `go test ./internal/symbolicdiff`
- `go test ./cmd/... ./internal/...`
- `go test ./linters`
- `scripts/build.sh`
- `staticcheck ./cmd/... ./internal/...`
- `scripts/test-all.sh`
- `scripts/lint.sh`
- CLI smoke：临时 Git 仓库中把 `F` 拆成 `F -> inc`，且 `inc` 位于新增 helper 文件；`mlse-diff` 生成 case 后，现有 probe 的 old/new 两侧均通过 `mlse-go -> mlse-opt -> lower-go-bootstrap -> mlir-opt -> mlir-translate -> llvm-as`，最终只因未传 `-run-klee` 停在 `klee_run_not_requested`。

## 交接

- 本轮代码、文档和本地验证已完成；等待 PR/CI。
