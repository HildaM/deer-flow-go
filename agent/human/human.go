package human

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/hildam/deer-flow-go/entity/consts"
	"github.com/hildam/deer-flow-go/entity/model"
	"github.com/hildam/deer-flow-go/repo/llm"
)

// humanImpl 人工代理
type humanImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewHuman 创建实例
func NewHuman[I, O any](ctx context.Context) *humanImpl[I, O] {
	return &humanImpl[I, O]{
		llm: llm.NewChatModel(ctx),
	}
}

// NewGraph 创建任务图
func (h *humanImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	// 创建新的DAG图实例
	graph := compose.NewGraph[I, O]()

	// 添加节点
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(router))

	// 构建线性工作流：开始 → 路由决策 → 结束
	graph.AddEdge(compose.START, "router")
	graph.AddEdge("router", compose.END)
	return consts.Human, graph, compose.WithNodeName(consts.Human)
}

// router 路由决策
func router(ctx context.Context, input string, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 延迟执行：确保在函数返回前设置输出值并清理中断反馈状态
		defer func() {
			output = state.Goto          // 将下一步流向作为输出返回
			state.InterruptFeedback = "" // 清理中断反馈状态，避免影响后续流程
		}()

		// 默认流向：返回调度中心进行下一步决策
		state.Goto = consts.ResearchTeam

		// 关键逻辑：检查计划是否需要人工确认
		if !state.AutoAcceptedPlan {
			// 根据用户的中断反馈决定具体流向
			switch state.InterruptFeedback {
			case consts.AcceptPlan:
				// 用户接受当前计划，继续执行（保持默认流向ResearchTeam）
				return nil
			case consts.EditPlan:
				// 用户要求修改计划，流向Planner重新规划
				state.Goto = consts.Planner
				return nil
			default:
				// 其他情况或无有效反馈，中断并重新运行等待用户输入
				return compose.InterruptAndRerun
			}
		}

		// 计划已自动接受，直接返回调度中心继续执行
		state.Goto = consts.ResearchTeam
		return nil
	})
	return output, err
}
