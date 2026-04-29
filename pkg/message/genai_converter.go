package message

import (
	"encoding/json"
	"fmt"
)

// GenAIPart represents a part in OpenTelemetry GenAI format.
// See: https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/
type GenAIPart map[string]interface{}

// GenAIMessage represents a message in OpenTelemetry GenAI format.
type GenAIMessage struct {
	Role         string       `json:"role"`
	Parts        []GenAIPart  `json:"parts"`
	Name         string       `json:"name,omitempty"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

// ConvertBlockToPart converts a content block to OpenTelemetry GenAI part format.
// This follows the same logic as Python's _convert_block_to_part in _converter.py.
func ConvertBlockToPart(block ContentBlock) GenAIPart {
	blockType := GetBlockType(block)

	switch blockType {
	case BlockText:
		return GenAIPart{
			"type":    "text",
			"content": GetBlockText(block),
		}

	case BlockThinking:
		return GenAIPart{
			"type":    "reasoning",
			"content": GetBlockThinking(block),
		}

	case BlockToolUse:
		return GenAIPart{
			"type":      "tool_call",
			"id":        GetBlockToolUseID(block),
			"name":      GetBlockToolUseName(block),
			"arguments": GetBlockToolUseInput(block),
		}

	case BlockToolResult:
		output := GetBlockToolResultOutput(block)
		var response string
		switch v := output.(type) {
		case string:
			response = v
		default:
			if data, err := json.Marshal(output); err == nil {
				response = string(data)
			} else {
				response = fmt.Sprintf("%v", output)
			}
		}

		return GenAIPart{
			"type":     "tool_call_response",
			"id":       GetBlockToolResultID(block),
			"response": response,
		}

	case BlockImage:
		return convertMediaBlock(block, "image")

	case BlockAudio:
		return convertMediaBlock(block, "audio")

	case BlockVideo:
		return convertMediaBlock(block, "video")

	default:
		return nil
	}
}

// convertMediaBlock converts a media block (image/audio/video) to OpenTelemetry format.
// This follows the same logic as Python's _convert_media_block in _converter.py.
func convertMediaBlock(block ContentBlock, modality string) GenAIPart {
	source, ok := block["source"].(Source)
	if !ok {
		return nil
	}

	sourceType := GetSourceType(source)

	if sourceType == "url" {
		url := GetSourceURL(source)
		return GenAIPart{
			"type":     "uri",
			"uri":      url,
			"modality": modality,
		}
	}

	if sourceType == "base64" {
		data := GetSourceData(source)
		mediaType := GetSourceMediaType(source)
		if mediaType == "" {
			defaultMediaTypes := map[string]string{
				"image": "image/jpeg",
				"audio": "audio/wav",
				"video": "video/mp4",
			}
			mediaType = defaultMediaTypes[modality]
		}
		return GenAIPart{
			"type":       "blob",
			"content":    data,
			"media_type": mediaType,
			"modality":   modality,
		}
	}

	return nil
}

// ConvertMsgToGenAIMessages converts an AgentScope Msg to OpenTelemetry GenAI message format.
// This follows the same logic as Python's _get_agent_messages in _extractor.py.
func ConvertMsgToGenAIMessage(msg *Msg) GenAIMessage {
	parts := make([]GenAIPart, 0)
	for _, block := range msg.GetContentBlocks() {
		part := ConvertBlockToPart(block)
		if part != nil {
			parts = append(parts, part)
		}
	}

	return GenAIMessage{
		Role:         msg.Role,
		Parts:        parts,
		Name:         msg.Name,
		FinishReason: "stop",
	}
}

// ConvertMsgsToGenAIMessages converts multiple AgentScope Msgs to OpenTelemetry GenAI message format.
func ConvertMsgsToGenAIMessages(msgs []*Msg) []GenAIMessage {
	result := make([]GenAIMessage, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, ConvertMsgToGenAIMessage(msg))
	}
	return result
}
