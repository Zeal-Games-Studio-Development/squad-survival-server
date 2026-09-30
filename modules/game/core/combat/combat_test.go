package combat

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/spatial"
)

func TestMeleeLocksNearestTargetAndSchedulesAttack(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	far := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 1.5}, 2, 0.5)
	near := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)

	players := playerMap(attacker, far, near)
	events := NewSimulation().Step(players, allNearby(players), 10, rand.New(rand.NewSource(1)))
	started := findEvent(events, EventAttackStarted, "a:1")
	if started == nil || started.TargetCharacterID != "b:1" {
		t.Fatalf("expected nearest target b:1, got %+v", started)
	}
	if started.StartTick != 10 || started.ImpactTick != 13 || started.CompleteTick != 15 {
		t.Fatalf("unexpected attack timing: %+v", started)
	}
}

func TestTargetTieBreaksByUserAndCharacterID(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 3, 0.5)
	targetB2 := combatPlayer("b", "b:2", entity.RangeMelee, entity.Vector2{X: 1}, 1, 0.5)
	targetB1 := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: -1}, 1, 0.5)

	players := playerMap(attacker, targetB2, targetB1)
	events := NewSimulation().Step(players, allNearby(players), 1, rand.New(rand.NewSource(1)))
	started := findEvent(events, EventAttackStarted, "a:1")
	if started == nil || started.TargetCharacterID != "b:1" {
		t.Fatalf("expected deterministic target b:1, got %+v", started)
	}
}

func TestRangedSpawnsAndHitsWithProjectile(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeRanged, entity.Vector2{}, 10, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 2}, 1, 0.5)
	attacker.Characters[0].AttackSpeed = 5
	attacker.Characters[0].Weapon.ProjectileSpeed = 20
	attacker.Characters[0].Weapon.Name = "bow"
	attacker.Characters[0].Weapon.Type = entity.WeaponBow
	random := rand.New(rand.NewSource(1))
	players := playerMap(attacker, target)
	simulation := NewSimulation()

	nearby := allNearby(players)
	if event := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, "a:1"); event == nil {
		t.Fatal("expected ranged attack to start")
	}
	spawnEvents := simulation.Step(players, nearby, 2, random)
	if event := findEvent(spawnEvents, EventProjectileSpawned, "a:1"); event == nil {
		t.Fatalf("expected projectile spawn, got %+v", spawnEvents)
	}
	if len(simulation.Projectiles()) != 1 {
		t.Fatalf("expected active projectile, got %+v", simulation.Projectiles())
	}
	hitEvents := simulation.Step(players, nearby, 3, random)
	if findEvent(hitEvents, EventProjectileHit, "a:1") == nil || findEvent(hitEvents, EventDamageApplied, "a:1") == nil {
		t.Fatalf("expected projectile hit and damage, got %+v", hitEvents)
	}
	if len(simulation.Projectiles()) != 0 || target.Characters[0].Health >= 100 {
		t.Fatalf("projectile did not resolve: projectiles=%+v health=%f", simulation.Projectiles(), target.Characters[0].Health)
	}
}

func TestProjectileExpiresWhenTargetDiesBeforeHit(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeRanged, entity.Vector2{}, 10, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 8}, 1, 0.5)
	attacker.Characters[0].AttackSpeed = 5
	attacker.Characters[0].Weapon = entity.Weapon{Type: entity.WeaponBow, Name: "bow", ProjectileSpeed: 10}
	random := rand.New(rand.NewSource(1))
	players := playerMap(attacker, target)
	simulation := NewSimulation()
	nearby := allNearby(players)
	simulation.Step(players, nearby, 1, random)
	simulation.Step(players, nearby, 2, random)
	target.Characters[0].Health = 0

	events := simulation.Step(players, nearby, 3, random)
	if findEvent(events, EventProjectileExpired, "a:1") == nil {
		t.Fatalf("expected projectile expired event, got %+v", events)
	}
	if len(simulation.Projectiles()) != 0 {
		t.Fatalf("expired projectile remained active: %+v", simulation.Projectiles())
	}
}

func TestLeavingRangeCancelsAndResetsAttack(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	random := rand.New(rand.NewSource(1))
	players := playerMap(attacker, target)
	simulation := NewSimulation()
	nearby := allNearby(players)
	simulation.Step(players, nearby, 1, random)
	target.Characters[0].Position.X = 3

	events := simulation.Step(players, nearby, 2, random)
	if attacker.Characters[0].TargetCharacterID != "" || attacker.Characters[0].AttackStartTick != 0 {
		t.Fatalf("attack was not reset: %+v", attacker.Characters[0])
	}
	if event := findEvent(events, EventDamageApplied, "a:1"); event != nil {
		t.Fatalf("cancelled attack dealt damage: %+v", event)
	}
}

