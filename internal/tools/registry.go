package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go-tiny-claw/internal/schema"
	"log"
)

type MiddlewareFunc func(ctx context.Context, call schema.ToolCall) (allowed bool, rejectReason string)

// Registry 定义了工具的注册与分发执行接口
type Registry interface {
	// Register 挂载新的工具到系统
	Register(tool BaseTool)

	// GetAvailableTools 返回当前系统挂载的所有可用工具的 schema
	GetAvailableTools() []schema.ToolDefinition

	// Execute 实际执行模型请求的工具，并返回结果
	Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult

	Use(mw MiddlewareFunc) // middleware 挂载点
}

type registryImpl struct {
	tools       map[string]BaseTool
	middlewares []MiddlewareFunc
}

func NewRegistry() Registry {
	return &registryImpl{
		tools:       map[string]BaseTool{},
		middlewares: make([]MiddlewareFunc, 0),
	}
}

func (r *registryImpl) Use(m MiddlewareFunc) {
	r.middlewares = append(r.middlewares, m)
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
	//2. 【核心防御】执行底层逻辑前，依次运行所有 middleware
	for _, mw := range r.middlewares {
		allowed, reason := mw(ctx, call)
		if !allowed {
			log.Printf("[Registry] ⚠️ 工具 %s 被 Middleware 拦截: %s\n", call.Name, reason)
			return schema.ToolResult{
				ToolCallID: call.ID,
				Output:     fmt.Sprintf("执行被系统拦截。原因：%s", call.Name),
				IsError:    true, // 必须返回 Error，强制大模型阅读拒绝理由
			}
		}
	}
	//3. 所有 Middleware 都放行了，执行工具逻辑。
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
