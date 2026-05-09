package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/studio"
	"github.com/vearne/agentscope-go/pkg/tool"
	"github.com/vearne/agentscope-go/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const defaultMaxIters = 10

// gen_ai.*.messages OTEL attributes are strings; Studio parses JSON. Keep a
// conservative size so OTLP collectors do not drop oversized spans.
const maxGenAIMessagesAttrBytes = 32000

func truncateGenAIMessagesJSON(s string) string {
	if len(s) <= maxGenAIMessagesAttrBytes {
		return s
	}
	return s[:maxGenAIMessagesAttrBytes] + "…[truncated]"
}

func stringifyGenAITracePayload(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		fallback, _ := json.Marshal(map[string]string{"error": err.Error()})
		return truncateGenAIMessagesJSON(string(fallback))
	}
	return truncateGenAIMessagesJSON(string(b))
}

// toGenAIMessages converts various input types to GenAI message format.
// Supports []*message.Msg, []message.ContentBlock, and []model.FormattedMessage.
func toGenAIMessages(input any) []message.GenAIMessage {
	switch v := input.(type) {
	case []*message.Msg:
		return message.ConvertMsgsToGenAIMessages(v)
	case []message.ContentBlock:
		if len(v) == 0 {
			return []message.GenAIMessage{}
		}
		msg := &message.Msg{Content: v, Role: "user"}
		return []message.GenAIMessage{message.ConvertMsgToGenAIMessage(msg)}
	case []model.FormattedMessage:
		if len(v) == 0 {
			return []message.GenAIMessage{}
		}
		result := make([]message.GenAIMessage, len(v))
		for i, fm := range v {
			role, _ := fm["role"].(string)
			content, _ := fm["content"].(string)
			result[i] = message.GenAIMessage{
				Role: role,
				Parts: []message.GenAIPart{
					{"type": "text", "content": content},
				},
				FinishReason: "stop",
			}
		}
		return result
	default:
		if m, ok := input.(*message.Msg); ok {
			return []message.GenAIMessage{message.ConvertMsgToGenAIMessage(m)}
		}
		return []message.GenAIMessage{}
	}
}

// spanIOAttrs sets both OpenTelemetry GenAI keys and flat "input"/"output"
// keys. agentscope-studio's TraceDetailPage reads gen_ai.*.messages first,
// then falls back to top-level "input" / "output"; flat keys also survive
// attribute unflattening edge cases in the OTLP decoder.
func spanIOAttrs(input, output any) []attribute.KeyValue {
	inMsgs := toGenAIMessages(input)
	outMsgs := toGenAIMessages(output)
	return []attribute.KeyValue{
		attribute.String("gen_ai.input.messages", stringifyGenAITracePayload(inMsgs)),
		attribute.String("gen_ai.output.messages", stringifyGenAITracePayload(outMsgs)),
		attribute.String("input", stringifyGenAITracePayload(inMsgs)),
		attribute.String("output", stringifyGenAITracePayload(outMsgs)),
	}
}

func spanInputAttrsOnly(input any) []attribute.KeyValue {
	inMsgs := toGenAIMessages(input)
	return []attribute.KeyValue{
		attribute.String("gen_ai.input.messages", stringifyGenAITracePayload(inMsgs)),
		attribute.String("input", stringifyGenAITracePayload(inMsgs)),
	}
}

func spanOutputAttrsOnly(output any) []attribute.KeyValue {
	outMsgs := toGenAIMessages(output)
	return []attribute.KeyValue{
		attribute.String("gen_ai.output.messages", stringifyGenAITracePayload(outMsgs)),
		attribute.String("output", stringifyGenAITracePayload(outMsgs)),
	}
}

func spanToolInputAttrsOnly(args map[string]any) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("gen_ai.tool.call.arguments", stringifyGenAITracePayload(args)),
	}
}

func spanToolOutputAttrsOnly(result map[string]any) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("gen_ai.tool.call.result", stringifyGenAITracePayload(result)),
	}
}

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

	interrupted atomic.Bool
	cancelMu    sync.Mutex
	cancelFunc  context.CancelFunc
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
	if sc := studio.GetClient(); sc != nil {
		a.hooks.preReply = append(a.hooks.preReply, func(ctx context.Context, ag AgentBase, msg *message.Msg, resp *message.Msg) {
			studio.ForwardMessage(ctx, msg.Name, msg.Role, msg)
		})
		a.hooks.postReply = append(a.hooks.postReply, func(ctx context.Context, ag AgentBase, msg *message.Msg, resp *message.Msg) {
			studio.ForwardMessage(ctx, ag.Name(), "assistant", resp)
		})
	}
	return a
}

func (a *ReActAgent) ID() string   { return a.id }
func (a *ReActAgent) Name() string { return a.name }

func (a *ReActAgent) Memory() memory.MemoryBase {
	return a.mem
}

