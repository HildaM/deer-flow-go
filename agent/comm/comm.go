package comm

import (
	"context"
	"io"

	"github.com/HildaM/logs/slog"

	"github.com/cloudwego/eino/schema"
	"github.com/hildam/deer-flow-go/entity/conf"
)

// ModifyInputFunc 输入消息修改函数
func ModifyInputFunc(ctx context.Context, inputList []*schema.Message) []*schema.Message {
	sum := 0
	maxLimit := conf.GetCfg().Setting.MaxLimitToken
	for _, input := range inputList {
		if input == nil {
			slog.Debug("ModifyInputFunc debug, input is nil")
			continue
		}

		length := len(input.Content)
		if length >= maxLimit {
			slog.Debug("ModifyInputFunc debug, input content length is %d, max limit token is %d", length, maxLimit)
			// 截断, 取后半段部分的最新信息
			input.Content = input.Content[length-maxLimit:]
		}

		sum += len(input.Content)
	}

	slog.Debug("ModifyInputFunc debug, input content sum length is %d", sum)
	return inputList
}

// ToolCallChecker 工具调用检查函数
func ToolCallChecker(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
	defer sr.Close()

	// 遍历流式响应中的所有消息
	for {
		msg, err := sr.Recv()
		if err == io.EOF {
			// 流结束，未发现工具调用
			slog.Debug("toolCallChecker debug, stream message eof")
			return false, nil
		}
		if err != nil {
			slog.Error("toolCallChecker failed, recv stream message failed", "err", err)
			return false, err
		}

		// 检查当前消息是否包含工具调用
		if len(msg.ToolCalls) > 0 {
			return true, nil
		}
	}
}
