package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-tiny-claw/internal/schema"
	"os/exec"
	"time"
)

type BashTool struct {
	workDir string
}

func NewBashTool(workDir string) *BashTool {
	return &BashTool{workDir: workDir}
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.Name(),
		Description: "在当前工作区执行任意的 bash 命令。支持链式命理（如 &&）。返回标准输出 stdout 和 标准错误 stderr。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "要执行的 bash 命令，例如：ls -la 或 go test ./..",
				},
			},
			"required": []string{"command"},
		},
	}
}

type bashArgs struct {
	Command string `json:"command"`
}

func (t *BashTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input bashArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	// 【驾驭底线 1】：Time Budgeting (时间预算和超时控制)
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 【驾驭底线 2】：绑定执行的工作区目录
	cmd := exec.CommandContext(timeoutCtx, "bash", "-c", input.Command)
	cmd.Dir = t.workDir

	// 执行并捕获 CombineOutput (合并 stdout 和 stderr)
	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
		return outputStr + "\n[警告: 命令执行超时(30s)，已被系统强制终止。如果是启动常驻服务，请尝试将其转入后台。]", nil
	}
	// 【驾驭底线 3】错误原样回传 （Self-Correction 自愈机制）。bash 报错时，不能返回 Go error 阻断程序。把 Err 信息拼成字符串返回，利用大模型自纠错能力，自己分析报错。
	if err != nil {
		return fmt.Sprintf("执行报错: %v\n输出：\n%s", err, outputStr), nil
	}

	// 如果没有终端输出，给模型一个明确的执行成功的反馈。
	if outputStr == "" {
		return "命令执行成功，无终端输出", nil
	}

	// 【驾驭底线 4】 长度截断保护
	const maxLen = 8000
	if len(outputStr) > maxLen {
		return fmt.Sprintf("%s\n\n...[终端输入过长，已截断至前 %d 字节]...", outputStr[:maxLen], maxLen), nil
	}
	return outputStr, nil
}
