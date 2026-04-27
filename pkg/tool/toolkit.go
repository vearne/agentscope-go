package tool

import (
	"context"
	"fmt"
	"sync"

	"github.com/vearne/agentscope-go/pkg/model"
)

type registeredTool struct {
	schema   model.ToolSchema
	function ToolFunc
}

type Toolkit struct {
	tools  map[string]*registeredTool
	skills map[string]*AgentSkill
	mu     sync.RWMutex
}

func NewToolkit() *Toolkit {
	return &Toolkit{
		tools:  make(map[string]*registeredTool),
		skills: make(map[string]*AgentSkill),
	}
}

func (t *Toolkit) Register(name, description string, params map[string]interface{}, fn ToolFunc) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	t.tools[name] = &registeredTool{
		schema: model.ToolSchema{
			Type: "function",
			Function: model.FuncSchema{
				Name:        name,
				Description: description,
				Parameters:  params,
			},
		},
		function: fn,
	}
	return nil
}

func (t *Toolkit) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResponse, error) {
	t.mu.RLock()
	rt, exists := t.tools[toolName]
	t.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("tool %q not found", toolName)
	}
	return rt.function(ctx, args)
}

func (t *Toolkit) GetSchemas() []model.ToolSchema {
	t.mu.RLock()
	defer t.mu.RUnlock()
	schemas := make([]model.ToolSchema, 0, len(t.tools))
	for _, rt := range t.tools {
		schemas = append(schemas, rt.schema)
	}
	return schemas
}

func (t *Toolkit) GetToolNames() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	names := make([]string, 0, len(t.tools))
	for name := range t.tools {
		names = append(names, name)
	}
	return names
}

func (t *Toolkit) HasTool(name string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.tools[name]
	return exists
}
