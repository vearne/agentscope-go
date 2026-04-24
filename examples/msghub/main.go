// This example demonstrates the MsgHub pattern for multi-agent coordination.
// MsgHub supports broadcast (send to all) and gather (collect from all) operations,
// as well as dynamic participant management.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/pipeline"
)

func main() {
	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", false)
	f := formatter.NewOpenAIChatFormatter()

	expertA := agent.NewReActAgent(
		agent.WithReActName("physicist"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a physicist. Answer questions from a physics perspective."),
	)

	expertB := agent.NewReActAgent(
		agent.WithReActName("philosopher"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a philosopher. Answer questions from a philosophical perspective."),
	)

	hub := pipeline.NewMsgHub(
		[]agent.AgentBase{expertA, expertB},
		message.NewMsg("system", "You are participating in a knowledge-sharing round table.", "system"),
	)

	fmt.Println("=== Round 1: Broadcast and Gather ===")
	announcement := message.NewMsg("host", "What is time?", "user")
	if err := hub.Broadcast(context.Background(), announcement); err != nil {
		log.Fatal(err)
	}

	responses, err := hub.Gather(context.Background(), announcement)
	if err != nil {
		log.Fatal(err)
	}
	for _, resp := range responses {
		fmt.Printf("[%s] %s\n\n", resp.Name, resp.GetTextContent())
	}

	fmt.Println("=== Dynamic Participant Management ===")
	expertC := agent.NewReActAgent(
		agent.WithReActName("biologist"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a biologist. Answer from a biology perspective."),
	)
	hub.Add(expertC)
	fmt.Printf("Added biologist. Participants: %d\n", len(hub.Participants()))

	hub.Remove(expertA)
	fmt.Printf("Removed physicist. Participants: %d\n", len(hub.Participants()))

	fmt.Println("\n=== Round 2: Gather with updated participants ===")
	responses, err = hub.Gather(context.Background(), message.NewMsg("host", "What is consciousness?", "user"))
	if err != nil {
		log.Fatal(err)
	}
	for _, resp := range responses {
		fmt.Printf("[%s] %s\n\n", resp.Name, resp.GetTextContent())
	}
}
