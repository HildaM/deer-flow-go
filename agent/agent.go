package agent

import (
	"context"
	"fmt"

	"github.com/HildaM/logs/slog"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/hildam/deer-flow-go/agent/coder"
	"github.com/hildam/deer-flow-go/agent/coordinator"
	"github.com/hildam/deer-flow-go/agent/human"
	"github.com/hildam/deer-flow-go/agent/investigator"
	"github.com/hildam/deer-flow-go/agent/planner"
	"github.com/hildam/deer-flow-go/agent/repoter"
	"github.com/hildam/deer-flow-go/agent/researcher"
	"github.com/hildam/deer-flow-go/entity/conf"
	"github.com/hildam/deer-flow-go/entity/consts"
	"github.com/hildam/deer-flow-go/entity/model"
	"github.com/hildam/deer-flow-go/repo/checkpoint"
)

// Agent 定义了一个代理接口，用于创建和管理代理实例
type Agent[I, O any] interface {
	// NewGraphNode 获取代理节点
	NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt)
}

// BuildAgentGraph 用于构建代理图
func BuildAgentGraph[I, O any](ctx context.Context, userMessage []*schema.Message) (compose.Runnable[I, O], error) {
	// 初始化状态
	stateGenFunc := func(ctx context.Context) *model.State {
		return &model.State{
			MaxPlanIterations: conf.GetCfg().Setting.MaxPlanIterations,
			AutoAcceptedPlan:  true,
			MaxStepNum:        conf.GetCfg().Setting.TotalMaxRound,
			Messages:          userMessage,
			Goto:              consts.Coordinator,
		}
	}

	// 创建 Agent 流程图
	graph := compose.NewGraph[I, O](
		compose.WithGenLocalState(stateGenFunc),
	)

	// 定义agent实例映射，确保节点名字与实例严格对应
	agentInstances := map[string]Agent[I, O]{
		consts.Coordinator:            coordinator.NewCoordinator[I, O](ctx),
		consts.Planner:                planner.NewPlanner[I, O](ctx),
		consts.Reporter:               repoter.NewRepoter[I, O](ctx),
		consts.Researcher:             researcher.NewSingleResearcher[I, O](ctx),
		consts.ResearchTeam:           researcher.NewResearcherTeam[I, O](ctx),
		consts.Coder:                  coder.NewCoder[I, O](ctx),
		consts.BackgroundInvestigator: investigator.NewInvestigator[I, O](ctx),
		consts.Human:                  human.NewHuman[I, O](ctx),
	}

	// 构造任务图 - 使用映射确保名字与实例对应
	for agentName, agentInstance := range agentInstances {
		key, node, nameOption := agentInstance.NewGraphNode(ctx)
		// 验证返回的key与预期的agentName一致
		if key != agentName {
			slog.Error("Agent key mismatch: expected %s, got %s", agentName, key)
			return nil, fmt.Errorf("agent key mismatch: expected %s, got %s", agentName, key)
		}

		// 添加节点
		graph.AddGraphNode(key, node, nameOption)
	}

	// 构造branch - 只为实际存在的agent创建分支
	for agentName := range agentInstances {
		graph.AddBranch(agentName,
			compose.NewGraphBranch(routeToNextAgent, getAgentGraphMap()))
	}

	// 构造起始边
	graph.AddEdge(compose.START, consts.Coordinator)

	// 编译图
	runnable, err := graph.Compile(ctx,
		compose.WithGraphName(consts.GraphName),
		compose.WithNodeTriggerMode(compose.AnyPredecessor),
		compose.WithCheckPointStore(checkpoint.NewCheckPoint()), // 全局状态存储点
	)
	if err != nil {
		slog.Error("BuildAgentGraph failed, err = %v", err)
		return nil, err
	}
	return runnable, nil
}

// routeToNextAgent 根据状态中的Goto字段路由到下一个代理节点
// 该函数从状态中读取目标代理名称，实现代理间的流程控制转移
func routeToNextAgent(ctx context.Context, input string) (next string, err error) {
	defer func() {
		slog.Info("route_to_next_agent info, input = %s, next = %s", input, next)
	}()
	_ = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		next = state.Goto
		return nil
	})
	return next, nil
}

// getAgentGraphMap 返回所有可用的agent节点及其启用状态
// 注意：这个函数应该与BuildAgentGraph中的agentInstances保持一致
func getAgentGraphMap() map[string]bool {
	return map[string]bool{
		consts.Coordinator:            true, // 任务协调者，负责整体任务调度和协调
		consts.Planner:                true, // 计划者，负责制定和优化执行计划
		consts.Reporter:               true, // 报告者，负责生成和整理报告内容
		consts.Researcher:             true, // 研究者，负责信息收集和分析
		consts.ResearchTeam:           true, // 研究团队，负责协调多个研究任务
		consts.Coder:                  true, // 代码生成者，负责编写和优化代码
		consts.BackgroundInvestigator: true, // 背景调查者，负责深度背景信息挖掘
		consts.Human:                  true, // 人工代理，负责人工干预和反馈
		compose.END:                   true, // 流程结束节点，标记任务完成
	}
}
