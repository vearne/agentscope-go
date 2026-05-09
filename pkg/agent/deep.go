package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
	"github.com/vearne/agentscope-go/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const avgCharsPerToken = 4

type DeepAgent struct {
	id        string
	name      string
	sysPrompt string
	model     model.ChatModelBase
	mem       memory.MemoryBase
	fmt       formatter.FormatterBase
	toolkit   *tool.Toolkit
	maxIters  int

	compressor       ContextCompressor
	offloader        *OffloadManager
	maxCtxTokens     int
	offloadDir       string
	offloadThreshold int

	subFactory *SubagentFactory

	internalToolkit *tool.Toolkit

	hooks hooks

	interrupted atomic.Bool
	cancelMu    sync.Mutex
	cancelFunc  context.CancelFunc
}

func NewDeepAgent(opts ...DeepOption) *DeepAgent {
	a := &DeepAgent{
		id:               utils.ShortUUID(),
		name:             "deep_agent",
		maxIters:         defaultDeepMaxIters,
		maxCtxTokens:     defaultDeepMaxCtxTokens,
		offloadDir:       defaultDeepOffloadDir,
		offloadThreshold: defaultDeepOffloadThreshold,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.mem == nil {
		a.mem = memory.NewInMemoryMemory()
	}
	if a.compressor == nil && a.model != nil && a.fmt != nil {
		a.compressor = NewLLMCompressor(a.model, a.fmt, "")
	}
	if a.compressor == nil {
		a.compressor = &TruncatingCompressor{}
	}
	a.offloader = NewOffloadManager(a.offloadDir, a.offloadThreshold)
	a.buildInternalToolkit()
	return a
}

func (a *DeepAgent) ID() string   { return a.id }
func (a *DeepAgent) Name() string { return a.name }

func (a *DeepAgent) Memory() memory.MemoryBase {
	return a.mem
}

func (a *DeepAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
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
		if err := a.mem.Clear(ctx); err != nil {
			return nil, fmt.Errorf("clear memory: %w", err)
		}
		if err := a.mem.Add(ctx, restored...); err != nil {
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
		a.maybeCompressContext(ctx)

		resp, err = a.thinkAndAct(ctx)
		if err != nil {
			if a.interrupted.Load() {
				return a.HandleInterrupt(ctx, msg)
			}
			return nil, err
		}

		if a.interrupted.Load() {
			return a.HandleInterrupt(ctx, msg)
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

func (a *DeepAgent) Observe(ctx context.Context, msg *message.Msg) error {
	if msg != nil {
		return a.mem.Add(ctx, msg)
	}
	return nil
}

func (a *DeepAgent) Interrupt() {
	a.interrupted.Store(true)
	a.cancelMu.Lock()
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	a.cancelMu.Unlock()
}

func (a *DeepAgent) HandleInterrupt(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	resp := &message.Msg{
		ID:        utils.ShortUUID(),
		Name:      a.name,
		Role:      "assistant",
		Content:   []message.ContentBlock{message.NewTextBlock("I noticed that you have interrupted me. What can I do for you?")},
		Metadata:  map[string]interface{}{"_is_interrupted": true},
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
	}
	if err := a.mem.Add(ctx, resp); err != nil {
		return nil, fmt.Errorf("add interrupted message to memory: %w", err)
	}

	for _, h := range a.hooks.postReply {
		h(ctx, a, msg, resp)
	}
	return resp, nil
}

// thinkAndAct executes a single iteration of the DeepAgent loop:
//  1. Format the conversation history from memory into a provider-specific request
//  2. Call the LLM (with user tools + internal tools like delegate_task)
//  3. If the LLM requests tool calls, execute each tool, optionally offload large results to disk
//
// Unlike ReActAgent, this function does NOT forward messages to a studio UI.
// The caller (Reply loop) handles iteration control and context compression.
func (a *DeepAgent) thinkAndAct(ctx context.Context) (*message.Msg, error) {
	ctx, span := tracing.StartSpan(ctx, "chat "+a.model.ModelName(), tracingAttributes(
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", a.model.ModelName()),
	)...)
	defer span.End()

	// --- Step 1: Format conversation history ---
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
	// internalToolkit wraps user tools and adds delegate_task; fall back to user toolkit
	var opts []model.CallOption
	tk := a.internalToolkit
	if tk == nil {
		tk = a.toolkit
	}
	if tk != nil && len(tk.GetSchemas()) > 0 {
		opts = append(opts, model.CallOption{
			Tools:      tk.GetSchemas(),
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

	// --- Step 4: Handle tool calls (if any) ---
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

			// Execute the tool with its own tracing span
			_, toolSpan := tracing.StartSpan(ctx, "execute_tool "+toolName, tracingAttributes(
				attribute.String("gen_ai.operation.name", "execute_tool"),
				attribute.String("gen_ai.tool.name", toolName),
			)...)
			toolSpan.SetAttributes(spanToolInputAttrsOnly(args)...)
			result, execErr := tk.Execute(ctx, toolName, args)
			if execErr != nil {
				toolSpan.SetAttributes(spanToolOutputAttrsOnly(map[string]any{
					"error": execErr.Error(),
				})...)
				toolSpan.RecordError(execErr)
				toolSpan.SetStatus(codes.Error, execErr.Error())
				toolSpan.End()
				toolResultBlocks = append(toolResultBlocks, message.NewToolResultBlock(
					toolID, execErr.Error(), true,
				))
				continue
			}

			// Offload large tool outputs to disk to avoid bloating the context window
			content := fmt.Sprintf("%v", result.Content)
			if len(content) > a.offloadThreshold {
				offloadID := utils.ShortUUID()
				ref, offloaded := a.offloader.MaybeOffload(content, offloadID)
				if offloaded {
					result.Content = ref
				}
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
	}

	span.SetStatus(codes.Ok, "")
	return assistantMsg, nil
}

func (a *DeepAgent) maybeCompressContext(ctx context.Context) {
	msgs := a.mem.GetMessages()
	estimated := estimateTokens(msgs)
	threshold := int(float64(a.maxCtxTokens) * 0.85)

	if estimated > threshold && len(msgs) > 6 {
		compressed, err := a.compressor.Compress(ctx, msgs, 6)
		if err != nil {
			log.Printf("[DeepAgent] compression error: %v, continuing with original context", err)
			return
		}
		if err := a.mem.Clear(ctx); err != nil {
			log.Printf("[DeepAgent] clear memory error: %v", err)
			return
		}
		if err := a.mem.Add(ctx, compressed...); err != nil {
			log.Printf("[DeepAgent] add compressed messages error: %v", err)
		}
	}
}

func estimateTokens(msgs []*message.Msg) int {
	totalChars := 0
	for _, msg := range msgs {
		for _, block := range msg.Content {
			if text := message.GetBlockText(block); text != "" {
				totalChars += len(text)
			}
		}
	}
	return totalChars / avgCharsPerToken
}

func (a *DeepAgent) buildInternalToolkit() {
	if a.toolkit == nil && a.subFactory == nil {
		return
	}

	a.internalToolkit = tool.NewToolkit()

	if a.toolkit != nil {
		schemas := a.toolkit.GetSchemas()
		for _, s := range schemas {
			toolName := s.Function.Name
			if err := a.internalToolkit.Register(s.Function.Name, s.Function.Description, s.Function.Parameters,
				func(ctx context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
					return a.toolkit.Execute(ctx, toolName, args)
				},
			); err != nil {
				log.Printf("[DeepAgent] register tool %q: %v", s.Function.Name, err)
			}
		}
	}

	if a.subFactory != nil {
	if err := a.internalToolkit.Register("delegate_task",
		"Delegate a task to a subagent. The subagent runs independently with its own context. Use this for complex multi-step work that would clutter your context.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_description": map[string]interface{}{
					"type":        "string",
					"description": "Clear description of what the subagent should accomplish",
				},
				"subagent_name": map[string]interface{}{
					"type":        "string",
					"description": "Name for the subagent",
				},
				"system_prompt": map[string]interface{}{
					"type":        "string",
					"description": "Optional system prompt for the subagent",
				},
			},
			"required": []string{"task_description", "subagent_name"},
		},
		func(ctx context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
			desc, _ := args["task_description"].(string)
			name, _ := args["subagent_name"].(string)
			sysPrompt, _ := args["system_prompt"].(string)

			if desc == "" || name == "" {
				return tool.NewErrorResponse("task_description and subagent_name are required"), nil
			}

			result, err := a.subFactory.DelegateTask(ctx, DelegateTaskArgs{
				TaskDescription: desc,
				SubagentName:    name,
				SystemPrompt:    sysPrompt,
			})
			if err != nil {
				return tool.NewErrorResponse(fmt.Sprintf("subagent failed: %v", err)), nil
			}
			return tool.NewToolResponse(result), nil
		},
	); err != nil {
		log.Printf("[DeepAgent] register tool %q: %v", "delegate_task", err)
	}
	}
}
