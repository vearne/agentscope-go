// This example demonstrates various ways to register and use tools:
// manual registration, reflection-based registration, and built-in tools.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/tool"
)

func main() {
	tk := tool.NewToolkit()

	// --- Manual registration ---
	err := tk.Register(
		"greet",
		"Generate a greeting message for a given name",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name to greet",
				},
			},
			"required": []string{"name"},
		},
		greetFunc,
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- Reflection-based registration ---
	err = tk.RegisterFunc(searchFunc, tool.RegisterOption{
		Name:        "search",
		Description: "Search the web for information",
	})
	if err != nil {
		log.Fatal(err)
	}

	// --- Built-in tools ---
	if err := tool.RegisterPrintTool(tk); err != nil {
		log.Fatal(err)
	}
	if err := tool.RegisterShellTool(tk); err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Registered Tools ===")
	for _, name := range tk.GetToolNames() {
		fmt.Printf("  - %s\n", name)
	}

	fmt.Println("\n=== Tool Schemas ===")
	for _, schema := range tk.GetSchemas() {
		fmt.Printf("  %s: %s\n", schema.Function.Name, schema.Function.Description)
	}

	fmt.Println("\n=== HasTool Check ===")
	fmt.Printf("  Has 'greet': %v\n", tk.HasTool("greet"))
	fmt.Printf("  Has 'nonexistent': %v\n", tk.HasTool("nonexistent"))

	fmt.Println("\n=== Execute Tools ===")
	result, err := tk.Execute(context.Background(), "greet", map[string]interface{}{"name": "Alice"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  greet(name=Alice) -> %v\n", result.Content)

	result, err = tk.Execute(context.Background(), "search", map[string]interface{}{
		"query": "golang agents",
		"limit": float64(5),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  search(query=golang agents, limit=5) -> %v\n", result.Content)

	result, err = tk.Execute(context.Background(), "print_text", map[string]interface{}{
		"text": "Hello from built-in tool!",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  print_text(text=Hello...) -> %v\n", result.Content)
}

func greetFunc(_ context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
	name, _ := args["name"].(string)
	return &tool.ToolResponse{
		Content: fmt.Sprintf("Hello, %s! Nice to meet you!", name),
	}, nil
}

type SearchArgs struct {
	Query string `json:"query" description:"the search query string"`
	Limit int    `json:"limit,omitempty" description:"maximum number of results to return"`
}

func searchFunc(_ context.Context, args SearchArgs) (*tool.ToolResponse, error) {
	return &tool.ToolResponse{
		Content: fmt.Sprintf("Found %d results for '%s' (mock)", args.Limit, args.Query),
	}, nil
}
