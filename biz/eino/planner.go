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
	"encoding/json"
	"fmt"
	"time"

	"github.com/RanFeng/ilog"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/hildam/deer-flow-go/biz/consts"
	"github.com/hildam/deer-flow-go/biz/infra"
	"github.com/hildam/deer-flow-go/biz/model"
)

// loadPlannerMsg 是Planner的load节点处理函数，负责加载计划生成的提示词模板
// 这是Planner工作流程的第一步：load → agent → router
// 根据是否启用背景调查，动态构建不同的提示词模板
// 参数:
//   - ctx: 上下文
//   - name: 提示词模板名称（通常是"planner"）
//   - opts: 可选参数（未使用）
// 返回:
//   - output: 格式化后的消息列表，包含系统提示词、用户输入和可选的背景调查结果
//   - err: 错误信息
func loadPlannerMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	// 使用ProcessState处理状态，确保状态的线程安全访问和修改
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 从基础设施层获取提示词模板（通常是planner.md）
		sysPrompt, err := infra.GetPromptTemplate(ctx, name)
		if err != nil {
			ilog.EventInfo(ctx, "get prompt template fail")
			return err
		}

		// 根据是否启用背景调查和是否有调查结果，构建不同的提示词模板
		var promptTemp *prompt.DefaultChatTemplate
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
			"locale":              state.Locale,              // 用户语言设置
			"max_step_num":        state.MaxStepNum,          // 最大步骤数
			"max_plan_iterations": state.MaxPlanIterations,   // 最大计划迭代次数
			"CURRENT_TIME":        time.Now().Format("2006-01-02 15:04:05"), // 当前时间
			"user_input":          state.Messages,            // 用户输入消息
		}
		// 使用变量格式化提示词模板，生成最终的消息列表
		output, err = promptTemp.Format(ctx, variables)
		return err
	})
	return output, err
}

// routerPlanner 是Planner的router节点处理函数，负责解析AI生成的计划并决定下一步路由
// 这是Planner工作流程的最后一步：load → agent → router
// 根据计划生成结果和上下文充分性，决定是直接生成报告还是需要人工反馈
// 参数:
//   - ctx: 上下文
//   - input: 来自agent节点的消息，包含JSON格式的计划内容
//   - opts: 可选参数（未使用）
// 返回:
//   - output: 下一个要执行的节点名称
//   - err: 错误信息
func routerPlanner(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	// 使用ProcessState处理状态，确保状态的线程安全访问和修改
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
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
		err = json.Unmarshal([]byte(input.Content), state.CurrentPlan)
		if err != nil {
			// 计划解析失败的处理逻辑
			ilog.EventInfo(ctx, "gen_plan_fail", "input.Content", input.Content, "err", err)
			// 如果已经有过计划迭代，直接跳转到Reporter生成报告
			if state.PlanIterations > 0 {
				state.Goto = consts.Reporter
				return nil
			}
			// 首次失败则结束流程
			return nil
		}
		// 计划生成成功，记录日志并增加迭代计数
		ilog.EventInfo(ctx, "gen_plan_ok", "plan", state.CurrentPlan)
		state.PlanIterations++
		// 检查计划是否包含足够的上下文信息
		if state.CurrentPlan.HasEnoughContext {
			// 如果上下文充分，直接跳转到Reporter生成最终报告
			state.Goto = consts.Reporter
			return nil
		}

		// 如果上下文不充分，需要人工反馈来完善计划
		state.Goto = consts.Human // TODO: 改成 human_feedback
		return nil
	})
	return output, nil
}

// NewPlanner 创建并返回Planner智能体的DAG图结构
// Planner是多智能体系统中的计划生成节点，负责根据用户输入和背景调查结果生成执行计划
// 它采用标准的三段式处理模式：load → agent → router
// 泛型参数:
//   - I: 输入类型
//   - O: 输出类型
// 参数:
//   - ctx: 上下文
// 返回:
//   - *compose.Graph[I, O]: 构建好的DAG图
func NewPlanner[I, O any](ctx context.Context) *compose.Graph[I, O] {
	// 创建新的DAG图实例
	cag := compose.NewGraph[I, O]()

	// 构建DAG图的三个核心节点：
	// 1. load节点：加载提示词模板，整合用户输入和背景调查结果
	_ = cag.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadPlannerMsg))
	// 2. agent节点：使用专门的计划模型生成JSON格式的执行计划
	_ = cag.AddChatModelNode("agent", infra.PlanModel)
	// 3. router节点：解析生成的计划，根据上下文充分性决定下一步路由
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(routerPlanner))

	// 构建节点间的连接关系，形成线性的处理流程
	_ = cag.AddEdge(compose.START, "load")    // 开始 → load
	_ = cag.AddEdge("load", "agent")         // load → agent
	_ = cag.AddEdge("agent", "router")       // agent → router
	_ = cag.AddEdge("router", compose.END)   // router → 结束
	return cag
}
