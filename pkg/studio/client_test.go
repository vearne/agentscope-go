package studio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClient_RegisterRun(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trpc/registerRun" {
			t.Errorf("expected path /trpc/registerRun, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &StudioClient{
		baseURL:    server.URL,
		runID:      "test-run-id",
		project:    "test-project",
		name:       "test-run",
		httpClient: server.Client(),
	}

	err := client.RegisterRun(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["id"] != "test-run-id" {
		t.Errorf("expected id 'test-run-id', got %v", receivedBody["id"])
	}
	if receivedBody["project"] != "test-project" {
		t.Errorf("expected project 'test-project', got %v", receivedBody["project"])
	}
	if receivedBody["status"] != "running" {
		t.Errorf("expected status 'running', got %v", receivedBody["status"])
	}
}

func TestClient_PushMessage(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trpc/pushMessage" {
			t.Errorf("expected path /trpc/pushMessage, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &StudioClient{
		baseURL:    server.URL,
		runID:      "test-run-id",
		httpClient: server.Client(),
	}

	req := &PushMessageRequest{
		RunID:     "test-run-id",
		ReplyID:   "msg-1",
		ReplyName: "assistant",
		ReplyRole: "assistant",
		Msg: map[string]interface{}{
			"id":       "msg-1",
			"name":     "assistant",
			"role":     "assistant",
			"content":  "hello",
			"metadata": map[string]interface{}{},
		},
	}

	err := client.PushMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["runId"] != "test-run-id" {
		t.Errorf("expected runId 'test-run-id', got %v", receivedBody["runId"])
	}
	if receivedBody["replyRole"] != "assistant" {
		t.Errorf("expected replyRole 'assistant', got %v", receivedBody["replyRole"])
	}
}

func TestClient_PushMessage_RetryOnFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &StudioClient{
		baseURL:    server.URL,
		runID:      "test-run-id",
		httpClient: server.Client(),
	}

	req := &PushMessageRequest{
		RunID:     "test-run-id",
		ReplyID:   "msg-1",
		ReplyName: "assistant",
		ReplyRole: "assistant",
		Msg:       map[string]interface{}{"id": "msg-1"},
	}

	err := client.PushMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", callCount)
	}
}

func TestClient_RunID(t *testing.T) {
	client := &StudioClient{runID: "my-run"}
	if got := client.RunID(); got != "my-run" {
		t.Errorf("expected 'my-run', got %s", got)
	}
}

func TestClient_PID(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &StudioClient{
		baseURL:    server.URL,
		runID:      "test-run-id",
		project:    "test-project",
		name:       "test-run",
		httpClient: server.Client(),
	}

	_ = client.RegisterRun(context.Background())

	pid, ok := receivedBody["pid"].(float64)
	if !ok {
		t.Fatalf("expected pid to be a number, got %T", receivedBody["pid"])
	}
	if int(pid) != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), int(pid))
	}
}
