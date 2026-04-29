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

	"github.com/vearne/agentscope-go/internal/utils"
)

type OpenAIChatModel struct {
	modelName  string
	apiKey     string
	baseURL    string
	stream     bool
	httpClient *http.Client
	config     *ModelConfig
}

type OpenAIModelOption func(*OpenAIChatModel)

func NewOpenAIChatModel(modelName, apiKey, baseURL string, stream bool, opts ...OpenAIModelOption) *OpenAIChatModel {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := &OpenAIChatModel{
		modelName:  modelName,
		apiKey:     apiKey,
		baseURL:    baseURL,
		stream:     stream,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(model)
	}

	return model
}

func (m *OpenAIChatModel) ModelName() string { return m.modelName }
func (m *OpenAIChatModel) IsStream() bool    { return m.stream }

func WithTemperature(temp float64) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.Temperature = &temp
	}
}

func WithTopP(topP float64) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.TopP = &topP
	}
}

func WithMaxTokens(maxTokens int) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.MaxTokens = &maxTokens
	}
}

func WithStop(stop []string) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.Stop = stop
	}
}

func WithPresencePenalty(penalty float64) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.PresencePenalty = &penalty
	}
}

func WithFrequencyPenalty(penalty float64) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.FrequencyPenalty = &penalty
	}
}

func WithSeed(seed int) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.Seed = &seed
	}
}

func WithResponseFormat(formatType string) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.ResponseFormat = &ResponseFormat{Type: formatType}
	}
}

func WithUser(user string) OpenAIModelOption {
	return func(m *OpenAIChatModel) {
		if m.config == nil {
			m.config = &ModelConfig{}
		}
		m.config.User = &user
	}
}

func (m *OpenAIChatModel) Call(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (*ChatResponse, error) {
	opt := m.mergeOpts(opts)
	if opt.ToolChoice != "" {
		validateToolChoice(opt.ToolChoice, opt.Tools)
	}

	body := m.buildRequestBody(messages, opt, false)
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return ParseOpenAICompletion(respBody)
}

func (m *OpenAIChatModel) Stream(ctx context.Context, messages []FormattedMessage, opts ...CallOption) (<-chan ChatResponse, error) {
	opt := m.mergeOpts(opts)
	if opt.ToolChoice != "" {
		validateToolChoice(opt.ToolChoice, opt.Tools)
	}

	body := m.buildRequestBody(messages, opt, true)
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan ChatResponse, 64)
	go m.consumeStream(resp.Body, ch)
	return ch, nil
}

func (m *OpenAIChatModel) consumeStream(body io.ReadCloser, ch chan<- ChatResponse) {
	defer close(ch)
	defer body.Close()

	text := ""
	thinking := ""
	toolCallsMap := make(map[int]map[string]interface{})
	var responseID string
	var lastUsage *ChatUsage

	for event := range ParseSSEStream(body) {
		if event.Data == "" || event.Data == "[DONE]" {
			break
		}

		rid, deltaContent, deltaThinking, tcDeltas, usage, done, err := ParseOpenAIStreamChunk(event.Data)
		if err != nil {
			continue
		}
		if done {
			break
		}
		if rid != "" {
			responseID = rid
		}

		if deltaContent != nil {
			text += *deltaContent
		}
		if deltaThinking != nil {
			thinking += *deltaThinking
		}
		if usage != nil {
			lastUsage = usage
		}

		for _, tc := range tcDeltas {
			idx := -1
			if v, ok := tc["index"].(float64); ok {
				idx = int(v)
			}
			if idx < 0 {
				continue
			}
			if existing, ok := toolCallsMap[idx]; ok {
				if args, ok := tc["function"].(map[string]interface{}); ok {
					if a, ok := args["arguments"].(string); ok {
						existing["input"] = existing["input"].(string) + a
					}
				}
			} else {
				entry := map[string]interface{}{
					"type":  "tool_use",
					"id":    "",
					"name":  "",
					"input": "",
				}
				if id, ok := tc["id"].(string); ok {
					entry["id"] = id
				}
				if fn, ok := tc["function"].(map[string]interface{}); ok {
					if n, ok := fn["name"].(string); ok {
						entry["name"] = n
					}
					if a, ok := fn["arguments"].(string); ok {
						entry["input"] = a
					}
				}
				toolCallsMap[idx] = entry
			}
		}

		var toolCallList []map[string]interface{}
		for i := 0; i < len(toolCallsMap); i++ {
			if tc, ok := toolCallsMap[i]; ok {
				toolCallList = append(toolCallList, tc)
			}
		}

		cr := BuildStreamResponse(responseID, text, thinking, toolCallList, nil, lastUsage, nil)
		if cr != nil {
			cr.ID = utils.ShortUUID()
			ch <- *cr
		}
	}

	var toolCallList []map[string]interface{}
	for i := 0; i < len(toolCallsMap); i++ {
		if tc, ok := toolCallsMap[i]; ok {
			toolCallList = append(toolCallList, tc)
		}
	}
	final := BuildStreamResponse(responseID, text, thinking, toolCallList, nil, lastUsage, nil)
	if final != nil {
		ch <- *final
	}
}

