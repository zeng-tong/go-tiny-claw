package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go-tiny-claw/internal/context/schema"
	"log"
)

// Registry 定义了工具的注册与分发执行接口
type Registry interface {
	// Register 挂载新的工具到系统
	Register(tool BaseTool)

	// GetAvailableTools 返回当前系统挂载的所有可用工具的 schema
	GetAvailableTools() []schema.ToolDefinition

	// Execute 实际执行模型请求的工具，并返回结果
	Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult
}

type registryImpl struct {
	tools map[string]BaseTool
}

func NewRegistry() Registry {
	return &registryImpl{
		tools: map[string]BaseTool{},
	}
}

func (r *registryImpl) Register(tool BaseTool) {
	name := tool.Name()
	if _, exist := r.tools[name]; exist {
		log.Printf("[Warning] 工具 '%s' 已经注册，将被覆盖", tool.Name())
	}
	r.tools[name] = tool
	log.Printf("[Registry] 成功挂载工具 '%s'", tool.Name())
}

func (r *registryImpl) GetAvailableTools() []schema.ToolDefinition {
	var defs []schema.ToolDefinition
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

func (r *registryImpl) Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult {
	//1. 路由查找
	tool, exist := r.tools[call.Name]
	if !exist {
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     fmt.Sprintf("Error: 系统中不存在 '%s' 的工具", call.Name),
			IsError:    true,
		}
	}
	output, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     fmt.Sprintf("Error executing '%s': %v ", call.Name, err),
			IsError:    true,
		}
	}
	return schema.ToolResult{
		ToolCallID: call.ID,
		Output:     output,
		IsError:    false,
	}
}

type BaseTool interface {
	// Name 工具全局唯一名称
	Name() string

	// Definition 给大模型提供工具元信息和参数 JSON Schema
	Definition() schema.ToolDefinition

	// Execute 接受大模型吐出的 JSON 参数，执行具体业务逻辑。
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}
