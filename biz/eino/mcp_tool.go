package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/hildam/deer-flow-go/biz/infra"
)

// 修复版本的MCP工具包装器
type FixedMCPTool struct {
	cli         client.MCPClient
	toolName    string
	toolDesc    string
	inputSchema mcpgo.ToolInputSchema
}

func (t *FixedMCPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
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

func (t *FixedMCPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
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

// 将MCP的InputSchema转换为eino的ParamsOneOf
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

// GetFixedMCPTools 获取所有修复版本的MCP工具
func GetFixedMCPTools(ctx context.Context) ([]tool.BaseTool, error) {
	var allTools []tool.BaseTool

	// 遍历所有MCP服务器
	for serverName, mcpClient := range infra.MCPServer {
		log.Printf("Loading tools from MCP server: %s", serverName)

		// 获取工具列表
		listToolsReq := mcpgo.ListToolsRequest{}
		toolsResp, err := mcpClient.ListTools(ctx, listToolsReq)
		if err != nil {
			log.Printf("Error listing tools from %s: %v", serverName, err)
			continue
		}

		log.Printf("Found %d tools from %s", len(toolsResp.Tools), serverName)

		// 为每个工具创建FixedMCPTool包装器
		for _, mcpTool := range toolsResp.Tools {
			fixedTool := &FixedMCPTool{
				cli:         mcpClient,
				toolName:    mcpTool.Name,
				toolDesc:    mcpTool.Description,
				inputSchema: mcpTool.InputSchema,
			}
			allTools = append(allTools, fixedTool)
			log.Printf("Added fixed tool: %s", mcpTool.Name)
		}
	}

	log.Printf("Total fixed tools loaded: %d", len(allTools))
	return allTools, nil
}
