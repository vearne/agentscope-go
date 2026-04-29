# DeepAgent Design Specification

**Date**: 2026-04-29
**Status**: Draft
**Aligns with**: LangChain Deep Agents — context engineering terminology

## Overview

DeepAgent is a long-running autonomous agent for agentscope-go, designed for scenarios where a single agent must operate over many iterations with complex tool interactions. It implements three core context engineering mechanisms:

1. **Offloading** — automatically stores large tool results to the filesystem, keeping only a reference in conversation history
2. **Summarization** — compresses old conversation history via a pluggable compressor when the context window approaches capacity
3. **Subagent delegation** — delegates tasks to lightweight in-process subagents with isolated context, receiving only final results

DeepAgent implements the existing `AgentBase` interface and integrates with all current infrastructure (pipelines, A2A, session persistence, Studio, tracing).

## Goals

- Support both coding-agent scenarios (heavy file/command tool usage) and long-conversation scenarios
- Prevent context window overflow in long-running sessions
- Keep the main agent's context clean by isolating subagent work
- Provide extension points (interfaces) so users can customize compression, offloading, and subagent behavior
- Follow existing project conventions (functional options, `AgentBase` interface, hooks pattern)

## Non-Goals

- Distributed subagent orchestration over network (can be added later via A2A)
- Built-in token counting library (use heuristic character-based estimation)
- Replacement for ReActAgent (ReActAgent remains the default for short conversations)

## File Structure

All new files in `pkg/agent/`. No modifications to existing files.

**Note on the `delegate_task` tool**: `NewDeepAgent()` creates an internal toolkit that wraps the user-provided toolkit and adds the `delegate_task` built-in tool. The user's original toolkit is not mutated. If no subagent factory is configured, `delegate_task` is not registered.

```
pkg/agent/
├── agent.go           # existing — AgentBase interface
├── react.go           # existing — ReActAgent
├── user.go            # existing — UserAgent
├── deep.go            # NEW — DeepAgent struct + execution loop
├── deep_options.go    # NEW — WithDeep* functional options
├── compressor.go      # NEW — ContextCompressor interface + LLMCompressor
├── offload.go         # NEW — OffloadManager (filesystem-based)
└── subagent.go        # NEW — SubagentFactory (in-process ReActAgent instances)
```

## Architecture

### Layered Design

```
DeepAgent
├── Execution Layer       — independent think-and-act loop
├── Context Management    — ContextCompressor + OffloadManager
└── Delegation Layer      — SubagentFactory + delegate tool
```

Each layer has a single responsibility and communicates through well-defined interfaces. Layers can be tested independently.

### DeepAgent Struct

```go
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
    maxCtxTokens     int   // trigger threshold for compression
    offloadThreshold int   // character threshold for offloading tool results

    // Delegation
    subFactory *SubagentFactory

    // Lifecycle hooks (reuses existing hooks struct)
    hooks hooks
}
```

`DeepAgent` satisfies the `AgentBase` interface (`Reply`, `Observe`, `Name`, `ID`), making it compatible with all existing pipelines, A2A, and session mechanisms.

## Execution Loop

The `Reply` method implements an independent think-and-act loop with integrated context management:

```
Reply(msg):
  1. Add user message to memory
  2. Inject system prompt (prepend to memory)
  3. Loop up to maxIters:
     a. maybeCompressContext()  — check context size, trigger summarization if > 85%
     b. thinkAndAct()           — memory → format → model.Call
     c. If model response has tool use:
        - executeTools()        — run each tool, collect results
        - maybeOffloadResults() — offload large results to filesystem
        - Add tool result to memory
        - Continue loop
     d. If no tool use:
        - Return final response
  4. Run post-reply hooks
```

### thinkAndAct()

Identical flow to ReActAgent's method but as an independent implementation:
1. `msgs := a.mem.GetMessages()`
2. `formatted, err := a.fmt.Format(msgs)`
3. `chatResp, err := a.model.Call(ctx, formatted, toolOpts...)`
4. Construct assistant message, add to memory
5. Check for tool use blocks, return to caller for execution

### maybeCompressContext()

Called before each model invocation:
1. Estimate current context size: `len(msgs) * avgCharsPerToken` (heuristic)
2. If estimated tokens > `maxCtxTokens * 0.85`:
   - Call `compressor.Compress(ctx, msgs, keepRecent=6)`
   - Replace memory contents with compressed result
   - Offloaded original messages are saved to the offload directory

### executeTools()

For each tool use block in the model response:
1. Extract tool name, ID, input arguments
2. Call `a.toolkit.Execute(ctx, toolName, args)`
3. Wrap result in `ToolResultBlock`
4. Check result size against `offloadThreshold`

### maybeOffloadResults()

