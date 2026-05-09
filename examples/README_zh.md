# 示例

本目录包含演示如何使用 agentscope-go 的示例程序。

## 快速开始

```bash
# 设置 API Key
export OPENAI_API_KEY="your-api-key"

# 运行示例
go run ./examples/hello
```

## 示例列表

### 入门

| 示例 | 说明 |
|------|------|
| [hello](./hello) | 基础示例：创建 Agent 并进行单轮对话 |
| [react_agent](./react_agent) | ReAct Agent 搭配工具使用（计算器、天气查询） |
| [streaming](./streaming) | 通过 SSE 流式输出模型响应 |

### Agent

| 示例 | 说明 |
|------|------|
| [tool_usage](./tool_usage) | 工具注册：手动注册、反射注册（`RegisterFunc`）和内置工具 |
| [agent_skill](./agent_skill) | Agent 技能：注册 SKILL.md 目录、生成技能提示词、自定义模板 |
| [session_persistence](./session_persistence) | 使用 JSON 文件会话保存和恢复 Agent 记忆 |
| [deep_agent](./deep_agent) | DeepAgent：上下文压缩、结果卸载和子 Agent 委托 |
| [multi_model](./multi_model) | 同时使用 OpenAI、Anthropic、Gemini 模型提供商 |

### 工作流

| 示例 | 说明 |
|------|------|
| [multi_agent](./multi_agent) | 顺序流水线和 ChatRoom 多 Agent 对话 |
| [fanout_pipeline](./fanout_pipeline) | 使用 FanoutPipeline 进行并行代码审查（并发执行） |
| [msghub](./msghub) | MsgHub 多 Agent 广播与收集，支持动态参与者管理 |
| [debate](./debate) | 多 Agent 辩论：正方、反方和裁判 |

### 集成

| 示例 | 说明 |
|------|------|
| [a2a_agent](./a2a_agent) | Agent-to-Agent（A2A）协议：HTTP 服务端/客户端、服务发现和总线注册 |
| [tracing](./tracing) | OpenTelemetry 链路追踪：自定义 Span 和 Agent 钩子 |

## API Key 配置

大部分示例需要 API Key，请设置对应的环境变量：

```bash
# OpenAI（默认）
export OPENAI_API_KEY="sk-..."

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Google Gemini
export GEMINI_API_KEY="AIza..."
```

## 示例分类

### Agent 基础

- **hello** — 最简单的示例。创建 ReAct Agent，发送消息，打印响应。
- **react_agent** — 展示如何构建工具（天气、计算器）并附加到 ReAct Agent。Agent 会自主推理何时调用工具。
- **streaming** — 演示通过 `Stream` 方法逐 token 输出。

### 工具系统

- **tool_usage** — 涵盖三种注册方式：
  - 手动 `Register()` 配合显式参数 Schema
  - 基于 `RegisterFunc()` 反射注册，使用结构体标签
  - 内置工具（`RegisterShellTool`、`RegisterPrintTool`）
- **agent_skill** — 演示 Agent 技能系统：
  - 从 SKILL.md 目录注册技能（YAML Front Matter 格式）
  - 生成技能提示词用于 Agent 系统提示词（`GetAgentSkillPrompt`）
  - 移除技能、使用自定义模板
  - 无需 API Key（独立运行）

### 会话持久化

- **session_persistence** — 演示将对话历史保存到 JSON 文件，并在新 Agent 会话中恢复。

### 多模型

- **multi_model** — 演示使用不同模型提供商（OpenAI、Anthropic、Gemini）及其对应的 Formatter。未配置 API Key 的提供商会自动跳过。
- **deep_agent** — DeepAgent 用于长时间运行的任务，支持自动上下文压缩（LLM 摘要或截断）、大结果卸载到磁盘、以及子 Agent 委托。

### 流水线

- **multi_agent** — 顺序流水线（Agent 依次执行）和 ChatRoom（Agent 观察彼此消息）。
- **fanout_pipeline** — 并行执行：多个 Agent 同时处理相同输入。
- **msghub** — MsgHub 模式：向所有参与者广播消息、收集响应、动态添加/移除 Agent。
- **debate** — 完整的多 Agent 工作流：两个 Agent 辩论，第三个 Agent 评判。

### A2A 协议

- **a2a_agent** — 通过 `A2AServer` 将本地 Agent 暴露为 HTTP 服务。使用 `A2AClient` 发现和调用远程 Agent。通过 `A2ABus` 管理 Agent 注册。

### 可观测性

- **tracing** — 配置 OpenTelemetry 链路追踪，使用 OTLP gRPC 导出器。围绕 Agent 操作创建 Span，使用钩子（`PreReply`/`PostReply`）实现自动埋点。
