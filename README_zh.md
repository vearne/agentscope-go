# agentscope-go

一个用于构建多 Agent LLM 应用的 Go 框架，采用模块化、可组合的架构设计。

受 [AgentScope](https://github.com/modelscope/agentscope) 启发，agentscope-go 将地道的 Go 设计带入多 Agent 编排领域——原生支持 ReAct Agent、工具调用、多模型提供商、流水线、A2A 协议和 OpenTelemetry 链路追踪。

## 特性

- **ReAct Agent** — 内置 ReAct（推理 + 行动）Agent，支持可配置的最大迭代次数、系统提示词和钩子函数
- **多模型支持** — 支持 OpenAI、Anthropic（Claude）和 Google Gemini，各提供商独立适配；支持自定义 Base URL 以兼容 OpenAI 兼容 API
- **流式输出** — 所有模型提供商均支持基于 SSE 的流式响应
- **工具系统** — 支持手动注册、基于反射的 `RegisterFunc()`（使用结构体标签）以及内置工具（shell、print）
- **流水线** — Sequential（顺序）、Fanout（并行）、ChatRoom 和 MsgHub 四种多 Agent 编排模式
- **会话持久化** — 支持 JSON 文件和 Redis 两种会话存储后端
- **A2A 协议** — Agent-to-Agent HTTP 服务端/客户端，支持服务发现和总线注册
- **OpenTelemetry 链路追踪** — 内置 OTLP gRPC 导出器，通过 Agent 钩子实现自动埋点
- **多模态消息** — 支持文本、思考、工具调用/结果、图片、音频和视频等内容块
- **状态管理** — 线程安全的键值状态模块，用于在 Agent 间共享数据

## 安装

```bash
go get github.com/vearne/agentscope-go
```

要求 Go 1.21+。

## 快速开始

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

    msg := message.NewMsg("user", "你好！你能做什么？", "user")
    resp, err := ag.Reply(ctx, msg)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.GetTextContent())
}
```

## 架构

```
pkg/
├── agent/       # Agent 接口与实现（ReAct、User）
├── model/       # 模型提供商（OpenAI、Anthropic、Gemini）
├── formatter/   # 各提供商的消息格式化器
├── message/     # 消息类型与内容块
├── memory/      # 记忆后端（内存）
├── pipeline/    # 多 Agent 编排（Sequential、Fanout、ChatRoom、MsgHub）
├── tool/        # 工具注册与执行
├── session/     # 会话持久化（JSON、Redis）
├── a2a/         # Agent-to-Agent 协议（HTTP 服务端/客户端/总线）
├── module/      # 状态管理
└── tracing/     # OpenTelemetry 链路追踪
```

## 核心概念

### Agent

所有 Agent 均实现 `AgentBase` 接口：

```go
type AgentBase interface {
    Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error)
    Observe(ctx context.Context, msg *message.Msg) error
    Name() string
    ID() string
}
```

**ReActAgent** — 主要的 Agent 类型。它遵循 ReAct 循环：LLM 先对用户消息进行推理，可选择调用工具，然后返回最终响应。通过函数选项配置：

```go
ag := agent.NewReActAgent(
    agent.WithReActName("my-agent"),
    agent.WithReActModel(model),
    agent.WithReActFormatter(formatter),
    agent.WithReActMemory(memory.NewInMemoryMemory()),
    agent.WithReActToolkit(toolkit),
    agent.WithReActMaxIters(10),
    agent.WithReActSystemPrompt("你是一个有用的助手。"),
    agent.WithReActPreReply(preHook),
    agent.WithReActPostReply(postHook),
)
```

**UserAgent** — 从标准输入读取用户输入，适用于交互式流水线。

### 模型提供商

```go
// OpenAI（或 OpenAI 兼容 API）
m := model.NewOpenAIChatModel("gpt-4o", apiKey, "https://api.openai.com/v1", false)

// Anthropic
m := model.NewAnthropicChatModel("claude-sonnet-4-20250514", apiKey, "", false)

// Google Gemini
m := model.NewGeminiChatModel("gemini-2.5-pro", apiKey, "", false)
```

每个提供商均支持 `Call()`（同步）和 `Stream()`（SSE 流式）方法。

### 格式化器

格式化器将内部 `Msg` 对象转换为各提供商特定的请求格式：

```go
f := formatter.NewOpenAIFormatter()      // 用于 OpenAI
f := formatter.NewAnthropicFormatter()   // 用于 Anthropic
f := formatter.NewGeminiFormatter()      // 用于 Gemini
```

### 工具系统

```go
tk := tool.NewToolkit()

// 方式一：手动注册
tk.Register("calculator", "计算数学表达式", params, calcFunc)

// 方式二：基于反射注册（使用结构体标签）
type CalcArgs struct {
    Expression string `json:"expression" description:"需要计算的数学表达式"`
}
tk.RegisterFunc(func(ctx context.Context, args CalcArgs) (*tool.ToolResponse, error) {
    // ...
}, tool.RegisterOption{Name: "calc", Description: "计算器"})

