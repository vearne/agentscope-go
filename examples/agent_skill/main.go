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
	"github.com/vearne/agentscope-go/pkg/tool"
)

func main() {
	ctx := context.Background()

	m := model.NewOpenAIChatModel("gpt-4o", os.Getenv("OPENAI_API_KEY"), "", false)
	f := formatter.NewOpenAIChatFormatter()

	tk := tool.NewToolkit()
	tool.RegisterShellTool(tk)

	if err := tk.RegisterAgentSkill("./weather_skill"); err != nil {
		log.Fatalf("register skill: %v", err)
	}

	skillPrompt := tk.GetAgentSkillPrompt()
	fmt.Println("=== Skill Prompt ===")
	fmt.Println(skillPrompt)
	fmt.Println("====================")

	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActToolkit(tk),
		agent.WithReActSystemPrompt(skillPrompt),
	)

	msg := message.NewMsg("user", "What's the weather like in Beijing?", "user")
	resp, err := ag.Reply(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