func TestMovementInputControlsAttackEligibility(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	random := rand.New(rand.NewSource(1))
	players := playerMap(attacker, target)
	simulation := NewSimulation()

	attacker.Direction = entity.Vector2{X: 0.19, Y: -0.19}
	nearby := allNearby(players)
	if findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, "a:1") == nil {
		t.Fatal("expected input below movement threshold to allow attack")
	}
	attacker.Direction = entity.Vector2{X: 0.2}
	events := simulation.Step(players, nearby, 2, random)
	if attacker.Characters[0].TargetCharacterID != "" {
		t.Fatalf("movement did not reset active attack: %+v", attacker.Characters[0])
	}
	if findEvent(events, EventDamageApplied, "a:1") != nil {
		t.Fatalf("movement above threshold allowed damage: %+v", events)
	}
	if findEvent(events, EventAttackStarted, "a:1") != nil {
		t.Fatalf("movement above threshold started another attack: %+v", events)
	}
}

func TestTargetDeathBeforeImpactCancelsAttack(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	random := rand.New(rand.NewSource(1))
	players := playerMap(attacker, target)
	simulation := NewSimulation()
	nearby := allNearby(players)
	simulation.Step(players, nearby, 1, random)
	target.Characters[0].Health = 0

	events := simulation.Step(players, nearby, 2, random)
	if attacker.Characters[0].TargetCharacterID != "" {
		t.Fatalf("dead target remained locked: %+v", attacker.Characters[0])
	}
	if event := findEvent(events, EventDamageApplied, "a:1"); event != nil {
		t.Fatalf("dead target received damage: %+v", event)
	}
}

func TestSimultaneousImpactsCanKillBothCharacters(t *testing.T) {
	playerA := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 1)
	playerB := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 1)
	for _, player := range []*entity.Player{playerA, playerB} {
		character := player.Characters[0]
		character.Health = 5
		character.Damage = 10
		character.DamageRatio = 1
		character.AttackSpeed = 10
	}
	random := rand.New(rand.NewSource(1))
	players := playerMap(playerA, playerB)
	simulation := NewSimulation()
	nearby := allNearby(players)
	simulation.Step(players, nearby, 1, random)
	events := simulation.Step(players, nearby, 2, random)

	if len(playerA.Characters) != 0 || len(playerB.Characters) != 0 {
		t.Fatalf("expected both characters removed: a=%d b=%d", len(playerA.Characters), len(playerB.Characters))
	}
	if countEvents(events, EventDamageApplied) != 2 || countEvents(events, EventCharacterDied) != 2 {
		t.Fatalf("unexpected simultaneous combat events: %+v", events)
	}
}

func TestAttackRangeIncludesBoundary(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 2}, 2, 0.5)

	players := playerMap(attacker, target)
	events := NewSimulation().Step(players, allNearby(players), 1, rand.New(rand.NewSource(1)))
	if findEvent(events, EventAttackStarted, "a:1") == nil {
		t.Fatal("expected target on attack boundary to be selected")
	}
}

func TestTargetAcquisitionUsesOnlySpatialCandidatesAcrossCells(t *testing.T) {
	for _, test := range []struct {
		name   string
		target entity.Vector2
	}{
		{name: "adjacent cell", target: entity.Vector2{X: 20.5, Y: 19.5}},
		{name: "diagonal cell", target: entity.Vector2{X: 20.5, Y: 20.5}},
	} {
		t.Run(test.name, func(t *testing.T) {
			attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{X: 19.5, Y: 19.5}, 2, 0.5)
			target := combatPlayer("b", "b:1", entity.RangeMelee, test.target, 2, 0.5)
			players := playerMap(attacker, target)
			nearby := spatialCandidates(t, players)
			events := NewSimulation().Step(players, nearby, 1, rand.New(rand.NewSource(1)))
			if findEvent(events, EventAttackStarted, "a:1") == nil {
				t.Fatal("expected target in adjacent spatial cell to be acquired")
			}
		})
	}
}

