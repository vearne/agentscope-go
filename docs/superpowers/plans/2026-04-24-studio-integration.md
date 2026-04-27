# agentscope-studio Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add agentscope-studio data reporting to agentscope-go, enabling Go agents to report runs and messages to an agentscope-studio server.

**Architecture:** New `pkg/studio/` package with a global `Init()` function that registers a run and auto-injects message-forwarding hooks into ReActAgent instances. HTTP client talks to studio's tRPC endpoints. OTLP HTTP tracing forwards spans to studio's `/v1/traces`.

**Tech Stack:** Go stdlib `net/http`, existing `pkg/tracing` (OTLP), existing `pkg/agent` hooks, existing `pkg/message` types.

**Design Spec:** `docs/superpowers/specs/2026-04-24-studio-integration-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `pkg/studio/types.go` | Create | Data models: RunData, PushMessageRequest |
| `pkg/studio/convert.go` | Create | Msg → studio payload conversion |
| `pkg/studio/convert_test.go` | Create | Tests for conversion |
| `pkg/studio/client.go` | Create | HTTP client: RegisterRun, PushMessage |
| `pkg/studio/client_test.go` | Create | Tests for HTTP client with mock server |
| `pkg/studio/hooks.go` | Create | PreReplyHook, PostReplyHook |
| `pkg/studio/hooks_test.go` | Create | Tests for hooks |
| `pkg/studio/studio.go` | Create | Init, Shutdown, GetClient, options |
| `pkg/studio/studio_test.go` | Create | Tests for Init/Shutdown lifecycle |
| `pkg/agent/react.go` | Modify | Auto-inject studio hooks in NewReActAgent |
| `pkg/tracing/tracing.go` | Modify | Add SetupTracingHTTP function |
| `examples/studio/main.go` | Create | Usage example |

---

### Task 1: Data Models (`types.go`)

**Files:**
- Create: `pkg/studio/types.go`

- [ ] **Step 1: Create the types file**

```go
// File: pkg/studio/types.go
package studio

// RunData represents the payload sent to /trpc/registerRun.
// It matches the Python agentscope registerRun payload format.
type RunData struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
	PID       int    `json:"pid"`
	Status    string `json:"status"`
	RunDir    string `json:"run_dir"`
}

