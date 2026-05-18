---
date: 2026-05-18
status: 已完成
owner: Codex
scope: cmd/mlse-debug trace/scopes, symbolic-diff artifacts, docs, tests
---

# 为 mlse-debug 增加 trace/scopes 调试基础

## 摘要

本轮按用户要求继续推进 `cmd/mlse-debug`，目标是基于当前符号执行与 symbolic-diff 产物，为 debug 页面建立第一版可扩展的 frame、scope 和 trace 数据模型与 UI。

## 状态流转

- 2026-05-18 20:01 CST：`正在执行`，取得工作锁，开始梳理现有 debug 页面、Go scope metadata 和 symbolic-diff 产物格式。
- 2026-05-18 20:14 CST：`已完成`，完成 trace/scopes 第一版实现、本地验证和文档同步，并释放工作锁。

## 变更

- `cmd/mlse-debug` 新增 `-trace` 参数，可载入统一 trace JSON 或 symbolic-diff Go pipeline probe 的 `summary.json`。
- `internal/debugview` 新增 `Scope`、`TraceSnapshot`、`TracePath`、`TraceFrame`、`TraceEvent` 数据模型，并解析 `go.scope_table` 与 symbolic-diff summary。
- debug 页面保留源码 / 指令双栏，并新增下方 `Scopes` / `Trace` 面板；scope 可筛选指令，trace 可展示 path、old/new frame 与 stage timeline。
- 新增 debugview 测试，覆盖 scope table 解析、trace summary 转换和 `/trace.json` endpoint。
- 更新 `README.md`、`docs/dev-setup.md`、`docs/spec.md`，记录 `-trace` 用法和当前边界。

## 验证

- `go test ./internal/debugview`
- `go test ./cmd/... ./internal/...`
- `staticcheck ./cmd/... ./internal/...`
- `scripts/build.sh`
- `scripts/lint.sh`
- `scripts/test-all.sh`
- HTTP/UI smoke：启动 `go run ./cmd/mlse-debug -addr 127.0.0.1:18081 -trace artifacts/symbolic-diff-go-pipeline-probe/summary.json ./examples/go/simple_add.go`，确认 `/` 含 Scopes/Trace 面板，`/debug.json` 返回 1 个 scope 与 3 条 trace path，`/trace.json` 返回 3 条 path 且第一条含 2 个 frame 和 14 个 event。
- `git diff --check`

## 交接

- 本地实现和验证完成，工作锁已释放；发布状态通过对应 PR 跟踪。
