package memory

import (
	"context"

	"github.com/vearne/agentscope-go/pkg/message"
)

// MemoryReader is the interface for reading messages from memory.
type MemoryReader interface {
	GetMessages() []*message.Msg
}

// MemoryBase is the base interface for memory storage in agentscope-go.
// It supports adding, retrieving, deleting, and managing messages with marks.
type MemoryBase interface {
	MemoryReader
	// Add adds message(s) into the memory storage with optional marks.
	// If marks is nil or empty, no marks are associated with the messages.
	Add(ctx context.Context, msgs ...*message.Msg) error

	// AddWithMarks adds message(s) into the memory storage with specified marks.
	// Each message will be associated with all the provided marks.
	AddWithMarks(ctx context.Context, msgs []*message.Msg, marks []string) error

	// Clear clears all messages from the storage.
	Clear(ctx context.Context) error

	// Delete removes message(s) from the storage by their IDs.
	// Returns the number of messages removed.
	Delete(ctx context.Context, msgIDs []string) (int, error)

	// DeleteByMark removes messages from the memory by their marks.
	// Returns the number of messages removed.
	DeleteByMark(ctx context.Context, mark []string) (int, error)

	// GetMemory retrieves messages from the memory with optional filtering.
	// If mark is non-empty, only messages with that mark are returned.
	// If excludeMark is non-empty, messages with that mark are excluded.
	// If prependSummary is true, the compressed summary is prepended as a message.
	GetMemory(ctx context.Context, mark string, excludeMark string, prependSummary bool) ([]*message.Msg, error)

	// Size returns the number of messages in the storage.
	Size() int

	// ToStrList converts messages to a list of strings.
	ToStrList() []string

	// UpdateCompressedSummary updates the compressed summary of the memory.
	UpdateCompressedSummary(ctx context.Context, summary string) error

	// UpdateMessagesMark updates marks of messages in the storage.
	// If msgIDs is provided, the update applies to those messages.
	// If oldMark is provided, the update applies to messages with that mark.
	// If newMark is empty, the mark is removed.
	// Returns the number of messages updated.
	UpdateMessagesMark(ctx context.Context, newMark string, oldMark string, msgIDs []string) (int, error)
}

// TruncatedMemory extends MemoryBase with truncation capability.
type TruncatedMemory interface {
	MemoryBase
	Truncate(ctx context.Context, maxSize int) error
}
