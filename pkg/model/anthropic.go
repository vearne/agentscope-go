package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vearne/agentscope-go/pkg/message"
)

type AnthropicChatModel struct {
	modelName  string
	apiKey     string
	baseURL    string
	stream     bool
	httpClient *http.Client
	config     *ModelConfig
}

type AnthropicModelOption func(*AnthropicChatModel)

func NewAnthropicChatModel(modelName, apiKey, baseURL string, stream bool, opts ...AnthropicModelOption) *AnthropicChatModel {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := &AnthropicChatModel{
		modelName: modelName,
		apiKey:    apiKey,
		baseURL:   baseURL,
		stream:    stream,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		config: &ModelConfig{},
	}

	for _, opt := range opts {
		opt(model)
	}

	return model
}

func (m *AnthropicChatModel) ModelName() string { return m.modelName }
func (m *AnthropicChatModel) IsStream() bool    { return m.stream }

func WithAnthropicTemperature(temp float64) AnthropicModelOption {
	return func(m *AnthropicChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.Temperature = &temp
	}
}

func WithAnthropicTopP(topP float64) AnthropicModelOption {
	return func(m *AnthropicChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.TopP = &topP
	}
}

func WithAnthropicTopK(topK int) AnthropicModelOption {
	return func(m *AnthropicChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.TopK = &topK
	}
}

func WithAnthropicMaxTokens(maxTokens int) AnthropicModelOption {
	return func(m *AnthropicChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.MaxTokens = &maxTokens
	}
}

func WithAnthropicStop(stop []string) AnthropicModelOption {
	return func(m *AnthropicChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.Stop = stop
	}
}

