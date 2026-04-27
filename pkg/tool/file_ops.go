package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RegisterViewTextFileTool registers the view_text_file tool that reads file
// content with line numbers, optionally within a given line range.
func RegisterViewTextFileTool(tk *Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The target file path.",
			},
			"ranges": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "integer"},
				"description": "The range of lines to be viewed (e.g. [1, 100]), inclusive. If not provided, the entire file will be returned. To view the last 100 lines, use [-100, -1].",
			},
		},
		"required": []string{"file_path"},
	}

	return tk.Register("view_text_file",
		"View the file content in the specified range with line numbers. If ranges is not provided, the entire file will be returned.",
		params, viewTextFile)
}

// RegisterWriteTextFileTool registers the write_text_file tool that can create,
// overwrite, or replace content within a range of lines in a text file.
func RegisterWriteTextFileTool(tk *Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The target file path.",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to be written.",
			},
			"ranges": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "integer"},
				"description": "The range of lines to be replaced. If not provided, the entire file will be overwritten.",
			},
		},
		"required": []string{"file_path", "content"},
	}

	return tk.Register("write_text_file",
		"Create/Replace/Overwrite content in a text file. When ranges is provided, the content will be replaced in the specified range. Otherwise, the entire file (if exists) will be overwritten.",
		params, writeTextFile)
}

// RegisterInsertTextFileTool registers the insert_text_file tool that inserts
// content at a specified line number in a text file.
func RegisterInsertTextFileTool(tk *Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The target file path.",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to be inserted.",
			},
			"line_number": map[string]interface{}{
				"type":        "integer",
				"description": "The line number at which the content should be inserted, starting from 1. If exceeds the number of lines in the file, it will be appended to the end of the file.",
			},
		},
		"required": []string{"file_path", "content", "line_number"},
	}

	return tk.Register("insert_text_file",
		"Insert the content at the specified line number in a text file.",
		params, insertTextFile)
}

// viewTextFile implements the core logic for viewing file content with line numbers.
func viewTextFile(_ context.Context, args map[string]interface{}) (*ToolResponse, error) {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return &ToolResponse{Content: "file_path must be a string", IsError: true}, nil
	}

	filePath = expandHome(filePath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &ToolResponse{
			Content: fmt.Sprintf("Error: The file %s does not exist.", filePath),
		}, nil
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
	}
	if info.IsDir() {
		return &ToolResponse{
			Content: fmt.Sprintf("Error: The path %s is not a file.", filePath),
		}, nil
	}

	var ranges []int
	if raw, ok := args["ranges"]; ok {
		ranges = toIntSlice(raw)
	}

	content, err := viewFileContent(filePath, ranges)
	if err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
	}

	if ranges == nil {
		return &ToolResponse{
			Content: fmt.Sprintf("The content of %s:\n```\n%s```", filePath, content),
		}, nil
	}
	return &ToolResponse{
		Content: fmt.Sprintf("The content of %s in %v lines:\n```\n%s```", filePath, ranges, content),
	}, nil
}

// writeTextFile implements the core logic for writing/replacing content in a file.
func writeTextFile(_ context.Context, args map[string]interface{}) (*ToolResponse, error) {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return &ToolResponse{Content: "file_path must be a string", IsError: true}, nil
	}
	content, ok := args["content"].(string)
	if !ok {
		return &ToolResponse{Content: "content must be a string", IsError: true}, nil
	}

	filePath = expandHome(filePath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
		}
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
		}

		var ranges []int
		if raw, ok := args["ranges"]; ok {
			ranges = toIntSlice(raw)
		}
		if ranges != nil {
			return &ToolResponse{
				Content: fmt.Sprintf("Create and write %s successfully. The ranges %v is ignored because the file does not exist.", filePath, ranges),
			}, nil
		}
		return &ToolResponse{
			Content: fmt.Sprintf("Create and write %s successfully.", filePath),
		}, nil
	}

	originalLines, err := readFileLines(filePath)
	if err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
	}

	var ranges []int
	if raw, ok := args["ranges"]; ok {
		ranges = toIntSlice(raw)
	}

	if ranges != nil {
		if len(ranges) != 2 {
			return &ToolResponse{
				Content: fmt.Sprintf("Error: Invalid range format. Expected an array of two integers, but got %v.", ranges),
			}, nil
		}
		start, end := ranges[0], ranges[1]
		if start > len(originalLines) {
			return &ToolResponse{
				Content: fmt.Sprintf("Error: The start line %d is invalid. The file only has %d lines.", start, len(originalLines)),
			}, nil
		}

		// Lines are 1-indexed, inclusive — matching the Python agentscope convention.
		var newContent []string
		newContent = append(newContent, originalLines[:start-1]...)
		newContent = append(newContent, content)
		newContent = append(newContent, originalLines[end:]...)

		if err := os.WriteFile(filePath, []byte(strings.Join(newContent, "")), 0o644); err != nil {
			return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
		}

		newLines, err := readFileLines(filePath)
		if err != nil {
			return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
		}

		viewStart, viewEnd := calculateViewRanges(len(originalLines), len(newLines), start, end, 5)

		viewContent := formatLinesWithNumbers(newLines, viewStart, viewEnd)

		return &ToolResponse{
			Content: fmt.Sprintf("Write %s successfully. The new content snippet:\n```\n%s```", filePath, viewContent),
		}, nil
	}

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
	}

	return &ToolResponse{
		Content: fmt.Sprintf("Overwrite %s successfully.", filePath),
	}, nil
}

