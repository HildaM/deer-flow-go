package researcher

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

// singleResearcherImpl 单个研究者
type singleResearcherImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewSingleResearcher 创建实例
func NewSingleResearcher[I, O any](ctx context.Context) *singleResearcherImpl[I, O] {
	return &singleResearcherImpl[I, O]{
		llm: llm.NewChatModel(ctx),
	}
}

// NewGraph 创建任务图
func (r *singleResearcherImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	// 创建图实例
	graph := compose.NewGraph[I, O]()

	// 使用全部 mcp 工具
	tools, err := mcp.GetMCPTools(ctx)
	if err != nil {
		slog.Error("NewGraphNode failed, get mcp tools err = %+v", err)
		// 失败不影响使用
		tools = []tool.BaseTool{}
	}
	slog.Debug("singleResearcherImpl NewGraphNode, mcp tools = %+v", tools)

	// 创建 ReAct Agent
	reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		MaxStep:               conf.GetCfg().Setting.AgentMaxStep,
		ToolCallingModel:      r.llm,
		ToolsConfig:           compose.ToolsNodeConfig{Tools: tools},
		MessageModifier:       comm.ModifyInputFunc, // 消息长度限制处理器
		StreamToolCallChecker: comm.ToolCallChecker, // 工具调用检测器
	})
	if err != nil {
		slog.Fatal("NewGraphNode failed, create react agent err = %+v", err)
	}

	// 封装为 lambda 节点
	agentLambda, err := compose.AnyLambda(reactAgent.Generate, reactAgent.Stream, nil, nil)
	if err != nil {
		slog.Fatal("NewGraphNode failed, create lambda node err = %+v", err)
	}

	// 添加节点
	graph.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadMsg))
	graph.AddLambdaNode("agent", agentLambda)
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(singleRouter))

	// 构造关联
	graph.AddEdge(compose.START, "load")
	graph.AddEdge("load", "agent")
	graph.AddEdge("agent", "router")
	graph.AddEdge("router", compose.END)

	return consts.Researcher, graph, compose.WithNodeName(consts.Researcher)
}

// loadMsg 为Researcher智能体加载消息和提示词模板
func loadMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 获取Researcher的系统提示词模板，定义研究任务的执行方式
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

		// 从当前计划中找到第一个未执行的研究步骤
		var curStep *model.Step
		for i := range state.CurrentPlan.Steps {
			if state.CurrentPlan.Steps[i].ExecutionRes == nil {
				curStep = &state.CurrentPlan.Steps[i]
				break
			}
		}

		// 确保找到了待执行的步骤
		if curStep == nil {
			panic("no step found")
		}

		// 构建消息列表，包含当前研究步骤的详细信息
		msg := []*schema.Message{}
		// 添加当前研究步骤的任务信息（标题、描述、语言设置）
		msg = append(msg,
			schema.UserMessage(fmt.Sprintf("#Task\n\n##title\n\n %v \n\n##description\n\n %v \n\n##locale\n\n %v", curStep.Title, curStep.Description, state.Locale)),
			// 添加引用格式指导，要求在文末统一列出参考资料而非内联引用
			schema.SystemMessage("IMPORTANT: DO NOT include inline citations in the text. Instead, track all sources and include a References section at the end using link reference format. Include an empty line between each citation for better readability. Use this format for each reference:\n- [Source Title](URL)\n\n- [Another Source](URL)"),
		)
		variables := map[string]any{
			"locale":              state.Locale,
			"max_step_num":        state.MaxStepNum,
			"max_plan_iterations": state.MaxPlanIterations,
			"CURRENT_TIME":        time.Now().Format("2006-01-02 15:04:05"),
			"user_input":          msg,
		}
		output, err = promptTemp.Format(ctx, variables)
		return err
	})
	return output, err
}

// singleRouter 为Researcher智能体路由函数
func singleRouter(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	slog.Debug("singleRouter debug, input = %+v", input)
	last := input
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()
		// 将研究结果保存到第一个未执行步骤的ExecutionRes字段中
		for i, step := range state.CurrentPlan.Steps {
			if step.ExecutionRes == nil {
				// 克隆研究结果内容并保存
				str := strings.Clone(last.Content)
				state.CurrentPlan.Steps[i].ExecutionRes = &str
				break
			}
		}
		// 记录研究任务完成的事件，包含更新后的计划状态
		slog.Debug("routerResearcher debug, researcher_end, plan = %+v", state.CurrentPlan)

		// 返回调度中心，由ResearchTeam决定下一步执行哪个智能体
		state.Goto = consts.ResearchTeam
		return nil
	})
	return output, nil
}
