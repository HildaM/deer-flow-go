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
	"fmt"
	"strings"
	"time"

	"github.com/RanFeng/ilog"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/hildam/deer-flow-go/biz/consts"
	"github.com/hildam/deer-flow-go/biz/infra"
	"github.com/hildam/deer-flow-go/biz/model"
)

// loadCoderMsg 为Coder智能体加载消息和提示词模板
// 这是Coder工作流程中的第一个环节，负责准备执行代码生成任务所需的输入信息
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - name: 提示词模板名称，通常为"coder"，用于获取代码生成任务的系统提示词
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 格式化后的消息列表，包含系统提示词、当前代码生成步骤的任务信息
//   - err: 错误信息
//
// 核心逻辑:
// 1. 获取Coder的系统提示词模板，定义代码生成任务的执行方式和输出格式
// 2. 从当前计划中找到第一个未执行的代码生成步骤
// 3. 构建包含任务标题、描述和语言设置的用户消息
// 4. 设置模板变量并格式化最终的消息列表
func loadCoderMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 获取Coder的系统提示词模板，定义代码生成任务的执行方式
		sysPrompt, err := infra.GetPromptTemplate(ctx, name)
		if err != nil {
			ilog.EventError(ctx, err, "get prompt template error")
			return err
		}

		// 创建Jinja2模板，包含系统提示词和用户输入占位符
		promptTemp := prompt.FromMessages(schema.Jinja2,
			schema.SystemMessage(sysPrompt),
			schema.MessagesPlaceholder("user_input", true),
		)

		// 从当前计划中找到第一个未执行的代码生成步骤
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

		// 构建消息列表，包含当前代码生成步骤的详细信息
		msg := []*schema.Message{}
		// 添加当前代码生成步骤的任务信息（标题、描述、语言设置）
		msg = append(msg,
			schema.UserMessage(fmt.Sprintf("#Task\n\n##title\n\n %v \n\n##description\n\n %v \n\n##locale\n\n %v", curStep.Title, curStep.Description, state.Locale)),
		)
		// 设置模板变量，包含系统配置和当前任务信息
		variables := map[string]any{
			"locale":              state.Locale,              // 语言设置
			"max_step_num":        state.MaxStepNum,          // 最大步骤数
			"max_plan_iterations": state.MaxPlanIterations,   // 最大计划迭代次数
			"CURRENT_TIME":        time.Now().Format("2006-01-02 15:04:05"), // 当前时间
			"user_input":          msg,                       // 用户输入消息
		}
		// 格式化模板并生成最终的消息列表
		output, err = promptTemp.Format(ctx, variables)
		return err
	})
	return output, err
}

// routerCoder Coder智能体的路由决策函数
// 这是Coder工作流程中的最后一个环节，负责保存代码生成结果并决定下一步流向
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - input: Coder智能体生成的代码结果消息
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 下一个节点的名称，固定为consts.ResearchTeam，返回调度中心
//   - err: 错误信息
//
// 核心逻辑:
// 1. 将代码生成结果保存到当前执行步骤的ExecutionRes字段中
// 2. 记录代码生成任务完成的日志，包含更新后的计划状态
// 3. 设置流向为ResearchTeam，返回调度中心进行下一步决策
func routerCoder(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	//ilog.EventInfo(ctx, "routerResearcher", "input", input)
	last := input
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()
		// 将代码生成结果保存到第一个未执行步骤的ExecutionRes字段中
		for i, step := range state.CurrentPlan.Steps {
			if step.ExecutionRes == nil {
				// 克隆代码生成结果内容并保存
				str := strings.Clone(last.Content)
				state.CurrentPlan.Steps[i].ExecutionRes = &str
				break
			}
		}
		// 记录代码生成任务完成的事件，包含更新后的计划状态
		ilog.EventInfo(ctx, "coder_end", "plan", state.CurrentPlan)
		// 返回调度中心，由ResearchTeam决定下一步执行哪个智能体
		state.Goto = consts.ResearchTeam
		return nil
	})
	return output, nil
}