// insertTextFile implements the core logic for inserting content at a line number.
func insertTextFile(_ context.Context, args map[string]interface{}) (*ToolResponse, error) {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return &ToolResponse{Content: "file_path must be a string", IsError: true}, nil
	}
	content, ok := args["content"].(string)
	if !ok {
		return &ToolResponse{Content: "content must be a string", IsError: true}, nil
	}
	lineNumber := toInt(args["line_number"])
	if lineNumber <= 0 {
		return &ToolResponse{
			Content: fmt.Sprintf("InvalidArgumentsError: The line number %d is invalid.", lineNumber),
		}, nil
	}

	filePath = expandHome(filePath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &ToolResponse{
			Content: fmt.Sprintf("InvalidArgumentsError: The target file %s does not exist.", filePath),
		}, nil
	}

	originalLines, err := readFileLines(filePath)
	if err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
	}

	if lineNumber == len(originalLines)+1 {
		originalLines = append(originalLines, "\n"+content)
	} else if lineNumber < len(originalLines)+1 {
		// Insert at lineNumber (1-indexed).
		insertContent := content + "\n"
		originalLines = append(
			originalLines[:lineNumber-1],
			append([]string{insertContent}, originalLines[lineNumber-1:]...)...,
		)
	} else {
		return &ToolResponse{
			Content: fmt.Sprintf("InvalidArgumentsError: The given line_number (%d) is not in the valid range [1, %d].", lineNumber, len(originalLines)+1),
		}, nil
	}

	if err := os.WriteFile(filePath, []byte(strings.Join(originalLines, "")), 0o644); err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
	}

	newLines, err := readFileLines(filePath)
	if err != nil {
		return &ToolResponse{Content: fmt.Sprintf("Error: %v", err), IsError: true}, nil
	}

	viewStart, viewEnd := calculateViewRanges(len(originalLines), len(newLines), lineNumber, lineNumber, 5)
	showContent := formatLinesWithNumbers(newLines, viewStart, viewEnd)

	return &ToolResponse{
		Content: fmt.Sprintf("Insert content into %s at line %d successfully. The new content between lines %d-%d is:\n```\n%s```", filePath, lineNumber, viewStart, viewEnd, showContent),
	}, nil
}

// expandHome replaces a leading ~/ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// readFileLines reads a file and returns its lines (each includes the trailing newline).
func readFileLines(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return splitLines(string(data)), nil
}

// splitLines splits text into lines preserving trailing newlines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// viewFileContent returns file content with line numbers, optionally within a range.
func viewFileContent(filePath string, ranges []int) (string, error) {
	lines, err := readFileLines(filePath)
	if err != nil {
		return "", err
	}

	if ranges == nil {
		return formatLinesWithNumbers(lines, 1, len(lines)), nil
	}

	if len(ranges) != 2 {
		return "", fmt.Errorf("InvalidArgumentError: Invalid range format. Expected an array of two integers, but got %v", ranges)
	}

	start, end := ranges[0], ranges[1]
	if start > end {
		return "", fmt.Errorf("InvalidArgumentError: The start line is greater than the end line in the given range %v", ranges)
	}
	if start > len(lines) {
		return "", fmt.Errorf("InvalidArgumentError: The range '%v' is out of bounds for the file '%s', which has only %d lines", ranges, filePath, len(lines))
	}

	return formatLinesWithNumbers(lines, start, end), nil
}

// formatLinesWithNumbers formats lines from start to end (1-indexed, inclusive) with line numbers.
func formatLinesWithNumbers(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}

	var sb strings.Builder
	for i := start; i <= end; i++ {
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(": ")
		sb.WriteString(lines[i-1])
	}
	return sb.String()
}

// calculateViewRanges computes the view window after a write operation,
// including extraViewNLines of context before and after the changed region.
func calculateViewRanges(oldNLines, newNLines, start, end, extraViewNLines int) (int, int) {
	viewStart := start - extraViewNLines
	if viewStart < 1 {
		viewStart = 1
	}

	deltaLines := newNLines - oldNLines
	viewEnd := end + deltaLines + extraViewNLines
	if viewEnd > newNLines {
		viewEnd = newNLines
	}

	return viewStart, viewEnd
}

// toIntSlice converts an interface{} (from JSON) to []int.
func toIntSlice(v interface{}) []int {
	switch val := v.(type) {
	case []interface{}:
		result := make([]int, 0, len(val))
		for _, item := range val {
			switch n := item.(type) {
			case float64:
				result = append(result, int(n))
			case int:
				result = append(result, n)
			case json_number:
				i, _ := n.Int64()
				result = append(result, int(i))
			}
		}
		return result
	case []int:
		return val
	default:
		return nil
	}
}

// toInt converts an interface{} to int (handles float64 from JSON).
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// json_number is a minimal interface to handle encoding/json Number values.
type json_number interface {
	Int64() (int64, error)
}
