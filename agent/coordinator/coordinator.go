package coordinator

import (
	"context"
	"encoding/json"
	"time"

	"github.com/HildaM/logs/slog"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/hildam/deer-flow-go/entity/consts"
	"github.com/hildam/deer-flow-go/entity/model"
	"github.com/hildam/deer-flow-go/repo/llm"
	"github.com/hildam/deer-flow-go/repo/template"
)

// coordinatorImpl 任务协调者
type coordinatorImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewCoordinator 创建实例
func NewCoordinator[I, O any](ctx context.Context) *coordinatorImpl[I, O] {
	return &coordinatorImpl[I, O]{
		llm: llm.NewChatModel(ctx),
	}
}

// NewGraph 创建任务图
func (c *coordinatorImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	// 创建新的DAG图实例
	graph := compose.NewGraph[I, O]()

	// 定义"hand_to_planner"工具，这是Coordinator唯一可用的工具
	// 该工具用于将任务移交给Planner，同时检测用户语言
	hand_to_planner := &schema.ToolInfo{
		Name: "hand_to_planner",
		Desc: "Handoff to planner agent to do plan.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"task_title": {
				Type:     schema.String,
				Desc:     "The title of the task to be handed off.",
				Required: true,
			},
			"locale": {
				Type:     schema.String,
				Desc:     "The user's detected language locale (e.g., en-US, zh-CN).",
				Required: true,
			},
		}),
	}

	// 创建配置了工具的聊天模型，该模型能够调用hand_to_planner工具
	coorModel, _ := c.llm.WithTools([]*schema.ToolInfo{hand_to_planner})

	// 构建DAG图的三个核心节点：
	// 1. load节点：加载提示词模板和准备输入数据
	graph.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadMsg))
	// 2. agent节点：执行AI模型推理，检测语言并决定是否移交给Planner
	graph.AddChatModelNode("agent", coorModel)
	// 3. router节点：解析工具调用结果，决定下一步路由（BackgroundInvestigator或Planner）
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(router))

	// 构建节点间的连接关系，形成线性的处理流程
	graph.AddEdge(compose.START, "load") // 开始 → load
	graph.AddEdge("load", "agent")       // load → agent
	graph.AddEdge("agent", "router")     // agent → router
	graph.AddEdge("router", compose.END) // router → 结束

	return consts.Coordinator, graph, compose.WithNodeName(consts.Coordinator)
}

// loadMsg 加载提示词模板和准备输入数据
func loadMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(ctx context.Context, state *model.State) error {
		// 从基础设施层获取提示词模板
		sysPrompt, err := template.GetPromptTemplate(ctx, name)
		if err != nil {
			slog.Error("loadMsg failed, get prompt template fail", "err", err)
			return err
		}

		// 构建Jinja2格式的提示词模板，包含系统消息和用户输入占位符
		promptTemp := prompt.FromMessages(schema.Jinja2,
			schema.SystemMessage(sysPrompt),
			schema.MessagesPlaceholder("user_input", true),
		)

		// 准备模板变量，这些变量会被注入到提示词模板中
		variables := map[string]any{
			"locale":              state.Locale,                             // 用户语言设置
			"max_step_num":        state.MaxStepNum,                         // 最大步骤数
			"max_plan_iterations": state.MaxPlanIterations,                  // 最大计划迭代次数
			"CURRENT_TIME":        time.Now().Format("2006-01-02 15:04:05"), // 当前时间
			"user_input":          state.Messages,                           // 用户输入消息
		}
		// 使用变量格式化提示词模板，生成最终的消息列表
		output, err = promptTemp.Format(ctx, variables)
		if err != nil {
			slog.Error("loadMsg failed, format prompt template fail", "err", err)
			return err
		}
		return nil
	})
	return output, err
}

// router Coordinator的router节点处理函数，负责解析AI模型的工具调用结果并决定下一步路由
func router(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	// 使用ProcessState处理状态，确保状态的线程安全访问和修改
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 使用defer确保output总是被设置为state.Goto的值
		defer func() {
			output = state.Goto
		}()

		// 默认设置为结束，如果没有工具调用就直接结束流程
		state.Goto = compose.END

		// 检查是否有工具调用，且工具名称为"hand_to_planner"
		if len(input.ToolCalls) > 0 && input.ToolCalls[0].Function.Name == "hand_to_planner" {
			// 解析工具调用的参数，获取任务标题和语言设置
			argMap := map[string]string{}
			if err = json.Unmarshal([]byte(input.ToolCalls[0].Function.Arguments), &argMap); err != nil {
				slog.Error("router failed, unmarshal tool arguments fail", "err", err)
				return err
			}

			// 从工具参数中提取并保存用户的语言设置
			state.Locale = argMap["locale"]

			// 根据配置决定下一步：是否启用背景调查
			if state.EnableBackgroundInvestigation {
				// 如果启用背景调查，先跳转到BackgroundInvestigator
				state.Goto = consts.BackgroundInvestigator
			} else {
				// 否则直接跳转到Planner开始计划生成
				state.Goto = consts.Planner
			}
		}
		return nil
	})
	return output, nil
}