// modifyCoderfunc 消息内容修改器，用于处理Coder输入消息的长度限制
// 这是Coder智能体配置中的MessageModifier，确保输入内容不超过模型的处理限制
//
// 参数:
//   - ctx: 上下文对象，用于日志记录
//   - input: 输入的消息列表
//
// 返回值:
//   - []*schema.Message: 处理后的消息列表
//
// 核心逻辑:
// 1. 遍历所有输入消息，检查每条消息的内容长度
// 2. 对于超过50000字符限制的消息，截取后半部分内容
// 3. 记录处理过程中的警告和统计信息
// 4. 确保输入内容适合大语言模型的上下文窗口限制
func modifyCoderfunc(ctx context.Context, input []*schema.Message) []*schema.Message {
	sum := 0
	maxLimit := 50000 // 单条消息的最大字符数限制
	for i := range input {
		// 跳过空消息
		if input[i] == nil {
			ilog.EventWarn(ctx, "modify_inputfunc_nil", "input", input[i])
			continue
		}
		l := len(input[i].Content)
		// 如果消息内容超过限制，截取后半部分（保留最新信息）
		if l > maxLimit {
			ilog.EventWarn(ctx, "modify_inputfunc_clip", "raw_len", l)
			input[i].Content = input[i].Content[l-maxLimit:]
		}
		sum += len(input[i].Content)
	}
	// 记录处理后的统计信息
	ilog.EventInfo(ctx, "modify_inputfunc", "sum", sum, "input_len", input)
	return input
}

// NewCoder 创建Coder智能体的工作流图
// 这是Coder智能体的构造函数，负责配置代码生成智能体并构建完整的工作流程
//
// 参数:
//   - ctx: 上下文对象，用于获取聊天模型等资源
//
// 返回值:
//   - *compose.Graph[I, O]: 配置完成的Coder工作流图
//
// 工作流程:
// START → load → agent → router → END
//
// 核心逻辑:
// 1. load: 加载代码生成任务的提示词和当前步骤信息
// 2. agent: 使用ReAct智能体执行代码生成任务，配置Python工具和消息长度限制
// 3. router: 保存代码生成结果并返回调度中心
// 4. 构建线性的代码生成工作流，确保任务按序执行
func NewCoder[I, O any](ctx context.Context) *compose.Graph[I, O] {
	// 创建新的工作流图实例
	cag := compose.NewGraph[I, O]()

	// 使用修复版本的MCP工具，并过滤出python相关工具
	allTools, err := GetFixedMCPTools(ctx)
	if err != nil {
		ilog.EventError(ctx, err, "failed_to_get_fixed_mcp_tools")
		allTools = []tool.BaseTool{} // 如果失败，使用空工具列表
	}

	// 过滤出python相关的工具，为代码生成任务提供专业工具支持
	researchTools := []tool.BaseTool{}
	for _, t := range allTools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		// 检查工具名称是否包含python相关关键词
		if strings.Contains(strings.ToLower(info.Name), "python") ||
			strings.Contains(strings.ToLower(info.Desc), "python") {
			researchTools = append(researchTools, t)
		}
	}
	ilog.EventDebug(ctx, "coder_end", "coder_tools", researchTools)

	// 创建ReAct智能体，配置代码生成相关的参数和工具
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		MaxStep:               40,                                           // 最大执行步骤数
		ToolCallingModel:      infra.ChatModel,                             // 工具调用模型
		ToolsConfig:           compose.ToolsNodeConfig{Tools: researchTools}, // Python相关工具配置
		MessageModifier:       modifyCoderfunc,                             // 消息长度限制处理器
		StreamToolCallChecker: toolCallChecker,                             // 流式工具调用检查器
	})
	if err != nil {
		panic(err)
	}

	// 将智能体包装为Lambda节点
	agentLambda, err := compose.AnyLambda(agent.Generate, agent.Stream, nil, nil)
	if err != nil {
		panic(err)
	}

	// 添加工作流节点：消息加载 → 智能体执行 → 路由决策
	_ = cag.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadCoderMsg))
	_ = cag.AddLambdaNode("agent", agentLambda)
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(routerCoder))

	// 构建线性工作流：加载 → 生成 → 路由 → 结束
	_ = cag.AddEdge(compose.START, "load")
	_ = cag.AddEdge("load", "agent")
	_ = cag.AddEdge("agent", "router")
	_ = cag.AddEdge("router", compose.END)
	return cag
}
