package memory

import (
	"context"

	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/tool"
)

// LongTermMemoryBase is the base interface for long-term memory storage.
// It provides methods for recording and retrieving information over extended time periods.
type LongTermMemoryBase interface {
	// Record records information from the given message(s) to long-term memory.
	// This is a developer-designed method for programmatic memory management.
	Record(ctx context.Context, msgs []*message.Msg) (interface{}, error)

	// Retrieve retrieves information from long-term memory based on the given input message(s).
	// The retrieved information can be added to the agent's system prompt.
	Retrieve(ctx context.Context, msg *message.Msg, limit int) (string, error)

	// RecordToMemory is a tool function for agents to voluntarily record important information.
	// The target content should be specific and concise (e.g., who, when, where, do what, why, how).
	RecordToMemory(ctx context.Context, thinking string, content []string) (*tool.ToolResponse, error)

	// RetrieveFromMemory retrieves memory based on the given keywords.
	// This is a tool function for agents to voluntarily search their long-term memory.
	RetrieveFromMemory(ctx context.Context, keywords []string, limit int) (*tool.ToolResponse, error)
}
