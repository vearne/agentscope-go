package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/vearne/agentscope-go/pkg/message"
)

type SSEEvent struct {
	Data  string
	Event string
	ID    string
}

func ParseSSEStream(reader io.Reader) <-chan SSEEvent {
	ch := make(chan SSEEvent, 64)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var currentEvent SSEEvent
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if currentEvent.Data != "" {
					ch <- currentEvent
					currentEvent = SSEEvent{}
				}
				continue
			}
			if len(line) > 6 && line[:6] == "event:" {
				currentEvent.Event = trimSSE(line[6:])
			} else if len(line) > 5 && line[:5] == "data:" {
				currentEvent.Data = trimSSE(line[5:])
			} else if len(line) > 3 && line[:3] == "id:" {
				currentEvent.ID = trimSSE(line[3:])
			}
		}
		if currentEvent.Data != "" {
			ch <- currentEvent
		}
	}()
	return ch
}

func trimSSE(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}

func ParseOpenAIStreamChunk(data string) (responseID string, deltaContent *string, deltaThinking *string, toolCalls []map[string]interface{}, usage *ChatUsage, done bool, err error) {
	if data == "[DONE]" {
		return "", nil, nil, nil, nil, true, nil
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", nil, nil, nil, nil, false, fmt.Errorf("parse chunk: %w", err)
	}

	responseID, _ = chunk["id"].(string)

	if u, ok := chunk["usage"].(map[string]interface{}); ok {
		usage = &ChatUsage{}
		if pt, ok := u["prompt_tokens"].(float64); ok {
			usage.InputTokens = int(pt)
		}
		if ct, ok := u["completion_tokens"].(float64); ok {
			usage.OutputTokens = int(ct)
		}
	}

	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return responseID, nil, nil, nil, usage, false, nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return responseID, nil, nil, nil, usage, false, nil
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return responseID, nil, nil, nil, usage, false, nil
	}

	if c, ok := delta["content"].(string); ok && c != "" {
		deltaContent = &c
	}

	if rc, ok := delta["reasoning_content"].(string); ok && rc != "" {
		deltaThinking = &rc
	}
	if rc, ok := delta["reasoning"].(string); ok && rc != "" {
		deltaThinking = &rc
	}

	if tcs, ok := delta["tool_calls"].([]interface{}); ok {
		for _, tc := range tcs {
			if tcMap, ok := tc.(map[string]interface{}); ok {
				toolCalls = append(toolCalls, tcMap)
			}
		}
	}

	return responseID, deltaContent, deltaThinking, toolCalls, usage, false, nil
}

func ParseOpenAICompletion(data []byte) (*ChatResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse completion: %w", err)
	}

	cr := &ChatResponse{Type: "chat"}
	cr.ID, _ = resp["id"].(string)

	if u, ok := resp["usage"].(map[string]interface{}); ok {
		cr.Usage = &ChatUsage{}
		if pt, ok := u["prompt_tokens"].(float64); ok {
			cr.Usage.InputTokens = int(pt)
		}
		if ct, ok := u["completion_tokens"].(float64); ok {
			cr.Usage.OutputTokens = int(ct)
		}
	}

	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return cr, nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return cr, nil
	}

	msg, ok := choice["message"].(map[string]interface{})
	if !ok {
		return cr, nil
	}

	if rc, ok := msg["reasoning_content"].(string); ok && rc != "" {
		cr.Content = append(cr.Content, message.NewThinkingBlock(rc))
	}
	if rc, ok := msg["reasoning"].(string); ok && rc != "" {
		cr.Content = append(cr.Content, message.NewThinkingBlock(rc))
	}

	if content, ok := msg["content"].(string); ok && content != "" {
		cr.Content = append(cr.Content, message.NewTextBlock(content))
	}

	if tcs, ok := msg["tool_calls"].([]interface{}); ok {
		for _, tc := range tcs {
			tcMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := tcMap["id"].(string)
			fn, _ := tcMap["function"].(map[string]interface{})
			name, _ := fn["name"].(string)
			argsStr, _ := fn["arguments"].(string)

			var input interface{}
			if argsStr != "" {
				_ = json.Unmarshal([]byte(argsStr), &input)
			}
			if input == nil {
				input = map[string]interface{}{}
			}
			cr.Content = append(cr.Content, message.NewToolUseBlock(id, name, input))
		}
	}

	return cr, nil
}

func ExtractTextFromDeltaContent(buffers map[string]string, key string, newContent string) string {
	buffers[key] += newContent
	return buffers[key]
}

func BuildStreamResponse(responseID string, text, thinking string, toolCalls []map[string]interface{}, audioAccum *string, usage *ChatUsage, metadata map[string]interface{}) *ChatResponse {
	var blocks []message.ContentBlock

	if thinking != "" {
		blocks = append(blocks, message.NewThinkingBlock(thinking))
	}
	if text != "" {
		blocks = append(blocks, message.NewTextBlock(text))
	}
	if audioAccum != nil && *audioAccum != "" {
		blocks = append(blocks, message.NewAudioBlock(message.NewBase64Source("audio/wav", *audioAccum)))
	}

	for _, tc := range toolCalls {
		id, _ := tc["id"].(string)
		name, _ := tc["name"].(string)
		inputStr, _ := tc["input"].(string)
		var input interface{}
		if inputStr != "" {
			decoder := json.NewDecoder(bytes.NewReader([]byte(inputStr)))
			decoder.UseNumber()
			_ = decoder.Decode(&input)
		}
		if input == nil {
			input = map[string]interface{}{}
		}
		blocks = append(blocks, message.NewToolUseBlock(id, name, input))
	}

	if len(blocks) == 0 {
		return nil
	}

	return &ChatResponse{
		ID:       responseID,
		Content:  blocks,
		Usage:    usage,
		Metadata: metadata,
		Type:     "chat",
	}
}
