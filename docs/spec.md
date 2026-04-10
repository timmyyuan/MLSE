# MLSE Spec

## 文档定位

这是一份面向实现阶段的技术规格说明。

它定义 MLSE 的项目目标、技术边界、编译链路、模块拆分、阶段性里程碑和主要风险，用于指导仓库从空仓库演进为可运行的多语言编译基础设施。

## 1. 项目定义

MLSE 是一个多语言到 MLIR 的编译基础设施项目。

项目的核心目标有两个：

- 将 `C/C++/Python/Go` 程序转换为 `MLIR`。
- 支持将 `MLIR` 继续 lowering 成 `LLVM IR`。
- 提供一个共享 IR 执行层，用来解释执行 MLSE 当前生成的 MLIR / LLVM IR 子集，并服务语义差分验证。

MLSE 不是单一语言编译器，而是一个统一的多语言前端和共享后端框架。它的价值在于把不同语言映射到可分析、可优化、可复用的中间表示，再复用同一条 MLIR 到 LLVM 的后端链路。

## 2. 项目目标

### 2.1 核心目标

- 提供统一的编译驱动，接收多语言源程序并输出 MLIR 或 LLVM IR。
- 为不同语言建立可插拔前端，避免后端逻辑重复实现。
- 优先复用 MLIR 标准 dialect，减少自定义 IR 负担。
- 在高层语义无法统一表示时，引入最小化的 MLSE 自定义 dialect。
- 建立可测试、可扩展、可逐步演进的编译管线。

### 2.2 输出目标

MLSE 第一阶段至少应支持以下输出模式：

- `source -> MLIR`
- `source -> LLVM IR`
- `MLIR -> LLVM IR`

建议同时补一条执行验证路径：

- `MLIR / LLVM IR -> observable program behavior`

### 2.3 成功标准

- 开发者可以通过统一命令对受支持语言子集生成 MLIR。
- 开发者可以对手写或前端生成的 MLIR 稳定输出 LLVM IR。
- 开发者可以对 MLSE 当前生成的 IR 子集执行，并比较原生程序与 `mlse-run` 的 stdout / stderr。
- 每种语言至少有一个清晰定义的“可运行子集”和对应测试集。
- IR 结果可以被 golden test、round-trip test 或 pass-level test 验证。

## 3. 非目标

以下内容不作为第一阶段目标：

- 完整覆盖 `C++` 全语义，包括模板元编程、异常、RTTI、协程等复杂特性。
- 完整覆盖 Python 动态语义，包括 `eval`、猴子补丁、元类、动态导入等。
- 完整覆盖 Go 的并发、反射、`cgo` 和全部工具链细节。
- 直接生成目标文件、可执行文件和完整链接产物。
- 在第一阶段解决跨语言 ABI 兼容、调试信息、增量编译和 IDE 集成。

## 4. 关键设计原则

- 后端先行：先打通 `MLIR -> LLVM IR`，降低整个项目的技术不确定性。
- 前端分层：每种语言单独建 front-end adapter，但共享统一的 lowering contract。
- C/C++ 复用优先：优先集成 `ClangIR/CIR`，而不是从零实现 C/C++ 到 MLIR 的前端。
- 语义保真优先：先保证正确性，再考虑激进优化。
- 最小自定义：能用 MLIR 标准 dialect 表达的，不新增 MLSE dialect。
- 支持分级：允许“高层结构化 MLIR”与“兼容性 MLIR”两种支持等级共存。

## 5. 支持等级模型

为了让项目能持续推进，MLSE 将不同语言的支持拆为两个等级：

### 5.1 Structured MLIR

前端尽可能保留源码中的结构化语义，生成以标准 MLIR dialect 为主的结果，例如：

- `func`
- `arith`
- `scf`
- `cf`
- `memref`
- `tensor`

必要时补少量 MLSE 自定义 dialect。

这个等级更适合后续分析、优化和跨语言统一处理，是长期目标。

