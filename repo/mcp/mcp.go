package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/HildaM/logs/slog"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hildam/deer-flow-go/entity/conf"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// InitMcpServer 初始化MCP服务端
func InitMcpServer() (err error) {
	mcpServer, err = createMcpClients()
	if err != nil {
		return err
	}
	return nil
}

// createMcpClients 创建MCP客户端
func createMcpClients() (map[string]client.MCPClient, error) {
	ctx := context.Background()

	// 将 DeerConfig 转换为 MCPConfig
	mcpConfig := &MCPConfig{
		MCPServers: make(map[string]ServerConfigWrapper),
	}

	for name, server := range conf.GetCfg().MCP.Servers {
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

		slog.Debug("createMcpClients debug, load mcp client = %+v, mcp type = %+v", name, server.Config.GetType())
		if server.Config.GetType() == transportSSE {
			slog.Debug("createMcpClients debug, load mcp sse client = %+v, mcp type = %+v, sse config = %+v", name, server.Config.GetType(), server.Config)

			sseConfig := server.Config.(SSEServerConfig)

			options := []transport.ClientOption{}

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
				options = append(options, transport.WithHeaders(headers))
			}

			mcpClient, err = client.NewSSEMCPClient(
				sseConfig.Url,
				options...,
			)
			if err == nil {
				slog.Error("createMcpClients error, name = %+v, err = %+v", name, err)
				err = mcpClient.(*client.Client).Start(ctx)
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

			slog.Debug("createMcpClients debug, load mcp stdio client = %+v, mcp type = %+v, stdio config = %+v, args = %+v, env = %+v", name, server.Config.GetType(), stdioConfig, stdioConfig.Args, env)
		}
		if err != nil {
			for _, c := range clients {
				_ = c.Close()
			}
			slog.Error("createMcpClients error, name = %+v, err = %+v", name, err)
			return nil, fmt.Errorf(
				"failed to create MCP client for %s: %w",
				name,
				err,
			)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		slog.Debug("createMcpClients debug, initialize server, name = %+v", name)
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
			slog.Error("createMcpClients error, name = %+v, err = %+v", name, err)

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

var (
	// 工具缓存相关变量
	cachedTools []tool.BaseTool // 缓存的MCP工具
	toolsOnce   sync.Once       // 确保工具只被初始化一次
	toolsErr    error           // 初始化工具时的错误
)

// GetMCPTools 获取所有MCP工具
func GetMCPTools(ctx context.Context) ([]tool.BaseTool, error) {
	// 使用 sync.Once 确保工具只被初始化一次
	toolsOnce.Do(func() {
		cachedTools, toolsErr = loadMCPTools(ctx)
	})
	return cachedTools, toolsErr
}

// loadMCPTools 加载所有MCP工具（内部函数）
func loadMCPTools(ctx context.Context) ([]tool.BaseTool, error) {
	var allTools []tool.BaseTool

	// 遍历所有MCP服务器
	for serverName, mcpClient := range mcpServer {
		slog.Debug("loadMCPTools debug, Loading tools from MCP server = %s", serverName)

		// 获取工具列表
		listToolsReq := mcpgo.ListToolsRequest{}
		toolsResp, err := mcpClient.ListTools(ctx, listToolsReq)
		if err != nil {
			slog.Debug("loadMCPTools failed, Error listing tools from %s = %v", serverName, err)
			continue
		}

		slog.Debug("loadMCPTools debug, Found %d tools from %s", len(toolsResp.Tools), serverName)

		// 为每个工具创建MCPTool包装器
		for _, mcpTool := range toolsResp.Tools {
			tool := &MCPTool{
				cli:         mcpClient,
				toolName:    mcpTool.Name,
				toolDesc:    mcpTool.Description,
				inputSchema: mcpTool.InputSchema,
			}
			allTools = append(allTools, tool)
			slog.Debug("loadMCPTools debug, Added tool: %s", mcpTool.Name)
		}
	}

	slog.Debug("loadMCPTools debug, Total tools loaded: %d", len(allTools))
	return allTools, nil
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
