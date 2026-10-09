package combat

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func TestSwordSkillUsesLockedConeAndCurrentTargetPositions(t *testing.T) {
	caster := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 3, 0.5)
	primary := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	inside := combatPlayer("c", "c:1", entity.RangeMelee, entity.Vector2{X: 2, Y: 0.5}, 2, 0.5)
	outside := combatPlayer("d", "d:1", entity.RangeMelee, entity.Vector2{X: 1, Y: 2}, 2, 0.5)
	for _, player := range []*entity.Player{primary, inside, outside} {
		player.Characters[0].AttackSpeed = 0
	}
	character := caster.Characters[0]
	character.AttackCount = 3
	character.CritChance = 0
	if err := character.StartCooldown("sword_cone", entity.CooldownTicks, 30); err != nil {
		t.Fatal(err)
	}
	character.Cooldowns["sword_cone"].Progress = 30
	players := playerMap(caster, primary, inside, outside)
	nearby := spatialCandidates(t, players)
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	start := findEvent(simulation.Step(players, nearby, 1, random), EventSkillStarted, character.ID)
	if start == nil || start.SkillID != "sword_cone" || start.ImpactTick != 3 || start.CompleteTick != 6 || start.Direction.X != 1 || start.Direction.Y != 0 || !entity.HasActionTag(start.Tags, entity.TagSkill) {
		t.Fatalf("wrong skill start: %+v", start)
	}
	if character.Cooldowns["sword_cone"].Progress != 0 {
		t.Fatal("skill did not reset cooldown when started")
	}
	primary.Characters[0].Position = entity.Vector2{X: -1}
	caster.Direction = entity.Vector2{X: 1}
	if event := findEvent(simulation.Step(players, nearby, 2, random), EventDamageApplied, character.ID); event != nil {
		t.Fatalf("skill hit before impact: %+v", event)
	}
	events := simulation.Step(players, nearby, 3, random)
	hits := 0
	for _, event := range events {
		if event.Type != EventDamageApplied || event.AttackerCharacterID != character.ID {
			continue
		}
		hits++
		if event.TargetCharacterID != inside.Characters[0].ID || event.AttackID != start.AttackID || event.Damage != 10 || !entity.HasActionTag(event.Tags, entity.TagAOE) || !entity.HasActionTag(event.Tags, entity.TagSkill) {
			t.Fatalf("wrong skill hit: %+v", event)
		}
	}
	if hits != 1 || character.Cooldowns["sword_cone"].Progress != 2 {
		t.Fatalf("unexpected hit count or cooldown: %d %+v", hits, character.Cooldowns["sword_cone"])
	}
	for tick := int64(4); tick <= 6; tick++ {
		simulation.Step(players, nearby, tick, random)
	}
	if character.ActiveSkillID != "" {
		t.Fatal("skill did not complete")
	}
}

func TestSwordWeaponVariantsStartAndHitWithSwordCone(t *testing.T) {
	for _, weapon := range entity.DefaultWeaponCatalog() {
		if weapon.Type != entity.WeaponSword || weapon.ID == "swordman" {
			continue
		}
		t.Run(weapon.ID, func(t *testing.T) {
			caster := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
			character := caster.Characters[0]
			character.ApplyWeapon(weapon)
			cooldown := character.Cooldowns["sword_cone"]
			if cooldown == nil {
				t.Fatal("sword variant has no sword_cone cooldown")
			}
			cooldown.Progress = cooldown.Required
			target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
			target.Characters[0].AttackSpeed = 0
			players := playerMap(caster, target)
			nearby := allNearby(players)
			simulation := NewSimulation()
			random := rand.New(rand.NewSource(1))
			if event := findEvent(simulation.Step(players, nearby, 1, random), EventSkillStarted, character.ID); event == nil || event.SkillID != "sword_cone" {
				t.Fatalf("sword variant did not start sword_cone: %+v", event)
			}
			simulation.Step(players, nearby, 2, random)
			if event := findEvent(simulation.Step(players, nearby, 3, random), EventDamageApplied, character.ID); event == nil || !entity.HasActionTag(event.Tags, entity.TagSkill) {
				t.Fatalf("sword variant did not deal skill damage: %+v", event)
			}
		})
	}
}

