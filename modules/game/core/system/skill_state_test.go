package system

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestSkillCombatAndStateProtocol(t *testing.T) {
	player := entity.NewPlayer("a", "session-a", "", entity.Vector2{}, rand.New(rand.NewSource(1)))
	for _, weapon := range entity.DefaultWeaponCatalog() {
		if weapon.ID == "swordman" {
			player.Characters[0].ApplyWeapon(weapon)
		}
	}
	character := player.Characters[0]
	rosterBytes, err := EncodePlayerRosterBatch(1, []*entity.Player{player})
	if err != nil {
		t.Fatal(err)
	}
	var roster PlayerRosterBatch
	if err := proto.Unmarshal(rosterBytes, &roster); err != nil {
		t.Fatal(err)
	}
	cooldowns := roster.Players[0].Characters[0].SkillCooldowns
	if len(cooldowns) != 1 || cooldowns[0].SkillId != "sword_cone" || cooldowns[0].Required != 30 || cooldowns[0].Progress != 0 {
		t.Fatalf("roster missing initial cooldown: %+v", cooldowns)
	}
	seen := make(map[string]uint64)
	states := ChangedSkillStates([]*entity.Player{player}, seen)
	if len(states) != 1 {
		t.Fatalf("missing first skill state: %+v", states)
	}
	data, err := EncodeSkillStateBatch(2, states)
	if err != nil {
		t.Fatal(err)
	}
	var batch SkillStateBatch
	if err := proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Tick != 2 || batch.States[0].CharacterId != character.ID || batch.States[0].Cooldowns[0].Mode != "ticks" {
		t.Fatalf("wrong cooldown state: %+v", &batch)
	}
	seen[SkillStateKey(states[0])] = states[0].Version
	if changed := ChangedSkillStates([]*entity.Player{player}, seen); len(changed) != 0 {
		t.Fatalf("unchanged skill state resent: %+v", changed)
	}
	character.AdvanceCooldowns(entity.CooldownTicks, "")
	if changed := ChangedSkillStates([]*entity.Player{player}, seen); len(changed) != 1 || changed[0].Cooldowns[0].Progress != 1 {
		t.Fatalf("cooldown progress not sent: %+v", changed)
	}

	started, err := EncodeCombatEventBatch(3, []combat.Event{{
		Type: combat.EventSkillStarted, AttackID: "a:1:1", SkillID: "sword_cone",
		AttackerUserID: player.UserID, AttackerCharacterID: character.ID,
		WeaponID: "swordman", Direction: entity.Vector2{X: 1},
		StartTick: 3, ImpactTick: 5, CompleteTick: 8,
		Tags: []entity.ActionTag{entity.TagSkill, entity.TagAOE},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var events CombatEventBatch
	if err := proto.Unmarshal(started, &events); err != nil {
		t.Fatal(err)
	}
	skill := events.Events[0].GetSkillStarted()
	if skill == nil || skill.SkillId != "sword_cone" || skill.ActionId != "a:1:1" || skill.Direction.X != 1 || skill.ImpactTick != 5 || len(skill.Tags) != 2 {
		t.Fatalf("wrong skill start event: %+v", skill)
	}
}
