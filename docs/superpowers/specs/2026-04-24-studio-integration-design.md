# agentscope-studio Integration Design

## Summary

Add agentscope-studio data reporting support to agentscope-go, enabling Go-based multi-agent applications to report run data, agent messages, and OpenTelemetry traces to an agentscope-studio server. This provides wire-protocol compatibility with the Python agentscope library's `agentscope.init(studio_url=..., project=...)` pattern.

## Motivation

Python agentscope supports connecting to [agentscope-studio](https://github.com/agentscope-ai/agentscope-studio) for real-time visualization of agent conversations, tool usage, and tracing. The Go port currently lacks this capability, making it impossible to use the studio dashboard with Go-based agents.

## Scope

### In Scope

1. **Global `Init()` function** — single call to configure studio connection
2. **Run registration** — `POST /trpc/registerRun` on init
3. **Message forwarding** — `POST /trpc/pushMessage` via agent PostReply hooks
4. **OTLP tracing** — forward OpenTelemetry traces to `{studio_url}/v1/traces`
5. **Graceful degradation** — studio failures must never crash or block the agent

### Out of Scope

- `requestUserInput` (blocking user input from studio web UI)
- `registerReply` (server supports it but Python client doesn't use it)
- WebSocket / SSE communication
- Authentication headers (studio doesn't require them)
- Modifications to the `AgentBase` interface

## Protocol Reference

The agentscope-studio server exposes a tRPC-based HTTP API. The Python client communicates via plain HTTP POST with JSON payloads. No authentication is required.

### Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/trpc/registerRun` | POST | Register a new run instance |
| `/trpc/pushMessage` | POST | Forward agent messages |
| `/v1/traces` | POST | OpenTelemetry OTLP HTTP trace ingestion |

### Payloads

**registerRun:**

```json
{
  "id": "run-uuid",
  "project": "project-name",
  "name": "run-name",
  "timestamp": "2026-04-24 10:30:00.000",
  "pid": 12345,
  "status": "running",
  "run_dir": ""
}
```

**pushMessage:**

```json
{
  "runId": "run-uuid",
  "replyId": "msg-uuid",
  "replyName": "agent-name",
  "replyRole": "assistant",
  "msg": {
    "id": "msg-uuid",
    "name": "agent-name",
    "role": "assistant",
    "content": "text or content blocks",
    "metadata": {},
    "timestamp": "2026-04-24 10:30:01.000"
  }
}
```

The `msg.content` field can be either a `string` (plain text) or an array of content block objects, each with a `type` field (`text`, `thinking`, `tool_use`, `tool_result`, `image`, `audio`, `video`).

## Architecture

### Package Structure

```
pkg/studio/
├── studio.go    # Init(), Shutdown(), GetClient() — global state management
├── client.go    # StudioClient — HTTP calls to studio server
├── types.go     # RunData, PushMessageRequest, MsgPayload data models
├── hooks.go     # PostReplyHook — agent lifecycle integration
└── convert.go   # msgToPayload — Go Msg to studio msg format conversion
```

### Data Flow

```
User calls studio.Init(url, project)
    │
    ├─ 1. Create StudioClient (HTTP client + config)
    ├─ 2. POST /trpc/registerRun
    ├─ 3. Store as global singleton
    └─ 4. SetupTracing(ctx, url+"/v1/traces")

Agent.Reply(ctx, msg)
    │
    ├─ PreReply hooks
    │      │
    │      └─ studio.PreReplyHook fires
    │             │
    │             ├─ Convert input msg → PushMessageRequest (replyRole: "user")
    │             └─ POST /trpc/pushMessage (synchronous with retry)
    │
    ├─ thinkAndAct loop (model.Call → tool.Execute)
    │
    ├─ PostReply hooks
    │      │
    │      └─ studio.PostReplyHook fires
    │             │
    │             ├─ Convert resp Msg → PushMessageRequest (replyRole: "assistant")
    │             └─ POST /trpc/pushMessage (synchronous with retry)
    │
    └─ return resp
```

### Component Design

#### 1. Global State (`studio.go`)

```go
package studio

var (
    globalMu     sync.Mutex
    globalClient *StudioClient
)

type Option func(*config)

func WithURL(url string) Option
func WithProject(project string) Option
func WithName(name string) Option
func WithRunID(id string) Option

func Init(opts ...Option) error
func Shutdown(ctx context.Context) error
func GetClient() *StudioClient
```

`Init()` behavior:
1. Build config from options, apply defaults (auto-generated runID, project name from timestamp)
2. Create `StudioClient`
3. Call `client.RegisterRun(ctx)` — POST to `/trpc/registerRun`
4. Set global client
5. Setup OTLP HTTP tracing to `{url}/v1/traces`

`GetClient()` returns nil if `Init()` hasn't been called. Agent code checks for nil before attempting studio operations.

#### 2. HTTP Client (`client.go`)

```go
type StudioClient struct {
    baseURL    string
    runID      string
    project    string
    name       string
    httpClient *http.Client
}

func (c *StudioClient) RegisterRun(ctx context.Context) error
func (c *StudioClient) PushMessage(ctx context.Context, req *PushMessageRequest) error
func (c *StudioClient) RunID() string
```

Both methods:
- Use `context.Context` for cancellation
- Retry up to 3 times on network errors (matching Python behavior)
- Log warnings on failure, never return errors that crash the caller

#### 3. Data Models (`types.go`)

```go
type RunData struct {
    ID        string `json:"id"`
    Project   string `json:"project"`
    Name      string `json:"name"`
    Timestamp string `json:"timestamp"`
    PID       int    `json:"pid"`
    Status    string `json:"status"`
    RunDir    string `json:"run_dir"`
}

type PushMessageRequest struct {
    RunID     string                 `json:"runId"`
    ReplyID   string                 `json:"replyId"`
    ReplyName string                 `json:"replyName"`
    ReplyRole string                 `json:"replyRole"`  // "user" or "assistant"
    Msg       map[string]interface{} `json:"msg"`
}
```

#### 4. Hook Integration (`hooks.go`)

Two hooks are needed to capture the full conversation flow — both user input messages and assistant response messages. This matches the Python `pre_print` hook which fires for all messages from all agents.

**PreReplyHook** — forwards the input message (user → agent) to studio:

```go
func PreReplyHook(ctx context.Context, ag agent.AgentBase, msg *message.Msg, resp *message.Msg) {
    client := GetClient()
    if client == nil || msg == nil {
        return
    }

    err := client.PushMessage(ctx, &PushMessageRequest{
        RunID:     client.RunID(),
        ReplyID:   msg.ID,
        ReplyName: msg.Name,
        ReplyRole: msg.Role,  // "user" for initial messages
        Msg:       MsgToPayload(msg),
    })
    if err != nil {
        log.Printf("studio: pushMessage (pre) failed: %v", err)
    }
}
```

**PostReplyHook** — forwards the assistant's response to studio:

```go
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

Both hooks are synchronous (matching Python's `requests.post()` behavior). Each call retries up to 3 times before logging a warning and returning. The agent's Reply() method waits for the hook to complete, but this is acceptable because: (a) the HTTP calls are fast for localhost studio, and (b) graceful degradation ensures the agent never crashes.

#### 5. Message Conversion (`convert.go`)

Convert Go `*message.Msg` to the studio-compatible `map[string]interface{}` format matching Python's `Msg.to_dict()`.

Go's `ContentBlock` type (`map[string]interface{}`) already uses the same field names as the studio server expects (`type`, `text`, `thinking`, `id`, `name`, `input`, `output`, etc.), so the conversion is mostly pass-through. The main transformation is:

- If the Msg has a single text block with only text content → flatten to a `string` (matches Python's behavior for simple text messages)
- Otherwise → pass `Content` through as `[]map[string]interface{}`

Block type mapping (already aligned, no transformation needed):

| Go Constant | Studio `type` Value |
|-------------|---------------------|
| `BlockText` | `"text"` |
| `BlockThinking` | `"thinking"` |
| `BlockToolUse` | `"tool_use"` |
| `BlockToolResult` | `"tool_result"` |
| `BlockImage` | `"image"` |
| `BlockAudio` | `"audio"` |
| `BlockVideo` | `"video"` |

ToolUse block fields: `type`, `id`, `name`, `input` — already match studio's expected format.
ToolResult block fields: `type`, `id`, `output`, `is_error` — already match studio's expected format.

### Auto-Injection Mechanism

Modify `pkg/agent/react.go` — `NewReActAgent()` constructor appends the studio hook if the global client is available:

```go
// At the end of NewReActAgent, after all options are applied:
if sc := studio.GetClient(); sc != nil {
    a.hooks.preReply = append(a.hooks.preReply, studio.PreReplyHook)
    a.hooks.postReply = append(a.hooks.postReply, studio.PostReplyHook)
}
```

This is a 4-line addition with zero performance cost when studio is not initialized.

### OTLP Tracing Integration

Modify `pkg/tracing/tracing.go` to support HTTP exporter alongside the existing gRPC exporter:

```go
func SetupTracingHTTP(ctx context.Context, endpoint string, opts ...TracingOption) (func(context.Context) error, error)
```

`studio.Init()` calls `SetupTracingHTTP` with `{studio_url}/v1/traces` as the endpoint. This sends OpenTelemetry spans to the studio server's trace ingestion endpoint.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `pkg/studio/studio.go` | New | Init, Shutdown, GetClient, global state |
| `pkg/studio/client.go` | New | StudioClient with RegisterRun, PushMessage |
| `pkg/studio/types.go` | New | RunData, PushMessageRequest data models |
| `pkg/studio/hooks.go` | New | PreReplyHook, PostReplyHook for agent integration |
| `pkg/studio/convert.go` | New | Msg to studio payload conversion |
| `pkg/agent/react.go` | Modify | 4-line auto-injection of studio hooks (pre + post) |
| `pkg/tracing/tracing.go` | Modify | Add SetupTracingHTTP function |
| `examples/studio/main.go` | New | Usage example |

## Error Handling Strategy

Following the Python agentscope pattern of graceful degradation:

1. **Network errors during Init/RegisterRun** — Return error to caller. The caller decides whether to proceed without studio.
2. **Network errors during PushMessage** — Retry up to 3 times with best-effort delivery. Log warning on final failure. Never block or crash the agent.
3. **Nil global client** — All hook functions check `GetClient() != nil` before proceeding. No-op when studio is not initialized.

## Testing Strategy

1. **Unit tests** for `convert.go` — verify Msg→payload conversion matches Python's `Msg.to_dict()` format
2. **Unit tests** for `client.go` — use `httptest.Server` to mock studio server, verify request payloads
3. **Integration test** — `studio.Init()` against a real or mocked studio server, create an agent, verify messages appear
4. **Example** — `examples/studio/main.go` demonstrating full usage

## Dependencies

- `net/http` (stdlib) — HTTP client for studio API calls
- `github.com/google/uuid` or equivalent — for generating run IDs and reply IDs
- Existing `pkg/tracing` — for OTLP HTTP exporter
- Existing `pkg/agent` — for `HookFunc` type and `AgentBase` interface
- Existing `pkg/message` — for `Msg` and `ContentBlock` types

## Open Questions

None. All design decisions have been resolved through the brainstorming process.
