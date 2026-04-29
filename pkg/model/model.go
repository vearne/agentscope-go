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
	// 工具相关
	Tools      []ToolSchema
	ToolChoice string

	// 生成参数
	Temperature *float64
	TopP        *float64
	TopK        *int
	MaxTokens   *int
	Stop        []string

	// OpenAI 特定参数
	PresencePenalty  *float64
	FrequencyPenalty *float64
	Seed             *int
	ResponseFormat   *ResponseFormat

	// 用户标识
	User *string
}

// ResponseFormat defines the response format for OpenAI API
type ResponseFormat struct {
	Type string `json:"type"` // "text" or "json_object"
}

// ModelConfig holds default configuration parameters for a model
type ModelConfig struct {
	// 基础参数
	Temperature *float64
	TopP        *float64
	TopK        *int
	MaxTokens   *int
	Stop        []string

	// OpenAI 特定参数
	PresencePenalty  *float64
	FrequencyPenalty *float64
	Seed             *int
	ResponseFormat   *ResponseFormat

	// 用户标识
	User *string
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

func configValueFloat64(defaultVal *float64, optVal *float64) *float64 {
	if optVal != nil {
		return optVal
	}
	return defaultVal
}

func configValueInt(defaultVal *int, optVal *int) *int {
	if optVal != nil {
		return optVal
	}
	return defaultVal
}

func configValueResponseFormat(defaultVal *ResponseFormat, optVal *ResponseFormat) *ResponseFormat {
	if optVal != nil {
		return optVal
	}
	return defaultVal
}

func configValueString(defaultVal *string, optVal *string) *string {
	if optVal != nil {
		return optVal
	}
	return defaultVal
}
