# AgentScope-Go 移植实施计划

> 基于 [agentscope-ai/agentscope](https://github.com/agentscope-ai/agentscope) v1.0.x（Apache-2.0）
> 范围：核心 + 分布式（session, tracing, a2a）
> LLM Provider：OpenAI 兼容 + Anthropic + Gemini

---

## 一、Go 项目结构

```
agentscope-go/
├── go.mod                          # github.com/vearne/agentscope-go
├── go.sum
├── LICENSE
├── README.md
│
├── pkg/                            # 公开 API
│   ├── message/                    # Phase 1: 消息系统
│   │   ├── msg.go                  # Msg 结构体
│   │   ├── block.go                # ContentBlock 类型 (Text/Thinking/ToolUse/ToolResult/Image/Audio/Video)
│   │   └── source.go               # Base64Source, URLSource
│   │
│   ├── model/                      # Phase 2: LLM 抽象层
│   │   ├── model.go                # ChatModelBase 接口 + ChatResponse
│   │   ├── openai.go               # OpenAI 兼容 (覆盖 DashScope/DeepSeek/Ollama)
│   │   ├── anthropic.go            # Anthropic Claude
│   │   └── gemini.go               # Google Gemini
│   │
│   ├── formatter/                  # Phase 2: 消息格式适配
│   │   ├── formatter.go            # FormatterBase 接口
│   │   ├── openai.go               # OpenAI Chat/MultiAgent 格式化
│   │   ├── anthropic.go            # Anthropic Chat/MultiAgent 格式化
│   │   └── gemini.go               # Gemini Chat/MultiAgent 格式化
│   │
│   ├── memory/                     # Phase 3: 记忆系统
│   │   ├── memory.go               # MemoryBase 接口
│   │   ├── inmemory.go             # InMemory 实现
│   │   └── redis.go                # Redis 实现
│   │
│   ├── tool/                       # Phase 3: 工具系统
│   │   ├── toolkit.go              # Toolkit 注册/调用
│   │   ├── schema.go               # JSON Schema 工具描述
│   │   └── builtin.go              # 内置工具 (代码执行/Shell/文件)
│   │
│   ├── agent/                      # Phase 4: Agent
│   │   ├── agent.go                # AgentBase 接口
│   │   ├── react.go                # ReActAgent
│   │   └── user.go                 # UserAgent
│   │
│   ├── pipeline/                   # Phase 5: 工作流编排
│   │   ├── msghub.go               # MsgHub
│   │   ├── sequential.go           # SequentialPipeline
│   │   ├── fanout.go               # FanoutPipeline
│   │   └── chatroom.go             # ChatRoom
│   │
│   ├── session/                    # Phase 6: 会话持久化
│   │   ├── session.go              # SessionBase 接口
│   │   ├── json_session.go         # JSON 文件持久化
│   │   └── redis_session.go        # Redis 持久化
│   │
│   ├── tracing/                    # Phase 6: 链路追踪
│   │   └── tracing.go              # OpenTelemetry 集成
│   │
│   ├── a2a/                        # Phase 6: A2A 协议
│   │   └── a2a.go                  # Agent-to-Agent 协议
│   │
│   └── module/                     # 状态管理基类
│       └── state.go                # StateModule
│
├── internal/                       # 内部实现
│   └── utils/
│       └── uuid.go                 # ID 生成
│
├── examples/                       # 示例
│   ├── hello/
│   │   └── main.go                 # Hello AgentScope
│   ├── react_agent/
│   │   └── main.go                 # ReAct Agent 示例
│   └── multi_agent/
│       └── main.go                 # 多 Agent 对话示例
│
└── Makefile
```

---

## 二、核心接口设计

### 2.1 Message 消息系统

```go
// pkg/message/block.go
type BlockType string

const (
    BlockText       BlockType = "text"
    BlockThinking   BlockType = "thinking"
    BlockToolUse    BlockType = "tool_use"
    BlockToolResult BlockType = "tool_result"
    BlockImage      BlockType = "image"
    BlockAudio      BlockType = "audio"
    BlockVideo      BlockType = "video"
)

type ContentBlock map[string]interface{}  // Typed dict pattern

func NewTextBlock(text string) ContentBlock
func NewThinkingBlock(thinking string) ContentBlock
func NewToolUseBlock(id, name string, input interface{}) ContentBlock
func NewToolResultBlock(id string, result interface{}, isError bool) ContentBlock
func NewImageBlock(source Source) ContentBlock
// ...
```

```go
// pkg/message/msg.go
type Msg struct {
    ID           string         `json:"id"`
    Name         string         `json:"name"`
    Role         string         `json:"role"`           // "user" | "assistant" | "system"
    Content      []ContentBlock `json:"content"`        // 多模态内容块列表
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
    Timestamp    string         `json:"timestamp"`
    InvocationID string        `json:"invocation_id,omitempty"`
}

func NewMsg(name string, content interface{}, role string) *Msg
func (m *Msg) GetTextContent() string          // 提取文本内容
func (m *Msg) GetContentBlocks() []ContentBlock
func (m *Msg) Clone() *Msg
```

**设计要点**：
- Python 版用 TypedDict 表示 ContentBlock，Go 用 `map[string]interface{}` + 类型安全构造函数
- `Content` 字段支持 `string`（纯文本）和 `[]ContentBlock`（多模态）两种模式
- 所有消息有唯一 ID（UUID short）

### 2.2 Model 模型层

```go
// pkg/model/model.go
type ChatModelBase interface {
    // Call 同步调用模型（内部实现异步）
    Call(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (*ChatResponse, error)
    // Stream 流式调用模型
    Stream(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (<-chan ChatResponseChunk, error)
    // ModelName 返回模型名
    ModelName() string
    // IsStream 是否流式
    IsStream() bool
}

type ChatResponse struct {
    ID       string         `json:"id"`
    Choices  []Choice       `json:"choices"`
    Usage    Usage          `json:"usage"`
    Raw      interface{}    `json:"-"`              // 原始响应
}

type Choice struct {
    Index        int          `json:"index"`
    Message      MsgContent   `json:"message"`
    FinishReason string       `json:"finish_reason"`
}

type CallOption struct {
    Tools       []ToolSchema
    ToolChoice  string        // "auto" | "none" | "required" | function name
    Temperature *float64
    MaxTokens   *int
    Stop        []string
}
```

```go
// OpenAI 兼容实现
type OpenAIChatModel struct {
    modelName  string
    apiKey     string
    baseURL    string           // 可覆盖为 DashScope/DeepSeek/Ollama 端点
    stream     bool
    httpClient *http.Client
}
```

**设计要点**：
- 统一接口，不同 Provider 通过实现 `ChatModelBase` 适配
- OpenAI 兼容模式覆盖 DashScope/DeepSeek/Ollama（它们都是 OpenAI API 兼容的）
- `CallOption` 使用 functional options 模式
- 流式输出用 Go channel（`<-chan ChatResponseChunk`）而非 Python 的 `AsyncGenerator`

### 2.3 Formatter 格式化层

```go
// pkg/formatter/formatter.go

// FormattedMessage 是格式化后的消息，可直接传给 LLM API
type FormattedMessage map[string]interface{}

// FormatterBase 将内部 Msg 转换为 LLM API 所需格式
type FormatterBase interface {
    // FormatChatMessages 将消息历史格式化为 Chat 格式（单Agent）
    FormatChatMessages(ctx context.Context, memory MemoryReader) ([]FormattedMessage, error)
    // FormatMultiAgentMessages 将消息历史格式化为 MultiAgent 格式
    FormatMultiAgentMessages(ctx context.Context, memory MemoryReader) ([]FormattedMessage, error)
}

// MemoryReader 只读记忆接口（formatter 不需要写权限）
type MemoryReader interface {
    GetMessages() []*Msg
}
```

**设计要点**：
- Python 版每个 Provider 有 Chat 和 MultiAgent 两种格式化器
- Chat 模式：标准 `system/user/assistant` 角色
- MultiAgent 模式：保留发言者 name 信息
- 每种 LLM Provider 的 tool_use/tool_result 格式不同，formatter 负责转换

### 2.4 Memory 记忆系统

```go
// pkg/memory/memory.go
type MemoryBase interface {
    // Add 添加消息到记忆
    Add(ctx context.Context, msgs ...*Msg) error
    // GetMessages 获取所有消息
    GetMessages() []*Msg
    // Clear 清空记忆
    Clear(ctx context.Context) error
    // Size 返回消息数量
    Size() int
    // ToStrList 将记忆转为字符串列表（用于截断策略）
    ToStrList() []string
}

// TruncatedMemory 支持截断的记忆接口
type TruncatedMemory interface {
    MemoryBase
    // Truncate 按策略截断记忆
    Truncate(ctx context.Context, maxSize int) error
}
```

```go
// InMemory 实现
type InMemoryMemory struct {
    messages []*Msg
    mu       sync.RWMutex
}
```

**设计要点**：
- 用接口隔离读写权限（`MemoryReader` vs `MemoryBase`）
- InMemory 是默认实现，Redis 可选
- 后续可扩展 SQLAlchemy/Tablestore 后端

### 2.5 Tool 工具系统

```go
// pkg/tool/schema.go
type ToolSchema struct {
    Type     string       `json:"type"`              // "function"
    Function FuncSchema   `json:"function"`
}

type FuncSchema struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`  // JSON Schema
}