// 方式三：内置工具
tool.RegisterShellTool(tk)  // execute_shell
tool.RegisterPrintTool(tk)  // print_text
```

### 流水线

**Sequential（顺序）** — Agent 依次执行，前一个的输出传递给下一个：

```go
result, err := pipeline.SequentialPipeline(ctx, []agent.AgentBase{agent1, agent2, agent3}, msg)
```

**Fanout（并行）** — 多个 Agent 并行处理相同的输入：

```go
results, err := pipeline.FanoutPipeline(ctx, []agent.AgentBase{agent1, agent2, agent3}, msg)
```

**ChatRoom（聊天室）** — 多轮对话，所有 Agent 可以观察彼此的消息：

```go
cr := pipeline.NewChatRoom([]agent.AgentBase{agent1, agent2}, announcement, 5)
history, err := cr.Run(ctx, msg)
```

**MsgHub（消息中心）** — 广播与收集模式，支持动态参与者管理：

```go
hub := pipeline.NewMsgHub(participants, announcement)
hub.Broadcast(ctx, msg)                    // 向所有参与者广播
responses, _ := hub.Gather(ctx, msg)       // 从所有参与者收集
hub.Add(newAgent)                          // 添加参与者
hub.Remove(oldAgent)                       // 移除参与者
```

### 会话持久化

```go
// JSON 文件会话
session := session.NewJSONSession("conversation.json")
session.Save(ctx, agent.Memory())  // 持久化
session.Load(ctx, agent.Memory())  // 恢复

// Redis 会话
redisSess := session.NewRedisSession("localhost:6379", "session:123",
    session.WithRedisPassword("secret"),
    session.WithRedisDB(0),
    session.WithRedisTTL(24 * time.Hour),
)
redisSess.Save(ctx, agent.Memory())
```

### A2A 协议

将本地 Agent 暴露为 HTTP 服务：

```go
card := a2a.AgentCard{Name: "my-agent", ID: "agent-1", Endpoint: "http://localhost:8080"}
srv := a2a.NewA2AServer(agent, card)
srv.Start(":8080")
```

调用远程 Agent：

```go
client := a2a.NewA2AClient("http://localhost:8080")
client.Discover(ctx)                        // 获取 Agent Card
resp, _ := client.Reply(ctx, msg)           // 调用远程 Agent
```

服务注册中心：

```go
bus := a2a.NewA2ABus()
bus.Register(card)
bus.List()     // 所有 Agent
bus.Get("id")  // 按 ID 查询
```

### 链路追踪

```go
shutdown, err := tracing.SetupTracing(ctx, "localhost:4317", tracing.WithInsecure())
defer shutdown(ctx)

// 围绕 Agent 操作创建 Span
ctx, span := tracing.StartSpan(ctx, "agent.reply", attribute.String("agent", ag.Name()))
defer span.End()
```

使用 `WithReActPreReply` / `WithReActPostReply` 钩子实现自动 Span 创建。

## 消息

消息支持多模态内容块：

```go
msg := message.NewMsg("user", "你好", "user")

// 文本
msg.SetContent("纯文本")

// 结构化内容块
msg.Content = []message.ContentBlock{
    message.NewTextBlock("描述这张图片"),
    message.NewImageBlock(message.NewURLSource("https://example.com/img.png")),
}

// 工具调用 / 结果
message.NewToolUseBlock("tool-1", "calculator", map[string]interface{}{"expr": "2+2"})
message.NewToolResultBlock("tool-1", "4", false)

// 思考（扩展思考模型）
message.NewThinkingBlock("让我一步步思考...")
```

## 示例

完整可运行的示例请参阅 [examples 目录](./examples/)：

| 示例 | 说明 |
|------|------|
| [hello](./examples/hello) | 基础示例：创建 Agent 并进行单轮对话 |
| [react_agent](./examples/react_agent) | ReAct Agent 搭配工具使用（计算器、天气查询） |
| [streaming](./examples/streaming) | 通过 SSE 流式输出模型响应 |
| [tool_usage](./examples/tool_usage) | 工具注册方式：手动、反射、内置 |
| [session_persistence](./examples/session_persistence) | 使用 JSON 会话保存/恢复 Agent 记忆 |
| [multi_model](./examples/multi_model) | 同时使用 OpenAI、Anthropic、Gemini 模型提供商 |
| [multi_agent](./examples/multi_agent) | 顺序流水线和 ChatRoom 多 Agent 对话 |
| [fanout_pipeline](./examples/fanout_pipeline) | 使用 FanoutPipeline 并行执行 |
| [msghub](./examples/msghub) | MsgHub 广播/收集，支持动态参与者管理 |
| [debate](./examples/debate) | 多 Agent 辩论（正方、反方、裁判） |
| [a2a_agent](./examples/a2a_agent) | A2A 协议：服务端、客户端和总线 |
| [tracing](./examples/tracing) | OpenTelemetry 链路追踪与 Agent 钩子 |

## API Key 配置

```bash
# OpenAI（默认）
export OPENAI_API_KEY="sk-..."

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Google Gemini
export GEMINI_API_KEY="AIza..."
```

## 开发

```bash
make build    # 构建所有包
make test     # 运行测试（带竞态检测）
make lint     # 运行 golangci-lint
make tidy     # 整理模块依赖
make fmt      # 使用 gofmt 和 goimports 格式化
```

## 许可证

本项目依据 [LICENSE](LICENSE) 文件中声明的条款授权。
