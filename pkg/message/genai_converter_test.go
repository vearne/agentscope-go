package message

import (
	"testing"
)

func TestConvertBlockToPart_TextBlock(t *testing.T) {
	block := NewTextBlock("hello world")
	part := ConvertBlockToPart(block)

	if part == nil {
		t.Fatal("part is nil")
	}
	if part["type"] != "text" {
		t.Errorf("expected type 'text', got %v", part["type"])
	}
	if part["content"] != "hello world" {
		t.Errorf("expected content 'hello world', got %v", part["content"])
	}
}

func TestConvertBlockToPart_ThinkingBlock(t *testing.T) {
	block := NewThinkingBlock("let me think")
	part := ConvertBlockToPart(block)

	if part == nil {
		t.Fatal("part is nil")
	}
	if part["type"] != "reasoning" {
		t.Errorf("expected type 'reasoning', got %v", part["type"])
	}
	if part["content"] != "let me think" {
		t.Errorf("expected content 'let me think', got %v", part["content"])
	}
}

func TestConvertBlockToPart_ToolUseBlock(t *testing.T) {
	block := NewToolUseBlock("tool-1", "calculator", map[string]interface{}{"expr": "2+2"})
	part := ConvertBlockToPart(block)

	if part == nil {
		t.Fatal("part is nil")
	}
	if part["type"] != "tool_call" {
		t.Errorf("expected type 'tool_call', got %v", part["type"])
	}
	if part["id"] != "tool-1" {
		t.Errorf("expected id 'tool-1', got %v", part["id"])
	}
	if part["name"] != "calculator" {
		t.Errorf("expected name 'calculator', got %v", part["name"])
	}
	if part["arguments"].(map[string]interface{})["expr"] != "2+2" {
		t.Errorf("unexpected arguments: %v", part["arguments"])
	}
}

func TestConvertBlockToPart_ToolResultBlock(t *testing.T) {
	block := NewToolResultBlock("tool-1", "4", false)
	part := ConvertBlockToPart(block)

	if part == nil {
		t.Fatal("part is nil")
	}
	if part["type"] != "tool_call_response" {
		t.Errorf("expected type 'tool_call_response', got %v", part["type"])
	}
	if part["id"] != "tool-1" {
		t.Errorf("expected id 'tool-1', got %v", part["id"])
	}
	if part["response"] != "4" {
		t.Errorf("expected response '4', got %v", part["response"])
	}
}

func TestConvertBlockToPart_ImageBlock(t *testing.T) {
	block := NewImageBlock(NewURLSource("https://example.com/image.png"))
	part := ConvertBlockToPart(block)

	if part == nil {
		t.Fatal("part is nil")
	}
	if part["type"] != "uri" {
		t.Errorf("expected type 'uri', got %v", part["type"])
	}
	if part["uri"] != "https://example.com/image.png" {
		t.Errorf("expected uri 'https://example.com/image.png', got %v", part["uri"])
	}
	if part["modality"] != "image" {
		t.Errorf("expected modality 'image', got %v", part["modality"])
	}
}

func TestConvertMsgToGenAIMessage(t *testing.T) {
	msg := NewMsg("user", "hello", "user")
	genaiMsg := ConvertMsgToGenAIMessage(msg)

	if genaiMsg.Role != "user" {
		t.Errorf("expected role 'user', got %v", genaiMsg.Role)
	}
	if len(genaiMsg.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(genaiMsg.Parts))
	}
	if genaiMsg.Parts[0]["type"] != "text" {
		t.Errorf("expected part type 'text', got %v", genaiMsg.Parts[0]["type"])
	}
	if genaiMsg.Parts[0]["content"] != "hello" {
		t.Errorf("expected part content 'hello', got %v", genaiMsg.Parts[0]["content"])
	}
	if genaiMsg.FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %v", genaiMsg.FinishReason)
	}
}
