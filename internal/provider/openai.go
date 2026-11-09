package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
	"go-tiny-claw/internal/schema"
	"os"
)

type OpenAIProvider struct {
	client openai.Client
	model  string
}

func NewGLMOpenAIProvider(model string) *OpenAIProvider {
	apiKey := os.Getenv("GLM_API_KEY")
	baseURL := "https://ark.cn-beijing.volces.com/api/coding/v3"
	if apiKey == "" {
		panic("请设置 GLM_API_KEY 环境变量")
	}
	return &OpenAIProvider{
		client: openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL)),
		model:  model,
	}
}

func (p *OpenAIProvider) Generate(ctx context.Context, messages []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error) {
	var openaiMsgs []openai.ChatCompletionMessageParamUnion
	// 1. 翻译上下文消息
	for _, msg := range messages {
		switch msg.Role {
		case schema.RoleSystem:
			openaiMsgs = append(openaiMsgs, openai.SystemMessage(msg.Content))
		case schema.RoleUser:
			if msg.ToolCallID != "" {
				openaiMsgs = append(openaiMsgs, openai.ToolMessage(msg.Content, msg.ToolCallID))
			} else {
				openaiMsgs = append(openaiMsgs, openai.UserMessage(msg.Content))
			}
		case schema.RoleAssistant:
			astParam := openai.ChatCompletionAssistantMessageParam{}
			if msg.Content != "" {
				astParam.Content = openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(msg.Content)}
			}
			if len(msg.ToolCalls) > 0 {
				var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
				for _, toolCall := range msg.ToolCalls {
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID:   toolCall.ID,
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Arguments: string(toolCall.Arguments),
								Name:      toolCall.Name,
							},
						},
					})
				}
				astParam.ToolCalls = toolCalls
			}
			openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &astParam,
			})
		}
	}
	// 2. 翻译工具
	var openaiTools []openai.ChatCompletionToolUnionParam
	for _, tool := range availableTools {
		var params shared.FunctionParameters
		if m, ok := tool.InputSchema.(map[string]interface{}); ok {
			params = shared.FunctionParameters(m)
		} else {
			b, _ := json.Marshal(tool.InputSchema)
			_ = json.Unmarshal(b, &params)
		}
		openaiTools = append(openaiTools, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  params,
		}))
	}
	// 3. 构建 param 并发送
	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: openaiMsgs,
	}

	// 慢思考机制, 仅当 availableTools 存在才挂载
	if len(openaiTools) > 0 {
		params.Tools = openaiTools
	}

	// 4. 请求 llm
	//stream, err := p.client.Chat.Completions.NewStreaming(ctx, params)
	//for stream.Next() {
	//	stream.Current()
	//}
	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("OPENAI/Zhipu API 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OPENAI/Zhipu API 返回了空 Choices")
	}

	// 5. api response 反向翻译为内部 Message
	choice := resp.Choices[0].Message
	resultMsg := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: choice.Content,
	}
	for _, tc := range choice.ToolCalls {
		if tc.Type == "function" {
			resultMsg.ToolCalls = append(resultMsg.ToolCalls, schema.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: []byte(tc.Function.Arguments),
			})
		}
	}

	return resultMsg, nil
}
