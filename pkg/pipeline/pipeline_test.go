package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

type mockAgent struct {
	id       string
	name     string
	response string
	observed []*message.Msg
}

func (a *mockAgent) Reply(_ context.Context, msg *message.Msg) (*message.Msg, error) {
	resp := message.NewMsg(a.name, a.response, "assistant")
	return resp, nil
}

func (a *mockAgent) Observe(_ context.Context, msg *message.Msg) error {
	a.observed = append(a.observed, msg)
	return nil
}

func (a *mockAgent) Name() string { return a.name }
func (a *mockAgent) ID() string   { return a.id }

func (a *mockAgent) Interrupt() {}

func (a *mockAgent) HandleInterrupt(_ context.Context, _ *message.Msg) (*message.Msg, error) {
	return nil, fmt.Errorf("mockAgent does not support interrupt handling")
}

func TestSequentialPipeline(t *testing.T) {
	agents := []agent.AgentBase{
		&mockAgent{id: "1", name: "agent_a", response: "response_a"},
		&mockAgent{id: "2", name: "agent_b", response: "response_b"},
		&mockAgent{id: "3", name: "agent_c", response: "response_c"},
	}

	msg := message.NewMsg("user", "hello", "user")
	result, err := SequentialPipeline(context.Background(), agents, msg)
	if err != nil {
		t.Fatalf("SequentialPipeline failed: %v", err)
	}

	if result.Name != "agent_c" {
		t.Errorf("expected last agent 'agent_c', got '%s'", result.Name)
	}
	if result.GetTextContent() != "response_c" {
		t.Errorf("unexpected content: %s", result.GetTextContent())
	}
}

func TestSequentialPipeline_SingleAgent(t *testing.T) {
	agents := []agent.AgentBase{
		&mockAgent{id: "1", name: "only", response: "only_response"},
	}

	result, err := SequentialPipeline(context.Background(), agents, message.NewMsg("user", "hi", "user"))
	if err != nil {
		t.Fatalf("SequentialPipeline failed: %v", err)
	}
	if result.GetTextContent() != "only_response" {
		t.Errorf("unexpected: %s", result.GetTextContent())
	}
}

func TestSequentialPipeline_Empty(t *testing.T) {
	_, err := SequentialPipeline(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for empty agents")
	}
}

func TestSequentialPipeline_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := SequentialPipeline(ctx, []agent.AgentBase{
		&mockAgent{id: "1", name: "a", response: "r"},
	}, message.NewMsg("user", "hi", "user"))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFanoutPipeline(t *testing.T) {
	agents := []agent.AgentBase{
		&mockAgent{id: "1", name: "agent_a", response: "resp_a"},
		&mockAgent{id: "2", name: "agent_b", response: "resp_b"},
		&mockAgent{id: "3", name: "agent_c", response: "resp_c"},
	}

	results, err := FanoutPipeline(context.Background(), agents, message.NewMsg("user", "hello", "user"))
	if err != nil {
		t.Fatalf("FanoutPipeline failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	if !names["agent_a"] || !names["agent_b"] || !names["agent_c"] {
		t.Errorf("missing agent responses: %v", names)
	}
}

func TestFanoutPipeline_Empty(t *testing.T) {
	_, err := FanoutPipeline(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for empty agents")
	}
}

func TestMsgHub(t *testing.T) {
	a1 := &mockAgent{id: "1", name: "agent_a"}
	a2 := &mockAgent{id: "2", name: "agent_b"}

	hub := NewMsgHub([]agent.AgentBase{a1, a2}, nil)

	msg := message.NewMsg("system", "welcome", "system")
	if err := hub.Broadcast(context.Background(), msg); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	if len(a1.observed) != 1 {
		t.Errorf("agent_a should have 1 observed msg, got %d", len(a1.observed))
	}
	if len(a2.observed) != 1 {
		t.Errorf("agent_b should have 1 observed msg, got %d", len(a2.observed))
	}
}

func TestMsgHub_AddRemove(t *testing.T) {
	hub := NewMsgHub(nil, nil)

	a1 := &mockAgent{id: "1", name: "a"}
	a2 := &mockAgent{id: "2", name: "b"}

	hub.Add(a1)
	hub.Add(a2)

	if len(hub.Participants()) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(hub.Participants()))
	}

	hub.Remove(a1)
	if len(hub.Participants()) != 1 {
		t.Fatalf("expected 1 participant after remove, got %d", len(hub.Participants()))
	}
}

func TestMsgHub_Gather(t *testing.T) {
	a1 := &mockAgent{id: "1", name: "a", response: "resp_a"}
	a2 := &mockAgent{id: "2", name: "b", response: "resp_b"}

	hub := NewMsgHub([]agent.AgentBase{a1, a2}, nil)

	results, err := hub.Gather(context.Background(), message.NewMsg("user", "hello", "user"))
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestChatRoom(t *testing.T) {
	a1 := &mockAgent{id: "1", name: "alice", response: "hi from alice"}
	a2 := &mockAgent{id: "2", name: "bob", response: "hi from bob"}

	room := NewChatRoom([]agent.AgentBase{a1, a2}, nil, 2)

	history, err := room.Run(context.Background(), message.NewMsg("user", "hello everyone", "user"))
	if err != nil {
		t.Fatalf("ChatRoom.Run failed: %v", err)
	}

	if len(history) == 0 {
		t.Fatal("expected non-empty history")
	}
}

func TestChatRoom_WithAnnouncement(t *testing.T) {
	a1 := &mockAgent{id: "1", name: "a", response: "r"}
	announcement := message.NewMsg("system", "Welcome to the chat!", "system")

	room := NewChatRoom([]agent.AgentBase{a1}, announcement, 1)

	history, err := room.Run(context.Background(), message.NewMsg("user", "hi", "user"))
	if err != nil {
		t.Fatalf("ChatRoom.Run failed: %v", err)
	}

	if len(a1.observed) < 1 {
		t.Error("agent should have observed the announcement")
	}
	if len(history) == 0 {
		t.Fatal("expected non-empty history")
	}
}

func TestStripThinking(t *testing.T) {
	msg := &message.Msg{
		ID:   "test",
		Name: "agent",
		Role: "assistant",
		Content: []message.ContentBlock{
			message.NewThinkingBlock("internal thought"),
			message.NewTextBlock("visible text"),
		},
	}

	stripped := stripThinking(msg)
	if len(stripped.Content) != 1 {
		t.Fatalf("expected 1 block after strip, got %d", len(stripped.Content))
	}
	if message.GetBlockType(stripped.Content[0]) != message.BlockText {
		t.Error("expected text block after stripping thinking")
	}
}
