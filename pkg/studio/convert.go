package studio

import (
	"github.com/vearne/agentscope-go/pkg/message"
)

// MsgToPayload converts a Go Msg to a studio-compatible map matching
// Python agentscope's Msg.to_dict() format.
func MsgToPayload(msg *message.Msg) map[string]interface{} {
	if msg == nil {
		return nil
	}

	return map[string]interface{}{
		"id":        msg.ID,
		"name":      msg.Name,
		"role":      msg.Role,
		"content":   convertContent(msg.Content),
		"metadata":  ensureMetadata(msg.Metadata),
		"timestamp": msg.Timestamp,
	}
}

// convertContent transforms ContentBlocks into the format studio expects.
// Single text-only content is flattened to a string (matching Python behavior).
// All other content is passed through as a slice of maps.
func convertContent(blocks []message.ContentBlock) interface{} {
	if len(blocks) == 1 && message.IsTextBlock(blocks[0]) {
		return message.GetBlockText(blocks[0])
	}

	result := make([]map[string]interface{}, len(blocks))
	for i, block := range blocks {
		// ContentBlock is map[string]interface{}, pass through directly
		result[i] = map[string]interface{}(block)
	}
	return result
}

// ensureMetadata returns an empty map if metadata is nil,
// matching Python's default empty dict behavior.
func ensureMetadata(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}
