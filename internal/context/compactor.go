package context

import (
	"fmt"
	"go-tiny-claw/internal/schema"
	"log"
)

// Compactor 负责监控和压缩上下文内容，防止 大模型 OOM
type Compactor struct {
	MaxChars       int // 触发压缩的最大字符数阈值
	RetainLastMsgs int // Working Memory 保护区，最近的 N 条消息
}

func NewCompactor(maxChars, retainLastMsgs int) *Compactor {
	return &Compactor{
		MaxChars:       maxChars,
		RetainLastMsgs: retainLastMsgs,
	}
}

// Compactor 接收准备发送给大模型的消息数组
// 如果总长度超标，对远期历史区进行全量掩码，对短期保护区进行超长局部截断。
func (c *Compactor) Compact(msgs []schema.Message) []schema.Message {
	currentLength := c.estimateLength(msgs)

	// 如果没有超过水位线，直接返回原数组 (大多数情况下的正常路径)
	if currentLength < c.MaxChars {
		return msgs
	}
	log.Printf("[Compactor] ⚠️ 内存告警：当前上下文长度 (%d 字符) 超过阈值 (%d)，触发压缩清理...\n", currentLength, c.MaxChars)
	var compacted []schema.Message
	msgCount := len(msgs)

	// 计算受保护的 Working Memory 起始索引
	protectStartIndex := msgCount - c.RetainLastMsgs
	if protectStartIndex < 0 {
		protectStartIndex = 0
	}
	for i, msg := range msgs {
		if msg.Role == schema.RoleSystem {
			compacted = append(compacted, msg)
			continue
		}
		// 我们必须拷贝一份新消息，因为在并发环境中直接修改原引用可能导致底层数据结构被污染
		newMsg := schema.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		}
		isInWorkingMemory := i >= protectStartIndex

		if msg.Role == schema.RoleUser && msg.ToolCallID != "" {
			if !isInWorkingMemory {
				if len(msg.Content) > 200 {
					newMsg.Content = fmt.Sprintf("...[为节省内存，早期工具输出已被系统清理。原始长度 %d 字节]...", len(msg.Content))
				}
			} else {
				const maxKeep = 1000
				if len(msg.Content) > maxKeep {
					head := msg.Content[:500]
					tail := msg.Content[len(msg.Content)-500:]
					newMsg.Content = fmt.Sprintf("%s\n\n...[内容过场，中间 %d 字节已被系统截断]...\n\n%s", head, len(msg.Content)-maxKeep, tail)
				}
			}
		} else if msg.Role == schema.RoleAssistant && msg.Content != "" {
			if !isInWorkingMemory && len(msg.Content) > 200 {
				newMsg.Content = "...[早期的推理思考过程已折叠]..."
			}
		}
		compacted = append(compacted, newMsg)
	}
	newLength := c.estimateLength(compacted)
	log.Printf("[Compactor] ✅ 压缩完成。上下文长度从 %d 降至 %d 字符。\n", currentLength, newLength)
	return compacted
}

func (c *Compactor) estimateLength(msgs []schema.Message) int {
	length := 0
	for _, msg := range msgs {
		length += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			length += len(tc.Name) + len(tc.Arguments)
		}
	}
	return length
}
