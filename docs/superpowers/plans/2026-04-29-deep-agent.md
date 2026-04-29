# DeepAgent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a long-running autonomous DeepAgent with context engineering (offloading, summarization, subagent delegation) for agentscope-go.

**Architecture:** Layered design — execution layer (independent think-and-act loop), context management layer (OffloadManager + ContextCompressor), delegation layer (SubagentFactory + delegate_task tool). All new files in `pkg/agent/`, no modifications to existing files.

**Tech Stack:** Go 1.21+, existing packages (model, formatter, memory, tool, message, tracing, studio), standard library (os, path/filepath, fmt, context, log)

**Spec:** `docs/superpowers/specs/2026-04-29-deep-agent-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `pkg/agent/offload.go` | OffloadManager — filesystem storage for large tool results |
| `pkg/agent/compressor.go` | ContextCompressor interface + TruncatingCompressor + LLMCompressor |
| `pkg/agent/subagent.go` | SubagentFactory + SubagentConfig + delegate_task tool registration |
| `pkg/agent/deep_options.go` | DeepOption type + all WithDeep* option functions |
| `pkg/agent/deep.go` | DeepAgent struct + Reply/Observe execution loop |
| `pkg/agent/deep_test.go` | Tests for DeepAgent, OffloadManager, Compressor, SubagentFactory |

---

### Task 1: OffloadManager

**Files:**
- Create: `pkg/agent/offload.go`
- Test: `pkg/agent/deep_test.go`

- [ ] **Step 1: Write the failing test for OffloadManager**

Create `pkg/agent/deep_test.go` with the OffloadManager tests:

```go
package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOffloadManager_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	om := NewOffloadManager(dir, 100)

	content := "short content"
	result, offloaded := om.MaybeOffload(content, "msg-1")
	if offloaded {
		t.Error("should not offload content below threshold")
	}
	if result != content {
		t.Error("content should be unchanged")
	}
}

func TestOffloadManager_AboveThreshold(t *testing.T) {
	dir := t.TempDir()
	om := NewOffloadManager(dir, 10)

	content := strings.Repeat("a", 100)
	result, offloaded := om.MaybeOffload(content, "msg-2")
	if !offloaded {
		t.Fatal("should offload content above threshold")
	}
	if strings.Contains(result, strings.Repeat("a", 50)) {
		t.Error("result should not contain full content")
	}
	if !strings.Contains(result, "offloaded") {
		t.Error("result should mention offloading")
	}
	if !strings.HasPrefix(result, "[") {
		t.Error("result should start with [")
	}
}

func TestOffloadManager_ReadBack(t *testing.T) {
	dir := t.TempDir()
	om := NewOffloadManager(dir, 10)

	content := strings.Repeat("x", 50)
	_, offloaded := om.MaybeOffload(content, "msg-3")
	if !offloaded {
		t.Fatal("should offload")
	}

	path := filepath.Join(dir, "msg-3.txt")
	readBack, err := om.Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readBack != content {
		t.Error("read-back content should match original")
	}
}

