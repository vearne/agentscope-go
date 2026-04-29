package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
)

const defaultCompressionPrompt = `Summarize the following conversation history concisely. Preserve:
- Session goals and intent
- Key decisions made
- Artifacts or files created/modified
- Current task status and progress
- Next steps or pending actions

Return ONLY the summary text, nothing else.`

// ContextCompressor compresses old conversation history into a shorter form.
type ContextCompressor interface {
	Compress(ctx context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error)
}

// TruncatingCompressor drops old messages, keeping only the N most recent.
type TruncatingCompressor struct{}

func (c *TruncatingCompressor) Compress(_ context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error) {
	if len(msgs) <= keepRecent {
		return msgs, nil
	}
	return msgs[len(msgs)-keepRecent:], nil
}

// LLMCompressor uses an LLM to generate a summary of old conversation history.
type LLMCompressor struct {
	model  model.ChatModelBase
	fmt    formatter.FormatterBase
	prompt string
}

func NewLLMCompressor(m model.ChatModelBase, f formatter.FormatterBase, prompt string) *LLMCompressor {
	p := prompt
	if p == "" {
		p = defaultCompressionPrompt
	}
	return &LLMCompressor{model: m, fmt: f, prompt: p}
}

func (c *LLMCompressor) Compress(ctx context.Context, msgs []*message.Msg, keepRecent int) ([]*message.Msg, error) {
	if len(msgs) <= keepRecent {
		return msgs, nil
	}

	oldMsgs := msgs[:len(msgs)-keepRecent]
	recentMsgs := msgs[len(msgs)-keepRecent:]

	sumMsg := message.NewMsg("user", c.prompt+"\n\n--- Conversation to summarize ---", "user")
	compressMsgs := append([]*message.Msg{sumMsg}, oldMsgs...)

	formatted, err := c.fmt.Format(compressMsgs)
	if err != nil {
		log.Printf("[LLMCompressor] format error: %v, returning original messages", err)
		return msgs, nil
	}

	resp, err := c.model.Call(ctx, formatted)
	if err != nil {
		log.Printf("[LLMCompressor] model call error: %v, returning original messages", err)
		return msgs, nil
	}

	summaryText := resp.GetTextContent()
	if summaryText == "" {
		log.Printf("[LLMCompressor] empty summary, returning original messages")
		return msgs, nil
	}

	summaryMsg := message.NewMsg("user", fmt.Sprintf("[Conversation Summary]\n%s", summaryText), "user")
	result := append([]*message.Msg{summaryMsg}, recentMsgs...)
	return result, nil
}
