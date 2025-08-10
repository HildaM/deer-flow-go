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
	"time"

	"github.com/RanFeng/ilog"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/hildam/deer-flow-go/biz/infra"
	"github.com/hildam/deer-flow-go/biz/model"
)

// loadReporterMsg 为Reporter智能体加载消息和提示词模板
// 这是Reporter工作流程中的第一个环节，负责准备生成最终研究报告所需的所有输入信息
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - name: 提示词模板名称，通常为"reporter"，用于获取报告生成的系统提示词
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 格式化后的消息列表，包含系统提示词、研究任务信息和所有步骤的执行结果
//   - err: 错误信息
//
// 核心逻辑:
// 1. 获取Reporter的系统提示词模板，定义报告的格式和要求
// 2. 构建用户消息，包含当前计划的标题和描述
// 3. 添加报告格式指导，强调使用Markdown表格和引用格式
// 4. 遍历所有已执行的研究步骤，将执行结果作为观察数据添加到消息中
// 5. 设置模板变量（语言、时间等）并格式化最终的消息列表
func loadReporterMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 获取Reporter的系统提示词模板，定义报告生成的格式和要求
		sysPrompt, err := infra.GetPromptTemplate(ctx, name)
		if err != nil {
			ilog.EventInfo(ctx, "get prompt template fail")
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
		output, err = promptTemp.Format(ctx, variables)
		return err
	})
	return output, err
}

// routerReporter Reporter智能体的路由决策函数
// 这是Reporter工作流程中的最后一个环节，负责处理生成的报告并决定下一步流向
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - input: Reporter智能体生成的报告消息
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 下一个节点的名称，固定为compose.END，表示整个研究流程结束
//   - err: 错误信息
//
// 核心逻辑:
// 1. 记录报告生成完成的日志，包含完整的报告内容
// 2. 设置状态流向为END，标志着整个多智能体研究流程的结束
// 3. 此时所有研究任务已完成，最终报告已生成
func routerReporter(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()
		// 记录报告生成完成的事件，包含完整的报告内容
		ilog.EventInfo(ctx, "report_end", "report", input.Content)
		// 设置流程结束标志，整个多智能体研究流程到此完成
		state.Goto = compose.END
		return nil
	})
	return output, nil
}

// NewReporter 创建Reporter智能体的DAG工作流图
// Reporter是多智能体系统中的最后一个环节，负责整合所有研究结果并生成最终报告
//
// 参数:
//   - ctx: 上下文对象
//
// 返回值:
//   - *compose.Graph[I, O]: Reporter的DAG工作流图
//
// 工作流程:
// START → load → agent → router → END
//
// 节点说明:
// 1. load: 加载消息和提示词，准备报告生成所需的所有输入信息
// 2. agent: 大语言模型节点，基于研究结果生成结构化的最终报告
// 3. router: 路由决策，标记整个研究流程结束
//
// 在多智能体协作中的位置:
// ResearchTeam → Planner → Researcher/Coder → Reporter（当前）
// Reporter接收所有前序智能体的研究成果，生成综合性的最终报告
func NewReporter[I, O any](ctx context.Context) *compose.Graph[I, O] {
	cag := compose.NewGraph[I, O]()

	// 添加消息加载节点，准备报告生成所需的输入信息
	_ = cag.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadReporterMsg))
	// 添加大语言模型节点，负责生成最终的研究报告
	_ = cag.AddChatModelNode("agent", infra.ChatModel)
	// 添加路由节点，处理报告并结束整个流程
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(routerReporter))

	// 构建线性的工作流程：load → agent → router
	_ = cag.AddEdge(compose.START, "load")
	_ = cag.AddEdge("load", "agent")
	_ = cag.AddEdge("agent", "router")
	_ = cag.AddEdge("router", compose.END)
	return cag
}
