package message

import (
	"encoding/json"
	"testing"
)

func TestNewMsgWithStringContent(t *testing.T) {
	m := NewMsg("assistant", "hello world", "assistant")
	if m.Name != "assistant" {
		t.Fatalf("expected assistant, got %s", m.Name)
	}
	if m.Role != "assistant" {
		t.Fatalf("expected assistant, got %s", m.Role)
	}
	if len(m.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.Content))
	}
	if !IsTextBlock(m.Content[0]) {
		t.Fatal("expected text block")
	}
	if m.GetTextContent() != "hello world" {
		t.Fatalf("expected 'hello world', got %s", m.GetTextContent())
	}
}

func TestNewMsgWithContentBlock(t *testing.T) {
	b := NewTextBlock("hi")
	m := NewMsg("user", b, "user")
	if len(m.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.Content))
	}
}

func TestNewMsgWithContentBlocks(t *testing.T) {
	blocks := []ContentBlock{
		NewTextBlock("part1"),
		NewTextBlock("part2"),
	}
	m := NewMsg("assistant", blocks, "assistant")
	if len(m.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(m.Content))
	}
	if m.GetTextContent() != "part1part2" {
		t.Fatalf("expected 'part1part2', got %s", m.GetTextContent())
	}
}

func TestMsgSetContent(t *testing.T) {
	m := NewMsg("test", "initial", "user")
	m.SetContent("updated")
	if m.GetTextContent() != "updated" {
		t.Fatalf("expected 'updated', got %s", m.GetTextContent())
	}

	blocks := []ContentBlock{NewTextBlock("a"), NewTextBlock("b")}
	m.SetContent(blocks)
	if m.GetTextContent() != "ab" {
		t.Fatalf("expected 'ab', got %s", m.GetTextContent())
	}
}

func TestMsgClone(t *testing.T) {
	m := NewMsg("assistant", "original", "assistant")
	m.Metadata = map[string]interface{}{"key": "value"}
	m.InvocationID = "inv_1"

	clone := m.Clone()
	if clone == nil {
		t.Fatal("expected non-nil clone")
	}
	if clone.ID != m.ID {
		t.Fatalf("expected %s, got %s", m.ID, clone.ID)
	}
	if clone.GetTextContent() != "original" {
		t.Fatalf("expected 'original', got %s", clone.GetTextContent())
	}

	clone.SetContent("modified")
	if m.GetTextContent() != "original" {
		t.Fatal("original msg should not be affected by clone modification")
	}
}

func TestMsgJSONRoundTrip(t *testing.T) {
	m := NewMsg("assistant", "hello", "assistant")
	m.Content = []ContentBlock{
		NewTextBlock("hello"),
		NewToolUseBlock("call_1", "search", map[string]interface{}{"q": "golang"}),
		NewToolResultBlock("call_1", "result data", false),
	}
	m.Metadata = map[string]interface{}{"turn": 1}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Msg
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "assistant" {
		t.Fatalf("expected assistant, got %s", decoded.Name)
	}
	if len(decoded.Content) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(decoded.Content))
	}
	if !IsTextBlock(decoded.Content[0]) {
		t.Fatal("expected text block at index 0")
	}
	if !IsToolUseBlock(decoded.Content[1]) {
		t.Fatal("expected tool_use block at index 1")
	}
	if !IsToolResultBlock(decoded.Content[2]) {
		t.Fatal("expected tool_result block at index 2")
	}
	if GetBlockToolUseName(decoded.Content[1]) != "search" {
		t.Fatalf("expected search, got %s", GetBlockToolUseName(decoded.Content[1]))
	}
}

func TestContentToString(t *testing.T) {
	blocks := []ContentBlock{
		NewTextBlock("hello "),
		NewThinkingBlock("hmm"),
		NewTextBlock("world"),
	}
	result := ContentToString(blocks)
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", result)
	}
}

func TestMsgIDUnique(t *testing.T) {
	ids := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		m := NewMsg("test", "hi", "user")
		if _, ok := ids[m.ID]; ok {
			t.Fatalf("duplicate id: %s", m.ID)
		}
		ids[m.ID] = struct{}{}
	}
}
