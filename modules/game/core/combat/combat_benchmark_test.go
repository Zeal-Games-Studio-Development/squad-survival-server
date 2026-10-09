package combat

import (
	"fmt"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func BenchmarkCombatCandidateAcquisition32Players416Characters(b *testing.B) {
	for _, scenario := range []struct {
		name   string
		nearby bool
	}{
		{name: "concentrated", nearby: true},
		{name: "dispersed", nearby: false},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			players := benchmarkPlayers(32, 13)
			candidates := make(map[string][]*entity.Player, len(players))
			if scenario.nearby {
				candidates = allNearby(players)
			}
			simulation := NewSimulation()
			random := rand.New(rand.NewSource(1))
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				for _, player := range players {
					for _, character := range player.Characters {
						character.ResetAttack()
						character.Health = 100
					}
				}
				simulation.Step(players, candidates, 1, random)
			}
		})
	}
}

// Each operation starts 416 sword skills and advances the simulation through their impact tick.
func BenchmarkCombatSwordSkillStartAndImpact32Players416Characters(b *testing.B) {
	players := benchmarkPlayers(32, 13)
	candidates := allNearby(players)
	for _, player := range players {
		for _, character := range player.Characters {
			if err := character.StartCooldown("sword_cone", entity.CooldownTicks, 30); err != nil {
				b.Fatal(err)
			}
		}
	}

	reset := func() {
		for _, player := range players {
			for _, character := range player.Characters {
				character.ResetAttack()
				character.ResetSkill()
				character.Health = 1_000_000
				character.Cooldowns["sword_cone"].Progress = 30
			}
		}
	}

	// Confirm this setup exercises both skill phases before timing it.
	reset()
	check := NewSimulation()
	random := rand.New(rand.NewSource(1))
	starts := check.Step(players, candidates, 1, random)
	started := 0
	for _, event := range starts {
		if event.Type == EventSkillStarted {
			started++
		}
	}
	check.Step(players, candidates, 2, random)
	impacts := check.Step(players, candidates, 3, random)
	hits := 0
	for _, event := range impacts {
		if event.Type == EventDamageApplied && entity.HasActionTag(event.Tags, entity.TagSkill) {
			hits++
		}
	}
	if started != 416 || hits == 0 {
		b.Fatalf("benchmark setup did not exercise skills: started=%d hits=%d", started, hits)
	}

	simulation := NewSimulation()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		reset()
		simulation.Step(players, candidates, 1, random)
		simulation.Step(players, candidates, 2, random)
		simulation.Step(players, candidates, 3, random)
	}
}

func benchmarkPlayers(playerCount, characterCount int) map[string]*entity.Player {
	players := make(map[string]*entity.Player, playerCount)
	for playerIndex := 0; playerIndex < playerCount; playerIndex++ {
		sessionID := fmt.Sprintf("session-%d", playerIndex)
		player := &entity.Player{UserID: fmt.Sprintf("user-%d", playerIndex), SessionID: sessionID, DetectionRadius: entity.DefaultDetectionRadius}
		for characterIndex := 0; characterIndex < characterCount; characterIndex++ {
			player.Characters = append(player.Characters, &entity.Character{
				ID: fmt.Sprintf("%s:%d", player.UserID, characterIndex), Position: entity.Vector2{X: float64(playerIndex % 4), Y: float64(playerIndex / 4)},
				Health: 100, Damage: 1, CritMultiplier: 1.5, AttackSpeed: 1, AttackRange: 8, ImpactRatio: 0.5,
				RangeClass: entity.RangeMelee, Weapon: entity.Weapon{Type: entity.WeaponSword, ID: "sword"},
			})
		}
		players[sessionID] = player
	}
	return players
}
