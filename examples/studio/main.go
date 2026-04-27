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
	"github.com/vearne/agentscope-go/pkg/studio"
)

func main() {
	ctx := context.Background()

	if err := studio.Init(
		studio.WithURL("http://localhost:3000"),
		studio.WithProject("hello-studio"),
	); err != nil {
		log.Printf("Warning: studio init failed (studio may not be running): %v", err)
		log.Println("Continuing without studio integration...")
	}
	defer studio.Shutdown(ctx)

	m := model.NewOpenAIChatModel("gpt-4o", os.Getenv("OPENAI_API_KEY"), "", false)
	f := formatter.NewOpenAIChatFormatter()

	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
	)

	msg := message.NewMsg("user", "Hello! What can you do?", "user")
	resp, err := ag.Reply(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
