package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const defaultMaxIters = 10

type ReActOption func(*ReActAgent)

func WithReActName(name string) ReActOption {
	return func(a *ReActAgent) { a.name = name }
}

func WithReActModel(m model.ChatModelBase) ReActOption {
	return func(a *ReActAgent) { a.model = m }
}

func WithReActMemory(mem memory.MemoryBase) ReActOption {
	return func(a *ReActAgent) { a.mem = mem }
}

func WithReActFormatter(f formatter.FormatterBase) ReActOption {
	return func(a *ReActAgent) { a.fmt = f }
}

func WithReActToolkit(tk *tool.Toolkit) ReActOption {
	return func(a *ReActAgent) { a.toolkit = tk }
}

func WithReActMaxIters(n int) ReActOption {
	return func(a *ReActAgent) { a.maxIters = n }
}

func WithReActSystemPrompt(prompt string) ReActOption {
	return func(a *ReActAgent) { a.sysPrompt = prompt }
}

func WithReActPreReply(h HookFunc) ReActOption {
	return func(a *ReActAgent) { a.hooks.preReply = append(a.hooks.preReply, h) }
}

func WithReActPostReply(h HookFunc) ReActOption {
	return func(a *ReActAgent) { a.hooks.postReply = append(a.hooks.postReply, h) }
}

type ReActAgent struct {
	id        string
	name      string
	sysPrompt string
	model     model.ChatModelBase
	mem       memory.MemoryBase
	fmt       formatter.FormatterBase
	toolkit   *tool.Toolkit
	maxIters  int
	hooks     hooks
}

func NewReActAgent(opts ...ReActOption) *ReActAgent {
	a := &ReActAgent{
		id:       utils.ShortUUID(),
		name:     "react_agent",
		maxIters: defaultMaxIters,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.mem == nil {
		a.mem = memory.NewInMemoryMemory()
	}
	return a
}

func (a *ReActAgent) ID() string   { return a.id }
func (a *ReActAgent) Name() string { return a.name }

func (a *ReActAgent) Memory() memory.MemoryBase {
	return a.mem
}

func (a *ReActAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	for _, h := range a.hooks.preReply {
		h(ctx, a, msg, nil)
	}

	if msg != nil {
		if err := a.mem.Add(ctx, msg); err != nil {
			return nil, fmt.Errorf("add message to memory: %w", err)
		}
	}

	if a.sysPrompt != "" {
		sysMsg := message.NewMsg("system", a.sysPrompt, "system")
		existing := a.mem.GetMessages()
		restored := append([]*message.Msg{sysMsg}, existing...)
		a.mem.Clear(ctx)
		a.mem.Add(ctx, restored...)
	}

	var resp *message.Msg
	var err error
	for i := 0; i < a.maxIters; i++ {
		resp, err = a.thinkAndAct(ctx)
		if err != nil {
			return nil, err
		}
		if !hasToolUse(resp) {
			break
		}
	}

	for _, h := range a.hooks.postReply {
		h(ctx, a, msg, resp)
	}

	return resp, nil
}

func (a *ReActAgent) Observe(ctx context.Context, msg *message.Msg) error {
	if msg != nil {
		return a.mem.Add(ctx, msg)
	}
	return nil
}

func (a *ReActAgent) thinkAndAct(ctx context.Context) (*message.Msg, error) {
	msgs := a.mem.GetMessages()
	formatted, err := a.fmt.Format(msgs)
	if err != nil {
		return nil, fmt.Errorf("format messages: %w", err)
	}

	var opts []model.CallOption
	if a.toolkit != nil && len(a.toolkit.GetSchemas()) > 0 {
		opts = append(opts, model.CallOption{
			Tools:      a.toolkit.GetSchemas(),
			ToolChoice: "auto",
		})
	}

	chatResp, err := a.model.Call(ctx, formatted, opts...)
	if err != nil {
		return nil, fmt.Errorf("model call: %w", err)
	}

	assistantMsg := &message.Msg{
		ID:        utils.ShortUUID(),
		Name:      a.name,
		Role:      "assistant",
		Content:   chatResp.Content,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
	}
	if err := a.mem.Add(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("add assistant message: %w", err)
	}

	if chatResp.HasToolUse() {
		toolUseBlocks := chatResp.GetToolUseBlocks()
		var toolResultBlocks []message.ContentBlock

		for _, block := range toolUseBlocks {
			toolName := message.GetBlockToolUseName(block)
			toolID := message.GetBlockToolUseID(block)
			toolInput := message.GetBlockToolUseInput(block)

			args, ok := toMap(toolInput)
			if !ok {
				toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
					toolID, fmt.Sprintf("invalid tool input: %v", toolInput), true,
				))
				continue
			}

			result, execErr := a.toolkit.Execute(ctx, toolName, args)
			if execErr != nil {
				toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
					toolID, execErr.Error(), true,
				))
				continue
			}

			toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
				toolID, result.Content, result.IsError,
			))
		}

		toolResultMsg := &message.Msg{
			ID:        utils.ShortUUID(),
			Name:      "tool",
			Role:      "tool",
			Content:   toolResultBlocks,
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		}
		if err := a.mem.Add(ctx, toolResultMsg); err != nil {
			return nil, fmt.Errorf("add tool result: %w", err)
		}

		return assistantMsg, nil
	}

	return assistantMsg, nil
}

func hasToolUse(msg *message.Msg) bool {
	for _, block := range msg.Content {
		if message.IsToolUseBlock(block) {
			return true
		}
	}
	return false
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	switch val := v.(type) {
	case map[string]interface{}:
		return val, true
	case string:
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(val), &m); err == nil {
			return m, true
		}
		return nil, false
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, false
		}
		return m, true
	}
}