For each tool result:
1. Get text content from the tool result
2. If `len(content) > offloadThreshold`:
   - Call `offloader.Offload(content, msgID)`
   - Replace content with the returned reference string
3. Add (possibly truncated) tool result to memory

## Context Compressor

### Interface

```go
// ContextCompressor compresses old conversation history into a summary.
type ContextCompressor interface {
    Compress(ctx context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error)
}
```

- Input: full message history + number of recent messages to preserve
- Output: compressed message list (summary + recent messages)
- Error: return original messages unchanged on failure

### LLMCompressor (default implementation)

```go
type LLMCompressor struct {
    model  model.ChatModelBase
    fmt    formatter.FormatterBase
    prompt string // compression instruction, has sensible default
}
```

Behavior:
1. Split messages: `oldMsgs = msgs[:len(msgs)-keepRecent]`, `recentMsgs = msgs[len(msgs)-keepRecent:]`
2. Build a compression prompt: "Summarize the following conversation, preserving: session goals, key decisions, artifacts created, current task status, next steps"
3. Call `model.Call()` with old messages + compression instruction
4. Return `[summaryMsg] + recentMsgs`

The `NewLLMCompressor()` constructor provides a default compression prompt. Users can override via `WithCompressionPrompt()`.

### TruncatingCompressor (alternative simple implementation)

A built-in simple compressor that just drops old messages beyond a threshold, keeping only the N most recent. Useful for testing or when LLM calls for summarization are too expensive.

```go
type TruncatingCompressor struct{}

func (c *TruncatingCompressor) Compress(ctx context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error) {
    if len(msgs) <= keepRecent {
        return msgs, nil
    }
    return msgs[len(msgs)-keepRecent:], nil
}
```

## Offload Manager

### Structure

```go
type OffloadManager struct {
    dir       string // filesystem directory for offloaded content
    threshold int    // character count threshold
}
```

### Behavior

**Offloading**: When tool result content exceeds `threshold` characters:
1. Generate filename: `{dir}/{messageID}.txt`
2. Write full content to file (create directory if needed)
3. Return reference string: `[Result offloaded to {path}. Preview: {first 200 chars}...]`
4. The reference replaces the full content in the `ToolResultBlock`

**Reading back**: `Read(filepath) (string, error)` reads a previously offloaded file.

**Configuration**:
- Default directory: `.deepagent/offload/`
- Default threshold: 8000 characters (approximately 2000 tokens)
- Directory is created on first offload if it doesn't exist

### Offload file format

Each offloaded file is plain UTF-8 text containing the original tool result content. The filename is `{messageID}.txt`, making it easy to correlate with conversation history.

## Subagent Delegation

### SubagentFactory

```go
type SubagentFactory struct {
    model   model.ChatModelBase
    fmt     formatter.FormatterBase
    toolkit *tool.Toolkit // default toolkit for subagents (optional)
}

type SubagentConfig struct {
    Name         string
    SystemPrompt string
    MaxIters     int
    Toolkit      *tool.Toolkit // nil = use factory's default toolkit
}
```

### Creating a subagent

```go
func (f *SubagentFactory) Create(cfg SubagentConfig) *ReActAgent
```

Creates a new `ReActAgent` instance with:
- Its own `InMemoryMemory` (contextually isolated from the main agent)
- The provided or default model, formatter, and toolkit
- The specified system prompt and max iterations

### Delegation tool

DeepAgent registers a built-in `delegate_task` tool that the LLM can invoke:

```go
// Tool schema
{
    "name": "delegate_task",
    "description": "Delegate a task to a subagent. The subagent runs independently with its own context. Use this for complex multi-step work that would clutter your context.",
    "parameters": {
        "task_description": "Clear description of what the subagent should accomplish",
        "subagent_name": "Name for the subagent",
        "system_prompt": "Optional system prompt for the subagent"
    }
}
```

When invoked:
1. Factory creates a new `ReActAgent` with the given config
2. Sends the task as a user message
3. Subagent runs its full ReAct loop independently
4. Only the final text result is returned to the main agent
5. Subagent's intermediate steps never enter the main agent's memory

### Context isolation guarantee