// PushMessageRequest represents the payload sent to /trpc/pushMessage.
// It matches the Python agentscope pushMessage payload format.
type PushMessageRequest struct {
	RunID     string                 `json:"runId"`
	ReplyID   string                 `json:"replyId"`
	ReplyName string                 `json:"replyName"`
	ReplyRole string                 `json:"replyRole"`
	Msg       map[string]interface{} `json:"msg"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./pkg/studio/`
Expected: compile success

- [ ] **Step 3: Commit**

```bash
git add pkg/studio/types.go
git commit -m "feat(studio): add data models for studio API payloads"
```

---

### Task 2: Message Conversion (`convert.go`)

**Files:**
- Create: `pkg/studio/convert.go`
- Create: `pkg/studio/convert_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// File: pkg/studio/convert_test.go
package studio

import (
	"testing"

	"github.com/vearne/agentscope-go/pkg/message"
)

func TestMsgToPayload_SingleText(t *testing.T) {
	msg := message.NewMsg("user", "hello", "user")
	payload := MsgToPayload(msg)

	if payload["id"] != msg.ID {
		t.Errorf("expected id %s, got %v", msg.ID, payload["id"])
	}
	if payload["name"] != "user" {
		t.Errorf("expected name 'user', got %v", payload["name"])
	}
	if payload["role"] != "user" {
		t.Errorf("expected role 'user', got %v", payload["role"])
	}
	// Single text block should flatten to string
	content, ok := payload["content"].(string)
	if !ok {
		t.Errorf("expected content to be string, got %T", payload["content"])
	}
	if content != "hello" {
		t.Errorf("expected content 'hello', got %s", content)
	}
	if _, ok := payload["timestamp"]; !ok {
		t.Error("expected timestamp field")
	}
}

func TestMsgToPayload_MultipleBlocks(t *testing.T) {
	msg := &message.Msg{
		ID:        "test-id",
		Name:      "assistant",
		Role:      "assistant",
		Content:   []message.ContentBlock{
			message.NewTextBlock("thinking..."),
			message.NewToolUseBlock("t1", "calculator", map[string]interface{}{"expr": "2+2"}),
		},
		Timestamp: "2026-04-24 10:00:00.000",
	}
	payload := MsgToPayload(msg)

	// Multiple blocks should remain as []map[string]interface{}
	blocks, ok := payload["content"].([]map[string]interface{})
	if !ok {
		t.Errorf("expected content to be []map[string]interface{}, got %T", payload["content"])
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0]["type"] != "text" {
		t.Errorf("expected first block type 'text', got %v", blocks[0]["type"])
	}
	if blocks[1]["type"] != "tool_use" {
		t.Errorf("expected second block type 'tool_use', got %v", blocks[1]["type"])
	}
}

func TestMsgToPayload_ToolResultBlock(t *testing.T) {
	msg := &message.Msg{
		ID:        "test-id",
		Name:      "tool",
		Role:      "tool",
		Content:   []message.ContentBlock{
			message.NewToolResultBlock("t1", "4", false),
		},
		Timestamp: "2026-04-24 10:00:00.000",
	}
	payload := MsgToPayload(msg)

	blocks, ok := payload["content"].([]map[string]interface{})
	if !ok {
		t.Errorf("expected content to be []map[string]interface{}, got %T", payload["content"])
	}
	if blocks[0]["type"] != "tool_result" {
		t.Errorf("expected type 'tool_result', got %v", blocks[0]["type"])
	}
	if blocks[0]["id"] != "t1" {
		t.Errorf("expected id 't1', got %v", blocks[0]["id"])
	}
}

func TestMsgToPayload_NilMetadata(t *testing.T) {
	msg := message.NewMsg("user", "hi", "user")
	payload := MsgToPayload(msg)

	// metadata should be an empty map, not nil
	metadata, ok := payload["metadata"].(map[string]interface{})
	if !ok {
		t.Errorf("expected metadata to be map[string]interface{}, got %T", payload["metadata"])
	}
	if len(metadata) != 0 {
		t.Errorf("expected empty metadata, got %v", metadata)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/studio/ -run TestMsgToPayload -v`
Expected: FAIL — `MsgToPayload` undefined

- [ ] **Step 3: Write the implementation**

```go
// File: pkg/studio/convert.go
package studio

import (
	"github.com/vearne/agentscope-go/pkg/message"
)

// MsgToPayload converts a Go Msg to a studio-compatible map matching
// Python agentscope's Msg.to_dict() format.
func MsgToPayload(msg *message.Msg) map[string]interface{} {
	if msg == nil {
		return nil
	}

	return map[string]interface{}{
		"id":        msg.ID,
		"name":      msg.Name,
		"role":      msg.Role,
		"content":   convertContent(msg.Content),
		"metadata":  ensureMetadata(msg.Metadata),
		"timestamp": msg.Timestamp,
	}
}

// convertContent transforms ContentBlocks into the format studio expects.
// Single text-only content is flattened to a string (matching Python behavior).
// All other content is passed through as a slice of maps.
func convertContent(blocks []message.ContentBlock) interface{} {
	if len(blocks) == 1 && message.IsTextBlock(blocks[0]) {
		return message.GetBlockText(blocks[0])
	}

	result := make([]map[string]interface{}, len(blocks))
	for i, block := range blocks {
		// ContentBlock is map[string]interface{}, pass through directly
		result[i] = map[string]interface{}(block)
	}
	return result
}

// ensureMetadata returns an empty map if metadata is nil,
// matching Python's default empty dict behavior.
func ensureMetadata(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/studio/ -run TestMsgToPayload -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/studio/convert.go pkg/studio/convert_test.go
git commit -m "feat(studio): add Msg to studio payload conversion with tests"
```

---

### Task 3: HTTP Client (`client.go`)

**Files:**
- Create: `pkg/studio/client.go`
- Create: `pkg/studio/client_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// File: pkg/studio/client_test.go
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
		json.Unmarshal(body, &receivedBody)
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
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
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
			"id":        "msg-1",
			"name":      "assistant",
			"role":      "assistant",
			"content":   "hello",
			"metadata":  map[string]interface{}{},
			"timestamp": "2026-04-24 10:00:00.000",
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
		json.Unmarshal(body, &receivedBody)
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

	client.RegisterRun(context.Background())

	pid, ok := receivedBody["pid"].(float64)
	if !ok {
		t.Fatalf("expected pid to be a number, got %T", receivedBody["pid"])
	}
	if int(pid) != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), int(pid))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/studio/ -run TestClient -v`
Expected: FAIL — `StudioClient` undefined

- [ ] **Step 3: Write the implementation**

```go
// File: pkg/studio/client.go
package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	maxRetries     = 3
	retryDelay     = 100 * time.Millisecond
	requestTimeout = 5 * time.Second
)

// StudioClient is an HTTP client for the agentscope-studio tRPC API.
type StudioClient struct {
	baseURL    string
	runID      string
	project    string
	name       string
	httpClient *http.Client
}

// newStudioClient creates a new StudioClient. It does NOT call RegisterRun;
// the caller should invoke RegisterRun separately.
func newStudioClient(baseURL, runID, project, name string) *StudioClient {
	return &StudioClient{
		baseURL: baseURL,
		runID:   runID,
		project: project,
		name:    name,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// RegisterRun POSTs to /trpc/registerRun to register this run with the studio.
func (c *StudioClient) RegisterRun(ctx context.Context) error {
	data := RunData{
		ID:        c.runID,
		Project:   c.project,
		Name:      c.name,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		PID:       os.Getpid(),
		Status:    "running",
		RunDir:    "",
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal run data: %w", err)
	}

	return c.postWithRetry(ctx, "/trpc/registerRun", body)
}

// PushMessage POSTs to /trpc/pushMessage to forward a message to the studio.
func (c *StudioClient) PushMessage(ctx context.Context, req *PushMessageRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal push message request: %w", err)
	}

	return c.postWithRetry(ctx, "/trpc/pushMessage", body)
}

// RunID returns the run ID associated with this client.
func (c *StudioClient) RunID() string {
	return c.runID
}

// postWithRetry sends a POST request with up to maxRetries retries on failure.
func (c *StudioClient) postWithRetry(ctx context.Context, path string, body []byte) error {
	url := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("studio: %s attempt %d failed: %v", path, attempt+1, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("studio: %s returned status %d", path, resp.StatusCode)
		log.Printf("studio: %s attempt %d returned %d", path, attempt+1, resp.StatusCode)
	}

	return lastErr
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/studio/ -run TestClient -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/studio/client.go pkg/studio/client_test.go
git commit -m "feat(studio): add StudioClient with RegisterRun and PushMessage"
```

---

### Task 4: Agent Hooks (`hooks.go`)

**Files:**
- Create: `pkg/studio/hooks.go`
- Create: `pkg/studio/hooks_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// File: pkg/studio/hooks_test.go
package studio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

// stubAgent implements agent.AgentBase for testing hooks.
type stubAgent struct{}

func (s *stubAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (s *stubAgent) Observe(ctx context.Context, msg *message.Msg) error { return nil }
func (s *stubAgent) Name() string                                         { return "test-agent" }
func (s *stubAgent) ID() string                                           { return "test-id" }

func TestPostReplyHook_PushesResponse(t *testing.T) {
	var pushCount int32
	var lastBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&pushCount, 1)
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set global client
	client := newStudioClient(server.URL, "run-123", "test", "test")
	globalClient = client
	defer func() { globalClient = nil }()

	ag := &stubAgent{}
	resp := message.NewMsg("assistant", "hello from agent", "assistant")

	PostReplyHook(context.Background(), ag, nil, resp)

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

func TestPreReplyHook_PushesInput(t *testing.T) {
	var lastBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newStudioClient(server.URL, "run-123", "test", "test")
	globalClient = client
	defer func() { globalClient = nil }()

	ag := &stubAgent{}
	inputMsg := message.NewMsg("user", "what is 2+2?", "user")

	PreReplyHook(context.Background(), ag, inputMsg, nil)

	if lastBody["replyRole"] != "user" {
		t.Errorf("expected replyRole 'user', got %v", lastBody["replyRole"])
	}
	if lastBody["runId"] != "run-123" {
		t.Errorf("expected runId 'run-123', got %v", lastBody["runId"])
	}
}

func TestHooks_NilClient_Noop(t *testing.T) {
	globalClient = nil
	defer func() { globalClient = nil }()

	ag := &stubAgent{}
	msg := message.NewMsg("user", "hello", "user")
	resp := message.NewMsg("assistant", "hi", "assistant")

	// Should not panic
	PreReplyHook(context.Background(), ag, msg, nil)
	PostReplyHook(context.Background(), ag, msg, resp)
}

func TestHooks_NilMessage_Noop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not have made an HTTP call")
	}))
	defer server.Close()

	client := newStudioClient(server.URL, "run-123", "test", "test")
	globalClient = client
	defer func() { globalClient = nil }()

	ag := &stubAgent{}

	// nil msg and nil resp should be no-ops
	PreReplyHook(context.Background(), ag, nil, nil)
	PostReplyHook(context.Background(), ag, nil, nil)
}

// Verify HookFunc compatibility
func TestHooks_MatchHookFuncSignature(t *testing.T) {
	// This test verifies that PreReplyHook and PostReplyHook
	// match the agent.HookFunc signature.
	var _ agent.HookFunc = PreReplyHook
	var _ agent.HookFunc = PostReplyHook
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/studio/ -run TestHooks -v`
Expected: FAIL — `PostReplyHook` undefined

- [ ] **Step 3: Write the implementation**

```go
// File: pkg/studio/hooks.go
package studio

import (
	"context"
	"log"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

// PreReplyHook forwards the input message to studio before the agent processes it.
// It reports the message with the original role (typically "user").
func PreReplyHook(ctx context.Context, ag agent.AgentBase, msg *message.Msg, resp *message.Msg) {
	client := GetClient()
	if client == nil || msg == nil {
		return
	}

	err := client.PushMessage(ctx, &PushMessageRequest{
		RunID:     client.RunID(),
		ReplyID:   msg.ID,
		ReplyName: msg.Name,
		ReplyRole: msg.Role,
		Msg:       MsgToPayload(msg),
	})
	if err != nil {
		log.Printf("studio: pushMessage (pre) failed: %v", err)
	}
}

// PostReplyHook forwards the agent's response to studio after processing.
// It reports the message with role "assistant".
func PostReplyHook(ctx context.Context, ag agent.AgentBase, msg *message.Msg, resp *message.Msg) {
	client := GetClient()
	if client == nil || resp == nil {
		return
	}

	err := client.PushMessage(ctx, &PushMessageRequest{
		RunID:     client.RunID(),
		ReplyID:   resp.ID,
		ReplyName: ag.Name(),
		ReplyRole: "assistant",
		Msg:       MsgToPayload(resp),
	})
	if err != nil {
		log.Printf("studio: pushMessage (post) failed: %v", err)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/studio/ -run Test -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/studio/hooks.go pkg/studio/hooks_test.go
git commit -m "feat(studio): add PreReplyHook and PostReplyHook for agent integration"
```

---

### Task 5: Global Init (`studio.go`)

**Files:**
- Create: `pkg/studio/studio.go`
- Create: `pkg/studio/studio_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// File: pkg/studio/studio_test.go
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
		json.Unmarshal(body, &receivedBody)
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
	Shutdown(context.Background()) // ensure clean state

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

func TestInit_IDEMPOTENT(t *testing.T) {
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

	// Second Init should be a no-op
	err = Init(WithURL(server.URL), WithProject("test2"))
	if err != nil {
		t.Fatalf("second Init failed: %v", err)
	}

	// Should still have the first client (idempotent)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/studio/ -run TestInit -v`
Expected: FAIL — `Init` undefined

- [ ] **Step 3: Write the implementation**

```go
// File: pkg/studio/studio.go
package studio

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/tracing"
)

var (
	globalMu     sync.Mutex
	globalClient *StudioClient
)

// Option configures the studio client.
type Option func(*config)

type config struct {
	url     string
	project string
	name    string
	runID   string
}

// WithURL sets the studio server URL (e.g. "http://localhost:3000").
func WithURL(url string) Option {
	return func(c *config) { c.url = url }
}

// WithProject sets the project name reported to studio.
func WithProject(project string) Option {
	return func(c *config) { c.project = project }
}

// WithName sets the run name reported to studio.
func WithName(name string) Option {
	return func(c *config) { c.name = name }
}

// WithRunID sets a custom run ID. If not provided, one is auto-generated.
func WithRunID(id string) Option {
	return func(c *config) { c.runID = id }
}

// Init initializes the studio connection, registers the run, and sets the
// global client. After calling Init, all subsequently created ReActAgent
// instances will automatically forward messages to the studio.
//
// Init is idempotent: calling it multiple times is a no-op after the first
// successful initialization.
func Init(opts ...Option) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalClient != nil {
		return nil
	}

	cfg := config{
		project: "UnnamedProject_At" + time.Now().Format("20060102"),
		name:    time.Now().Format("150405_") + utils.ShortUUID()[:4],
		runID:   utils.ShortUUID(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.url == "" {
		return fmt.Errorf("studio: URL is required")
	}

	client := newStudioClient(cfg.url, cfg.runID, cfg.project, cfg.name)

	if err := client.RegisterRun(context.Background()); err != nil {
		return fmt.Errorf("studio: register run: %w", err)
	}

	globalClient = client

	// Setup OTLP HTTP tracing to studio's /v1/traces endpoint
	tracingEndpoint := strings.TrimRight(cfg.url, "/") + "/v1/traces"
	if _, err := tracing.SetupTracingHTTP(context.Background(), tracingEndpoint, tracing.WithInsecure()); err != nil {
		// Tracing is best-effort; log warning but don't fail
		log.Printf("studio: failed to setup tracing: %v", err)
	}

	return nil
}

// Shutdown disconnects from the studio, clearing the global client.
// After Shutdown, agents will no longer forward messages.
func Shutdown(ctx context.Context) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalClient = nil
	// Note: tracing shutdown is handled by the tracing package's own global state
	return nil
}

// GetClient returns the global studio client, or nil if Init has not been called.
func GetClient() *StudioClient {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalClient
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/studio/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/studio/studio.go pkg/studio/studio_test.go
git commit -m "feat(studio): add global Init/Shutdown with client lifecycle management"
```

---

### Task 6: OTLP HTTP Tracing (`tracing.go`)

**Files:**
- Modify: `pkg/tracing/tracing.go`

- [ ] **Step 1: Add the OTLP HTTP exporter dependency**

Run: `go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`

- [ ] **Step 2: Add SetupTracingHTTP to tracing.go**

Add a new function after the existing `SetupTracing` function in `pkg/tracing/tracing.go`. The new function uses the OTLP HTTP exporter instead of gRPC:

```go
// Add to imports:
// "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

// SetupTracingHTTP initializes the global TracerProvider with an OTLP HTTP exporter.
// It returns a shutdown function that the caller should defer.
func SetupTracingHTTP(ctx context.Context, endpoint string, opts ...TracingOption) (func(context.Context) error, error) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if provider != nil {
		return func(context.Context) error { return nil }, nil
	}

	cfg := tracingConfig{
		serviceName: defaultServiceName,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
	}
	if cfg.insecure {
		exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, exporterOpts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	provider = tp

	return func(ctx context.Context) error {
		globalMu.Lock()
		defer globalMu.Unlock()
		if provider != nil {
			err := provider.Shutdown(ctx)
			provider = nil
			return err
		}
		return nil
	}, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./pkg/tracing/`
Expected: compile success

- [ ] **Step 4: Commit**

```bash
go mod tidy
git add pkg/tracing/tracing.go go.mod go.sum
git commit -m "feat(tracing): add SetupTracingHTTP for OTLP HTTP exporter"
```

---

### Task 7: Auto-Inject Hooks into ReActAgent (`react.go`)

**Files:**
- Modify: `pkg/agent/react.go` lines 69-82 (NewReActAgent function)

- [ ] **Step 1: Add the import and auto-injection in NewReActAgent**

In `pkg/agent/react.go`, add the studio import and the auto-injection at the end of `NewReActAgent`:

Add to import block:
```go
"github.com/vearne/agentscope-go/pkg/studio"
```

Add at the end of `NewReActAgent`, before `return a` (after line 80 `a.mem = memory.NewInMemoryMemory()`):

```go
	// Auto-inject studio hooks if studio has been initialized
	if sc := studio.GetClient(); sc != nil {
		a.hooks.preReply = append(a.hooks.preReply, studio.PreReplyHook)
		a.hooks.postReply = append(a.hooks.postReply, studio.PostReplyHook)
	}
```

The full `NewReActAgent` function becomes:

```go
func NewReActAgent(opts ...ReActOption) *ReActAgent {
	a := &ReActAgent{
		id:       utils.ShortUUID(),
		name:     "react_agent",
		maxIters: defaultMaxIters,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.mem == nil {
		a.mem = memory.NewInMemoryMemory()
	}
	// Auto-inject studio hooks if studio has been initialized
	if sc := studio.GetClient(); sc != nil {
		a.hooks.preReply = append(a.hooks.preReply, studio.PreReplyHook)
		a.hooks.postReply = append(a.hooks.postReply, studio.PostReplyHook)
	}
	return a
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./pkg/agent/`
Expected: compile success (with or without studio.Init being called)

- [ ] **Step 3: Commit**

```bash
git add pkg/agent/react.go
git commit -m "feat(agent): auto-inject studio hooks when studio is initialized"
```

---

### Task 8: Integration Example (`examples/studio/main.go`)

**Files:**
- Create: `examples/studio/main.go`

- [ ] **Step 1: Create the example**

```go
// File: examples/studio/main.go
// This example demonstrates how to use agentscope-studio integration.
//
// Prerequisites:
//   1. Start agentscope-studio: npx agentscope-studio@latest
//   2. Set OPENAI_API_KEY environment variable
//
// Usage:
//
//	go run ./examples/studio
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/studio"
)

func main() {
	ctx := context.Background()

	// Step 1: Initialize studio connection.
	// After this, all agents will automatically forward messages to studio.
	if err := studio.Init(
		studio.WithURL("http://localhost:3000"),
		studio.WithProject("hello-studio"),
	); err != nil {
		log.Printf("Warning: studio init failed (studio may not be running): %v", err)
		log.Println("Continuing without studio integration...")
	}
	defer studio.Shutdown(ctx)

	// Step 2: Create agent as usual — no studio-specific configuration needed
	m := model.NewOpenAIChatModel("gpt-4o", os.Getenv("OPENAI_API_KEY"), "", false)
	f := formatter.NewOpenAIFormatter()

	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
	)

	// Step 3: Send a message — it will be automatically forwarded to studio
	msg := message.NewMsg("user", "Hello! What can you do?", "user")
	resp, err := ag.Reply(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./examples/studio/`
Expected: compile success

- [ ] **Step 3: Commit**

```bash
git add examples/studio/main.go
git commit -m "docs(studio): add studio integration example"
```

---

### Task 9: Run Full Test Suite

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: all PASS

- [ ] **Step 2: Run build for all packages**

Run: `go build ./...`
Expected: compile success

- [ ] **Step 3: Final commit if any fixes needed**

If any test failures were found and fixed:

```bash
git add -A
git commit -m "fix(studio): address test failures from integration testing"
```
