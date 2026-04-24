package message

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
)

type Msg struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Role         string                 `json:"role"`
	Content      []ContentBlock         `json:"content"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Timestamp    string                 `json:"timestamp"`
	InvocationID string                `json:"invocation_id,omitempty"`
}

func NewMsg(name string, content interface{}, role string) *Msg {
	m := &Msg{
		ID:        utils.ShortUUID(),
		Name:      name,
		Role:      role,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
	}
	m.SetContent(content)
	return m
}

func (m *Msg) SetContent(content interface{}) {
	switch v := content.(type) {
	case string:
		m.Content = []ContentBlock{NewTextBlock(v)}
	case ContentBlock:
		m.Content = []ContentBlock{v}
	case []ContentBlock:
		m.Content = v
	default:
		m.Content = []ContentBlock{NewTextBlock(fmt.Sprint(v))}
	}
}

func (m *Msg) GetTextContent() string {
	return ContentToString(m.Content)
}

func (m *Msg) GetContentBlocks() []ContentBlock {
	return m.Content
}

func (m *Msg) Clone() *Msg {
	data, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	clone := &Msg{}
	if err := json.Unmarshal(data, clone); err != nil {
		return nil
	}
	return clone
}

func ContentToString(content []ContentBlock) string {
	var result string
	for _, block := range content {
		if IsTextBlock(block) {
			result += GetBlockText(block)
		}
	}
	return result
}