func TestOffloadManager_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "offload")
	om := NewOffloadManager(dir, 5)

	content := "hello world this is long"
	_, offloaded := om.MaybeOffload(content, "msg-4")
	if !offloaded {
		t.Fatal("should offload")
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory should be created")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/agent/ -run TestOffload -v`
Expected: FAIL — `NewOffloadManager` undefined

- [ ] **Step 3: Implement OffloadManager**

Create `pkg/agent/offload.go`:

```go
package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// OffloadManager stores large tool results to the filesystem,
// replacing them with a reference in conversation history.
type OffloadManager struct {
	dir       string // filesystem directory for offloaded content
	threshold int    // character count threshold for offloading
}

// NewOffloadManager creates a new OffloadManager.
func NewOffloadManager(dir string, threshold int) *OffloadManager {
	return &OffloadManager{dir: dir, threshold: threshold}
}

// MaybeOffload checks if content exceeds the threshold.
// If so, it writes the content to a file and returns a reference string.
// Returns (result, wasOffloaded).
func (m *OffloadManager) MaybeOffload(content string, msgID string) (string, bool) {
	if len(content) <= m.threshold {
		return content, false
	}

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		// Offload write failure: keep full content in memory (degraded but functional)
		return content, false
	}

	path := filepath.Join(m.dir, msgID+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		// Offload write failure: keep full content in memory
		return content, false
	}

	previewLen := 200
	if len(content) < previewLen {
		previewLen = len(content)
	}
	preview := content[:previewLen]
	ref := fmt.Sprintf("[Result offloaded to %s. Preview: %s...]", path, preview)
	return ref, true
}

// Read loads a previously offloaded file from disk.
func (m *OffloadManager) Read(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read offloaded file %s: %w", path, err)
	}
	return string(data), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/agent/ -run TestOffload -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/offload.go pkg/agent/deep_test.go
git commit -m "feat(agent): add OffloadManager for filesystem-based tool result offloading"
```

---

### Task 2: ContextCompressor Interface + TruncatingCompressor

**Files:**
- Create: `pkg/agent/compressor.go`
- Modify: `pkg/agent/deep_test.go`

- [ ] **Step 1: Write the failing test for TruncatingCompressor**

Add to `pkg/agent/deep_test.go`:

```go
func TestTruncatingCompressor_NoCompression(t *testing.T) {
	c := &TruncatingCompressor{}
	msgs := []*message.Msg{
		message.NewMsg("user", "a", "user"),
		message.NewMsg("assistant", "b", "assistant"),
	}
	result, err := c.Compress(context.Background(), msgs, 5)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages (below threshold), got %d", len(result))
	}
}

func TestTruncatingCompressor_Truncates(t *testing.T) {
	c := &TruncatingCompressor{}
	msgs := make([]*message.Msg, 10)
	for i := range msgs {
		msgs[i] = message.NewMsg("user", fmt.Sprintf("msg %d", i), "user")
	}
	result, err := c.Compress(context.Background(), msgs, 3)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 messages (keepRecent), got %d", len(result))
	}
	text := result[0].GetTextContent()
	if text != "msg 7" {
		t.Errorf("expected first kept message to be 'msg 7', got '%s'", text)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/agent/ -run TestTruncatingCompressor -v`
Expected: FAIL — `TruncatingCompressor` undefined

- [ ] **Step 3: Implement ContextCompressor interface + TruncatingCompressor**

Create `pkg/agent/compressor.go`:

```go
package agent

import (
	"context"

	"github.com/vearne/agentscope-go/pkg/message"
)

// ContextCompressor compresses old conversation history into a shorter form.
// Implementations may use LLM summarization, simple truncation, or other strategies.
type ContextCompressor interface {
	Compress(ctx context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error)
}

// TruncatingCompressor is a simple compressor that drops old messages,
// keeping only the N most recent. Useful for testing or when LLM
// summarization is not desired.
type TruncatingCompressor struct{}

func (c *TruncatingCompressor) Compress(_ context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error) {
	if len(msgs) <= keepRecent {
		return msgs, nil
	}
	return msgs[len(msgs)-keepRecent:], nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/agent/ -run TestTruncatingCompressor -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/compressor.go pkg/agent/deep_test.go
git commit -m "feat(agent): add ContextCompressor interface and TruncatingCompressor"
```

---

### Task 3: LLMCompressor

**Files:**
- Modify: `pkg/agent/compressor.go`
- Modify: `pkg/agent/deep_test.go`

- [ ] **Step 1: Write the failing test for LLMCompressor**

Add to `pkg/agent/deep_test.go`:

```go
func TestLLMCompressor_Compress(t *testing.T) {
	// Mock model returns a fixed summary
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("Summary: user discussed topic A and B"),
			}),
		},
	}
	c := NewLLMCompressor(mockM, &mockFormatter{}, "")
	msgs := make([]*message.Msg, 8)
	for i := range msgs {
		msgs[i] = message.NewMsg("user", fmt.Sprintf("message %d", i), "user")
	}
	result, err := c.Compress(context.Background(), msgs, 3)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	// Should be: 1 summary + 3 recent = 4
	if len(result) != 4 {
		t.Errorf("expected 4 messages (1 summary + 3 recent), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("first message should be summary with role 'user', got '%s'", result[0].Role)
	}
	if !strings.Contains(result[0].GetTextContent(), "Summary") {
		t.Errorf("summary should contain 'Summary', got '%s'", result[0].GetTextContent())
	}
	// Last 3 messages should be the most recent
	if result[1].GetTextContent() != "message 5" {
		t.Errorf("expected 'message 5' as first recent, got '%s'", result[1].GetTextContent())
	}
}

func TestLLMCompressor_FallbackOnError(t *testing.T) {
	// Mock model that errors
	errorMock := &errorModel{}
	c := NewLLMCompressor(errorMock, &mockFormatter{}, "")
	msgs := []*message.Msg{
		message.NewMsg("user", "a", "user"),
		message.NewMsg("assistant", "b", "assistant"),
	}
	result, err := c.Compress(context.Background(), msgs, 1)
	if err != nil {
		t.Fatalf("should not return error on compression failure")
	}
	// Should return original messages on failure
	if len(result) != 2 {
		t.Errorf("expected 2 original messages on failure, got %d", len(result))
	}
}
```

Add the `errorModel` helper near the top of `deep_test.go` (after the existing `mockModel`):

```go
type errorModel struct{}

func (m *errorModel) Call(_ context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (*model.ChatResponse, error) {
	return nil, fmt.Errorf("model error")
}
func (m *errorModel) Stream(_ context.Context, _ []model.FormattedMessage, _ ...model.CallOption) (<-chan model.ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *errorModel) ModelName() string { return "error-model" }
func (m *errorModel) IsStream() bool    { return false }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/agent/ -run TestLLMCompressor -v`
Expected: FAIL — `NewLLMCompressor` undefined

- [ ] **Step 3: Implement LLMCompressor**

Add to `pkg/agent/compressor.go`:

```go
import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)

const defaultCompressionPrompt = `Summarize the following conversation history concisely. Preserve:
- Session goals and intent
- Key decisions made
- Artifacts or files created/modified
- Current task status and progress
- Next steps or pending actions

Return ONLY the summary text, nothing else.`

// LLMCompressor uses an LLM to generate a summary of old conversation history.
type LLMCompressor struct {
	model  model.ChatModelBase
	fmt    formatter.FormatterBase
	prompt string
}

// NewLLMCompressor creates an LLMCompressor.
// If prompt is empty, a sensible default is used.
func NewLLMCompressor(m model.ChatModelBase, f formatter.FormatterBase, prompt string) *LLMCompressor {
	p := prompt
	if p == "" {
		p = defaultCompressionPrompt
	}
	return &LLMCompressor{model: m, fmt: f, prompt: p}
}

func (c *LLMCompressor) Compress(ctx context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error) {
	if len(msgs) <= keepRecent {
		return msgs, nil
	}

	oldMsgs := msgs[:len(msgs)-keepRecent]
	recentMsgs := msgs[len(msgs)-keepRecent:]

	// Build compression request: summary instruction + old messages
	sumMsg := message.NewMsg("user", c.prompt+"\n\n--- Conversation to summarize ---", "user")
	compressMsgs := append([]*message.Msg{sumMsg}, oldMsgs...)

	formatted, err := c.fmt.Format(compressMsgs)
	if err != nil {
		log.Printf("[LLMCompressor] format error: %v, returning original messages", err)
		return msgs, nil
	}

	resp, err := c.model.Call(ctx, formatted)
	if err != nil {
		log.Printf("[LLMCompressor] model call error: %v, returning original messages", err)
		return msgs, nil
	}

	summaryText := resp.GetTextContent()
	if summaryText == "" {
		log.Printf("[LLMCompressor] empty summary, returning original messages")
		return msgs, nil
	}

	summaryMsg := message.NewMsg("user", fmt.Sprintf("[Conversation Summary]\n%s", summaryText), "user")
	result := append([]*message.Msg{summaryMsg}, recentMsgs...)
	return result, nil
}
```

Note: This requires adding `"fmt"`, `"log"`, `"github.com/vearne/agentscope-go/pkg/formatter"`, `"github.com/vearne/agentscope-go/pkg/model"` to the imports in `compressor.go`. Remove the unused `"context"` if it's already there from the TruncatingCompressor — actually context is still needed. Just add the new imports.

The full imports for `compressor.go` will be:

```go
import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/agent/ -run "TestTruncatingCompressor|TestLLMCompressor" -v`
Expected: PASS (4 tests total)

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/compressor.go pkg/agent/deep_test.go
git commit -m "feat(agent): add LLMCompressor for LLM-based context summarization"
```

---

### Task 4: SubagentFactory

**Files:**
- Create: `pkg/agent/subagent.go`
- Modify: `pkg/agent/deep_test.go`

- [ ] **Step 1: Write the failing test for SubagentFactory**

Add to `pkg/agent/deep_test.go`:

```go
func TestSubagentFactory_Create(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("subagent result"),
			}),
		},
	}
	factory := NewSubagentFactory(mockM, &mockFormatter{}, nil)

	sub := factory.Create(SubagentConfig{
		Name:         "worker-1",
		SystemPrompt: "You are a worker.",
		MaxIters:     5,
	})
	if sub == nil {
		t.Fatal("Create should return non-nil")
	}
	if sub.Name() != "worker-1" {
		t.Errorf("expected name 'worker-1', got '%s'", sub.Name())
	}
	// Subagent should have its own isolated memory
	if sub.Memory().Size() != 0 {
		t.Errorf("new subagent memory should be empty, got %d", sub.Memory().Size())
	}
}

