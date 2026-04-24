package memory

import (
	"context"
	"sync"

	"github.com/vearne/agentscope-go/pkg/message"
)

type InMemoryMemory struct {
	messages []*message.Msg
	mu       sync.RWMutex
}

func NewInMemoryMemory() *InMemoryMemory {
	return &InMemoryMemory{
		messages: make([]*message.Msg, 0),
	}
}

func (m *InMemoryMemory) Add(_ context.Context, msgs ...*message.Msg) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msgs...)
	return nil
}

func (m *InMemoryMemory) GetMessages() []*message.Msg {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*message.Msg, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *InMemoryMemory) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = m.messages[:0]
	return nil
}

func (m *InMemoryMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

func (m *InMemoryMemory) ToStrList() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, 0, len(m.messages))
	for _, msg := range m.messages {
		result = append(result, msg.GetTextContent())
	}
	return result
}
