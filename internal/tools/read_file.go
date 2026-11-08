package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go-tiny-claw/internal/context/schema"
	"io"
	"os"
	"path/filepath"
)

type ReadFileTool struct {
	wordDir string
}

func (r *ReadFileTool) Name() string {
	return "read_file"
}

func (r *ReadFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        r.Name(),
		Description: "读取指定路径的文件内容。请提供相对工作区的路径",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "要读取的文件路径，如 cmd/claw/main.go",
				},
			},
			"required": []string{"path"},
		},
	}
}

type readFileArgs struct {
	Path string `json:"path"`
}

func (r *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var inputArgs readFileArgs
	if err := json.Unmarshal(args, &inputArgs); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	fullPath := filepath.Join(r.wordDir, inputArgs.Path)

	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("读取文件内容失败: %w", err)
	}
	const maxLen = 8000
	if len(content) > maxLen {
		truncatedMsg := fmt.Sprintf("%s\n\n...[由于内容过长，已被系统截断至前 %d 字节]...", content[:maxLen], maxLen)
		return truncatedMsg, nil
	}
	return string(content), nil
}

func NewReadFileTool(wordDir string) *ReadFileTool {
	return &ReadFileTool{
		wordDir: wordDir,
	}
}
