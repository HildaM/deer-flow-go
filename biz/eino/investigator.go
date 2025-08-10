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
	"strings"

	"github.com/RanFeng/ilog"
	"github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	"github.com/hildam/deer-flow-go/biz/consts"
	"github.com/hildam/deer-flow-go/biz/infra"
	"github.com/hildam/deer-flow-go/biz/model"
)

// search 背景调研搜索函数，用于在研究开始前进行初步信息收集
// 这是Investigator智能体的核心功能，负责为后续的研究计划提供背景信息
//
// 参数:
//   - ctx: 上下文对象，用于状态管理和日志记录
//   - name: 搜索名称（当前未使用）
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 搜索输出（当前为空字符串）
//   - err: 错误信息
//
// 核心逻辑:
// 1. 从MCP服务器中查找可用的搜索工具
// 2. 使用用户最后一条消息作为搜索查询
// 3. 执行搜索并将结果保存到state.BackgroundInvestigationResults
// 4. 为Planner提供背景调研信息，帮助制定更准确的研究计划
func search(ctx context.Context, name string, opts ...any) (output string, err error) {
	// 从MCP服务器中查找可用的搜索工具
	var searchTool tool.InvokableTool
	for _, cli := range infra.MCPServer {
		if searchTool != nil {
			break
		}
		// 获取当前MCP客户端的工具列表
		ts, err := mcp.GetTools(ctx, &mcp.Config{Cli: cli})
		if err != nil {
			ilog.EventError(ctx, err, "builder_error")
			continue
		}
		// 查找名称以"search"结尾的搜索工具
		for _, t := range ts {
			info, _ := t.Info(ctx)
			if strings.HasSuffix(info.Name, "search") {
				searchTool, _ = t.(tool.InvokableTool)
				break
			}
		}
	}

	// 处理状态并执行搜索
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		// 使用用户最后一条消息作为搜索查询
		args := map[string]any{
			"query": state.Messages[len(state.Messages)-1].Content,
		}
		// 序列化搜索参数
		argsBytes, err := json.Marshal(args)
		if err != nil {
			ilog.EventError(ctx, err, "json_marshal_error")
			return err
		}
		// 执行搜索工具调用
		result, err := searchTool.InvokableRun(ctx, string(argsBytes))
		if err != nil {
			ilog.EventError(ctx, err, "search_result_error")
		}
		// 记录搜索结果并保存到状态中
		ilog.EventDebug(ctx, "back_search_result", "result", result)
		// 将搜索结果保存为背景调研信息，供Planner使用
		state.BackgroundInvestigationResults = result
		return nil
	})
	return output, err
}

// bIRouter 背景调研路由器，决定调研完成后的流向
// 这是Investigator工作流程中的路由决策函数，负责将流程导向下一个智能体
//
// 参数:
//   - ctx: 上下文对象，用于状态管理
//   - input: 输入字符串（当前未使用）
//   - opts: 可选参数（当前未使用）
//
// 返回值:
//   - output: 下一个节点的名称，固定为consts.Planner
//   - err: 错误信息
//
// 核心逻辑:
// 1. 设置流向为Planner智能体
// 2. 将背景调研的结果传递给Planner，用于制定详细的研究计划
// 3. 完成Investigator的使命，启动正式的研究流程
func bIRouter(ctx context.Context, input string, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()
		// 设置下一步流向为Planner，开始制定研究计划
		state.Goto = consts.Planner
		return nil
	})
	return output, nil
}

// NewBAgent 创建背景调研智能体（Background Investigation Agent）的工作流图
// 这是Investigator智能体的构造函数，负责构建背景调研的完整工作流程
//
// 参数:
//   - ctx: 上下文对象，用于初始化和配置管理
//
// 返回值:
//   - *compose.Graph[I, O]: 配置完成的背景调研工作流图
//
// 核心逻辑:
// 1. 创建包含搜索和路由两个节点的简单工作流
// 2. 构建工作流图：search → router
// 3. 设置节点间的连接关系，形成完整的背景调研流程
//
// 工作流程说明:
// - search: 执行背景信息搜索，收集相关资料
// - router: 路由决策，将流程导向Planner进行计划制定
//
// 在整个系统中的作用:
// Investigator是研究流程的第一个环节，为后续的计划制定提供必要的背景信息
func NewBAgent[I, O any](ctx context.Context) *compose.Graph[I, O] {
	cag := compose.NewGraph[I, O]()

	// 添加背景搜索节点
	_ = cag.AddLambdaNode("search", compose.InvokableLambdaWithOption(search))
	// 添加路由决策节点
	_ = cag.AddLambdaNode("router", compose.InvokableLambdaWithOption(bIRouter))

	// 设置工作流的执行顺序
	_ = cag.AddEdge(compose.START, "search")  // 开始 → 背景搜索
	_ = cag.AddEdge("search", "router")      // 背景搜索 → 路由决策
	_ = cag.AddEdge("router", compose.END)   // 路由决策 → 结束
	return cag
}
