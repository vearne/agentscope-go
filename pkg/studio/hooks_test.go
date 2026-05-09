package studio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/vearne/agentscope-go/pkg/message"
)

func TestForwardMessage_PushesResponse(t *testing.T) {
	var pushCount int32
	var lastBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&pushCount, 1)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newStudioClient(server.URL, "run-123", "test", "test")
	globalClient = client
	defer func() { globalClient = nil }()

	resp := message.NewMsg("assistant", "hello from agent", "assistant")
	ForwardMessage(context.Background(), "test-agent", "assistant", resp)

	if atomic.LoadInt32(&pushCount) != 1 {
		t.Errorf("expected 1 pushMessage call, got %d", pushCount)
	}
	if lastBody["runId"] != "run-123" {
		t.Errorf("expected runId 'run-123', got %v", lastBody["runId"])
	}
	if lastBody["replyRole"] != "assistant" {
		t.Errorf("expected replyRole 'assistant', got %v", lastBody["replyRole"])
	}
	if lastBody["replyName"] != "test-agent" {
		t.Errorf("expected replyName 'test-agent', got %v", lastBody["replyName"])
	}
}

func TestForwardMessage_PushesUserInput(t *testing.T) {
	var lastBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newStudioClient(server.URL, "run-123", "test", "test")
	globalClient = client
	defer func() { globalClient = nil }()

	inputMsg := message.NewMsg("user", "what is 2+2?", "user")
	ForwardMessage(context.Background(), "user", "user", inputMsg)

	if lastBody["replyRole"] != "user" {
		t.Errorf("expected replyRole 'user', got %v", lastBody["replyRole"])
	}
	if lastBody["runId"] != "run-123" {
		t.Errorf("expected runId 'run-123', got %v", lastBody["runId"])
	}
}

func TestForwardMessage_NilClient_Noop(t *testing.T) {
	globalClient = nil

	msg := message.NewMsg("user", "hello", "user")
	ForwardMessage(context.Background(), "user", "user", msg)
}

func TestForwardMessage_NilMessage_Noop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not have made an HTTP call")
	}))
	defer server.Close()

	client := newStudioClient(server.URL, "run-123", "test", "test")
	globalClient = client
	defer func() { globalClient = nil }()

	ForwardMessage(context.Background(), "user", "user", nil)
}
