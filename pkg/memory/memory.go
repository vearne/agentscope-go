package memory

import (
	"context"

	"github.com/vearne/agentscope-go/pkg/message"
)

type MemoryReader interface {
	GetMessages() []*message.Msg
}

type MemoryBase interface {
	MemoryReader
	Add(ctx context.Context, msgs ...*message.Msg) error
	Clear(ctx context.Context) error
	Size() int
	ToStrList() []string
}

type TruncatedMemory interface {
	MemoryBase
	Truncate(ctx context.Context, maxSize int) error
}
