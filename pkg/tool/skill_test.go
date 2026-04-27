package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkillMD(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestRegisterAgentSkill(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "weather_skill")
	writeSkillMD(t, dir, `---
name: Weather Query
description: Query weather for any city.
---

# Weather Query
Use the shell to run `+"`curl wttr.in/{city}`"+` for weather info.
`)

	tk := NewToolkit()
	if err := tk.RegisterAgentSkill(dir); err != nil {
		t.Fatalf("RegisterAgentSkill failed: %v", err)
	}

	skills := tk.GetAgentSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Name != "Weather Query" {
		t.Errorf("name = %q, want %q", s.Name, "Weather Query")
	}
	if s.Description != "Query weather for any city." {
		t.Errorf("description = %q", s.Description)
	}
	if s.Dir != dir {
		t.Errorf("dir = %q, want %q", s.Dir, dir)
	}
}

func TestRegisterAgentSkill_MissingSKILLMD(t *testing.T) {
	dir := t.TempDir()
	tk := NewToolkit()
	err := tk.RegisterAgentSkill(dir)
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
	if !strings.Contains(err.Error(), "SKILL.md") {
		t.Errorf("error should mention SKILL.md: %v", err)
	}
}

func TestRegisterAgentSkill_MissingName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad_skill")
	writeSkillMD(t, dir, `---
description: A skill without a name.
---

No name here.
`)

	tk := NewToolkit()
	err := tk.RegisterAgentSkill(dir)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention name: %v", err)
	}
}

func TestRegisterAgentSkill_BadFrontMatter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad_fm")
	writeSkillMD(t, dir, "no front matter here at all\n")

	tk := NewToolkit()
	err := tk.RegisterAgentSkill(dir)
	if err == nil {
		t.Fatal("expected error for bad front matter")
	}
}

func TestRegisterAgentSkill_QuotedValues(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "quoted_skill")
	writeSkillMD(t, dir, `---
name: "My Skill"
description: 'A skill with: colons, and "quotes"'
---

Content here.
`)

	tk := NewToolkit()
	if err := tk.RegisterAgentSkill(dir); err != nil {
		t.Fatalf("RegisterAgentSkill failed: %v", err)
	}

	skills := tk.GetAgentSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "My Skill" {
		t.Errorf("name = %q, want %q", skills[0].Name, "My Skill")
	}
	if skills[0].Description != "A skill with: colons, and \"quotes\"" {
		t.Errorf("description = %q", skills[0].Description)
	}
}

func TestRemoveAgentSkill(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skill")
	writeSkillMD(t, dir, `---
name: Test Skill
description: A test.
---

Content.
`)

	tk := NewToolkit()
	tk.RegisterAgentSkill(dir)

	if err := tk.RemoveAgentSkill("Test Skill"); err != nil {
		t.Fatalf("RemoveAgentSkill failed: %v", err)
	}
	if len(tk.GetAgentSkills()) != 0 {
		t.Fatal("skill should be removed")
	}
}

func TestRemoveAgentSkill_NotFound(t *testing.T) {
	tk := NewToolkit()
	err := tk.RemoveAgentSkill("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestGetAgentSkillPrompt_Empty(t *testing.T) {
	tk := NewToolkit()
	if prompt := tk.GetAgentSkillPrompt(); prompt != "" {
		t.Errorf("expected empty prompt, got %q", prompt)
	}
}

func TestGetAgentSkillPrompt_SingleSkill(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skill")
	writeSkillMD(t, dir, `---
name: Weather Query
description: Get weather info.
---

Weather instructions.
`)

	tk := NewToolkit()
	tk.RegisterAgentSkill(dir)

	prompt := tk.GetAgentSkillPrompt()
	if !strings.Contains(prompt, "# Agent Skills") {
		t.Error("prompt should contain instruction header")
	}
	if !strings.Contains(prompt, "Weather Query") {
		t.Error("prompt should contain skill name")
	}
	if !strings.Contains(prompt, "Get weather info.") {
		t.Error("prompt should contain skill description")
	}
	if !strings.Contains(prompt, "SKILL.md") {
		t.Error("prompt should reference SKILL.md")
	}
}

func TestGetAgentSkillPrompt_MultipleSkills(t *testing.T) {
	base := t.TempDir()

	dir1 := filepath.Join(base, "skill_a")
	writeSkillMD(t, dir1, `---
name: Skill Alpha
description: First skill.
---

Alpha content.
`)

	dir2 := filepath.Join(base, "skill_b")
	writeSkillMD(t, dir2, `---
name: Skill Beta
description: Second skill.
---

Beta content.
`)

	tk := NewToolkit()
	tk.RegisterAgentSkill(dir1)
	tk.RegisterAgentSkill(dir2)

	prompt := tk.GetAgentSkillPrompt()
	if !strings.Contains(prompt, "Skill Alpha") {
		t.Error("prompt should contain Skill Alpha")
	}
	if !strings.Contains(prompt, "Skill Beta") {
		t.Error("prompt should contain Skill Beta")
	}
}

func TestGetAgentSkillPromptWithTemplate_Custom(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skill")
	writeSkillMD(t, dir, `---
name: MySkill
description: Does things.
---

Content.
`)

	tk := NewToolkit()
	tk.RegisterAgentSkill(dir)

	custom := "SKILL: %s | DESC: %s | PATH: %s"
	prompt := tk.GetAgentSkillPromptWithTemplate("Custom Header", custom)

	if !strings.Contains(prompt, "Custom Header") {
		t.Error("prompt should use custom instruction")
	}
	if !strings.Contains(prompt, "SKILL: MySkill") {
		t.Error("prompt should use custom template")
	}
	if strings.Contains(prompt, "# Agent Skills") {
		t.Error("prompt should NOT contain default instruction")
	}
}

func TestParseFrontMatter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectName  string
		expectDesc  string
		expectError bool
	}{
		{
			name:       "basic",
			input:      "---\nname: Foo\ndescription: Bar\n---\nBody",
			expectName: "Foo",
			expectDesc: "Bar",
		},
		{
			name:       "extra_fields_ignored",
			input:      "---\nname: Foo\nversion: 1.0\ndescription: Bar\n---\n",
			expectName: "Foo",
			expectDesc: "Bar",
		},
		{
			name:       "quoted",
			input:      "---\nname: \"Foo Bar\"\ndescription: 'Baz Qux'\n---\n",
			expectName: "Foo Bar",
			expectDesc: "Baz Qux",
		},
		{
			name:       "colon_in_value",
			input:      "---\nname: Foo\ndescription: A:B:C\n---\n",
			expectName: "Foo",
			expectDesc: "A:B:C",
		},
		{
			name:        "no_opening_delimiter",
			input:       "name: Foo\ndescription: Bar\n---\n",
			expectError: true,
		},
		{
			name:        "empty",
			input:       "",
			expectError: true,
		},
		{
			name:       "comments_skipped",
			input:      "---\n# comment\nname: Foo\ndescription: Bar\n---\n",
			expectName: "Foo",
			expectDesc: "Bar",
		},
		{
			name:       "blank_lines_skipped",
			input:      "---\n\nname: Foo\n\ndescription: Bar\n\n---\n",
			expectName: "Foo",
			expectDesc: "Bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc, err := parseFrontMatter([]byte(tt.input))
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.expectName {
				t.Errorf("name = %q, want %q", name, tt.expectName)
			}
			if desc != tt.expectDesc {
				t.Errorf("description = %q, want %q", desc, tt.expectDesc)
			}
		})
	}
}

