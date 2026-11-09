package schema

import "encoding/json"

type Role string

const (
	RoleSystem    Role = "system"    // 系统提示词，确立 Agent 性格与红线
	RoleUser      Role = "user"      // 用户输入
	RoleAssistant Role = "assistant" // 模型输出，包含推理或工具调用
)

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls"`   // 如果模型决定调用工具，此字段将被填充（支持并行调用多个工具）
	ToolCallID string     `json:"tool_call_id"` // 如果 Message 是针对某个工具调用的响应，该字段必须被填充，以告知模型上下文的关联性
}

type ToolCall struct {
	ID        string          `json:"id"`        // 工具调用的唯一 id
	Name      string          `json:"name"`      // 工具名称
	Arguments json.RawMessage `json:"arguments"` // 存放 json 参数，使用 RawMessage 是为了延迟解析，将解析责任交给具体工具。
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`   // 工具执行的输出或者报错的堆栈
	IsError    bool   `json:"is_error"` // 标记是否失败
}

// ToolDefinition 描述一个大模型可以调用的工具元信息
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"` // 对应 Json Schema
}
