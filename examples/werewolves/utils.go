package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/pipeline"
)

const (
	maxGameRounds       = 30
	maxDiscussionRounds = 3
)

// Players tracks the state of all players in the game.
type Players struct {
	NameToRole   map[string]string
	RoleToNames  map[string][]string
	NameToAgent  map[string]*agent.ReActAgent
	Werewolves   []*agent.ReActAgent
	Villagers    []*agent.ReActAgent
	Seer         []*agent.ReActAgent
	Hunter       []*agent.ReActAgent
	Witch        []*agent.ReActAgent
	CurrentAlive []*agent.ReActAgent
	AllPlayers   []*agent.ReActAgent
}

func NewPlayers() *Players {
	return &Players{
		NameToRole:  make(map[string]string),
		RoleToNames: make(map[string][]string),
		NameToAgent: make(map[string]*agent.ReActAgent),
	}
}

func (p *Players) AddPlayer(player *agent.ReActAgent, role string) {
	name := player.Name()
	p.NameToRole[name] = role
	p.NameToAgent[name] = player
	p.RoleToNames[role] = append(p.RoleToNames[role], name)
	p.AllPlayers = append(p.AllPlayers, player)
	p.CurrentAlive = append(p.CurrentAlive, player)

	switch role {
	case "werewolf":
		p.Werewolves = append(p.Werewolves, player)
	case "villager":
		p.Villagers = append(p.Villagers, player)
	case "seer":
		p.Seer = append(p.Seer, player)
	case "hunter":
		p.Hunter = append(p.Hunter, player)
	case "witch":
		p.Witch = append(p.Witch, player)
	}
}

func (p *Players) UpdatePlayers(deadPlayerNames []string) {
	deadSet := make(map[string]bool)
	for _, name := range deadPlayerNames {
		if name != "" {
			deadSet[name] = true
		}
	}
	if len(deadSet) == 0 {
		return
	}

	p.Werewolves = filterDead(p.Werewolves, deadSet)
	p.Villagers = filterDead(p.Villagers, deadSet)
	p.Seer = filterDead(p.Seer, deadSet)
	p.Hunter = filterDead(p.Hunter, deadSet)
	p.Witch = filterDead(p.Witch, deadSet)
	p.CurrentAlive = filterDead(p.CurrentAlive, deadSet)
}

func filterDead(agents []*agent.ReActAgent, dead map[string]bool) []*agent.ReActAgent {
	var result []*agent.ReActAgent
	for _, a := range agents {
		if !dead[a.Name()] {
			result = append(result, a)
		}
	}
	return result
}

func (p *Players) PrintRoles() {
	fmt.Println("=== Role Assignment ===")
	for name, role := range p.NameToRole {
		fmt.Printf("  %s: %s\n", name, role)
	}
	fmt.Println()
}

// CheckWinning returns a win message if the game is over, or empty string if it continues.
func (p *Players) CheckWinning() string {
	trueRoles := fmt.Sprintf(
		"%s are werewolves, %s are villagers, %s is the seer, %s is the hunter, and %s is the witch.",
		nameListToStr(p.RoleToNames["werewolf"]),
		nameListToStr(p.RoleToNames["villager"]),
		nameListToStr(p.RoleToNames["seer"]),
		nameListToStr(p.RoleToNames["hunter"]),
		nameListToStr(p.RoleToNames["witch"]),
	)

	if len(p.Werewolves)*2 >= len(p.CurrentAlive) {
		return fmt.Sprintf("There are %d players alive, and %d of them are werewolves. "+
			"The game is over and werewolves win! "+
			"In this game, the true roles of all players are: %s",
			len(p.CurrentAlive), len(p.Werewolves), trueRoles)
	}

	if len(p.CurrentAlive) > 0 && len(p.Werewolves) == 0 {
		return fmt.Sprintf("All the werewolves have been eliminated. "+
			"The game is over and villagers win! "+
			"In this game, the true roles of all players are: %s", trueRoles)
	}

	return ""
}

func majorityVote(votes []string) (winner string, summary string) {
	counts := make(map[string]int)
	for _, v := range votes {
		counts[v]++
	}

	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	var winners []string
	for name, c := range counts {
		if c == maxCount {
			winners = append(winners, name)
		}
	}
	sort.Strings(winners)
	winner = winners[0]

	var parts []string
	for name, c := range counts {
		parts = append(parts, fmt.Sprintf("%s: %d", name, c))
	}
	sort.Strings(parts)
	summary = strings.Join(parts, ", ")
	return
}

func namesToStr(agents []*agent.ReActAgent) string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name()
	}
	return nameListToStr(names)
}

func nameListToStr(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func extractPlayerName(response string, validNames []string) string {
	for _, name := range validNames {
		if strings.Contains(response, name) {
			return name
		}
	}
	return ""
}

func moderatorMsg(content string) *message.Msg {
	return message.NewMsg("Moderator", content, "assistant")
}

func toAgentBases(agents []*agent.ReActAgent) []agent.AgentBase {
	bases := make([]agent.AgentBase, len(agents))
	for i, a := range agents {
		bases[i] = a
	}
	return bases
}

func broadcastToOthers(ctx context.Context, sender *agent.ReActAgent, msg *message.Msg, recipients []*agent.ReActAgent) error {
	for _, r := range recipients {
		if r.ID() != sender.ID() {
			if err := r.Observe(ctx, msg); err != nil {
				return fmt.Errorf("observe failed for %s: %w", r.Name(), err)
			}
		}
	}
	return nil
}

func broadcastAll(ctx context.Context, agents []*agent.ReActAgent, msg *message.Msg) error {
	hub := pipeline.NewMsgHub(toAgentBases(agents), nil)
	return hub.Broadcast(ctx, msg)
}
