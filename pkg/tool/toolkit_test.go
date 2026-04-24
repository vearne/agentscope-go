package tool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestNewToolkit(t *testing.T) {
	tk := NewToolkit()
	if tk == nil {
		t.Fatal("NewToolkit returned nil")
	}
	if len(tk.GetToolNames()) != 0 {
		t.Fatal("new toolkit should have no tools")
	}
}

func TestToolkit_RegisterAndExecute(t *testing.T) {
	tk := NewToolkit()
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{
				"type":        "string",
				"description": "The city name",
			},
		},
		"required": []string{"city"},
	}

	weatherFn := func(_ context.Context, args map[string]interface{}) (*ToolResponse, error) {
		city, _ := args["city"].(string)
		return &ToolResponse{Content: "sunny in " + city}, nil
	}

	if err := tk.Register("get_weather", "Get weather for a city", params, weatherFn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !tk.HasTool("get_weather") {
		t.Fatal("HasTool should return true for registered tool")
	}
	if tk.HasTool("nonexistent") {
		t.Fatal("HasTool should return false for unregistered tool")
	}

	resp, err := tk.Execute(context.Background(), "get_weather", map[string]interface{}{
		"city": "Beijing",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.Content != "sunny in Beijing" {
		t.Errorf("unexpected content: %v", resp.Content)
	}
	if resp.IsError {
		t.Error("IsError should be false")
	}
}

func TestToolkit_RegisterDuplicate(t *testing.T) {
	tk := NewToolkit()
	fn := func(_ context.Context, _ map[string]interface{}) (*ToolResponse, error) {
		return &ToolResponse{Content: "ok"}, nil
	}

	if err := tk.Register("dup", "first", nil, fn); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if err := tk.Register("dup", "second", nil, fn); err == nil {
		t.Fatal("duplicate Register should return error")
	}
}

func TestToolkit_ExecuteNotFound(t *testing.T) {
	tk := NewToolkit()
	_, err := tk.Execute(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("Execute on nonexistent tool should return error")
	}
}

func TestToolkit_GetSchemas(t *testing.T) {
	tk := NewToolkit()
	params := map[string]interface{}{
		"type": "object",
	}

	fn := func(_ context.Context, _ map[string]interface{}) (*ToolResponse, error) {
		return &ToolResponse{Content: "ok"}, nil
	}

	tk.Register("tool_a", "Tool A", params, fn)
	tk.Register("tool_b", "Tool B", params, fn)

	schemas := tk.GetSchemas()
	if len(schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(schemas))
	}

	names := map[string]bool{}
	for _, s := range schemas {
		names[s.Function.Name] = true
		if s.Type != "function" {
			t.Errorf("expected type 'function', got '%s'", s.Type)
		}
	}
	if !names["tool_a"] || !names["tool_b"] {
		t.Errorf("missing tool names in schemas: %v", names)
	}
}

func TestToolkit_GetToolNames(t *testing.T) {
	tk := NewToolkit()
	fn := func(_ context.Context, _ map[string]interface{}) (*ToolResponse, error) {
		return &ToolResponse{Content: "ok"}, nil
	}

	tk.Register("alpha", "Alpha", nil, fn)
	tk.Register("beta", "Beta", nil, fn)

	names := tk.GetToolNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}

	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["alpha"] || !found["beta"] {
		t.Errorf("missing names: %v", found)
	}
}

func TestToolkit_RegisterFunc(t *testing.T) {
	type WeatherArgs struct {
		City string `json:"city" description:"The city name"`
	}

	fn := func(_ context.Context, args WeatherArgs) (*ToolResponse, error) {
		return &ToolResponse{Content: "weather in " + args.City}, nil
	}

	tk := NewToolkit()
	err := tk.RegisterFunc(fn, RegisterOption{
		Name:        "get_weather",
		Description: "Get weather",
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	if !tk.HasTool("get_weather") {
		t.Fatal("tool not registered")
	}

	schemas := tk.GetSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	params := schemas[0].Function.Parameters
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties not found in schema")
	}
	if _, ok := props["city"]; !ok {
		t.Fatal("city property not found")
	}

	resp, err := tk.Execute(context.Background(), "get_weather", map[string]interface{}{
		"city": "Shanghai",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.Content != "weather in Shanghai" {
		t.Errorf("unexpected content: %v", resp.Content)
	}
}

func TestToolkit_RegisterFuncError(t *testing.T) {
	tk := NewToolkit()

	if err := tk.RegisterFunc("not a function"); err == nil {
		t.Fatal("RegisterFunc with non-function should error")
	}

	if err := tk.RegisterFunc(func() {}); err == nil {
		t.Fatal("RegisterFunc with wrong signature should error")
	}
}

func TestToolkit_RegisterFuncToolError(t *testing.T) {
	type Args struct {
		Input string `json:"input"`
	}

	fn := func(_ context.Context, _ Args) (*ToolResponse, error) {
		return nil, errors.New("something went wrong")
	}

	tk := NewToolkit()
	tk.RegisterFunc(fn, RegisterOption{Name: "fail_tool", Description: "always fails"})

	_, err := tk.Execute(context.Background(), "fail_tool", map[string]interface{}{
		"input": "test",
	})
	if err == nil {
		t.Fatal("expected error from tool execution")
	}
	if err.Error() != "something went wrong" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterShellTool(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterShellTool(tk); err != nil {
		t.Fatalf("RegisterShellTool failed: %v", err)
	}
	if !tk.HasTool("execute_shell") {
		t.Fatal("execute_shell not registered")
	}

	resp, err := tk.Execute(context.Background(), "execute_shell", map[string]interface{}{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	content, ok := resp.Content.(string)
	if !ok {
		t.Fatalf("content should be string, got %T", resp.Content)
	}
	if content != "hello\n" {
		t.Errorf("unexpected output: %q", content)
	}
}

func TestRegisterPrintTool(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterPrintTool(tk); err != nil {
		t.Fatalf("RegisterPrintTool failed: %v", err)
	}
	if !tk.HasTool("print_text") {
		t.Fatal("print_text not registered")
	}

	resp, err := tk.Execute(context.Background(), "print_text", map[string]interface{}{
		"text": "hello world",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("unexpected content: %v", resp.Content)
	}
}

func TestToolkit_Concurrent(t *testing.T) {
	tk := NewToolkit()
	fn := func(_ context.Context, args map[string]interface{}) (*ToolResponse, error) {
		return &ToolResponse{Content: args["name"]}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("tool_%d", n)
			tk.Register(name, "desc", nil, fn)
		}(i)
	}
	wg.Wait()

	if len(tk.GetToolNames()) != 50 {
		t.Fatalf("expected 50 tools, got %d", len(tk.GetToolNames()))
	}
}
