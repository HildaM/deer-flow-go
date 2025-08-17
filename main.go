package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/HildaM/logs/slog"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/hildam/deer-flow-go/agent"
	"github.com/hildam/deer-flow-go/entity/conf"
	"github.com/hildam/deer-flow-go/entity/consts"
	"github.com/hildam/deer-flow-go/repo/callback"
	"github.com/hildam/deer-flow-go/repo/mcp"
)

func main() {
	runConsule()
}

// runConsule 运行控制台
func runConsule() {
	ctx := context.Background()

	// 初始化配置
	funcs := []func() error{conf.Init, mcp.InitMcpServer}
	for _, f := range funcs {
		if err := f(); err != nil {
			log.Fatal(err)
		}
	}

	// 读取用户终端输入
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("请输入你的需求： ")

	// 构造用户输入Prompt
	userPrompt, _ := reader.ReadString('\n')
	userPrompt = strings.TrimSpace(userPrompt) // 去除换行符
	userMessage := []*schema.Message{
		schema.UserMessage(userPrompt),
	}

	// 创建 Agent 工作流
	graph, err := agent.BuildAgentGraph[string, string](ctx, userMessage)
	if err != nil {
		slog.Fatal("BuildAgentGraph failed, err: %v", err)
	}

	// 流式输出
	outChan := make(chan string)
	go func() {
		for out := range outChan {
			fmt.Print(out)
		}
	}()

	_, err = graph.Stream(ctx, consts.Coordinator,
		compose.WithCallbacks(&callback.LoggerCallback{
			Out: outChan,
		}))
	if err != nil {
		slog.Error("Stream failed, err: %v", err)
	}
}
