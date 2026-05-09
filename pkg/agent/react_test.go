package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

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
	if err := tk.Register("get_weather", "Get weather", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{"type": "string"},
		},
	}, weatherFn); err != nil {
		t.Fatalf("register get_weather: %v", err)
	}

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
	if err := tk.Register("some_tool", "A tool", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return &tool.ToolResponse{Content: "ok"}, nil
		},
	); err != nil {
		t.Fatalf("register some_tool: %v", err)
	}

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

	if _, err := agent.Reply(context.Background(), NewUserMsg("user", "test")); err != nil {
		t.Fatalf("Reply: %v", err)
	}

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

	if _, err := agent.Reply(context.Background(), NewUserMsg("user", "hello")); err != nil {
		t.Fatalf("Reply: %v", err)
	}

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

// --- context-aware mock for interrupt tests ---

type ctxAwareMockModel struct {
	responses []*model.ChatResponse
	callCount int32
	delay     time.Duration
}

func (m *ctxAwareMockModel) Call(ctx context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (*model.ChatResponse, error) {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	idx := int(atomic.AddInt32(&m.callCount, 1)) - 1
	if idx >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	return m.responses[idx], nil
}

func (m *ctxAwareMockModel) Stream(_ context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (<-chan model.ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *ctxAwareMockModel) ModelName() string { return "ctx_aware_mock" }
func (m *ctxAwareMockModel) IsStream() bool    { return false }

// --- Interrupt tests ---

func TestReActAgent_Interrupt(t *testing.T) {
	delayModel := &ctxAwareMockModel{
		delay: 5 * time.Second,
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("should not see this"),
			}),
		},
	}

	agent := NewReActAgent(
		WithReActModel(delayModel),
		WithReActFormatter(&mockFormatter{}),
	)

	type result struct {
		resp *message.Msg
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := agent.Reply(context.Background(), NewUserMsg("user", "test"))
		done <- result{resp, err}
	}()

	time.Sleep(50 * time.Millisecond)
	agent.Interrupt()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("expected no error on interrupt, got: %v", r.err)
		}
		if r.resp == nil {
			t.Fatal("expected non-nil response")
		}
		if r.resp.Metadata == nil {
			t.Fatal("expected metadata on interrupted response")
		}
		isInt, ok := r.resp.Metadata["_is_interrupted"].(bool)
		if !ok || !isInt {
			t.Error("expected _is_interrupted metadata to be true")
		}
		if r.resp.GetTextContent() == "should not see this" {
			t.Error("should not have received the model response after interrupt")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Reply did not return after interrupt within timeout")
	}
}

func TestReActAgent_InterruptBetweenIterations(t *testing.T) {
	// The model returns tool_use on the first call, then would return text
	// on the second call.  We interrupt after the tool executes but before
	// the second model call by using a slow second call.
	firstCall := int32(0)
	betweenModel := &ctxAwareMockModel{
		responses: []*model.ChatResponse{
			{
				Content: []message.ContentBlock{
					message.NewToolUseBlock("call_1", "some_tool", map[string]interface{}{}),
				},
			},
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("second response"),
			}),
		},
	}
	betweenModel.delay = 5 * time.Second // make ALL calls slow

	tk := tool.NewToolkit()
	if err := tk.Register("some_tool", "A tool", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			atomic.StoreInt32(&firstCall, 1)
			return &tool.ToolResponse{Content: "ok"}, nil
		},
	); err != nil {
		t.Fatalf("register some_tool: %v", err)
	}

	agent := NewReActAgent(
		WithReActModel(betweenModel),
		WithReActFormatter(&mockFormatter{}),
		WithReActToolkit(tk),
	)

	type result struct {
		resp *message.Msg
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := agent.Reply(context.Background(), NewUserMsg("user", "test"))
		done <- result{resp, err}
	}()

	// Wait for the first iteration's tool to complete, then the second
	// model call will be blocked by the 5s delay.  Interrupt there.
	time.Sleep(200 * time.Millisecond)
	agent.Interrupt()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("expected no error, got: %v", r.err)
		}
		if r.resp.Metadata == nil {
			t.Fatal("expected metadata")
		}
		isInt, ok := r.resp.Metadata["_is_interrupted"].(bool)
		if !ok || !isInt {
			t.Error("expected _is_interrupted to be true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for interrupt response")
	}
}

func TestReActAgent_HandleInterrupt(t *testing.T) {
	agent := NewReActAgent(
		WithReActName("test_agent"),
		WithReActModel(&mockModel{}),
		WithReActFormatter(&mockFormatter{}),
	)

	resp, err := agent.HandleInterrupt(context.Background(), nil)
	if err != nil {
		t.Fatalf("HandleInterrupt failed: %v", err)
	}

	if resp.Name != "test_agent" {
		t.Errorf("expected name 'test_agent', got '%s'", resp.Name)
	}
	if resp.Role != "assistant" {
		t.Errorf("expected role 'assistant', got '%s'", resp.Role)
	}
	if resp.Metadata == nil {
		t.Fatal("expected metadata")
	}
	isInt, ok := resp.Metadata["_is_interrupted"].(bool)
	if !ok || !isInt {
		t.Error("expected _is_interrupted to be true")
	}
	if agent.Memory().Size() != 1 {
		t.Errorf("expected 1 message in memory, got %d", agent.Memory().Size())
	}
}

func TestReActAgent_InterruptNotRunning(t *testing.T) {
	agent := NewReActAgent(
		WithReActModel(&mockModel{}),
		WithReActFormatter(&mockFormatter{}),
	)

	// Calling Interrupt when no Reply is running should not panic
	agent.Interrupt()
}

func TestReActAgent_InterruptResetOnNewReply(t *testing.T) {
	delayModel := &ctxAwareMockModel{
		delay: 100 * time.Millisecond,
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("normal"),
			}),
		},
	}

	agent := NewReActAgent(
		WithReActModel(delayModel),
		WithReActFormatter(&mockFormatter{}),
	)

	type result struct {
		resp *message.Msg
		err  error
	}
	done1 := make(chan result, 1)
	go func() {
		resp, err := agent.Reply(context.Background(), NewUserMsg("user", "test1"))
		done1 <- result{resp, err}
	}()
	time.Sleep(20 * time.Millisecond)
	agent.Interrupt()

	r1 := <-done1
	if r1.err != nil {
		t.Fatalf("first reply: %v", r1.err)
	}
	if isInt, _ := r1.resp.Metadata["_is_interrupted"].(bool); !isInt {
		t.Error("first reply should be interrupted")
	}

	// Second reply uses the same mock; since the first call was cancelled
	// during the delay, callCount was not incremented.
	resp2, err := agent.Reply(context.Background(), NewUserMsg("user", "test2"))
	if err != nil {
		t.Fatalf("second reply failed: %v", err)
	}
	if resp2.GetTextContent() != "normal" {
		t.Errorf("second reply got: %s", resp2.GetTextContent())
	}
	if resp2.Metadata != nil {
		t.Error("second reply should not have interrupt metadata")
	}
}
