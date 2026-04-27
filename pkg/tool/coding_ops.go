package tool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// RegisterExecuteShellCommandTool registers the execute_shell_command tool
// that runs a shell command and returns returncode, stdout, and stderr.
func RegisterExecuteShellCommandTool(tk *Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute.",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "The maximum time (in seconds) allowed for the command to run.",
			},
		},
		"required": []string{"command"},
	}

	return tk.Register("execute_shell_command",
		"Execute given command and return the return code, standard output and error.",
		params, executeShellCommand)
}

// RegisterExecutePythonCodeTool registers the execute_python_code tool
// that executes Python code in a temp file and captures stdout/stderr.
func RegisterExecutePythonCodeTool(tk *Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"code": map[string]interface{}{
				"type":        "string",
				"description": "The Python code to be executed.",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "The maximum time (in seconds) allowed for the code to run.",
			},
		},
		"required": []string{"code"},
	}

	return tk.Register("execute_python_code",
		"Execute the given python code in a temp file and capture the return code, standard output and error. Note you must print the output to get the result.",
		params, executePythonCode)
}

func executeShellCommand(ctx context.Context, args map[string]interface{}) (*ToolResponse, error) {
	cmdStr, ok := args["command"].(string)
	if !ok {
		return &ToolResponse{Content: "command must be a string", IsError: true}, nil
	}

	timeout := 300
	if t, ok := args["timeout"]; ok {
		timeout = toInt(t)
		if timeout <= 0 {
			timeout = 300
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	returncode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			returncode = exitErr.ExitCode()
		} else {
			returncode = -1
			timeoutSuffix := fmt.Sprintf("TimeoutError: The command execution exceeded the timeout of %d seconds.", timeout)
			if stderrStr != "" {
				stderrStr += "\n" + timeoutSuffix
			} else {
				stderrStr = timeoutSuffix
			}
		}
	}

	return &ToolResponse{
		Content: fmt.Sprintf("<returncode>%d</returncode><stdout>%s</stdout><stderr>%s</stderr>",
			returncode, stdoutStr, stderrStr),
	}, nil
}

func executePythonCode(ctx context.Context, args map[string]interface{}) (*ToolResponse, error) {
	code, ok := args["code"].(string)
	if !ok {
		return &ToolResponse{Content: "code must be a string", IsError: true}, nil
	}

	timeout := 300
	if t, ok := args["timeout"]; ok {
		timeout = toInt(t)
		if timeout <= 0 {
			timeout = 300
		}
	}

	tmpDir, err := os.MkdirTemp("", "agentscope-python-*")
	if err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error creating temp dir: %v", err), IsError: true}, nil
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("tmp_%s.py", uuid.New().String()[:8]))
	if err := os.WriteFile(tmpFile, []byte(code), 0o644); err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error writing temp file: %v", err), IsError: true}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-u", tmpFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")

	err = cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	returncode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			returncode = exitErr.ExitCode()
		} else {
			returncode = -1
			timeoutSuffix := fmt.Sprintf("TimeoutError: The code execution exceeded the timeout of %d seconds.", timeout)
			if stderrStr != "" {
				stderrStr += "\n" + timeoutSuffix
			} else {
				stderrStr = timeoutSuffix
			}
		}
	}

	return &ToolResponse{
		Content: fmt.Sprintf("<returncode>%d</returncode><stdout>%s</stdout><stderr>%s</stderr>",
			returncode, stdoutStr, stderrStr),
	}, nil
}
