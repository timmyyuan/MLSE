# Development Setup

MLSE 当前还处于早期原型阶段。

这份文档只描述**当前仓库已经存在并支持的开发方式**，不提前承诺尚未落地的 LLVM/MLIR/CIR 集成细节。

## 当前依赖

- Go `1.25+`
- Docker（用于 tinygo 基础镜像与实验）
- LLVM/MLIR 开发安装（可选，用于构建 `mlse-opt` 和 `mlse-run`）

## 当前可用命令

统一入口位于 `scripts/`：

- `scripts/build.sh`：构建当前 Go MVP 工具
- `scripts/build-mlir.sh`：配置并构建 `mlse-opt` / `mlse-run` 的最小 MLIR/LLVM 工程链路
- `scripts/test.sh`：运行仓库主线 Go 测试
- `scripts/test-all.sh`：运行仓库当前统一测试入口，覆盖 Go、linters 和 repo-owned MLIR bridge 样例
- `scripts/fmt.sh`：格式化仓库 Go 代码
- `scripts/lint.sh`：运行仓库当前的 Go/C++/Python 规范检查
- `scripts/clean.sh`：清理仓库内临时产物和 tinygo 实验目录
- `scripts/go-gobench-mlir-suite.py`：对外部 `../gobench-eq` Go 样本跑 formal MLIR / `mlse-opt` round-trip / `mlse-opt --lower-go-bootstrap` / LLVM probe
- `scripts/go-exec-diff-suite.py`：对 repo-owned `test/GoExec/cases/` 样例跑 native Go vs `mlse-run` 的 stdout / stderr 差分

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

默认测试现在只覆盖仓库主线 Go 包：

- `./cmd/...`
- `./internal/...`

这样可以避免把 `tmp/` 下的 TinyGo 安装树、probe 目标和其它实验缓存错误地纳入 `go test ./...`。

如果要跑仓库当前“能稳定自动化”的完整测试面，可以运行：

```bash
scripts/test-all.sh
```

这条入口当前会顺序执行：

- `scripts/test.sh`
- `go test ./linters`
- `scripts/build.sh`
- `scripts/build-mlir.sh`
- `mlse-opt` 解析 `test/GoIR/ir/*.mlir`
- `mlse-opt --lower-go-builtins` 对 `test/GoIR/ir/bootstrap_ops.mlir` 的 builtin lowering 检查
- `mlse-opt --lower-go-bootstrap` 对 `test/GoIR/ir/bootstrap_ops.mlir` 的 bootstrap lowering 检查
- `cmd/mlse-go` 生成并桥接验证 `examples/go/simple_add.go`、`sign_if.go`、`choose_merge.go`、`sum_for.go`
- `scripts/go-exec-diff-suite.py --skip-build`，验证 `test/GoExec/cases/goexec-spec-*` 的 native Go vs `mlse-run` 差分

其中 `mlir-fixtures` 和 `frontend-bridge` 两段会逐个打印正在跑的文件，以及对应的 `PASS` / `FAIL` 状态。

### Lint

```bash
scripts/lint.sh
```

当前这条入口会转发到仓库内的 `linters/` 目录，并按语言分别运行：

- Go：`gofmt -l`、`go vet ./cmd/... ./internal/...`，以及参数个数 / 函数长度 / 文件长度阈值检查；另外还会检查“单次调用的纯转发 wrapper + 单次调用 callee”这类可直接内联的 helper 链
- C++：参数个数 / 函数长度 / 文件长度阈值检查
- Python：`py_compile`，以及参数个数 / 函数长度 / 文件长度阈值检查

默认阈值是：

- 参数个数不超过 `5`
- 函数长度不超过 `200` 行
- 文件长度不超过 `2000` 行

如果要临时收紧或放宽阈值，可以在运行前覆盖环境变量，例如：

```bash
MAX_PARAMS=4 MAX_FUNCTION_LINES=150 scripts/lint.sh
```

其中 `internal/gofrontend` 当前已经把 frontend 输出检查改成 file-driven `FileCheck` fixtures：

- fixture 放在 `internal/gofrontend/testdata/`
- `go test ./internal/gofrontend` 会逐个编译这些 `.go` 样例，再用 `FileCheck` 校验 formal 输出
- 如果本机没有 `FileCheck`，这组检查会被显式 skip，而不是把整个 Go 测试链路直接打死

同一个包现在还补了一层 `GoIR/formal MLIR -> LLVM IR` 的 file-driven `FileCheck` fixture：

- fixture 放在 `internal/gofrontend/testdata/llvm/`
- `go test ./internal/gofrontend` 会把这些样例跑过 `mlse-opt -> mlse-opt --lower-go-bootstrap -> mlir-opt -> mlir-translate`
- 只有经过 `--lower-go-bootstrap` 后已经不含 unresolved `go` dialect 语法的样例，才允许继续检查最终 LLVM IR
- 这层测试依赖 `mlse-opt`、`mlir-opt`、`mlir-translate` 和 `FileCheck`；缺工具时会 skip

### 构建最小 MLIR 工程

```bash
scripts/build-mlir.sh
```

当前脚本默认面向本机已验证过的一套 Homebrew LLVM/MLIR 安装：

- `LLVM_PREFIX=/opt/homebrew/Cellar/llvm@20/20.1.8`
- `MLIR_DIR=$LLVM_PREFIX/lib/cmake/mlir`
- `LLVM_DIR=$LLVM_PREFIX/lib/cmake/llvm`
- `CMAKE_C_COMPILER=$LLVM_PREFIX/bin/clang`
- `CMAKE_CXX_COMPILER=$LLVM_PREFIX/bin/clang++`

如果你的 LLVM 安装路径不同，可以覆盖这些环境变量，例如：

