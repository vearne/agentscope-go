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
}

type HookFunc func(ctx context.Context, agent AgentBase, msg *message.Msg, resp *message.Msg)

type hooks struct {
	preReply  []HookFunc
	postReply []HookFunc
}
