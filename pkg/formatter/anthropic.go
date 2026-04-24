package formatter

import (
	"encoding/json"

	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)

type AnthropicChatFormatter struct{}

func NewAnthropicChatFormatter() *AnthropicChatFormatter {
	return &AnthropicChatFormatter{}
}

func (f *AnthropicChatFormatter) Format(msgs []*message.Msg) ([]model.FormattedMessage, error) {
	assertListOfMsgs(msgs)
	var result []model.FormattedMessage

	for _, msg := range msgs {
		if msg.Role == "system" {
			result = append(result, model.FormattedMessage{
				"role":    "system",
				"content": formatAnthropicContent(msg),
			})
			continue
		}

		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}

		var content []interface{}
		var toolUseBlocks []interface{}

		for _, block := range msg.GetContentBlocks() {
			typ := message.GetBlockType(block)
			switch typ {
			case message.BlockText:
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": message.GetBlockText(block),
				})
			case message.BlockThinking:
				// skip thinking blocks
			case message.BlockToolUse:
				id := message.GetBlockToolUseID(block)
				name := message.GetBlockToolUseName(block)
				input := message.GetBlockToolUseInput(block)
				toolUseBlocks = append(toolUseBlocks, map[string]interface{}{
					"type":  "tool_use",
					"id":    id,
					"name":  name,
					"input": input,
				})
			case message.BlockToolResult:
				id := message.GetBlockToolResultID(block)
				rawOutput := message.GetBlockToolResultOutput(block)
				output := outputToBlocks(rawOutput)
				result = append(result, model.FormattedMessage{
					"role":    "user",
					"content": []interface{}{map[string]interface{}{
						"type":           "tool_result",
						"tool_use_id":    id,
						"content":        output,
					}},
				})
			case message.BlockImage:
				src, _ := block["source"].(message.Source)
				if src == nil {
					continue
				}
				srcType := message.GetSourceType(src)
				imgContent := map[string]interface{}{
					"type": "image",
				}
				if srcType == "base64" {
					imgContent["source"] = map[string]interface{}{
						"type":       "base64",
						"media_type": message.GetSourceMediaType(src),
						"data":       message.GetSourceData(src),
					}
				} else if srcType == "url" {
					imgContent["source"] = map[string]interface{}{
						"type": "url",
						"url":  message.GetSourceURL(src),
					}
				}
				content = append(content, imgContent)
			}
		}

		allContent := append(content, toolUseBlocks...)

		if len(allContent) > 0 {
			result = append(result, model.FormattedMessage{
				"role":    role,
				"content": allContent,
			})
		}
	}

	return result, nil
}

func formatAnthropicContent(msg *message.Msg) interface{} {
	var parts []interface{}
	for _, block := range msg.GetContentBlocks() {
		if message.IsTextBlock(block) {
			parts = append(parts, map[string]interface{}{
				"type": "text",
				"text": message.GetBlockText(block),
			})
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 0 {
		return ""
	}
	return parts
}

func outputToBlocks(output interface{}) interface{} {
	switch v := output.(type) {
	case string:
		return v
	case []message.ContentBlock:
		var parts []interface{}
		for _, b := range v {
			parts = append(parts, b)
		}
		return parts
	default:
		b, _ := json.Marshal(output)
		return string(b)
	}
}

type AnthropicMultiAgentFormatter struct {
	HistoryPrompt string
}

func NewAnthropicMultiAgentFormatter() *AnthropicMultiAgentFormatter {
	return &AnthropicMultiAgentFormatter{
		HistoryPrompt: "# Conversation History\n",
	}
}

func (f *AnthropicMultiAgentFormatter) Format(msgs []*message.Msg) ([]model.FormattedMessage, error) {
	chatFmt := NewAnthropicChatFormatter()
	return chatFmt.Format(msgs)
}
