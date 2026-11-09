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
	/*promptPtr := flag.String("prompt", "", "要交给 Agent 执行的任务描述")
	flag.Parse()
	if *promptPtr == "" {
		fmt.Println("用法: go run cmd/claw/main.go -prompt \"你的任务指令\"")
		os.Exit(1)
	}*/
	if os.Getenv("GLM_API_KEY") == "" {
		log.Fatal("请先导出 GLM_API_KEY 环境变量")
	}
	workDir, _ := os.Getwd()
	workDir += "/workspace"
	llmProvider := provider.NewGLMOpenAIProvider("deepseek-v4-flash")
	// 挂载 4 大基础工具
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))
	registry.Register(tools.NewEditFileTool(workDir))

	// 实例化引擎并开启计划模式 (PlanMode=true)
	eng := engine.NewAgentEngine(llmProvider, registry, false, false)
	reporter := engine.NewTerminalReporter()

	// 我们使用一个固定的 SessionID，以便在多次运行之间共享基于内存的“短期工作记忆”。
	// (在真实的 CLI 中，如果进程重启，Session 的内存历史其实是丢失的。
	// 但这正是我们要演示的重点：即便短期内存丢失，只要 TODO.md 还在，任务就能继续！)
	sessionID := "test_recovery_001"
	sess := ctxpkg.GlobalSessionManager.GetOrCreate(sessionID, workDir)

	// 这是一个巨大的陷阱指令：
	// 我们不给它查看文件的机会，直接命令它凭初始上下文去修改文件，目的是诱发 old_text 不匹配的错误。
	prompt := `
    我当前目录下有一个 auth.go 文件。
    请修改 auth.go 中的 login 函数。
    请直接使用 edit_file 工具替换下面的代码块，将判断条件改为同时允许"admin"、"root"和"guest"三种用户登录：
    // 鉴权入口函数
    func login(user string) bool {
        // 检查用户名
        if user == "admin" {
            return true
        }
        return false
    }
	`
	log.Printf("\n>>> 🚀 收到指令: %s\n", prompt)
	// 将用户的 Prompt 压入 Session
	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	// 唤醒引擎执行
	err := eng.Run(context.Background(), sess, reporter)
	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
