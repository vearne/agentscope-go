package tool

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AgentSkill struct {
	Name        string
	Description string
	Dir         string
}

const (
	defaultAgentSkillInstruction = "# Agent Skills\n" +
		"The agent skills are a collection of folders of instructions, scripts, " +
		"and resources that you can load dynamically to improve performance " +
		"on specialized tasks. Each agent skill has a `SKILL.md` file in its " +
		"folder that describes how to use the skill. If you want to use a " +
		"skill, you MUST read its `SKILL.md` file carefully."

	defaultAgentSkillTemplate = "## %s\n%s\nCheck \"%s/SKILL.md\" for how to use this skill"
)

// RegisterAgentSkill loads an agent skill from skillDir, which must contain
// a SKILL.md file with YAML front matter:
//
//	---
//	name: Weather Query
//	description: This skill provides weather query capabilities.
//	---
//
//	# Weather Query Skill
//	Instructions go here...
func (t *Toolkit) RegisterAgentSkill(skillDir string) error {
	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return fmt.Errorf("resolve skill directory path: %w", err)
	}

	skillMDPath := filepath.Join(absDir, "SKILL.md")
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return fmt.Errorf("read SKILL.md in %q: %w", absDir, err)
	}

	name, description, err := parseFrontMatter(data)
	if err != nil {
		return fmt.Errorf("parse SKILL.md front matter in %q: %w", absDir, err)
	}

	if name == "" {
		return fmt.Errorf("SKILL.md in %q: missing required field \"name\"", absDir)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.skills[name] = &AgentSkill{
		Name:        name,
		Description: description,
		Dir:         absDir,
	}

	return nil
}

func (t *Toolkit) RemoveAgentSkill(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.skills[name]; !exists {
		return fmt.Errorf("agent skill %q not found", name)
	}
	delete(t.skills, name)
	return nil
}

func (t *Toolkit) GetAgentSkillPrompt() string {
	return t.GetAgentSkillPromptWithTemplate("", "")
}

func (t *Toolkit) GetAgentSkillPromptWithTemplate(instruction, skillTemplate string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.skills) == 0 {
		return ""
	}

	if instruction == "" {
		instruction = defaultAgentSkillInstruction
	}
	if skillTemplate == "" {
		skillTemplate = defaultAgentSkillTemplate
	}

	var sb strings.Builder
	sb.WriteString(instruction)
	sb.WriteString("\n\n")

	for _, skill := range t.skills {
		sb.WriteString(fmt.Sprintf(skillTemplate, skill.Name, skill.Description, skill.Dir))
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String())
}

func (t *Toolkit) GetAgentSkills() []*AgentSkill {
	t.mu.RLock()
	defer t.mu.RUnlock()

	skills := make([]*AgentSkill, 0, len(t.skills))
	for _, s := range t.skills {
		skills = append(skills, s)
	}
	return skills
}

func parseFrontMatter(data []byte) (name, description string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	if !scanner.Scan() {
		return "", "", fmt.Errorf("empty content")
	}
	line := strings.TrimSpace(scanner.Text())
	if line != "---" {
		return "", "", fmt.Errorf("expected opening \"---\", got %q", line)
	}

	fields := make(map[string]string)
	for scanner.Scan() {
		line = strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := parseKVLine(line)
		if !ok {
			continue
		}
		fields[key] = value
	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("scan content: %w", err)
	}

	name = fields["name"]
	description = fields["description"]
	return name, description, nil
}

func parseKVLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}

	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, true
}
