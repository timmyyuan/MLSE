# Go Frontend MVP

这是 MLSE 在空仓库状态下落下的第一个 Go 前端原型。

目标不是“真的生成可被 mlir-opt 消费的完整 MLIR”，而是先验证一条最小链路：

```text
极小 Go 输入 -> Go AST -> typed-ish frontend skeleton -> MLIR-like 文本
```

当前提供的工具是 `cmd/mlse-go`。

## 当前支持的最小子集

- 单文件 Go 源码
- package 级函数
- 参数和返回值中的 `int`
- 条件位置和参数位置中的 `bool`
- 函数体中的：
  - `var` 零值初始化和 `:=` 定义单个局部变量
  - 标识符赋值
  - `+ / - / * / /` 和整数比较二元表达式
  - `if / else`
  - 基本 `for`
  - 表达式 `switch` 的 `case/default`
  - `return`
  - 标识符 / 整数字面量

## 当前输出

输出为 **MLIR-like 文本**，风格上靠近 `func` + `arith`，但目前仍是手写打印原型。

示例：

```mlir
module {
  func.func @add(%a: i32, %b: i32) -> i32 {
    %c = arith.addi %a, %b : i32
    return %c : i32
  }
}
```

## 临时 stub

以下内容是刻意保留的 stub，而不是遗漏：

- **类型系统**：当前仅把 Go `int` 直接映射成 `i32`
- **语义分析**：主要依赖 AST 结构，不做完整 `go/types` / SSA 分析
- **控制流**：`if / for / switch` 目前只是打印成实验性 `mlse.*` 文本；真正能否继续 lower，取决于 `docs/goir-llvm-experiment.md` 里当前验证过的子集
- **调用与包解析**：只保留最小 `mlse.call` 占位文本，还没有导入解析、跨文件 package 和稳定调用约束
- **真实 MLIR 集成**：当前只是输出清晰的 MLIR-like 文本，没有链接 MLIR C++/Go 绑定

## 未来扩展点

1. 接 `go/types`，把名字解析和类型检查做实。
2. 接 SSA，为后续结构化 lowering 和控制流打基础。
3. 把现在的实验性 `if / for / switch` 文本收敛成明确 contract，再扩到 `call / struct / slice`。
4. 把当前 emitter 抽成真正的 frontend IR + printer。
5. 后续再把文本原型替换成真正的 MLIR builder 或稳定中间层。

## 运行

```bash
go run ./cmd/mlse-go -- examples/go/simple_add.go
```
