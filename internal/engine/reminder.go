package engine

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"go-tiny-claw/internal/schema"
	"log"
)

type ReminderInjector struct {
	consecutiveFailures map[string]int
}

func NewReminderInjector() *ReminderInjector {
	return &ReminderInjector{
		consecutiveFailures: make(map[string]int),
	}
}

// generateFingerprint 生成工具调用的唯一指纹，用于判断大模型是否在重复相同的动作
func generateFingerprint(toolName string, args []byte) string {
	hash := md5.New()
	hash.Write([]byte(toolName))
	hash.Write(args)
	return hex.EncodeToString(hash.Sum(nil))
}

func (r *ReminderInjector) CheckAndInject(lastToolCall schema.ToolCall, lastResult schema.ToolResult) *schema.Message {
	fingerprint := generateFingerprint(lastToolCall.Name, lastToolCall.Arguments)
	if !lastResult.IsError {
		r.consecutiveFailures = map[string]int{} // 执行成功了就清除计数
		return nil
	}
	r.consecutiveFailures[fingerprint]++
	failCount := r.consecutiveFailures[fingerprint]
	log.Printf("[Reminder] 监控到工具 %s 执行失败，该参数特征连续失败次数: %d\n", lastToolCall.Name, failCount)

	// 【驾驭底线】：触发死循环打断机制！
	if failCount > 3 {
		log.Println("[Reminder] ⚠️ 触发死循环干预！注入强力修正指令。")
		// 构造一条极其严厉的行动指南
		nudgeMsg := fmt.Sprintf(`[SYSTEM REMINDER 警告] 
		你似乎陷入了死循环。你刚刚连续 %d 次使用相同的参数调用了 '%s' 工具，并且都失败了。
		请立即停止这种无效的重试！你的注意力被当前的报错过度吸引了。
		你需要：
		1. 停止猜测参数。跳出当前的局部思维。
		2. 彻底改变你的策略。
		3. 如果你确实无法通过系统工具解决当前问题，请直接结束任务并向用户说明你需要什么人工帮助，而不是继续盲目消耗 API 资源尝试。`, failCount, lastToolCall.Name)
		return &schema.Message{
			Role:    schema.RoleUser,
			Content: nudgeMsg,
		}
	}
	return nil
}
