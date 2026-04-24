// This example demonstrates how to use different model providers
// (OpenAI, Anthropic, Gemini) with their respective formatters.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
)

func main() {
	question := "Explain the concept of goroutines in 2 sentences."

	agents := buildAgents()
	if len(agents) == 0 {
		log.Fatal("No API keys set. Set at least one of: OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY")
	}

	msg := agent.NewUserMsg("user", question)
	for _, a := range agents {
		fmt.Printf("=== %s ===\n", a.name)
		resp, err := a.agent.Reply(context.Background(), msg)
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}
		fmt.Printf("  %s\n\n", resp.GetTextContent())
	}
}

type namedAgent struct {
	name  string
	agent *agent.ReActAgent
}

func buildAgents() []namedAgent {
	var agents []namedAgent

	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		m := model.NewOpenAIChatModel("gpt-4o", key, "", false)
		agents = append(agents, namedAgent{
			name: "OpenAI (gpt-4o)",
			agent: agent.NewReActAgent(
				agent.WithReActName("openai-assistant"),
				agent.WithReActModel(m),
				agent.WithReActFormatter(formatter.NewOpenAIChatFormatter()),
				agent.WithReActMemory(memory.NewInMemoryMemory()),
			),
		})
	}

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		m := model.NewAnthropicChatModel("claude-sonnet-4-20250514", key, "", false)
		agents = append(agents, namedAgent{
			name: "Anthropic (claude-sonnet-4)",
			agent: agent.NewReActAgent(
				agent.WithReActName("anthropic-assistant"),
				agent.WithReActModel(m),
				agent.WithReActFormatter(formatter.NewAnthropicChatFormatter()),
				agent.WithReActMemory(memory.NewInMemoryMemory()),
			),
		})
	}

	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		m := model.NewGeminiChatModel("gemini-2.0-flash", key, "", false)
		agents = append(agents, namedAgent{
			name: "Gemini (gemini-2.0-flash)",
			agent: agent.NewReActAgent(
				agent.WithReActName("gemini-assistant"),
				agent.WithReActModel(m),
				agent.WithReActFormatter(formatter.NewGeminiChatFormatter()),
				agent.WithReActMemory(memory.NewInMemoryMemory()),
			),
		})
	}

	return agents
}
