// This example demonstrates streaming model responses using SSE.
// Tokens are printed progressively as they arrive from the model.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)

func main() {
	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", true)
	f := formatter.NewOpenAIChatFormatter()

	mem := memory.NewInMemoryMemory()
	mem.Add(context.Background(),
		message.NewMsg("system", "You are a helpful assistant.", "system"),
		message.NewMsg("user", "Write a short poem about programming in Go.", "user"),
	)

	formatted, err := f.Format(mem.GetMessages())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Streaming Response ===")
	ch, err := m.Stream(context.Background(), formatted)
	if err != nil {
		log.Fatal(err)
	}

	var lastText string
	for resp := range ch {
		text := resp.GetTextContent()
		if text != "" && text != lastText {
			fmt.Print(text[len(lastText):])
			lastText = text
		}
	}
	fmt.Println()

	if lastText == "" {
		fmt.Println("(no content received)")
	}
}
