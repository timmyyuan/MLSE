# 修复 mlse-diff method receiver 与 variadic helper 边界

| 字段 | 值 |
| --- | --- |
| 日期 | 2026-05-18 |
| 状态 | 已完成 |
| 范围 | internal/symbolicdiff、cmd/mlse-diff 文档 |
| 关联请求 | 从底层解决 Motus showcase 中 `blocked` 的 method diff 与 variadic helper 问题 |

## 摘要

本轮修复 `mlse-diff prepare` 的两个底层边界：

- 支持入口是 Go method 的函数级 diff。
- 允许文件内存在非入口 variadic helper。

修复后，Motus showcase 里原先 `blocked` 的 `mod8`、`mod20`、`mod22`、`mod32`、`mod33`、`mod37` 都能生成 case。它们当前仍显示为 `unsupported`，含义是 case 已生成，但 KLEE 输入/输出 model 尚未覆盖这些复杂签名。

## 实现

method entry 现在会生成普通函数 wrapper：

```go
func MLSEDiffEntry(receiver T, arg A) R {
	return receiver.Method(arg)
}
```

也就是说 receiver 作为 wrapper 的第一个显式参数进入 same-input harness。value receiver 和 pointer receiver 都保留原始 Go 类型。

variadic 的处理边界调整为：

- 入口函数本身是 variadic：仍拒绝，避免生成当前 harness 无法表达的调用。
- 非入口 helper 是 variadic：允许解析并原样写入生成的 old/new 源码。

## 测试

- 新增 method entry wrapper 单元测试。
- 新增非入口 variadic helper 单元测试。
- 新增 variadic entry 明确拒绝单元测试。
- 重新生成本地 Motus showcase 数据，`mod1..mod39` 全部 `prepareStatus=ok`。

验证命令：

```bash
go test ./internal/symbolicdiff
go test ./cmd/... ./internal/...
staticcheck ./cmd/... ./internal/...
scripts/lint.sh
python3 scripts/mlse-diff-smoke.py
```

## 剩余边界

这轮只解决 prepare 阶段的底层 blocker。复杂样例要从 `unsupported` 变成可证明，还需要继续扩展 KLEE model / frontend 支持，例如 context、指针、map、error、多返回值、方法 receiver 对应的可执行输入模型。