func (m *AnthropicChatModel) Call(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (*ChatResponse, error) {
	opt := m.mergeOpts(opts)
	body, systemPrompt := m.buildRequestBody(messages, opt, false)
	if systemPrompt != nil {
		body["system"] = systemPrompt
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/v1/messages", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", m.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return m.parseCompletion(respBody)
}

func (m *AnthropicChatModel) Stream(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (<-chan ChatResponse, error) {
	opt := m.mergeOpts(opts)
	body, systemPrompt := m.buildRequestBody(messages, opt, true)
	if systemPrompt != nil {
		body["system"] = systemPrompt
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/v1/messages", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", m.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan ChatResponse, 64)
	go m.consumeStream(resp.Body, ch)
	return ch, nil
}

func (m *AnthropicChatModel) parseCompletion(data []byte) (*ChatResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse completion: %w", err)
	}

	cr := &ChatResponse{Type: "chat"}
	cr.ID, _ = resp["id"].(string)

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		cr.Usage = &ChatUsage{}
		if it, ok := usage["input_tokens"].(float64); ok {
			cr.Usage.InputTokens = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok {
			cr.Usage.OutputTokens = int(ot)
		}
	}

	content, ok := resp["content"].([]interface{})
	if !ok {
		return cr, nil
	}

	for _, c := range content {
		block, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		typ, _ := block["type"].(string)
		switch typ {
		case "text":
			text, _ := block["text"].(string)
			cr.Content = append(cr.Content, message.NewTextBlock(text))
		case "thinking":
			thinking, _ := block["thinking"].(string)
			cr.Content = append(cr.Content, message.NewThinkingBlock(thinking))
		case "tool_use":
			id, _ := block["id"].(string)
			name, _ := block["name"].(string)
			input, _ := block["input"].(map[string]interface{})
			if input == nil {
				input = map[string]interface{}{}
			}
			cr.Content = append(cr.Content, message.NewToolUseBlock(id, name, input))
		}
	}

	return cr, nil
}

func (m *AnthropicChatModel) consumeStream(body io.ReadCloser, ch chan<- ChatResponse) {
	defer close(ch)
	defer func() { _ = body.Close() }()

	text := ""
	thinking := ""
	var responseID string
	toolInputs := make(map[string]string)

	for event := range ParseSSEStream(body) {
		if event.Data == "" {
			continue
		}

		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(event.Data), &ev); err != nil {
			continue
		}

		evType, _ := ev["type"].(string)
		if id, ok := ev["id"].(string); ok && id != "" {
			responseID = id
		}

		switch evType {
		case "content_block_delta":
			delta, _ := ev["delta"].(map[string]interface{})
			if delta == nil {
				continue
			}
			deltaType, _ := delta["type"].(string)
			switch deltaType {
			case "text_delta":
				t, _ := delta["text"].(string)
				text += t
			case "thinking_delta":
				th, _ := delta["thinking"].(string)
				thinking += th
			case "input_json_delta":
				index := -1
				if idx, ok := ev["index"].(float64); ok {
					index = int(idx)
				}
				partial, _ := delta["partial_json"].(string)
				key := fmt.Sprintf("%d", index)
				toolInputs[key] += partial
			}

			var blocks []message.ContentBlock
			if thinking != "" {
				blocks = append(blocks, message.NewThinkingBlock(thinking))
			}
			if text != "" {
				blocks = append(blocks, message.NewTextBlock(text))
			}

			for i := 0; i < len(toolInputs)+1; i++ {
				key := fmt.Sprintf("%d", i)
				if inputStr, ok := toolInputs[key]; ok && inputStr != "" {
					var input interface{}
					_ = json.Unmarshal([]byte(inputStr), &input)
					if input == nil {
						input = map[string]interface{}{}
					}
					blocks = append(blocks, message.NewToolUseBlock("", "", input))
				}
			}

			if len(blocks) > 0 {
				ch <- ChatResponse{ID: responseID, Content: blocks, Type: "chat"}
			}

		case "message_start":
			// initial event, extract usage if present
			if msg, ok := ev["message"].(map[string]interface{}); ok {
				if usage, ok := msg["usage"].(map[string]interface{}); ok {
					it, _ := usage["input_tokens"].(float64)
					cr := ChatResponse{
						ID:   responseID,
						Type: "chat",
						Usage: &ChatUsage{
							InputTokens: int(it),
						},
					}
					ch <- cr
				}
			}

		case "message_delta":
			delta, _ := ev["delta"].(map[string]interface{})
			stopReason, _ := delta["stop_reason"].(string)
			if stopReason != "" || ev["type"] == "message_stop" {
				usage, _ := ev["usage"].(map[string]interface{})
				var chatUsage *ChatUsage
				if usage != nil {
					ot, _ := usage["output_tokens"].(float64)
					chatUsage = &ChatUsage{OutputTokens: int(ot)}
				}

				var blocks []message.ContentBlock
				if thinking != "" {
					blocks = append(blocks, message.NewThinkingBlock(thinking))
				}
				if text != "" {
					blocks = append(blocks, message.NewTextBlock(text))
				}

				if len(blocks) > 0 || chatUsage != nil {
					ch <- ChatResponse{ID: responseID, Content: blocks, Usage: chatUsage, Type: "chat"}
				}
			}
		}
	}
}

func (m *AnthropicChatModel) buildRequestBody(messages []FormattedMessage, opt CallOption, stream bool) (map[string]interface{}, interface{}) {
	var systemContent interface{}
	var filtered []FormattedMessage

	for _, msg := range messages {
		if role, ok := msg["role"].(string); ok && role == "system" {
			if content, ok := msg["content"].(string); ok {
				systemContent = content
			} else if contentList, ok := msg["content"].([]interface{}); ok {
				systemContent = contentList
			}
			continue
		}
		filtered = append(filtered, msg)
	}

	if filtered == nil {
		filtered = []FormattedMessage{}
	}

	body := map[string]interface{}{
		"model":    m.modelName,
		"messages": filtered,
		"stream":   stream,
	}

	maxTokens := configValueInt(m.config.MaxTokens, opt.MaxTokens)
	if maxTokens != nil {
		body["max_tokens"] = *maxTokens
	} else {
		body["max_tokens"] = 4096
	}

	temp := configValueFloat64(m.config.Temperature, opt.Temperature)
	if temp != nil {
		body["temperature"] = *temp
	}

	topP := configValueFloat64(m.config.TopP, opt.TopP)
	if topP != nil {
		body["top_p"] = *topP
	}

	topK := configValueInt(m.config.TopK, opt.TopK)
	if topK != nil {
		body["top_k"] = *topK
	}

	stop := m.mergeStopSequences(opt.Stop)
	if len(stop) > 0 {
		body["stop_sequences"] = stop
	}

	if len(opt.Tools) > 0 {
		body["tools"] = formatAnthropicTools(opt.Tools)
	}
	if opt.ToolChoice != "" {
		body["tool_choice"] = formatAnthropicToolChoice(opt.ToolChoice)
	}

	return body, systemContent
}

func (m *AnthropicChatModel) mergeOpts(opts []CallOption) CallOption {
	var opt CallOption
	if len(opts) > 0 {
		opt = opts[0]
	}
	return opt
}

func formatAnthropicTools(schemas []ToolSchema) []map[string]interface{} {
	result := make([]map[string]interface{}, len(schemas))
	for i, s := range schemas {
		result[i] = map[string]interface{}{
			"name":        s.Function.Name,
			"description": s.Function.Description,
			"input_schema": s.Function.Parameters,
		}
	}
	return result
}

func formatAnthropicToolChoice(choice string) interface{} {
	switch choice {
	case "auto":
		return map[string]interface{}{"type": "auto"}
	case "none":
		return map[string]interface{}{"type": "none"}
	case "required":
		return map[string]interface{}{"type": "any"}
	default:
		return map[string]interface{}{
			"type": "tool",
			"name": choice,
		}
	}
}

func (m *AnthropicChatModel) mergeStopSequences(optStop []string) []string {
	result := []string{}
	if m.config != nil {
		result = append(result, m.config.Stop...)
	}
	result = append(result, optStop...)
	return result
}
