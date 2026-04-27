package tool

import (
	"context"
	"time"

	"github.com/vearne/agentscope-go/pkg/message"
)

// ToolResponse represents the result of a tool invocation.
// Content holds the payload (typically a string or []message.ContentBlock for
// multimodal results). Metadata is optional structured data for internal use
// by the agent without needing to parse Content.
type ToolResponse struct {
	Content       interface{}            `json:"content"`
	IsError       bool                   `json:"is_error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	IsLast        bool                   `json:"is_last,omitempty"`
	IsInterrupted bool                   `json:"is_interrupted,omitempty"`
	ID            string                 `json:"id,omitempty"`
}

// ToolFunc is the function signature that every tool must implement.
type ToolFunc func(ctx context.Context, args map[string]interface{}) (*ToolResponse, error)

// NewToolResponse creates a ToolResponse with text content.
func NewToolResponse(content string) *ToolResponse {
	return &ToolResponse{
		Content: content,
		IsLast:  true,
		ID:      formatTimestamp(),
	}
}

// NewErrorResponse creates a ToolResponse that indicates an error.
func NewErrorResponse(content string) *ToolResponse {
	return &ToolResponse{
		Content: content,
		IsError: true,
		IsLast:  true,
		ID:      formatTimestamp(),
	}
}

// NewContentBlockResponse creates a ToolResponse with typed content blocks
// (TextBlock, ImageBlock, AudioBlock, VideoBlock).
func NewContentBlockResponse(blocks []message.ContentBlock) *ToolResponse {
	return &ToolResponse{
		Content: blocks,
		IsLast:  true,
		ID:      formatTimestamp(),
	}
}

// NewStreamResponse creates a streaming ToolResponse chunk.
// Set isLast to true for the final chunk in the stream.
func NewStreamResponse(content string, isLast bool) *ToolResponse {
	return &ToolResponse{
		Content: content,
		Stream:  true,
		IsLast:  isLast,
		ID:      formatTimestamp(),
	}
}

// WithMetadata attaches metadata to the response and returns it for chaining.
func (r *ToolResponse) WithMetadata(metadata map[string]interface{}) *ToolResponse {
	r.Metadata = metadata
	return r
}

// formatTimestamp returns a millisecond-precision timestamp string used as
// the ToolResponse ID, matching the Python agentscope convention.
func formatTimestamp() string {
	return time.Now().Format("20060102_150405.000")
}
