/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package eino

import (
	"context"

	"github.com/RanFeng/ilog"
	"github.com/cloudwego/eino/compose"

	"github.com/hildam/deer-flow-go/biz/consts"
	"github.com/hildam/deer-flow-go/biz/model"
)

// routerResearchTeam 研究团队的核心路由决策函数
// 这是整个多智能体系统的调度中心，负责根据当前状态和计划步骤决定下一个执行的智能体
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - input: 输入字符串（当前未使用）
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 下一个要执行的智能体名称（Planner/Researcher/Coder/Reporter）
//   - err: 错误信息
//
// 核心决策逻辑:
// 1. 默认流向Planner，用于重新规划或优化计划
// 2. 如果当前没有计划，直接返回Planner进行初始规划
// 3. 遍历计划中的所有步骤，寻找第一个未执行的步骤：
//    - Research类型步骤 → 分派给Researcher智能体
//    - Processing类型步骤 → 分派给Coder智能体
// 4. 如果所有步骤都已执行完成，检查计划迭代次数：
//    - 达到最大迭代次数 → 分派给Reporter生成最终报告
//    - 未达到最大次数 → 返回Planner进行计划优化
func routerResearchTeam(ctx context.Context, input string, opts ...any) (output string, err error) {
	//ilog.EventInfo(ctx, "routerResearchTeam", "input", input)
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()
		// 默认流向Planner，用于计划制定或优化
		state.Goto = consts.Planner
		// 如果当前没有计划，需要先进行规划
		if state.CurrentPlan == nil {
			return nil
		}
		// 遍历计划中的所有步骤，寻找第一个未执行的步骤
		for i, step := range state.CurrentPlan.Steps {
			// 跳过已经执行完成的步骤
			if step.ExecutionRes != nil {
				continue
			}
			// 记录当前要执行的步骤信息
			ilog.EventInfo(ctx, "research_team_step", "step", step, "index", i)
			// 根据步骤类型分派给相应的智能体
			switch step.StepType {
			case model.Research:
				// 研究类型步骤分派给Researcher智能体
				state.Goto = consts.Researcher
				return nil
			case model.Processing:
				// 处理类型步骤分派给Coder智能体
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
	return output, nil
}

// NewResearchTeamNode 创建研究团队的DAG工作流图
// ResearchTeam是整个多智能体系统的调度中心，负责协调各个智能体的执行顺序
//
// 参数:
//   - ctx: 上下文对象
//
// 返回值:
//   - *compose.Graph[I, O]: 研究团队的DAG工作流图
//
// 工作流程:
// START → router → END
//
// 节点说明:
// 1. router: 唯一的路由决策节点，负责分析当前状态并决定下一个执行的智能体
//
// 在多智能体协作中的作用:
// ResearchTeam（当前）→ Planner/Researcher/Coder/Reporter
// 作为系统的调度中心，根据计划执行情况动态分派任务给不同的专业智能体：
// - 需要规划时 → Planner
// - 需要研究时 → Researcher  
// - 需要代码处理时 → Coder
// - 需要生成报告时 → Reporter
func NewResearchTeamNode[I, O any](ctx context.Context) *compose.Graph[I, O] {
	cag := compose.NewGraph[I, O]()
	// 添加路由决策节点，作为整个系统的调度中心
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(routerResearchTeam))

	// 构建简单的线性流程：START → router → END
	_ = cag.AddEdge(compose.START, "router")
	_ = cag.AddEdge("router", compose.END)

	return cag
}
