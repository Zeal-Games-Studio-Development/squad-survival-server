package entity

import "testing"

func TestCooldownModesScaleAndDedupeActions(t *testing.T) {
	character := NewCharacter()
	character.CooldownScale = 1.25
	for _, mode := range []CooldownMode{CooldownTicks, CooldownAttackActions, CooldownDamageReceived} {
		if err := character.StartCooldown(string(mode), mode, 2.5); err != nil {
			t.Fatal(err)
		}
	}
	character.AdvanceCooldowns(CooldownTicks, "")
	character.AdvanceCooldowns(CooldownAttackActions, "attack:a")
	character.AdvanceCooldowns(CooldownDamageReceived, "hit:a")
	character.AdvanceCooldowns(CooldownAttackActions, "attack:a")
	character.AdvanceCooldowns(CooldownDamageReceived, "hit:a")
	if character.Cooldowns[string(CooldownTicks)].Progress != 1.25 || character.Cooldowns[string(CooldownAttackActions)].Progress != 1.25 || character.Cooldowns[string(CooldownDamageReceived)].Progress != 1.25 {
		t.Fatalf("wrong cooldown progress: %+v", character.Cooldowns)
	}
	character.AdvanceCooldowns(CooldownDamageReceived, "hit:b")
	character.AdvanceCooldowns(CooldownAttackActions, "attack:b")
	character.AdvanceCooldowns(CooldownTicks, "")
	for _, mode := range []CooldownMode{CooldownTicks, CooldownAttackActions, CooldownDamageReceived} {
		if !character.CooldownReady(string(mode)) {
			t.Fatalf("%s cooldown did not finish", mode)
		}
	}
}

func TestCooldownResetStartsNextCycleWithoutCountingOldAction(t *testing.T) {
	character := NewCharacter()
	character.CooldownScale = 1.25
	if err := character.StartCooldown("skill", CooldownDamageReceived, 1.25); err != nil {
		t.Fatal(err)
	}
	character.AdvanceCooldowns(CooldownDamageReceived, "action:a")
	if !character.ResetCooldown("skill") || character.Cooldowns["skill"].Progress != 0 {
		t.Fatal("ready skill did not reset")
	}
	character.AdvanceCooldowns(CooldownDamageReceived, "action:a")
	if character.Cooldowns["skill"].Progress != 0 {
		t.Fatal("old action advanced a new cooldown cycle")
	}
	character.AdvanceCooldowns(CooldownDamageReceived, "action:b")
	if !character.CooldownReady("skill") {
		t.Fatal("new action did not recharge skill")
	}
}
