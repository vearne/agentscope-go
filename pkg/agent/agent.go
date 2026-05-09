package agent

import (
	"context"

	"github.com/vearne/agentscope-go/pkg/message"
)

type AgentBase interface {
	Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error)
	Observe(ctx context.Context, msg *message.Msg) error
	Name() string
	ID() string
	// Interrupt cancels the in-flight Reply() call.  Safe to call from any
	// goroutine while Reply() is running.
	Interrupt()
	// HandleInterrupt is called by Reply() after an interrupt is detected.
	// It returns a message with "_is_interrupted" metadata so callers can
	// distinguish graceful interruption from errors.
	HandleInterrupt(ctx context.Context, msg *message.Msg) (*message.Msg, error)
}

type HookFunc func(ctx context.Context, agent AgentBase, msg *message.Msg, resp *message.Msg)

type hooks struct {
	preReply  []HookFunc
	postReply []HookFunc
}
