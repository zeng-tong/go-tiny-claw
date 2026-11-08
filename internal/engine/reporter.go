package engine

import "context"

type Reporter interface {
	// OnThinking 当模型开始慢思考时调用
	OnThinking(ctx context.Context)

	// OnToolCall 当模型决定并发调用工具时调用
	OnToolCall(ctx context.Context, toolName string, args string)

	// OnToolResult 当模型在底层执行完毕返回结果时调用
	OnToolResult(ctx context.Context, toolName string, result string, isError bool)

	// OnMessage 当模型宣告任务完成，向用户输出最终纯文本回答时调用
	OnMessage(ctx context.Context, content string)
}
