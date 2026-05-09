package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

// --- Helpers ---

type errorModel struct{}

func (m *errorModel) Call(_ context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (*model.ChatResponse, error) {
	return nil, fmt.Errorf("model error")
}
func (m *errorModel) Stream(_ context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (<-chan model.ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *errorModel) ModelName() string { return "error-model" }
func (m *errorModel) IsStream() bool    { return false }

// --- OffloadManager Tests ---

func TestOffloadManager_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	om := NewOffloadManager(dir, 100)

	content := "short content"
	result, offloaded := om.MaybeOffload(content, "msg-1")
	if offloaded {
		t.Error("should not offload content below threshold")
	}
	if result != content {
		t.Error("content should be unchanged")
	}
}

func TestOffloadManager_AboveThreshold(t *testing.T) {
	dir := t.TempDir()
	om := NewOffloadManager(dir, 10)

	content := strings.Repeat("a", 500)
	result, offloaded := om.MaybeOffload(content, "msg-2")
	if !offloaded {
		t.Fatal("should offload content above threshold")
	}
	if strings.Contains(result, strings.Repeat("a", 300)) {
		t.Error("result preview should not contain bulk of original content")
	}
	if !strings.Contains(result, "offloaded") {
		t.Error("result should mention offloading")
	}
	if !strings.HasPrefix(result, "[") {
		t.Error("result should start with [")
	}
}

func TestOffloadManager_ReadBack(t *testing.T) {
	dir := t.TempDir()
	om := NewOffloadManager(dir, 10)

	content := strings.Repeat("x", 50)
	_, offloaded := om.MaybeOffload(content, "msg-3")
	if !offloaded {
		t.Fatal("should offload")
	}

	path := filepath.Join(dir, "msg-3.txt")
	readBack, err := om.Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readBack != content {
		t.Error("read-back content should match original")
	}
}

func TestOffloadManager_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "offload")
	om := NewOffloadManager(dir, 5)

	content := "hello world this is long"
	_, offloaded := om.MaybeOffload(content, "msg-4")
	if !offloaded {
		t.Fatal("should offload")
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory should be created")
	}
}

// --- TruncatingCompressor Tests ---

func TestTruncatingCompressor_NoCompression(t *testing.T) {
	c := &TruncatingCompressor{}
	msgs := []*message.Msg{
		message.NewMsg("user", "a", "user"),
		message.NewMsg("assistant", "b", "assistant"),
	}
	result, err := c.Compress(context.Background(), msgs, 5)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages (below threshold), got %d", len(result))
	}
}

func TestTruncatingCompressor_Truncates(t *testing.T) {
	c := &TruncatingCompressor{}
	msgs := make([]*message.Msg, 10)
	for i := range msgs {
		msgs[i] = message.NewMsg("user", fmt.Sprintf("msg %d", i), "user")
	}
	result, err := c.Compress(context.Background(), msgs, 3)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 messages (keepRecent), got %d", len(result))
	}
	text := result[0].GetTextContent()
	if text != "msg 7" {
		t.Errorf("expected first kept message to be 'msg 7', got '%s'", text)
	}
}

// --- LLMCompressor Tests ---

func TestLLMCompressor_Compress(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("Summary: user discussed topic A and B"),
			}),
		},
	}
	c := NewLLMCompressor(mockM, &mockFormatter{}, "")
	msgs := make([]*message.Msg, 8)
	for i := range msgs {
		msgs[i] = message.NewMsg("user", fmt.Sprintf("message %d", i), "user")
	}
	result, err := c.Compress(context.Background(), msgs, 3)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("expected 4 messages (1 summary + 3 recent), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("first message should be summary with role 'user', got '%s'", result[0].Role)
	}
	if !strings.Contains(result[0].GetTextContent(), "Summary") {
		t.Errorf("summary should contain 'Summary', got '%s'", result[0].GetTextContent())
	}
	if result[1].GetTextContent() != "message 5" {
		t.Errorf("expected 'message 5' as first recent, got '%s'", result[1].GetTextContent())
	}
}

