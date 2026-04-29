package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// OffloadManager stores large tool results to the filesystem,
// replacing them with a reference in conversation history.
type OffloadManager struct {
	dir       string // filesystem directory for offloaded content
	threshold int    // character count threshold for offloading
}

// NewOffloadManager creates a new OffloadManager.
func NewOffloadManager(dir string, threshold int) *OffloadManager {
	return &OffloadManager{dir: dir, threshold: threshold}
}

// MaybeOffload checks if content exceeds the threshold.
// If so, it writes the content to a file and returns a reference string.
// Returns (result, wasOffloaded).
func (m *OffloadManager) MaybeOffload(content string, msgID string) (string, bool) {
	if len(content) <= m.threshold {
		return content, false
	}

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		// Offload write failure: keep full content in memory (degraded but functional)
		return content, false
	}

	path := filepath.Join(m.dir, msgID+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		// Offload write failure: keep full content in memory
		return content, false
	}

	previewLen := 200
	if len(content) < previewLen {
		previewLen = len(content)
	}
	preview := content[:previewLen]
	ref := fmt.Sprintf("[Result offloaded to %s. Preview: %s...]", path, preview)
	return ref, true
}

// Read loads a previously offloaded file from disk.
func (m *OffloadManager) Read(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read offloaded file %s: %w", path, err)
	}
	return string(data), nil
}
