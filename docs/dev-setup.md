# Development Setup

MLSE 当前还处于早期原型阶段。

这份文档只描述**当前仓库已经存在并支持的开发方式**，不提前承诺尚未落地的 LLVM/MLIR/CIR 集成细节。

## 当前依赖

- Go `1.25+`
- Docker（用于 tinygo 基础镜像与实验）

## 当前可用命令

统一入口位于 `scripts/`：

- `scripts/build.sh`：构建当前 Go MVP 工具
- `scripts/test.sh`：运行仓库 Go 测试
- `scripts/fmt.sh`：格式化仓库 Go 代码
- `scripts/lint.sh`：运行 `go vet`
- `scripts/clean.sh`：清理仓库内临时产物和 tinygo 实验目录

### 构建

```bash
scripts/build.sh
```

构建产物默认输出到：

```text
artifacts/bin/mlse-go
```

### 测试

```bash
scripts/test.sh
```

### 运行 Go 前端 MVP

```bash
go run ./cmd/mlse-go ./examples/go/simple_add.go
```

或者运行构建后的二进制：

```bash
./artifacts/bin/mlse-go ./examples/go/simple_add.go
```

## 目录约定

当前仓库把实验和临时文件约束在仓库目录内：

- `artifacts/`：构建与实验输出
- `tmp/`：临时工作目录
- `docker/`：容器相关定义
- `testdata/`：golden / fixture / 测试输入

不要把临时实验文件散落到仓库外部。

## tinygo 实验

tinygo 相关实验约定写入：

- `docker/`：镜像定义
- `scripts/`：运行脚本
- `tmp/`：clone、workdir、中间产物
- `artifacts/tinygo/`：统计结果与导出产物

## 现阶段边界

当前仓库已经有：

- 一个最小 `Go -> MLIR-like text` 原型
- 基础脚本入口
- 最小 golden test 骨架

当前仓库**还没有**：

- 真正的 MLIR builder
- LLVM/MLIR/CIR 正式依赖接入
- Docker 化完整开发环境
- 统一 CI 流程

后续一旦这些能力落地，需要同步更新本文件。
