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
	"io"
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

// loadResearcherMsg 为Researcher智能体加载消息和提示词模板
// 这是Researcher工作流程中的第一个环节，负责准备执行具体研究任务所需的输入信息
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - name: 提示词模板名称，通常为"researcher"，用于获取研究任务的系统提示词
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 格式化后的消息列表，包含系统提示词、当前研究步骤的任务信息
//   - err: 错误信息
//
// 核心逻辑:
// 1. 获取Researcher的系统提示词模板，定义研究任务的执行方式和输出格式
// 2. 从当前计划中找到第一个未执行的研究步骤
// 3. 构建包含任务标题、描述和语言设置的用户消息
// 4. 添加引用格式指导，要求在文末统一列出参考资料
// 5. 设置模板变量并格式化最终的消息列表
func loadResearcherMsg(ctx context.Context, name string, opts ...any) (output []*schema.Message, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 获取Researcher的系统提示词模板，定义研究任务的执行方式
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

		// 从当前计划中找到第一个未执行的研究步骤
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

		// 构建消息列表，包含当前研究步骤的详细信息
		msg := []*schema.Message{}
		// 添加当前研究步骤的任务信息（标题、描述、语言设置）
		msg = append(msg,
			schema.UserMessage(fmt.Sprintf("#Task\n\n##title\n\n %v \n\n##description\n\n %v \n\n##locale\n\n %v", curStep.Title, curStep.Description, state.Locale)),
			// 添加引用格式指导，要求在文末统一列出参考资料而非内联引用
			schema.SystemMessage("IMPORTANT: DO NOT include inline citations in the text. Instead, track all sources and include a References section at the end using link reference format. Include an empty line between each citation for better readability. Use this format for each reference:\n- [Source Title](URL)\n\n- [Another Source](URL)"),
		)
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

// routerResearcher Researcher智能体的路由决策函数
// 这是Researcher工作流程中的最后一个环节，负责保存研究结果并决定下一步流向
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - input: Researcher智能体生成的研究结果消息
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 下一个节点的名称，固定为consts.ResearchTeam，返回调度中心
//   - err: 错误信息
//
// 核心逻辑:
// 1. 将研究结果保存到当前执行步骤的ExecutionRes字段中
// 2. 记录研究任务完成的日志，包含更新后的计划状态
// 3. 设置流向为ResearchTeam，返回调度中心进行下一步决策
func routerResearcher(ctx context.Context, input *schema.Message, opts ...any) (output string, err error) {
	//ilog.EventInfo(ctx, "routerResearcher", "input", input)
	last := input
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()
		// 将研究结果保存到第一个未执行步骤的ExecutionRes字段中
		for i, step := range state.CurrentPlan.Steps {
			if step.ExecutionRes == nil {
				// 克隆研究结果内容并保存
				str := strings.Clone(last.Content)
				state.CurrentPlan.Steps[i].ExecutionRes = &str
				break
			}
		}
		// 记录研究任务完成的事件，包含更新后的计划状态
		ilog.EventInfo(ctx, "researcher_end", "plan", state.CurrentPlan)
		// 返回调度中心，由ResearchTeam决定下一步执行哪个智能体
		state.Goto = consts.ResearchTeam
		return nil
	})
	return output, nil
}

// modifyInputfunc 消息内容修改器，用于处理输入消息的长度限制
// 这是Researcher智能体配置中的MessageModifier，确保输入内容不超过模型的处理限制
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
func modifyInputfunc(ctx context.Context, input []*schema.Message) []*schema.Message {
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
	ilog.EventInfo(ctx, "modify_inputfunc", "sum", sum, "input_len", len(input))
	return input
}

// toolCallChecker 工具调用检测器，用于检查流式响应中是否包含工具调用
// 这是Researcher智能体配置中的StreamToolCallChecker，用于ReAct模式的工具调用检测
//
// 参数:
//   - _: 上下文对象（当前未使用）
//   - sr: 消息流读取器
//
// 返回值:
//   - bool: 是否检测到工具调用
//   - error: 错误信息
//
// 核心逻辑:
// 1. 遍历流式响应中的所有消息
// 2. 检查每条消息是否包含工具调用（ToolCalls）
// 3. 一旦发现工具调用，立即返回true
// 4. 如果流结束仍未发现工具调用，返回false
// 5. 用于ReAct智能体判断是否需要执行工具调用步骤
func toolCallChecker(_ context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
	defer sr.Close()

	// 遍历流式响应中的所有消息
	for {
		msg, err := sr.Recv()
		if err == io.EOF {
			// 流结束，未发现工具调用
			return false, nil
		}
		if err != nil {
			return false, err
		}

		// 检查当前消息是否包含工具调用
		if len(msg.ToolCalls) > 0 {
			return true, nil
		}
	}
}

// NewResearcher 创建Researcher智能体的工作流图
// 这是Researcher智能体的构造函数，负责配置ReAct智能体并构建完整的工作流程
//
// 参数:
//   - ctx: 上下文对象，用于工具初始化和配置管理
//
// 返回值:
//   - *compose.Graph[I, O]: 配置完成的Researcher工作流图
//
// 核心逻辑:
// 1. 初始化MCP工具集合，为Researcher提供搜索、分析等能力
// 2. 创建ReAct智能体，配置模型、工具、消息修改器和工具调用检测器
// 3. 构建工作流图：load → agent → router
// 4. 设置节点间的连接关系，形成完整的研究任务执行流程
//
// 工作流程说明:
// - load: 加载研究任务和提示词
// - agent: 执行具体的研究工作（ReAct模式，支持工具调用）
// - router: 保存研究结果并决定下一步流向
func NewResearcher[I, O any](ctx context.Context) *compose.Graph[I, O] {
	cag := compose.NewGraph[I, O]()

	// 使用MCP工具，为Researcher提供搜索、分析等能力
	researchTools, err := infra.GetMCPTools(ctx)
	if err != nil {
		ilog.EventError(ctx, err, "failed_to_get_mcp_tools")
		researchTools = []tool.BaseTool{} // 如果失败，使用空工具列表
	}
	ilog.EventDebug(ctx, "researcher_end", "research_tools", len(researchTools))

	// 创建ReAct智能体，支持工具调用和多步推理
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		MaxStep:               40,                                              // 最大推理步数
		ToolCallingModel:      infra.ChatModel,                                // 大语言模型配置
		ToolsConfig:           compose.ToolsNodeConfig{Tools: researchTools}, // 可用工具列表
		MessageModifier:       modifyInputfunc,                                // 消息长度限制处理器
		StreamToolCallChecker: toolCallChecker,                                // 工具调用检测器
	})
	if err != nil {
		panic(err)
	}

	// 将智能体包装为Lambda函数，支持生成和流式处理
	agentLambda, err := compose.AnyLambda(agent.Generate, agent.Stream, nil, nil)
	if err != nil {
		panic(err)
	}

	// 构建Researcher工作流图
	// 添加消息加载节点
	_ = cag.AddLambdaNode("load", compose.InvokableLambdaWithOption(loadResearcherMsg))
	// 添加Researcher智能体节点
	_ = cag.AddLambdaNode("agent", agentLambda)
	// 添加路由决策节点
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(routerResearcher))

	// 设置工作流的执行顺序
	_ = cag.AddEdge(compose.START, "load")   // 开始 → 加载消息
	_ = cag.AddEdge("load", "agent")        // 加载消息 → 执行研究
	_ = cag.AddEdge("agent", "router")      // 执行研究 → 路由决策
	_ = cag.AddEdge("router", compose.END)  // 路由决策 → 结束
	return cag
}
