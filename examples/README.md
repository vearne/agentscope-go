# Examples

This directory contains example programs demonstrating how to use agentscope-go.

## Quick Start

```bash
# Set your API key
export OPENAI_API_KEY="your-api-key"

# Run an example
go run ./examples/hello
```

## Examples

### Getting Started

| Example | Description |
|---------|-------------|
| [hello](./hello) | Basic agent creation and single-turn conversation |
| [react_agent](./react_agent) | ReAct agent with tool usage (calculator, weather) |
| [streaming](./streaming) | Streaming model responses with SSE |

### Agent

| Example | Description |
|---------|-------------|
| [tool_usage](./tool_usage) | Tool registration: manual, reflection-based (`RegisterFunc`), and built-in tools |
| [agent_skill](./agent_skill) | Agent skills: register SKILL.md directories, generate skill prompts, custom templates |
| [session_persistence](./session_persistence) | Save and restore agent memory using JSON file sessions |
| [deep_agent](./deep_agent) | DeepAgent with context compression, result offloading, and subagent delegation |
| [multi_model](./multi_model) | Using OpenAI, Anthropic, and Gemini model providers |

### Workflow

| Example | Description |
|---------|-------------|
| [multi_agent](./multi_agent) | Sequential pipeline and ChatRoom for multi-agent conversation |
| [fanout_pipeline](./fanout_pipeline) | Parallel code review with FanoutPipeline (concurrent execution) |
| [msghub](./msghub) | MsgHub for multi-agent broadcast and gather with dynamic participants |
| [debate](./debate) | Multi-agent debate with proponent, opponent, and judge |
| [werewolves](./werewolves) | Nine-player werewolves game with role-based gameplay (werewolf, seer, witch, hunter, villager) |

### Integration

| Example | Description |
|---------|-------------|
| [a2a_agent](./a2a_agent) | Agent-to-Agent (A2A) protocol: HTTP server/client, discovery, and bus registry |
| [studio](./studio) | agentscope-studio integration: real-time visualization of agent conversations |
| [tracing](./tracing) | OpenTelemetry tracing with custom spans and agent hooks |

## API Key Setup

Most examples require an API key. Set the appropriate environment variable:

```bash
# OpenAI (default)
export OPENAI_API_KEY="sk-..."

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Google Gemini
export GEMINI_API_KEY="AIza..."
```

## Example Categories

### Agent Basics

- **hello** — The simplest example. Create a ReAct agent, send a message, print the response.
- **react_agent** — Shows how to build tools (weather, calculator) and attach them to a ReAct agent. The agent reasons about when to call tools.
- **streaming** — Demonstrates progressive token output via the `Stream` method.

### Tool System

- **tool_usage** — Covers all three registration methods:
  - Manual `Register()` with explicit parameter schema
  - Reflection-based `RegisterFunc()` with struct tags
  - Built-in tools (`RegisterShellTool`, `RegisterPrintTool`)
- **agent_skill** — Demonstrates the agent skill system:
  - Register skills from SKILL.md directories with YAML front matter
  - Generate skill prompts for agent system prompts (`GetAgentSkillPrompt`)
  - Remove skills and use custom templates
  - No API key required (standalone demo)

### Persistence

- **session_persistence** — Shows how to save conversation history to a JSON file and restore it in a new agent session.

### Multi-Model

- **multi_model** — Demonstrates using different model providers (OpenAI, Anthropic, Gemini) with their respective formatters. Skips providers without API keys.
- **deep_agent** — DeepAgent for long-running tasks with automatic context compression (LLM summarization or truncation), large result offloading to disk, and subagent delegation to independent workers.

### Pipelines

- **multi_agent** — Sequential pipeline (agents run one after another) and ChatRoom (agents observe each other's messages).
- **fanout_pipeline** — Parallel execution: multiple agents process the same input concurrently.
- **msghub** — MsgHub pattern: broadcast messages to all participants, gather responses, dynamically add/remove agents.
- **debate** — A complete multi-agent workflow: two agents debate, a third judges.
- **werewolves** — A nine-player werewolves (Mafia-style) game: 3 werewolves, 3 villagers, 1 seer, 1 witch, and 1 hunter. Demonstrates MsgHub broadcasting, sequential discussion, parallel voting, role-based prompts, and multi-phase game orchestration.

### A2A Protocol

- **a2a_agent** — Expose a local agent as an HTTP server using `A2AServer`. Discover and call remote agents using `A2AClient`. Manage agent registration with `A2ABus`.

### Observability

- **studio** — Connect to agentscope-studio for real-time visualization of agent conversations, tool usage, and tracing. A single `studio.Init()` call enables automatic message forwarding.
- **tracing** — Set up OpenTelemetry tracing with an OTLP gRPC exporter. Create spans around agent operations and use hooks (`PreReply`/`PostReply`) for automatic instrumentation.
