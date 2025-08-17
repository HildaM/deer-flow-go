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

package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/RanFeng/ilog"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/hildam/deer-flow-go/conf"
)

const (
	transportStdio = "stdio"
	transportSSE   = "sse"
)

var (
	MCPServer map[string]client.MCPClient
)

func InitMCP() {
	var err error
	MCPServer, err = CreateMCPClients()
	if err != nil {
		panic(err)
	}
}

type MCPConfig struct {
	MCPServers map[string]ServerConfigWrapper `json:"mcpServers"`
}

type ServerConfig interface {
	GetType() string
}

type STDIOServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

func (s STDIOServerConfig) GetType() string {
	return transportStdio
}

type SSEServerConfig struct {
	Url     string   `json:"url"`
	Headers []string `json:"headers,omitempty"`
}

func (s SSEServerConfig) GetType() string {
	return transportSSE
}

type ServerConfigWrapper struct {
	Config ServerConfig
}

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
func (w ServerConfigWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Config)
}

func CreateMCPClients() (map[string]client.MCPClient, error) {
	// 将 DeerConfig 转换为 MCPConfig
	mcpConfig := &MCPConfig{
		MCPServers: make(map[string]ServerConfigWrapper),
	}

	for name, server := range conf.Config.MCP.Servers {
		mcpConfig.MCPServers[name] = ServerConfigWrapper{
			Config: STDIOServerConfig{
				Command: server.Command,
				Args:    server.Args,
				Env:     server.Env,
			},
		}
	}

	clients := make(map[string]client.MCPClient)

	for name, server := range mcpConfig.MCPServers {
		var mcpClient client.MCPClient
		var err error
		ilog.EventInfo(context.Background(), "load mcp client", name, server.Config.GetType())
		if server.Config.GetType() == transportSSE {
			sseConfig := server.Config.(SSEServerConfig)

			options := []client.ClientOption{}

			if sseConfig.Headers != nil {
				// Parse headers from the conf
				headers := make(map[string]string)
				for _, header := range sseConfig.Headers {
					parts := strings.SplitN(header, ":", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						headers[key] = value
					}
				}
				options = append(options, client.WithHeaders(headers))
			}

			mcpClient, err = client.NewSSEMCPClient(
				sseConfig.Url,
				options...,
			)
			if err == nil {
				err = mcpClient.(*client.SSEMCPClient).Start(context.Background())
			}
		} else {
			stdioConfig := server.Config.(STDIOServerConfig)
			var env []string
			for k, v := range stdioConfig.Env {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
			mcpClient, err = client.NewStdioMCPClient(
				stdioConfig.Command,
				env,
				stdioConfig.Args...)
		}
		if err != nil {
			for _, c := range clients {
				_ = c.Close()
			}
			return nil, fmt.Errorf(
				"failed to create MCP client for %s: %w",
				name,
				err,
			)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		ilog.EventInfo(ctx, "Initializing server...", "name", name)
		initRequest := mcpgo.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcpgo.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcpgo.Implementation{
			Name:    "mcphost",
			Version: "0.1.0",
		}
		initRequest.Params.Capabilities = mcpgo.ClientCapabilities{}

		_, err = mcpClient.Initialize(ctx, initRequest)
		if err != nil {
			_ = mcpClient.Close()
			for _, c := range clients {
				_ = c.Close()
			}
			return nil, fmt.Errorf(
				"failed to initialize MCP client for %s: %w",
				name,
				err,
			)
		}

		clients[name] = mcpClient
	}

	return clients, nil
}

// convertMCPSchemaToEinoParams 将MCP的InputSchema转换为eino的ParamsOneOf
func convertMCPSchemaToEinoParams(inputSchema mcpgo.ToolInputSchema) (*schema.ParamsOneOf, error) {
	// 将inputSchema转换为OpenAPI v3 Schema
	schemaBytes, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input schema: %w", err)
	}

	// 解析为map以便修改
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	// 确保schema有type字段
	if _, hasType := schemaMap["type"]; !hasType {
		if _, hasAnyOf := schemaMap["anyOf"]; !hasAnyOf {
			// 如果既没有type也没有anyOf，设置默认type为object
			schemaMap["type"] = "object"
		}
	}

	// 检查properties中的每个字段是否有type
	if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
		for _, propValue := range properties {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				if _, hasType := propMap["type"]; !hasType {
					if _, hasAnyOf := propMap["anyOf"]; !hasAnyOf {
						propMap["type"] = "string"
					}
				}
			}
		}
	}

	// 重新序列化修改后的schema
	fixedSchemaBytes, err := json.Marshal(schemaMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fixed schema: %w", err)
	}

	// 解析为OpenAPI v3 Schema
	var openAPISchema openapi3.Schema
	if err := json.Unmarshal(fixedSchemaBytes, &openAPISchema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to OpenAPI schema: %w", err)
	}

	// 使用NewParamsOneOfByOpenAPIV3创建ParamsOneOf
	result := schema.NewParamsOneOfByOpenAPIV3(&openAPISchema)
	return result, nil
}

// MCPTool MCP工具包装器
type MCPTool struct {
	cli         client.MCPClient
	toolName    string
	toolDesc    string
	inputSchema mcpgo.ToolInputSchema
}

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

// GetMCPTools 获取所有MCP工具
func GetMCPTools(ctx context.Context) ([]tool.BaseTool, error) {
	var allTools []tool.BaseTool

	// 遍历所有MCP服务器
	for serverName, mcpClient := range MCPServer {
		log.Printf("Loading tools from MCP server: %s", serverName)

		// 获取工具列表
		listToolsReq := mcpgo.ListToolsRequest{}
		toolsResp, err := mcpClient.ListTools(ctx, listToolsReq)
		if err != nil {
			log.Printf("Error listing tools from %s: %v", serverName, err)
			continue
		}

		log.Printf("Found %d tools from %s", len(toolsResp.Tools), serverName)

		// 为每个工具创建MCPTool包装器
		for _, mcpTool := range toolsResp.Tools {
			tool := &MCPTool{
				cli:         mcpClient,
				toolName:    mcpTool.Name,
				toolDesc:    mcpTool.Description,
				inputSchema: mcpTool.InputSchema,
			}
			allTools = append(allTools, tool)
			log.Printf("Added tool: %s", mcpTool.Name)
		}
	}

	log.Printf("Total tools loaded: %d", len(allTools))
	return allTools, nil
}