func TestLLMCompressor_FallbackOnError(t *testing.T) {
	errorMock := &errorModel{}
	c := NewLLMCompressor(errorMock, &mockFormatter{}, "")
	msgs := []*message.Msg{
		message.NewMsg("user", "a", "user"),
		message.NewMsg("assistant", "b", "assistant"),
	}
	result, err := c.Compress(context.Background(), msgs, 1)
	if err != nil {
		t.Fatalf("should not return error on compression failure")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 original messages on failure, got %d", len(result))
	}
}

// --- SubagentFactory Tests ---

func TestSubagentFactory_Create(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("subagent result"),
			}),
		},
	}
	factory := NewSubagentFactory(mockM, &mockFormatter{}, nil)

	sub := factory.Create(SubagentConfig{
		Name:         "worker-1",
		SystemPrompt: "You are a worker.",
		MaxIters:     5,
	})
	if sub == nil {
		t.Fatal("Create should return non-nil")
	}
	if sub.Name() != "worker-1" {
		t.Errorf("expected name 'worker-1', got '%s'", sub.Name())
	}
	if sub.Memory().Size() != 0 {
		t.Errorf("new subagent memory should be empty, got %d", sub.Memory().Size())
	}
}

func TestSubagentFactory_CreateWithCustomToolkit(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("done"),
			}),
		},
	}
	customTK := tool.NewToolkit()
	if err := customTK.Register("custom_tool", "A custom tool", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return tool.NewToolResponse("custom result"), nil
		},
	); err != nil {
		t.Fatalf("register custom tool: %v", err)
	}

	factory := NewSubagentFactory(mockM, &mockFormatter{}, nil)
	sub := factory.Create(SubagentConfig{
		Name:     "worker-2",
		MaxIters: 3,
		Toolkit:  customTK,
	})
	if sub == nil {
		t.Fatal("Create should return non-nil")
	}
}

func TestSubagentFactory_DelegateTask(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("research complete: found 3 papers"),
			}),
		},
	}
	factory := NewSubagentFactory(mockM, &mockFormatter{}, nil)

	result, err := factory.DelegateTask(context.Background(), DelegateTaskArgs{
		TaskDescription: "Research topic X",
		SubagentName:    "researcher",
		SystemPrompt:    "You are a research assistant.",
	})
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}
	if result != "research complete: found 3 papers" {
		t.Errorf("unexpected result: %s", result)
	}
}

// --- DeepAgent Tests ---

func TestNewDeepAgent_Defaults(t *testing.T) {
	ag := NewDeepAgent()
	if ag == nil {
		t.Fatal("NewDeepAgent should return non-nil")
	}
	if ag.Name() == "" {
		t.Error("should have a default name")
	}
	if ag.maxIters != 50 {
		t.Errorf("default maxIters should be 50, got %d", ag.maxIters)
	}
	if ag.maxCtxTokens != 128000 {
		t.Errorf("default maxCtxTokens should be 128000, got %d", ag.maxCtxTokens)
	}
	if ag.offloadThreshold != 8000 {
		t.Errorf("default offloadThreshold should be 8000, got %d", ag.offloadThreshold)
	}
}

func TestNewDeepAgent_WithOptions(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ok")}),
		},
	}
	ag := NewDeepAgent(
		WithDeepName("test-deep"),
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepMaxIters(100),
		WithDeepMaxContextTokens(200000),
		WithDeepOffloadThreshold(5000),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	if ag.Name() != "test-deep" {
		t.Errorf("expected name 'test-deep', got '%s'", ag.Name())
	}
	if ag.maxIters != 100 {
		t.Errorf("expected maxIters 100, got %d", ag.maxIters)
	}
	if ag.maxCtxTokens != 200000 {
		t.Errorf("expected maxCtxTokens 200000, got %d", ag.maxCtxTokens)
	}
}

func TestNewDeepAgent_InterfaceCompliance(t *testing.T) {
	var _ AgentBase = NewDeepAgent()
}

