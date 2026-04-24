package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vearne/agentscope-go/pkg/message"
)

func TestNewChatResponse(t *testing.T) {
	blocks := []message.ContentBlock{
		message.NewTextBlock("hello"),
		message.NewToolUseBlock("call_1", "search", map[string]interface{}{"q": "golang"}),
	}
	cr := NewChatResponse(blocks)
	if cr.Type != "chat" {
		t.Fatalf("expected chat, got %s", cr.Type)
	}
	if len(cr.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(cr.Content))
	}
}

func TestChatResponseGetTextContent(t *testing.T) {
	cr := NewChatResponse([]message.ContentBlock{
		message.NewTextBlock("hello "),
		message.NewThinkingBlock("hmm"),
		message.NewTextBlock("world"),
	})
	if cr.GetTextContent() != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", cr.GetTextContent())
	}
}

func TestChatResponseHasToolUse(t *testing.T) {
	cr1 := NewChatResponse([]message.ContentBlock{message.NewTextBlock("hi")})
	if cr1.HasToolUse() {
		t.Fatal("expected false")
	}

	cr2 := NewChatResponse([]message.ContentBlock{
		message.NewTextBlock("let me search"),
		message.NewToolUseBlock("c1", "search", nil),
	})
	if !cr2.HasToolUse() {
		t.Fatal("expected true")
	}
}

func TestChatResponseGetToolUseBlocks(t *testing.T) {
	cr := NewChatResponse([]message.ContentBlock{
		message.NewTextBlock("hi"),
		message.NewToolUseBlock("c1", "fn1", nil),
		message.NewToolUseBlock("c2", "fn2", nil),
	})
	blocks := cr.GetToolUseBlocks()
	if len(blocks) != 2 {
		t.Fatalf("expected 2, got %d", len(blocks))
	}
}

func TestParseOpenAICompletion(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-123",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "Hello! How can I help?"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5
		}
	}`

	cr, err := ParseOpenAICompletion([]byte(respJSON))
	if err != nil {
		t.Fatal(err)
	}
	if cr.ID != "chatcmpl-123" {
		t.Fatalf("expected chatcmpl-123, got %s", cr.ID)
	}
	if cr.GetTextContent() != "Hello! How can I help?" {
		t.Fatalf("unexpected text: %s", cr.GetTextContent())
	}
	if cr.Usage == nil || cr.Usage.InputTokens != 10 || cr.Usage.OutputTokens != 5 {
		t.Fatal("usage mismatch")
	}
}

func TestParseOpenAICompletionWithToolCalls(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-456",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"id": "call_abc",
					"type": "function",
					"function": {
						"name": "get_weather",
						"arguments": "{\"city\": \"Beijing\"}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}]
	}`

	cr, err := ParseOpenAICompletion([]byte(respJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !cr.HasToolUse() {
		t.Fatal("expected tool use")
	}
	blocks := cr.GetToolUseBlocks()
	if len(blocks) != 1 {
		t.Fatalf("expected 1 tool use block, got %d", len(blocks))
	}
	if message.GetBlockToolUseName(blocks[0]) != "get_weather" {
		t.Fatalf("expected get_weather, got %s", message.GetBlockToolUseName(blocks[0]))
	}
}

func TestParseOpenAICompletionWithThinking(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-789",
		"choices": [{
			"message": {
				"role": "assistant",
				"reasoning_content": "Let me think about this...",
				"content": "Here is my answer."
			},
			"finish_reason": "stop"
		}]
	}`

	cr, err := ParseOpenAICompletion([]byte(respJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(cr.Content) != 2 {
		t.Fatalf("expected 2 blocks (thinking + text), got %d", len(cr.Content))
	}
	if !message.IsThinkingBlock(cr.Content[0]) {
		t.Fatal("expected thinking block first")
	}
	if !message.IsTextBlock(cr.Content[1]) {
		t.Fatal("expected text block second")
	}
}

func TestParseSSEStream(t *testing.T) {
	sseData := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\ndata: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\" World\"}}]}\n\ndata: [DONE]\n\n"

	ch := ParseSSEStream(strings.NewReader(sseData))
	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Last [DONE] breaks parsing so we get 2 valid events + possibly the DONE
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(events[0].Data), &chunk); err != nil {
		t.Fatal(err)
	}
	choices := chunk["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if delta["content"] != "Hello" {
		t.Fatalf("expected Hello, got %v", delta["content"])
	}
}

func TestParseOpenAIStreamChunk(t *testing.T) {
	data := `{"id":"chatcmpl-1","choices":[{"delta":{"content":"Hi"}}]}`
	_, deltaContent, _, _, _, done, err := ParseOpenAIStreamChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if done {
		t.Fatal("expected not done")
	}
	if deltaContent == nil || *deltaContent != "Hi" {
		t.Fatal("expected Hi")
	}
}

func TestParseOpenAIStreamChunkDone(t *testing.T) {
	_, _, _, _, _, done, err := ParseOpenAIStreamChunk("[DONE]")
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("expected done")
	}
}

func TestFormatOpenAITools(t *testing.T) {
	schemas := []ToolSchema{
		{
			Type: "function",
			Function: FuncSchema{
				Name:        "get_weather",
				Description: "Get weather for a city",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}
	result := formatOpenAITools(schemas)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	fn := result[0]["function"].(map[string]interface{})
	if fn["name"] != "get_weather" {
		t.Fatalf("expected get_weather, got %v", fn["name"])
	}
}

func TestFormatOpenAIToolChoice(t *testing.T) {
	if formatOpenAIToolChoice("auto") != "auto" {
		t.Fatal("expected auto")
	}
	if formatOpenAIToolChoice("none") != "none" {
		t.Fatal("expected none")
	}
	specific := formatOpenAIToolChoice("my_func")
	m, ok := specific.(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	fn := m["function"].(map[string]interface{})
	if fn["name"] != "my_func" {
		t.Fatal("expected my_func")
	}
}

func TestValidateToolChoice(t *testing.T) {
	tools := []ToolSchema{{Function: FuncSchema{Name: "search"}}}
	validateToolChoice("auto", tools)
	validateToolChoice("search", tools)
}

func TestBuildStreamResponse(t *testing.T) {
	resp := BuildStreamResponse("id1", "hello", "thinking", nil, nil, nil, nil)
	if resp == nil {
		t.Fatal("expected non-nil")
	}
	if len(resp.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(resp.Content))
	}
	if !message.IsThinkingBlock(resp.Content[0]) {
		t.Fatal("expected thinking first")
	}
	if !message.IsTextBlock(resp.Content[1]) {
		t.Fatal("expected text second")
	}
}

func TestBuildStreamResponseEmpty(t *testing.T) {
	resp := BuildStreamResponse("", "", "", nil, nil, nil, nil)
	if resp != nil {
		t.Fatal("expected nil for empty response")
	}
}
