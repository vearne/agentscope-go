// This example demonstrates Agent-to-Agent (A2A) protocol for inter-agent communication.
// One agent runs as an HTTP server, another discovers and calls it as a client.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vearne/agentscope-go/pkg/a2a"
	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
)

func main() {
	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", false)
	f := formatter.NewOpenAIChatFormatter()

	localAgent := agent.NewReActAgent(
		agent.WithReActName("math-expert"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a math expert. Solve math problems concisely."),
	)

	card := a2a.AgentCard{
		Name:        "math-expert",
		ID:          "math-expert-001",
		Description: "An agent that solves math problems",
		Endpoint:    "http://localhost:18080",
		Capabilities: []string{"math", "reasoning"},
	}

	server := a2a.NewA2AServer(localAgent, card)
	if err := server.Start(":18080"); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = server.Stop(context.Background()) }()

	<-server.Ready()
	fmt.Println("A2A server started on :18080")

	client := a2a.NewA2AClient("http://localhost:18080")
	if err := client.Discover(context.Background()); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Discovered agent: %s (ID: %s)\n", client.Name(), client.ID())

	msg := agent.NewUserMsg("user", "What is 15 * 37 + 42?")
	resp, err := client.Reply(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[%s] %s\n", resp.Name, resp.GetTextContent())

	bus := a2a.NewA2ABus()
	bus.Register(card)
	fmt.Printf("\nRegistered agents in bus: %d\n", len(bus.List()))
	if c, ok := bus.Get("math-expert-001"); ok {
		fmt.Printf("  - %s: %s\n", c.Name, c.Description)
	}
	bus.Deregister("math-expert-001")
	fmt.Printf("After deregister: %d agents\n", len(bus.List()))

	time.Sleep(100 * time.Millisecond)
}
