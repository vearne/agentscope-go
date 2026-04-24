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

	writer := agent.NewReActAgent(
		agent.WithReActName("writer"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a creative writer. Write short stories."),
	)

	reviewer := agent.NewReActAgent(
		agent.WithReActName("reviewer"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt("You are a literary critic. Provide brief feedback."),
	)

	msg := agent.NewUserMsg("user", "Write a story about a robot learning to paint.")

	fmt.Println("=== Sequential Pipeline ===")
	result, err := pipeline.SequentialPipeline(
		context.Background(),
		[]agent.AgentBase{writer, reviewer},
		msg,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[%s]: %s\n\n", result.Name, result.GetTextContent())

	fmt.Println("=== ChatRoom ===")
	room := pipeline.NewChatRoom(
		[]agent.AgentBase{writer, reviewer},
		message.NewMsg("system", "You are in a writing workshop.", "system"),
		2,
	)
	history, err := room.Run(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}
	for _, h := range history {
		fmt.Printf("[%s]: %s\n", h.Name, h.GetTextContent())
	}
}
