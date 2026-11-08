package main

import (
	"context"
	"github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
	"go-tiny-claw/internal/context/schema"
	"go-tiny-claw/internal/engine"
	"go-tiny-claw/internal/feishu"
	"go-tiny-claw/internal/provider"
	"go-tiny-claw/internal/tools"
	"log"
	"net/http"
	"os"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
/*
func main() {
	fmt.Println("🚀 welcome to go-tiny-claw, engine is staring")
	// TODO 1. 初始化模型
	// provider := provider.NewClaudeProvider()

	// TODO 2. 初始化 工具仓库  Tool Repository
	// registry := tools.NewRegistry()
	// registry.Register(tools.NewBashTool())

	// TODO 3. 初始化 上下文管理器
	// ctxManager := context.NewManager()

	// TODO 4.组装并启动核心引擎
	// engine := engine.NewAgentEngine(provider, registry, ctxManager)

	// fmt.Println("开始执行任务")
	// err := engine.Run("帮我检查一下当前目录下的文件并输出一个 README.md 大纲")
	// if err != nil {
	//   log.Fatalf("引擎运行崩溃: %v", err)
	//}
	log.Println("架构搭建完毕")
}*/

func main() {
	// 1. 初始化引擎依赖
	// 确保已设置 GLM_API_KEY
	if os.Getenv("GLM_API_KEY") == "" {
		log.Fatal("请先导出 GLM_API_KEY 环境变量")
	}
	workDir, _ := os.Getwd()
	p := provider.NewGLMOpenAIProvider("deepseek-v4-flash")
	//r := &mockRegistry{}
	r := tools.NewRegistry()
	r.Register(tools.NewReadFileTool(workDir))
	r.Register(tools.NewWriteFileTool(workDir))
	r.Register(tools.NewBashTool(workDir))
	r.Register(tools.NewEditFileTool(workDir))
	eng := engine.NewAgentEngine(p, r, workDir, true)

	// 2. 初始化飞书 Bot 调度器
	bot := feishu.NewFeishuBot(eng)
	handler := httpserverext.NewEventHandlerFunc(bot.GetEventDispatcher())
	// 3. 注册并启动 HTTP 服务
	http.HandleFunc("/webhook/event", handler)
	port := ":48080"
	log.Printf("🚀 go-tiny-claw 飞书服务端已启动，正在监听 %s 端口\n", port)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("服务启动失败：%v", err)
	}
}

type mockProvider struct {
	turn int
}

func (m *mockProvider) Generate(ctx context.Context, msgs []schema.Message, tools []schema.ToolDefinition) (*schema.Message, error) {
	// 如果工具列表为空，说明这是引擎发起的 Phase 1: Thinking 阶段
	if len(tools) == 0 {
		return &schema.Message{
			Role:    schema.RoleAssistant,
			Content: "【推理中】目标是检查文件。我不能直接盲猜，我需要先调用 bash 工具执行 ls 命令，看看当前目录下有什么，然后再做定夺。",
		}, nil
	}
	// 如果工具列表不为空，说明这是 Phase 2: Action 阶段
	m.turn++
	if m.turn == 1 {
		return &schema.Message{
			Role:    schema.RoleAssistant,
			Content: "让我看看当前目录下有什么文件",
			ToolCalls: []schema.ToolCall{
				{
					ID:        "call_123",
					Name:      "bash",
					Arguments: []byte(`{"command": "ls -la"}`),
				},
			},
		}, nil
	}
	return &schema.Message{
		Role:    schema.RoleAssistant,
		Content: "我看到了 main.go\n",
	}, nil
}

type mockRegistry struct {
}

func (m *mockRegistry) GetAvailableTools() []schema.ToolDefinition {
	return []schema.ToolDefinition{
		{
			Name:        "get_weather",
			Description: "获取指定城市的当前天气情况",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"city"},
			},
		},
	}
}

func (m *mockRegistry) Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult {
	log.Printf("[Mock 工具执行] 获取 %s 的天气中...", call.Name)
	return schema.ToolResult{
		ToolCallID: call.ID,
		Output:     "API 返回： 今天是晴天，气温 25度",
		IsError:    false,
	}
}
