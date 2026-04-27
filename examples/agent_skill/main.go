// This example demonstrates the agent skill system:
// registering skills from SKILL.md directories, generating prompts,
// removing skills, and using custom templates.
//
// Run: go run ./examples/agent_skill
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/vearne/agentscope-go/pkg/tool"
)

func main() {
	tk := tool.NewToolkit()

	skillDir := func(name string) string {
		return filepath.Join(".", name)
	}

	fmt.Println("=== Register Agent Skills ===")
	if err := tk.RegisterAgentSkill(skillDir("weather_skill")); err != nil {
		log.Fatalf("register weather_skill: %v", err)
	}
	fmt.Println("  Registered: weather_skill")

	if err := tk.RegisterAgentSkill(skillDir("code_review_skill")); err != nil {
		log.Fatalf("register code_review_skill: %v", err)
	}
	fmt.Println("  Registered: code_review_skill")

	fmt.Println("\n=== Registered Skills ===")
	for _, s := range tk.GetAgentSkills() {
		fmt.Printf("  - %s: %s\n", s.Name, s.Description)
	}

	fmt.Println("\n=== Default Skill Prompt ===")
	prompt := tk.GetAgentSkillPrompt()
	fmt.Println(prompt)

	fmt.Println("\n=== Custom Template Prompt ===")
	customPrompt := tk.GetAgentSkillPromptWithTemplate(
		"# Available Skills\nUse these skills to assist the user.",
		"- **%s**: %s (see %s/SKILL.md)",
	)
	fmt.Println(customPrompt)

	fmt.Println("\n=== Remove 'Weather Query' ===")
	if err := tk.RemoveAgentSkill("Weather Query"); err != nil {
		log.Fatalf("remove skill: %v", err)
	}
	fmt.Println("  Removed.")

	fmt.Println("\n=== Skills After Removal ===")
	for _, s := range tk.GetAgentSkills() {
		fmt.Printf("  - %s: %s\n", s.Name, s.Description)
	}

	fmt.Println("\n=== Remove 'Code Review' ===")
	tk.RemoveAgentSkill("Code Review")
	if tk.GetAgentSkillPrompt() == "" {
		fmt.Println("  (prompt is empty after removing all skills)")
	}

	fmt.Println("\n=== Error Handling ===")
	err := tk.RegisterAgentSkill("./nonexistent_dir")
	fmt.Printf("  Missing dir: %v\n", err)

	tmpDir := os.TempDir()
	err = tk.RegisterAgentSkill(tmpDir)
	fmt.Printf("  Missing SKILL.md: %v\n", err)
}
