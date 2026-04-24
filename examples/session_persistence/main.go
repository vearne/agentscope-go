// This example demonstrates how to persist and restore agent memory
// using JSON file-based sessions.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/session"
)

const sessionFile = "/tmp/agentscope_session.json"

func main() {
	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", false)
	f := formatter.NewOpenAIChatFormatter()

	fmt.Println("=== Session 1: Build conversation ===")
	mem1 := memory.NewInMemoryMemory()
	a1 := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(mem1),
	)

	msg1 := agent.NewUserMsg("user", "My name is Alice and I love Go programming.")
	resp1, err := a1.Reply(context.Background(), msg1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[%s] %s\n", resp1.Name, resp1.GetTextContent())

	sess := session.NewJSONSession(sessionFile)
	if err := sess.Save(context.Background(), mem1); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session saved to %s (%d messages)\n\n", sessionFile, mem1.Size())

	fmt.Println("=== Session 2: Restore conversation ===")
	mem2 := memory.NewInMemoryMemory()
	if err := sess.Load(context.Background(), mem2); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Restored %d messages from session file\n", mem2.Size())

	for i, msg := range mem2.GetMessages() {
		fmt.Printf("  [%d] %s (%s): %s\n", i+1, msg.Name, msg.Role, msg.GetTextContent())
	}

	a2 := agent.NewReActAgent(
		agent.WithReActName("assistant"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(mem2),
	)

	msg2 := agent.NewUserMsg("user", "What is my name and what do I love?")
	resp2, err := a2.Reply(context.Background(), msg2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n[%s] %s\n", resp2.Name, resp2.GetTextContent())

	os.Remove(sessionFile)
}
