package investigator

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/HildaM/logs/slog"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/hildam/deer-flow-go/entity/consts"
	"github.com/hildam/deer-flow-go/entity/model"
	"github.com/hildam/deer-flow-go/repo/llm"
	"github.com/hildam/deer-flow-go/repo/mcp"
)

// investigatorImpl 调查者
type investigatorImpl[I, O any] struct {
	llm *openai.ChatModel // llm模型服务
}

// NewInvestigator 创建实例
func NewInvestigator[I, O any](ctx context.Context) *investigatorImpl[I, O] {
	return &investigatorImpl[I, O]{
		llm: llm.NewChatModel(ctx),
	}
}

// NewGraph 创建任务图
func (i *investigatorImpl[I, O]) NewGraphNode(ctx context.Context) (key string, node compose.AnyGraph, nameOption compose.GraphAddNodeOpt) {
	graph := compose.NewGraph[I, O]()

	// 添加节点
	graph.AddLambdaNode("search", compose.InvokableLambdaWithOption(search))
	graph.AddLambdaNode("router", compose.InvokableLambdaWithOption(router))

	// 构造工作流
	graph.AddEdge(compose.START, "search")
	graph.AddEdge("search", "router")
	graph.AddEdge("router", compose.END)

	return consts.BackgroundInvestigator, graph, compose.WithNodeName(consts.BackgroundInvestigator)
}

// search 网络搜索节点
func search(ctx context.Context, name string, opts ...any) (output string, err error) {
	// 获取网络搜索 mcp 工具
	toolList, err := mcp.GetMCPTools(ctx)
	if err != nil {
		slog.Error("search failed, get mcp tools err = %+v", err)
		return output, err
	}

	// 选择网络搜索工具
	var searchTool tool.InvokableTool
	for _, mcpTool := range toolList {
		// 获取工具详情
		toolInfo, err := mcpTool.Info(ctx)
		if err != nil {
			slog.Error("search failed, get tool info err = %+v", err)
			continue
		}
		if strings.HasSuffix(toolInfo.Name, "search") {
			searchTool = mcpTool.(tool.InvokableTool)
			break
		}
	}

	// 调用工具
	err = compose.ProcessState[*model.State](ctx, func(ctx context.Context, state *model.State) error {
		// 使用用户最后一条消息作为搜索查询
		args := map[string]any{
			"query": state.Messages[len(state.Messages)-1].Content,
		}

		// 序列化参数
		argsJSON, err := json.Marshal(args)
		if err != nil {
			slog.Error("search failed, marshal args err = %+v", err)
			return err
		}

		// 调用工具
		result, err := searchTool.InvokableRun(ctx, string(argsJSON))
		if err != nil {
			slog.Error("search failed, invokable run err = %+v", err)
			return err
		}
		slog.Debug("search debug, result = %+v, args = %+v", result, args)

		// 将搜索结果保存为背景调研信息，供Planner使用
		state.BackgroundInvestigationResults = result
		return nil
	})
	return output, err
}

// router 路由节点
func router(ctx context.Context, input string, opts ...any) (output string, err error) {
	err = compose.ProcessState[*model.State](ctx, func(ctx context.Context, state *model.State) error {
		defer func() {
			output = state.Goto
		}()

		// 设置下一步流向为Planner，开始制定研究计划
		state.Goto = consts.Planner
		return nil
	})
	return output, err
}
