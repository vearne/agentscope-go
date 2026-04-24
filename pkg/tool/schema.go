package tool

import "context"

type ToolResponse struct {
	Content interface{} `json:"content"`
	IsError bool        `json:"is_error,omitempty"`
}

type ToolFunc func(ctx context.Context, args map[string]interface{}) (*ToolResponse, error)