对于 `C/C++`，MLSE 不要求第一天就直接生成纯 `func/arith/scf/...` 风格的 canonical MLIR。`ClangIR/CIR` 本身就是建立在 MLIR 之上的高层语言 IR，因此在 MLSE 中应视为合法的前端入口表示。后续再按需要将 `CIR` 逐步 normalize 到 `MLSE canonical IR` 或直接走 `LLVM` lowering。

### 5.2 Compatibility MLIR

当短期内难以保留高层语义时，允许先走兼容路径：

- 源语言先通过现有工具链生成 `LLVM IR`
- 再导入 MLIR 的 `LLVM` dialect

这个等级能快速打通端到端流程，但得到的 MLIR 更低层，分析和优化空间较弱。

对于 `C/C++`，在引入 `ClangIR/CIR` 后，兼容路径的优先级应下降。相比 `LLVM IR -> MLIR LLVM dialect import`，`CIR` 仍保留更多语言语义，更适合作为默认输入。

## 6. 总体技术架构

整体编译链路规划如下：

```text
Source Code
  -> Language Frontend
  -> Frontend Semantic Layer
  -> MLIR Import / Build
  -> Canonical MLIR Pipeline
  -> LLVM Dialect Lowering
  -> LLVM IR Translation
```

更具体的实现上，MLSE 建议分成以下模块：

- `Driver`：负责命令行解析、输入调度、pipeline 选择和产物输出。
- `Frontends`：负责各语言的解析、类型信息、语义建模和前端 lowering。
- `Dialect/IR`：负责 MLSE 自定义 dialect、类型系统补充和属性定义。
- `Passes`：负责 canonicalization、normalization、合法化和 lowering。
- `Backend`：负责 `MLIR -> LLVM dialect -> LLVM IR`。
- `Runtime`：负责需要运行时支持的语言特性封装。
- `Execution`：负责共享解释执行内核、IR importer、stdout/stderr 捕获和语言 runtime bundle。
- `Testing`：负责 IR golden、前端语义、后端集成和回归测试。

对于 `C/C++`，建议在总体架构中加入一条明确的 `CIR` 通路：

```text
C/C++ Source
  -> Clang Parser/Sema
  -> CIRGen
  -> CIR Dialect
  -> CIR Normalization / MLSE Integration Passes
  -> LLVM Dialect Lowering
  -> LLVM IR
```

这条路径的关键意义是：MLSE 复用 Clang 的解析和语义能力，把工作重点放在 `CIR` 接入、归一化和后续 lowering，而不是重做一套 C/C++ 编译前端。

## 7. IR 设计策略

### 7.1 以标准 MLIR 为主

MLSE 的 canonical IR 应优先复用标准 MLIR dialect：

- 控制流：`scf`、`cf`
- 算术：`arith`
- 函数：`func`
- 内存与张量：`memref`、`tensor`
- 最终后端：`LLVM`

### 7.2 自定义 dialect 最小化

仅在以下情况引入自定义 MLSE dialect：

- 语义暂时无法直接映射到标准 dialect。
- 需要统一承载跨语言但又非 LLVM 级别的概念。
- 需要描述运行时调用、异常语义、切片/对象元信息等。

建议至少保留两个自定义层：

- `mlse.core`：承载标准 dialect 难以覆盖的跨语言语义节点。
- `mlse.runtime`：承载运行时调用、辅助元数据和后续 runtime lowering。

对于执行层，建议同时明确一条稳定 runtime ABI 边界：

- frontend / dialect / lowering 负责把语言语义收口到 runtime ABI
- `mlse-run` 通过语言 runtime bundle 实现这些 ABI
- 共享执行内核不直接内建 Go 或 Python 语义

### 7.3 不建议长期保留重型语言专属 dialect

除非某语言存在明显无法避免的建模需求，否则不建议维护四套长期存在的 `c`、`cpp`、`py`、`go` 专属 dialect。更可控的方式是：

- 语言专属 frontend 尽快下沉到 `standard dialect + mlse.core/mlse.runtime`
- 将语言特有语义限制在前端和局部 lowering 中处理

`CIR` 是这里的例外。因为它已经是外部成熟项目提供的 C/C++ 高层 MLIR 表示，MLSE 可以把它视为“外部前端 dialect”，而不是自己长期维护的一套自定义 C/C++ dialect。MLSE 的职责应是消费、转换和集成 `CIR`，而不是复制一套功能等价实现。