func (m *OpenAIChatModel) buildRequestBody(messages []FormattedMessage, opt CallOption, stream bool) map[string]interface{} {
	body := map[string]interface{}{
		"model":    m.modelName,
		"messages": messages,
		"stream":   stream,
	}

	if stream {
		body["stream_options"] = map[string]interface{}{"include_usage": true}
	}

	temp := configValueFloat64(m.config.Temperature, opt.Temperature)
	if temp != nil {
		body["temperature"] = *temp
	}

	topP := configValueFloat64(m.config.TopP, opt.TopP)
	if topP != nil {
		body["top_p"] = *topP
	}

	maxTokens := configValueInt(m.config.MaxTokens, opt.MaxTokens)
	if maxTokens != nil {
		body["max_tokens"] = *maxTokens
	}

	stop := m.mergeStopSequences(opt.Stop)
	if len(stop) > 0 {
		body["stop"] = stop
	}

	presencePenalty := configValueFloat64(m.config.PresencePenalty, opt.PresencePenalty)
	if presencePenalty != nil {
		body["presence_penalty"] = *presencePenalty
	}

	frequencyPenalty := configValueFloat64(m.config.FrequencyPenalty, opt.FrequencyPenalty)
	if frequencyPenalty != nil {
		body["frequency_penalty"] = *frequencyPenalty
	}

	seed := configValueInt(m.config.Seed, opt.Seed)
	if seed != nil {
		body["seed"] = *seed
	}

	responseFormat := configValueResponseFormat(m.config.ResponseFormat, opt.ResponseFormat)
	if responseFormat != nil {
		body["response_format"] = responseFormat
	}

	user := configValueString(m.config.User, opt.User)
	if user != nil {
		body["user"] = *user
	}

	if len(opt.Tools) > 0 {
		body["tools"] = formatOpenAITools(opt.Tools)
	}
	if opt.ToolChoice != "" {
		body["tool_choice"] = formatOpenAIToolChoice(opt.ToolChoice)
	}

	return body
}

func (m *OpenAIChatModel) mergeOpts(opts []CallOption) CallOption {
	var opt CallOption
	if len(opts) > 0 {
		opt = opts[0]
	}
	return opt
}

func formatOpenAITools(schemas []ToolSchema) []map[string]interface{} {
	result := make([]map[string]interface{}, len(schemas))
	for i, s := range schemas {
		result[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        s.Function.Name,
				"description": s.Function.Description,
				"parameters":  s.Function.Parameters,
			},
		}
	}
	return result
}

func formatOpenAIToolChoice(choice string) interface{} {
	switch choice {
	case "auto", "none", "required":
		return choice
	default:
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": choice,
			},
		}
	}
}

func (m *OpenAIChatModel) mergeStopSequences(optStop []string) []string {
	result := []string{}
	if m.config != nil {
		result = append(result, m.config.Stop...)
	}
	result = append(result, optStop...)
	return result
}
