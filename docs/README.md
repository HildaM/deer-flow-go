# Deer Flow Go 项目文档

本文档库包含了 `deer-flow-go` 项目的详细技术文档，按主题分类整理。

## 📁 文档分类

### 🏗️ Architecture (架构设计)

包含项目架构相关的文档：

- **[architecture.md](./architecture/architecture.md)** - 项目架构图和系统设计
  - 多智能体协作系统架构
  - DAG 工作流设计
  - 各 Agent 节点的内部结构（Coordinator、Planner、Researcher、Coder等）
  - 状态管理机制
  - MCP 工具集成
  - 技术实现细节和扩展性分析

### 🔧 Generics (Go 泛型技术)

包含 Go 泛型设计理念和应用的深度分析：

- **[golang-generics-guide.md](./generics/golang-generics-guide.md)** - Go 泛型设计理念深度解析
  - 泛型基础概念
  - `Builder[I, O, S any]` 设计分析
  - `compose.Runnable[I, O]` 接口设计
  - 实际应用场景

- **[generics-usage-analysis.md](./generics/generics-usage-analysis.md)** - 泛型参数具体使用分析
  - `I`, `O`, `S`, `T` 参数的实际绑定
  - Agent 创建函数中的泛型传递
  - 状态管理中的类型安全
  - 数据流示例

- **[go-generics-type-erasure.md](./generics/go-generics-type-erasure.md)** - Go 泛型类型擦除详解
  - 类型擦除概念
  - 编译时检查机制
  - 运行时行为分析
  - 与 Java 泛型对比
  - 性能影响分析

- **[generics-demo.go](./generics/generics-demo.go)** - 泛型概念演示程序
  - 泛型函数和结构体示例
  - 类型约束演示
  - 编译时类型安全验证
  - 运行时类型信息检查
  - 内存布局对比

### 🔌 MCP (Model Context Protocol)

包含 MCP 协议集成相关的文档：

- **[README.md](./mcp/README.md)** - MCP 集成完整指南
  - MCP 架构设计和系统集成
  - Python MCP 服务器实现
  - 搜索工具和代码执行工具
  - 配置说明和环境设置
  - Agent 中的 MCP 工具使用
  - 开发调试和性能优化
  - 安全考虑和最佳实践

## 🎯 文档使用指南

### 新手入门
1. 先阅读 [architecture.md](./architecture/architecture.md) 了解项目整体架构
2. 再阅读 [golang-generics-guide.md](./generics/golang-generics-guide.md) 理解泛型设计理念

### 深入学习
1. 阅读 [generics-usage-analysis.md](./generics/generics-usage-analysis.md) 了解泛型的具体应用
2. 阅读 [go-generics-type-erasure.md](./generics/go-generics-type-erasure.md) 深入理解泛型机制
3. 运行 [generics-demo.go](./generics/generics-demo.go) 实践泛型概念

### 开发参考
- 架构设计时参考 `architecture/` 目录
- 泛型编程时参考 `generics/` 目录

## 📝 文档维护

- 架构相关文档请放入 `architecture/` 目录
- 泛型技术相关文档请放入 `generics/` 目录
- 新增文档时请更新本 README 文件

---

*最后更新时间：2024年8月10日*