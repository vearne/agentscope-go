package pipeline

import (
	"context"
	"fmt"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

type ChatRoom struct {
	hub       *MsgHub
	agents    []agent.AgentBase
	maxRounds int
}

func NewChatRoom(agents []agent.AgentBase, announcement *message.Msg, maxRounds int) *ChatRoom {
	return &ChatRoom{
		hub:       NewMsgHub(agents, announcement),
		agents:    agents,
		maxRounds: maxRounds,
	}
}

func (cr *ChatRoom) Run(ctx context.Context, msg *message.Msg) ([]*message.Msg, error) {
	var history []*message.Msg
	current := msg

	if cr.hub.announcement != nil {
		if err := cr.hub.Broadcast(ctx, cr.hub.announcement); err != nil {
			return nil, fmt.Errorf("broadcast announcement: %w", err)
		}
	}

	for round := 0; round < cr.maxRounds; round++ {
		select {
		case <-ctx.Done():
			return history, ctx.Err()
		default:
		}

		responses, err := cr.hub.Gather(ctx, current)
		if err != nil {
			return history, fmt.Errorf("round %d gather: %w", round, err)
		}

		history = append(history, responses...)

		for _, resp := range responses {
			stripped := stripThinking(resp)
			if err := cr.hub.Broadcast(ctx, stripped); err != nil {
				return history, fmt.Errorf("round %d broadcast: %w", round, err)
			}
		}

		if len(responses) > 0 {
			current = responses[len(responses)-1]
		}
	}

	return history, nil
}

func stripThinking(msg *message.Msg) *message.Msg {
	filtered := make([]message.ContentBlock, 0, len(msg.Content))
	for _, block := range msg.Content {
		if !message.IsThinkingBlock(block) {
			filtered = append(filtered, block)
		}
	}

	return &message.Msg{
		ID:        msg.ID,
		Name:      msg.Name,
		Role:      msg.Role,
		Content:   filtered,
		Metadata:  msg.Metadata,
		Timestamp: msg.Timestamp,
	}
}
