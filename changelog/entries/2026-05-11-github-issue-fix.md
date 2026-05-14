---
date: 2026-05-11
status: 发布受阻
owner: Codex automation mlse
scope: github-issue, implementation, tests
---

# GitHub issue 修复自动化运行

## 摘要

本轮选择 GitHub issue #14 `SD-P0: Add regression for any boxing of pointer-to-struct`，实现了 Go frontend 中 `*struct` 传入 `any` 形参时的 boxing 修复，并在隔离 worktree 中完成本地提交与验证。

远端发布尚未完成：当前线程里的 shell `gh` 无可用 keyring token，且 shell 网络无法解析 `github.com`；GitHub connector 写入大文件对象也被用户拒绝。因此本轮停在“本地 commit 已准备好、等待发布通道恢复”的状态。

## 状态流转

- 2026-05-11 21:48:09 CST：`正在执行`，取得 changelog 工作锁并准备读取仓库文档与 GitHub issue。
- 2026-05-12 11:37:38 CST：`发布受阻`，本地修复和验证完成，但无法通过当前线程发布 PR。

## 选择依据

- 已读取仓库说明、`README.md`、`docs/spec.md` 与 Obsidian vault `next` 中 `mlse设计/00-MLSE设计文档.md`、`mlse设计/02-GoIR/` 相关文档。
- 已检查 GitHub issue；#1 是父级 tracker，范围过大；#14 是较早创建的 P0 具体回归，且直接对应当前 GoIR symbolic-diff 链路中的 mod18 round-trip blocker。
- 处理前先合并了已通过 CI 的 PR #10，避免在过期 main 上继续修复。

## 变更

- `internal/gofrontend/formal.go`：局部标识符表达式在带有目标类型 hint 时执行 `coerceFormalValueToHint`，让 `sink(p)` 这类调用保留形参期望类型。
- `internal/gofrontend/formal_type_convert.go`：当 coercion 目标是 `any` handle 时调用 runtime any boxing。
- `internal/gofrontend/testdata/any_pointer_struct_box.go`：新增 FileCheck 回归，要求 `*pair` 传入 `sink(any)` 时生成 `runtime.any.box.ptr.pair`，且 `sink` 接收 `!go.named<"any">`。
- `test/SymbolicDiff/cases/motus-mod18-foo1-foo2/case.json`：将 blocker 从 `old_mlse_opt_roundtrip_failed` 更新为 `klee_model_unavailable`，因为 MLIR round-trip 已修通。
- `scripts/mlse-diff-fuzz-smoke.py`：整理 Python metrics 违规函数参数，并把临时 fuzz module 的 `go.mod` 从 `go 1.25` 改为 `go 1.22.0`，避免本地 Go 1.23 自动下载 1.25 toolchain。
- 本地提交：`e98d7a802f18fd3f7702bf5ea491ef39ee8f4df2`，提交信息 `fix Go any boxing for struct pointers`。

## 验证

- `go run ./cmd/mlse-go internal/gofrontend/testdata/any_pointer_struct_box.go | rg 'runtime.any.box.ptr.pair|testdata.sink|func.func @testdata.F'`
- `go test ./internal/gofrontend`
- `go test ./cmd/... ./internal/...`
- `scripts/lint.sh`
- `scripts/test-all.sh`
- `git diff --check`

验证结果均通过。`mlse-go` 输出中已确认 `runtime.any.box.ptr.pair(%new3)`，随后 `testdata.sink` 接收 `!go.named<"any">`。

## PR 与 CI

- 已创建本地分支名：`codex/issue-14-any-pointer-boxing`。
- 尚未创建 PR，尚未等待 CI，尚未合并。
- 当前线程中按 `env -u GH_TOKEN -u GITHUB_TOKEN` 绕开环境变量 token 后，`gh auth token` 返回 `no oauth token found for github.com`。
- 当前线程中 `git push origin e98d7a802f18fd3f7702bf5ea491ef39ee8f4df2:refs/heads/codex/issue-14-any-pointer-boxing` 返回 `Could not resolve host: github.com`。
- 之前尝试通过 GitHub connector 的 Git blob API 发布时，已校验部分小文件 blob SHA；但大文件对象写入被用户拒绝，未创建 tree/commit，也未移动远端分支。

## 交接

- 若继续本轮修复，优先让当前线程可用 `env -u GH_TOKEN -u GITHUB_TOKEN gh ...` 访问 keyring 登录态，并恢复 shell 到 `github.com` 的 DNS/网络。
- 发布通道恢复后，推送本地 commit 到 `codex/issue-14-any-pointer-boxing`，创建 PR，添加 `codex` 与可用时的 `codex-automation` 标签，等待 GitHub CI 完成。
- CI 通过后 squash merge PR，并关闭 issue #14。