## 8. 语言前端规划

### 8.1 C 前端

目标：以 `ClangIR/CIR` 作为默认前端入口，优先获得高保真的 C 到 MLIR 路径。

阶段性策略：

- 第一阶段：直接集成启用 `CIR` 的 Clang，支持 `C -> CIR` 输出和 `CIR -> LLVM IR` 端到端链路。
- 第二阶段：在 MLSE 中新增 `CIR` 接入和验证能力，将 `CIR` 纳入统一驱动与测试体系。
- 第三阶段：按需要实现 `CIR -> MLSE canonical IR` 的归一化 pass，服务跨语言分析与优化。

建议首批支持特性：

- 继承 `ClangIR` 当前已经稳定覆盖的常见 C 代码模式
- 函数、局部变量、基本类型
- `if` / `while` / `for`
- 数组、结构体、函数调用

暂缓特性：

- 复杂预处理器展开语义建模
- 内联汇编
- 极端依赖未定义行为的程序

实施说明：

- MLSE 不单独维护 C parser 和 Sema。
- MLSE 优先复用 `Clang + CIRGen + CIR passes`。
- 当需要跨语言统一时，再把 `CIR` 的子集 lower 到 `mlse.core` 或标准 dialect。

### 8.2 C++ 前端

目标：同样以 `ClangIR/CIR` 为主入口，但接受 C++ 支持范围小于 C 的现实。

阶段性策略：

- 第一阶段：复用启用 `CIR` 的 Clang，先获得 `C++ -> CIR -> LLVM IR` 的可工作路径。
- 第二阶段：围绕 `ClangIR` 已支持的 C++ 子集建立回归测试、能力矩阵和 MLSE driver 集成。
- 第三阶段：根据 `ClangIR` upstream 进度和 MLSE 需求，逐步扩展保守子集。

建议首批支持特性：

- 命名空间
- 类/结构体的基本字段与方法
- 简单重载解析后的函数实体
- 已实例化后的有限模板结果

暂缓特性：

- 异常
- RTTI
- 协程
- 通用模板语义建模
- 大量依赖标准库魔法行为的高级模式

实施说明：

- MLSE 不直接承诺完整 C++ 语义覆盖。
- `C++` 支持范围应以 `ClangIR` 实际能力和测试覆盖为准。
- 对暂未被 `ClangIR` 稳定覆盖的特性，MLSE 不额外重做一套专有 lowering。

### 8.3 Python 前端

目标：面向可静态约束的 Python 子集生成结构化 MLIR。

阶段性策略：

- 第一阶段：基于 Python `ast` 构建前端，只支持显式可推断类型的子集。
- 第二阶段：支持类型注解、简单容器和数值计算模式。
- 第三阶段：通过 runtime dialect 扩展部分对象语义。

建议首批支持特性：

- 模块级函数
- 基本数值类型
- `if` / `for` / `while`
- 局部变量和纯函数调用
- 显式类型注解驱动的 lowering

暂缓特性：

- `eval` / `exec`
- 动态属性注入
- 反射式元编程
- 高度动态的对象模型

规划假设：

- Python 前端不追求完全兼容解释器语义。
- Python MVP 更接近“可编译的静态子集”，而不是完整 CPython 替代品。

### 8.4 Go 前端

目标：基于 Go 的静态类型和 SSA 能力构建较稳健的结构化前端。

阶段性策略：

- 第零阶段：先在仓库里建立正式 `go` dialect 的 `TableGen + CMake + mlse-opt` 骨架，固定类型系统和目录边界。
- 第一阶段：基于 `go/packages`、`go/types` 和 SSA 构建 typed frontend。
- 第二阶段：将 SSA 结果映射到结构化 MLIR 与 runtime dialect。
- 第三阶段：逐步扩展到接口、切片和部分标准库交互。

建议首批支持特性：

- package 级函数
- 基本类型、结构体、数组、切片
- `if` / `for` / `switch`
- 普通函数调用

当前仓库已经落了这条路线的第一批正式骨架：

