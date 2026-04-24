package model

import (
	"context"

	"github.com/vearne/agentscope-go/pkg/message"
)

type ChatResponse struct {
	Content   []message.ContentBlock    `json:"content"`
	ID        string                    `json:"id"`
	CreatedAt string                    `json:"created_at"`
	Type      string                    `json:"type"`
	Usage     *ChatUsage                `json:"usage,omitempty"`
	Metadata  map[string]interface{}    `json:"metadata,omitempty"`
}

type ChatUsage struct {
	InputTokens  int                    `json:"input_tokens"`
	OutputTokens int                    `json:"output_tokens"`
	Time         float64                `json:"time"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type CallOption struct {
	Tools       []ToolSchema
	ToolChoice  string
	Temperature *float64
	MaxTokens   *int
	Stop        []string
}

type ToolSchema struct {
	Type     string     `json:"type"`
	Function FuncSchema `json:"function"`
}

type FuncSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type FormattedMessage map[string]interface{}

type ChatModelBase interface {
	Call(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (*ChatResponse, error)
	Stream(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (<-chan ChatResponse, error)
	ModelName() string
	IsStream() bool
}

func NewChatResponse(blocks []message.ContentBlock) *ChatResponse {
	return &ChatResponse{
		Content: blocks,
		Type:    "chat",
	}
}

func (r *ChatResponse) GetTextContent() string {
	return message.ContentToString(r.Content)
}

func (r *ChatResponse) HasToolUse() bool {
	for _, b := range r.Content {
		if message.IsToolUseBlock(b) {
			return true
		}
	}
	return false
}

func (r *ChatResponse) GetToolUseBlocks() []message.ContentBlock {
	var blocks []message.ContentBlock
	for _, b := range r.Content {
		if message.IsToolUseBlock(b) {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

func validateToolChoice(toolChoice string, tools []ToolSchema) {
	validModes := map[string]bool{"auto": true, "none": true, "required": true}
	if validModes[toolChoice] {
		return
	}
	for _, t := range tools {
		if t.Function.Name == toolChoice {
			return
		}
	}
}