func (a *ReActAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	ctx, span := tracing.StartSpan(ctx, "invoke_agent "+a.name, tracingAttributes(
		attribute.String("gen_ai.operation.name", "invoke_agent"),
		attribute.String("gen_ai.agent.name", a.name),
	)...)
	defer span.End()

	if msg != nil {
		span.SetAttributes(spanInputAttrsOnly([]*message.Msg{msg})...)
	}

	for _, h := range a.hooks.preReply {
		h(ctx, a, msg, nil)
	}

	if msg != nil {
		if err := a.mem.Add(ctx, msg); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("add message to memory: %w", err)
		}
	}

	if a.sysPrompt != "" {
		sysMsg := message.NewMsg("system", a.sysPrompt, "system")
		existing := a.mem.GetMessages()
		restored := append([]*message.Msg{sysMsg}, existing...)
		if err := a.mem.Clear(ctx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("clear memory: %w", err)
		}
		if err := a.mem.Add(ctx, restored...); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("add messages to memory: %w", err)
		}
	}

	a.interrupted.Store(false)
	ctx, cancel := context.WithCancel(ctx)
	a.cancelMu.Lock()
	a.cancelFunc = cancel
	a.cancelMu.Unlock()
	defer func() {
		cancel()
		a.interrupted.Store(false)
		a.cancelMu.Lock()
		a.cancelFunc = nil
		a.cancelMu.Unlock()
	}()

	var resp *message.Msg
	var err error
	for i := 0; i < a.maxIters; i++ {
		resp, err = a.thinkAndAct(ctx)
		if err != nil {
			if a.interrupted.Load() {
				span.AddEvent("interrupted")
				hResp, hErr := a.HandleInterrupt(ctx, msg)
				if hErr != nil {
					span.RecordError(hErr)
					span.SetStatus(codes.Error, hErr.Error())
					return nil, hErr
				}
				span.SetStatus(codes.Ok, "")
				return hResp, nil
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		if a.interrupted.Load() {
			span.AddEvent("interrupted")
			hResp, hErr := a.HandleInterrupt(ctx, msg)
			if hErr != nil {
				span.RecordError(hErr)
				span.SetStatus(codes.Error, hErr.Error())
				return nil, hErr
			}
			span.SetStatus(codes.Ok, "")
			return hResp, nil
		}

		if !hasToolUse(resp) {
			break
		}
	}

	for _, h := range a.hooks.postReply {
		h(ctx, a, msg, resp)
	}

	if resp != nil {
		span.SetAttributes(spanOutputAttrsOnly([]*message.Msg{resp})...)
	}

	span.SetStatus(codes.Ok, "")
	return resp, nil
}

func (a *ReActAgent) Observe(ctx context.Context, msg *message.Msg) error {
	if msg != nil {
		return a.mem.Add(ctx, msg)
	}
	return nil
}

// thinkAndAct executes a single iteration of the ReAct loop:
//  1. Format the conversation history from memory into a provider-specific request
//  2. Call the LLM to get a response (with optional tool schemas)
//  3. If the LLM requests tool calls, execute each tool and collect results into memory
//
// The caller (Reply loop) decides whether to iterate again based on whether tool results
// were produced. This function only handles one think-and-act step.
func (a *ReActAgent) thinkAndAct(ctx context.Context) (*message.Msg, error) {
	// --- Top-level tracing span for the entire chat call ---
	ctx, span := tracing.StartSpan(ctx, "chat "+a.model.ModelName(), tracingAttributes(
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", a.model.ModelName()),
	)...)
	defer span.End()

	// --- Step 1: Format conversation history ---
	// Convert internal Msg objects into the model provider's expected request format
	msgs := a.mem.GetMessages()
	formatCtx, formatSpan := tracing.StartSpan(ctx, "format "+a.name, tracingAttributes(
		attribute.String("gen_ai.operation.name", "format"),
		attribute.String("gen_ai.agent.name", a.name),
	)...)
	formatted, err := a.fmt.Format(msgs)
	if err != nil {
		formatSpan.SetAttributes(spanInputAttrsOnly(msgs)...)
		formatSpan.RecordError(err)
		formatSpan.SetStatus(codes.Error, err.Error())
		formatSpan.End()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("format messages: %w", err)
	}
	formatSpan.SetAttributes(spanIOAttrs(msgs, formatted)...)
	formatSpan.SetStatus(codes.Ok, "")
	formatSpan.End()

	// --- Step 2: Call the LLM ---
	// Attach tool schemas if a toolkit is configured, allowing the model to request tool use
	var opts []model.CallOption
	if a.toolkit != nil && len(a.toolkit.GetSchemas()) > 0 {
		opts = append(opts, model.CallOption{
			Tools:      a.toolkit.GetSchemas(),
			ToolChoice: "auto",
		})
	}

	chatResp, err := a.model.Call(formatCtx, formatted, opts...)
	if err != nil {
		span.SetAttributes(spanInputAttrsOnly(formatted)...)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("model call: %w", err)
	}
	var usageAttrs []attribute.KeyValue
	if chatResp != nil {
		usageAttrs = spanIOAttrs(formatted, chatResp.Content)
		if chatResp.Usage != nil {
			usageAttrs = append(usageAttrs,
				attribute.Int("gen_ai.usage.input_tokens", chatResp.Usage.InputTokens),
				attribute.Int("gen_ai.usage.output_tokens", chatResp.Usage.OutputTokens),
			)
		}
	}
	span.SetAttributes(usageAttrs...)

	// --- Step 3: Persist the assistant's response ---
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

	// Forward to studio UI if connected
	if sc := studio.GetClient(); sc != nil {
		studio.ForwardMessage(ctx, a.name, "assistant", assistantMsg)
	}

	// --- Step 4: Handle tool calls (if any) ---
	// When the model returns tool_use blocks, execute each tool and collect the results.
	// The tool results are stored in memory so the next iteration can see them.
	if chatResp.HasToolUse() {
		toolUseBlocks := chatResp.GetToolUseBlocks()
		var toolResultBlocks []message.ContentBlock

		for _, block := range toolUseBlocks {
			toolName := message.GetBlockToolUseName(block)
			toolID := message.GetBlockToolUseID(block)
			toolInput := message.GetBlockToolUseInput(block)

			// Parse tool input into a map; if parsing fails, record an error result
			args, ok := toMap(toolInput)
			if !ok {
				toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
					toolID, fmt.Sprintf("invalid tool input: %v", toolInput), true,
				))
				continue
			}

			// Execute the tool with its own tracing span
			_, toolSpan := tracing.StartSpan(ctx, "execute_tool "+toolName, tracingAttributes(
				attribute.String("gen_ai.operation.name", "execute_tool"),
				attribute.String("gen_ai.tool.name", toolName),
			)...)
			toolSpan.SetAttributes(spanToolInputAttrsOnly(args)...)
			result, execErr := a.toolkit.Execute(ctx, toolName, args)
			if execErr != nil {
				toolSpan.SetAttributes(spanToolOutputAttrsOnly(map[string]any{
					"error": execErr.Error(),
				})...)
				toolSpan.RecordError(execErr)
				toolSpan.SetStatus(codes.Error, execErr.Error())
				toolSpan.End()
				// Record execution error as a tool result so the LLM can react to it
				toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
					toolID, execErr.Error(), true,
				))
				continue
			}
			toolSpan.SetAttributes(spanToolOutputAttrsOnly(map[string]any{
				"content":   result.Content,
				"is_error":  result.IsError,
				"tool_name": toolName,
			})...)
			toolSpan.SetStatus(codes.Ok, "")
			toolSpan.End()

			toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
				toolID, result.Content, result.IsError,
			))
		}

		// Store all tool results as a single "tool" message in memory
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

		if sc := studio.GetClient(); sc != nil {
			studio.ForwardMessage(ctx, "tool", "tool", toolResultMsg)
		}

		return assistantMsg, nil
	}

	// No tool calls — the model gave a direct text response, this iteration is complete
	span.SetStatus(codes.Ok, "")
	return assistantMsg, nil
}

