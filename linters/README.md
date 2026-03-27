# Linters

这个目录集中管理仓库的代码规范检查入口。

当前默认阈值：

- 函数参数个数不超过 `5`
- 函数长度不超过 `200` 行
- 单文件长度不超过 `2000` 行

统一入口：

```bash
linters/run-all.sh
```

按语言单独运行：

```bash
linters/run-go.sh
linters/run-cpp.sh
linters/run-python.sh
```

默认阈值定义在 [limits.env](limits.env)。

运行前可以通过环境变量覆盖，例如：

```bash
MAX_PARAMS=4 MAX_FUNCTION_LINES=150 linters/run-all.sh
```

当前各入口的行为：

- Go：`gofmt -l`、`go vet ./cmd/... ./internal/...`，再检查 Go 文件的参数个数、函数长度、文件长度
- C++：检查 `include/`、`lib/`、`tools/` 下 C/C++ 文件的参数个数、函数长度、文件长度
- Python：先做 `py_compile`，再检查 `scripts/` 和 `linters/` 下 Python 文件的参数个数、函数长度、文件长度

当前 C++ 函数识别使用的是仓库内自带的轻量脚本，适合作为工程阈值守卫；后续如果接入 `clang-tidy` 或编译数据库，可以再把这里替换成更强的 AST 级检查。
