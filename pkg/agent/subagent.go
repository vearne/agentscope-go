package agent

import (
	"context"
	"fmt"

	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const defaultSubagentMaxIters = 10

type SubagentFactory struct {
	model   model.ChatModelBase
	fmt     formatter.FormatterBase
	toolkit *tool.Toolkit
}

type SubagentConfig struct {
	Name         string
	SystemPrompt string
	MaxIters     int
	Toolkit      *tool.Toolkit
}

type DelegateTaskArgs struct {
	TaskDescription string `json:"task_description"`
	SubagentName    string `json:"subagent_name"`
	SystemPrompt    string `json:"system_prompt"`
}

func NewSubagentFactory(m model.ChatModelBase, f formatter.FormatterBase, tk *tool.Toolkit) *SubagentFactory {
	return &SubagentFactory{model: m, fmt: f, toolkit: tk}
}

func (f *SubagentFactory) Create(cfg SubagentConfig) *ReActAgent {
	opts := []ReActOption{
		WithReActName(cfg.Name),
		WithReActModel(f.model),
		WithReActFormatter(f.fmt),
		WithReActMemory(memory.NewInMemoryMemory()),
	}

	tk := cfg.Toolkit
	if tk == nil {
		tk = f.toolkit
	}
	if tk != nil {
		opts = append(opts, WithReActToolkit(tk))
	}

	maxIters := cfg.MaxIters
	if maxIters <= 0 {
		maxIters = defaultSubagentMaxIters
	}
	opts = append(opts, WithReActMaxIters(maxIters))

	if cfg.SystemPrompt != "" {
		opts = append(opts, WithReActSystemPrompt(cfg.SystemPrompt))
	}

	return NewReActAgent(opts...)
}

func (f *SubagentFactory) DelegateTask(ctx context.Context, args DelegateTaskArgs) (string, error) {
	sysPrompt := args.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = "You are a helpful assistant. Complete the task and return the result concisely."
	}

	sub := f.Create(SubagentConfig{
		Name:         args.SubagentName,
		SystemPrompt: sysPrompt,
	})

	msg := message.NewMsg("user", args.TaskDescription, "user")
	resp, err := sub.Reply(ctx, msg)
	if err != nil {
		return "", fmt.Errorf("subagent %q failed: %w", args.SubagentName, err)
	}

	return resp.GetTextContent(), nil
}
