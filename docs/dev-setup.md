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
- `cmd/mlse-diff`：从目标 Git 仓库两个 commit 的 Go 函数级 diff 生成 symbolic-diff case，并复用现有 Go/KLEE probe
- `scripts/mlse-diff-smoke.py`：准备函数级 symbolic diff fixtures，并可在安装了 KLEE 的环境里跑最小 KLEE 工具链 smoke
- `scripts/docker-symbolic-diff-build.sh`：构建 symbolic diff / KLEE 开发镜像
- `scripts/docker-symbolic-diff-run.sh`：以当前仓库挂载方式进入 symbolic diff / KLEE 开发镜像

### 构建

```bash
scripts/build.sh
```

构建产物默认输出到：

```text
artifacts/bin/mlse-go
artifacts/bin/mlse-go-ssa-dump
artifacts/bin/mlse-debug
artifacts/bin/mlse-diff
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
- `cmd/mlse-diff` 是否已由 `scripts/build.sh` 产出
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

### 运行 Go formal debug 页面

```bash
go run ./cmd/mlse-debug ./examples/go/simple_add.go
```

或使用构建后的二进制：

```bash
./artifacts/bin/mlse-debug ./examples/go/simple_add.go
```

默认监听 `127.0.0.1:8080`，页面左侧展示 Go 源码，右侧展示 formal MLIR 指令，并依据 `loc(...)` 元数据做行级联动。下方面板会展示来自 `go.scope_table` 的 scope 信息。`-addr 127.0.0.1:0` 可以让系统选择临时端口，`-open` 可以尝试自动打开系统浏览器。

如果已经跑过 symbolic-diff Go pipeline probe，也可以载入它的 `summary.json`：

```bash
go run ./cmd/mlse-debug \
  -trace artifacts/symbolic-diff-go-pipeline-probe/summary.json \
  ./examples/go/simple_add.go
```

当前 trace 面板支持读取 probe summary 中的 path、old/new frame、stage timeline、blocker 和 counterexample 文件列表。它是 MLSE 自己的调试视图格式入口，不直接暴露 KLEE 内部状态；后续更完整的 symbolic trace 可以继续收敛到同一个 JSON 形状。

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

## 函数级 symbolic diff 早期环境

仓库现在为后续“代码更改后函数是否等价”的 KLEE vertical slice 先放了最小测试与容器入口。

测试样例位于：

```text
test/SymbolicDiff/cases/
```

当前已有这些函数级样例：

- `scalar-add-commutative`：`x + 1` vs `1 + x`，期望等价
- `scalar-add-shift`：`x + 1` vs `x + 2`，期望 KLEE 找到反例
- `motus-mod3-slice-append`：从 Motus `smtcmp/testdata/mod3` 抽出的 `append(res, pl[0])` slice 等价样例，当前 KLEE harness 固定输入 slice 长度为 `1`，比较返回 slice 的长度和元素值
- `motus-mod4` 到 `motus-mod39`：从 Motus `pkg/analysis/ssa/smtcmp/testdata/` 批量抽出的函数级 old/new fixture；其中 `mod4`、`mod5`、`mod6`、`mod7`、`mod9` 当前会进入最小 `[]int` KLEE harness，`mod10` 会进入带 nil / empty / full 输入和 strict 返回比较的 `[]int` KLEE harness，`mod8`、`mod11`、`mod12`、`mod14`、`mod16`、`mod17`、`mod18`、`mod20`、`mod21`、`mod25`、`mod26`、`mod27`、`mod28`、`mod29`、`mod30`、`mod34`、`mod35`、`mod38`、`mod39` 会进入 Go LLVM ABI harness，其余 case 作为 expected blocker 进入 readiness matrix

已通过完整 Go -> LLVM -> KLEE 路径、需要在 CI 中保持稳定的 Motus case 会显式登记在：

```text
test/SymbolicDiff/supported-klee-mod-cases.txt
```

本机没有 KLEE 时，也可以先检查 fixture 和 artifact 布局：

```bash
python3 scripts/mlse-diff-smoke.py
```

这会生成：

```text
artifacts/symbolic-diff-smoke/summary.json
artifacts/symbolic-diff-smoke/<case>/old.go
artifacts/symbolic-diff-smoke/<case>/new.go
artifacts/symbolic-diff-smoke/<case>/case.json
```

如果当前环境里有 `clang` 和 `klee`，可以额外跑 KLEE 工具链 smoke：

```bash
python3 scripts/mlse-diff-smoke.py --run-klee-toolchain-smoke
```

这条 smoke 会根据 `case.json` 里的 `c_model` 生成一个极小 C harness，编译成 LLVM bitcode，再交给 KLEE；mismatch 路径通过 `klee_report_error(..., "assert.err")` 产生反例文件。带 `--run-klee-toolchain-smoke` 时，如果结果是 `inconclusive`，脚本会返回失败。没有 `c_model` 的 case 会跳过这层 C-only smoke。它的作用是验证 KLEE / LLVM bitcode 工具链，不代表 Go/MLSE 的完整 symbolic diff 已经完成。

如果要确认真正 Go 链路当前卡在哪，可以运行：

```bash
python3 scripts/mlse-diff-go-pipeline-probe.py
```

这条探针会把 `old.go` / `new.go` 分别跑到 `mlse-go -> mlse-opt -> mlse-opt --lower-go-bootstrap -> mlir-opt -> mlir-translate -> llvm-as`，并在 `artifacts/symbolic-diff-go-pipeline-probe/summary.json` 里记录第一个阻塞点。不带 `--run-klee` 时，它只验证 old/new 两侧 Go 函数已经能产出 bitcode。

如果输入来自另一个 Git 仓库的两个 commit，可以先用 `cmd/mlse-diff` 生成同一套 case 布局并自动调用上面的 probe：

```bash
go run ./cmd/mlse-diff \
  -file pkg/calc.go \
  -function F \
  ../target-repo old-commit new-commit
