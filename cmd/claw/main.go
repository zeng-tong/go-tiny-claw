package main

import (
	"context"
	"go-tiny-claw/internal/engine"
	"go-tiny-claw/internal/provider"
	"go-tiny-claw/internal/schema"
	"go-tiny-claw/internal/tools"
	"log"
	"os"
	"sync"
	"time"
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

/*
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
	eng := engine.NewAgentEngine(p, r, workDir, false)

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
}*/
/*
func main() {
	// 1. 初始化引擎依赖
	// 确保已设置 GLM_API_KEY
	if os.Getenv("GLM_API_KEY") == "" {
		log.Fatal("请先导出 GLM_API_KEY 环境变量")
	}
	workDir, _ := os.Getwd()
	workDir += "/workspace"

	p := provider.NewGLMOpenAIProvider("deepseek-v4-flash")
	r := tools.NewRegistry()
	r.Register(tools.NewReadFileTool(workDir))
	r.Register(tools.NewWriteFileTool(workDir))
	r.Register(tools.NewBashTool(workDir))
	r.Register(tools.NewEditFileTool(workDir))

	eng := engine.NewAgentEngine(p, r, workDir, false)
	reporter := engine.NewTerminalReporter()

	prompt := `
		我需要在当前目录下新建一个 ping.go，提供一个简单的 http ping 接口。 写完之后，帮我把代码用 git 提交一下。
	`
	err := eng.Run(context.Background(), prompt, reporter)
	if err != nil {
		log.Fatalf("服务启动失败：%v", err)
	}
}*/

// cmd/claw/main.go

func main() {
	if os.Getenv("GLM_API_KEY") == "" {
		log.Fatal("请先导出 GLM_API_KEY 环境变量")
	}
	llmProvider := provider.NewGLMOpenAIProvider("deepseek-v4-flash")
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool("tmp/project_front"))

	// 引擎本身变成无状态的，它不绑定 WorkDir（仅适用于本讲演示）
	eng := engine.NewAgentEngine(llmProvider, registry, false)
	reporter := engine.NewTerminalReporter()

	var wg sync.WaitGroup

	// ================= 模拟并发场景 1：飞书前端群 =================
	wg.Add(1)
	go func() {
		defer wg.Done()
		sessionA := engine.GlobalSessionManager.GetOrCreate("chat_front_001", "tmp/project_front")

		// 回合 1：获取机密
		log.Println("\n>>> 🙋‍♂️ [Session A / Turn 1]: 帮我看看 README.md 里记录了什么密钥？")
		sessionA.Append(schema.Message{Role: schema.RoleUser, Content: "帮我看看 README.md 里记录了什么密钥？"})
		_ = eng.Run(context.Background(), sessionA, reporter)

		// 故意制造大量“废话”对话，刷掉记忆 (假设 Working Memory Limit=6)
		for i := 0; i < 6; i++ {
			sessionA.Append(schema.Message{Role: schema.RoleUser, Content: "这只是一句闲聊占位符。"})
			sessionA.Append(schema.Message{Role: schema.RoleAssistant, Content: "好的，收到闲聊。"})
		}

		// 回合 2：验证记忆截断 (此时第一轮的密钥已经被挤出 Working Memory 了！)
		log.Println("\n>>> 🙋‍♂️ [Session A / Turn 2]: 请直接告诉我，刚才第一轮你查到的那个密钥是什么？")
		sessionA.Append(schema.Message{Role: schema.RoleUser, Content: "请直接告诉我，刚才第一轮你查到的那个密钥是什么？不准调用工具！"})
		_ = eng.Run(context.Background(), sessionA, reporter)
	}()

	// ================= 模拟并发场景 2：飞书后端群 =================
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 稍微错开一点时间发起请求
		time.Sleep(1 * time.Second)

		sessionB := engine.GlobalSessionManager.GetOrCreate("chat_back_002", "tmp/project_back")

		log.Println("\n>>> 🙋‍♂️ [Session B]: 别人查到了一个密钥，你这里能看到吗？")
		sessionB.Append(schema.Message{Role: schema.RoleUser, Content: "别人查到了一个密钥，你这里能看到吗？不准调用工具！"})
		_ = eng.Run(context.Background(), sessionB, reporter)
	}()

	wg.Wait()
}
