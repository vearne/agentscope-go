// Example: Model Configuration
// This example demonstrates how to set model parameters at initialization time.
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

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	fmt.Println("=== Example 1: Basic Temperature Configuration ===")
	example1(ctx, apiKey)

	fmt.Println("\n=== Example 2: Multiple Parameters ===")
	example2(ctx, apiKey)

	fmt.Println("\n=== Example 3: JSON Mode ===")
	example3(ctx, apiKey)

	fmt.Println("\n=== Example 4: Dynamic Override ===")
	example4(ctx, apiKey)
}

func example1(ctx context.Context, apiKey string) {
	// Create model with temperature configuration
	m := model.NewOpenAIChatModel(
		"gpt-4o",
		apiKey,
		"",
		false,
		model.WithTemperature(0.3), // Low temperature for more deterministic output
	)

	f := formatter.NewOpenAIFormatter()
	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a helpful assistant."),
	)

	msg := message.NewMsg("user", "What is 2+2? Give me a short answer.", "user")
	resp, err := ag.Reply(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", resp.GetTextContent())
}

func example2(ctx context.Context, apiKey string) {
	// Create model with multiple parameters
	m := model.NewOpenAIChatModel(
		"gpt-4o",
		apiKey,
		"",
		false,
		model.WithTemperature(0.7),
		model.WithTopP(0.9),
		model.WithMaxTokens(1024),
		model.WithPresencePenalty(0.1),
		model.WithFrequencyPenalty(0.1),
	)

	f := formatter.NewOpenAIFormatter()
	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a creative writer."),
	)

	msg := message.NewMsg("user", "Write a short haiku about coding.", "user")
	resp, err := ag.Reply(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", resp.GetTextContent())
}

func example3(ctx context.Context, apiKey string) {
	// Create model with JSON mode
	m := model.NewOpenAIChatModel(
		"gpt-4o",
		apiKey,
		"",
		false,
		model.WithResponseFormat("json_object"),
		model.WithTemperature(0.0), // Zero temperature for JSON mode
	)

	f := formatter.NewOpenAIFormatter()
	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a data generator. Always respond with valid JSON."),
	)

	msg := message.NewMsg("user", "Generate a JSON object with name and age fields.", "user")
	resp, err := ag.Reply(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", resp.GetTextContent())
}

func example4(ctx context.Context, apiKey string) {
	// Create model with default configuration
	m := model.NewOpenAIChatModel(
		"gpt-4o",
		apiKey,
		"",
		false,
		model.WithTemperature(0.5), // default temperature
	)

	f := formatter.NewOpenAIFormatter()
	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a helpful assistant."),
	)

	// First call uses default temperature (0.5)
	msg1 := message.NewMsg("user", "What's your favorite color? (short answer)", "user")
	resp1, err := ag.Reply(ctx, msg1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Default temp (0.5): %s\n", resp1.GetTextContent())

	// Second call overrides temperature
	msg2 := message.NewMsg("user", "What's your favorite food? (short answer)", "user")

	// Note: For demonstration, we're calling the model directly
	// In a real scenario, you might need to extend the Agent interface
	// to support CallOption or use a different approach
	_ = msg2
	_ = ag

	fmt.Println("Note: To override temperature per-call, you would need to:")
	fmt.Println("1. Call the model directly with CallOption")
	fmt.Println("2. Or extend the Agent interface to support configuration overrides")
}
