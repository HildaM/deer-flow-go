package planner

import (
	"context"
	"encoding/json"
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

// plannerImpl 计划者
type plannerImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewPlanner 创建实例
func NewPlanner[I, O any](ctx context.Context) *plannerImpl[I, O] {
	return &plannerImpl[I, O]{
		llm: llm.NewPlanModel(ctx),
	}
}

// NewGraph 创建任务图
func (p *plannerImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	// 创建图实例
	graph := compose.NewGraph[I, O]()

	// 添加节点
	graph.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadMsg))
	graph.AddChatModelNode("agent", p.llm)
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(router))

	// 构造关联
	graph.AddEdge(compose.START, "load")
	graph.AddEdge("load", "agent")
	graph.AddEdge("agent", "router")
	graph.AddEdge("router", compose.END)

	return consts.Planner, graph, compose.WithNodeName(consts.Planner)
}

// loadMsg Planner的load节点处理函数，负责加载计划生成的提示词模板
func loadMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(ctx context.Context, state *model.State) error {
		// 加载模板
		sysPrompt, err := template.GetPromptTemplate(ctx, name)
		if err != nil {
			slog.Error("loadMsg failed, GetPromptTemplate err = %+v", err)
			return err
		}

		// 根据是否启用背景调查和是否有调查结果，构建不同的提示词模板
		promptTemp := &prompt.DefaultChatTemplate{}
		if state.EnableBackgroundInvestigation && len(state.BackgroundInvestigationResults) > 0 {
			// 如果有背景调查结果，将其作为额外的用户消息添加到模板中
			promptTemp = prompt.FromMessages(schema.Jinja2,
				schema.SystemMessage(sysPrompt),
				schema.MessagesPlaceholder("user_input", true),
				schema.UserMessage(fmt.Sprintf("background investigation results of user query: \n %s", state.BackgroundInvestigationResults)),
			)
		} else {
			// 没有背景调查结果时，使用标准的提示词模板
			promptTemp = prompt.FromMessages(schema.Jinja2,
				schema.SystemMessage(sysPrompt),
				schema.MessagesPlaceholder("user_input", true),
			)
		}

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
		return err
	})
	return output, err
}

// router 路由
func router(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(ctx context.Context, state *model.State) error {
		// 使用defer确保output总是被设置为state.Goto的值
		defer func() {
			output = state.Goto
		}()

		// 默认设置为结束
		state.Goto = compose.END
		// 初始化当前计划结构体
		state.CurrentPlan = &model.Plan{}

		// TODO: 修复可能存在的markdown代码块标记问题
		// 尝试将AI生成的JSON格式计划内容解析到CurrentPlan结构体中
		if err := json.Unmarshal([]byte(input.Content), state.CurrentPlan); err != nil {
			// 计划解析失败的处理逻辑
			slog.Error("router failed, Unmarshal err = %+v, input.Content = %+v", err, input.Content)

			// 如果已经有过计划迭代，直接跳转到Reporter生成报告
			if state.PlanIterations > 0 {
				state.Goto = consts.Reporter
				return nil
			}
			// 首次失败则结束流程
			return nil
		}

		// 计划生成成功，记录日志并增加迭代计数
		slog.Debug("router success, input.Content = %+v, state.CurrentPlan = %+v", input.Content, state.CurrentPlan)
		state.PlanIterations++

		// 检查计划是否包含足够的上下文信息
		if state.CurrentPlan.HasEnoughContext {
			// 如果上下文充分，直接跳转到Reporter生成最终报告
			state.Goto = consts.Reporter
			return nil
		}

		// 如果上下文不充分，需要人工反馈来完善计划
		state.Goto = consts.Human
		return nil
	})
	return output, err
}
