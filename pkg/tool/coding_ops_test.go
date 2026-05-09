package tool

import (
	"context"
	"strings"
	"testing"
)

func TestRegisterExecuteShellCommandTool(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecuteShellCommandTool(tk); err != nil {
		t.Fatalf("RegisterExecuteShellCommandTool failed: %v", err)
	}
	if !tk.HasTool("execute_shell_command") {
		t.Fatal("execute_shell_command not registered")
	}
}

func TestExecuteShellCommand_Success(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecuteShellCommandTool(tk); err != nil {
		t.Fatalf("RegisterExecuteShellCommandTool failed: %v", err)
	}

	resp, err := tk.Execute(context.Background(), "execute_shell_command", map[string]interface{}{
		"command": "echo hello world",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "<returncode>0</returncode>") {
		t.Errorf("expected returncode 0 in output: %s", text)
	}
	if !strings.Contains(text, "<stdout>hello world") {
		t.Errorf("expected stdout in output: %s", text)
	}
}

func TestExecuteShellCommand_NonZeroExit(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecuteShellCommandTool(tk); err != nil {
		t.Fatalf("RegisterExecuteShellCommandTool failed: %v", err)
	}

	resp, err := tk.Execute(context.Background(), "execute_shell_command", map[string]interface{}{
		"command": "exit 42",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "<returncode>42</returncode>") {
		t.Errorf("expected returncode 42 in output: %s", text)
	}
}

func TestExecuteShellCommand_Stderr(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecuteShellCommandTool(tk); err != nil {
		t.Fatalf("RegisterExecuteShellCommandTool failed: %v", err)
	}

	resp, err := tk.Execute(context.Background(), "execute_shell_command", map[string]interface{}{
		"command": "echo error_msg >&2",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "error_msg") {
		t.Errorf("expected stderr in output: %s", text)
	}
}

func TestExecuteShellCommand_Timeout(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecuteShellCommandTool(tk); err != nil {
		t.Fatalf("RegisterExecuteShellCommandTool failed: %v", err)
	}

	resp, err := tk.Execute(context.Background(), "execute_shell_command", map[string]interface{}{
		"command": "sleep 10",
		"timeout": float64(1),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "returncode>") {
		t.Errorf("expected returncode tag in output: %s", text)
	}
	if resp.IsError {
		t.Error("IsError should be false for command execution results")
	}
}

func TestRegisterExecutePythonCodeTool(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecutePythonCodeTool(tk); err != nil {
		t.Fatalf("RegisterExecutePythonCodeTool failed: %v", err)
	}
	if !tk.HasTool("execute_python_code") {
		t.Fatal("execute_python_code not registered")
	}
}

func TestExecutePythonCode_Print(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecutePythonCodeTool(tk); err != nil {
		t.Fatalf("RegisterExecutePythonCodeTool failed: %v", err)
	}

	resp, err := tk.Execute(context.Background(), "execute_python_code", map[string]interface{}{
		"code": "print('hello from python')",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "<returncode>0</returncode>") {
		t.Errorf("expected returncode 0 in output: %s", text)
	}
	if !strings.Contains(text, "hello from python") {
		t.Errorf("expected python output in result: %s", text)
	}
}

func TestExecutePythonCode_SyntaxError(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecutePythonCodeTool(tk); err != nil {
		t.Fatalf("RegisterExecutePythonCodeTool failed: %v", err)
	}

	resp, err := tk.Execute(context.Background(), "execute_python_code", map[string]interface{}{
		"code": "print(",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "returncode>") {
		t.Errorf("expected returncode tag in output: %s", text)
	}
	if !strings.Contains(text, "<stderr>") {
		t.Errorf("expected stderr tag in output: %s", text)
	}
}

func TestExecutePythonCode_Computation(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterExecutePythonCodeTool(tk); err != nil {
		t.Fatalf("RegisterExecutePythonCodeTool failed: %v", err)
	}

	resp, err := tk.Execute(context.Background(), "execute_python_code", map[string]interface{}{
		"code": "x = 2 + 3\nprint(x)",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "5") {
		t.Errorf("expected 5 in output: %s", text)
	}
}
