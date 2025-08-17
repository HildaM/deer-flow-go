# deer-flow-go

[bytedance/deer-flow](https://github.com/bytedance/deer-flow) 项目的 Golang 版本实现

## 🙏 致谢


- **[eino-examples/deer-go](https://github.com/cloudwego/eino-examples/tree/main/flow/agent/deer-go)** - 原项目
- **[bytedance/deer-flow](https://github.com/bytedance/deer-flow)**

感谢 CloudWeGo 团队和字节跳动开源社区为 AI Agent 生态做出的贡献！

## 📖 项目简介

**deer-flow-go** 是基于 [deer-flow](https://github.com/bytedance/deer-flow) 和 [eino-examples/deer-go](https://github.com/cloudwego/eino-examples/tree/main/flow/agent/deer-go) 项目进行的二次开发和增强。

### 🌟 核心特性

- **智能任务规划**: 自动将复杂任务分解为多个可执行步骤
- **多角色协作**: 包含协调员、规划师、研究员、编码员等多个专业角色
- **MCP 工具集成**: 支持 Tavily 搜索、Python 代码执行等多种工具
- **流式响应**: 实时展示任务执行过程和结果
- **Web API**: 提供 RESTful API 接口，支持前端集成
- **灵活配置**: 支持多种 LLM 模型和自定义参数

### 🏗️ 技术架构

- **框架**: [CloudWeGo Eino](https://github.com/cloudwego/eino) - 企业级 AI Agent 开发框架
- **Web 服务**: [Hertz](https://github.com/cloudwego/hertz) - 高性能 HTTP 框架
- **工具协议**: [MCP (Model Context Protocol)](https://github.com/modelcontextprotocol/specification) - 标准化工具集成
- **语言**: Go 1.23+ - 高性能并发处理


## 🚀 快速开始

### 环境要求

- **Go**: 1.23.0 或更高版本
- **Python**: 3.8+ (用于 MCP Python 服务器)
- **Node.js**: 16+ (用于 Tavily MCP 服务器)
- **uv**: Python 包管理工具 ([安装指南](https://docs.astral.sh/uv/getting-started/installation/))

### 安装步骤

#### 1. 克隆项目
```bash
git clone <repository-url>
cd deer-flow-go
```

#### 2. 安装 Python MCP 服务器依赖
```bash
cd mcps/python
uv sync
cd ../..
```

#### 3. 配置项目

复制配置模板并填入必要的 API 密钥：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml` 文件，配置以下参数：

```yaml
mcp:
  servers:
    tavily:  # 网络搜索工具
      command: "npx"
      args: ["-y", "tavily-mcp@0.1.3"]
      env: { "TAVILY_API_KEY": "your-tavily-api-key" }
    python:  # Python 代码执行工具
      command: "uv"
      args: [ "--directory", "/path/to/your/project/mcps/python", "run", "server.py" ]

model:
  default_model: "gpt-4"  # 或其他支持的模型
  api_key: "your-openai-api-key"
  base_url: "https://api.openai.com/v1"  # 或其他兼容的 API 端点

setting:
  max_plan_iterations: 1  # 最大规划迭代次数
  max_step_num: 3        # 每个计划的最大步骤数
```

#### 4. 获取 API 密钥

- **Tavily API**: 访问 [Tavily](https://tavily.com/) 获取搜索 API 密钥
- **OpenAI API**: 访问 [OpenAI](https://platform.openai.com/) 或使用兼容的服务提供商

### 运行项目

#### 控制台模式（交互式）
```bash
./run.sh
```


## 🔧 高级配置

### MCP 服务器配置

项目支持多种 MCP 服务器，您可以根据需要启用或禁用：

```yaml
mcp:
  servers:
    # 网络搜索（推荐）
    tavily:
      command: "npx"
      args: ["-y", "tavily-mcp@0.1.3"]
      env: { "TAVILY_API_KEY": "your-key" }
    
    # 网页抓取（可选）
    firecrawl:
      command: "npx"
      args: ["-y", "firecrawl-mcp"]
      env: { "FIRECRAWL_API_KEY": "your-key" }
    
    # Python 代码执行（推荐）
    python:
      command: "uv"
      args: ["--directory", "/path/to/project/mcps/python", "run", "server.py"]
```

### 模型配置

支持多种 LLM 提供商：

```yaml
# OpenAI
model:
  default_model: "gpt-4"
  api_key: "sk-..."
  base_url: "https://api.openai.com/v1"

# Azure OpenAI
model:
  default_model: "gpt-4"
  api_key: "your-azure-key"
  base_url: "https://your-resource.openai.azure.com/"

# 其他兼容服务
model:
  default_model: "claude-3-sonnet"
  api_key: "your-key"
  base_url: "https://api.anthropic.com/v1"
```

## 🛠️ 开发指南

### 项目结构

```
deer-flow-go/
├── agent/                 # Agent 实现
│   ├── agent.go          # Agent 接口定义
│   ├── coordinator/      # 协调员角色
│   │   └── coordinator.go
│   ├── planner/          # 规划师角色
│   │   └── planner.go
│   ├── researcher/       # 研究员角色
│   │   ├── researcher.go
│   │   └── research_team.go
│   ├── coder/            # 编码员角色
│   │   └── coder.go
│   ├── repoter/          # 报告员角色
│   │   └── repoter.go
│   ├── investigator/     # 背景调查员角色
│   │   └── investigator.go
│   ├── human/            # 人工反馈处理
│   │   └── human.go
│   └── comm/             # 通用组件
│       └── comm.go
├── entity/               # 数据实体
│   ├── conf/             # 配置结构体
│   │   ├── conf.go
│   │   └── types.go
│   ├── consts/           # 常量定义
│   │   └── consts.go
│   └── model/            # 数据模型
│       ├── plan.go
│       ├── server.go
│       └── state.go
├── repo/                 # 基础设施层
│   ├── callback/         # 回调处理
│   │   └── logger_callback.go
│   ├── checkpoint/       # 检查点管理
│   │   └── checkpoint.go
│   ├── llm/              # LLM 模型服务
│   │   └── llm.go
│   ├── mcp/              # MCP 工具集成
│   │   ├── mcp.go
│   │   └── types.go
│   └── template/         # 模板管理
│       └── template.go
├── mcps/                 # MCP 服务器
│   └── python/           # Python MCP 服务器
│       ├── server.py
│       ├── pyproject.toml
│       └── uv.lock
├── prompts/              # 提示词模板
│   ├── coordinator.md
│   ├── planner.md
│   ├── researcher.md
│   ├── coder.md
│   └── reporter.md
├── docs/                 # 项目文档
│   ├── README.md
│   ├── architecture/
│   │   └── architecture.md
│   └── generics/
│       ├── golang-generics-guide.md
│       ├── generics-usage-analysis.md
│       ├── go-generics-type-erasure.md
│       └── generics-demo.go
├── config.yaml.example   # 配置文件模板
├── config.yaml          # 配置文件
├── main.go              # 程序入口
├── go.mod               # Go 模块文件
├── go.sum               # Go 依赖锁定文件
└── script/              # 脚本文件
    └── bootstrap.sh
```

### 添加新的 MCP 工具

1. 在 `conf/deer-go.yaml` 中添加新的 MCP 服务器配置
2. 在相应的 Agent 中集成新工具
3. 更新提示词模板以指导 AI 使用新工具

### 自定义 Agent 角色

参考 `agent/` 目录下的现有实现，创建新的 Agent 角色：

```go
// 实现 Agent 接口
type customAgentImpl[I, O any] struct {
    llm *openai.ChatModel
}

func NewCustomAgent[I, O any](ctx context.Context) *customAgentImpl[I, O] {
    return &customAgentImpl[I, O]{
        llm: llm.NewChatModel(ctx),
    }
}

func (c *customAgentImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
    // 实现自定义 Agent 逻辑
    graph := compose.NewGraph[I, O]()
    // 添加节点和边
    return "custom_agent", graph, compose.WithNodeName("CustomAgent")
}
```

## 🐛 故障排除

### 常见问题

**Q: Python MCP 服务器启动失败**
```bash
# 确保 uv 已安装
curl -LsSf https://astral.sh/uv/install.sh | sh

# 重新安装依赖
cd mcps/python
uv sync --reinstall
```

**Q: Tavily 搜索不工作**
- 检查 API 密钥是否正确
- 确认网络连接正常
- 验证 Node.js 环境

**Q: 模型调用失败**
- 检查 API 密钥和 base_url 配置
- 确认模型名称正确
- 查看日志获取详细错误信息



## 📄 许可证

Apache License 2.0 - 详见 [LICENSE](LICENSE) 文件

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📞 支持

如有问题，请通过以下方式联系：

- 提交 [GitHub Issue](../../issues)
- 查看 [CloudWeGo Eino 文档](https://github.com/cloudwego/eino)