// ToolResponse 工具执行结果
type ToolResponse struct {
    Content interface{} `json:"content"`
    IsError bool        `json:"is_error,omitempty"`
}
```

```go
// pkg/tool/toolkit.go
type ToolFunc func(ctx context.Context, args map[string]interface{}) (*ToolResponse, error)

type Toolkit struct {
    tools map[string]*registeredTool
    mu    sync.RWMutex
}

type registeredTool struct {
    schema    ToolSchema
    function  ToolFunc
}

// Register 注册工具函数
func (t *Toolkit) Register(name, description string, params map[string]interface{}, fn ToolFunc) error

// RegisterFunc 通过反射自动提取参数 Schema（类似 Python 的 docstring_parser）
func (t *Toolkit) RegisterFunc(fn interface{}, opts ...RegisterOption) error

// Execute 执行工具
func (t *Toolkit) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResponse, error)

// GetSchemas 获取所有工具的 JSON Schema
func (t *Toolkit) GetSchemas() []ToolSchema
```

**设计要点**：
- Python 版用 `docstring_parser` 自动生成 Schema，Go 可用 struct tag + 反射
- Toolkit 管理工具的注册、Schema 生成、执行
- 工具函数签名统一为 `func(ctx, args) -> (*ToolResponse, error)`

### 2.6 Agent

```go
// pkg/agent/agent.go
type AgentBase interface {
    // Reply Agent 核心逻辑 - 接收消息，返回回复
    Reply(ctx context.Context, msg *Msg) (*Msg, error)
    // Observe 观察消息但不回复
    Observe(ctx context.Context, msg *Msg) error
    // Name 返回 Agent 名称
    Name() string
    // ID 返回 Agent 唯一标识
    ID() string
}
```

```go
// pkg/agent/react.go

