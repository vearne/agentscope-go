package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

type mockModel struct {
	responses []*model.ChatResponse
	callCount int
}

func (m *mockModel) Call(_ context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (*model.ChatResponse, error) {
	if m.callCount >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func (m *mockModel) Stream(_ context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (<-chan model.ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockModel) ModelName() string { return "mock" }
func (m *mockModel) IsStream() bool    { return false }

type mockFormatter struct{}

func (f *mockFormatter) Format(msgs []*message.Msg) ([]model.FormattedMessage, error) {
	result := make([]model.FormattedMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = model.FormattedMessage{
			"role":    msg.Role,
			"content": msg.GetTextContent(),
		}
	}
	return result, nil
}

func TestReActAgent_SimpleReply(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("Hello! How can I help you?"),
			}),
		},
	}

	agent := NewReActAgent(
		WithReActName("assistant"),
		WithReActModel(mockM),
		WithReActFormatter(&mockFormatter{}),
	)

	msg := NewUserMsg("user", "Hi there")
	resp, err := agent.Reply(context.Background(), msg)
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}

	if resp.Name != "assistant" {
		t.Errorf("expected name 'assistant', got '%s'", resp.Name)
	}
	text := resp.GetTextContent()
	if text != "Hello! How can I help you?" {
		t.Errorf("unexpected text: %s", text)
	}
	if agent.Memory().Size() != 2 {
		t.Errorf("expected 2 messages in memory (user + assistant), got %d", agent.Memory().Size())
	}
}

func TestReActAgent_ToolUse(t *testing.T) {
	tk := tool.NewToolkit()
	weatherFn := func(_ context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
		city, _ := args["city"].(string)
		return &tool.ToolResponse{Content: "sunny, 25°C in " + city}, nil
	}
	tk.Register("get_weather", "Get weather", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{"type": "string"},
		},
	}, weatherFn)

	mockM := &mockModel{
		responses: []*model.ChatResponse{
			{
				Content: []message.ContentBlock{
					message.NewToolUseBlock("call_1", "get_weather", map[string]interface{}{"city": "Beijing"}),
				},
			},
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("The weather in Beijing is sunny, 25°C."),
			}),
		},
	}

	agent := NewReActAgent(
		WithReActName("weather_agent"),
		WithReActModel(mockM),
		WithReActFormatter(&mockFormatter{}),
		WithReActToolkit(tk),
	)

	resp, err := agent.Reply(context.Background(), NewUserMsg("user", "What's the weather in Beijing?"))
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}

	text := resp.GetTextContent()
	if text != "The weather in Beijing is sunny, 25°C." {
		t.Errorf("unexpected final text: %s", text)
	}

	memSize := agent.Memory().Size()
	if memSize != 4 {
		t.Errorf("expected 4 messages (user+assistant+tool+assistant), got %d", memSize)
	}
}

func TestReActAgent_MaxIters(t *testing.T) {
	infiniteToolUse := &mockModel{
		responses: []*model.ChatResponse{},
	}
	for i := 0; i < 20; i++ {
		infiniteToolUse.responses = append(infiniteToolUse.responses,
			&model.ChatResponse{
				Content: []message.ContentBlock{
					message.NewToolUseBlock(fmt.Sprintf("call_%d", i), "some_tool", map[string]interface{}{}),
				},
			},
		)
	}

	tk := tool.NewToolkit()
	tk.Register("some_tool", "A tool", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return &tool.ToolResponse{Content: "ok"}, nil
		},
	)

	agent := NewReActAgent(
		WithReActModel(infiniteToolUse),
		WithReActFormatter(&mockFormatter{}),
		WithReActToolkit(tk),
		WithReActMaxIters(3),
	)

	resp, err := agent.Reply(context.Background(), NewUserMsg("user", "test"))
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if infiniteToolUse.callCount != 3 {
		t.Errorf("expected 3 calls (max_iters), got %d", infiniteToolUse.callCount)
	}
}

func TestReActAgent_Observe(t *testing.T) {
	agent := NewReActAgent(
		WithReActModel(&mockModel{}),
		WithReActFormatter(&mockFormatter{}),
	)

	msg := message.NewMsg("other_agent", "I observed something", "assistant")
	if err := agent.Observe(context.Background(), msg); err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	if agent.Memory().Size() != 1 {
		t.Errorf("expected 1 message after observe, got %d", agent.Memory().Size())
	}
}

func TestReActAgent_Hooks(t *testing.T) {
	preCalled := false
	postCalled := false

	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ok")}),
		},
	}

	agent := NewReActAgent(
		WithReActModel(mockM),
		WithReActFormatter(&mockFormatter{}),
		WithReActPreReply(func(_ context.Context, _ AgentBase, _ *message.Msg, _ *message.Msg) {
			preCalled = true
		}),
		WithReActPostReply(func(_ context.Context, _ AgentBase, _ *message.Msg, _ *message.Msg) {
			postCalled = true
		}),
	)

	agent.Reply(context.Background(), NewUserMsg("user", "test"))

	if !preCalled {
		t.Error("pre-reply hook not called")
	}
	if !postCalled {
		t.Error("post-reply hook not called")
	}
}

func TestReActAgent_NilInput(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ready")}),
		},
	}

	agent := NewReActAgent(
		WithReActModel(mockM),
		WithReActFormatter(&mockFormatter{}),
	)

	resp, err := agent.Reply(context.Background(), nil)
	if err != nil {
		t.Fatalf("Reply with nil input failed: %v", err)
	}
	if resp.GetTextContent() != "ready" {
		t.Errorf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestReActAgent_SystemPrompt(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("done")}),
		},
	}

	agent := NewReActAgent(
		WithReActModel(mockM),
		WithReActFormatter(&mockFormatter{}),
		WithReActSystemPrompt("You are a helpful assistant."),
	)

	agent.Reply(context.Background(), NewUserMsg("user", "hello"))

	msgs := agent.Memory().GetMessages()
	found := false
	for _, m := range msgs {
		if m.Role == "system" && m.GetTextContent() == "You are a helpful assistant." {
			found = true
			break
		}
	}
	if !found {
		t.Error("system prompt not found in memory")
	}
}

func TestNewUserMsg(t *testing.T) {
	msg := NewUserMsg("alice", "hello world")
	if msg.Name != "alice" {
		t.Errorf("expected name 'alice', got '%s'", msg.Name)
	}
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got '%s'", msg.Role)
	}
	if msg.GetTextContent() != "hello world" {
		t.Errorf("unexpected content: %s", msg.GetTextContent())
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ AgentBase = NewReActAgent()
	var _ AgentBase = NewUserAgent("user")
	var _ formatter.FormatterBase = &mockFormatter{}
	var _ memory.MemoryBase = memory.NewInMemoryMemory()
}
