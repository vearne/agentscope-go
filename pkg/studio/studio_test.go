package studio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestInit_RegistersRun(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Init(
		WithURL(server.URL),
		WithProject("test-project"),
		WithName("test-run"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer Shutdown(context.Background())

	if receivedBody["project"] != "test-project" {
		t.Errorf("expected project 'test-project', got %v", receivedBody["project"])
	}
	if receivedBody["status"] != "running" {
		t.Errorf("expected status 'running', got %v", receivedBody["status"])
	}
}

func TestInit_SetsGlobalClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Init(WithURL(server.URL), WithProject("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer Shutdown(context.Background())

	client := GetClient()
	if client == nil {
		t.Fatal("expected global client to be set")
	}
	if client.RunID() == "" {
		t.Error("expected non-empty run ID")
	}
}

func TestGetClient_BeforeInit(t *testing.T) {
	Shutdown(context.Background())

	client := GetClient()
	if client != nil {
		t.Error("expected nil client before Init")
	}
}

func TestShutdown_ClearsGlobalClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	Init(WithURL(server.URL), WithProject("test"))

	if GetClient() == nil {
		t.Fatal("expected client after Init")
	}

	Shutdown(context.Background())

	if GetClient() != nil {
		t.Error("expected nil client after Shutdown")
	}
}

func TestInit_Idempotent(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Init(WithURL(server.URL), WithProject("test"))
	if err != nil {
		t.Fatalf("first Init failed: %v", err)
	}
	firstClient := GetClient()

	err = Init(WithURL(server.URL), WithProject("test2"))
	if err != nil {
		t.Fatalf("second Init failed: %v", err)
	}

	if GetClient() != firstClient {
		t.Error("expected same client on second Init")
	}
	if callCount != 1 {
		t.Errorf("expected 1 registerRun call, got %d", callCount)
	}

	Shutdown(context.Background())
}

func TestInit_WithCustomRunID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Init(
		WithURL(server.URL),
		WithProject("test"),
		WithRunID("my-custom-id"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer Shutdown(context.Background())

	if GetClient().RunID() != "my-custom-id" {
		t.Errorf("expected runID 'my-custom-id', got %s", GetClient().RunID())
	}
}

func TestInit_ConcurrentSafe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Init(WithURL(server.URL), WithProject("test"))
		}()
	}
	wg.Wait()
	defer Shutdown(context.Background())

	if GetClient() == nil {
		t.Error("expected client after concurrent Init")
	}
}