func TestRegisterAgentSkillWithTools(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "weather_skill")
	writeSkillMD(t, dir, `---
name: Weather Query
description: Query weather for any city.
---

# Weather Query
Use the shell to run `+"`curl wttr.in/{city}`"+` for weather info.
`)

	tk := NewToolkit()

	if tk.HasTool("view_text_file") {
		t.Fatal("view_text_file should not exist before registration")
	}

	if err := tk.RegisterAgentSkillWithTools(dir); err != nil {
		t.Fatalf("RegisterAgentSkillWithTools failed: %v", err)
	}

	skills := tk.GetAgentSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "Weather Query" {
		t.Errorf("skill name = %q, want %q", skills[0].Name, "Weather Query")
	}

	if !tk.HasTool("view_text_file") {
		t.Error("view_text_file tool should be registered")
	}
}

func TestRegisterAgentSkillWithTools_IdempotentTool(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skill_a")
	writeSkillMD(t, dir, `---
name: Skill A
description: First skill.
---

Content A.
`)

	tk := NewToolkit()

	if err := RegisterViewTextFileTool(tk); err != nil {
		t.Fatalf("manual RegisterViewTextFileTool failed: %v", err)
	}

	if err := tk.RegisterAgentSkillWithTools(dir); err != nil {
		t.Fatalf("RegisterAgentSkillWithTools should not fail when tool already exists: %v", err)
	}

	if !tk.HasTool("view_text_file") {
		t.Error("view_text_file should still be registered")
	}
}

func TestRegisterAgentSkillWithTools_BadDir(t *testing.T) {
	tk := NewToolkit()
	err := tk.RegisterAgentSkillWithTools("./nonexistent_dir")
	if err == nil {
		t.Fatal("expected error for bad skill dir")
	}
	if tk.HasTool("view_text_file") {
		t.Error("view_text_file should not be registered when skill registration fails")
	}
}

func TestRegisterAgentSkillWithTools_Multiple(t *testing.T) {
	base := t.TempDir()

	dir1 := filepath.Join(base, "skill_a")
	writeSkillMD(t, dir1, `---
name: Skill A
description: First.
---

A content.
`)

	dir2 := filepath.Join(base, "skill_b")
	writeSkillMD(t, dir2, `---
name: Skill B
description: Second.
---

B content.
`)

	tk := NewToolkit()
	tk.RegisterAgentSkillWithTools(dir1)
	tk.RegisterAgentSkillWithTools(dir2)

	skills := tk.GetAgentSkills()
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	if !tk.HasTool("view_text_file") {
		t.Error("view_text_file should be registered (only once)")
	}
}

func TestRegisterAgentSkill_DuplicateOverwrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skill")
	writeSkillMD(t, dir, `---
name: Unique
description: First.
---

First.
`)

	tk := NewToolkit()
	tk.RegisterAgentSkill(dir)

	writeSkillMD(t, dir, `---
name: Unique
description: Second.
---

Second.
`)
	tk.RegisterAgentSkill(dir)

	skills := tk.GetAgentSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (overwritten), got %d", len(skills))
	}
	if skills[0].Description != "Second." {
		t.Errorf("description = %q, want %q", skills[0].Description, "Second.")
	}
}
