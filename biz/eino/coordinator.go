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
	"time"

	"github.com/RanFeng/ilog"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/hildam/deer-flow-go/biz/consts"
	"github.com/hildam/deer-flow-go/biz/infra"
	"github.com/hildam/deer-flow-go/biz/model"
)

// loadMsg 是Coordinator的load节点处理函数，负责加载和格式化提示词模板
// 这是Coordinator工作流程的第一步：load → agent → router
// 参数:
//   - ctx: 上下文
//   - name: 提示词模板名称（通常是"coordinator"）
//   - opts: 可选参数（未使用）
// 返回:
//   - output: 格式化后的消息列表，包含系统提示词和用户输入占位符
//   - err: 错误信息
func loadMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	// 使用ProcessState处理状态，确保状态的线程安全访问和修改
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 从基础设施层获取提示词模板（通常是coordinator.md）
		sysPrompt, err := infra.GetPromptTemplate(ctx, name)
		if err != nil {
			ilog.EventInfo(ctx, "get prompt template fail")
			return err
		}

		// 构建Jinja2格式的提示词模板，包含系统消息和用户输入占位符
		promptTemp := prompt.FromMessages(schema.Jinja2,
			schema.SystemMessage(sysPrompt),
			schema.MessagesPlaceholder("user_input", true),
		)

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

// router 是Coordinator的router节点处理函数，负责解析AI模型的工具调用结果并决定下一步路由
// 这是Coordinator工作流程的最后一步：load → agent → router
// 参数:
//   - ctx: 上下文
//   - input: 来自agent节点的消息，可能包含工具调用
//   - opts: 可选参数（未使用）
// 返回:
//   - output: 下一个要执行的节点名称
//   - err: 错误信息
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
			_ = json.Unmarshal([]byte(input.ToolCalls[0].Function.Arguments), &argMap)
			// 从工具参数中提取并保存用户的语言设置
			state.Locale, _ = argMap["locale"]
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

// NewCAgent 创建并返回Coordinator智能体的DAG图结构
// Coordinator是整个多智能体系统的入口节点，负责任务分发和语言检测
// 它采用标准的三段式处理模式：load → agent → router
// 泛型参数:
//   - I: 输入类型
//   - O: 输出类型
// 参数:
//   - ctx: 上下文
// 返回:
//   - *compose.Graph[I, O]: 构建好的DAG图
func NewCAgent[I, O any](ctx context.Context) *compose.Graph[I, O] {
	// 创建新的DAG图实例
	cag := compose.NewGraph[I, O]()

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
	coorModel, _ := infra.ChatModel.WithTools([]*schema.ToolInfo{hand_to_planner})

	// 构建DAG图的三个核心节点：
	// 1. load节点：加载提示词模板和准备输入数据
	_ = cag.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadMsg))
	// 2. agent节点：执行AI模型推理，检测语言并决定是否移交给Planner
	_ = cag.AddChatModelNode("agent", coorModel)
	// 3. router节点：解析工具调用结果，决定下一步路由（BackgroundInvestigator或Planner）
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(router))

	// 构建节点间的连接关系，形成线性的处理流程
	_ = cag.AddEdge(compose.START, "load")    // 开始 → load
	_ = cag.AddEdge("load", "agent")         // load → agent
	_ = cag.AddEdge("agent", "router")       // agent → router
	_ = cag.AddEdge("router", compose.END)   // router → 结束
	return cag
}