- `include/mlse/Go/IR/`：`go` dialect TableGen 与头文件
- `include/mlse/Go/Conversion/`：Go 专属 conversion 接口
- `lib/Go/IR/`：dialect/type 注册实现
- `lib/Go/Conversion/`：Go bootstrap lowering 实现
- `tools/mlse-opt/`：最小 GoIR 驱动，负责解析输入并接线显式 lowering 入口
- `test/GoIR/`：正式 GoIR 方向的最小样本目录

现阶段这条线已经覆盖：

- `!go.*` 类型系统 bootstrap
- 第一批最小必要 op：`go.string_constant`、`go.nil`、`go.make_slice`、`go.len`、`go.cap`、`go.index`、`go.append`、`go.append_slice`、`go.elem_addr`、`go.field_addr`、`go.load`、`go.store`、`go.todo`、`go.todo_value`
- `cmd/mlse-go` 默认输出正式 `go` dialect 的最小 parseable 子集

暂缓特性：

- goroutine / channel
- `select`
- `reflect`
- `cgo`

## 9. 后端规划：MLIR 到 LLVM IR

这是整个项目的第一优先级。

### 9.1 目标

- 支持从 canonical MLIR lowering 到 `LLVM` dialect。
- 支持从 `LLVM` dialect 稳定翻译到 `LLVM IR`。
- 形成最小可复用后端，不依赖具体前端是否完善。

### 9.2 实施顺序

1. 先用手写 MLIR 样例验证 lowering pipeline。
2. 打通 `func/arith/scf/cf/memref -> LLVM dialect`。
3. 打通 `LLVM dialect -> LLVM IR` 输出。
4. 为前端输出建立合法化规则和 verifier。

### 9.3 关键要求

- 每个进入后端的 op 都必须满足清晰的 legality 约束。
- 不同前端生成的 canonical MLIR 应尽量共享同一套 lowering pass。
- runtime dialect 必须有明确的 lowering 目标，不能长期停留在抽象层。
- `CIR` 需要被视为后端合法输入之一，至少支持 `CIR -> LLVM IR` 的稳定路径。

## 10. 执行层规划：IR 到可观测行为

MLSE 建议增加一个共享执行层：

- 工具：`mlse-run`
- 输入：MLSE 当前生成的 MLIR / LLVM IR 子集
- 输出：程序的可观测行为，例如 stdout / stderr、panic 和退出状态

设计原则：

- `mlse-run` 不重做一套 Go 或 Python 源码解释器
- `mlse-run` 以共享执行内核为主，语言差异通过 runtime bundle 注入
- 第一期优先支持 MLSE 当前生成的 LLVM-legal MLIR 及其对应 LLVM IR 子集

执行层的主要价值不是替代 native codegen，而是提供一条可自动化的语义差分路径：

- 原生语言执行
- MLSE lowering 后 IR 执行
- 对比 stdout / stderr

## 11. 推荐实现语言与技术栈

### 11.1 核心实现

- 核心编译驱动、dialect、pass、后端建议使用 `C++` 实现。
- 原因是 MLIR/LLVM 的核心 API 和生态主要集中在 C++。

### 11.2 LLVM/Clang 依赖策略

- `C/C++` 路线建议直接基于启用 `CIR` 的 Clang 构建。
- 优先选择可复现的固定版本来源：
  - `llvm/clangir` incubator 仓库
  - 或包含足够 `CIR` upstream 内容的 `llvm-project` 固定 revision
- 构建时需要启用 `mlir` 与 `clang`，并打开 `CLANG_ENABLE_CIR=ON`。

### 11.3 辅助实现

- Python 可用于 Python 前端原型、测试驱动和开发脚本。
- Go 前端可以用 Go 自身工具链实现语义分析，再通过稳定接口接入核心驱动。

### 11.4 构建与测试

- 构建系统建议使用 `CMake + Ninja`。
- 单元测试建议使用 `gtest`。
- IR 测试建议使用 `lit + FileCheck`。
- 端到端测试建议使用 golden files 和示例程序。

### 11.5 Docker 运行与开发环境

MLSE 的官方支持运行环境应以 `Docker` 为基线。

这样做的目的有四个：

