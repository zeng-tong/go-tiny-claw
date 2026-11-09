package provider

import (
	"context"
	"go-tiny-claw/internal/schema"
)

type LLMProvider interface {
	Generate(ctx context.Context, messages []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error)
}