// ReActAgent 实现 ReAct (Reason+Act) 循环
type ReActAgent struct {
    id        string
    name      string
    sysPrompt string
    model     model.ChatModelBase
    memory    memory.MemoryBase
    formatter formatter.FormatterBase
    toolkit   *tool.Toolkit
    // ...
}

func NewReActAgent(opts ...ReActOption) *ReActAgent

func (a *ReActAgent) Reply(ctx context.Context, msg *Msg) (*Msg, error) {
    // 1. 如果 msg != nil，加入 memory
    // 2. 循环：
    //    a. formatter.FormatChatMessages(memory)
    //    b. model.Call(formatted, tools=toolkit.GetSchemas())
    //    c. 如果有 tool_use → toolkit.Execute() → 结果加入 memory → 继续循环
    //    d. 如果纯文本回复 → 加入 memory → 返回
}
```

**设计要点**：
- ReAct 循环是核心：LLM 决定调用工具 → 执行工具 → 结果反馈给 LLM → 重复直到完成
- Python 版有 `max_iters` 限制防止死循环
- 支持 `context.Context` 取消（对应 Python 的 `asyncio.CancelledError`）
- Hooks 系统用 Go 的 channel 或回调函数替代

### 2.7 Pipeline 工作流

```go
// pkg/pipeline/sequential.go
func SequentialPipeline(ctx context.Context, agents []AgentBase, msg *Msg) (*Msg, error)
// 串行执行：agent1(msg) → agent2(result1) → agent3(result2) → ...