- 固定 `LLVM/MLIR/ClangIR` 版本和系统依赖。
- 让本地开发与 CI 使用同一套工具链环境。
- 降低不同机器上的构建差异。
- 为后续定期清理和回归验证提供可重复环境。

建议在仓库中预留以下内容：

- `docker/Dockerfile.dev`：本地开发镜像
- `docker/Dockerfile.ci`：CI 镜像
- `docker/compose.yml`：可选的多服务或带缓存配置

建议支持的运行模式：

- 交互式开发 shell
- 单次构建
- 单次测试
- 单次 lint / format

目标命令形态示例：

```bash
docker build -f docker/Dockerfile.dev -t mlse-dev .
docker run --rm -it -v "$PWD":/workspace -w /workspace mlse-dev bash
```

说明：

- 当前仓库尚未实现这些文件和命令。
- 这里定义的是后续工程骨架必须满足的接口方向。

### 11.6 标准脚本与命令约定

为了避免把构建、测试和质量检查逻辑散落在文档、CI 和个人 shell 历史中，MLSE 应约定统一脚本入口。

建议至少提供：

- `scripts/build.sh`
- `scripts/test.sh`
- `scripts/lint.sh`
- `scripts/fmt.sh`
- `scripts/clean.sh`

这些脚本需要满足以下约束：

- 本地开发和 CI 复用同一套入口。
- 默认支持在 Docker 环境中执行。
- 输出路径和临时目录保持稳定，便于清理。
- 一旦命令接口公开到 README 或 CI，就视为工程契约的一部分。

## 12. 推荐仓库结构

```text
.
├── README.md
├── docs/
│   ├── README.md
│   ├── spec.md
│   ├── architecture.md
│   ├── execution.md
│   └── dev-setup.md
├── docker/
├── cmake/
├── third_party/
│   └── llvm-project/ or clangir/
├── include/mlse/
│   └── Execution/
├── lib/
│   ├── Dialect/
│   ├── Conversion/
│   ├── Execution/
│   │   ├── Core/
│   │   └── Import/
│   ├── Frontend/
│   │   ├── C/
│   │   ├── CXX/
│   │   ├── Python/
│   │   └── Go/
│   ├── Go/
│   │   └── Execution/
│   ├── Python/
│   │   └── Execution/
│   ├── Driver/
│   └── Runtime/
├── tools/
│   ├── mlsec/
│   ├── mlse-opt/
│   ├── mlse-translate/
│   └── mlse-run/
├── test/
│   ├── GoExec/
│   └── PythonExec/
├── examples/
└── scripts/
```

各层职责建议如下：

- `docker/`：开发和 CI 的标准运行环境定义。
- `third_party/`：固定外部编译基础设施依赖，优先用于 `ClangIR/CIR` 集成。
- `include/mlse/`：对外头文件和核心接口。
- `include/mlse/Execution/`：共享执行层接口、值模型和 runtime 注册边界。
- `lib/Frontend/`：各语言前端实现。
- `lib/Dialect/`：MLSE 自定义 dialect 和类型系统。
- `lib/Conversion/`：canonicalization、合法化和 lowering。
- `lib/Execution/`：共享执行内核与输入 importer。
- `lib/Go/Execution/`、`lib/Python/Execution/`：语言专属 runtime bundle。
- `lib/Driver/`：命令行入口和 pipeline 编排。
- `lib/Runtime/`：运行时抽象和 runtime lowering。
- `tools/`：开发和调试工具。
- `test/GoExec/`、`test/PythonExec/`：执行差分样例，关注 stdout / stderr 与 panic 行为。
- `test/`：lit、FileCheck、单元测试和端到端样例。

## 13. 编译驱动与工具规划

建议至少提供以下工具：

- `mlsec`：主编译驱动，输入源码并输出 MLIR 或 LLVM IR。
- `mlse-opt`：调试和执行 MLIR pass pipeline。
- `mlse-translate`：负责 MLIR 与 LLVM IR 相关翻译或导出。
- `mlse-run`：执行 MLSE 当前支持的 MLIR / LLVM IR 子集，并输出可观测程序行为。

建议输出接口至少覆盖：