func TestSubagentFactory_CreateWithCustomToolkit(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("done"),
			}),
		},
	}
	customTK := tool.NewToolkit()
	customTK.Register("custom_tool", "A custom tool", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return tool.NewToolResponse("custom result"), nil
		},
	)

	factory := NewSubagentFactory(mockM, &mockFormatter{}, nil)
	sub := factory.Create(SubagentConfig{
		Name:     "worker-2",
		MaxIters: 3,
		Toolkit:  customTK,
	})
	if sub == nil {
		t.Fatal("Create should return non-nil")
	}
}

func TestSubagentFactory_DelegateTask(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("research complete: found 3 papers"),
			}),
		},
	}
	factory := NewSubagentFactory(mockM, &mockFormatter{}, nil)

	result, err := factory.DelegateTask(context.Background(), DelegateTaskArgs{
		TaskDescription: "Research topic X",
		SubagentName:    "researcher",
		SystemPrompt:    "You are a research assistant.",
	})
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}
	if result != "research complete: found 3 papers" {
		t.Errorf("unexpected result: %s", result)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/agent/ -run TestSubagentFactory -v`
Expected: FAIL — `NewSubagentFactory` undefined

- [ ] **Step 3: Implement SubagentFactory**

Create `pkg/agent/subagent.go`:

```go
package agent

import (
	"context"
	"fmt"

	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const defaultSubagentMaxIters = 10

// SubagentFactory creates lightweight in-process subagents with isolated context.
type SubagentFactory struct {
	model   model.ChatModelBase
	fmt     formatter.FormatterBase
	toolkit *tool.Toolkit
}

// SubagentConfig configures a single subagent instance.
type SubagentConfig struct {
	Name         string
	SystemPrompt string
	MaxIters     int
	Toolkit      *tool.Toolkit // nil = use factory's default toolkit
}

// DelegateTaskArgs holds the arguments for the delegate_task tool.
type DelegateTaskArgs struct {
	TaskDescription string `json:"task_description"`
	SubagentName    string `json:"subagent_name"`
	SystemPrompt    string `json:"system_prompt"`
}

// NewSubagentFactory creates a factory that produces subagents
// sharing the given model and formatter.
func NewSubagentFactory(m model.ChatModelBase, f formatter.FormatterBase, tk *tool.Toolkit) *SubagentFactory {
	return &SubagentFactory{model: m, fmt: f, toolkit: tk}
}

// Create builds a new ReActAgent with its own isolated memory.
func (f *SubagentFactory) Create(cfg SubagentConfig) *ReActAgent {
	opts := []ReActOption{
		WithReActName(cfg.Name),
		WithReActModel(f.model),
		WithReActFormatter(f.fmt),
		WithReActMemory(memory.NewInMemoryMemory()),
	}

	tk := cfg.Toolkit
	if tk == nil {
		tk = f.toolkit
	}
	if tk != nil {
		opts = append(opts, WithReActToolkit(tk))
	}

	maxIters := cfg.MaxIters
	if maxIters <= 0 {
		maxIters = defaultSubagentMaxIters
	}
	opts = append(opts, WithReActMaxIters(maxIters))

	if cfg.SystemPrompt != "" {
		opts = append(opts, WithReActSystemPrompt(cfg.SystemPrompt))
	}

	return NewReActAgent(opts...)
}

// DelegateTask creates a subagent, runs it with the given task, and returns
// only the final text result. The subagent's intermediate steps never
// enter the main agent's memory.
func (f *SubagentFactory) DelegateTask(ctx context.Context, args DelegateTaskArgs) (string, error) {
	sysPrompt := args.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = "You are a helpful assistant. Complete the task and return the result concisely."
	}

	sub := f.Create(SubagentConfig{
		Name:         args.SubagentName,
		SystemPrompt: sysPrompt,
	})

	msg := message.NewMsg("user", args.TaskDescription, "user")
	resp, err := sub.Reply(ctx, msg)
	if err != nil {
		return "", fmt.Errorf("subagent %q failed: %w", args.SubagentName, err)
	}

	return resp.GetTextContent(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/agent/ -run TestSubagentFactory -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/subagent.go pkg/agent/deep_test.go
git commit -m "feat(agent): add SubagentFactory for in-process subagent delegation"
```

---

### Task 5: DeepAgent Options

**Files:**
- Create: `pkg/agent/deep_options.go`

- [ ] **Step 1: Write the failing test for options**

Add to `pkg/agent/deep_test.go`:

```go
func TestNewDeepAgent_Defaults(t *testing.T) {
	ag := NewDeepAgent()
	if ag == nil {
		t.Fatal("NewDeepAgent should return non-nil")
	}
	if ag.Name() == "" {
		t.Error("should have a default name")
	}
	if ag.maxIters != 50 {
		t.Errorf("default maxIters should be 50, got %d", ag.maxIters)
	}
	if ag.maxCtxTokens != 128000 {
		t.Errorf("default maxCtxTokens should be 128000, got %d", ag.maxCtxTokens)
	}
	if ag.offloadThreshold != 8000 {
		t.Errorf("default offloadThreshold should be 8000, got %d", ag.offloadThreshold)
	}
}

func TestNewDeepAgent_WithOptions(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ok")}),
		},
	}
	ag := NewDeepAgent(
		WithDeepName("test-deep"),
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepMaxIters(100),
		WithDeepMaxContextTokens(200000),
		WithDeepOffloadThreshold(5000),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	if ag.Name() != "test-deep" {
		t.Errorf("expected name 'test-deep', got '%s'", ag.Name())
	}
	if ag.maxIters != 100 {
		t.Errorf("expected maxIters 100, got %d", ag.maxIters)
	}
	if ag.maxCtxTokens != 200000 {
		t.Errorf("expected maxCtxTokens 200000, got %d", ag.maxCtxTokens)
	}
}

func TestNewDeepAgent_InterfaceCompliance(t *testing.T) {
	var _ AgentBase = NewDeepAgent()
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/agent/ -run "TestNewDeepAgent" -v`
Expected: FAIL — `NewDeepAgent` undefined

- [ ] **Step 3: Implement options**

Create `pkg/agent/deep_options.go`:

```go
package agent

import (
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const (
	defaultDeepMaxIters        = 50
	defaultDeepMaxCtxTokens    = 128000
	defaultDeepOffloadDir      = ".deepagent/offload/"
	defaultDeepOffloadThreshold = 8000
)

// DeepOption configures a DeepAgent.
type DeepOption func(*DeepAgent)

func WithDeepName(name string) DeepOption {
	return func(a *DeepAgent) { a.name = name }
}

func WithDeepModel(m interface{}) DeepOption {
	return func(a *DeepAgent) {
		// Accept model.ChatModelBase; using interface{} to avoid import cycle
		// in this options file — the actual type check happens in NewDeepAgent
		a.modelField = m
	}
}

func WithDeepMemory(mem memory.MemoryBase) DeepOption {
	return func(a *DeepAgent) { a.mem = mem }
}

func WithDeepFormatter(f formatter.FormatterBase) DeepOption {
	return func(a *DeepAgent) { a.fmt = f }
}

func WithDeepToolkit(tk *tool.Toolkit) DeepOption {
	return func(a *DeepAgent) { a.toolkit = tk }
}

func WithDeepMaxIters(n int) DeepOption {
	return func(a *DeepAgent) { a.maxIters = n }
}

func WithDeepSystemPrompt(prompt string) DeepOption {
	return func(a *DeepAgent) { a.sysPrompt = prompt }
}

func WithDeepMaxContextTokens(n int) DeepOption {
	return func(a *DeepAgent) { a.maxCtxTokens = n }
}

func WithDeepOffloadDir(dir string) DeepOption {
	return func(a *DeepAgent) { a.offloadDir = dir }
}

func WithDeepOffloadThreshold(chars int) DeepOption {
	return func(a *DeepAgent) { a.offloadThreshold = chars }
}

func WithDeepCompressor(c ContextCompressor) DeepOption {
	return func(a *DeepAgent) { a.compressor = c }
}

func WithDeepSubagentFactory(f *SubagentFactory) DeepOption {
	return func(a *DeepAgent) { a.subFactory = f }
}

func WithDeepPreReply(h HookFunc) DeepOption {
	return func(a *DeepAgent) { a.hooks.preReply = append(a.hooks.preReply, h) }
}

func WithDeepPostReply(h HookFunc) DeepOption {
	return func(a *DeepAgent) { a.hooks.postReply = append(a.hooks.postReply, h) }
}
```

Note: `WithDeepModel` uses `interface{}` because `deep_options.go` avoids importing `model` to keep option file lightweight. The actual type assertion happens in `NewDeepAgent()` in `deep.go`. However, this is a deviation from the spec — let's use the proper type instead:

Actually, looking at how ReActAgent does it — `react.go` imports model directly and `WithReActModel` takes `model.ChatModelBase`. The option functions are in the same file as the struct. Let's do the same: put the type-safe option in `deep.go` and keep simple options in `deep_options.go`.

**Correction**: Move `WithDeepModel` and `WithDeepFormatter` to `deep.go` since they need typed imports. `deep_options.go` only has options that use types already available (string, int, interfaces defined in the agent package).

- [ ] **Step 4: Run tests to verify they fail (still — need deep.go)**

Run: `go test ./pkg/agent/ -run "TestNewDeepAgent" -v`
Expected: FAIL — `NewDeepAgent` undefined (need deep.go from Task 6)

This task depends on Task 6. Tests will pass once `deep.go` is implemented.

---

### Task 6: DeepAgent Core (Struct + Execution Loop)

**Files:**
- Create: `pkg/agent/deep.go`

This is the largest task. It depends on Tasks 1-5.

- [ ] **Step 1: Write additional failing tests for the execution loop**

Add to `pkg/agent/deep_test.go`:

```go
func TestDeepAgent_SimpleReply(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("Hello from deep agent!"),
			}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	msg := NewUserMsg("user", "Hi")
	resp, err := ag.Reply(context.Background(), msg)
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if resp.GetTextContent() != "Hello from deep agent!" {
		t.Errorf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestDeepAgent_ToolUse(t *testing.T) {
	tk := tool.NewToolkit()
	tk.Register("calc", "Calculate", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return tool.NewToolResponse("42"), nil
		},
	)
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			{
				Content: []message.ContentBlock{
					message.NewToolUseBlock("call_1", "calc", map[string]interface{}{"expr": "6*7"}),
				},
			},
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("The answer is 42."),
			}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepToolkit(tk),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	resp, err := ag.Reply(context.Background(), NewUserMsg("user", "What is 6*7?"))
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if resp.GetTextContent() != "The answer is 42." {
		t.Errorf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestDeepAgent_Offloading(t *testing.T) {
	dir := t.TempDir()
	bigResult := strings.Repeat("data line\n", 2000) // ~20000 chars, well above threshold

	tk := tool.NewToolkit()
	tk.Register("big_tool", "Returns big data", map[string]interface{}{"type": "object"},
		func(_ context.Context, _ map[string]interface{}) (*tool.ToolResponse, error) {
			return tool.NewToolResponse(bigResult), nil
		},
	)
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			{
				Content: []message.ContentBlock{
					message.NewToolUseBlock("call_1", "big_tool", map[string]interface{}{}),
				},
			},
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("Done with big data."),
			}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepToolkit(tk),
		WithDeepOffloadDir(dir),
		WithDeepOffloadThreshold(1000), // low threshold to trigger offloading
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	resp, err := ag.Reply(context.Background(), NewUserMsg("user", "Get big data"))
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if resp.GetTextContent() != "Done with big data." {
		t.Errorf("unexpected final response: %s", resp.GetTextContent())
	}
	// Verify offloaded file exists
	msgs := ag.Memory().GetMessages()
	found := false
	for _, m := range msgs {
		text := m.GetTextContent()
		if strings.Contains(text, "offloaded") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find an offloaded reference in memory")
	}
}

func TestDeepAgent_Observe(t *testing.T) {
	ag := NewDeepAgent(
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	msg := message.NewMsg("other", "observed", "assistant")
	if err := ag.Observe(context.Background(), msg); err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if ag.Memory().Size() != 1 {
		t.Errorf("expected 1 message after observe, got %d", ag.Memory().Size())
	}
}

func TestDeepAgent_SystemPrompt(t *testing.T) {
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ok")}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepSystemPrompt("You are a deep agent."),
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	ag.Reply(context.Background(), NewUserMsg("user", "hello"))
	msgs := ag.Memory().GetMessages()
	found := false
	for _, m := range msgs {
		if m.Role == "system" && m.GetTextContent() == "You are a deep agent." {
			found = true
			break
		}
	}
	if !found {
		t.Error("system prompt not found in memory")
	}
}

func TestDeepAgent_Compression(t *testing.T) {
	// Use TruncatingCompressor with low maxCtxTokens to trigger compression
	mockM := &mockModel{
		responses: []*model.ChatResponse{},
	}
	// Generate enough responses for multiple iterations
	for i := 0; i < 20; i++ {
		mockM.responses = append(mockM.responses,
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock(fmt.Sprintf("response %d", i)),
			}),
		)
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepMaxContextTokens(200), // very low to trigger compression
		WithDeepCompressor(&TruncatingCompressor{}),
	)
	// First reply adds messages
	ag.Reply(context.Background(), NewUserMsg("user", "hello"))
	// Memory should be compressed (TruncatingCompressor keeps only 6 recent)
	msgs := ag.Memory().GetMessages()
	// After compression, should have fewer messages than without
	// At least verify compression was triggered (memory bounded)
	if len(msgs) > 20 {
		t.Errorf("expected compression to bound memory, got %d messages", len(msgs))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/agent/ -run "TestDeepAgent_" -v`
Expected: FAIL — `DeepAgent` struct undefined

- [ ] **Step 3: Implement DeepAgent struct and execution loop**

Create `pkg/agent/deep.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const avgCharsPerToken = 4

type DeepAgent struct {
	id          string
	name        string
	sysPrompt   string
	model       model.ChatModelBase
	mem         memory.MemoryBase
	fmt         formatter.FormatterBase
	toolkit     *tool.Toolkit
	maxIters    int

	// Context management
	compressor       ContextCompressor
	offloader        *OffloadManager
	maxCtxTokens     int
	offloadDir       string
	offloadThreshold int

	// Delegation
	subFactory *SubagentFactory

	// Internal toolkit that wraps user toolkit + delegate_task
	internalToolkit *tool.Toolkit

	// Lifecycle hooks
	hooks hooks
}

func NewDeepAgent(opts ...DeepOption) *DeepAgent {
	a := &DeepAgent{
		id:               utils.ShortUUID(),
		name:             "deep_agent",
		maxIters:         defaultDeepMaxIters,
		maxCtxTokens:     defaultDeepMaxCtxTokens,
		offloadDir:       defaultDeepOffloadDir,
		offloadThreshold: defaultDeepOffloadThreshold,
	}
	for _, opt := range opts {
		opt(a)
	}

	// Apply modelField if set via WithDeepModel (see deep_options.go)
	// The option sets modelField directly if using typed version
	if a.model == nil {
		// Check if modelField was set
		if mf, ok := a.modelField.(model.ChatModelBase); ok {
			a.model = mf
		}
	}
	a.modelField = nil // clear the untyped field

	if a.mem == nil {
		a.mem = memory.NewInMemoryMemory()
	}

	// Default compressor: LLMCompressor if model is available, else TruncatingCompressor
	if a.compressor == nil && a.model != nil && a.fmt != nil {
		a.compressor = NewLLMCompressor(a.model, a.fmt, "")
	}
	if a.compressor == nil {
		a.compressor = &TruncatingCompressor{}
	}

	// Setup offloader
	a.offloader = NewOffloadManager(a.offloadDir, a.offloadThreshold)

	// Build internal toolkit: copy user tools + add delegate_task
	a.buildInternalToolkit()

	return a
}

func (a *DeepAgent) ID() string   { return a.id }
func (a *DeepAgent) Name() string { return a.name }

func (a *DeepAgent) Memory() memory.MemoryBase {
	return a.mem
}

// modelField is an untyped temporary field used by WithDeepModel option
// to avoid importing model in deep_options.go. Cleared in NewDeepAgent.
// This field is unexported and only used during construction.
type modelFieldType = interface{}

// this is a hack — let's just import model in deep_options.go instead.
```

Wait — the `modelField` approach is messy. Let's keep it clean: `deep_options.go` DOES import `model` (just like `react.go` does). The project already imports model in the agent package via `react.go`. There's no circular dependency issue.

**Revised approach**: `deep_options.go` imports `model.ChatModelBase` directly. Remove `modelField` hack.

Rewrite `deep_options.go` to use proper types:

```go
package agent

import (
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

// ... (constants same as before)

type DeepOption func(*DeepAgent)

// WithDeepModel sets the LLM model for the DeepAgent.
func WithDeepModel(m model.ChatModelBase) DeepOption {
	return func(a *DeepAgent) { a.model = m }
}

// ... rest of options with proper types
```

And `deep.go` — the core:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const avgCharsPerToken = 4

type DeepAgent struct {
	id          string
	name        string
	sysPrompt   string
	model       model.ChatModelBase
	mem         memory.MemoryBase
	fmt         formatter.FormatterBase
	toolkit     *tool.Toolkit
	maxIters    int

	// Context management
	compressor       ContextCompressor
	offloader        *OffloadManager
	maxCtxTokens     int
	offloadDir       string
	offloadThreshold int

	// Delegation
	subFactory *SubagentFactory

	// Internal toolkit wrapping user toolkit + built-in tools
	internalToolkit *tool.Toolkit

	hooks hooks
}

func NewDeepAgent(opts ...DeepOption) *DeepAgent {
	a := &DeepAgent{
		id:               utils.ShortUUID(),
		name:             "deep_agent",
		maxIters:         defaultDeepMaxIters,
		maxCtxTokens:     defaultDeepMaxCtxTokens,
		offloadDir:       defaultDeepOffloadDir,
		offloadThreshold: defaultDeepOffloadThreshold,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.mem == nil {
		a.mem = memory.NewInMemoryMemory()
	}
	if a.compressor == nil && a.model != nil && a.fmt != nil {
		a.compressor = NewLLMCompressor(a.model, a.fmt, "")
	}
	if a.compressor == nil {
		a.compressor = &TruncatingCompressor{}
	}
	a.offloader = NewOffloadManager(a.offloadDir, a.offloadThreshold)
	a.buildInternalToolkit()
	return a
}

func (a *DeepAgent) ID() string   { return a.id }
func (a *DeepAgent) Name() string { return a.name }
func (a *DeepAgent) Memory() memory.MemoryBase { return a.mem }

func (a *DeepAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	for _, h := range a.hooks.preReply {
		h(ctx, a, msg, nil)
	}

	if msg != nil {
		if err := a.mem.Add(ctx, msg); err != nil {
			return nil, fmt.Errorf("add message to memory: %w", err)
		}
	}

	if a.sysPrompt != "" {
		sysMsg := message.NewMsg("system", a.sysPrompt, "system")
		existing := a.mem.GetMessages()
		restored := append([]*message.Msg{sysMsg}, existing...)
		a.mem.Clear(ctx)
		a.mem.Add(ctx, restored...)
	}

	var resp *message.Msg
	var err error
	for i := 0; i < a.maxIters; i++ {
		a.maybeCompressContext(ctx)

		resp, err = a.thinkAndAct(ctx)
		if err != nil {
			return nil, err
		}
		if !hasToolUse(resp) {
			break
		}
	}

	for _, h := range a.hooks.postReply {
		h(ctx, a, msg, resp)
	}
	return resp, nil
}

func (a *DeepAgent) Observe(ctx context.Context, msg *message.Msg) error {
	if msg != nil {
		return a.mem.Add(ctx, msg)
	}
	return nil
}

func (a *DeepAgent) thinkAndAct(ctx context.Context) (*message.Msg, error) {
	msgs := a.mem.GetMessages()
	formatted, err := a.fmt.Format(msgs)
	if err != nil {
		return nil, fmt.Errorf("format messages: %w", err)
	}

	var opts []model.CallOption
	tk := a.internalToolkit
	if tk != nil && len(tk.GetSchemas()) > 0 {
		opts = append(opts, model.CallOption{
			Tools:      tk.GetSchemas(),
			ToolChoice: "auto",
		})
	}

	chatResp, err := a.model.Call(ctx, formatted, opts...)
	if err != nil {
		return nil, fmt.Errorf("model call: %w", err)
	}

	assistantMsg := &message.Msg{
		ID:        utils.ShortUUID(),
		Name:      a.name,
		Role:      "assistant",
		Content:   chatResp.Content,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
	}
	if err := a.mem.Add(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("add assistant message: %w", err)
	}

	if chatResp.HasToolUse() {
		toolUseBlocks := chatResp.GetToolUseBlocks()
		var toolResultBlocks []message.ContentBlock

		for _, block := range toolUseBlocks {
			toolName := message.GetBlockToolUseName(block)
			toolID := message.GetBlockToolUseID(block)
			toolInput := message.GetBlockToolUseInput(block)

			args, ok := toMap(toolInput)
			if !ok {
				toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
					toolID, fmt.Sprintf("invalid tool input: %v", toolInput), true,
				))
				continue
			}

			result, execErr := tk.Execute(ctx, toolName, args)
			if execErr != nil {
				toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
					toolID, execErr.Error(), true,
				))
				continue
			}

			// Check if result content should be offloaded
			content := fmt.Sprintf("%v", result.Content)
			if len(content) > a.offloadThreshold {
				msgID := utils.ShortUUID()
				ref, offloaded := a.offloader.MaybeOffload(content, msgID)
				if offloaded {
					result.Content = ref
				}
			}

			toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
				toolID, result.Content, result.IsError,
			))
		}

		toolResultMsg := &message.Msg{
			ID:        utils.ShortUUID(),
			Name:      "tool",
			Role:      "tool",
			Content:   toolResultBlocks,
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		}
		if err := a.mem.Add(ctx, toolResultMsg); err != nil {
			return nil, fmt.Errorf("add tool result: %w", err)
		}
	}

	return assistantMsg, nil
}

func (a *DeepAgent) maybeCompressContext(ctx context.Context) {
	msgs := a.mem.GetMessages()
	estimated := estimateTokens(msgs)
	threshold := int(float64(a.maxCtxTokens) * 0.85)

	if estimated > threshold && len(msgs) > 6 {
		compressed, err := a.compressor.Compress(ctx, msgs, 6)
		if err != nil {
			log.Printf("[DeepAgent] compression error: %v, continuing with original context", err)
			return
		}
		a.mem.Clear(ctx)
		a.mem.Add(ctx, compressed...)
	}
}

func estimateTokens(msgs []*message.Msg) int {
	totalChars := 0
	for _, msg := range msgs {
		for _, block := range msg.Content {
			if text := message.GetBlockText(block); text != "" {
				totalChars += len(text)
			}
		}
	}
	return totalChars / avgCharsPerToken
}

// buildInternalToolkit creates an internal toolkit that wraps the user's toolkit
// and adds built-in DeepAgent tools (e.g., delegate_task).
func (a *DeepAgent) buildInternalToolkit() {
	if a.toolkit == nil && a.subFactory == nil {
		return
	}

	a.internalToolkit = tool.NewToolkit()

	// Copy user tools
	if a.toolkit != nil {
		for _, name := range a.toolkit.GetToolNames() {
			// Re-register tools from user toolkit into internal toolkit
			// We need to get the function and schema
			schemas := a.toolkit.GetSchemas()
			for _, s := range schemas {
				if s.Function.Name == name {
					// Execute via original toolkit
					toolName := s.Function.Name
					a.internalToolkit.Register(s.Function.Name, s.Function.Description, s.Function.Parameters,
						func(ctx context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
							return a.toolkit.Execute(ctx, toolName, args)
						},
					)
					break
				}
			}
		}
	}

	// Add delegate_task tool if subagent factory is configured
	if a.subFactory != nil {
		a.internalToolkit.Register("delegate_task",
			"Delegate a task to a subagent. The subagent runs independently with its own context. Use this for complex multi-step work that would clutter your context.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_description": map[string]interface{}{
						"type":        "string",
						"description": "Clear description of what the subagent should accomplish",
					},
					"subagent_name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the subagent",
					},
					"system_prompt": map[string]interface{}{
						"type":        "string",
						"description": "Optional system prompt for the subagent",
					},
				},
				"required": []string{"task_description", "subagent_name"},
			},
			func(ctx context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
				desc, _ := args["task_description"].(string)
				name, _ := args["subagent_name"].(string)
				sysPrompt, _ := args["system_prompt"].(string)

				if desc == "" || name == "" {
					return tool.NewErrorResponse("task_description and subagent_name are required"), nil
				}

				result, err := a.subFactory.DelegateTask(ctx, DelegateTaskArgs{
					TaskDescription: desc,
					SubagentName:    name,
					SystemPrompt:    sysPrompt,
				})
				if err != nil {
					return tool.NewErrorResponse(fmt.Sprintf("subagent failed: %v", err)), nil
				}
				return tool.NewToolResponse(result), nil
			},
		)
	}
}
```

- [ ] **Step 4: Fix the `WithDeepModel` option in deep_options.go to use proper type**

Update `deep_options.go` — replace the `interface{}` version:

```go
package agent

import (
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const (
	defaultDeepMaxIters         = 50
	defaultDeepMaxCtxTokens     = 128000
	defaultDeepOffloadDir       = ".deepagent/offload/"
	defaultDeepOffloadThreshold = 8000
)

type DeepOption func(*DeepAgent)

func WithDeepName(name string) DeepOption {
	return func(a *DeepAgent) { a.name = name }
}

func WithDeepModel(m model.ChatModelBase) DeepOption {
	return func(a *DeepAgent) { a.model = m }
}

func WithDeepMemory(mem memory.MemoryBase) DeepOption {
	return func(a *DeepAgent) { a.mem = mem }
}

func WithDeepFormatter(f formatter.FormatterBase) DeepOption {
	return func(a *DeepAgent) { a.fmt = f }
}

func WithDeepToolkit(tk *tool.Toolkit) DeepOption {
	return func(a *DeepAgent) { a.toolkit = tk }
}

func WithDeepMaxIters(n int) DeepOption {
	return func(a *DeepAgent) { a.maxIters = n }
}

func WithDeepSystemPrompt(prompt string) DeepOption {
	return func(a *DeepAgent) { a.sysPrompt = prompt }
}

func WithDeepMaxContextTokens(n int) DeepOption {
	return func(a *DeepAgent) { a.maxCtxTokens = n }
}

func WithDeepOffloadDir(dir string) DeepOption {
	return func(a *DeepAgent) { a.offloadDir = dir }
}

func WithDeepOffloadThreshold(chars int) DeepOption {
	return func(a *DeepAgent) { a.offloadThreshold = chars }
}

func WithDeepCompressor(c ContextCompressor) DeepOption {
	return func(a *DeepAgent) { a.compressor = c }
}

func WithDeepSubagentFactory(f *SubagentFactory) DeepOption {
	return func(a *DeepAgent) { a.subFactory = f }
}

func WithDeepPreReply(h HookFunc) DeepOption {
	return func(a *DeepAgent) { a.hooks.preReply = append(a.hooks.preReply, h) }
}

func WithDeepPostReply(h HookFunc) DeepOption {
	return func(a *DeepAgent) { a.hooks.postReply = append(a.hooks.postReply, h) }
}
```

- [ ] **Step 5: Run all DeepAgent tests**

Run: `go test ./pkg/agent/ -v`
Expected: PASS (all existing tests + new DeepAgent tests)

- [ ] **Step 6: Run full project test suite**

Run: `go test ./... -count=1`
Expected: PASS (no regressions)

- [ ] **Step 7: Commit**

```bash
git add pkg/agent/deep.go pkg/agent/deep_options.go pkg/agent/deep_test.go
git commit -m "feat(agent): add DeepAgent with context engineering (offloading, compression, subagent delegation)"
```

---

### Task 7: DeepAgent Hooks Integration

**Files:**
- Modify: `pkg/agent/deep.go`
- Modify: `pkg/agent/deep_test.go`

- [ ] **Step 1: Write the failing test for hooks**

Add to `pkg/agent/deep_test.go`:

```go
func TestDeepAgent_Hooks(t *testing.T) {
	preCalled := false
	postCalled := false
	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{message.NewTextBlock("ok")}),
		},
	}
	ag := NewDeepAgent(
		WithDeepModel(mockM),
		WithDeepFormatter(&mockFormatter{}),
		WithDeepCompressor(&TruncatingCompressor{}),
		WithDeepPreReply(func(_ context.Context, _ AgentBase, _ *message.Msg, _ *message.Msg) {
			preCalled = true
		}),
		WithDeepPostReply(func(_ context.Context, _ AgentBase, _ *message.Msg, _ *message.Msg) {
			postCalled = true
		}),
	)
	ag.Reply(context.Background(), NewUserMsg("user", "test"))
	if !preCalled {
		t.Error("pre-reply hook not called")
	}
	if !postCalled {
		t.Error("post-reply hook not called")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./pkg/agent/ -run TestDeepAgent_Hooks -v`
Expected: PASS (hooks already implemented in deep.go Reply method)

- [ ] **Step 3: Commit**

```bash
git add pkg/agent/deep_test.go
git commit -m "test(agent): add DeepAgent hooks test"
```

---

### Task 8: Lint and Final Validation

**Files:** All

- [ ] **Step 1: Run linter**

Run: `make lint`
Expected: No errors in new files. Fix any issues.

- [ ] **Step 2: Run full test suite**

Run: `make test`
Expected: All tests pass, no regressions.

- [ ] **Step 3: Run go vet**

Run: `go vet ./pkg/agent/`
Expected: No issues.

- [ ] **Step 4: Verify build**

Run: `make build`
Expected: Build succeeds.

- [ ] **Step 5: Final commit if any fixes were needed**

```bash
git add -u
git commit -m "fix(agent): address lint/vet issues in DeepAgent implementation"
```
