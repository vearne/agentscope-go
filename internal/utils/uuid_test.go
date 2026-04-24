package utils

import (
	"sync"
	"testing"
)

func TestShortUUID(t *testing.T) {
	id := ShortUUID()
	if len(id) != 22 {
		t.Fatalf("expected length 22, got %d", len(id))
	}
}

func TestShortUUIDUniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		id := ShortUUID()
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestTimestampID(t *testing.T) {
	id := TimestampID()
	if len(id) == 0 {
		t.Fatal("expected non-empty id")
	}
	// Format: 20060102_150405_xxxxxx (22 chars)
	if len(id) != 22 {
		t.Fatalf("expected length 22, got %d: %s", len(id), id)
	}
}

func TestTimestampIDUniqueness(t *testing.T) {
	var mu sync.Mutex
	seen := make(map[string]struct{})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := TimestampID()
			mu.Lock()
			if _, ok := seen[id]; ok {
				t.Errorf("duplicate id: %s", id)
			}
			seen[id] = struct{}{}
			mu.Unlock()
		}()
	}
	wg.Wait()
}

func BenchmarkShortUUID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ShortUUID()
	}
}


