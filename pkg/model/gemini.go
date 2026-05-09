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

type GeminiChatModel struct {
	modelName  string
	apiKey     string
	baseURL    string
	stream     bool
	httpClient *http.Client
	config     *ModelConfig
}

type GeminiModelOption func(*GeminiChatModel)

func NewGeminiChatModel(modelName, apiKey, baseURL string, stream bool, opts ...GeminiModelOption) *GeminiChatModel {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := &GeminiChatModel{
		modelName: modelName,
		apiKey:    apiKey,
		baseURL:   baseURL,
		stream:    stream,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(model)
	}

	return model
}

func (m *GeminiChatModel) ModelName() string { return m.modelName }
func (m *GeminiChatModel) IsStream() bool    { return m.stream }

func WithGeminiTemperature(temp float64) GeminiModelOption {
	return func(m *GeminiChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.Temperature = &temp
	}
}

func WithGeminiTopP(topP float64) GeminiModelOption {
	return func(m *GeminiChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.TopP = &topP
	}
}

func WithGeminiTopK(topK int) GeminiModelOption {
	return func(m *GeminiChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.TopK = &topK
	}
}

func WithGeminiMaxTokens(maxTokens int) GeminiModelOption {
	return func(m *GeminiChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.MaxTokens = &maxTokens
	}
}

func WithGeminiStop(stop []string) GeminiModelOption {
	return func(m *GeminiChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.Stop = stop
	}
}

func (m *GeminiChatModel) Call(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (*ChatResponse, error) {
	opt := m.mergeOpts(opts)
	body := m.buildRequestBody(messages, opt)

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", m.baseURL, m.modelName, m.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

func (m *GeminiChatModel) Stream(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (<-chan ChatResponse, error) {
	opt := m.mergeOpts(opts)
	body := m.buildRequestBody(messages, opt)

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", m.baseURL, m.modelName, m.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

func (m *GeminiChatModel) parseCompletion(data []byte) (*ChatResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse completion: %w", err)
	}

	cr := &ChatResponse{Type: "chat"}

	if usage, ok := resp["usageMetadata"].(map[string]interface{}); ok {
		cr.Usage = &ChatUsage{}
		if pt, ok := usage["promptTokenCount"].(float64); ok {
			cr.Usage.InputTokens = int(pt)
		}
		if ct, ok := usage["candidatesTokenCount"].(float64); ok {
			cr.Usage.OutputTokens = int(ct)
		}
	}

	candidates, ok := resp["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return cr, nil
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return cr, nil
	}

	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return cr, nil
	}

	parts, ok := content["parts"].([]interface{})
	if !ok {
		return cr, nil
	}

	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := part["text"].(string); ok {
			cr.Content = append(cr.Content, message.NewTextBlock(text))
		}
		if fnCall, ok := part["functionCall"].(map[string]interface{}); ok {
			name, _ := fnCall["name"].(string)
			args, _ := fnCall["args"].(map[string]interface{})
			if args == nil {
				args = map[string]interface{}{}
			}
			cr.Content = append(cr.Content, message.NewToolUseBlock("", name, args))
		}
		if thought, ok := part["thought"].(bool); ok && thought {
			if text, ok := part["text"].(string); ok {
				cr.Content = append(cr.Content, message.NewThinkingBlock(text))
			}
		}
	}

	return cr, nil
}

func (m *GeminiChatModel) consumeStream(body io.ReadCloser, ch chan<- ChatResponse) {
	defer close(ch)
	defer func() { _ = body.Close() }()

	text := ""
	thinking := ""

	for event := range ParseSSEStream(body) {
		if event.Data == "" {
			continue
		}

		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(event.Data), &resp); err != nil {
			continue
		}

		candidates, ok := resp["candidates"].([]interface{})
		if !ok || len(candidates) == 0 {
			continue
		}

		candidate, ok := candidates[0].(map[string]interface{})
		if !ok {
			continue
		}

		content, ok := candidate["content"].(map[string]interface{})
		if !ok {
			continue
		}

		parts, ok := content["parts"].([]interface{})
		if !ok {
			continue
		}

		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			if t, ok := part["text"].(string); ok {
				if thought, ok := part["thought"].(bool); ok && thought {
					thinking += t
				} else {
					text += t
				}
			}
			if fnCall, ok := part["functionCall"].(map[string]interface{}); ok {
				name, _ := fnCall["name"].(string)
				args, _ := fnCall["args"].(map[string]interface{})
				if args == nil {
					args = map[string]interface{}{}
				}
				var blocks []message.ContentBlock
				if thinking != "" {
					blocks = append(blocks, message.NewThinkingBlock(thinking))
				}
				if text != "" {
					blocks = append(blocks, message.NewTextBlock(text))
				}
				blocks = append(blocks, message.NewToolUseBlock("", name, args))
				ch <- ChatResponse{Content: blocks, Type: "chat"}
			}
		}

		var blocks []message.ContentBlock
		if thinking != "" {
			blocks = append(blocks, message.NewThinkingBlock(thinking))
		}
		if text != "" {
			blocks = append(blocks, message.NewTextBlock(text))
		}
		if len(blocks) > 0 {
			ch <- ChatResponse{Content: blocks, Type: "chat"}
		}
	}
}

