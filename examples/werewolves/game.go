package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/pipeline"
)

func WerewolvesGame(ctx context.Context, agents []*agent.ReActAgent) error {
	if len(agents) != 9 {
		return fmt.Errorf("werewolves game requires exactly 9 players, got %d", len(agents))
	}

	players := NewPlayers()
	healing, poison := true, true
	firstDay := true

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println(Prompts["new_game"], namesToStr(agents))
	fmt.Println(strings.Repeat("=", 60))
	broadcastAll(ctx, agents, moderatorMsg(
		fmt.Sprintf("%s Now we randomly reassign the roles to each player and inform them of their roles privately.", namesToStr(agents)),
	))

	// --- Role Assignment ---
	roles := []string{"werewolf", "werewolf", "werewolf",
		"villager", "villager", "villager",
		"seer", "witch", "hunter"}
	shuffledAgents := make([]*agent.ReActAgent, len(agents))
	copy(shuffledAgents, agents)
	rand.Shuffle(len(shuffledAgents), func(i, j int) {
		shuffledAgents[i], shuffledAgents[j] = shuffledAgents[j], shuffledAgents[i]
	})
	rand.Shuffle(len(roles), func(i, j int) {
		roles[i], roles[j] = roles[j], roles[i]
	})

	for i, a := range shuffledAgents {
		role := roles[i]
		a.Observe(ctx, moderatorMsg(
			fmt.Sprintf("[%s ONLY] %s, your role is %s.", a.Name(), a.Name(), role),
		))
		players.AddPlayer(a, role)
	}
	players.PrintRoles()

	// --- Main Game Loop ---
	for round := 0; round < maxGameRounds; round++ {
		fmt.Printf("\n%s Round %d %s\n", strings.Repeat("=", 20), round+1, strings.Repeat("=", 20))

		// ===== Night Phase =====
		fmt.Println("\n--- Night Phase ---")
		broadcastAll(ctx, players.CurrentAlive, moderatorMsg(Prompts["night"]))

		var killedPlayer, poisonedPlayer, shotPlayer string

		// --- Werewolf Phase ---
		if len(players.Werewolves) > 0 {
			fmt.Println("\n[Werewolves discussing...]")
			wolfDiscussMsg := moderatorMsg(fmt.Sprintf(Prompts["wolf_discuss"],
				namesToStr(players.Werewolves),
				namesToStr(players.CurrentAlive),
			))
			for _, w := range players.Werewolves {
				w.Observe(ctx, wolfDiscussMsg)
			}

			nWolves := len(players.Werewolves)
			for r := 1; r <= maxDiscussionRounds*nWolves; r++ {
				currentWolf := players.Werewolves[(r-1)%nWolves]
				resp, err := currentWolf.Reply(ctx, wolfDiscussMsg)
				if err != nil {
					fmt.Printf("  [WARN] %s failed to discuss: %v\n", currentWolf.Name(), err)
					continue
				}
				fmt.Printf("  [%s]: %s\n", currentWolf.Name(), resp.GetTextContent())
				broadcastToOthers(ctx, currentWolf, resp, players.Werewolves)
			}

			// Werewolf voting (parallel, private)
			aliveNames := agentNames(players.CurrentAlive)
			voteMsg := moderatorMsg(Prompts["wolf_vote"])
			voteResults, err := pipeline.FanoutPipeline(ctx, toAgentBases(players.Werewolves), voteMsg)
			if err != nil {
				return fmt.Errorf("werewolf vote failed: %w", err)
			}
			votes := make([]string, len(voteResults))
			for i, resp := range voteResults {
				votes[i] = extractPlayerName(resp.GetTextContent(), aliveNames)
				fmt.Printf("  [%s] voted for: %s\n", players.Werewolves[i].Name(), votes[i])
			}
			killedPlayer, voteSummary := majorityVote(votes)

			wolfResultMsg := moderatorMsg(fmt.Sprintf(Prompts["wolf_result"], voteSummary, killedPlayer))
			broadcastAll(ctx, players.Werewolves, wolfResultMsg)
			fmt.Printf("  Werewolves chose to kill: %s\n", killedPlayer)
		}

		// --- Witch Phase ---
		if len(players.Witch) > 0 {
			fmt.Println("\n[Witch's turn...]")
			broadcastAll(ctx, players.CurrentAlive, moderatorMsg(Prompts["witch_turn"]))

			for _, witchAgent := range players.Witch {
				witchResurrected := false

				if healing && killedPlayer != "" && killedPlayer != witchAgent.Name() {
					resurrectMsg := moderatorMsg(fmt.Sprintf(Prompts["witch_resurrect"],
						"witch_name", witchAgent.Name(),
						"dead_name", killedPlayer,
					))
					resp, err := witchAgent.Reply(ctx, resurrectMsg)
					if err != nil {
						fmt.Printf("  [WARN] Witch resurrection failed: %v\n", err)
					} else {
						text := strings.ToUpper(resp.GetTextContent())
						if strings.Contains(text, "YES") {
							killedPlayer = ""
							healing = false
							witchResurrected = true
							fmt.Println("  Witch used healing potion!")
						}
					}
				}

				if poison && !witchResurrected {
					poisonMsg := moderatorMsg(fmt.Sprintf(Prompts["witch_poison"],
						"witch_name", witchAgent.Name(),
					))
					resp, err := witchAgent.Reply(ctx, poisonMsg)
					if err != nil {
						fmt.Printf("  [WARN] Witch poison failed: %v\n", err)
					} else {
						text := resp.GetTextContent()
						aliveNames := agentNames(players.CurrentAlive)
						target := extractPlayerName(text, aliveNames)
						if target != "" && !strings.EqualFold(strings.TrimSpace(text), "NO") {
							poisonedPlayer = target
							poison = false
							fmt.Printf("  Witch poisoned: %s\n", poisonedPlayer)
						}
					}
				}
			}
		}

		// --- Seer Phase ---
		if len(players.Seer) > 0 {
			fmt.Println("\n[Seer's turn...]")
			broadcastAll(ctx, players.CurrentAlive, moderatorMsg(Prompts["seer_turn"]))

			for _, seerAgent := range players.Seer {
				aliveNames := agentNames(players.CurrentAlive)
				checkMsg := moderatorMsg(fmt.Sprintf(Prompts["seer_check"],
					seerAgent.Name(), namesToStr(players.CurrentAlive),
				))
				resp, err := seerAgent.Reply(ctx, checkMsg)
				if err != nil {
					fmt.Printf("  [WARN] Seer check failed: %v\n", err)
					continue
				}
				target := extractPlayerName(resp.GetTextContent(), aliveNames)
				if target != "" {
					role := players.NameToRole[target]
					resultMsg := moderatorMsg(fmt.Sprintf(Prompts["seer_result"],
						target, role,
					))
					seerAgent.Observe(ctx, resultMsg)
					fmt.Printf("  Seer checked %s\n", target)
				}
			}
		}

		// --- Hunter Phase (Night) ---
		for _, hunterAgent := range players.Hunter {
			if killedPlayer == hunterAgent.Name() && poisonedPlayer != hunterAgent.Name() {
				shot, err := hunterStage(ctx, hunterAgent, players.CurrentAlive)
				if err != nil {
					fmt.Printf("  [WARN] Hunter stage failed: %v\n", err)
				} else if shot != "" {
					shotPlayer = shot
					fmt.Printf("  Hunter shot: %s\n", shotPlayer)
				}
			}
		}

			deadTonight := []string{}
		for _, name := range []string{killedPlayer, poisonedPlayer, shotPlayer} {
			if name != "" {
				deadTonight = append(deadTonight, name)
			}
		}
		players.UpdatePlayers(deadTonight)

		// ===== Day Phase =====
		fmt.Println("\n--- Day Phase ---")
		if len(deadTonight) > 0 {
			broadcastAll(ctx, players.CurrentAlive, moderatorMsg(
				fmt.Sprintf(Prompts["day"], nameListToStr(deadTonight)),
			))
			fmt.Printf("Last night eliminated: %s\n", nameListToStr(deadTonight))

			// First day: killed player gets last words
			if killedPlayer != "" && firstDay {
				deadMsg := moderatorMsg(fmt.Sprintf(Prompts["dead_player"], killedPlayer))
				broadcastAll(ctx, players.CurrentAlive, deadMsg)
				if deadAgent, ok := players.NameToAgent[killedPlayer]; ok {
					lastWords, err := deadAgent.Reply(ctx, deadMsg)
					if err == nil {
						fmt.Printf("  [%s last words]: %s\n", killedPlayer, lastWords.GetTextContent())
						broadcastAll(ctx, players.CurrentAlive, lastWords)
					}
				}
			}
		} else {
			broadcastAll(ctx, players.CurrentAlive, moderatorMsg(Prompts["peace"]))
			fmt.Println("Peaceful night, no one was eliminated.")
		}

		// Check win after night
		if result := players.CheckWinning(); result != "" {
			fmt.Println("\n" + strings.Repeat("=", 60))
			fmt.Println(result)
			broadcastAll(ctx, players.AllPlayers, moderatorMsg(result))
			break
		}

		// --- Day Discussion ---
		fmt.Println("\n[Day Discussion]")
		aliveNames := agentNames(players.CurrentAlive)
		discussMsg := moderatorMsg(fmt.Sprintf(Prompts["discuss"],
			namesToStr(players.CurrentAlive), namesToStr(players.CurrentAlive),
		))
		broadcastAll(ctx, players.CurrentAlive, discussMsg)

		for i, p := range players.CurrentAlive {
			var replyMsg *message.Msg
			var err error
			if i == 0 {
				replyMsg, err = p.Reply(ctx, discussMsg)
			} else {
				replyMsg, err = p.Reply(ctx, moderatorMsg(
					fmt.Sprintf("Now it's your turn to speak, %s.", p.Name()),
				))
			}
			if err != nil {
				fmt.Printf("  [WARN] %s failed to speak: %v\n", p.Name(), err)
				continue
			}
			fmt.Printf("  [%s]: %s\n", p.Name(), replyMsg.GetTextContent())
			broadcastToOthers(ctx, p, replyMsg, players.CurrentAlive)
		}

		// --- Day Voting ---
		fmt.Println("\n[Day Voting]")
		aliveNames = agentNames(players.CurrentAlive)
		voteMsg := moderatorMsg(fmt.Sprintf(Prompts["vote"], nameListToStr(aliveNames)))
		voteResults, err := pipeline.FanoutPipeline(ctx, toAgentBases(players.CurrentAlive), voteMsg)
		if err != nil {
			return fmt.Errorf("day vote failed: %w", err)
		}

		votes := make([]string, len(voteResults))
		for i, resp := range voteResults {
			votes[i] = extractPlayerName(resp.GetTextContent(), aliveNames)
			fmt.Printf("  [%s] voted for: %s\n", players.CurrentAlive[i].Name(), votes[i])
		}
		votedPlayer, voteSummary := majorityVote(votes)

		voteResultMsg := moderatorMsg(fmt.Sprintf(Prompts["vote_result"], voteSummary, votedPlayer))
		fmt.Printf("Voting result: %s -> %s eliminated\n", voteSummary, votedPlayer)

		if votedPlayer != "" {
			deadMsg := moderatorMsg(fmt.Sprintf(Prompts["dead_player"], votedPlayer))
			if deadAgent, ok := players.NameToAgent[votedPlayer]; ok {
				lastWords, err := deadAgent.Reply(ctx, deadMsg)
				if err == nil {
					voteResultMsg2 := moderatorMsg(fmt.Sprintf("Last words from %s: %s", votedPlayer, lastWords.GetTextContent()))
					broadcastAll(ctx, players.CurrentAlive, voteResultMsg)
					broadcastAll(ctx, players.CurrentAlive, voteResultMsg2)
					fmt.Printf("  [%s last words]: %s\n", votedPlayer, lastWords.GetTextContent())
				}
			}
		}

		// --- Hunter Phase (Day) ---
		shotPlayer = ""
		for _, hunterAgent := range players.Hunter {
			if votedPlayer == hunterAgent.Name() {
				shot, err := hunterStage(ctx, hunterAgent, players.CurrentAlive)
				if err != nil {
					fmt.Printf("  [WARN] Hunter day stage failed: %v\n", err)
				} else if shot != "" {
					shotPlayer = shot
					shootMsg := moderatorMsg(fmt.Sprintf(Prompts["hunter_shoot"], shotPlayer))
					broadcastAll(ctx, players.CurrentAlive, shootMsg)
					fmt.Printf("  Hunter shot: %s\n", shotPlayer)
				}
			}
		}

		var deadToday []string
		for _, name := range []string{votedPlayer, shotPlayer} {
			if name != "" {
				deadToday = append(deadToday, name)
			}
		}
		players.UpdatePlayers(deadToday)

		// Check win after day
		if result := players.CheckWinning(); result != "" {
			fmt.Println("\n" + strings.Repeat("=", 60))
			fmt.Println(result)
			broadcastAll(ctx, players.AllPlayers, moderatorMsg(result))
			break
		}

		firstDay = false
	}

	fmt.Println("\n[Game Over - Reflection]")
	reflectMsg := moderatorMsg(Prompts["reflect"])
	pipeline.FanoutPipeline(ctx, toAgentBases(players.AllPlayers), reflectMsg)

	return nil
}

func hunterStage(ctx context.Context, hunterAgent *agent.ReActAgent, alivePlayers []*agent.ReActAgent) (string, error) {
	aliveNames := agentNames(alivePlayers)
	hunterMsg := moderatorMsg(fmt.Sprintf(Prompts["hunter"], hunterAgent.Name()))
	resp, err := hunterAgent.Reply(ctx, hunterMsg)
	if err != nil {
		return "", fmt.Errorf("hunter reply failed: %w", err)
	}

	text := strings.TrimSpace(resp.GetTextContent())
	if strings.EqualFold(text, "SKIP") {
		fmt.Printf("  Hunter %s chose not to shoot.\n", hunterAgent.Name())
		return "", nil
	}

	target := extractPlayerName(text, aliveNames)
	if target != "" {
		return target, nil
	}
	return "", nil
}

func agentNames(agents []*agent.ReActAgent) []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name()
	}
	return names
}
