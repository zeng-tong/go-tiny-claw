package engine

import (
	"context"
	"fmt"
	ctxpkg "go-tiny-claw/internal/context"
	"go-tiny-claw/internal/provider"
	"go-tiny-claw/internal/schema"
	"go-tiny-claw/internal/tools"
	"log"
	"strings"
	"sync"
)

type AgentEngine struct {
	provider       provider.LLMProvider
	registry       tools.Registry
	EnableThinking bool
	compactor      *ctxpkg.Compactor
	planMode       bool
	recovery       *ctxpkg.RecoveryManager
}

func NewAgentEngine(provider provider.LLMProvider, registry tools.Registry, enableThinking bool, planMode bool) *AgentEngine {
	return &AgentEngine{
		provider:  provider,
		registry:  registry,
		compactor: ctxpkg.NewCompactor(3000, 6),
		planMode:  planMode,
		recovery:  ctxpkg.NewRecoveryManager(),
	}
}

func (e *AgentEngine) Run(ctx context.Context, session *ctxpkg.Session, reporter Reporter) error {
	log.Printf("[Engine] 唤醒会话 [%s], 锁定工作区: %s", session.ID, session.WorkDir)
	log.Printf("[Engine] 慢思考模式(thinking phase): %v", e.EnableThinking)

	// 动态 Prompt 组装器加载 AGENTS.md
	composer := ctxpkg.NewPromptComposer(session.WorkDir, e.planMode)
	systemMsg := composer.Build()

	// main loop
	turnCount := 0
	for {
		turnCount++
		availableTools := e.registry.GetAvailableTools()
		workingMemory := session.GetWorkingMemory(20) // 截取最近 20 条消息作为 Working Memory

		// ================= Phase 0: 上下文组装 =================
		var contextHistory []schema.Message
		contextHistory = append(contextHistory, systemMsg)
		contextHistory = append(contextHistory, workingMemory...)
		compactedContext := e.compactor.Compact(contextHistory)
		// ================= Phase 1: Thinking =================
		// 开启慢思考，不给任何工具。
		var currentTurnThinkingContent string
		if e.EnableThinking {
			//log.Println("[Engine][Phase 1] 剥夺工具访问权，强制进入慢思考与规划阶段...")
			if reporter != nil {
				reporter.OnThinking(ctx) // 触发 Reporter
			}
			thinkResp, err := e.provider.Generate(ctx, compactedContext, nil)
			if err != nil {
				return fmt.Errorf("[Engine] Thinking 阶段失败 %w", err)
			}
			if thinkResp.Content != "" {
				log.Printf("🧠 [内部思考 Trace]: %s\n", thinkResp.Content)
				session.Append(*thinkResp)
				compactedContext = append(compactedContext, *thinkResp)
				currentTurnThinkingContent = thinkResp.Content
			}
		}

		// ================= Phase 2: Action =================
		//log.Println("[Engine][Phase 2] 恢复工具挂载，等待模型采取行动...")
		actionResp, err := e.provider.Generate(ctx, contextHistory, availableTools)
		if err != nil {
			return fmt.Errorf("action 阶段失败: %w", err)
		}
		finalAssistMsg := schema.Message{
			Role:      schema.RoleAssistant,
			Content:   strings.TrimSpace(currentTurnThinkingContent + "\n" + actionResp.Content),
			ToolCalls: actionResp.ToolCalls,
		}
		session.Append(finalAssistMsg)

		if actionResp.Content != "" && reporter != nil {
			log.Printf("[🧠模型对外输出]: %s\n", actionResp.Content)
			reporter.OnMessage(ctx, actionResp.Content)
		}

		if len(actionResp.ToolCalls) == 0 {
			log.Println("[Engine] 任务完成，退出循环。")
			break
		}

		//log.Printf("[Engine] 模型请求调用 %d 个工具...\n", len(actionResp.ToolCalls))
		observationMsgs := make([]schema.Message, len(actionResp.ToolCalls))

		var wg sync.WaitGroup
		for i, call := range actionResp.ToolCalls {
			wg.Add(1)
			go func(idx int, call schema.ToolCall) {
				defer wg.Done()
				if reporter != nil {
					reporter.OnToolCall(ctx, call.Name, string(call.Arguments))
				}
				result := e.registry.Execute(ctx, call)
				if result.IsError {
					finalOutput := e.recovery.AnalyzeAndInject(call.Name, result.Output)
					log.Printf(" -> [Go-%d] ❌ 注入救援指南: %s\n", idx, finalOutput)
				} else {
					log.Printf(" -> [Go-%d] ✅ 工具执行成功 (返回 %d 字节)\n", idx, len(result.Output))
				}
				if reporter != nil {
					displayOut := result.Output
					if len(displayOut) > 200 {
						displayOut = displayOut[:200] + "...(已截断)"
					}
					reporter.OnToolResult(ctx, call.Name, displayOut, result.IsError)
				}
				// 将工具执行的观察结果封装为 User Message 追加到上下文中
				// 注意：TooCallID 必须携带
				observationMsgs[i] = schema.Message{
					Role:       schema.RoleUser,
					Content:    result.Output,
					ToolCallID: call.ID,
				}
			}(i, call)
		}
		wg.Wait()
		session.Append(observationMsgs...)
	}
	return nil
}
