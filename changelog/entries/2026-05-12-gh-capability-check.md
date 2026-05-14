---
date: 2026-05-12
status: 已完成
owner: Codex
scope: gh, issue, pull-request, merge
---

# gh 能力验证

## 摘要

本轮根据用户要求接管工作锁，验证当前环境是否能通过 `gh` 在 GitHub 上创建 issue、创建 PR，并合并 PR。测试改动限定在 changelog 记录，避免触碰业务代码。

## 状态流转

- 2026-05-12 11:22:42 CST：`正在执行`，用户明确要求接管上一轮工作锁并开始 gh 能力验证。
- 2026-05-12 11:30 CST：发现环境变量 `GH_TOKEN` / `GITHUB_TOKEN` 覆盖 keyring 登录态，导致 issue/PR 写入 API 返回 403。
- 2026-05-12 11:31:28 CST：`已完成`，用 keyring 登录态完成 issue 创建、PR 创建、PR 合并，并清理临时远端分支。

## 变更

- 本地更新 `changelog/status.md`，记录用户接管要求和本轮验证结果。
- 本地新增本记录文件 `changelog/entries/2026-05-12-gh-capability-check.md`。
- 远端临时分支 `codex/gh-capability-base-20260512-112242` 和 `codex/gh-capability-pr-20260512-112242` 曾用于环境变量 token 测试，测试后已删除。
- 远端临时分支 `codex/gh-capability-base-20260512-1132` 和 `codex/gh-capability-pr-20260512-1132` 曾用于 keyring 登录态测试；PR 合并后已删除。

## 验证

- `gh auth status`：当前可用 GitHub 登录态存在，仓库访问账号为 `timmyyuan`。
- `gh repo view --json nameWithOwner,viewerPermission,defaultBranchRef,isPrivate,url`：仓库为 `timmyyuan/MLSE`，默认分支 `main`，viewerPermission 为 `ADMIN`。
- `env -u GH_TOKEN -u GITHUB_TOKEN gh auth status`：keyring 登录态可用，scope 包含 `repo`。
- `gh issue list --limit 3`：可读取 issue 列表。
- `gh pr list --limit 3`：可读取 PR 列表。
- `git push -u origin codex/gh-capability-base-20260512-112242`：可创建远端分支。
- `git push -u origin codex/gh-capability-pr-20260512-112242`：可推送测试 PR 分支。
- `git push origin --delete codex/gh-capability-pr-20260512-112242 codex/gh-capability-base-20260512-112242`：可删除临时远端分支。
- `env -u GH_TOKEN -u GITHUB_TOKEN gh issue create ...`：成功创建测试 issue #66。
- `env -u GH_TOKEN -u GITHUB_TOKEN gh pr create ...`：成功创建测试 PR #67。
- `env -u GH_TOKEN -u GITHUB_TOKEN gh pr merge 67 --squash --delete-branch ...`：成功合并测试 PR #67，merge commit 为 `9ed54aa13e5cd898172e2f5127f30e329d3254b5`。
- `env -u GH_TOKEN -u GITHUB_TOKEN gh issue close 66 ...`：成功关闭测试 issue #66。
- `git ls-remote --heads origin 'refs/heads/codex/gh-capability-*'`：确认测试远端分支已清理。

## PR 与 CI

- 使用当前环境变量 token 时，`gh issue create ...` 失败：`GraphQL: Resource not accessible by personal access token (createIssue)`。
- 使用当前环境变量 token 时，`gh api repos/timmyyuan/MLSE/issues ...` 失败：HTTP 403，`Resource not accessible by personal access token`。
- 使用当前环境变量 token 时，`gh pr create ...` 失败：`GraphQL: Resource not accessible by personal access token (createPullRequest)`。
- 使用当前环境变量 token 时，`gh api repos/timmyyuan/MLSE/pulls ...` 失败：HTTP 403，`Resource not accessible by personal access token`。
- unset `GH_TOKEN` / `GITHUB_TOKEN` 后，keyring 登录态可以完成完整链路：issue #66、PR #67、squash merge。
- PR #67 的 base 是隔离分支 `codex/gh-capability-base-20260512-1132`，不是 `main`；本轮没有改动默认分支。

## 交接

- 如果后续要用 `gh` 执行 issue/PR 写操作，需要避免当前 `GH_TOKEN` / `GITHUB_TOKEN` 覆盖 keyring 登录态；可在命令前加 `env -u GH_TOKEN -u GITHUB_TOKEN`。
- 测试 issue #66 已关闭，测试 PR #67 已合并到已删除的隔离 base 分支，临时远端分支和本地 worktree 均已清理。
