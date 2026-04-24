package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

type MsgHub struct {
	participants []agent.AgentBase
	mu           sync.RWMutex
	announcement *message.Msg
}

func NewMsgHub(participants []agent.AgentBase, announcement *message.Msg) *MsgHub {
	hub := &MsgHub{
		participants: make([]agent.AgentBase, len(participants)),
		announcement: announcement,
	}
	copy(hub.participants, participants)
	return hub
}

func (h *MsgHub) Add(a agent.AgentBase) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.participants = append(h.participants, a)
}

func (h *MsgHub) Remove(a agent.AgentBase) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, p := range h.participants {
		if p.ID() == a.ID() {
			h.participants = append(h.participants[:i], h.participants[i+1:]...)
			return
		}
	}
}

func (h *MsgHub) Participants() []agent.AgentBase {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]agent.AgentBase, len(h.participants))
	copy(result, h.participants)
	return result
}

func (h *MsgHub) Broadcast(ctx context.Context, msg *message.Msg) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, a := range h.participants {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := a.Observe(ctx, msg); err != nil {
			return fmt.Errorf("broadcast to %s failed: %w", a.Name(), err)
		}
	}
	return nil
}

func (h *MsgHub) Gather(ctx context.Context, msg *message.Msg) ([]*message.Msg, error) {
	h.mu.RLock()
	participants := make([]agent.AgentBase, len(h.participants))
	copy(participants, h.participants)
	h.mu.RUnlock()

	return FanoutPipeline(ctx, participants, msg)
}
