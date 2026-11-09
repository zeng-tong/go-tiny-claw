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
	injector       *ReminderInjector
}

func NewAgentEngine(provider provider.LLMProvider, registry tools.Registry, enableThinking bool, planMode bool) *AgentEngine {
	return &AgentEngine{
		provider:  provider,
		registry:  registry,
		compactor: ctxpkg.NewCompactor(3000, 6),
		planMode:  planMode,
		recovery:  ctxpkg.NewRecoveryManager(),
		injector:  NewReminderInjector(),
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

		// 用于收集本轮执行的工具，供 Reminder 探测器分析.
		toolCalls := make([]schema.ToolCall, len(actionResp.ToolCalls))
		toolResults := make([]schema.ToolResult, len(actionResp.ToolCalls))

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
				observationMsgs[idx] = schema.Message{
					Role:       schema.RoleUser,
					Content:    result.Output,
					ToolCallID: call.ID,
				}
				toolCalls[idx] = call
				toolResults[i] = result
			}(i, call)
		}
		wg.Wait()
		// 1. 先将普通的工具执行结果存入 Session
		session.Append(observationMsgs...)
		// 2. 【核心防线】：在准备进入下一轮之前，进行死循环探测！
		for i := 0; i < len(observationMsgs); i++ {
			reminderMsg := e.injector.CheckAndInject(toolCalls[i], toolResults[i])
			if reminderMsg != nil {
				session.Append(*reminderMsg)
			}
		}
	}
	return nil
}

func (e *AgentEngine) RunSub(ctx context.Context, taskPrompt string, readOnlyRegistry tools.Registry, reporter any) (string, error) {
	// 【核心优化】：子智能体极其容易偷懒。我们必须在 System Prompt 中严厉警告它必须使用工具！
	contextHistory := []schema.Message{
		{
			Role: schema.RoleSystem,
			Content: `你是一个专门负责深度探索的探路者 (Explorer Subagent)。你的任务是根据主架构师的指令，在当前工作区内仔细阅读代码、查阅日志，搜集足够的信息。
			【核心纪律】
			 1. 你必须、且只能依靠内置工具（如 bash 的 find/grep，或 read_file）去寻找答案。绝对不允许凭空捏造或猜测！
			 2. 如果你没有找到确切的答案，你必须继续使用工具深入搜索。
			 3. 当且仅当你找到了确切的线索后，停止调用工具，直接输出一段纯文本作为你的终极汇报。主架构师会根据你的汇报来做下一步决策。`,
		},
		{
			Role:    schema.RoleUser,
			Content: taskPrompt,
		},
	}
	// 限制子智能体最多只能跑 10 个 Turn，防止它自己卡死
	const maxSubTurns = 10
	turnCount := 0
	for {
		turnCount++
		if turnCount > maxSubTurns {
			return "", fmt.Errorf("子智能体探索过于深入，超过 %d 轮被强制召回，请主 Agent 给它更明确的指令", maxSubTurns)
		}
		// 【驾驭底线】：子智能体仅能获取传入的只读工具注册表
		availableTools := readOnlyRegistry.GetAvailableTools()
		compactedContext := e.compactor.Compact(contextHistory)
		// 子任务要求急速响应，强制关闭主体的慢思考，直接预测行动
		actionResp, err := e.provider.Generate(ctx, compactedContext, availableTools)
		if err != nil {
			return "", fmt.Errorf("子智能体推理失败: %w", err)
		}
		contextHistory = append(contextHistory, compactedContext...)

		// 【核心退出条件】：子智能体一旦不调用工具了，说明它做好了总结汇报
		if len(actionResp.ToolCalls) == 0 {
			return actionResp.Content, nil
		}
		// 执行只读工具的并发循环
		observationMsgs := make([]schema.Message, len(actionResp.ToolCalls))
		var wg sync.WaitGroup
		for i, toolCall := range actionResp.ToolCalls {
			wg.Add(1)
			go func(idx int, call schema.ToolCall) {
				defer wg.Done()
				// 【可视化的关键】：让终端用户看到 Subagent 正在干嘛
				var r Reporter
				if reporter != nil {
					r = reporter.(Reporter)
					r.OnToolCall(ctx, fmt.Sprintf("[Subagent] %s", call.Name), string(call.Arguments))
				}
				result := e.registry.Execute(ctx, call)
				finalOutput := result.Output
				if result.IsError {
					finalOutput = e.recovery.AnalyzeAndInject(call.Name, result.Output)
				}
				if reporter != nil {
					display := finalOutput
					if len(display) > 200 {
						display = display[:200] + "...(已截断)"
					}
					r.OnToolResult(ctx, fmt.Sprintf("[Subagent] %s", call.Name), display, result.IsError)
				}
				observationMsgs[idx] = schema.Message{
					Role:       schema.RoleUser,
					Content:    result.Output,
					ToolCallID: call.ID,
				}
			}(i, toolCall)
		}
		wg.Wait()
		contextHistory = append(contextHistory, observationMsgs...)
	}
}
