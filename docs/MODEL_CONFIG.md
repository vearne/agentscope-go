# Model Configuration Guide

## Overview

agentscope-go now supports setting model parameters at initialization time, similar to the Python version of AgentScope. You can configure parameters like temperature, top_p, max_tokens, and more when creating a model instance.

## Supported Parameters

### Common Parameters (All Models)

| Parameter | Type | Description | OpenAI | Anthropic | Gemini |
|-----------|------|-------------|--------|-----------|--------|
| `Temperature` | `float64` | Sampling temperature (0-2 for OpenAI, 0-1 for Anthropic) | ✅ | ✅ | ✅ |
| `TopP` | `float64` | Nucleus sampling (0-1) | ✅ | ✅ | ✅ |
| `MaxTokens` | `int` | Maximum tokens to generate | ✅ | ✅ | ✅ |
| `Stop` | `[]string` | Stop sequences | ✅ | ✅ | ✅ |

### OpenAI-Specific Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `PresencePenalty` | `float64` | Presence penalty (-2.0 to 2.0) |
| `FrequencyPenalty` | `float64` | Frequency penalty (-2.0 to 2.0) |
| `Seed` | `int` | Random seed for reproducible results |
| `ResponseFormat` | `ResponseFormat` | Response format (text or json_object) |
| `User` | `string` | User identifier |

### Anthropic-Specific Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `TopK` | `int` | Top-K sampling |

### Gemini-Specific Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `TopK` | `int` | Top-K sampling |

## Usage Examples

### OpenAI Model

```go
import (
    "github.com/vearne/agentscope-go/pkg/model"
)

// Basic configuration with temperature
m := model.NewOpenAIChatModel(
    "gpt-4o",
    apiKey,
    baseURL,
    false, // stream
    model.WithTemperature(0.7),
)

// Multiple parameters
m := model.NewOpenAIChatModel(
    "gpt-4o",
    apiKey,
    baseURL,
    false,
    model.WithTemperature(0.7),
    model.WithTopP(0.9),
    model.WithMaxTokens(2048),
    model.WithPresencePenalty(0.1),
    model.WithFrequencyPenalty(0.1),
)

// JSON mode
m := model.NewOpenAIChatModel(
    "gpt-4o",
    apiKey,
    baseURL,
    false,
    model.WithResponseFormat("json_object"),
    model.WithTemperature(0.0),
)

// With stop sequences
m := model.NewOpenAIChatModel(
    "gpt-4o",
    apiKey,
    baseURL,
    false,
    model.WithStop([]string{"END", "STOP"}),
)

// With user identifier
m := model.NewOpenAIChatModel(
    "gpt-4o",
    apiKey,
    baseURL,
    false,
    model.WithUser("user-123"),
    model.WithSeed(42), // for reproducible results
)
```

### Anthropic Model

```go
// Basic configuration
m := model.NewAnthropicChatModel(
    "claude-3-sonnet-20240229",
    apiKey,
    baseURL,
    false,
    model.WithAnthropicTemperature(0.7),
)

// With TopK (Anthropic-specific)
m := model.NewAnthropicChatModel(
    "claude-3-sonnet-20240229",
    apiKey,
    baseURL,
    false,
    model.WithAnthropicTemperature(0.5),
    model.WithAnthropicTopP(0.9),
    model.WithAnthropicTopK(40),
    model.WithAnthropicMaxTokens(4096),
)
```

### Gemini Model

```go
// Basic configuration
m := model.NewGeminiChatModel(
    "gemini-pro",
    apiKey,
    baseURL,
    false,
    model.WithGeminiTemperature(0.7),
)

// With TopK (Gemini-specific)
m := model.NewGeminiChatModel(
    "gemini-pro",
    apiKey,
    baseURL,
    false,
    model.WithGeminiTemperature(0.5),
    model.WithGeminiTopP(0.9),
    model.WithGeminiTopK(40),
    model.WithGeminiMaxTokens(2048),
)
```

## Dynamic Overrides with CallOption

You can still override the default configuration at call time using `CallOption`:

```go
// Set default configuration at model creation
m := model.NewOpenAIChatModel(
    "gpt-4o",
    apiKey,
    baseURL,
    false,
    model.WithTemperature(0.5), // default temperature
    model.WithMaxTokens(4096),   // default max tokens
)

// Most calls use default configuration
resp1, err := m.Call(ctx, messages1, model.CallOption{})

// Special call overrides temperature
highTemp := 0.9
resp2, err := m.Call(ctx, messages2, model.CallOption{
    Temperature: &highTemp, // override default
})

// Special call overrides max tokens
maxTokens := 8192
resp3, err := m.Call(ctx, messages3, model.CallOption{
    MaxTokens: &maxTokens,
})
```

## Priority Rules

When both default configuration and `CallOption` are provided, `CallOption` takes precedence:

1. **CallOption value** → Used if not nil
2. **Default configuration** → Used if CallOption is nil
3. **API default** → Used if both are nil

## Complete Example with Agent

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/vearne/agentscope-go/pkg/agent"
    "github.com/vearne/agentscope-go/pkg/formatter"
    "github.com/vearne/agentscope-go/pkg/memory"
    "github.com/vearne/agentscope-go/pkg/model"
    "github.com/vearne/agentscope-go/pkg/message"
)

func main() {
    ctx := context.Background()

    // Create model with default configuration
    m := model.NewOpenAIChatModel(
        "gpt-4o",
        os.Getenv("OPENAI_API_KEY"),
        "",
        false,
        model.WithTemperature(0.7),
        model.WithTopP(0.9),
        model.WithMaxTokens(2048),
    )

    f := formatter.NewOpenAIFormatter()

    // Create agent
    ag := agent.NewReActAgent(
        agent.WithReActName("assistant"),
        agent.WithReActModel(m),
        agent.WithReActFormatter(f),
        agent.WithReActMemory(memory.NewInMemoryMemory()),
        agent.WithReActSystemPrompt("You are a helpful assistant."),
    )

    // Send message
    msg := message.NewMsg("user", "Hello! What can you do?", "user")
    resp, err := ag.Reply(ctx, msg)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Response: %s", resp.GetTextContent())
}
```

## Migration Guide

### From Old API (No Configuration)

```go
// Old way - no default configuration
m := model.NewOpenAIChatModel("gpt-4o", apiKey, "", false)
```

### To New API (With Configuration)

```go
// New way - with default configuration
m := model.NewOpenAIChatModel(
    "gpt-4o",
    apiKey,
    "",
    false,
    model.WithTemperature(0.7),
)
```

**Note**: The new API is backward compatible. If you don't provide any configuration options, the model will use API defaults.

## Testing

Run the tests to verify the configuration functionality:

```bash
go test ./pkg/model/... -v -run "TestModelConfig|TestAnthropicModel|TestGeminiModel"
```

## Comparison with Python AgentScope

### Python AgentScope

```python
model_configs = [
    {
        "model_type": "openai",
        "config_name": "gpt-3.5-turbo",
        "model_name": "gpt-3.5-turbo",
        "generate_args": {
            "temperature": 0.5,
            "max_tokens": 2048,
        },
    },
]
```

### agentscope-go (New)

```go
m := model.NewOpenAIChatModel(
    "gpt-3.5-turbo",
    apiKey,
    "",
    false,
    model.WithTemperature(0.5),
    model.WithMaxTokens(2048),
)
```

The Go version provides the same capability with type-safe configuration options.