func (m *GeminiChatModel) buildRequestBody(messages []FormattedMessage, opt CallOption) map[string]interface{} {
	var systemInstruction interface{}
	var contents []interface{}

	for _, msg := range messages {
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			if content, ok := msg["content"].(string); ok {
				systemInstruction = map[string]interface{}{
					"parts": []interface{}{
						map[string]interface{}{"text": content},
					},
				}
			}
		case "user":
			parts := geminiContentParts(msg)
			contents = append(contents, map[string]interface{}{
				"role":  "user",
				"parts": parts,
			})
		case "assistant":
			parts := geminiContentParts(msg)
			contents = append(contents, map[string]interface{}{
				"role":  "model",
				"parts": parts,
			})
		}
	}

	body := map[string]interface{}{
		"contents": contents,
	}

	if systemInstruction != nil {
		body["systemInstruction"] = systemInstruction
	}

	generationConfig := map[string]interface{}{}

	temp := configValueFloat64(m.config.Temperature, opt.Temperature)
	if temp != nil {
		generationConfig["temperature"] = *temp
	}

	topP := configValueFloat64(m.config.TopP, opt.TopP)
	if topP != nil {
		generationConfig["topP"] = *topP
	}

	topK := configValueInt(m.config.TopK, opt.TopK)
	if topK != nil {
		generationConfig["topK"] = *topK
	}

	maxTokens := configValueInt(m.config.MaxTokens, opt.MaxTokens)
	if maxTokens != nil {
		generationConfig["maxOutputTokens"] = *maxTokens
	}

	stop := m.mergeStopSequences(opt.Stop)
	if len(stop) > 0 {
		generationConfig["stopSequences"] = stop
	}

	if len(generationConfig) > 0 {
		body["generationConfig"] = generationConfig
	}

	if len(opt.Tools) > 0 {
		body["tools"] = formatGeminiTools(opt.Tools)
	}
	if opt.ToolChoice != "" {
		if gc, ok := body["generationConfig"].(map[string]interface{}); ok {
			gc["tool_config"] = formatGeminiToolConfig(opt.ToolChoice, opt.Tools)
		} else {
			body["generationConfig"] = map[string]interface{}{
				"tool_config": formatGeminiToolConfig(opt.ToolChoice, opt.Tools),
			}
		}
	}

	return body
}

func geminiContentParts(msg FormattedMessage) []interface{} {
	var parts []interface{}

	if content, ok := msg["content"].(string); ok {
		parts = append(parts, map[string]interface{}{"text": content})
		return parts
	}

	if contentList, ok := msg["content"].([]interface{}); ok {
		for _, c := range contentList {
			if block, ok := c.(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					parts = append(parts, map[string]interface{}{"text": text})
				}
			}
		}
	}

	if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			if tcMap, ok := tc.(map[string]interface{}); ok {
				fn, _ := tcMap["function"].(map[string]interface{})
				name, _ := fn["name"].(string)
				argsStr, _ := fn["arguments"].(string)
				var args interface{}
				if argsStr != "" {
					_ = json.Unmarshal([]byte(argsStr), &args)
				}
				if args == nil {
					args = map[string]interface{}{}
				}
				parts = append(parts, map[string]interface{}{
					"functionCall": map[string]interface{}{
						"name": name,
						"args": args,
					},
				})
			}
		}
	}

	if len(parts) == 0 {
		parts = append(parts, map[string]interface{}{"text": ""})
	}
	return parts
}

func formatGeminiTools(schemas []ToolSchema) []interface{} {
	declarations := make([]interface{}, len(schemas))
	for i, s := range schemas {
		declarations[i] = map[string]interface{}{
			"name":        s.Function.Name,
			"description": s.Function.Description,
			"parameters":  s.Function.Parameters,
		}
	}
	return []interface{}{
		map[string]interface{}{
			"function_declarations": declarations,
		},
	}
}

func formatGeminiToolConfig(choice string, tools []ToolSchema) map[string]interface{} {
	switch choice {
	case "auto":
		return map[string]interface{}{
			"function_calling_config": map[string]interface{}{
				"mode": "AUTO",
			},
		}
	case "none":
		return map[string]interface{}{
			"function_calling_config": map[string]interface{}{
				"mode": "NONE",
			},
		}
	case "required":
		return map[string]interface{}{
			"function_calling_config": map[string]interface{}{
				"mode": "ANY",
			},
		}
	default:
		return map[string]interface{}{
			"function_calling_config": map[string]interface{}{
				"mode":              "ANY",
				"allowed_function_names": []string{choice},
			},
		}
	}
}

func (m *GeminiChatModel) mergeOpts(opts []CallOption) CallOption {
	var opt CallOption
	if len(opts) > 0 {
		opt = opts[0]
	}
	return opt
}

func (m *GeminiChatModel) mergeStopSequences(optStop []string) []string {
	result := []string{}
	if m.config != nil {
		result = append(result, m.config.Stop...)
	}
	result = append(result, optStop...)
	return result
}
