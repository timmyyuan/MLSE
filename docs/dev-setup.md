# Development Setup

MLSE 当前还处于早期原型阶段。

这份文档只描述**当前仓库已经存在并支持的开发方式**，不提前承诺尚未落地的 LLVM/MLIR/CIR 集成细节。

## 当前依赖

- Go `1.25+`
- Docker（用于 tinygo 基础镜像与实验）
- Clang（可选，用于实验性 LLVM IR 编译检查）
- `opt` 或 `llvm-as`（可选但更强，用于实验性 LLVM IR verifier / assembler 检查）
- `mlir-opt` / `mlir-translate`（可选，目前只做本地可用性探测）

## 当前可用命令

统一入口位于 `scripts/`：

- `scripts/build.sh`：构建当前 Go MVP 工具
- `scripts/test.sh`：运行仓库 Go 测试
- `scripts/fmt.sh`：格式化仓库 Go 代码
- `scripts/lint.sh`：运行 `go vet`
- `scripts/clean.sh`：清理仓库内临时产物和 tinygo 实验目录
- `scripts/goir-llvm-experiment.sh`：运行实验性 `GoIR -> LLVM IR` 样本验证

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

当前仓库把实验和临时文件约束在仓库目录内，并区分**瞬时目录**与**可提交目录**：

- `artifacts/`：构建与实验输出，属于瞬时目录，默认不入库
- `tmp/`：临时工作目录和缓存，属于瞬时目录，默认不入库
- `docker/`：容器相关定义
- `testdata/`：golden / fixture / 可持久化保存的实验样本

不要把临时实验文件散落到仓库外部。

`scripts/clean.sh` 会直接清空仓库内的 `artifacts/` 和 `tmp/`，因此需要长期保留、便于 code review 或后续比对的实验产物，必须放到 `testdata/` 或其它非瞬时目录。

## tinygo 实验

tinygo 相关实验约定写入：

- `docker/`：镜像定义
- `scripts/`：运行脚本
- `tmp/`：clone、workdir、中间产物
- `artifacts/tinygo/`：统计结果与导出产物

当前 tinygo probe 还有两条额外约定：

- 如果要对固定 etcd ref 运行 probe，优先通过 `ETCD_SOURCE_REPO=<local-git-clone> ETCD_REF=<ref>` 驱动脚本。脚本会把该 ref 通过 `git archive` 导出到仓库内 `tmp/etcd-targets/`，并在导出目录写 `.mlse-target.json`，避免直接修改外部 checkout。
- probe 运行时会把 `HOME`、Go build cache 和 module cache 收到仓库内 `tmp/`，避免 tinygo / go 默认把缓存写到用户目录。

### TinyGo Package Probe

```bash
scripts/tinygo-probe-packages.sh
```

这条 probe 现在会：

- 遍历 etcd 树下所有 `go.mod`
- 对 `main` 包直接运行 `tinygo build`
- 对库包自动生成 blank-import stub，再让 tinygo 通过合法入口编译该包

这比“只扫根 module、只直接编 import path”更接近 tinygo 当前 CLI 的真实能力边界。

### TinyGo File Probe

```bash
scripts/tinygo-probe-local.sh
```

这条 probe 现在采用**package-context file attribution**：

- 不再把单个 `.go` 文件直接当可执行入口编译
- 先按 package context 编译，再把结果归因到该 package 的非 test 源文件
- `TestGoFiles` / `XTestGoFiles` 单独记为 omitted，而不是混进失败统计

因此它比旧的“逐文件直接 build”更适合区分 tinygo 真正的语义失败和方法学噪声。

## GoIR 到 LLVM IR 实验

实验性路径约定写入：

- `cmd/mlse-goir-llvm-exp/`：最小 GoIR 子集翻译器
- `scripts/goir-llvm-experiment.sh`：端到端实验入口
- `testdata/goir-llvm-exp/`：稳定实验夹具与持久化成功样本
- `testdata/goir-llvm-exp/successes/`：每个成功样本一目录，保存 `goir.mlir`、`llvm.ll` 和索引
- `artifacts/goir-llvm-exp/`：本地实验日志、校验输出与当次运行报告

当前脚本会自动探测本地 `opt`、`llvm-as`、`mlir-opt`、`mlir-translate`、`clang`。

验证优先级如下：

1. `opt` verifier
2. `llvm-as`
3. 无 dedicated verifier 时继续记录 `clang` compile check

报告会把三类状态分开写出：

- translation success / failure
- verifier success / failure / unavailable
- compile success / failure / unavailable

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

当前仓库新增了一条**实验性** `GoIR -> LLVM IR` 文本导出路径，但它只覆盖极小子集，不能视为正式后端能力。

这条实验路径当前直接生成 LLVM IR 文本，而不是 LLVM dialect MLIR；因此 `mlir-opt` / `mlir-translate` 目前只在报告里体现可用性，不在主动验证链路上。

后续一旦这些能力落地，需要同步更新本文件。
