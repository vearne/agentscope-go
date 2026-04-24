// This example demonstrates the FanoutPipeline for parallel agent execution.
// Multiple agents process the same input concurrently and all results are collected.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/pipeline"
)

func main() {
	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", false)
	f := formatter.NewOpenAIChatFormatter()

	reviewer1 := agent.NewReActAgent(
		agent.WithReActName("security-reviewer"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a security expert. Review code for security vulnerabilities."),
	)

	reviewer2 := agent.NewReActAgent(
		agent.WithReActName("performance-reviewer"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a performance expert. Review code for performance issues."),
	)

	reviewer3 := agent.NewReActAgent(
		agent.WithReActName("style-reviewer"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a code style expert. Review code for style and readability."),
	)

	msg := agent.NewUserMsg("user", "Review this code snippet: func add(a, b int) int { return a + b }")

	fmt.Println("=== FanoutPipeline: Parallel Code Review ===")
	results, err := pipeline.FanoutPipeline(
		context.Background(),
		[]agent.AgentBase{reviewer1, reviewer2, reviewer3},
		msg,
	)
	if err != nil {
		log.Fatal(err)
	}

	for i, resp := range results {
		fmt.Printf("\n--- Reviewer %d: %s ---\n", i+1, resp.Name)
		fmt.Printf("%s\n", resp.GetTextContent())
	}
}