```

这个命令假设 diff 是单个 Go 函数级入口：函数内部修改会直接比较同名函数；“一个函数拆成多个 helper”或反向合并时，只要入口函数签名不变，命令会保留入口文件和同 package 其它变更 Go 文件里的 helper，并比较入口函数。如果入口函数被改名，可以显式传入 `-old-function` 和 `-new-function`，命令会在两侧生成同签名 wrapper，让现有 same-input harness 比较同一个符号。当前自动 KLEE model 覆盖 `int` / `int64` 标量参数返回值、一个 `[]int -> []int` 的 `slice_i64` model，以及基于 Go LLVM ABI 的 `string` / `[]string` / `void` / `error` / `(bool,error)` 等 repo-owned 子集；其它签名会生成 `klee_model_unavailable` blocker，而不是伪装成已证明等价。

如果想先用 concrete 输入快速探索 old/new 是否出现可观测差异，可以运行：

```bash
python3 scripts/mlse-diff-fuzz-smoke.py
```

这条入口会把每个 case 的 `old.go` / `new.go` 改写成两个临时 Go package，生成 same-input Go test harness，并逐 seed 运行 `go test -coverprofile`。每个 seed 都会比较 old/new 的返回值、panic、以及可观测入参变更；覆盖率有提升的 seed 会记录到 `selected_inputs`。输出状态包括：

- `fuzz-counterexample`：某个 concrete seed 已经触发 old/new 可观测差异
- `fuzz-no-diff-found`：当前 seed 集合没有发现差异；这不是等价证明
- `skipped` / `blocked`：当前 fixture 缺外部包、签名不可从外部 harness 调用，或 harness 编译失败

CI 里只跑一组稳定快路径，用来验证这层 concrete fuzz 链路能发现已知反例，并能给部分等价样例产出 coverage 报告。全量 case 可以本地按需扩大 `--iterations`；结果应作为 fuzz 信号，不应替代 KLEE / bounded exhaustive 的 `equivalent` 结论。

在带 KLEE 的容器内可以运行完整标量 smoke：

```bash
python3 scripts/mlse-diff-go-pipeline-probe.py --run-klee --expect-status ok
```

CI 会在这条命令上额外传入 `--require-case-list test/SymbolicDiff/supported-klee-mod-cases.txt`。这会继续为当前 repo-owned 样例生成 same-input KLEE harness，重命名并链接 old/new bitcode，再检查等价样例无 KLEE `.err` 文件、非等价样例能通过 `assert.err` 产生 counterexample；同时清单中的已通过 Motus case 必须完成 KLEE 且满足各自 `expected_status`，不能退回 `expected_blocker` 或其它 blocker。当前 KLEE harness 有两类 Go ABI 支持：`slice_i64` 覆盖 `[]int` 的最小 ABI，既能做逻辑 slice 比较，也能用 nil / empty / full 输入和 strict 返回比较锁住 nil slice 与非 nil 空 slice 的差异；`go_llvm` 覆盖 bounded symbolic `string`、`[]string`、`void` 入口、简单全局 `int`、简单 struct pointer、pointer slice、request struct 指针、常量 `error` 返回、`string,bool` / `(bool,error)` 多返回、简单 `ptr_i64` 首字段返回比较，以及测试所需的 `runtime.add.string`、`runtime.any.box.*`、`runtime.fmt.Sprintf` 的 `%s` / `%d` / `%.1fK` equality-preserving 子集、`runtime.errors.New`、无格式参数的 `runtime.fmt.Errorf`、`runtime.makeslice`、`runtime.growslice`、`runtime.newobject`、`runtime.make.map` 和少量 map/string/helper stub。`mod12` / `mod29` / `mod30` 这类新增 case 使用固定外部数据源或固定 helper 结果来覆盖当前 fixture 的 rewrite 行为，不代表完整 set/map/DB/binding 语义。带格式参数的 `fmt.Errorf`、真实 map 语义、复杂对象图和外部 client 仍然保持 unsupported / inconclusive。

Motus 批量样例里暂未进入 KLEE 的 case 会在 `case.json` 中声明 `expected_blocker`。CI 会继续跑到真实阻塞点，并要求它和声明一致；这样可以把当前缺口固定下来，而不是把 unsupported 语义伪装成已证明等价。当前主要 blocker 是：

- `klee_model_unavailable`：Go/MLIR/LLVM bitcode 已可达，但还缺对应参数 / 返回值 / runtime 的 KLEE model
- `old_mlse_opt_roundtrip_failed`：frontend 产物尚不能通过 `mlse-opt` round-trip；普通 `any` 参数 boxing 和用户自定义 `...any` 打包已经有回归测试，剩余这类 blocker 应视为新的 frontend 边界

### Docker 环境

构建 symbolic diff 开发镜像：

```bash
scripts/docker-symbolic-diff-build.sh
```

进入镜像：

```bash
scripts/docker-symbolic-diff-run.sh
```

在镜像内可以运行：

```bash
scripts/build.sh
scripts/build-mlir.sh
python3 scripts/mlse-diff-smoke.py --run-klee-toolchain-smoke
```

镜像默认使用双 LLVM 工具链：MLSE / MLIR 构建使用 LLVM/MLIR `20`，KLEE 构建和 KLEE bitcode smoke 使用 LLVM `16`。这个选择是为了同时满足仓库当前 Go dialect 依赖的 MLIR C++ API、后续 `mlir-go` 绑定目标，以及 KLEE 当前更稳定的 LLVM 支持范围。如果后续要改 LLVM 版本，需要同时确认 MLSE 的 MLIR C++ API、`mlir-go` 绑定和 KLEE 支持范围。

### GitHub Actions

仓库现在新增了：

```text
.github/workflows/symbolic-diff.yml
```

这条 workflow 会在 GitHub runner 上跑三层检查：

- `go-smoke`：`go test ./cmd/... ./internal/...`、Python 脚本语法检查、`python3 scripts/mlse-diff-smoke.py`，以及一组 concrete fuzz diff 快路径
- `dockerfile-check`：`docker build --check -f docker/Dockerfile.symbolic-diff .`
- `docker-symbolic-diff`：用 GitHub Actions cache 构建 `docker/Dockerfile.symbolic-diff`，再在容器内运行 `scripts/build.sh`、`scripts/build-mlir.sh`、`python3 scripts/mlse-diff-smoke.py --run-klee-toolchain-smoke` 和 `python3 scripts/mlse-diff-go-pipeline-probe.py --run-klee --expect-status ok`

也就是说，后续 KLEE / LLVM / MLIR 的完整环境验证会交给 GitHub CI 资源运行；本机开发只需要在必要时手动构建镜像。

## 目录约定

当前仓库把实验和临时文件约束在仓库目录内，并区分**瞬时目录**与**可提交目录**：

- `artifacts/`：构建与实验输出，属于瞬时目录，默认不入库
- `tmp/`：临时工作目录和缓存，属于瞬时目录，默认不入库
- `docker/`：容器相关定义
- `test/SymbolicDiff/`：函数级 symbolic diff 的 repo-owned fixture
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
