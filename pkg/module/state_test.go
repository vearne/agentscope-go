package module

import (
	"sync"
	"testing"
)

func TestStateModuleSetGet(t *testing.T) {
	s := NewStateModule()
	s.SetState("key1", "value1")
	v, ok := s.GetState("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if v != "value1" {
		t.Fatalf("expected value1, got %v", v)
	}
}

func TestStateModuleGetMissing(t *testing.T) {
	s := NewStateModule()
	_, ok := s.GetState("nonexistent")
	if ok {
		t.Fatal("expected false for missing key")
	}
}

func TestStateModuleDelete(t *testing.T) {
	s := NewStateModule()
	s.SetState("key1", "value1")
	s.DeleteState("key1")
	_, ok := s.GetState("key1")
	if ok {
		t.Fatal("expected key1 to be deleted")
	}
}

func TestStateModuleClear(t *testing.T) {
	s := NewStateModule()
	s.SetState("a", 1)
	s.SetState("b", 2)
	s.ClearState()
	if len(s.GetAllState()) != 0 {
		t.Fatal("expected empty state after clear")
	}
}

func TestStateModuleGetAllState(t *testing.T) {
	s := NewStateModule()
	s.SetState("x", 10)
	s.SetState("y", 20)
	all := s.GetAllState()
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
	if all["x"] != 10 || all["y"] != 20 {
		t.Fatalf("unexpected values: %v", all)
	}

	// returned map is a copy
	all["x"] = 999
	v, _ := s.GetState("x")
	if v != 10 {
		t.Fatal("GetAllState should return a copy")
	}
}

func TestStateModuleConcurrent(t *testing.T) {
	s := NewStateModule()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.SetState("key", i)
			s.GetState("key")
			s.GetAllState()
		}(i)
	}
	wg.Wait()
}
