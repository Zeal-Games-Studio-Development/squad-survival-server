package entity

import (
	"testing"
)

func TestNewCharacterUsesDefaultStats(t *testing.T) {
	character := NewCharacter()

	if character.Health != 100 || character.MaxHealth != 100 || character.Damage != 10 || character.MoveSpeed != 5 {
		t.Fatalf("unexpected base stats: %+v", character)
	}
	if character.AttackSpeed != 1.2 || character.AttackRange != 0 {
		t.Fatalf("unexpected attack stats: %+v", character)
	}
	if character.RegenRate != 0 || character.CritChance != 0 || character.CritMultiplier != 1.5 || character.DamageReduction != 0 {
		t.Fatalf("unexpected recovery or damage stats: %+v", character)
	}
}

func TestCharacterRetainsAssignedSkin(t *testing.T) {
	character := NewCharacter()
	character.Skin = Skin{HairID: 2, WeaponID: 3}
	if character.Skin.HairID != 2 || character.Skin.WeaponID != 3 {
		t.Fatalf("unexpected character skin: %+v", character.Skin)
	}
}

func TestCharactersByRangeClass(t *testing.T) {
	melee := &Character{RangeClass: RangeMelee}
	ranged := &Character{RangeClass: RangeRanged}
	player := &Player{Characters: []*Character{melee, nil, ranged}}

	got := player.CharactersByRangeClass(RangeMelee)
	if len(got) != 1 || got[0] != melee {
		t.Fatalf("unexpected melee characters: %+v", got)
	}
	if got := player.CharactersByRangeClass(RangeClass("reach")); len(got) != 0 {
		t.Fatalf("expected no removed reach characters, got %+v", got)
	}
}
