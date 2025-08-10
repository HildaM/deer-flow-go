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

	"github.com/cloudwego/eino/compose"

	"github.com/hildam/deer-flow-go/biz/consts"
	"github.com/hildam/deer-flow-go/biz/model"
)

// routerHuman 人工反馈节点的路由决策函数
// 这是人工反馈工作流程中的核心环节，负责处理用户反馈并决定下一步流向
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - input: 用户提供的反馈内容
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 下一个节点的名称
//   - err: 错误信息
//
// 核心逻辑:
// 1. 根据计划是否自动接受来决定处理方式
// 2. 如果计划未自动接受，根据用户反馈决定流向
// 3. 清理中断反馈状态，确保流程正常进行
func routerHuman(ctx context.Context, input string, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 延迟执行：确保在函数返回前设置输出值并清理中断反馈状态
		defer func() {
			output = state.Goto              // 将下一步流向作为输出返回
			state.InterruptFeedback = ""     // 清理中断反馈状态，避免影响后续流程
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

// NewHumanNode 创建人工反馈节点的工作流图
// 这是人工反馈节点的构造函数，负责构建人工交互的工作流程
//
// 参数:
//   - ctx: 上下文对象（当前未使用）
//
// 返回值:
//   - *compose.Graph[I, O]: 配置完成的人工反馈工作流图
//
// 工作流程:
// START → router → END
//
// 核心逻辑:
// 1. 创建包含路由决策节点的简单工作流图
// 2. router节点负责处理人工反馈并决定下一步流向
// 3. 构建线性的执行流程，确保反馈处理的正确性
func NewHumanNode[I, O any](ctx context.Context) *compose.Graph[I, O] {
	// 创建新的工作流图实例
	cag := compose.NewGraph[I, O]()
	// 添加路由决策节点，负责处理人工反馈
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(routerHuman))

	// 构建线性工作流：开始 → 路由决策 → 结束
	_ = cag.AddEdge(compose.START, "router")
	_ = cag.AddEdge("router", compose.END)

	return cag
}
