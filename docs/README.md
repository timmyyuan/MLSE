# Docs

`docs/` 用来集中维护 MLSE 的项目文档。

当前仓库还处于初始化阶段，因此这里的文档重点是建立统一入口和后续扩展规则，而不是沉淀已经稳定的实现细节。

## 当前已有文档

- [spec.md](spec.md): 仓库初始化阶段的规格说明，定义目标、范围、目录规划和协作基线。
- [go-frontend.md](go-frontend.md): Go 前端 MVP 的能力边界、临时 stub 和后续扩展方向。
- [go-frontend-lowering.md](go-frontend-lowering.md): `internal/gofrontend/formal_*` 的 lowering 入口地图与源码前后对照示例。
- [goir-dialect.md](goir-dialect.md): 正式 `go` dialect 的 TableGen/CMake bootstrap、当前边界和构建方式。
- [dev-setup.md](dev-setup.md): 当前可用脚本、目录约定以及 tinygo 实验入口。

## 建议后续补充的文档

- `architecture.md`：系统结构、模块边界、关键依赖和数据流。
- `dev-setup.md`：本地开发环境、依赖安装、运行方式和测试命令。
- `api.md`：如果后续存在对外接口，可集中描述接口契约和示例。
- `decisions/`：记录重要设计决策、替代方案和最终取舍。

## 文档维护规则

- 文档应该描述当前真实状态，计划中的内容要明确标为计划。
- 根 `README.md` 保持简洁，负责提供项目概览和入口。
- 需要展开解释的内容放在 `docs/` 下，不把根 README 堆成实现细节合集。
- 当代码、命令、目录或接口发生变化时，相关文档要同步更新。

## 推荐阅读顺序

1. 先看根目录 `README.md`，了解仓库现状和当前阶段目标。
2. 再看 `docs/spec.md`，理解仓库级约束和目录规划。
3. 等代码落地后，再按主题进入开发指南、架构说明或接口文档。
