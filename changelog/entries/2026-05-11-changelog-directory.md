---
date: 2026-05-11
status: 已完成
owner: Codex
scope: changelog, docs, agent-workflow
---

# 新增 changelog 目录与工作状态锁

## 摘要

新增根目录 `changelog/`，用它记录仓库级变更和 agent 工作状态。目录按渐进式披露组织：入口说明放在 `README.md`，当前工作锁放在 `status.md`，详细历史放在 `entries/`。

## 状态流转

- 2026-05-11：`正在执行`，创建 changelog 目录结构并同步仓库文档。
- 2026-05-11：`验证中`，检查 Markdown 链接、目录约定和最终 diff。
- 2026-05-11：`已完成`，释放工作锁，`status.md` 回到 `空闲`。

## 变更

- 新增 `changelog/README.md`，说明渐进式披露结构和单 agent 工作锁规则。
- 新增 `changelog/status.md`，作为唯一当前工作状态文件。
- 新增 `changelog/entries/README.md`，按最新优先索引详细记录。
- 新增 `changelog/templates/entry.md`，统一后续记录格式。
- 同步 README、docs 索引、spec、AGENTS 以及 Obsidian `next/mlse设计` 入口中的协作约束。

## 验证

- 已执行 `git diff --check`，未发现空白或补丁格式问题。
- 已检查新增 changelog 文件和主要 Markdown 链接目标存在。
- 已用 `rg` 核对 `changelog/status.md`、`工作状态`、`正在执行` 在入口文档和目录内的引用。
- 已尝试执行 `scripts/lint.sh`；失败点是既有 Go frontend 文件 `internal/gofrontend/formal_core_module.go` 需要 gofmt，本次未改动该文件。

## 交接

后续 agent 开始前，必须先读取 `changelog/status.md`。只有 `工作状态` 为 `空闲` 时，才能把它改成 `正在执行` 并开始工作。
