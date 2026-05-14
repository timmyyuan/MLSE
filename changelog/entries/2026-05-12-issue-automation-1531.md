---
date: 2026-05-12
status: 已完成
owner: Codex automation mlse
scope: github-issue, implementation, tests, pull-request
---

# GitHub issue 自动化修复运行

## 摘要

本轮按自动化要求先读取 changelog 工作状态；确认 `工作状态` 为 `空闲` 后取得工作锁。随后继续上一轮未发布的 issue #14 修复，将旧分支上的实际修复移植到最新 `origin/main` 的 clean 分支，完成本地验证、PR、CI 修复和 squash merge。

## 状态流转

- 2026-05-12 15:31 CST：`正在执行`，取得 changelog 工作锁，开始检查 GitHub 权限、未发布工作与可执行范围。
- 2026-05-12 15:45 CST：确认 issue #14 仍 open，旧本地修复 commit 仍有效，但旧分支混有已 squash 合并的 PR #10 历史；新建 clean worktree 从最新 `origin/main` cherry-pick 修复。
- 2026-05-12 15:52 CST：完成本地验证并创建 PR #68。
- 2026-05-12 15:58 CST：首轮 CI 中 Docker KLEE symbolic-diff smoke 失败；日志显示 issue #14 已到达 `klee_model_unavailable`，另有 mod20 / mod37 的 expected blocker 滞后。
- 2026-05-12 16:01 CST：补充 mod20 / mod37 的 expected blocker，推送更新后第二轮 CI 全部通过。
- 2026-05-12 16:07 CST：PR #68 已 squash merge 到 `main`，issue #14 自动关闭，本轮完成并释放工作锁。

## 选择依据

- 继续 issue #14 `SD-P0: Add regression for any boxing of pointer-to-struct`。
- 选择原因：上一轮已经完成本地修复但发布受阻；本轮 `gh` keyring 登录态可用，且 issue #14 仍 open，适合优先完成发布闭环。

## 变更

- `internal/gofrontend/formal.go`：局部标识符表达式在带目标类型 hint 时会触发 coercion。
- `internal/gofrontend/formal_type_convert.go`：目标类型为 `any` handle 时调用 `runtime.any.box.*` 装箱。
- `internal/gofrontend/testdata/any_pointer_struct_box.go`：新增 `*pair` 传入 `sink(any)` 的 FileCheck 回归。
- `scripts/mlse-diff-fuzz-smoke.py`：整理参数对象以满足 lint 阈值，并把临时 `go.mod` 固定到 `go 1.22.0`。
- `test/SymbolicDiff/cases/motus-mod18-foo1-foo2/case.json`：blocker 从 `old_mlse_opt_roundtrip_failed` 前进到 `klee_model_unavailable`。
- `test/SymbolicDiff/cases/motus-mod20-foo1-foo2/case.json`、`test/SymbolicDiff/cases/motus-mod37-getrankextend1-getrankextend2/case.json`：CI 暴露这两例也已前进到 `klee_model_unavailable`，同步 expected blocker。

## 验证

- `go test ./internal/gofrontend`：通过。
- `go test ./cmd/... ./internal/...`：通过。
- `staticcheck ./cmd/... ./internal/...`：通过。
- `scripts/test-all.sh`：通过。
- `scripts/lint.sh`：通过。
- `python3 scripts/mlse-diff-go-pipeline-probe.py --case motus-mod18-foo1-foo2 --expect-blocker klee_run_not_requested`：通过；本机未跑 KLEE，但 old/new 均通过到 bitcode 阶段。
- `python3 scripts/mlse-diff-go-pipeline-probe.py --case motus-mod18-foo1-foo2 --case motus-mod20-foo1-foo2 --case motus-mod37-getrankextend1-getrankextend2 --expect-blocker klee_run_not_requested`：通过；用于验证补充 metadata 的三例本地均通过到 bitcode 阶段。
- `python3 -m json.tool` 校验三份相关 `case.json`：通过。
- `git diff --check` / `git diff --check HEAD~1..HEAD`：通过。

## PR 与 CI

- PR：[timmyyuan/MLSE#68](https://github.com/timmyyuan/MLSE/pull/68)。
- 首次创建 PR 时当前 `GH_TOKEN` 缺少 `createPullRequest` 权限；改用 keyring 登录态后创建成功。
- 首轮 CI：`Go and symbolic-diff smoke`、`Dockerfile check` 通过；`Docker KLEE symbolic-diff smoke` 失败，原因是 mod20 / mod37 expected blocker 滞后。
- 第二轮 CI：三项 check 全部通过。
- 已 squash merge，merge commit `c790f80c49929c5fd43b18b6eae7c56603f674c8`。
- issue #14 已自动关闭。

## 交接

- 当前主 PR 已合并，无后续发布事项。
- 仓库根工作树在本轮开始前已有用户/前序 agent 未提交改动；本轮发布工作在 `tmp/issue14-pr` clean worktree 内完成，没有回退那些既有改动。
