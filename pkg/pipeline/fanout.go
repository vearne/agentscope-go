package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

func FanoutPipeline(ctx context.Context, agents []agent.AgentBase, msg *message.Msg) ([]*message.Msg, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents provided")
	}

	results := make([]*message.Msg, len(agents))
	errs := make([]error, len(agents))

	var wg sync.WaitGroup
	for i, a := range agents {
		wg.Add(1)
		go func(idx int, ag agent.AgentBase) {
			defer wg.Done()
			resp, err := ag.Reply(ctx, msg)
			results[idx] = resp
			errs[idx] = err
		}(i, a)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("agent %d (%s) failed: %w", i, agents[i].Name(), err)
		}
	}

	return results, nil
}
