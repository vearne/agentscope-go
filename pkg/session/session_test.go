package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
)

func TestJSONSession_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.json")
	ctx := context.Background()

	origMem := memory.NewInMemoryMemory()
	msg1 := message.NewMsg("user", "hello", "user")
	msg2 := message.NewMsg("assistant", "hi there", "assistant")
	if err := origMem.Add(ctx, msg1, msg2); err != nil {
		t.Fatalf("add messages: %v", err)
	}

	sess := NewJSONSession(filePath)
	if err := sess.Save(ctx, origMem); err != nil {
		t.Fatalf("save: %v", err)
	}

	loadedMem := memory.NewInMemoryMemory()
	if err := sess.Load(ctx, loadedMem); err != nil {
		t.Fatalf("load: %v", err)
	}

	msgs := loadedMem.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("unexpected roles: %s, %s", msgs[0].Role, msgs[1].Role)
	}
	if msgs[0].GetTextContent() != "hello" {
		t.Errorf("expected 'hello', got '%s'", msgs[0].GetTextContent())
	}
	if msgs[1].GetTextContent() != "hi there" {
		t.Errorf("expected 'hi there', got '%s'", msgs[1].GetTextContent())
	}
}

func TestJSONSession_LoadNonexistent(t *testing.T) {
	sess := NewJSONSession("/tmp/agentscope_test_nonexistent_session.json")
	ctx := context.Background()

	mem := memory.NewInMemoryMemory()
	err := sess.Load(ctx, mem)
	if err == nil {
		t.Fatal("expected error loading nonexistent file")
	}
}

func TestJSONSession_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "roundtrip.json")
	ctx := context.Background()

	origMem := memory.NewInMemoryMemory()
	msg := message.NewMsg("assistant", "complex message", "assistant")
	msg.Metadata = map[string]interface{}{
		"key1": "value1",
		"key2": float64(42),
	}
	if err := origMem.Add(ctx, msg); err != nil {
		t.Fatalf("add message: %v", err)
	}

	sess := NewJSONSession(filePath)
	if err := sess.Save(ctx, origMem); err != nil {
		t.Fatalf("save: %v", err)
	}

	loadedMem := memory.NewInMemoryMemory()
	if err := sess.Load(ctx, loadedMem); err != nil {
		t.Fatalf("load: %v", err)
	}

	loaded := loadedMem.GetMessages()
	if len(loaded) != 1 {
		t.Fatalf("expected 1 message, got %d", len(loaded))
	}

	orig := origMem.GetMessages()[0]
	got := loaded[0]

	if got.ID != orig.ID {
		t.Errorf("ID mismatch: %s != %s", got.ID, orig.ID)
	}
	if got.Name != orig.Name {
		t.Errorf("Name mismatch: %s != %s", got.Name, orig.Name)
	}
	if got.Role != orig.Role {
		t.Errorf("Role mismatch: %s != %s", got.Role, orig.Role)
	}
	if got.Timestamp != orig.Timestamp {
		t.Errorf("Timestamp mismatch: %s != %s", got.Timestamp, orig.Timestamp)
	}
	if got.Metadata["key1"] != "value1" {
		t.Errorf("Metadata key1 mismatch: %v", got.Metadata["key1"])
	}
	if got.Metadata["key2"] != float64(42) {
		t.Errorf("Metadata key2 mismatch: %v", got.Metadata["key2"])
	}
}

func TestJSONSession_EmptyMemory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.json")
	ctx := context.Background()

	mem := memory.NewInMemoryMemory()
	sess := NewJSONSession(filePath)

	if err := sess.Save(ctx, mem); err != nil {
		t.Fatalf("save empty: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected file to be created")
	}

	loadedMem := memory.NewInMemoryMemory()
	if err := sess.Load(ctx, loadedMem); err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if loadedMem.Size() != 0 {
		t.Fatalf("expected 0 messages, got %d", loadedMem.Size())
	}
}