func TestReadySkillWaitsForTargetAndCurrentAttack(t *testing.T) {
	caster := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 2, 0.5)
	character := caster.Characters[0]
	if err := character.StartCooldown("sword_cone", entity.CooldownTicks, 30); err != nil {
		t.Fatal(err)
	}
	character.Cooldowns["sword_cone"].Progress = 30
	simulation := NewSimulation()
	random := rand.New(rand.NewSource(1))
	if event := findEvent(simulation.Step(playerMap(caster), nil, 1, random), EventSkillStarted, character.ID); event != nil || !character.CooldownReady("sword_cone") {
		t.Fatal("skill consumed without a target")
	}
	target := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 1}, 2, 0.5)
	target.Characters[0].AttackSpeed = 0
	players := playerMap(caster, target)
	nearby := allNearby(players)
	character.AttackSequence = 1
	character.TargetUserID, character.TargetCharacterID = target.UserID, target.Characters[0].ID
	character.AttackImpactTick, character.AttackCompleteTick = 4, 5
	if event := findEvent(simulation.Step(players, nearby, 2, random), EventSkillStarted, character.ID); event != nil {
		t.Fatal("skill interrupted basic attack")
	}
	if !character.CooldownReady("sword_cone") {
		t.Fatal("basic attack consumed skill cooldown")
	}
}

func TestSkillAOEHitsHaveIndependentCritsAndIDs(t *testing.T) {
	caster := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 3, 0.5)
	character := caster.Characters[0]
	character.SkillActionID = "a:1:skill:1"
	character.SkillDirection = entity.Vector2{X: 1}
	character.CritChance = 0.5
	candidates := make([]ownedCharacter, 0, 4)
	for _, id := range []string{"b", "c", "d", "e"} {
		player := combatPlayer(id, id+":1", entity.RangeMelee, entity.Vector2{X: 1}, 1, 0.5)
		candidates = append(candidates, ownedCharacter{owner: player, character: player.Characters[0]})
	}
	skill := entity.DefaultSkillCatalog()[0]
	intents := skillDamageIntents(skill, ownedCharacter{owner: caster, character: character}, candidates, 5, rand.New(rand.NewSource(1))).damageIntents
	if len(intents) != 4 {
		t.Fatalf("expected four skill hits, got %d", len(intents))
	}
	critical, normal := 0, 0
	ids := make(map[string]bool)
	for _, intent := range intents {
		if ids[intent.event.HitID] || intent.event.AttackID != character.SkillActionID {
			t.Fatalf("duplicate hit or wrong action: %+v", intent.event)
		}
		ids[intent.event.HitID] = true
		if intent.event.Critical {
			critical++
		} else {
			normal++
		}
	}
	if critical == 0 || normal == 0 {
		t.Fatalf("crit rolls were not independent: critical=%d normal=%d", critical, normal)
	}
	skill.Effect.CanCrit = false
	for _, intent := range skillDamageIntents(skill, ownedCharacter{owner: caster, character: character}, candidates, 5, rand.New(rand.NewSource(1))).damageIntents {
		if intent.event.Critical || intent.event.Damage != character.Damage {
			t.Fatalf("noncritical skill still rolled crit: %+v", intent.event)
		}
	}
}

func TestSkillTargetRelationsOnlyUseCharactersFromCasterPlayer(t *testing.T) {
	caster := combatPlayer("a", "a:1", entity.RangeMelee, entity.Vector2{}, 3, 0.5)
	ally := &entity.Character{ID: "a:2", Position: entity.Vector2{X: 1}, Health: 100}
	caster.Characters = append(caster.Characters, ally)
	enemy := combatPlayer("b", "b:1", entity.RangeMelee, entity.Vector2{X: 0.5}, 3, 0.5)
	owned := ownedCharacter{owner: caster, character: caster.Characters[0]}
	enemies := []ownedCharacter{{owner: enemy, character: enemy.Characters[0]}}
	skill := entity.DefaultSkillCatalog()[0]
	skill.Target.Relation = "ally"
	if target, ok := nearestSkillTarget(skill, owned, enemies); !ok || target.character != ally {
		t.Fatalf("ally selector chose wrong character: %+v", target)
	}
	skill.Target.Relation = "self"
	if target, ok := nearestSkillTarget(skill, owned, enemies); !ok || target.character != owned.character {
		t.Fatalf("self selector chose wrong character: %+v", target)
	}
	skill.Target.Relation = "enemy"
	if target, ok := nearestSkillTarget(skill, owned, enemies); !ok || target.character != enemy.Characters[0] {
		t.Fatalf("enemy selector chose wrong character: %+v", target)
	}
}
