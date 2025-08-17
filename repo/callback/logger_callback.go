package callback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/HildaM/logs/slog"
	"github.com/cloudwego/eino/callbacks"
	ecmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	"github.com/google/uuid"
	"github.com/hildam/deer-flow-go/entity/model"
)

// LoggerCallback 日志回调
type LoggerCallback struct {
	callbacks.HandlerBuilder // 可以用 callbacks.HandlerBuilder 来辅助实现 callback

	ID  string      // 线程ID，用于标识当前对话会话
	SSE *sse.Writer // SSE写入器，用于向客户端推送实时流式数据
	Out chan string // 输出通道，用于异步传递消息内容
}

// pushF 推送格式化数据到客户端
// 将聊天响应数据序列化后通过SSE和输出通道进行双路推送
func (cb *LoggerCallback) pushF(ctx context.Context, event string, data *model.ChatResp) error {
	// 将响应数据序列化为JSON格式
	dataByte, err := json.Marshal(data)
	if err != nil {
		slog.Error("pushF failed, marshal data err = %+v, data = %+v", err, data)
		return err
	}
	// 通过SSE推送到客户端（如果SSE连接存在）
	if cb.SSE != nil {
		err = cb.SSE.WriteEvent("", event, dataByte)
	}
	// 通过输出通道异步传递消息内容（如果通道存在）
	if cb.Out != nil {
		cb.Out <- data.Content
	}
	return nil
}

// pushMsg 推送消息到客户端
// 根据消息类型（普通消息、工具调用、工具结果）进行不同的处理和推送
func (cb *LoggerCallback) pushMsg(ctx context.Context, msgID string, msg *schema.Message) error {
	// 空消息检查
	if msg == nil {
		return nil
	}

	// 从状态中获取当前智能体名称
	agentName := ""
	_ = compose.ProcessState[*model.State](ctx, func(_ context.Context, state *model.State) error {
		agentName = state.Goto
		return nil
	})

	// 提取完成原因（如果存在响应元数据）
	fr := ""
	if msg.ResponseMeta != nil {
		fr = msg.ResponseMeta.FinishReason
	}
	// 构建标准聊天响应数据结构
	data := &model.ChatResp{
		ThreadID:      cb.ID,
		Agent:         agentName,
		ID:            msgID,
		Role:          "assistant",
		Content:       msg.Content,
		FinishReason:  fr,
		MessageChunks: msg.Content,
	}

	// 处理工具调用结果消息
	if msg.Role == schema.Tool {
		data.ToolCallID = msg.ToolCallID
		return cb.pushF(ctx, "tool_call_result", data)
	}

	// 处理包含工具调用的消息
	if len(msg.ToolCalls) > 0 {
		event := "tool_call_chunks"
		// 当前只支持单个工具调用，多个工具调用会记录警告并跳过
		if len(msg.ToolCalls) != 1 {
			slog.Error("pushMsg failed, tool_calls len not 1, msg = %+v", msg)
			return nil
		}

		// 初始化工具调用响应数据结构
		ts := []model.ToolResp{}
		tcs := []model.ToolChunkResp{}
		fn := msg.ToolCalls[0].Function.Name
		// 如果工具名称存在，构建完整的工具调用响应
		if len(fn) > 0 {
			event = "tool_calls"
			// 特殊处理：将搜索相关工具统一命名为web_search
			if strings.HasSuffix(fn, "search") {
				fn = "web_search"
			}
			ts = append(ts, model.ToolResp{
				Name: fn,
				Args: map[string]interface{}{},
				Type: "tool_call",
				ID:   msg.ToolCalls[0].ID,
			})
		}
		// 构建工具调用块响应（包含实际参数）
		tcs = append(tcs, model.ToolChunkResp{
			Name: fn,
			Args: msg.ToolCalls[0].Function.Arguments,
			Type: "tool_call_chunk",
			ID:   msg.ToolCalls[0].ID,
		})
		data.ToolCalls = ts
		data.ToolCallChunks = tcs
		return cb.pushF(ctx, event, data)
	}
	// 处理普通消息块
	return cb.pushF(ctx, "message_chunk", data)
}

// OnStart 智能体开始执行时的回调方法
// 当智能体或组件开始执行时被调用，用于记录执行开始的信息
//
// 参数:
//   - ctx: 上下文对象
//   - info: 运行信息，包含组件名称、类型等元数据
//   - input: 回调输入数据
//
// 返回值:
//   - context.Context: 可能被修改的上下文对象
func (cb *LoggerCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	// 如果输入是字符串类型，通过输出通道记录开始信息
	if inputStr, ok := input.(string); ok {
		if cb.Out != nil {
			cb.Out <- "\n==================\n"
			cb.Out <- fmt.Sprintf(" [OnStart] %s ", inputStr)
			cb.Out <- "\n==================\n"
		}
	}
	return ctx
}

