package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go-tiny-claw/internal/schema"
	"os"
	"path/filepath"
)

type WriteFileTool struct {
	workDir string
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func NewWriteFileTool(workDir string) *WriteFileTool {
	return &WriteFileTool{workDir: workDir}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        t.Name(),
		Description: "创建或覆盖写入一个文件。如果目录不存在会自动创建。请提供相对于工作区的相对路径。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "要写入的文件路径，如 src/main.go",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "要写入文件的完整内容",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input writeFileArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	fullPath := filepath.Join(t.workDir, input.Path) // [安全防线]： 限制在 workDir 下执行，防止大模型修改系统级文件。
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("创建父目录失败: %w", err)
	}

	err := os.WriteFile(fullPath, []byte(input.Content), 0644)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}
	return fmt.Sprintf("成功将内容写入到文件：%s", input.Content), nil
}
