package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"go-tiny-claw/internal/engine"
	"log"
	"os"
	"strings"
)

// FeishuBot 封装了飞书机器人的配置与核心业务流
type FeishuBot struct {
	client    *lark.Client
	appID     string
	appSecret string
	engine    *engine.AgentEngine // 持有核心引擎引用
}

func NewFeishuBot(eng *engine.AgentEngine) *FeishuBot {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	if appID == "" || appSecret == "" {
		log.Fatalf("请设置 FEISHU_APP_ID 和 FEISHU_APP_SECRET")
	}
	return &FeishuBot{
		client:    lark.NewClient(appID, appSecret),
		appID:     appID,
		appSecret: appSecret,
		engine:    eng,
	}
}

func (e *FeishuBot) GetEventDispatcher() *dispatcher.EventDispatcher {
	encryptKey := os.Getenv("FEISHU_ENCRYPT_KEY")
	verifyToken := os.Getenv("FEISHU_VERIFY_TOKEN")
	handler := dispatcher.NewEventDispatcher(verifyToken, encryptKey).OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		contentStr := *event.Event.Message.Content
		contentStr = strings.TrimPrefix(contentStr, `{"text":"`)
		contentStr = strings.TrimSuffix(contentStr, `"}`)

		chatID := *event.Event.Message.ChatId
		log.Printf("[Feishu] 收到会话 %s 消息: %s\n", chatID, contentStr)
		// [驾驭并发]： 收到消息后，不能阻塞 http 回调
		go e.handlerAgentRun(chatID, contentStr)
		return nil
	}).OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
		return nil // 消息已读事件，静默忽略。
	})
	return handler
}

func (e *FeishuBot) handlerAgentRun(chatID string, promopt string) {
	reporter := &FeishuReporter{
		client: e.client,
		chatId: chatID,
	}
	err := e.engine.Run(context.Background(), promopt, reporter)
	if err != nil {
		reporter.sendMsg(fmt.Sprintf("❌ Agent 运行崩溃: %v", err))
	}
}

type FeishuReporter struct {
	client *lark.Client
	chatId string
}

// sendMsg 封装了调用飞书 OpenAPI 发送卡片/文本的操作
func (r *FeishuReporter) sendMsg(text string) {
	textContent := map[string]string{
		"text": text,
	}
	content, _ := json.Marshal(textContent)
	contentStr := string(content)

	msgReq := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.CreateMessageV1ReceiveIDTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().ReceiveId(r.chatId).MsgType(larkim.MsgTypeText).Content(contentStr).Build()).
		Build()
	_, _ = r.client.Im.Message.Create(context.Background(), msgReq)
}

func (r *FeishuReporter) OnThinking(ctx context.Context) {
	// 仅发一个轻量级提示，避免飞书刷屏
	r.sendMsg("🤔 模型正在慢思考 (Thinking)...")
}

func (r *FeishuReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	r.sendMsg(fmt.Sprintf("🛠️ **正在执行工具**：`%s`\n参数：`%s`", toolName, args))
}

func (r *FeishuReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	if isError {
		r.sendMsg(fmt.Sprintf("⚠️ **执行报错** (%s)：\n%s", toolName, result))
	} else {
		// 成功时仅汇报成功，不刷全量日志
		r.sendMsg(fmt.Sprintf("✅ **执行成功** (%s)", toolName))
	}
}

func (r *FeishuReporter) OnMessage(ctx context.Context, content string) {
	// 将模型最终的纯文本回答发给用户
	r.sendMsg(content)
}

// 编译时类型检查，确保 FeishuReporter 实现了 Reporter 接口
var _ engine.Reporter = (*FeishuReporter)(nil)