- `--emit=mlir`
- `--emit=llvm-ir`
- `--frontend={c,cxx,python,go}`
- `--print-pipeline`

对 `C/C++` 建议额外支持：

- `--c-family-mode={cir,clang-compat}`
- `--emit=cir`

其中：

- `cir` 表示优先使用 `ClangIR/CIR` 作为前端入口。
- `clang-compat` 仅在调试或兼容需要时保留。

对 `mlse-run` 的执行边界建议明确为：

- 第一阶段首选执行 `--lower-go-bootstrap` 之后的 LLVM-legal MLIR。
- `02-formal.mlir` 这类更高层输入可以通过现有 lowering 先 normalize，再进入执行层。
- 只承诺执行 MLSE 当前生成的 LLVM IR 子集，不承诺任意外部 LLVM IR。

## 14. 测试策略

测试体系建议分五层：

### 14.1 Frontend parser/type tests

- 验证不同语言前端对子集语义的解析和类型绑定是否正确。

### 14.2 MLIR golden tests

- 验证给定输入源码是否生成预期 MLIR。

### 14.3 Lowering tests

- 验证 canonical MLIR 是否按预期 lowering 到 `LLVM` dialect 和 `LLVM IR`。

### 14.4 End-to-end tests

- 验证 `source -> MLIR -> LLVM IR` 端到端链路可运行且结果稳定。

### 14.5 Execution differential tests

- 为 `mlse-run` 增加基于 stdout / stderr 的差分测试层。
- 同一份源码一边走原生语言执行，一边走 `frontend -> lowering -> mlse-run`。
- 比较重点应落在可观测行为：
  - stdout / stderr
  - panic / trap 文本
  - 退出状态
- 不应把不稳定的地址、map 迭代顺序或完整栈轨迹作为主要断言。
- 建议为 Go 和 Python 各自维护 repo-owned 的执行样例集，并保持同一套差分 harness。

### 14.6 Docker 化执行与 CI 基线

- `build / test / lint` 的官方执行路径应以 Docker 为准。
- 裸机本地运行可以作为开发加速手段，但不能替代容器基线。
- CI 应直接调用 `scripts/build.sh`、`scripts/test.sh`、`scripts/lint.sh`，而不是复制一套独立命令。
- 新增依赖、工具链版本或环境变量时，必须同步更新 Docker 文件和开发文档。

### 14.7 Lint 与格式化策略

MLSE 需要把 lint 和格式化当作持续工程约束，而不是后期补丁。

建议按语言分层引入：

- `C/C++`：`clang-format`、`clang-tidy`
- `Python`：`ruff check`、`ruff format`
- `Go`：`gofmt`、`go vet`、`golangci-lint`
- `Shell`：`shfmt`、`shellcheck`
- `Dockerfile`：`hadolint`

约束如下：

- 某种语言一旦进入仓库，对应 lint/format 即进入必过检查。
- 格式化工具优先自动修复，lint 工具负责拦截语义和风格问题。
- PR 合入前，必须至少通过与变更文件类型对应的检查。

### 14.8 定期代码清理与仓库卫生

MLSE 应把代码清理视为常规维护工作，而不是偶发重构。

建议清理触发时机：

- 每个阶段性里程碑前
- 较大重构完成后
- 活跃开发期间按固定节奏执行，例如每 `2-4` 周一次

清理范围建议包括：

- 已废弃但仍残留的 pass、helper、选项和兼容路径
- 无人维护的实验代码和过期示例
- 长期注释掉但未删除的代码块
- 失效文档、过期说明和与实现不一致的描述
- 无 owner 的禁用测试或长期跳过项
- 误提交的构建产物、缓存、临时输出

建议由 `scripts/clean.sh` 承担基础清理能力，至少覆盖：

- `build/`
- `test-output/`
- `artifacts/`
- 常见临时目录和缓存产物

清理规则：

- 清理若改变行为，必须同步补测试和文档。
- 清理若仅删除死代码，也必须确认没有被脚本、文档或 CI 隐式依赖。
- 仓库中不应长期保留“未来可能有用”的无主代码。

## 15. 分阶段实施路线