func tracingAttributes(extra ...attribute.KeyValue) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(extra)+1)
	if sc := studio.GetClient(); sc != nil {
		attrs = append(attrs, attribute.String("gen_ai.conversation.id", sc.RunID()))
	}
	attrs = append(attrs, extra...)
	return attrs
}

func (a *ReActAgent) Interrupt() {
	a.interrupted.Store(true)
	a.cancelMu.Lock()
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	a.cancelMu.Unlock()
}

func (a *ReActAgent) HandleInterrupt(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	// Use a context that is not cancelled so that Studio forwarding and
	// post-reply hooks can complete. The caller's ctx is already cancelled
	// because Interrupt() invoked cancelFunc().
	studioCtx := context.WithoutCancel(ctx)

	resp := &message.Msg{
		ID:        utils.ShortUUID(),
		Name:      a.name,
		Role:      "assistant",
		Content:   []message.ContentBlock{message.NewTextBlock("I noticed that you have interrupted me. What can I do for you?")},
		Metadata:  map[string]interface{}{"_is_interrupted": true},
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
	}
	if err := a.mem.Add(studioCtx, resp); err != nil {
		return nil, fmt.Errorf("add interrupted message to memory: %w", err)
	}

	for _, h := range a.hooks.postReply {
		h(studioCtx, a, msg, resp)
	}

	if sc := studio.GetClient(); sc != nil {
		studio.ForwardMessage(studioCtx, a.name, "assistant", resp)
	}
	return resp, nil
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
