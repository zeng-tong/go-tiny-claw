package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go-tiny-claw/internal/schema"
	"log"
)

type AgentRunner interface {
	RunSub(ctx context.Context, taskPrompt string, readOnlyRegistry Registry, reporter interface{}) (string, error)
}

type SubagentTool struct {
	runner           AgentRunner
	readOnlyRegistry Registry
	reporter         interface{}
}

func NewSubagentTool(runner AgentRunner, readOnlyRegistry Registry, reporter interface{}) *SubagentTool {
	return &SubagentTool{
		runner:           runner,
		readOnlyRegistry: readOnlyRegistry,
		reporter:         reporter,
	}
}

func (t *SubagentTool) Name() string {
	return "spawn_subagent"
}

func (t *SubagentTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.Name(),
		Description: "派出一个专门用于深度探索（Exploration）的子智能体。当你需要阅读大量代码、跨文件查找逻辑时请调用此工具。它在探索完毕后，会给你返回一份极度精炼的摘要报告。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_prompt": map[string]interface{}{
					"type":        "string",
					"description": "给子智能体下达明确的指令",
				},
			},
			"required": []string{"task_prompt"},
		},
	}
}

func (t *SubagentTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input subagentArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	log.Printf("[Subagent] 🚀 主 Agent 发起委派！正在拉起探路者: [%s]...\n", input.TaskPrompt)

	// 【核心逻辑】拉起一个完全物理隔离的子循环
	summary, err := t.runner.RunSub(ctx, input.TaskPrompt, t.readOnlyRegistry, t.reporter)
	if err != nil {
		return fmt.Errorf("子agent执行失败: %w", err).Error(), nil
	}
	log.Printf("[Subagent] ✅ 子智能体任务结束。报告返回给主干...")
	// 最终探索化作一段轻量级的 Summary
	// 就像一次普通的 API 调用一样，返回给了始终保持清醒的主 Agent。
	return fmt.Sprintf("【子智能体探索报告】:\n%s", summary), nil
}

type subagentArgs struct {
	TaskPrompt string `json:"task_prompt"`
}