func TestCombatQueryCoversOpposingFiveByFiveFormationCorners(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{X: 3, Y: 3}, 10, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 10, Y: 10}, 1, 0.5)
	attacker.Position = entity.Vector2{}
	target.Position = entity.Vector2{X: 13, Y: 13}
	players := playerMap(attacker, target)
	nearby := spatialCandidates(t, players)
	if len(nearby[attacker.SessionID]) != 1 {
		t.Fatalf("target player center was not detected: %+v", nearby[attacker.SessionID])
	}
	events := NewSimulation().Step(players, nearby, 1, rand.New(rand.NewSource(1)))
	if findEvent(events, EventAttackStarted, "a:1") == nil {
		t.Fatal("characters at opposing 5x5 formation corners were not considered for attack")
	}
}

func TestTargetOutsideCandidateSetIsNotAcquiredAndActiveAttackResets(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	players := playerMap(attacker, target)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	nearby := allNearby(players)
	if findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, "a:1") == nil {
		t.Fatal("expected initial attack")
	}
	if events := simulation.Step(players, map[string][]*entity.Player{}, 2, random); findEvent(events, EventDamageApplied, "a:1") != nil || attacker.Characters[0].TargetCharacterID != "" {
		t.Fatalf("attack continued after target left candidate set: events=%+v character=%+v", events, attacker.Characters[0])
	}
}

func TestAttackRangePlusBufferCannotExceedDetectionRadius(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 10.01, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	players := playerMap(attacker, target)
	events := NewSimulation().Step(players, allNearby(players), 1, rand.New(rand.NewSource(1)))
	if findEvent(events, EventAttackStarted, "a:1") != nil {
		t.Fatal("character with attack range plus buffer above detection radius started an attack")
	}
}

func TestProjectileContinuesAfterTargetLeavesCandidateSet(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeRanged, entity.Vector2{}, 10, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 2}, 1, 0.5)
	attacker.Characters[0].AttackSpeed = 5
	attacker.Characters[0].Weapon = entity.Weapon{Type: entity.WeaponBow, Name: "bow", ProjectileSpeed: 20}
	players := playerMap(attacker, target)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	simulation.Step(players, nearby, 1, random)
	if findEvent(simulation.Step(players, nearby, 2, random), EventProjectileSpawned, "a:1") == nil {
		t.Fatal("expected projectile to spawn")
	}
	events := simulation.Step(players, map[string][]*entity.Player{}, 3, random)
	if findEvent(events, EventProjectileHit, "a:1") == nil || findEvent(events, EventDamageApplied, "a:1") == nil {
		t.Fatalf("projectile stopped with candidate acquisition: %+v", events)
	}
}

func combatPlayer(userID, characterID string, rangeClass entity.RangeClass, position entity.Vector2, attackRange, impactRatio float64) *entity.Player {
	return &entity.Player{
		UserID: userID, SessionID: userID, Position: position, DetectionRadius: entity.DefaultDetectionRadius,
		Characters: []*entity.Character{{
			ID: characterID, RangeClass: rangeClass, Position: position,
			Health: 100, AttackSpeed: 2, AttackRange: attackRange, ImpactRatio: impactRatio,
			Damage: 10, DamageRatio: 1, Weapon: entity.Weapon{Type: entity.WeaponSword, Name: "sword"},
		}},
	}
}

func spatialCandidates(t *testing.T, players map[string]*entity.Player) map[string][]*entity.Player {
	t.Helper()
	grid := spatial.NewGrid(20)
	for _, player := range players {
		if err := grid.Insert(player); err != nil {
			t.Fatal(err)
		}
	}
	result := make(map[string][]*entity.Player, len(players))
	for _, player := range players {
		result[player.SessionID] = grid.QueryPlayers(player)
	}
	return result
}

func allNearby(players map[string]*entity.Player) map[string][]*entity.Player {
	result := make(map[string][]*entity.Player, len(players))
	for _, attacker := range players {
		for _, target := range players {
			if target != attacker {
				result[attacker.SessionID] = append(result[attacker.SessionID], target)
			}
		}
	}
	return result
}

func playerMap(players ...*entity.Player) map[string]*entity.Player {
	result := make(map[string]*entity.Player, len(players))
	for index, player := range players {
		result[player.UserID+string(rune(index))] = player
	}
	return result
}

func findEvent(events []Event, eventType EventType, attackerID string) *Event {
	for index := range events {
		if events[index].Type == eventType && events[index].AttackerCharacterID == attackerID {
			return &events[index]
		}
	}
	return nil
}

func countEvents(events []Event, eventType EventType) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}
