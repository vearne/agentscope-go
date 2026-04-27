package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterViewTextFileTool(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterViewTextFileTool(tk); err != nil {
		t.Fatalf("RegisterViewTextFileTool failed: %v", err)
	}
	if !tk.HasTool("view_text_file") {
		t.Fatal("view_text_file not registered")
	}
}

func TestViewTextFile_FullContent(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterViewTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "view_text_file", map[string]interface{}{
		"file_path": fpath,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "1: line1") {
		t.Errorf("missing line 1 in output: %s", text)
	}
	if !strings.Contains(text, "2: line2") {
		t.Errorf("missing line 2 in output: %s", text)
	}
	if !strings.Contains(text, "3: line3") {
		t.Errorf("missing line 3 in output: %s", text)
	}
}

func TestViewTextFile_WithRanges(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterViewTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "view_text_file", map[string]interface{}{
		"file_path": fpath,
		"ranges":    []interface{}{float64(2), float64(4)},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "2: line2") {
		t.Errorf("missing line 2 in output: %s", text)
	}
	if !strings.Contains(text, "3: line3") {
		t.Errorf("missing line 3 in output: %s", text)
	}
	if !strings.Contains(text, "4: line4") {
		t.Errorf("missing line 4 in output: %s", text)
	}
	if strings.Contains(text, "1: line1") {
		t.Errorf("line 1 should not be in range output: %s", text)
	}
	if strings.Contains(text, "5: line5") {
		t.Errorf("line 5 should not be in range output: %s", text)
	}
}

func TestViewTextFile_FileNotExist(t *testing.T) {
	tk := NewToolkit()
	RegisterViewTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "view_text_file", map[string]interface{}{
		"file_path": "/nonexistent/file.txt",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "does not exist") {
		t.Errorf("expected not exist error, got: %s", text)
	}
}

func TestViewTextFile_IsDir(t *testing.T) {
	tmpDir := t.TempDir()

	tk := NewToolkit()
	RegisterViewTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "view_text_file", map[string]interface{}{
		"file_path": tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "not a file") {
		t.Errorf("expected not-a-file error, got: %s", text)
	}
}

