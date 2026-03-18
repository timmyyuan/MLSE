# Agent Guide

本仓库的代码实现与设计文档需要一起看。

在开始写代码前，先阅读两类内容：

1. 仓库内文档
2. Obsidian 中的 `mlse设计`

## 1. 必读顺序

建议按下面顺序建立上下文：

1. `README.md`
2. `docs/spec.md`
3. Obsidian vault `next` 中的 `mlse设计/00-MLSE设计文档.md`
4. 本次修改直接相关的语言文档：
   - `mlse设计/01-PythonIR/`
   - `mlse设计/02-GoIR/`

## 2. 如何定位 Obsidian 文档

不要在仓库文档里写绝对路径。

如果当前环境里不知道 Obsidian vault 的具体位置，先从用户主目录搜索：

```bash
find ~ -path '*/next/mlse设计/00-MLSE设计文档.md' 2>/dev/null | head -n 1
```

找到总入口后，再进入对应语言目录继续阅读。

## 3. 修改代码时的要求

- 如果改动涉及架构、前端边界、方言、pass、运行时或测试策略，先看 Obsidian 设计文档再动手。
- 如果实现与设计不一致，先判断是设计落后于实现，还是实现偏离设计。
- 如果改动改变了既有设计约束，代码完成后要同步更新 Obsidian 中的 `mlse设计`。

## 4. 文档同步要求

以下变更默认需要同步文档：

- 新增或删除前端阶段
- 新增或删除 dialect / op / pass
- 修改 `PythonIR` 或 `GoIR` 的支持边界
- 修改 Docker、test、lint、clean 等工程约束
- 修改默认命令接口、输出产物或目录约定

## 5. 风格约束

- 不要在仓库文件中写绝对路径。
- 文档中引用 Obsidian 内容时，使用 vault 名和相对逻辑路径。
- 如果需要告诉后续 agent 如何找到文档，优先提供搜索命令，而不是硬编码本机路径。