Each subagent:
- Has its own `InMemoryMemory` — no shared state with main agent
- Runs in the same goroutine (sequential, blocking call from main agent's perspective)
- Returns only a string result, not intermediate tool calls or memory entries
- Is garbage-collected after the delegation call completes

## Functional Options

All configuration follows the established `WithReAct*` pattern:

```go
type DeepOption func(*DeepAgent)

// Core (mirrors ReActAgent options)
func WithDeepName(name string) DeepOption
func WithDeepModel(m model.ChatModelBase) DeepOption
func WithDeepMemory(mem memory.MemoryBase) DeepOption
func WithDeepFormatter(f formatter.FormatterBase) DeepOption
func WithDeepToolkit(tk *tool.Toolkit) DeepOption
func WithDeepMaxIters(n int) DeepOption           // default: 50
func WithDeepSystemPrompt(prompt string) DeepOption

// Context management
func WithDeepMaxContextTokens(n int) DeepOption    // default: 128000
func WithDeepOffloadDir(dir string) DeepOption     // default: ".deepagent/offload/"
func WithDeepOffloadThreshold(chars int) DeepOption // default: 8000
func WithDeepCompressor(c ContextCompressor) DeepOption // default: LLMCompressor

// Delegation
func WithDeepSubagentFactory(f *SubagentFactory) DeepOption

// Hooks (reuses HookFunc type)
func WithDeepPreReply(h HookFunc) DeepOption
func WithDeepPostReply(h HookFunc) DeepOption
```

### Constructor

```go
func NewDeepAgent(opts ...DeepOption) *DeepAgent
```

Applies all options, initializes defaults:
- Memory defaults to `InMemoryMemory`
- Compressor defaults to `LLMCompressor` using the same model/formatter
- Offloader defaults to `.deepagent/offload/` with 8000 char threshold
- MaxIters defaults to 50 (higher than ReActAgent's 10, reflecting long-running nature)
- Studio hooks auto-attached if studio client is available (same pattern as ReActAgent)

## Usage Example

```go
m := model.NewOpenAIChatModel("gpt-4o", apiKey, "", false)
f := formatter.NewOpenAIFormatter()
tk := tool.NewToolkit()
tool.RegisterShellTool(tk)

// Create subagent factory (subagents share model/formatter)
subFactory := agent.NewSubagentFactory(m, f, tk)

ag := agent.NewDeepAgent(
    agent.WithDeepName("deep-assistant"),
    agent.WithDeepModel(m),
    agent.WithDeepFormatter(f),
    agent.WithDeepToolkit(tk),
    agent.WithDeepMaxContextTokens(128000),
    agent.WithDeepOffloadDir("./offload"),
    agent.WithDeepSubagentFactory(subFactory),
    agent.WithDeepSystemPrompt("You are a deep research assistant. Use delegate_task for complex sub-problems."),
)

msg := message.NewMsg("user", "Analyze the entire codebase and generate a report", "user")
resp, err := ag.Reply(ctx, msg)
```

## Token Estimation

Since the project has no built-in tokenizer, DeepAgent uses a heuristic:

```go
const avgCharsPerToken = 4

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
```

This is intentionally conservative. Users can provide their own `ContextCompressor` with precise token counting if needed.

## Integration Points

### With existing pipelines

DeepAgent implements `AgentBase`, so it works with all existing pipelines:

```go
// Sequential with a DeepAgent
result, err := pipeline.SequentialPipeline(ctx, []agent.AgentBase{deepAgent, reactAgent}, msg)

// Fanout
results, err := pipeline.FanoutPipeline(ctx, []agent.AgentBase{deepAgent1, deepAgent2}, msg)
```

### With session persistence

DeepAgent uses `MemoryBase` like ReActAgent, so session persistence works unchanged:

```go
session := session.NewJSONSession("deep-conversation.json")
session.Save(ctx, ag.Memory())
session.Load(ctx, ag.Memory())
```

### With Studio

Auto-integrated via hooks (same pattern as ReActAgent). If `studio.GetClient()` is non-nil, pre/post hooks forward messages to Studio.

### With tracing

DeepAgent uses the same `tracing.StartSpan` pattern as ReActAgent for OpenTelemetry integration.

## Error Handling

- **Compressor failure**: Log warning, continue with original (uncompressed) messages. Never block the agent loop.
- **Offload write failure**: Log warning, keep full content in memory. Degraded but functional.
- **Subagent failure**: Return error as tool result content (with `isError=true`), allowing the main agent to handle it.
- **Model call failure**: Propagate error up from `Reply()`, same as ReActAgent.

## Testing Strategy

Each layer tested independently:

1. **Compressor tests**: Unit tests for `LLMCompressor` and `TruncatingCompressor` with mock model
2. **OffloadManager tests**: Unit tests with temp directories, verify write/read/reference behavior
3. **SubagentFactory tests**: Verify subagent creation, context isolation, result propagation
4. **DeepAgent integration tests**: End-to-end test with mock model that exercises the full loop
5. **Offload + compress integration**: Verify that large tool results trigger offloading, and accumulated history triggers compression

## Dependencies

No new external dependencies. DeepAgent uses only existing project packages:
- `pkg/model` — ChatModelBase
- `pkg/formatter` — FormatterBase
- `pkg/memory` — MemoryBase
- `pkg/tool` — Toolkit, ToolResponse
- `pkg/message` — Msg, ContentBlock
- `pkg/tracing` — StartSpan
- `pkg/studio` — GetClient, ForwardMessage
- Standard library: `os`, `path/filepath`, `fmt`, `context`
