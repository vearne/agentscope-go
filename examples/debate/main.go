// This example demonstrates a multi-agent debate workflow.
// A topic is proposed and multiple agents take turns arguing for or against it.
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

	proponent := agent.NewReActAgent(
		agent.WithReActName("proponent"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt(
			"You are arguing IN FAVOR of the topic. Make strong, logical arguments. Be concise (2-3 sentences).",
		),
	)

	opponent := agent.NewReActAgent(
		agent.WithReActName("opponent"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt(
			"You are arguing AGAINST the topic. Make strong counter-arguments. Be concise (2-3 sentences).",
		),
	)

	judge := agent.NewReActAgent(
		agent.WithReActName("judge"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActSystemPrompt(
			"You are a neutral judge. After hearing both sides, summarize and declare a winner.",
		),
	)

	topic := "AI will replace most software engineering jobs within 10 years."
	fmt.Printf("=== Debate Topic: %s ===\n\n", topic)

	room := pipeline.NewChatRoom(
		[]agent.AgentBase{proponent, opponent},
		message.NewMsg("moderator", fmt.Sprintf("Debate topic: %s", topic), "system"),
		3,
	)

	msg := agent.NewUserMsg("moderator", fmt.Sprintf("Let's debate: %s", topic))
	history, err := room.Run(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}

	for _, h := range history {
		fmt.Printf("[%s]: %s\n\n", h.Name, h.GetTextContent())
	}

	fmt.Println("=== Judge's Verdict ===")
	summary := ""
	for _, h := range history {
		summary += fmt.Sprintf("[%s]: %s\n", h.Name, h.GetTextContent())
	}
	verdict, err := judge.Reply(context.Background(), agent.NewUserMsg("moderator",
		fmt.Sprintf("Here is the debate summary:\n%s\n\nPlease give your verdict.", summary),
	))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[judge]: %s\n", verdict.GetTextContent())
}