func TestViewTextFile_RangeOutOfBounds(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(fpath, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterViewTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "view_text_file", map[string]interface{}{
		"file_path": fpath,
		"ranges":    []interface{}{float64(1), float64(100)},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "1: a") {
		t.Errorf("expected line 1 in output: %s", text)
	}
}

func TestRegisterWriteTextFileTool(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterWriteTextFileTool(tk); err != nil {
		t.Fatalf("RegisterWriteTextFileTool failed: %v", err)
	}
	if !tk.HasTool("write_text_file") {
		t.Fatal("write_text_file not registered")
	}
}

func TestWriteTextFile_CreateNew(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "new.txt")

	tk := NewToolkit()
	RegisterWriteTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "write_text_file", map[string]interface{}{
		"file_path": fpath,
		"content":   "hello world",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "successfully") {
		t.Errorf("expected success message, got: %s", text)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

func TestWriteTextFile_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(fpath, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterWriteTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "write_text_file", map[string]interface{}{
		"file_path": fpath,
		"content":   "new content",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "Overwrite") {
		t.Errorf("expected overwrite message, got: %s", text)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new content" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

func TestWriteTextFile_ReplaceRange(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "replace.txt")
	if err := os.WriteFile(fpath, []byte("line1\nline2\nline3\nline4\nline5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterWriteTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "write_text_file", map[string]interface{}{
		"file_path": fpath,
		"content":   "replaced\n",
		"ranges":    []interface{}{float64(2), float64(4)},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "successfully") {
		t.Errorf("expected success, got: %s", text)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}

	expected := "line1\nreplaced\nline5\n"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

func TestWriteTextFile_InvalidStartLine(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "short.txt")
	if err := os.WriteFile(fpath, []byte("only one line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterWriteTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "write_text_file", map[string]interface{}{
		"file_path": fpath,
		"content":   "x",
		"ranges":    []interface{}{float64(10), float64(20)},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "invalid") && !strings.Contains(text, "Error") {
		t.Errorf("expected error about invalid start, got: %s", text)
	}
}

func TestRegisterInsertTextFileTool(t *testing.T) {
	tk := NewToolkit()
	if err := RegisterInsertTextFileTool(tk); err != nil {
		t.Fatalf("RegisterInsertTextFileTool failed: %v", err)
	}
	if !tk.HasTool("insert_text_file") {
		t.Fatal("insert_text_file not registered")
	}
}

func TestInsertTextFile_Middle(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "insert.txt")
	if err := os.WriteFile(fpath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterInsertTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "insert_text_file", map[string]interface{}{
		"file_path":   fpath,
		"content":     "inserted",
		"line_number": float64(2),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "successfully") {
		t.Errorf("expected success, got: %s", text)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	expected := []string{"line1", "inserted", "line2", "line3"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %v", len(expected), len(lines), lines)
	}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, lines[i])
		}
	}
}

func TestInsertTextFile_Append(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "append.txt")
	if err := os.WriteFile(fpath, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterInsertTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "insert_text_file", map[string]interface{}{
		"file_path":   fpath,
		"content":     "appended",
		"line_number": float64(3),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "successfully") {
		t.Errorf("expected success, got: %s", text)
	}
}

func TestInsertTextFile_InvalidLineNumber(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(fpath, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterInsertTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "insert_text_file", map[string]interface{}{
		"file_path":   fpath,
		"content":     "x",
		"line_number": float64(0),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "invalid") {
		t.Errorf("expected invalid error, got: %s", text)
	}
}

func TestInsertTextFile_FileNotExist(t *testing.T) {
	tk := NewToolkit()
	RegisterInsertTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "insert_text_file", map[string]interface{}{
		"file_path":   "/nonexistent/file.txt",
		"content":     "x",
		"line_number": float64(1),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "does not exist") {
		t.Errorf("expected not exist error, got: %s", text)
	}
}

func TestInsertTextFile_LineNumberTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(fpath, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := NewToolkit()
	RegisterInsertTextFileTool(tk)

	resp, err := tk.Execute(context.Background(), "insert_text_file", map[string]interface{}{
		"file_path":   fpath,
		"content":     "x",
		"line_number": float64(99),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	text := resp.Content.(string)
	if !strings.Contains(text, "valid range") {
		t.Errorf("expected valid range error, got: %s", text)
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"~/test", true},
		{"/absolute/path", false},
		{"relative/path", false},
	}
	for _, tt := range tests {
		result := expandHome(tt.input)
		if tt.want && !strings.Contains(result, "Users") && !strings.Contains(result, "home") {
			t.Errorf("expandHome(%q) = %q, expected home dir expansion", tt.input, result)
		}
		if !tt.want && result != tt.input {
			t.Errorf("expandHome(%q) = %q, expected unchanged", tt.input, result)
		}
	}
}

func TestCalculateViewRanges(t *testing.T) {
	tests := []struct {
		name       string
		oldN       int
		newN       int
		start      int
		end        int
		extra      int
		wantStart  int
		wantEnd    int
	}{
		{"basic", 10, 10, 5, 5, 2, 3, 7},
		{"at_top", 10, 10, 1, 1, 5, 1, 6},
		{"at_bottom", 10, 10, 10, 10, 5, 5, 10},
		{"expanded", 5, 10, 3, 3, 2, 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := calculateViewRanges(tt.oldN, tt.newN, tt.start, tt.end, tt.extra)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Errorf("calculateViewRanges() = (%d, %d), want (%d, %d)",
					gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestToIntSlice(t *testing.T) {
	tests := []struct {
		input  interface{}
		expect []int
	}{
		{[]interface{}{float64(1), float64(2), float64(3)}, []int{1, 2, 3}},
		{[]interface{}{1, 2, 3}, []int{1, 2, 3}},
		{[]int{4, 5}, []int{4, 5}},
		{"not a slice", nil},
	}
	for _, tt := range tests {
		result := toIntSlice(tt.input)
		if len(result) != len(tt.expect) {
			t.Errorf("toIntSlice(%v) = %v, want %v", tt.input, result, tt.expect)
			continue
		}
		for i := range result {
			if result[i] != tt.expect[i] {
				t.Errorf("toIntSlice(%v)[%d] = %d, want %d", tt.input, i, result[i], tt.expect[i])
			}
		}
	}
}
