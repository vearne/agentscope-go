package memory

import (
	"context"
	"sync"
	"testing"

	"github.com/vearne/agentscope-go/pkg/message"
)

func TestNewInMemoryMemory(t *testing.T) {
	m := NewInMemoryMemory()
	if m == nil {
		t.Fatal("NewInMemoryMemory returned nil")
	}
	if m.Size() != 0 {
		t.Fatalf("expected empty memory, got size %d", m.Size())
	}
}

func TestInMemoryMemory_AddAndGet(t *testing.T) {
	m := NewInMemoryMemory()
	ctx := context.Background()

	msg1 := message.NewMsg("user", "hello", "user")
	msg2 := message.NewMsg("assistant", "hi there", "assistant")

	if err := m.Add(ctx, msg1); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := m.Add(ctx, msg2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if m.Size() != 2 {
		t.Fatalf("expected size 2, got %d", m.Size())
	}

	msgs := m.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Name != "user" {
		t.Errorf("expected msg0 name=user, got %s", msgs[0].Name)
	}
	if msgs[1].Name != "assistant" {
		t.Errorf("expected msg1 name=assistant, got %s", msgs[1].Name)
	}
}

func TestInMemoryMemory_AddMultiple(t *testing.T) {
	m := NewInMemoryMemory()
	ctx := context.Background()

	msg1 := message.NewMsg("user", "first", "user")
	msg2 := message.NewMsg("assistant", "second", "assistant")
	msg3 := message.NewMsg("user", "third", "user")

	if err := m.Add(ctx, msg1, msg2, msg3); err != nil {
		t.Fatalf("Add multiple failed: %v", err)
	}

	if m.Size() != 3 {
		t.Fatalf("expected size 3, got %d", m.Size())
	}
}

func TestInMemoryMemory_Clear(t *testing.T) {
	m := NewInMemoryMemory()
	ctx := context.Background()

	m.Add(ctx, message.NewMsg("user", "hello", "user"))
	if m.Size() != 1 {
		t.Fatalf("expected size 1, got %d", m.Size())
	}

	if err := m.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if m.Size() != 0 {
		t.Fatalf("expected size 0 after clear, got %d", m.Size())
	}

	msgs := m.GetMessages()
	if len(msgs) != 0 {
		t.Fatalf("expected empty messages after clear, got %d", len(msgs))
	}
}

func TestInMemoryMemory_ToStrList(t *testing.T) {
	m := NewInMemoryMemory()
	ctx := context.Background()

	m.Add(ctx, message.NewMsg("user", "hello world", "user"))
	m.Add(ctx, message.NewMsg("assistant", "hi there", "assistant"))

	strs := m.ToStrList()
	if len(strs) != 2 {
		t.Fatalf("expected 2 strings, got %d", len(strs))
	}
	if strs[0] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", strs[0])
	}
	if strs[1] != "hi there" {
		t.Errorf("expected 'hi there', got '%s'", strs[1])
	}
}

func TestInMemoryMemory_GetMessagesReturnsCopy(t *testing.T) {
	m := NewInMemoryMemory()
	ctx := context.Background()

	m.Add(ctx, message.NewMsg("user", "hello", "user"))
	msgs := m.GetMessages()

	msgs[0] = nil

	msgs2 := m.GetMessages()
	if msgs2[0] == nil {
		t.Fatal("GetMessages did not return a copy")
	}
}

func TestInMemoryMemory_Empty(t *testing.T) {
	m := NewInMemoryMemory()

	if m.Size() != 0 {
		t.Fatalf("expected size 0, got %d", m.Size())
	}
	if len(m.GetMessages()) != 0 {
		t.Fatal("expected empty messages")
	}
	if len(m.ToStrList()) != 0 {
		t.Fatal("expected empty string list")
	}
}

func TestInMemoryMemory_Concurrent(t *testing.T) {
	m := NewInMemoryMemory()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m.Add(ctx, message.NewMsg("user", "msg", "user"))
		}(i)
	}
	wg.Wait()

	if m.Size() != 100 {
		t.Fatalf("expected size 100 after concurrent adds, got %d", m.Size())
	}
}
