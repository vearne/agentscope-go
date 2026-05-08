// This example demonstrates the DeepAgent — an agent designed for long-running,
// multi-step tasks that may accumulate large conversation histories.
//
// DeepAgent provides three key capabilities beyond the standard ReActAgent:
//   - Context Compression: When conversation history exceeds a token threshold,
//     old messages are compressed (via LLM summarization or truncation) to stay
//     within context limits.
//   - Result Offloading: Large tool results are automatically saved to disk
//     and replaced with a reference, keeping the in-memory context lean.
//   - Subagent Delegation: The agent can delegate complex subtasks to independent
//     subagents, preventing context bloat from multi-step work.
//
// Run:
//
//	export OPENAI_API_KEY="your-api-key"
//	go run ./examples/deep_agent
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set OPENAI_API_KEY environment variable")
	}

	ctx := context.Background()

	m := model.NewOpenAIChatModel("gpt-4o", apiKey, "", false)
	f := formatter.NewOpenAIChatFormatter()

	tk := tool.NewToolkit()
	if err := registerTools(tk); err != nil {
		log.Fatal(err)
	}

	subFactory := agent.NewSubagentFactory(m, f, tk)

	deepAg := agent.NewDeepAgent(
		agent.WithDeepName("researcher"),
		agent.WithDeepModel(m),
		agent.WithDeepFormatter(f),
		agent.WithDeepToolkit(tk),
		agent.WithDeepSubagentFactory(subFactory),
		agent.WithDeepSystemPrompt(
			"You are a deep research assistant. You can use tools directly or delegate "+
				"complex subtasks to subagents via the delegate_task tool. Break large tasks "+
				"into smaller steps and delegate when appropriate."),
		agent.WithDeepMaxIters(20),
		agent.WithDeepMaxContextTokens(128000),
		agent.WithDeepOffloadThreshold(8000),
		agent.WithDeepOffloadDir(".deepagent/offload"),
	)
	msg := agent.NewUserMsg("user",
		"Research the following: What are the key differences between Go and Rust "+
			"in terms of memory management, concurrency model, and error handling? "+
			"For each topic, provide a detailed comparison. "+
			"If any subtopic is complex, delegate it to a subagent for deeper research.")

	resp, err := deepAg.Reply(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== DeepAgent Response ===")
	fmt.Printf("[%s] %s\n", resp.Name, resp.GetTextContent())
	fmt.Println()
	fmt.Printf("Memory size: %d messages\n", deepAg.Memory().Size())
}

func registerTools(tk *tool.Toolkit) error {
	if err := tool.RegisterShellTool(tk); err != nil {
		return err
	}

	if err := tool.RegisterPrintTool(tk); err != nil {
		return err
	}
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "The programming topic to look up",
			},
			"language": map[string]interface{}{
				"type":        "string",
				"description": "The programming language to search for",
			},
		},
		"required": []string{"topic", "language"},
	}
	return tk.Register("lookup_docs", "Look up documentation for a programming topic", params, lookupDocs)
}

func lookupDocs(_ context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
	topic, _ := args["topic"].(string)
	language, _ := args["language"].(string)

	docs := map[string]map[string]string{
		"Go": {
			"memory management": "Go uses garbage collection (GC) with a concurrent tri-color mark-and-sweep collector. Memory is automatically managed — developers allocate with make/new and the GC reclaims unreachable objects. No manual memory management required.",
			"concurrency":      "Go uses goroutines (lightweight green threads managed by the Go runtime) and channels (typed conduits for communicating between goroutines). The philosophy is: 'Do not communicate by sharing memory; instead, share memory by communicating.'",
			"error handling":   "Go uses explicit error return values: functions return (result, error). Errors are checked with if err != nil. Go also supports panic/recover for exceptional cases but idiomatic code uses error values.",
		},
		"Rust": {
			"memory management": "Rust uses ownership system with borrow checker at compile time. No garbage collector. Memory is managed through ownership rules: each value has one owner, and when the owner goes out of scope, the value is dropped.",
			"concurrency":      "Rust guarantees memory safety in concurrent code through ownership and type system (Send, Sync traits). Uses OS threads with std::thread, async/await with tokio, and message passing via channels (similar concept to Go channels but with different guarantees).",
			"error handling":   "Rust uses Result<T, E> enum for recoverable errors and Option<T> for nullable values. Uses the ? operator for error propagation. Unrecoverable errors use panic!/panic!, similar to Go's panic but with compile-time guarantees.",
		},
	}

	if langDocs, ok := docs[language]; ok {
		if doc, ok := langDocs[topic]; ok {
			return &tool.ToolResponse{Content: doc}, nil
		}
	}

	return &tool.ToolResponse{
		Content: fmt.Sprintf("No documentation found for '%s' in %s.", topic, language),
	}, nil
}
