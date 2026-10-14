package combat

import (
	"math"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func TestBasicAttackCountUsesOneActionAndIndependentCrits(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	second := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 1.2}, 2, 0.5)
	third := combatPlayer("d", "d:1", entity.RangeMelee, entity.Vector2{X: 1.4}, 2, 0.5)
	for _, player := range []*entity.Player{target, second, third} {
		player.Characters[0].AttackSpeed = 0
	}
	character := attacker.Characters[0]
	target.Characters[0].Weapon.Type = entity.WeaponStaff
	target.Characters[0].CooldownScale = 1.25
	if err := target.Characters[0].StartCooldown("damage", entity.CooldownDamageReceived, 3); err != nil {
		t.Fatal(err)
	}
	character.AttackSpeed, character.AttackCount, character.CritChance = 5, 3, 0.75
	character.CooldownScale = 1
	if err := character.StartCooldown("attack", entity.CooldownAttackActions, 2); err != nil {
		t.Fatal(err)
	}
	players := playerMap(attacker, target, second, third)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, character.ID)
	if start == nil || start.AttackCount != 3 || len(start.AttackTargets) != 3 || start.AttackTargets[0].CharacterID != "b:1" || start.AttackTargets[1].CharacterID != "c:1" || start.AttackTargets[2].CharacterID != "d:1" || !entity.HasActionTag(start.Tags, entity.TagBasicAttack) {
		t.Fatalf("invalid action start: %+v", start)
	}
	if character.Cooldowns["attack"].Progress != 1 {
		t.Fatal("attack cooldown advanced more than once for one action")
	}
	hits := make(map[string]bool)
	hitTargets := make(map[string]bool)
	crit, normal := 0, 0
	for tick := int64(2); tick <= 6; tick++ {
		events := simulation.Step(players, nearby, tick, random)
		for _, event := range events {
			if event.Type != EventDamageApplied || event.AttackerCharacterID != character.ID {
				continue
			}
			if event.AttackID != start.AttackID || event.HitID == "" || hits[event.HitID] || (tick-2)%BasicAttackHitDelayTicks != 0 {
				t.Fatalf("hit has wrong action, ID, or timing: %+v", event)
			}
			hits[event.HitID] = true
			if hitTargets[event.TargetCharacterID] {
				t.Fatalf("target received a second direct hit: %+v", event)
			}
			hitTargets[event.TargetCharacterID] = true
			if event.Critical {
				crit++
			} else {
				normal++
			}
		}
		if tick == 5 && character.Cooldowns["attack"].Progress != 1 {
			t.Fatal("attack cooldown advanced before the next action")
		}
	}
	if len(hits) != 3 || crit == 0 || normal == 0 || character.Cooldowns["attack"].Progress != 2 || target.Characters[0].Cooldowns["damage"].Progress != 1.25 {
		t.Fatalf("unexpected multi hit result: hits=%d crit=%d normal=%d cooldown=%+v", len(hits), crit, normal, character.Cooldowns["attack"])
	}
}

func TestSpearLocksTargetsAndOverlappingAOEHitsAgain(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 3, 0.5)
	attacker.Characters[0].Weapon.Type = entity.WeaponSpear
	attacker.Characters[0].AttackSpeed = 10
	attacker.Characters[0].AttackCount = 2
	primary := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	if err := primary.Characters[0].StartCooldown("received", entity.CooldownDamageReceived, 3); err != nil {
		t.Fatal(err)
	}
	second := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 2, Y: 0.5}, 2, 0.5)
	outside := combatPlayer("d", "d:1", entity.RangeMelee, entity.Vector2{X: 2, Y: 0.7}, 2, 0.5)
	players := playerMap(attacker, primary, second, outside)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, attacker.Characters[0].ID)
	if start == nil || start.AttackCount != 2 || len(start.AttackTargets) != 2 || start.AttackTargets[0].CharacterID != "b:1" || start.AttackTargets[1].CharacterID != "c:1" || !entity.HasActionTag(start.Tags, entity.TagAOE) {
		t.Fatalf("spear action is not AOE: %+v", start)
	}
	hits := make(map[string]int)
	ids := make(map[string]bool)
	for tick := int64(2); tick <= 4; tick++ {
		for _, event := range simulation.Step(players, nearby, tick, random) {
			if event.Type == EventDamageApplied && event.AttackID == start.AttackID {
				hits[event.TargetCharacterID]++
				if event.Damage != 10 || !entity.HasActionTag(event.Tags, entity.TagAOE) || ids[event.HitID] || tick == 3 {
					t.Fatalf("invalid spear hit: %+v", event)
				}
				ids[event.HitID] = true
			}
		}
	}
	if hits["b:1"] != 2 || hits["c:1"] != 2 || hits["d:1"] != 1 {
		t.Fatalf("wrong spear line targets: %+v", hits)
	}
	if primary.Characters[0].Cooldowns["received"].Progress != 1 {
		t.Fatal("overlapping AoE advanced damage_received twice for one action")
	}
}

