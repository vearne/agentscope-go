package message

import (
	"encoding/json"
	"testing"
)

func TestNewTextBlock(t *testing.T) {
	b := NewTextBlock("hello")
	if GetBlockType(b) != BlockText {
		t.Fatalf("expected text, got %s", GetBlockType(b))
	}
	if GetBlockText(b) != "hello" {
		t.Fatalf("expected hello, got %s", GetBlockText(b))
	}
	if !IsTextBlock(b) {
		t.Fatal("expected true")
	}
}

func TestNewThinkingBlock(t *testing.T) {
	b := NewThinkingBlock("let me think")
	if GetBlockType(b) != BlockThinking {
		t.Fatalf("expected thinking, got %s", GetBlockType(b))
	}
	if GetBlockThinking(b) != "let me think" {
		t.Fatalf("expected 'let me think', got %s", GetBlockThinking(b))
	}
	if !IsThinkingBlock(b) {
		t.Fatal("expected true")
	}
}

func TestNewToolUseBlock(t *testing.T) {
	input := map[string]interface{}{"city": "Beijing"}
	b := NewToolUseBlock("call_123", "get_weather", input)
	if GetBlockType(b) != BlockToolUse {
		t.Fatalf("expected tool_use, got %s", GetBlockType(b))
	}
	if GetBlockToolUseID(b) != "call_123" {
		t.Fatalf("expected call_123, got %s", GetBlockToolUseID(b))
	}
	if GetBlockToolUseName(b) != "get_weather" {
		t.Fatalf("expected get_weather, got %s", GetBlockToolUseName(b))
	}
	if !IsToolUseBlock(b) {
		t.Fatal("expected true")
	}
}

func TestNewToolResultBlock(t *testing.T) {
	b := NewToolResultBlock("call_123", "sunny, 25°C", false)
	if GetBlockType(b) != BlockToolResult {
		t.Fatalf("expected tool_result, got %s", GetBlockType(b))
	}
	if GetBlockToolResultID(b) != "call_123" {
		t.Fatalf("expected call_123, got %s", GetBlockToolResultID(b))
	}
	if GetBlockToolResultOutput(b) != "sunny, 25°C" {
		t.Fatalf("expected 'sunny, 25°C', got %v", GetBlockToolResultOutput(b))
	}
	if GetBlockToolResultIsError(b) {
		t.Fatal("expected false")
	}
	if !IsToolResultBlock(b) {
		t.Fatal("expected true")
	}
}

func TestToolResultBlockIsError(t *testing.T) {
	b := NewToolResultBlock("call_err", "something failed", true)
	if !GetBlockToolResultIsError(b) {
		t.Fatal("expected true")
	}
}

func TestNewImageBlock(t *testing.T) {
	src := NewBase64Source("image/png", "iVBORw==")
	b := NewImageBlock(src)
	if GetBlockType(b) != BlockImage {
		t.Fatalf("expected image, got %s", GetBlockType(b))
	}
	if !IsImageBlock(b) {
		t.Fatal("expected true")
	}
}

func TestNewAudioBlock(t *testing.T) {
	src := NewURLSource("https://example.com/audio.mp3")
	b := NewAudioBlock(src)
	if GetBlockType(b) != BlockAudio {
		t.Fatalf("expected audio, got %s", GetBlockType(b))
	}
	if !IsAudioBlock(b) {
		t.Fatal("expected true")
	}
}

func TestNewVideoBlock(t *testing.T) {
	src := NewBase64Source("video/mp4", "AAAA")
	b := NewVideoBlock(src)
	if GetBlockType(b) != BlockVideo {
		t.Fatalf("expected video, got %s", GetBlockType(b))
	}
	if !IsVideoBlock(b) {
		t.Fatal("expected true")
	}
}

func TestBlockJSONRoundTrip(t *testing.T) {
	b := NewToolUseBlock("call_1", "search", map[string]interface{}{"q": "golang"})
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if GetBlockType(decoded) != BlockToolUse {
		t.Fatalf("expected tool_use after round-trip, got %s", GetBlockType(decoded))
	}
	if GetBlockToolUseName(decoded) != "search" {
		t.Fatalf("expected search after round-trip, got %s", GetBlockToolUseName(decoded))
	}
}
