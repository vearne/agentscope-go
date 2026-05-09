# AGENTS.md

## Build & Test

```bash
make build          # go build ./...
make test           # go test -v -race ./...
make lint           # golangci-lint run ./...
make fmt            # gofmt -w . && goimports -w .
make tidy           # go mod tidy
```

Run a single package:
```bash
go test -v -race ./pkg/agent/
go test -v -race -run TestReActAgent_ToolUse ./pkg/agent/
```

**Order**: `make fmt` → `make lint` → `make test`

## Go Version

Go 1.25+ required (`go.mod` specifies `1.25.3`).
CI uses Go 1.25 with golangci-lint v2.6, 5m timeout.

## Linter

`.golangci.yml` enables: `copyloopvar`, `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`.
`govet` fieldalignment check is disabled.
Linter excludes `examples/` and `third_party/`.

## Architecture

Module: `github.com/vearne/agentscope-go`

```
pkg/
├── agent/       # AgentBase interface, ReActAgent, DeepAgent, UserAgent, subagents
├── model/       # ChatModelBase interface, OpenAI/Anthropic/Gemini providers + SSE
├── formatter/   # Provider-specific formatters (OpenAI, Anthropic, Gemini)
├── message/     # Msg, ContentBlock types (text, tool_use, thinking, image, audio, video)
├── memory/      # MemoryBase interface — InMemory, Redis, SQL backends + long-term
├── pipeline/    # Sequential, Fanout, ChatRoom, MsgHub orchestration
├── tool/        # Toolkit, RegisterFunc (reflection), built-in tools, agent skills
├── session/     # JSON and Redis session persistence
├── a2a/         # Agent-to-Agent HTTP protocol (server/client/bus)
├── module/      # Thread-safe key-value state store
├── studio/      # Studio UI integration (real-time message forwarding)
├── tracing/     # OpenTelemetry OTLP gRPC/HTTP exporters
internal/
└── utils/       # ShortUUID, TimestampID
```

### Key Interfaces

| Interface | Package | Purpose |
|-----------|---------|---------|
| `AgentBase` | `pkg/agent` | `Reply`, `Observe`, `Interrupt`, `HandleInterrupt`, `Name`, `ID` |
| `ChatModelBase` | `pkg/model` | `Call`, `Stream`, `ModelName`, `IsStream` |
| `FormatterBase` | `pkg/formatter` | `Format([]*Msg) → []FormattedMessage` |
| `MemoryBase` | `pkg/memory` | `Add`, `GetMessages`, `Clear`, mark-based CRUD |

### Agent Types

- **ReActAgent** (`react.go`) — ReAct loop with tool use, tracing spans, studio forwarding
- **DeepAgent** (`deep.go`) — Extended ReAct with context compression, offloading, subagent delegation
- **UserAgent** (`user.go`) — Reads from stdin for interactive pipelines

## Conventions

### Functional Options
All constructors use `WithXxx` functional options:
```go
agent.NewReActAgent(
    agent.WithReActName("assistant"),
    agent.WithReActModel(m),
    agent.WithReActFormatter(f),
)
```

### Testing
- Mock-based unit tests — inline mock structs implementing interfaces (no mockgen)
- Mock pattern: define struct in test file, implement `ChatModelBase`/`FormatterBase` methods
- Race detector always on (`-race` flag)
- No integration tests, no external service dependencies in CI

### Error Handling
- Wrap errors with `fmt.Errorf("context: %w", err)`
- No custom error types — plain `error` returns

### Tracing
- OpenTelemetry spans on every agent reply (`invoke_agent`), model call (`chat`), format, and tool execution
- Span attributes use `gen_ai.*` OpenTelemetry semantic conventions
- Studio forwarding via `studio.ForwardMessage()` in agent constructors (hooks) and `thinkAndAct()`

### Tool Registration
Three methods:
1. Manual: `tk.Register(name, desc, params, fn)`
2. Reflection: `tk.RegisterFunc(func(ctx, args T) (*ToolResponse, error), ...)` with struct tags
3. Built-in: `tool.RegisterShellTool(tk)`, `tool.RegisterPrintTool(tk)`

## Gotchas

- **Formatter is model-specific**: Each provider (OpenAI, Anthropic, Gemini) has its own formatter. Match formatter to model.
- **ContentBlock is `map[string]interface{}`**: Not a struct. Use helper functions (`message.NewTextBlock`, `message.IsToolUseBlock`, etc.).
- **FormattedMessage is `map[string]interface{}`**: Provider-formatted output from `Formatter.Format()`.
- **No `//go:generate`**: Pure Go, no code generation, no protos.
- **Studio hooks auto-register**: When `studio.GetClient() != nil`, constructors add `preReply`/`postReply` hooks automatically. Don't add duplicate forwarding.
- **DeepAgent internal toolkit**: Wraps user tools + adds `delegate_task`. Use `a.internalToolkit` not `a.toolkit` in `thinkAndAct`.
- **Memory `Add` is variadic**: `mem.Add(ctx, msg1, msg2, ...)` — single `*Msg` or multiple.
