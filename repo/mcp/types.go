package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// MCP 工具类型枚举
const (
	transportStdio = "stdio"
	transportSSE   = "sse"
)

var (
	mcpServer map[string]client.MCPClient // MCP服务端客户端管理
)

// MCPConfig MCP配置
type MCPConfig struct {
	MCPServers map[string]ServerConfigWrapper `json:"mcpServers"`
}

// ServerConfig 服务端配置接口
type ServerConfig interface {
	GetType() string
}

// STDIOServerConfig STDIO服务端配置
type STDIOServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// GetType 获取服务端类型
func (s STDIOServerConfig) GetType() string {
	return transportStdio
}

// SSEServerConfig SSE服务端配置
type SSEServerConfig struct {
	Url     string   `json:"url"`
	Headers []string `json:"headers,omitempty"`
}

// GetType 获取服务端类型
func (s SSEServerConfig) GetType() string {
	return transportSSE
}

// ServerConfigWrapper 服务端配置包装器
type ServerConfigWrapper struct {
	Config ServerConfig
}

// UnmarshalJSON 反序列化JSON
func (w *ServerConfigWrapper) UnmarshalJSON(data []byte) error {
	var typeField struct {
		Url string `json:"url"`
	}

	if err := json.Unmarshal(data, &typeField); err != nil {
		return err
	}
	if typeField.Url != "" {
		// If the URL field is present, treat it as an SSE server
		var sse SSEServerConfig
		if err := json.Unmarshal(data, &sse); err != nil {
			return err
		}
		w.Config = sse
	} else {
		// Otherwise, treat it as a STDIOServerConfig
		var stdio STDIOServerConfig
		if err := json.Unmarshal(data, &stdio); err != nil {
			return err
		}
		w.Config = stdio
	}

	return nil
}

// MarshalJSON 序列化JSON
func (w *ServerConfigWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Config)
}

// MCPTool MCP工具包装器
type MCPTool struct {
	cli         client.MCPClient      // MCP客户端
	toolName    string                // 工具名称
	toolDesc    string                // 工具描述
	inputSchema mcpgo.ToolInputSchema // 输入参数Schema
}

// Info 获取工具信息
func (t *MCPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params, err := convertMCPSchemaToEinoParams(t.inputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema: %w", err)
	}

	return &schema.ToolInfo{
		Name:        t.toolName,
		Desc:        t.toolDesc,
		ParamsOneOf: params,
	}, nil
}

// InvokableRun 可调用运行
func (t *MCPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析JSON参数
	var paramsMap map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &paramsMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal params: %w", err)
	}

	// 调用MCP工具
	callReq := mcpgo.CallToolRequest{}
	callReq.Params.Name = t.toolName
	callReq.Params.Arguments = paramsMap

	resp, err := t.cli.CallTool(ctx, callReq)
	if err != nil {
		return "", fmt.Errorf("MCP tool call failed: %w", err)
	}

	// 处理响应
	if resp.IsError {
		// Content是一个切片，需要处理第一个元素或合并所有内容
		if len(resp.Content) > 0 {
			return "", fmt.Errorf("MCP tool error: %v", resp.Content[0])
		}
		return "", fmt.Errorf("MCP tool error: unknown error")
	}

	// Content是一个切片，处理所有内容
	if len(resp.Content) == 0 {
		return "", nil
	}

	// 如果只有一个内容项，直接返回
	if len(resp.Content) == 1 {
		contentBytes, err := json.Marshal(resp.Content[0])
		if err != nil {
			return "", fmt.Errorf("failed to marshal response: %w", err)
		}
		return string(contentBytes), nil
	}

	// 如果有多个内容项，合并它们
	contentBytes, err := json.Marshal(resp.Content)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(contentBytes), nil
}