// pkg/pipeline/fanout.go
func FanoutPipeline(ctx context.Context, agents []AgentBase, msg *Msg) ([]*Msg, error)
// 并行执行：所有 agent 同时处理同一 msg，收集结果

// pkg/pipeline/msghub.go
type MsgHub struct {
    participants []AgentBase
    announcement *Msg
}

func NewMsgHub(participants []AgentBase, announcement *Msg) *MsgHub
func (h *MsgHub) Add(agent AgentBase)
func (h *MsgHub) Remove(agent AgentBase)
func (h *MsgHub) Broadcast(ctx context.Context, msg *Msg) error
func (h *MsgHub) Close()
```

**设计要点**：
- `MsgHub` 管理 Agent 间的消息广播（pub/sub 模式）
- Agent 回复时自动广播给同 MsgHub 的其他 Agent（剥离 thinking blocks）
- `SequentialPipeline` 和 `FanoutPipeline` 是无状态的函数（对应 Python 的 async function）
- 用 Go 的 goroutine + errgroup 实现 Fanout 的并行

---

## 三、Go 依赖选择

| 功能 | Python 依赖 | Go 依赖 |
|------|------------|---------|
| HTTP Client | openai/anthropic/dashscope SDK | `net/http` + 手写 API 调用 |
| JSON | json5, json_repair | `encoding/json` |
| UUID | shortuuid | `github.com/google/uuid` |
| Redis | redis-py | `github.com/redis/go-redis/v9` |
| OpenTelemetry | opentelemetry-* | `go.opentelemetry.io/otel/*` |
| Streaming SSE | openai SDK | 手写 SSE 解析 |
| Token Counting | tiktoken | `github.com/pkoukk/tiktoken-go` |
| YAML/Config | pyproject.toml | 标准 flag/viper |

**极简依赖策略**：Model 层直接用 `net/http` 调用 REST API，不引入重量级 SDK。这比 Python 版更 Go-idiomatic。

---

## 四、分阶段实施计划

### Phase 1: 基础层（message + module + types）—— 2-3 天

**目标**：所有模块共用的基础类型就位

| 任务 | 文件 | 说明 |
|------|------|------|
| 1.1 项目初始化 | `go.mod`, `Makefile` | 模块名、lint、test 命令 |
| 1.2 UUID 工具 | `internal/utils/uuid.go` | short UUID 生成 |
| 1.3 ContentBlock 类型 | `pkg/message/block.go` | 所有 Block 类型定义 + 构造函数 |
| 1.4 Source 类型 | `pkg/message/source.go` | Base64Source, URLSource |
| 1.5 Msg 结构体 | `pkg/message/msg.go` | 消息定义 + 序列化 + 工具方法 |
| 1.6 StateModule | `pkg/module/state.go` | 状态基类（save/load state） |
| 1.7 单元测试 | `pkg/message/*_test.go` | 消息序列化/反序列化测试 |

**验收标准**：
```go
msg := message.NewMsg("assistant", "Hello!", "assistant")
msg.Content = append(msg.Content, message.NewTextBlock("Hi there"))
jsonBytes, _ := json.Marshal(msg)
// 可正确序列化/反序列化
```

### Phase 2: 模型层（model + formatter）—— 5-7 天

**目标**：能调用 LLM 并获得结构化响应

| 任务 | 文件 | 说明 |
|------|------|------|
| 2.1 ChatModelBase 接口 | `pkg/model/model.go` | 接口 + ChatResponse + CallOption |
| 2.2 OpenAI 兼容实现 | `pkg/model/openai.go` | 支持所有 OpenAI 兼容 API |
| 2.3 Anthropic 实现 | `pkg/model/anthropic.go` | Claude 原生 API |
| 2.4 Gemini 实现 | `pkg/model/gemini.go` | Google Gemini API |
| 2.5 SSE 流式解析 | `pkg/model/sse.go` | Server-Sent Events 解析器 |
| 2.6 FormatterBase 接口 | `pkg/formatter/formatter.go` | 接口定义 |
| 2.7 OpenAI Formatter | `pkg/formatter/openai.go` | Chat + MultiAgent 格式 |
| 2.8 Anthropic Formatter | `pkg/formatter/anthropic.go` | Chat + MultiAgent 格式 |
| 2.9 Gemini Formatter | `pkg/formatter/gemini.go` | Chat + MultiAgent 格式 |
| 2.10 集成测试 | `pkg/model/*_test.go` | Mock server 测试 |

**验收标准**：
```go
model := model.NewOpenAIChatModel("gpt-4o", "sk-xxx", "", true)
resp, err := model.Call(ctx, formattedMessages,
    model.WithTools(toolSchemas),
)
// resp 包含 LLM 的回复，可能含 tool_use
```

### Phase 3: 记忆与工具（memory + tool）—— 3-4 天

**目标**：Agent 可以记住对话并调用工具

| 任务 | 文件 | 说明 |
|------|------|------|
| 3.1 MemoryBase 接口 | `pkg/memory/memory.go` | 接口定义 |
| 3.2 InMemory 实现 | `pkg/memory/inmemory.go` | 内存存储 |
| 3.3 ToolSchema 类型 | `pkg/tool/schema.go` | JSON Schema 工具描述 |
| 3.4 Toolkit | `pkg/tool/toolkit.go` | 注册/查找/执行 |
| 3.5 反射注册 | `pkg/tool/reflect.go` | 从 Go 函数自动生成 Schema |
| 3.6 内置工具 | `pkg/tool/builtin.go` | 代码执行/Shell 命令 |
| 3.7 单元测试 | `pkg/tool/*_test.go` | 工具注册和执行测试 |

**验收标准**：
```go
tk := tool.NewToolkit()
tk.Register("get_weather", "Get weather", params, weatherFunc)
schemas := tk.GetSchemas()       // 生成 JSON Schema
result, err := tk.Execute(ctx, "get_weather", args)  // 执行工具
```

### Phase 4: Agent —— 3-4 天

**目标**：完整的 ReAct Agent 可运行

| 任务 | 文件 | 说明 |
|------|------|------|
| 4.1 AgentBase 接口 | `pkg/agent/agent.go` | 接口定义 |
| 4.2 ReActAgent | `pkg/agent/react.go` | ReAct 循环核心实现 |
| 4.3 UserAgent | `pkg/agent/user.go` | 终端用户输入 Agent |
| 4.4 Hooks 系统 | `pkg/agent/hooks.go` | 生命周期钩子 |
| 4.5 集成测试 | `pkg/agent/*_test.go` | Mock model 端到端测试 |
| 4.6 Hello 示例 | `examples/hello/main.go` | 最小可运行示例 |

**验收标准**：
```go
agent := agent.NewReActAgent(
    agent.WithName("Friday"),
    agent.WithModel(openAIModel),
    agent.WithMemory(memory.NewInMemoryMemory()),
    agent.WithFormatter(formatter.NewOpenAIChatFormatter()),
    agent.WithToolkit(tk),
)
resp, err := agent.Reply(ctx, userMsg)
// 完整的 ReAct 循环：LLM 推理 → 调用工具 → 再推理 → 返回结果
```

### Phase 5: Pipeline 编排 —— 2-3 天

**目标**：多 Agent 协作工作流

| 任务 | 文件 | 说明 |
|------|------|------|
| 5.1 SequentialPipeline | `pkg/pipeline/sequential.go` | 串行管线 |
| 5.2 FanoutPipeline | `pkg/pipeline/fanout.go` | 并行管线 |
| 5.3 MsgHub | `pkg/pipeline/msghub.go` | 消息广播中心 |
| 5.4 ChatRoom | `pkg/pipeline/chatroom.go` | 聊天室 |
| 5.5 多 Agent 示例 | `examples/multi_agent/main.go` | 多 Agent 对话示例 |

**验收标准**：
```go
// 串行对话
result, err := pipeline.SequentialPipeline(ctx, []AgentBase{agent1, agent2, agent3}, msg)

// MsgHub 广播
hub := pipeline.NewMsgHub(participants, announcement)
defer hub.Close()
hub.Broadcast(ctx, broadcastMsg)
```

### Phase 6: 分布式能力（session + tracing + a2a）—— 4-5 天

**目标**：生产级可观测性和持久化

| 任务 | 文件 | 说明 |
|------|------|------|
| 6.1 SessionBase 接口 | `pkg/session/session.go` | 接口定义 |
| 6.2 JSON Session | `pkg/session/json_session.go` | 文件持久化 |
| 6.3 Redis Session | `pkg/session/redis_session.go` | Redis 持久化 |
| 6.4 Tracing 初始化 | `pkg/tracing/tracing.go` | OTel setup |
| 6.5 Span 注入 | 各模块 | 在 Agent/Model/Memory 层注入 trace span |
| 6.6 A2A 协议 | `pkg/a2a/a2a.go` | Agent-to-Agent 通信 |
| 6.7 集成测试 | 分布式测试 | 端到端测试 |

**验收标准**：
```go
// Session 持久化
sess := session.NewJSONSession("/tmp/agent_session.json")
sess.Save(ctx, agent.Memory())
sess.Load(ctx, agent.Memory())

// Tracing
tp := tracing.SetupTracing("http://localhost:4317")
// 所有 Agent 调用自动产生 OTel span
```

---

## 五、总计时间估算

| Phase | 天数 | 累计 |
|-------|------|------|
| Phase 1: 基础层 | 2-3 天 | 3 天 |
| Phase 2: 模型层 | 5-7 天 | 10 天 |
| Phase 3: 记忆与工具 | 3-4 天 | 13 天 |
| Phase 4: Agent | 3-4 天 | 17 天 |
| Phase 5: Pipeline | 2-3 天 | 20 天 |
| Phase 6: 分布式 | 4-5 天 | 25 天 |
| **总计** | **~20-26 天** | |

---

## 六、关键设计决策（Python → Go 差异）

| 方面 | Python | Go |
|------|--------|-----|
| 异步 | `async/await` + `asyncio` | `context.Context` + goroutine + channel |
| 继承 | `class ReActAgent(AgentBase)` | 接口 + 组合 |
| 错误处理 | `try/except` | `error` 返回值 |
| 动态类型 | TypedDict / dict | `map[string]interface{}` + 类型构造函数 |
| Hook 系统 | OrderedDict + class/instance 区分 | `[]HookFunc` slice + 方法注册 |
| 流式输出 | `AsyncGenerator` | `<-chan ChatResponseChunk` |
| 工具 Schema | `docstring_parser` 自动提取 | struct tag + 反射 |
| 取消机制 | `asyncio.CancelledError` | `ctx.Done()` |
| 包管理 | pip + pyproject.toml | go modules |

---

## 七、建议的 Go 依赖最小集

```
require (
    github.com/google/uuid v1.6.0          // ID 生成
    github.com/redis/go-redis/v9           // Redis 记忆/会话
    go.opentelemetry.io/otel v1.33.0       // 链路追踪
    go.opentelemetry.io/otel/trace v1.33.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.33.0
    github.com/pkoukk/tiktoken-go          // Token 计数（可选）
)
```

---

## 八、风险与注意事项

1. **Python 动态性无法完全映射**：TypedDict 在 Go 中需要 `map[string]interface{}` + 构造函数，牺牲部分类型安全
2. **LLM API 格式差异大**：OpenAI/Anthropic/Gemini 的 tool_use 格式完全不同，Formatter 层是移植难点
3. **流式处理**：Go 的 SSE 解析需要手写，Python 有现成 SDK
4. **MCP 协议**：Python 版有 `mcp>=1.13` 库，Go 版可能需要自行实现 MCP client
5. **不建议移植的模块**：tune/tuner（RL 训练）、realtime（实时语音）、evaluate（评估框架）—— 这些与 Python 生态绑定太深
