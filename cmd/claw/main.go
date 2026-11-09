package main

import (
	"context"
	ctxpkg "go-tiny-claw/internal/context"
	"go-tiny-claw/internal/engine"
	"go-tiny-claw/internal/provider"
	"go-tiny-claw/internal/schema"
	"go-tiny-claw/internal/tools"
	"log"
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
	workDir, _ := os.Getwd()
	llmProvider := provider.NewGLMOpenAIProvider("deepseek-v4-flash")
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))
	// 引擎本身变成无状态的，它不绑定 WorkDir（仅适用于本讲演示）
	eng := engine.NewAgentEngine(llmProvider, registry, false)
	reporter := engine.NewTerminalReporter()

	sessionID := "test_oom_protection_001"
	sess := ctxpkg.GlobalSessionManager.GetOrCreate(sessionID, workDir)
	prompt := ` 请帮我执行以下三个步骤： 
		1. 使用 bash 执行 echo "开始排查日志" 
		2. 使用 read_file 工具读取当前目录下的巨大文件 mock_log.txt 
		3. 使用 bash 执行 date 命令获取当前时间，并告诉我任务全部完成。 
	`
	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	err := eng.Run(context.Background(), sess, reporter)
	if err != nil {
		log.Fatalf("引擎运行崩溃：%v", err)
	}
}