func TestSpearKeepsLockedDirectionWhenTargetMoves(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 3, 0.5)
	attacker.Characters[0].Weapon.Type = entity.WeaponSpear
	attacker.Characters[0].AttackSpeed = 10
	attacker.Characters[0].AttackCount = 2
	first := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	second := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 2, Y: 0.5}, 2, 0.5)
	first.Characters[0].AttackSpeed = 0
	second.Characters[0].AttackSpeed = 0
	players := playerMap(attacker, first, second)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, "a:1")
	if start == nil || start.AttackTargets[1].Direction.Y <= 0 {
		t.Fatalf("second target direction was not locked: %+v", start)
	}
	simulation.Step(players, nearby, 2, random)
	second.Characters[0].Position.Y = -0.5
	simulation.Step(players, nearby, 3, random)
	for _, event := range simulation.Step(players, nearby, 4, random) {
		if event.Type == EventDamageApplied && event.AttackID == start.AttackID && event.TargetCharacterID == "c:1" {
			t.Fatalf("spear tracked moved target instead of locked direction: %+v", event)
		}
	}
}

func TestBowUsesTargetPositionAtProjectileImpact(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeRanged, entity.Vector2{}, 8, 0.5)
	attacker.Characters[0].Weapon = entity.Weapon{Type: entity.WeaponBow, ID: "bow", ProjectileSpeed: 100}
	attacker.Characters[0].AttackSpeed = 10
	attacker.Characters[0].AttackCount = 2
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 2}, 2, 0.5)
	second := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 6}, 2, 0.5)
	second.Characters[0].AttackSpeed = 0
	players := playerMap(attacker, target, second)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, attacker.Characters[0].ID)
	if start == nil || start.CompleteTick != 4 || start.AttackCount != 2 || start.AttackTargets[1].CharacterID != "c:1" {
		t.Fatalf("action must last through the second projectile: %+v", start)
	}
	spawn := findEvent(simulation.Step(players, nearby, 2, random), EventProjectileSpawned, attacker.Characters[0].ID)
	if spawn == nil || len(simulation.Projectiles()) != 1 {
		t.Fatalf("expected the first projectile at tick 2: %+v", spawn)
	}
	target.Characters[0].Position.X = 4
	events := simulation.Step(players, nearby, 3, random)
	firstHits := 0
	for _, event := range events {
		if event.Type == EventDamageApplied && event.AttackerCharacterID == attacker.Characters[0].ID {
			firstHits++
			if math.Abs(event.Damage-15) > 1e-9 || !entity.HasActionTag(event.Tags, entity.TagProjectile) {
				t.Fatalf("bow used the wrong impact distance: %+v", event)
			}
		}
	}
	if firstHits != 1 || countEvents(events, EventProjectileSpawned) != 0 {
		t.Fatalf("expected only the first bow hit at tick 3: %+v", events)
	}
	secondSpawn := findEvent(simulation.Step(players, nearby, 4, random), EventProjectileSpawned, attacker.Characters[0].ID)
	if secondSpawn == nil || secondSpawn.AttackID != spawn.AttackID || secondSpawn.ProjectileID == spawn.ProjectileID || secondSpawn.TargetCharacterID != "c:1" {
		t.Fatalf("second projectile must spawn two ticks later in the same action: %+v", secondSpawn)
	}
	second.Characters[0].Position.X = 8
	maxRangeHits := 0
	for _, event := range simulation.Step(players, nearby, 5, random) {
		if event.Type == EventDamageApplied && event.AttackID == start.AttackID {
			maxRangeHits++
			if math.Abs(event.Damage-20) > 1e-9 {
				t.Fatalf("bow did not deal double damage at max range: %+v", event)
			}
		}
	}
	if maxRangeHits != 1 {
		t.Fatalf("expected one max range bow hit, got %d", maxRangeHits)
	}
}

func TestCrossbowCritReadsHealthAtImpact(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeRanged, entity.Vector2{}, 8, 0.5)
	attacker.Characters[0].Weapon = entity.Weapon{Type: entity.WeaponCrossbow, ID: "crossbow", ProjectileSpeed: 100}
	attacker.Characters[0].AttackSpeed = 10
	attacker.Characters[0].CritChance = 0.65
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 2}, 2, 0.5)
	target.Characters[0].MaxHealth = 100
	players := playerMap(attacker, target)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	simulation.Step(players, nearby, 1, random)
	simulation.Step(players, nearby, 2, random)
	target.Characters[0].Health = 50
	event := findEvent(simulation.Step(players, nearby, 3, random), EventDamageApplied, attacker.Characters[0].ID)
	if event == nil || !event.Critical || event.Damage != 15 {
		t.Fatalf("crossbow did not use low target health at impact: %+v", event)
	}
}

func TestCancelledAttackKeepsSpawnedProjectileButSkipsRemainingCount(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeRanged, entity.Vector2{}, 8, 0.5)
	attacker.Characters[0].Weapon = entity.Weapon{Type: entity.WeaponBow, ID: "bow", ProjectileSpeed: 100}
	attacker.Characters[0].AttackSpeed = 10
	attacker.Characters[0].AttackCount = 2
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 2}, 2, 0.5)
	second := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 3}, 2, 0.5)
	players := playerMap(attacker, target, second)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	simulation.Step(players, nearby, 1, random)
	if countEvents(simulation.Step(players, nearby, 2, random), EventProjectileSpawned) != 1 {
		t.Fatal("first projectile was not spawned")
	}
	attacker.Direction.X = AttackMovementThreshold
	if countEvents(simulation.Step(players, nearby, 3, random), EventProjectileHit) != 1 {
		t.Fatal("already spawned projectile was cancelled")
	}
	if countEvents(simulation.Step(players, nearby, 4, random), EventProjectileSpawned) != 0 {
		t.Fatal("cancelled action spawned its second projectile")
	}
}
