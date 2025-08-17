package researcher

import (
	"context"

	"github.com/HildaM/logs/slog"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/hildam/deer-flow-go/entity/consts"
	"github.com/hildam/deer-flow-go/entity/model"
	"github.com/hildam/deer-flow-go/repo/llm"
)

// singleResearcherImpl 研究团队。这是整个多智能体系统的调度中心，负责根据当前状态和计划步骤决定下一个执行的智能体
type researcherTeamImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewResearcherTeam 创建实例
func NewResearcherTeam[I, O any](ctx context.Context) *researcherTeamImpl[I, O] {
	return &researcherTeamImpl[I, O]{
		llm: llm.NewChatModel(ctx),
	}
}

// NewGraph 创建任务图
func (r *researcherTeamImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	// 创建图实例
	graph := compose.NewGraph[I, O]()

	// 添加节点
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(teamRouter))

	// 构造关联
	graph.AddEdge(compose.START, "router")
	graph.AddEdge("router", compose.END)

	return consts.ResearchTeam, graph, compose.WithNodeName(consts.ResearchTeam)
}

// teamRouter 核心路由决策函数. 整个多智能体系统的调度中心，负责根据当前状态和计划步骤决定下一个执行的智能体
func teamRouter(ctx context.Context, input string, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(ctx context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()

		// 默认先去 Planner，用于计划指定
		state.Goto = consts.Planner

		// 如果当前没有计划，则先进行规划
		if state.CurrentPlan == nil {
			slog.Debug("router: current plan is nil, goto planner")
			return nil
		}

		// 遍历计划中的所有步骤，寻找第一个未执行的步骤
		for idx, step := range state.CurrentPlan.Steps {
			// 跳过已经执行完成的步骤
			if step.ExecutionRes != nil {
				continue
			}

			slog.Debug("router debug, research team current step: %v, step index: %v", step, idx)

			// 根据计划类型选择响应的节点
			switch step.StepType {
			case model.Research:
				state.Goto = consts.Researcher
				return nil
			case model.Processing:
				state.Goto = consts.Coder
				return nil
			}
		}

		// 所有步骤都已执行完成，检查是否需要生成最终报告
		if state.PlanIterations >= state.MaxPlanIterations {
			// 达到最大迭代次数，分派给Reporter生成最终报告
			state.Goto = consts.Reporter
			return nil
		}
		// 未达到最大迭代次数，返回Planner进行计划优化
		return nil
	})
	return output, err
}
