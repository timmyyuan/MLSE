---
date: 2026-05-18
status: 已完成
owner: Codex
scope: cmd/mlse-debug trace panel CSS fix
---

# 修复 mlse-debug trace 面板隐藏样式

## 摘要

真实浏览器检查发现 `Scopes` tab 下仍能暴露 `Trace` 面板内容。原因是 `.panel` 的 display 规则覆盖了 `.hidden`。本轮修复隐藏优先级并重新验证页面。

## 状态流转

- 2026-05-18 20:21 CST：`正在执行`，取得工作锁，修复 trace/scopes 页面 CSS 隐藏优先级。
- 2026-05-18 20:24 CST：`已完成`，完成 CSS 修复、真实浏览器复验和本地检查，并释放工作锁。

## 变更

- 修复 `internal/debugview/assets.go` 中 `.hidden` 的 display 优先级，避免 `.panel` 规则覆盖隐藏状态。
- 重新用 Safari 验证 `Scopes` tab 和 `Trace` tab 的可见内容隔离。

## 验证

- `go test ./internal/debugview`
- `go test ./cmd/... ./internal/...`
- `scripts/lint.sh`
- Safari smoke：启动 `go run ./cmd/mlse-debug -addr 127.0.0.1:18083 -trace artifacts/symbolic-diff-go-pipeline-probe/summary.json ./examples/go/simple_add.go`，确认 `Scopes` tab 仅显示 `scope0/main.add`，点击 `Trace` tab 后才显示 path、old/new frame 和 stage events。
- `git diff --check`

## 交接

- 本地实现和验证完成，工作锁已释放；发布状态通过对应 PR 跟踪。
