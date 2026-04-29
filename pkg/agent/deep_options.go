package agent

import (
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

const (
	defaultDeepMaxIters         = 50
	defaultDeepMaxCtxTokens     = 128000
	defaultDeepOffloadDir       = ".deepagent/offload/"
	defaultDeepOffloadThreshold = 8000
)

type DeepOption func(*DeepAgent)

func WithDeepName(name string) DeepOption {
	return func(a *DeepAgent) { a.name = name }
}

func WithDeepModel(m model.ChatModelBase) DeepOption {
	return func(a *DeepAgent) { a.model = m }
}

func WithDeepMemory(mem memory.MemoryBase) DeepOption {
	return func(a *DeepAgent) { a.mem = mem }
}

func WithDeepFormatter(f formatter.FormatterBase) DeepOption {
	return func(a *DeepAgent) { a.fmt = f }
}

func WithDeepToolkit(tk *tool.Toolkit) DeepOption {
	return func(a *DeepAgent) { a.toolkit = tk }
}

func WithDeepMaxIters(n int) DeepOption {
	return func(a *DeepAgent) { a.maxIters = n }
}

func WithDeepSystemPrompt(prompt string) DeepOption {
	return func(a *DeepAgent) { a.sysPrompt = prompt }
}

func WithDeepMaxContextTokens(n int) DeepOption {
	return func(a *DeepAgent) { a.maxCtxTokens = n }
}

func WithDeepOffloadDir(dir string) DeepOption {
	return func(a *DeepAgent) { a.offloadDir = dir }
}

func WithDeepOffloadThreshold(chars int) DeepOption {
	return func(a *DeepAgent) { a.offloadThreshold = chars }
}

func WithDeepCompressor(c ContextCompressor) DeepOption {
	return func(a *DeepAgent) { a.compressor = c }
}

func WithDeepSubagentFactory(f *SubagentFactory) DeepOption {
	return func(a *DeepAgent) { a.subFactory = f }
}

func WithDeepPreReply(h HookFunc) DeepOption {
	return func(a *DeepAgent) { a.hooks.preReply = append(a.hooks.preReply, h) }
}

func WithDeepPostReply(h HookFunc) DeepOption {
	return func(a *DeepAgent) { a.hooks.postReply = append(a.hooks.postReply, h) }
}
