package studio

import (
	"testing"

	"github.com/vearne/agentscope-go/pkg/message"
)

func TestMsgToPayload_SingleText(t *testing.T) {
	msg := message.NewMsg("user", "hello", "user")
	payload := MsgToPayload(msg)

	if payload["id"] != msg.ID {
		t.Errorf("expected id %s, got %v", msg.ID, payload["id"])
	}
	if payload["name"] != "user" {
		t.Errorf("expected name 'user', got %v", payload["name"])
	}
	if payload["role"] != "user" {
		t.Errorf("expected role 'user', got %v", payload["role"])
	}
	// Single text block should flatten to string
	content, ok := payload["content"].(string)
	if !ok {
		t.Errorf("expected content to be string, got %T", payload["content"])
	}
	if content != "hello" {
		t.Errorf("expected content 'hello', got %s", content)
	}
	if _, ok := payload["timestamp"]; !ok {
		t.Error("expected timestamp field")
	}
}

func TestMsgToPayload_MultipleBlocks(t *testing.T) {
	msg := &message.Msg{
		ID:      "test-id",
		Name:    "assistant",
		Role:    "assistant",
		Content: []message.ContentBlock{
			message.NewTextBlock("thinking..."),
			message.NewToolUseBlock("t1", "calculator", map[string]interface{}{"expr": "2+2"}),
		},
		Timestamp: "2026-04-24 10:00:00.000",
	}
	payload := MsgToPayload(msg)

	// Multiple blocks should remain as []map[string]interface{}
	blocks, ok := payload["content"].([]map[string]interface{})
	if !ok {
		t.Errorf("expected content to be []map[string]interface{}, got %T", payload["content"])
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0]["type"] != "text" {
		t.Errorf("expected first block type 'text', got %v", blocks[0]["type"])
	}
	if blocks[1]["type"] != "tool_use" {
		t.Errorf("expected second block type 'tool_use', got %v", blocks[1]["type"])
	}
}

func TestMsgToPayload_ToolResultBlock(t *testing.T) {
	msg := &message.Msg{
		ID:      "test-id",
		Name:    "tool",
		Role:    "tool",
		Content: []message.ContentBlock{
			message.NewToolResultBlock("t1", "4", false),
		},
		Timestamp: "2026-04-24 10:00:00.000",
	}
	payload := MsgToPayload(msg)

	blocks, ok := payload["content"].([]map[string]interface{})
	if !ok {
		t.Errorf("expected content to be []map[string]interface{}, got %T", payload["content"])
	}
	if blocks[0]["type"] != "tool_result" {
		t.Errorf("expected type 'tool_result', got %v", blocks[0]["type"])
	}
	if blocks[0]["id"] != "t1" {
		t.Errorf("expected id 't1', got %v", blocks[0]["id"])
	}
}

func TestMsgToPayload_NilMetadata(t *testing.T) {
	msg := message.NewMsg("user", "hi", "user")
	payload := MsgToPayload(msg)

	// metadata should be an empty map, not nil
	metadata, ok := payload["metadata"].(map[string]interface{})
	if !ok {
		t.Errorf("expected metadata to be map[string]interface{}, got %T", payload["metadata"])
	}
	if len(metadata) != 0 {
		t.Errorf("expected empty metadata, got %v", metadata)
	}
}

func TestMsgToPayload_NilMsg(t *testing.T) {
	payload := MsgToPayload(nil)
	if payload != nil {
		t.Errorf("expected nil payload for nil msg, got %v", payload)
	}
}