```bash
LLVM_PREFIX=$(brew --prefix llvm)
scripts/build-mlir.sh
```

成功后产物默认位于：

```text
tmp/cmake-mlir-build/tools/mlse-opt/mlse-opt
tmp/cmake-mlir-build/tools/mlse-run/mlse-run
```

当前 `mlse-run` 的 MVP 只支持 LLVM-dialect MLIR 输入，也就是更接近 `05-llvm-dialect.mlir` 这一层；`.ll` / LLVM IR importer 还没有在当前仓库里落地。

### 运行 Go 前端 MVP

```bash
go run ./cmd/mlse-go ./examples/go/simple_add.go
```

或者运行构建后的二进制：

```bash
./artifacts/bin/mlse-go ./examples/go/simple_add.go
```

当前 `cmd/mlse-go` 只输出正式 `go` dialect bridge 的最小 parseable 子集。

### 运行 gobench-eq MLIR suite

如果本机存在相邻 checkout `../gobench-eq`，可以运行：

```bash
python3 scripts/go-gobench-mlir-suite.py
```

当前默认会扫描 `../gobench-eq/dataset/cases/goeq-spec-*` 下的非 test Go 文件，并分阶段记录：

- `cmd/mlse-go` 是否能产出 formal MLIR
- `mlse-opt` 是否能解析并 round-trip 这份 MLIR
- `mlse-opt --lower-go-bootstrap` 是否能把这份 MLIR lower 成不含 unresolved `go` 语法的 LLVM-legal MLIR
- 对经过这一步的子集，是否能直接从 `MLIR` 继续 lower 到 LLVM dialect、翻译成 LLVM IR，并通过 `opt -Oz`

默认中间结果会保存在：

```text
artifacts/go-gobench-mlir-suite/
tmp/go-gobench-mlir-suite/<artifact-dir-name>-<hash>/
```

这些目录都位于仓库内，且不会进入 git 提交物。
这条 suite 现在按覆盖式落盘工作：每次运行都会先清空当前 `artifact-dir`，并重建一个按 `artifact-dir` 名字和稳定 hash 派生的 scratch 目录；因此即使不同输出目录只是在标点或路径分隔上看起来相近，也不会再误落到同一个临时树。同时脚本会对 `artifact-dir` 使用内核级文件锁，同一个输出目录如果已经有 run 在执行，会直接报错而不是互删产物，也不会再因为旧 pid 文件或 pid 复用产生误判。
如果显式传入 `--case-glob`，当前也只会跑你指定的 glob；不再隐式追加默认的 `goeq-spec-*` 全量扫描。

当前 CLI 保留的是最常用的一层：

- `--case-glob`
- `--limit`
- `--jobs`
- `--skip-build`
- `--artifact-dir`
- `--dataset-root`
- `--mlse-go-bin`
- `--mlse-opt-bin`

其余仓库根、scratch 路径和 LLVM 工具路径都改成脚本内部自动发现，不再暴露成顶层参数。

每个源文件对应的 artifact 目录现在会按阶段编号保存关键产物；同时每个 case 会共享一份源码树：

```text
artifacts/go-gobench-mlir-suite/cases/<case>/00-case-source/

files/<case>/<variant>/<file>.go/
00-source__<name>.go
01-ssa__main.Target.txt
02-formal.mlir
03-roundtrip.mlir
04-go-bootstrap-lowered.mlir
05-llvm-dialect.mlir
06-module.ll
07-module.Oz.ll
```

配套 stdout/stderr 也会带同样的阶段编号，例如 `02-frontend.stdout`、`03-mlse-opt.stderr`、`04-go-lower.stdout`、`07-opt.stderr`。其中共享的 `cases/<case>/00-case-source/` 会把当前 case 的源码树整体搬进 artifact 目录，`01-ssa__main.Target.txt` 只打印 `main.Target` 的 `x/tools/go/ssa` 文本；如果当前文件里没有这个函数，就会写一条显式说明。

如果只看这些编号还不容易建立直觉，可以直接看 [goir-dialect.md](goir-dialect.md) 里的“`00 -> 07：一个 Go 函数的生命周期`”小节。那一节用真实 gobench 样例把源码、SSA、formal MLIR、round-trip、bootstrap lowering、LLVM dialect、LLVM IR 和最终 `opt -Oz` 校验串成了一条完整叙事链。

需要注意，这套 suite 当前验证的是**工程可达性**，不是语义等价证明；当前仓库也还没有完整的 `Go -> MLIR -> LLVM IR` 语义收敛。

如果你只想单独查看当前 `go.len` / `go.cap` / string `go.index` / `go.append` 的 lowering，可以运行：

```bash
tmp/cmake-mlir-build/tools/mlse-opt/mlse-opt --lower-go-builtins test/GoIR/ir/bootstrap_ops.mlir
```

如果你要看完整 bootstrap lowering，可以运行：

```bash
tmp/cmake-mlir-build/tools/mlse-opt/mlse-opt --lower-go-bootstrap test/GoIR/ir/bootstrap_ops.mlir
```

如果你要直接执行一份已经 lower 到 LLVM dialect 的 MLIR，可以运行：

```bash
tmp/cmake-mlir-build/tools/mlse-run/mlse-run path/to/05-llvm-dialect.mlir
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

## 现阶段边界

当前仓库已经有：

- 一个最小 `Go -> formal go dialect` 桥接原型
- 基础脚本入口
- 最小 golden test 骨架

当前仓库**还没有**：

- 完整的 frontend / pass / lowering 管线
- 与 CI 集成的稳定 LLVM/MLIR toolchain provisioning
- Docker 化完整开发环境
- 统一 CI 流程

后续一旦这些能力落地，需要同步更新本文件。
