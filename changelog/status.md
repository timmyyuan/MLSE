# 当前工作状态

> 单 agent 工作锁：只有当 `工作状态` 为 `空闲` 时，新 agent 才能开始工作。

| 字段 | 值 |
| --- | --- |
| 工作状态 | 空闲 |
| 当前 agent | - |
| 工作范围 | - |
| 关联记录 | [2026-05-19-mlse-diff-simple-klee-models-0856.md](entries/2026-05-19-mlse-diff-simple-klee-models-0856.md) |
| 开始时间 | 2026-05-19 08:56 CST |
| 最后更新 | 2026-05-19 |
| 交接备注 | 已补 simple KLEE model 与文档；本地验证通过，本机未安装 KLEE，真实 KLEE 探索交给 symbolic-diff CI 容器。 |

## 状态说明

- `空闲`：没有 agent 持有工作锁。
- `正在执行`：已有 agent 正在修改、分析或实现，其它 agent 不应开始并行工作。
- `验证中`：已有 agent 正在跑检查或核对结果，其它 agent 不应开始并行工作。
- `等待用户`：已有 agent 暂停等待用户确认，工作锁仍然有效。
