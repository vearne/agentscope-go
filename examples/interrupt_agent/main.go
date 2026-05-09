// This example demonstrates how to interrupt a running ReAct agent.
//
// The agent is configured with a slow shell tool.  After 3 seconds the main
// goroutine calls agent.Interrupt(), which cancels the in-flight model call
// and returns a graceful "interrupted" response with metadata.
//
// Run:
//
//	export OPENAI_API_KEY="your-api-key"
//	go run ./examples/interrupt_agent
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
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/studio"
	"github.com/vearne/agentscope-go/pkg/tool"
)

func main() {
	ctx := context.Background()

	if err := studio.Init(
		studio.WithURL("http://localhost:3000"),
		studio.WithProject("interrupt-studio"),
	); err != nil {
		log.Printf("Warning: studio init failed (studio may not be running): %v", err)
		log.Println("Continuing without studio integration...")
	}
	defer func() { _ = studio.Shutdown(ctx) }()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set OPENAI_API_KEY environment variable")
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	modelName := os.Getenv("OPENAI_MODEL")
	m := model.NewOpenAIChatModel(modelName, apiKey, baseURL, false)
	f := formatter.NewOpenAIChatFormatter()

	tk := tool.NewToolkit()
	if err := tool.RegisterShellTool(tk); err != nil {
		log.Fatal(err)
	}

	ag := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActToolkit(tk),
		agent.WithReActMaxIters(10),
		agent.WithReActSystemPrompt(
			"You are a helpful assistant. Use the execute_shell tool to run commands when needed.",
		),
	)

	msg := agent.NewUserMsg("user",
		"Please run a long-running command: sleep 30 && echo done. "+
			"Wait for it to finish and report the result.")

	fmt.Println("=== Starting agent (will be interrupted in 3s) ===")

	type result struct {
		resp *message.Msg
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := ag.Reply(context.Background(), msg)
		done <- result{resp, err}
	}()

	time.Sleep(3 * time.Second)
	fmt.Println("=== Interrupting agent... ===")
	ag.Interrupt()

	r := <-done
	if r.err != nil {
		log.Fatalf("Reply returned error: %v", r.err)
	}

	fmt.Println()
	fmt.Println("=== Response after interrupt ===")
	fmt.Printf("[%s] %s\n", r.resp.Name, r.resp.GetTextContent())

	if r.resp.Metadata != nil {
		if isInt, ok := r.resp.Metadata["_is_interrupted"].(bool); ok && isInt {
			fmt.Println("\nThe agent was successfully interrupted.")
		}
	}

	fmt.Printf("\nMemory size: %d messages\n", ag.Memory().Size())

	// Demonstrate that the agent can be reused after interrupt.
	fmt.Println("\n=== Sending a follow-up message (agent is reusable) ===")

	followUp := agent.NewUserMsg("user", "What is 2 + 2?")
	resp2, err := ag.Reply(context.Background(), followUp)
	if err != nil {
		log.Fatalf("Follow-up reply failed: %v", err)
	}
	fmt.Printf("[%s] %s\n", resp2.Name, resp2.GetTextContent())
}
