package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
)

type JSONSession struct {
	filePath string
}

func NewJSONSession(filePath string) *JSONSession {
	return &JSONSession{filePath: filePath}
}

func (s *JSONSession) Save(ctx context.Context, mem memory.MemoryBase) error {
	msgs := mem.GetMessages()
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", s.filePath, err)
	}
	return nil
}

func (s *JSONSession) Load(ctx context.Context, mem memory.MemoryBase) error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", s.filePath, err)
	}
	var msgs []*message.Msg
	if err := json.Unmarshal(data, &msgs); err != nil {
		return fmt.Errorf("unmarshal messages: %w", err)
	}
	if err := mem.Add(ctx, msgs...); err != nil {
		return fmt.Errorf("add messages to memory: %w", err)
	}
	return nil
}
