package formatter

import (
	"encoding/json"
	"fmt"

	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)

type OpenAIChatFormatter struct{}

func NewOpenAIChatFormatter() *OpenAIChatFormatter {
	return &OpenAIChatFormatter{}
}

func (f *OpenAIChatFormatter) Format(msgs []*message.Msg) ([]model.FormattedMessage, error) {
	assertListOfMsgs(msgs)
	var result []model.FormattedMessage

	for _, msg := range msgs {
		var contentBlocks []interface{}
		var toolCalls []interface{}
		hasToolResult := false

		for _, block := range msg.GetContentBlocks() {
			typ := message.GetBlockType(block)
			switch typ {
			case message.BlockText:
				contentBlocks = append(contentBlocks, map[string]interface{}{
					"type": "text",
					"text": message.GetBlockText(block),
				})
			case message.BlockThinking:
				// skip thinking blocks for API
			case message.BlockToolUse:
				id := message.GetBlockToolUseID(block)
				name := message.GetBlockToolUseName(block)
				input := message.GetBlockToolUseInput(block)
				argsJSON, _ := json.Marshal(input)
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   id,
					"type": "function",
					"function": map[string]interface{}{
						"name":      name,
						"arguments": string(argsJSON),
					},
				})
			case message.BlockToolResult:
				id := message.GetBlockToolResultID(block)
				output := message.GetBlockToolResultOutput(block)
				textOutput, _ := convertToolResultOutput(output)
				result = append(result, model.FormattedMessage{
					"role":         "tool",
					"tool_call_id": id,
					"content":      textOutput,
				})
				hasToolResult = true
			case message.BlockImage:
				src, _ := block["source"].(message.Source)
				if src == nil {
					continue
				}
				url := formatOpenAIImageURL(src)
				contentBlocks = append(contentBlocks, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": url,
					},
				})
			case message.BlockAudio:
				// skip assistant audio in subsequent calls
				if msg.Role == "assistant" {
					continue
				}
			}
		}

		if hasToolResult {
			continue
		}

		openaiMsg := model.FormattedMessage{
			"role": msg.Role,
			"name": msg.Name,
		}

		if len(toolCalls) > 0 {
			openaiMsg["tool_calls"] = toolCalls
		}

		if len(contentBlocks) > 0 {
			openaiMsg["content"] = contentBlocks
		} else if len(toolCalls) == 0 {
			openaiMsg["content"] = nil
		}

		if openaiMsg["content"] != nil || openaiMsg["tool_calls"] != nil {
			result = append(result, openaiMsg)
		}
	}

	return result, nil
}

func formatOpenAIImageURL(src message.Source) string {
	srcType := message.GetSourceType(src)
	switch srcType {
	case "url":
		return message.GetSourceURL(src)
	case "base64":
		mediaType := message.GetSourceMediaType(src)
		data := message.GetSourceData(src)
		return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
	default:
		return ""
	}
}

type OpenAIMultiAgentFormatter struct {
	HistoryPrompt string
}

func NewOpenAIMultiAgentFormatter() *OpenAIMultiAgentFormatter {
	return &OpenAIMultiAgentFormatter{
		HistoryPrompt: "# Conversation History\nThe content between <history></history> tags contains your conversation history\n",
	}
}

func (f *OpenAIMultiAgentFormatter) Format(msgs []*message.Msg) ([]model.FormattedMessage, error) {
	assertListOfMsgs(msgs)

	var toolMsgs []*message.Msg
	var agentMsgs []*message.Msg

	for _, msg := range msgs {
		hasToolUse := false
		for _, block := range msg.GetContentBlocks() {
			if message.IsToolUseBlock(block) || message.IsToolResultBlock(block) {
				hasToolUse = true
				break
			}
		}
		if hasToolUse {
			toolMsgs = append(toolMsgs, msg)
		} else {
			agentMsgs = append(agentMsgs, msg)
		}
	}

	var result []model.FormattedMessage

	if len(toolMsgs) > 0 {
		chatFmt := NewOpenAIChatFormatter()
		toolResult, err := chatFmt.Format(toolMsgs)
		if err != nil {
			return nil, err
		}
		result = append(result, toolResult...)
	}

	if len(agentMsgs) > 0 {
		var lines []string
		for _, msg := range agentMsgs {
			for _, block := range msg.GetContentBlocks() {
				if message.IsTextBlock(block) {
					lines = append(lines, msg.Name+": "+message.GetBlockText(block))
				}
			}
		}

		if len(lines) > 0 {
			historyText := f.HistoryPrompt + "<history>\n"
			for i, line := range lines {
				if i > 0 {
					historyText += "\n"
				}
				historyText += line
			}
			historyText += "\n</history>"

			result = append(result, model.FormattedMessage{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": historyText,
					},
				},
			})
		}
	}

	return result, nil
}
