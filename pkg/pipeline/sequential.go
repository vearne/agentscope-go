package pipeline

import (
	"context"
	"fmt"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

func SequentialPipeline(ctx context.Context, agents []agent.AgentBase, msg *message.Msg) (*message.Msg, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents provided")
	}

	current := msg
	for i, a := range agents {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := a.Reply(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("agent %d (%s) reply failed: %w", i, a.Name(), err)
		}
		current = resp
	}

	return current, nil
}
