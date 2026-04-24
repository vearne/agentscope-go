package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/model"
)

func main() {
	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", false)
	f := formatter.NewOpenAIChatFormatter()

	a := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
	)

	msg := agent.NewUserMsg("user", "Hello! What can you do?")
	resp, err := a.Reply(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("[%s] %s\n", resp.Name, resp.GetTextContent())
}
