package tool

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/vearne/agentscope-go/pkg/model"
)

const basicGroup = "basic"

type registeredTool struct {
	schema   model.ToolSchema
	function ToolFunc
	group    string
}

type ToolGroup struct {
	Name        string
	Active      bool
	Description string
	Notes       string
}

type Toolkit struct {
	tools  map[string]*registeredTool
	groups map[string]*ToolGroup
	skills map[string]*AgentSkill
	mu     sync.RWMutex
}

func NewToolkit() *Toolkit {
	return &Toolkit{
		tools:  make(map[string]*registeredTool),
		groups: map[string]*ToolGroup{basicGroup: {Name: basicGroup, Active: true}},
		skills: make(map[string]*AgentSkill),
	}
}

func (t *Toolkit) Register(name, description string, params map[string]interface{}, fn ToolFunc) error {
	return t.RegisterInGroup(name, description, basicGroup, params, fn)
}

func (t *Toolkit) RegisterInGroup(name, description, group string, params map[string]interface{}, fn ToolFunc) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	if group != basicGroup {
		if _, exists := t.groups[group]; !exists {
			return fmt.Errorf("tool group %q does not exist", group)
		}
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
		group:    group,
	}
	return nil
}

func (t *Toolkit) RemoveTool(name string, allowNotExist bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.tools[name]; !exists && !allowNotExist {
		return fmt.Errorf("tool %q not found", name)
	}
	delete(t.tools, name)
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
		if rt.group == basicGroup || t.isGroupActive(rt.group) {
			schemas = append(schemas, rt.schema)
		}
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

func (t *Toolkit) CreateToolGroup(name, description string, active bool, notes string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if name == basicGroup {
		return fmt.Errorf("cannot create tool group with reserved name %q", basicGroup)
	}
	if _, exists := t.groups[name]; exists {
		return fmt.Errorf("tool group %q already exists", name)
	}
	t.groups[name] = &ToolGroup{
		Name:        name,
		Active:      active,
		Description: description,
		Notes:       notes,
	}
	return nil
}

func (t *Toolkit) UpdateToolGroups(names []string, active bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, name := range names {
		if name == basicGroup {
			continue
		}
		if g, exists := t.groups[name]; exists {
			g.Active = active
		}
	}
}

func (t *Toolkit) RemoveToolGroups(names []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, name := range names {
		if name == basicGroup {
			return fmt.Errorf("cannot remove the default %q tool group", basicGroup)
		}
		delete(t.groups, name)
	}
	for toolName, rt := range t.tools {
		if rt.group != basicGroup {
			for _, g := range names {
				if rt.group == g {
					delete(t.tools, toolName)
					break
				}
			}
		}
	}
	return nil
}

func (t *Toolkit) GetActivatedNotes() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var parts []string
	for _, g := range t.groups {
		if g.Active && g.Notes != "" {
			parts = append(parts, fmt.Sprintf("## About Tool Group '%s'\n%s", g.Name, g.Notes))
		}
	}
	return strings.Join(parts, "\n")
}

func (t *Toolkit) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tools = make(map[string]*registeredTool)
	for name := range t.groups {
		if name != basicGroup {
			delete(t.groups, name)
		}
	}
}

func (t *Toolkit) isGroupActive(group string) bool {
	g, exists := t.groups[group]
	return exists && g.Active
}
