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

func benchmarkPlayers(playerCount, characterCount int) map[string]*entity.Player {
	players := make(map[string]*entity.Player, playerCount)
	for playerIndex := 0; playerIndex < playerCount; playerIndex++ {
		sessionID := fmt.Sprintf("session-%d", playerIndex)
		player := &entity.Player{UserID: fmt.Sprintf("user-%d", playerIndex), SessionID: sessionID, DetectionRadius: entity.DefaultDetectionRadius}
		for characterIndex := 0; characterIndex < characterCount; characterIndex++ {
			player.Characters = append(player.Characters, &entity.Character{
				ID: fmt.Sprintf("%s:%d", player.UserID, characterIndex), Position: entity.Vector2{X: float64(playerIndex % 4), Y: float64(playerIndex / 4)},
				Health: 100, Damage: 1, DamageRatio: 1, AttackSpeed: 1, AttackRange: 8, ImpactRatio: 0.5,
				RangeClass: entity.RangeMelee, Weapon: entity.Weapon{Type: entity.WeaponSword, Name: "sword"},
			})
		}
		players[sessionID] = player
	}
	return players
}
