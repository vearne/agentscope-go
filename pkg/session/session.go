package session

import (
	"context"

	"github.com/vearne/agentscope-go/pkg/memory"
)

type SessionBase interface {
	Save(ctx context.Context, mem memory.MemoryBase) error
	Load(ctx context.Context, mem memory.MemoryBase) error
}
