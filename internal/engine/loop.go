package engine

import (
	"context"
	"fmt"
	"go-tiny-claw/internal/context/schema"
	"go-tiny-claw/internal/provider"
	"go-tiny-claw/internal/tools"
	"log"
	"sync"
)

type AgentEngine struct {
	provider       provider.LLMProvider
	registry       tools.Registry
	WorkDir        string // 工作区，Agent 必须有一个明确的物理边界
	EnableThinking bool
}

func NewAgentEngine(provider provider.LLMProvider, registry tools.Registry, workDir string, enableThinking bool) *AgentEngine {
	return &AgentEngine{
		provider:       provider,
		registry:       registry,
		WorkDir:        workDir,
		EnableThinking: enableThinking,
	}
}

func (e *AgentEngine) Run(ctx context.Context, userPrompt string, reporter Reporter) error {
	log.Printf("[Engine] Run started, lock workdir: %s\n]", e.WorkDir)
	log.Printf("[Engine] 慢思考模式(thinking phase): %v", e.EnableThinking)
	// 真实场景中，这里会由动态 Prompt 组装器加载 AGENTS.md ，目前先硬编码
	contextHistory := []schema.Message{
		{
			Role:    schema.RoleSystem,
			Content: "You are go-tiny-claw, an expert coding assistant. You have full access to tools in the workspace.",
		},
		{
			Role:    schema.RoleUser,
			Content: userPrompt,
		},
	}
	turnCount := 0

	for {
		turnCount++
		//log.Printf("=======[Turn %d] Start ========\n", turnCount)
		availableTools := e.registry.GetAvailableTools()

		// ================= Phase 1: Thinking =================
		// 开启慢思考，不给任何工具。
		if e.EnableThinking {
			//log.Println("[Engine][Phase 1] 剥夺工具访问权，强制进入慢思考与规划阶段...")
			if reporter != nil {
				reporter.OnThinking(ctx) // 触发 Reporter
			}
			thinkResp, err := e.provider.Generate(ctx, contextHistory, nil)
			if err != nil {
				return fmt.Errorf("[Engine] Thinking 生成失败 %w", err)
			}
			if thinkResp.Content != "" {
				//log.Printf("🧠 [内部思考 Trace]: %s\n", thinkResp.Content)
				contextHistory = append(contextHistory, *thinkResp)
			}
		}

		// ================= Phase 2: Action =================
		//log.Println("[Engine][Phase 2] 恢复工具挂载，等待模型采取行动...")
		responseMsg, err := e.provider.Generate(ctx, contextHistory, availableTools)
		if err != nil {
			return fmt.Errorf("模型生成失败: %w", err)
		}

		contextHistory = append(contextHistory, *responseMsg)

		if responseMsg.Content != "" && reporter != nil {
			//log.Printf("[🧠模型对外输出]: %s\n", responseMsg.Content)
			reporter.OnMessage(ctx, responseMsg.Content)
		}

		if len(responseMsg.ToolCalls) == 0 {
			//log.Println("[Engine] 任务完成，退出循环。")
			break
		}

		//log.Printf("[Engine] 模型请求调用 %d 个工具...\n", len(responseMsg.ToolCalls))
		observationMsgs := make([]schema.Message, len(responseMsg.ToolCalls))

		var wg sync.WaitGroup
		for i, call := range responseMsg.ToolCalls {
			wg.Add(1)
			go func(idx int, call schema.ToolCall) {
				defer wg.Done()
				//log.Printf(" -> [Go-%d] 🛠️ 触发并行执行: %s\n", idx, call.Name)
				//log.Printf(" -> 🛠️ 执行工具: %s, 参数: %s\n", call.Name, string(call.Arguments))
				reporter.OnToolCall(ctx, call.Name, string(call.Arguments))
				result := e.registry.Execute(ctx, call)
				if reporter != nil {
					displayOut := result.Output
					if len(displayOut) > 200 {
						displayOut = displayOut[:200] + "...(已截断)"
					}
					reporter.OnToolCall(ctx, call.Name, string(displayOut))
				}
				// 将工具执行的观察结果封装为 User Message 追加到上下文中
				// 注意：TooCallID 必须携带
				observationMsg := schema.Message{
					Role:       schema.RoleUser,
					Content:    result.Output,
					ToolCallID: call.ID,
				}
				//contextHistory = append(contextHistory, observationMsg)
				observationMsgs[i] = observationMsg
			}(i, call)
		}
		wg.Wait()
		//log.Println("[Engine] 所有并发工具执行完毕，开始聚合观察结果 (Observation)...")
		contextHistory = append(contextHistory, observationMsgs...)
		// 继续循环会到开头，llm 将带着新加入的 Observation 继续下一轮思考...
		//log.Printf("=======[Turn %d] End ========\n\n", turnCount)
		//log.Println()
	}
	return nil
}