// OnEnd 智能体执行结束时的回调方法
// 当智能体或组件执行完成时被调用，用于记录执行结果
//
// 参数:
//   - ctx: 上下文对象
//   - info: 运行信息，包含组件名称、类型等元数据
//   - output: 回调输出数据
//
// 返回值:
//   - context.Context: 可能被修改的上下文对象
//
// 注意: 当前实现中已注释掉调试输出，避免在生产环境中产生过多日志
func (cb *LoggerCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	//fmt.Println("=========[OnEnd]=========", info.Name, "|", info.Component, "|", info.Type)
	//outputStr, _ := json.MarshalIndent(output, "", "  ")
	//if len(outputStr) > 200 {
	//	outputStr = outputStr[:200]
	//}
	//fmt.Println(string(outputStr))
	return ctx
}

// OnError 智能体执行出错时的回调方法
// 当智能体或组件执行过程中发生错误时被调用，用于错误记录和处理
//
// 参数:
//   - ctx: 上下文对象
//   - info: 运行信息，包含组件名称、类型等元数据
//   - err: 发生的错误
//
// 返回值:
//   - context.Context: 可能被修改的上下文对象
func (cb *LoggerCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	fmt.Println("=========[OnError]=========")
	fmt.Println(err)
	return ctx
}

// OnEndWithStreamOutput 处理流式输出的回调方法
// 当智能体产生流式输出时被调用，负责实时处理和推送流式数据
// 这是实现SSE实时响应的核心方法
//
// 参数:
//   - ctx: 上下文对象
//   - info: 运行信息，包含组件名称、类型等元数据
//   - output: 流式输出读取器，用于接收连续的数据帧
//
// 返回值:
//   - context.Context: 可能被修改的上下文对象
//
// 核心逻辑:
// 1. 生成唯一消息ID用于标识本次流式会话
// 2. 启动异步goroutine处理流式数据，避免阻塞主流程
// 3. 循环接收数据帧，根据帧类型进行不同处理
// 4. 支持单个消息、模型回调输出、消息数组等多种数据类型
// 5. 异常恢复机制确保流处理的稳定性
func (cb *LoggerCallback) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo,
	output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	// 生成唯一消息ID，用于标识本次流式会话
	msgID := uuid.New().String()
	// 启动异步goroutine处理流式数据，避免阻塞主流程
	go func() {
		// 确保流在函数结束时被正确关闭
		defer output.Close() // remember to close the stream in defer
		// 异常恢复机制，防止panic导致整个程序崩溃
		defer func() {
			if err := recover(); err != nil {
				slog.Error("OnEndStream panic_recover, msgID = %s, err = %v", msgID, err)
			}
		}()
		// 循环接收流式数据帧
		for {
			frame, err := output.Recv()
			// 流结束标志，正常退出循环
			if errors.Is(err, io.EOF) {
				break
			}
			// 接收错误，记录日志并退出
			if err != nil {
				slog.Error("OnEndStream recv_error, msgID = %s, err = %v", msgID, err)
				return
			}

			// 根据数据帧类型进行不同处理
			switch v := frame.(type) {
			case *schema.Message:
				// 处理单个消息
				_ = cb.pushMsg(ctx, msgID, v)
			case *ecmodel.CallbackOutput:
				// 处理模型回调输出，提取其中的消息
				_ = cb.pushMsg(ctx, msgID, v.Message)
			case []*schema.Message:
				// 处理消息数组，逐个推送
				for _, m := range v {
					_ = cb.pushMsg(ctx, msgID, m)
				}
			//case string:
			//	ilog.EventInfo(ctx, "frame_type", "type", "str", "v", v)
			default:
				// 未知类型的数据帧，暂时忽略（调试代码已注释）
				//ilog.EventInfo(ctx, "frame_type", "type", "unknown", "v", v)
			}
		}

	}()
	return ctx
}

// OnStartWithStreamInput 处理流式输入的回调方法
// 当智能体接收流式输入时被调用，目前实现为简单的资源清理
//
// 参数:
//   - ctx: 上下文对象
//   - info: 运行信息，包含组件名称、类型等元数据
//   - input: 流式输入读取器
//
// 返回值:
//   - context.Context: 可能被修改的上下文对象
//
// 注意: 当前实现仅进行资源清理，未对输入流进行实际处理
func (cb *LoggerCallback) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo,
	input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	// 确保输入流被正确关闭，释放相关资源
	defer input.Close()
	return ctx
}
