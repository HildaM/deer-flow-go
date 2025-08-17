package coder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HildaM/logs/slog"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/hildam/deer-flow-go/agent/comm"
	"github.com/hildam/deer-flow-go/entity/conf"
	"github.com/hildam/deer-flow-go/entity/consts"
	"github.com/hildam/deer-flow-go/entity/model"
	"github.com/hildam/deer-flow-go/repo/llm"
	"github.com/hildam/deer-flow-go/repo/mcp"
	"github.com/hildam/deer-flow-go/repo/template"
)

// coderImpl 代码生成者
type coderImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewCoder 创建实例
func NewCoder[I, O any](ctx context.Context) *coderImpl[I, O] {
	return &coderImpl[I, O]{
		llm: llm.NewChatModel(ctx),
	}
}

// NewGraph 创建任务图
func (c *coderImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	// 创建工作流图
	graph := compose.NewGraph[I, O]()

	// 获取 mcp 工具
	allTools, err := mcp.GetMCPTools(ctx)
	if err != nil {
		slog.Fatal("NewGraphNode failed, get mcp tools failed", "err", err)
		return "", nil, nil
	}

	// 过滤出python相关的工具，为代码生成任务提供专业工具支持
	codeTools := []tool.BaseTool{}
	for _, t := range allTools {
		info, err := t.Info(ctx)
		if err != nil {
			slog.Error("NewGraphNode failed, get tool info failed", "err", err)
			continue
		}

		// 检查工具名称是否包含python相关关键词
		if strings.Contains(strings.ToLower(info.Name), "python") ||
			strings.Contains(strings.ToLower(info.Desc), "python") {
			codeTools = append(codeTools, t)
		}
	}
	slog.Debug("NewGraphNode debug, code tools = %+v", codeTools)

	// 创建react智能体
	reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		MaxStep:               conf.GetCfg().Setting.AgentMaxStep,        // 最大执行步骤数
		ToolCallingModel:      c.llm,                                     // 工具调用模型
		ToolsConfig:           compose.ToolsNodeConfig{Tools: codeTools}, // Python相关工具配置
		MessageModifier:       comm.ModifyInputFunc,                      // 消息长度限制处理器
		StreamToolCallChecker: comm.ToolCallChecker,                      // 流式工具调用检查器
	})
	if err != nil {
		slog.Fatal("NewGraphNode failed, create react agent failed", "err", err)
		return "", nil, nil
	}

	// 将 agent 包装为 lambda 节点
	agentLambda, err := compose.AnyLambda(reactAgent.Generate, reactAgent.Stream, nil, nil)
	if err != nil {
		slog.Fatal("NewGraphNode failed, create agent lambda failed", "err", err)
		return "", nil, nil
	}

	// 添加工作流节点
	graph.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadMsg))
	graph.AddLambdaNode("agent", agentLambda)
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(routerCoder))

	// 构造工作流
	graph.AddEdge(compose.START, "load")
	graph.AddEdge("load", "agent")
	graph.AddEdge("agent", "router")
	graph.AddEdge("router", compose.END)

	return consts.Coder, graph, compose.WithNodeName(consts.Coder)
}

// loadMsg 消息加载函数
func loadMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(ctx context.Context, state *model.State) error {
		// 获取 Prompt 模板
		sysPrompt, err := template.GetPromptTemplate(ctx, name)
		if err != nil {
			slog.Error("loadMsg failed, GetPromptTemplate err = %+v, prompt name = %+v", err, name)
			return err
		}

		// 创建Jinja2模板，包含系统提示词和用户输入占位符
		promptTemp := prompt.FromMessages(schema.Jinja2,
			schema.SystemMessage(sysPrompt),
			schema.MessagesPlaceholder("user_input", true),
		)

		// 从当前计划中找到第一个未执行的代码生成步骤
		var curStep *model.Step
		for i := range state.CurrentPlan.Steps {
			if state.CurrentPlan.Steps[i].ExecutionRes == nil {
				curStep = &state.CurrentPlan.Steps[i]
				slog.Error("loadMsg debug, found coder step, step = %+v", curStep)
				break
			}
		}

		// 确保找到了待执行的代码
		if curStep == nil {
			slog.Fatal("loadMsg failed, not found coder step")
		}

		// 构建消息列表，包含当前代码生成步骤的详细信息
		msg := []*schema.Message{}
		// 添加当前代码生成步骤的任务信息（标题、描述、语言设置）
		msg = append(msg,
			schema.UserMessage(fmt.Sprintf(
				"#Task\n\n##title\n\n %v \n\n##description\n\n %v \n\n##locale\n\n %v",
				curStep.Title, curStep.Description, state.Locale),
			),
		)

		// 设置模板变量，包含系统配置和当前任务信息
		variables := map[string]any{
			"locale":              state.Locale,                             // 语言设置
			"max_step_num":        state.MaxStepNum,                         // 最大步骤数
			"max_plan_iterations": state.MaxPlanIterations,                  // 最大计划迭代次数
			"CURRENT_TIME":        time.Now().Format("2006-01-02 15:04:05"), // 当前时间
			"user_input":          msg,                                      // 用户输入消息
		}

		// 格式化模板并生成最终的消息列表
		output, err = promptTemp.Format(ctx, variables)
		return err
	})
	return output, err
}

// routerCoder 代码生成者的路由函数
func routerCoder(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	slog.Debug("routerCoder debug, input = %+v", input)

	last := input
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			// 确保 output 返回最新值
			output = state.Goto
		}()

		// 将代码生成结果保存到第一个未执行步骤的ExecutionRes字段中
		for i, step := range state.CurrentPlan.Steps {
			if step.ExecutionRes == nil {
				// 克隆代码生成结果内容并保存
				str := strings.Clone(last.Content)
				state.CurrentPlan.Steps[i].ExecutionRes = &str
				break
			}
		}
		// 记录代码生成任务完成的事件，包含更新后的计划状态
		slog.Debug("routerCoder debug, plan = %+v", state.CurrentPlan)

		// 返回调度中心，由ResearchTeam决定下一步执行哪个智能体
		state.Goto = consts.ResearchTeam
		return nil
	})
	return output, err
}
