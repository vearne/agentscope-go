# agentscope-go

[中文文档](./README_zh.md)

A Go framework for building multi-agent LLM applications with a modular, composable architecture.

Inspired by [AgentScope](https://github.com/modelscope/agentscope), agentscope-go brings idiomatic Go design to multi-agent orchestration — with first-class support for ReAct agents, tool use, multi-model providers, pipelines, A2A protocol, and OpenTelemetry tracing.

## Features

- **ReAct Agent** — Built-in ReAct (Reason + Act) agent with configurable max iterations, system prompts, and hooks
- **Multi-Model Support** — OpenAI, Anthropic (Claude), and Google Gemini with provider-specific formatters; custom base URLs for OpenAI-compatible APIs
- **Streaming** — SSE-based streaming for all model providers
- **Tool System** — Manual registration, reflection-based `RegisterFunc()` with struct tags, and built-in tools (shell, print)
- **Pipelines** — Sequential, Fanout (parallel), ChatRoom, and MsgHub for multi-agent workflows
- **Session Persistence** — JSON file and Redis-backed session storage
- **A2A Protocol** — Agent-to-Agent HTTP server/client with service discovery and bus registry
- **OpenTelemetry Tracing** — Built-in OTLP gRPC exporter with agent hooks for automatic instrumentation
- **Multimodal Messages** — Text, thinking, tool use/result, image, audio, and video content blocks
- **State Management** — Thread-safe key-value state module for sharing data across agents

## Installation

```bash
go get github.com/vearne/agentscope-go
```

Requires Go 1.21+.

## Quick Start

```go
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
)

func main() {
    ctx := context.Background()

    m := model.NewOpenAIChatModel("gpt-4o", os.Getenv("OPENAI_API_KEY"), "", false)
    f := formatter.NewOpenAIFormatter()

    ag := agent.NewReActAgent(
        agent.WithReActName("assistant"),
        agent.WithReActModel(m),
        agent.WithReActFormatter(f),
        agent.WithReActMemory(memory.NewInMemoryMemory()),
    )

    msg := message.NewMsg("user", "Hello! What can you do?", "user")
    resp, err := ag.Reply(ctx, msg)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.GetTextContent())
}
```

## Architecture

```
pkg/
├── agent/       # Agent interfaces and implementations (ReAct, User)
├── model/       # Model providers (OpenAI, Anthropic, Gemini)
├── formatter/   # Provider-specific message formatters
├── message/     # Message types and content blocks
├── memory/      # Memory backends (in-memory)
├── pipeline/    # Multi-agent orchestration (Sequential, Fanout, ChatRoom, MsgHub)
├── tool/        # Tool registration and execution
├── session/     # Session persistence (JSON, Redis)
├── a2a/         # Agent-to-Agent protocol (HTTP server/client/bus)
├── module/      # State management
└── tracing/     # OpenTelemetry tracing
```

## Core Concepts

### Agent

All agents implement the `AgentBase` interface:

```go
type AgentBase interface {
    Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error)
    Observe(ctx context.Context, msg *message.Msg) error
    Name() string
    ID() string
}
```

**ReActAgent** — The primary agent type. It follows the ReAct loop: the LLM reasons about the user's message, optionally calls tools, and returns a final response. Configurable via functional options:

```go
ag := agent.NewReActAgent(
    agent.WithReActName("my-agent"),
    agent.WithReActModel(model),
    agent.WithReActFormatter(formatter),
    agent.WithReActMemory(memory.NewInMemoryMemory()),
    agent.WithReActToolkit(toolkit),
    agent.WithReActMaxIters(10),
    agent.WithReActSystemPrompt("You are a helpful assistant."),
    agent.WithReActPreReply(preHook),
    agent.WithReActPostReply(postHook),
)
```

**UserAgent** — Reads input from stdin. Useful for interactive pipelines.

### Model Providers

```go
// OpenAI (or OpenAI-compatible APIs)
m := model.NewOpenAIChatModel("gpt-4o", apiKey, "https://api.openai.com/v1", false)

// Anthropic
m := model.NewAnthropicChatModel("claude-sonnet-4-20250514", apiKey, "", false)

// Google Gemini
m := model.NewGeminiChatModel("gemini-2.5-pro", apiKey, "", false)
```

Each provider supports both `Call()` (synchronous) and `Stream()` (SSE streaming) methods.

### Formatters

Formatters convert internal `Msg` objects into provider-specific request formats:

```go
f := formatter.NewOpenAIFormatter()      // for OpenAI
f := formatter.NewAnthropicFormatter()   // for Anthropic
f := formatter.NewGeminiFormatter()      // for Gemini
```

### Tool System

```go
tk := tool.NewToolkit()

// Method 1: Manual registration
tk.Register("calculator", "Evaluate a math expression", params, calcFunc)

// Method 2: Reflection-based registration with struct tags
type CalcArgs struct {
    Expression string `json:"expression" description:"The math expression to evaluate"`
}
tk.RegisterFunc(func(ctx context.Context, args CalcArgs) (*tool.ToolResponse, error) {
    // ...
}, tool.RegisterOption{Name: "calc", Description: "Calculate"})

// Method 3: Built-in tools
tool.RegisterShellTool(tk)  // execute_shell
tool.RegisterPrintTool(tk)  // print_text
```

### Pipelines

**Sequential** — Agents run one after another, passing the output forward:

```go
result, err := pipeline.SequentialPipeline(ctx, []agent.AgentBase{agent1, agent2, agent3}, msg)
```

**Fanout** — Agents run in parallel on the same input:

```go
results, err := pipeline.FanoutPipeline(ctx, []agent.AgentBase{agent1, agent2, agent3}, msg)
```

**ChatRoom** — Multi-round conversation where all agents observe each other's messages:

```go
cr := pipeline.NewChatRoom([]agent.AgentBase{agent1, agent2}, announcement, 5)
history, err := cr.Run(ctx, msg)
```

**MsgHub** — Broadcast and gather pattern with dynamic participants:

```go
hub := pipeline.NewMsgHub(participants, announcement)
hub.Broadcast(ctx, msg)                    // send to all
responses, _ := hub.Gather(ctx, msg)       // collect from all
hub.Add(newAgent)                          // add participant
hub.Remove(oldAgent)                       // remove participant
```

### Session Persistence

```go
// JSON file session
session := session.NewJSONSession("conversation.json")
session.Save(ctx, agent.Memory())  // persist
session.Load(ctx, agent.Memory())  // restore

// Redis session
redisSess := session.NewRedisSession("localhost:6379", "session:123",
    session.WithRedisPassword("secret"),
    session.WithRedisDB(0),
    session.WithRedisTTL(24 * time.Hour),
)
redisSess.Save(ctx, agent.Memory())
```

### A2A Protocol

Expose a local agent as an HTTP service:

```go
card := a2a.AgentCard{Name: "my-agent", ID: "agent-1", Endpoint: "http://localhost:8080"}
srv := a2a.NewA2AServer(agent, card)
srv.Start(":8080")
```

Call a remote agent:

```go
client := a2a.NewA2AClient("http://localhost:8080")
client.Discover(ctx)                        // fetch agent card
resp, _ := client.Reply(ctx, msg)           // call remote agent
```

Service registry:

```go
bus := a2a.NewA2ABus()
bus.Register(card)
bus.List()     // all agents
bus.Get("id")  // by ID
```

### Tracing

```go
shutdown, err := tracing.SetupTracing(ctx, "localhost:4317", tracing.WithInsecure())
defer shutdown(ctx)

// Create spans around agent operations
ctx, span := tracing.StartSpan(ctx, "agent.reply", attribute.String("agent", ag.Name()))
defer span.End()
```

Use `WithReActPreReply` / `WithReActPostReply` hooks for automatic span creation.

## Messages

Messages support multimodal content blocks:

```go
msg := message.NewMsg("user", "Hello", "user")

// Text
msg.SetContent("plain text")

// Structured content blocks
msg.Content = []message.ContentBlock{
    message.NewTextBlock("Describe this image"),
    message.NewImageBlock(message.NewURLSource("https://example.com/img.png")),
}

// Tool use / result
message.NewToolUseBlock("tool-1", "calculator", map[string]interface{}{"expr": "2+2"})
message.NewToolResultBlock("tool-1", "4", false)

// Thinking (extended thinking models)
message.NewThinkingBlock("Let me think step by step...")
```

## Examples

See the [examples directory](./examples/) for complete, runnable examples:

| Example | Description |
|---------|-------------|
| [hello](./examples/hello) | Basic agent creation and single-turn conversation |
| [react_agent](./examples/react_agent) | ReAct agent with tool usage (calculator, weather) |
| [streaming](./examples/streaming) | Streaming model responses with SSE |
| [tool_usage](./examples/tool_usage) | Tool registration methods: manual, reflection, built-in |
| [session_persistence](./examples/session_persistence) | Save/restore agent memory via JSON session |
| [multi_model](./examples/multi_model) | Using OpenAI, Anthropic, and Gemini providers |
| [multi_agent](./examples/multi_agent) | Sequential pipeline and ChatRoom |
| [fanout_pipeline](./examples/fanout_pipeline) | Parallel execution with FanoutPipeline |
| [msghub](./examples/msghub) | MsgHub broadcast/gather with dynamic participants |
| [debate](./examples/debate) | Multi-agent debate with judge |
| [a2a_agent](./examples/a2a_agent) | A2A protocol: server, client, and bus |
| [tracing](./examples/tracing) | OpenTelemetry tracing with agent hooks |

## API Key Setup

```bash
# OpenAI (default)
export OPENAI_API_KEY="sk-..."

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Google Gemini
export GEMINI_API_KEY="AIza..."
```

## Development

```bash
make build    # build all packages
make test     # run tests with race detector
make lint     # run golangci-lint
make tidy     # tidy modules
make fmt      # format with gofmt and goimports
```

## License

This project is licensed under the terms found in the [LICENSE](LICENSE) file.