### M0: 规划与骨架

- 建立仓库结构、构建系统和基础测试框架。
- 建立 Docker 开发/CI 环境骨架。
- 引入 LLVM/MLIR 依赖。
- 创建 `mlsec`、`mlse-opt`、`mlse-translate` 的空骨架。
- 创建 `build / test / lint / fmt / clean` 脚本骨架。

### M1: 后端先行

- 支持手写 MLIR 输入。
- 打通 `MLIR -> LLVM dialect -> LLVM IR`。
- 建立第一批 lowering 和 golden tests。
- 接入基础 CI，并让容器内 `test` 与 `lint` 成为默认检查。
- 为后续 `mlse-run` 预留共享执行层接口，不把语言语义写进 driver 或测试脚本。

### M2: C/C++ CIR 集成

- 接入启用 `CIR` 的 Clang/ClangIR。
- 实现 `C/C++ -> CIR` 输出与验证。
- 打通 `C/C++ -> CIR -> LLVM IR`。
- 将 `CIR` 纳入 MLSE driver。

### M3: Python 结构化前端

- 基于 Python AST 落地静态子集。
- 生成 `func/arith/scf/cf/memref` 为主的结构化 MLIR。
- 为动态语义预留 runtime dialect 扩展点。

### M4: Go 结构化前端

- 基于 typed AST/SSA 接入 Go 子集。
- 补充数组、切片、结构体和函数调用的 lowering。
- 增加 `Go -> lowering -> mlse-run` 的 stdout / stderr 差分样例。

### M5: C/C++ CIR 归一化

- 为跨语言分析实现 `CIR -> MLSE canonical IR` 的局部 lowering。
- 识别最有价值的 `CIR` 子集并映射到标准 dialect 与 `mlse.core`。
- 保持 `CIR` 直通后端路径和归一化路径并存。

### M6: C++ 保守子集扩展

- 跟随 `ClangIR` upstream 能力扩展可支持范围。
- 视收益决定是否增强模板、对象模型和 ABI 相关处理中间层。

## 16. 主要风险

- `ClangIR` 仍处于 incubator/upstreaming 过程中，版本漂移和 API 变动风险较高。
- 当前官方状态更偏“C 大体可用，C++ 仍有缺口”，尤其清理、异常和部分 ABI 相关能力仍有限。
- `C++` 语义复杂度极高，若过早追求完整覆盖，会严重拖慢项目节奏。
- Python 动态语义与静态 MLIR 之间存在明显语义鸿沟，必须提前定义受支持子集。
- 纯 `LLVM` dialect 导入虽然容易打通，但会牺牲高层结构和优化价值。
- 多语言共用 runtime 抽象时，容易把前端问题推迟到 runtime，导致后期复杂度累积。
- 如果执行层没有保持“共享内核 + 语言 bundle”分层，Go 和 Python 很容易演化成两套彼此分叉的 runner。
- LLVM/MLIR 版本升级可能引入 API 和 pass 行为变化，需要早期建立版本固定策略。

## 17. 当前结论

基于当前目标，MLSE 的合理推进方式不是同时做四个完整前端，而是：

1. 先把 `MLIR -> LLVM IR` 做成稳定后端。
2. 直接复用 `ClangIR/CIR` 作为 `C/C++` 前端入口，尽快形成第一个端到端闭环。
3. 用 `Python` 和 `Go` 的受限子集验证结构化 MLIR 路线。
4. 在 `LLVM-legal MLIR / LLVM IR` 上增加共享执行层，用 stdout / stderr 差分验证 lowering 语义。
5. 再逐步把高价值 `CIR` 子集归一化到跨语言共享表示。

## 18. 待确认问题

- Python MVP 是否强制要求类型注解。
- Go MVP 是否完全排除并发语义。
- C/C++ 的默认交付形态是否明确以 `CIR` 作为一等输出与调试接口。
- 是否需要在第一阶段支持跨文件和多模块编译。
- 是否需要尽早定义 runtime ABI 和外部函数调用约定。
- 执行层第一阶段是否只覆盖 `stdout / stderr / panic / exit status`，还是同时要求稳定的结构化返回值协议。
