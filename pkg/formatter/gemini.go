package formatter

import (
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)

type GeminiChatFormatter struct{}

func NewGeminiChatFormatter() *GeminiChatFormatter {
	return &GeminiChatFormatter{}
}

func (f *GeminiChatFormatter) Format(msgs []*message.Msg) ([]model.FormattedMessage, error) {
	assertListOfMsgs(msgs)
	var result []model.FormattedMessage

	for _, msg := range msgs {
		switch msg.Role {
		case "system":
			result = append(result, model.FormattedMessage{
				"role":    "system",
				"content": msg.GetTextContent(),
			})
		case "user":
			parts := geminiPartsFromMsg(msg)
			result = append(result, model.FormattedMessage{
				"role":    "user",
				"content": parts,
			})
		case "assistant":
			parts := geminiPartsFromMsg(msg)
			result = append(result, model.FormattedMessage{
				"role":    "assistant",
				"content": parts,
			})
		}
	}

	return result, nil
}

func geminiPartsFromMsg(msg *message.Msg) interface{} {
	var parts []interface{}

	for _, block := range msg.GetContentBlocks() {
		typ := message.GetBlockType(block)
		switch typ {
		case message.BlockText:
			parts = append(parts, map[string]interface{}{
				"text": message.GetBlockText(block),
			})
		case message.BlockThinking:
		case message.BlockToolUse:
			name := message.GetBlockToolUseName(block)
			input := message.GetBlockToolUseInput(block)
			parts = append(parts, map[string]interface{}{
				"functionCall": map[string]interface{}{
					"name": name,
					"args": input,
				},
			})
		case message.BlockToolResult:
			id := message.GetBlockToolResultID(block)
			output := message.GetBlockToolResultOutput(block)
			var response interface{}
			switch v := output.(type) {
			case string:
				response = map[string]interface{}{"text": v}
			default:
				response = output
			}
			parts = append(parts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name":     id,
					"response": response,
				},
			})
		case message.BlockImage:
			src, _ := block["source"].(message.Source)
			if src == nil {
				continue
			}
			srcType := message.GetSourceType(src)
			if srcType == "base64" {
				parts = append(parts, map[string]interface{}{
					"inlineData": map[string]interface{}{
						"mimeType": message.GetSourceMediaType(src),
						"data":     message.GetSourceData(src),
					},
				})
			} else if srcType == "url" {
				parts = append(parts, map[string]interface{}{
					"fileData": map[string]interface{}{
						"fileUri": message.GetSourceURL(src),
					},
				})
			}
		}
	}

	if len(parts) == 0 {
		return []interface{}{map[string]interface{}{"text": ""}}
	}
	return parts
}

type GeminiMultiAgentFormatter struct{}

func NewGeminiMultiAgentFormatter() *GeminiMultiAgentFormatter {
	return &GeminiMultiAgentFormatter{}
}

func (f *GeminiMultiAgentFormatter) Format(msgs []*message.Msg) ([]model.FormattedMessage, error) {
	chatFmt := NewGeminiChatFormatter()
	return chatFmt.Format(msgs)
}
