package llm

import (
	"context"

	openai3 "github.com/cloudwego/eino-ext/libs/acl/openai"

	"github.com/HildaM/logs/slog"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/hildam/deer-flow-go/entity/conf"
	"github.com/hildam/deer-flow-go/entity/model"
)

// NewChatModel 创建Chat模型
func NewChatModel(ctx context.Context) *openai.ChatModel {
	llm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:   conf.GetCfg().Model.DefaultModel.ModelID,
		BaseURL: conf.GetCfg().Model.DefaultModel.BaseURL,
		APIKey:  conf.GetCfg().Model.DefaultModel.APIKey,
	})
	if err != nil {
		slog.Fatal("NewChatModel failed, err: %v", err)
		return nil
	}
	return llm
}

// NewPlanModel 创建计划模型
func NewPlanModel(ctx context.Context) *openai.ChatModel {
	// 定义返回结构
	planSchema, _ := openapi3gen.NewSchemaRefForValue(&model.Plan{}, nil)

	// 创建 LLM
	llm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:   conf.GetCfg().Model.DefaultModel.ModelID,
		BaseURL: conf.GetCfg().Model.DefaultModel.BaseURL,
		APIKey:  conf.GetCfg().Model.DefaultModel.APIKey,
		// 计划模型响应格式
		ResponseFormat: &openai3.ChatCompletionResponseFormat{
			Type: openai3.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai3.ChatCompletionResponseFormatJSONSchema{
				Name:   "plan",
				Strict: false,
				Schema: planSchema.Value,
			},
		},
	})
	if err != nil {
		slog.Fatal("NewPlanModel failed, err: %v", err)
		return nil
	}
	return llm
}
