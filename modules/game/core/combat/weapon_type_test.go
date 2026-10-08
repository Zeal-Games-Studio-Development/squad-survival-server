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
	players := playerMap(attacker, target)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, character.ID)
	if start == nil || start.AttackCount != 3 || !entity.HasActionTag(start.Tags, entity.TagBasicAttack) {
		t.Fatalf("invalid action start: %+v", start)
	}
	if character.Cooldowns["attack"].Progress != 1 {
		t.Fatal("attack cooldown advanced more than once for one action")
	}
	hits := make(map[string]bool)
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

func TestSpearLineIsOneAOEHitPerTarget(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 3, 0.5)
	attacker.Characters[0].Weapon.Type = entity.WeaponSpear
	attacker.Characters[0].AttackSpeed = 10
	attacker.Characters[0].AttackCount = 2
	primary := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	second := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 2, Y: 0.5}, 2, 0.5)
	outside := combatPlayer("d", "d:1", entity.RangeMelee, entity.Vector2{X: 2, Y: 0.7}, 2, 0.5)
	players := playerMap(attacker, primary, second, outside)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, attacker.Characters[0].ID)
	if start == nil || !entity.HasActionTag(start.Tags, entity.TagAOE) {
		t.Fatalf("spear action is not AOE: %+v", start)
	}
	events := simulation.Step(players, nearby, 2, random)
	hits := make(map[string]int)
	for _, event := range events {
		if event.Type == EventDamageApplied && event.AttackerCharacterID == attacker.Characters[0].ID {
			hits[event.TargetCharacterID]++
			if event.Damage != 30 || !entity.HasActionTag(event.Tags, entity.TagAOE) {
				t.Fatalf("invalid spear damage: %+v", event)
			}
		}
	}
	if hits["b:1"] != 1 || hits["c:1"] != 1 || hits["d:1"] != 0 {
		t.Fatalf("wrong spear line targets: %+v", hits)
	}
	for tick := int64(3); tick <= 4; tick++ {
		for _, event := range simulation.Step(players, nearby, tick, random) {
			if event.Type == EventDamageApplied && event.AttackID == start.AttackID {
				t.Fatalf("AOE action emitted a delayed hit at tick %d: %+v", tick, event)
			}
		}
	}
}

func TestBowUsesTargetPositionAtProjectileImpact(t *testing.T) {
	attacker := combatPlayer("a", "a:1", entity.RangeRanged, entity.Vector2{}, 8, 0.5)
	attacker.Characters[0].Weapon = entity.Weapon{Type: entity.WeaponBow, ID: "bow", ProjectileSpeed: 100}
	attacker.Characters[0].AttackSpeed = 10
	attacker.Characters[0].AttackCount = 2
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 2}, 2, 0.5)
	players := playerMap(attacker, target)
	nearby := allNearby(players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventAttackStarted, attacker.Characters[0].ID)
	if start == nil || start.CompleteTick != 4 {
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
	if secondSpawn == nil || secondSpawn.AttackID != spawn.AttackID || secondSpawn.ProjectileID == spawn.ProjectileID {
		t.Fatalf("second projectile must spawn two ticks later in the same action: %+v", secondSpawn)
	}
	target.Characters[0].Position.X = 8
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
	players := playerMap(attacker, target)
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
