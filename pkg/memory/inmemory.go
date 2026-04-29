package memory

import (
	"context"
	"sync"

	"github.com/vearne/agentscope-go/pkg/message"
)

// memoryEntry represents a message with its associated marks.
type memoryEntry struct {
	msg   *message.Msg
	marks []string
}

// InMemoryMemory is the in-memory implementation of memory storage.
type InMemoryMemory struct {
	messages          []memoryEntry
	compressedSummary string
	mu                sync.RWMutex
}

// NewInMemoryMemory creates a new in-memory storage.
func NewInMemoryMemory() *InMemoryMemory {
	return &InMemoryMemory{
		messages: make([]memoryEntry, 0),
	}
}

// Add adds message(s) into the memory storage.
func (m *InMemoryMemory) Add(_ context.Context, msgs ...*message.Msg) error {
	return m.AddWithMarks(nil, msgs, nil)
}

// AddWithMarks adds message(s) into the memory storage with specified marks.
func (m *InMemoryMemory) AddWithMarks(_ context.Context, msgs []*message.Msg, marks []string) error {
	if len(msgs) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	markList := marks
	if markList == nil {
		markList = []string{}
	}

	for _, msg := range msgs {
		m.messages = append(m.messages, memoryEntry{
			msg:   msg,
			marks: markList,
		})
	}

	return nil
}

// GetMessages returns all messages without filtering.
func (m *InMemoryMemory) GetMessages() []*message.Msg {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*message.Msg, len(m.messages))
	for i, entry := range m.messages {
		result[i] = entry.msg
	}
	return result
}

// GetMemory retrieves messages with optional mark filtering.
func (m *InMemoryMemory) GetMemory(_ context.Context, mark string, excludeMark string, prependSummary bool) ([]*message.Msg, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []memoryEntry

	for _, entry := range m.messages {
		if mark != "" {
			hasMark := false
			for _, mk := range entry.marks {
				if mk == mark {
					hasMark = true
					break
				}
			}
			if !hasMark {
				continue
			}
		}

		if excludeMark != "" {
			hasExcludedMark := false
			for _, mk := range entry.marks {
				if mk == excludeMark {
					hasExcludedMark = true
					break
				}
			}
			if hasExcludedMark {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	result := make([]*message.Msg, 0, len(filtered))
	if prependSummary && m.compressedSummary != "" {
		result = append(result, message.NewMsg("user", m.compressedSummary, "user"))
	}

	for _, entry := range filtered {
		result = append(result, entry.msg)
	}

	return result, nil
}

// Clear clears all messages from the storage.
func (m *InMemoryMemory) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = m.messages[:0]
	m.compressedSummary = ""
	return nil
}

// Delete removes message(s) from the storage by their IDs.
func (m *InMemoryMemory) Delete(_ context.Context, msgIDs []string) (int, error) {
	if len(msgIDs) == 0 {
		return 0, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	initialSize := len(m.messages)
	idSet := make(map[string]struct{})
	for _, id := range msgIDs {
		idSet[id] = struct{}{}
	}

	filtered := make([]memoryEntry, 0, len(m.messages))
	for _, entry := range m.messages {
		if _, exists := idSet[entry.msg.ID]; !exists {
			filtered = append(filtered, entry)
		}
	}

	m.messages = filtered
	return initialSize - len(filtered), nil
}

// DeleteByMark removes messages from the memory by their marks.
func (m *InMemoryMemory) DeleteByMark(_ context.Context, marks []string) (int, error) {
	if len(marks) == 0 {
		return 0, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	markSet := make(map[string]struct{})
	for _, mark := range marks {
		markSet[mark] = struct{}{}
	}

	initialSize := len(m.messages)
	filtered := make([]memoryEntry, 0, len(m.messages))
	for _, entry := range m.messages {
		hasMark := false
		for _, mk := range entry.marks {
			if _, exists := markSet[mk]; exists {
				hasMark = true
				break
			}
		}
		if !hasMark {
			filtered = append(filtered, entry)
		}
	}

	m.messages = filtered
	return initialSize - len(filtered), nil
}

// Size returns the number of messages in the storage.
func (m *InMemoryMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

// ToStrList converts messages to a list of strings.
func (m *InMemoryMemory) ToStrList() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, 0, len(m.messages))
	for _, entry := range m.messages {
		result = append(result, entry.msg.GetTextContent())
	}
	return result
}

// UpdateCompressedSummary updates the compressed summary.
func (m *InMemoryMemory) UpdateCompressedSummary(_ context.Context, summary string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compressedSummary = summary
	return nil
}

// UpdateMessagesMark updates marks of messages.
func (m *InMemoryMemory) UpdateMessagesMark(_ context.Context, newMark string, oldMark string, msgIDs []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	updatedCount := 0
	msgIDSet := make(map[string]struct{})
	for _, id := range msgIDs {
		msgIDSet[id] = struct{}{}
	}

	for i, entry := range m.messages {
		if len(msgIDs) > 0 {
			if _, exists := msgIDSet[entry.msg.ID]; !exists {
				continue
			}
		}

		if oldMark != "" {
			hasOldMark := false
			for _, mk := range entry.marks {
				if mk == oldMark {
					hasOldMark = true
					break
				}
			}
			if !hasOldMark {
				continue
			}
		}

		if newMark == "" {
			for j, mk := range entry.marks {
				if mk == oldMark {
					entry.marks = append(entry.marks[:j], entry.marks[j+1:]...)
					updatedCount++
					break
				}
			}
		} else {
			if oldMark != "" {
				for j, mk := range entry.marks {
					if mk == oldMark {
						entry.marks = append(entry.marks[:j], entry.marks[j+1:]...)
						break
					}
				}
			}
			hasNewMark := false
			for _, mk := range entry.marks {
				if mk == newMark {
					hasNewMark = true
					break
				}
			}
			if !hasNewMark {
				entry.marks = append(entry.marks, newMark)
				updatedCount++
			}
		}

		m.messages[i] = entry
	}

	return updatedCount, nil
}
