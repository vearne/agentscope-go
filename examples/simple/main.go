package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/studio"
	"github.com/vearne/agentscope-go/pkg/tool"
)

func main() {
	ctx := context.Background()

	if err := studio.Init(
		studio.WithURL("http://localhost:3000"),
		studio.WithProject("hello-date"),
	); err != nil {
		log.Printf("Warning: studio init failed (studio may not be running): %v", err)
		log.Println("Continuing without studio integration...")
	}
	defer func() { _ = studio.Shutdown(ctx) }()

	tk := tool.NewToolkit()

	// --- Built-in tools ---
	if err := tool.RegisterPrintTool(tk); err != nil {
		log.Fatal(err)
	}
	if err := tool.RegisterShellTool(tk); err != nil {
		log.Fatal(err)
	}

	modelName := os.Getenv("OPENAI_MODEL")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	apiKey := os.Getenv("OPENAI_API_KEY")

	m := model.NewOpenAIChatModel(modelName, apiKey, baseURL, false)
	f := formatter.NewOpenAIChatFormatter()
	a := agent.NewReActAgent(
		agent.WithReActName("Friday"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActToolkit(tk),
		agent.WithReActMaxIters(5),
		agent.WithReActSystemPrompt(
			"You are a helpful assistant named Friday. Use tools when needed to answer questions accurately.",
		),
	)

	msg := agent.NewUserMsg("user", "当前的时间是？后天是几号，星期几？")
	resp, err := a.Reply(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("[%s] %s\n", resp.Name, resp.GetTextContent())
	time.Sleep(time.Second)
}