func TestDeepAgent_SimpleReply(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("Hello from deep agent!"),
			}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	msg := NewUserMsg("user", "Hi")
	resp, err := ag.Reply(context.Background(), msg)
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if resp.GetTextContent() != "Hello from deep agent!" {
		t.Errorf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestDeepAgent_ToolUse(t *testing.T) {
	tk := tool.NewToolkit()
	if err := tk.Register("calc", "Calculate", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return tool.NewToolResponse("42"), nil
		},
	); err != nil {
		t.Fatalf("register calc tool: %v", err)
	}
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			{
				Content: []message.ContentBlock{
					message.NewToolUseBlock("call_1", "calc", map[string]interface{}{"expr": "6*7"}),
				},
			},
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("The answer is 42."),
			}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepToolkit(tk),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	resp, err := ag.Reply(context.Background(), NewUserMsg("user", "What is 6*7?"))
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if resp.GetTextContent() != "The answer is 42." {
		t.Errorf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestDeepAgent_Offloading(t *testing.T) {
	dir := t.TempDir()
	bigResult := strings.Repeat("data line\n", 2000)

	tk := tool.NewToolkit()
	if err := tk.Register("big_tool", "Returns big data", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return tool.NewToolResponse(bigResult), nil
		},
	); err != nil {
		t.Fatalf("register big_tool: %v", err)
	}
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			{
				Content: []message.ContentBlock{
					message.NewToolUseBlock("call_1", "big_tool", map[string]interface{}{}),
				},
			},
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("Done with big data."),
			}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepToolkit(tk),
		WithDeepOffloadDir(dir),
		WithDeepOffloadThreshold(1000),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	resp, err := ag.Reply(context.Background(), NewUserMsg("user", "Get big data"))
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if resp.GetTextContent() != "Done with big data." {
		t.Errorf("unexpected final response: %s", resp.GetTextContent())
	}
	msgs := ag.Memory().GetMessages()
	found := false
	for _, m := range msgs {
		for _, block := range m.Content {
			if output, ok := block["output"].(string); ok && strings.Contains(output, "offloaded") {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Error("expected to find an offloaded reference in memory")
	}
}

func TestDeepAgent_Observe(t *testing.T) {
	ag := NewDeepAgent(
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	msg := message.NewMsg("other", "observed", "assistant")
	if err := ag.Observe(context.Background(), msg); err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if ag.Memory().Size() != 1 {
		t.Errorf("expected 1 message after observe, got %d", ag.Memory().Size())
	}
}

func TestDeepAgent_SystemPrompt(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ok")}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepSystemPrompt("You are a deep agent."),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	if _, err := ag.Reply(context.Background(), NewUserMsg("user", "hello")); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	msgs := ag.Memory().GetMessages()
	found := false
	for _, m := range msgs {
		if m.Role == "system" && m.GetTextContent() == "You are a deep agent." {
			found = true
			break
		}
	}
	if !found {
		t.Error("system prompt not found in memory")
	}
}

func TestDeepAgent_Compression(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{},
	}
	for i := 0; i < 20; i++ {
		mockM.responses = append(mockM.responses,
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock(fmt.Sprintf("response %d", i)),
			}),
		)
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepMaxContextTokens(200),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	if _, err := ag.Reply(context.Background(), NewUserMsg("user", "hello")); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	msgs := ag.Memory().GetMessages()
	if len(msgs) > 20 {
		t.Errorf("expected compression to bound memory, got %d messages", len(msgs))
	}
}

func TestDeepAgent_Hooks(t *testing.T) {
	preCalled := false
	postCalled := false
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ok")}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepCompressor(&TruncatingCompressor{}),
		WithDeepPreReply(func(_ context.Context, _ AgentBase, _ *message.Msg, _ *message.Msg) {
			preCalled = true
		}),
		WithDeepPostReply(func(_ context.Context, _ AgentBase, _ *message.Msg, _ *message.Msg) {
			postCalled = true
		}),
	)
	if _, err := ag.Reply(context.Background(), NewUserMsg("user", "test")); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !preCalled {
		t.Error("pre-reply hook not called")
	}
	if !postCalled {
		t.Error("post-reply hook not called")
	}
}
