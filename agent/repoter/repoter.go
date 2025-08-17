package repoter

import (
	"context"
	"fmt"
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

// repoterImpl 报告者
type repoterImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewRepoter 创建实例
func NewRepoter[I, O any](ctx context.Context) *repoterImpl[I, O] {
	return &repoterImpl[I, O]{
		llm: llm.NewChatModel(ctx),
	}
}

// NewGraph 创建任务图
func (r *repoterImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	// 创建图示例
	graph := compose.NewGraph[I, O]()

	// 添加节点
	graph.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadMsg))
	graph.AddChatModelNode("agent", r.llm)
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(router))

	// 构造关联
	graph.AddEdge(compose.START, "load")
	graph.AddEdge("load", "agent")
	graph.AddEdge("agent", "router")
	graph.AddEdge("router", compose.END)

	return consts.Reporter, graph, compose.WithNodeName(consts.Reporter)
}

// loadMsg 加载消息，为Reporter智能体加载消息和提示词模板
func loadMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 获取Reporter的系统提示词模板，定义报告生成的格式和要求
		sysPrompt, err := template.GetPromptTemplate(ctx, name)
		if err != nil {
			slog.Error("loadMsg failed, GetPromptTemplate err = %+v, template name = %+v", err, name)
			return err
		}

		// 创建Jinja2模板，包含系统提示词和用户输入占位符
		promptTemp := prompt.FromMessages(schema.Jinja2,
			schema.SystemMessage(sysPrompt),
			schema.MessagesPlaceholder("user_input", true),
		)

		// 构建消息列表，包含研究任务信息和格式指导
		msg := []*schema.Message{}
		// 添加研究任务的基本信息（标题和描述）
		msg = append(msg,
			schema.UserMessage(fmt.Sprintf("# Research Requirements\n\n## Task\n\n %v \n\n## Description\n\n %v", state.CurrentPlan.Title, state.CurrentPlan.Thought)),
			// 添加报告格式的详细指导，强调结构化输出和Markdown表格的使用
			schema.SystemMessage("IMPORTANT: Structure your report according to the format in the prompt. Remember to include:\n\n1. Key Points - A bulleted list of the most important findings\n2. Overview - A brief introduction to the topic\n3. Detailed Analysis - Organized into logical sections\n4. Survey Note (optional) - For more comprehensive reports\n5. Key Citations - List all references at the end\n\nFor citations, DO NOT include inline citations in the text. Instead, place all citations in the 'Key Citations' section at the end using the format: `- [Source Title](URL)`. Include an empty line between each citation for better readability.\n\nPRIORITIZE USING MARKDOWN TABLES for data presentation and comparison. Use tables whenever presenting comparative data, statistics, features, or options. Structure tables with clear headers and aligned columns. Example table format:\n\n| Feature | Description | Pros | Cons |\n|---------|-------------|------|------|\n| Feature 1 | Description 1 | Pros 1 | Cons 1 |\n| Feature 2 | Description 2 | Pros 2 | Cons 2 |"),
		)

		// 遍历所有已执行的研究步骤，将执行结果作为观察数据添加到消息中
		for _, step := range state.CurrentPlan.Steps {
			msg = append(msg, schema.UserMessage(fmt.Sprintf("Below are some observations for the research task:\n\n %v", *step.ExecutionRes)))
		}
		variables := map[string]any{
			"locale":              state.Locale,
			"max_step_num":        state.MaxStepNum,
			"max_plan_iterations": state.MaxPlanIterations,
			"CURRENT_TIME":        time.Now().Format("2006-01-02 15:04:05"),
			"user_input":          msg,
		}

		// 构造
		output, err = promptTemp.Format(ctx, variables)
		return err
	})
	return output, err
}

// router 路由到下一个节点
func router(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()

		// 记录报告生成完成的事件，包含完整的报告内容
		slog.Debug("router success, input.Content = %+v", input.Content)

		// 设置流程结束标志，整个多智能体研究流程到此完成
		state.Goto = compose.END
		return nil
	})
	return output, nil
}
