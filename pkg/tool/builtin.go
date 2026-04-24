package tool

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

func RegisterShellTool(tk *Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default 30)",
			},
		},
		"required": []string{"command"},
	}

	return tk.Register("execute_shell", "Execute a shell command and return its output", params, executeShell)
}

func executeShell(ctx context.Context, args map[string]interface{}) (*ToolResponse, error) {
	cmdStr, ok := args["command"].(string)
	if !ok {
		return &ToolResponse{
			Content: "command must be a string",
			IsError: true,
		}, nil
	}

	timeout := 30
	if t, ok := args["timeout"].(float64); ok {
		timeout = int(t)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	if err != nil {
		return &ToolResponse{
			Content: output + "\nerror: " + err.Error(),
			IsError: true,
		}, nil
	}

	return &ToolResponse{
		Content: output,
	}, nil
}

func RegisterPrintTool(tk *Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "The text to print",
			},
		},
		"required": []string{"text"},
	}

	return tk.Register("print_text", "Print text to output", params, printText)
}

func printText(_ context.Context, args map[string]interface{}) (*ToolResponse, error) {
	text, ok := args["text"].(string)
	if !ok {
		return &ToolResponse{
			Content: "text must be a string",
			IsError: true,
		}, nil
	}

	return &ToolResponse{
		Content: text,
	}, nil
}
