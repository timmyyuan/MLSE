# 批量导入 Motus smtcmp mod* showcase

| 字段 | 值 |
| --- | --- |
| 日期 | 2026-05-18 |
| 状态 | 已完成 |
| 范围 | artifacts/mlse-diff-showcase |
| 关联请求 | 将 motus `pkg/analysis/ssa/smtcmp/testdata/mod*` 全部移植到浏览器 showcase |

## 摘要

本轮把 Motus `smtcmp/testdata/mod1` 到 `mod39` 全部加入本地 `mlse-diff` showcase。

页面现在保留原有 3 个演示例子，并额外加载 39 个 Motus 样例卡片。每个 Motus 样例展示入口函数 pair、synthetic commit 形式的 `mlse-diff -run-pipeline=false` 命令、case/model 结果、源码 diff 片段，以及 100 条 AFL 风格 same-input 队列。

## 结果

- 共导入 `39` 个 Motus mod 样例。
- `39` 个样例均可以完成 `mlse-diff` prepare 并生成 case。
- 早先被工具边界阻塞的 `mod8`、`mod20`、`mod22`、`mod32`、`mod33`、`mod37`，在后续 `method receiver` 和非入口 `variadic helper` 支持补齐后，已从 `blocked` 转为已生成 case 的 `unsupported` 状态。
- 页面总卡片数为 `42`：原有 3 个 + Motus 39 个。

## 展示策略

Motus 样例按渐进式披露展示：

- 卡片层只展示 mod 编号、函数 pair、model 状态和签名。
- 详情层展示命令、case metadata、old/new 源码片段。
- 队列层统一播放 100 条输入。
- 能用轻量页面模型表达的样例显示 `same` / `mismatch`。
- 复杂但已生成 case 的样例标记为 `queued`，表示等待后续 KLEE model 覆盖。
- prepare 阶段失败的样例标记为 `blocked`，直接展示 blocker 原因。

## 验证

- `node` 静态解析 `index.html` inline script 与 `motus-mods.js` 通过。
- `motus-mods.js` 覆盖 `mod1` 到 `mod39`，无缺号。
- 当前 `motus-mods.js` 中 `blocked = 0`。
- 本地 HTTP 服务 `http://127.0.0.1:8765/` 正常返回新页面与 `motus-mods.js`。
- 浏览器页面加载后显示 `42/42 个例子`，默认例子重放生成 `100` 条输入队列。

## 交接

这轮改动位于 ignored artifact 目录：

- `artifacts/mlse-diff-showcase/index.html`
- `artifacts/mlse-diff-showcase/motus-mods.js`
- `artifacts/mlse-diff-showcase/motus-demo-repos/`
- `artifacts/mlse-diff-showcase/motus-prepare/`

如果后续要把 showcase 固化进仓库，需要先把它从 `artifacts/` 移到受版本控制的文档或 demo 目录，再补对应测试入口。
