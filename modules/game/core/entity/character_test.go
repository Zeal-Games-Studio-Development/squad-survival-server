package entity

import (
	"math/rand"
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
	if character.RegenRate != 0 || character.DamageRatio != 1.1 {
		t.Fatalf("unexpected recovery or damage ratio stats: %+v", character)
	}
}

func TestCharacterRetainsAssignedSkin(t *testing.T) {
	character := NewCharacter()
	character.Skin = Skin{HairID: 2, WeaponID: 3}
	if character.Skin.HairID != 2 || character.Skin.WeaponID != 3 {
		t.Fatalf("unexpected character skin: %+v", character.Skin)
	}
}

func TestCharacterDamageRange(t *testing.T) {
	character := Character{Damage: 10, DamageRatio: 1.1}
	minimum, maximum := character.DamageRange()

	if !almostEqual(minimum, 9) || !almostEqual(maximum, 11) {
		t.Fatalf("expected damage range 9..11, got %f..%f", minimum, maximum)
	}
}

func TestCharacterDamageRangeDefaultsToFixedDamage(t *testing.T) {
	character := Character{Damage: 10}
	minimum, maximum := character.DamageRange()

	if minimum != 10 || maximum != 10 {
		t.Fatalf("expected fixed damage 10, got %f..%f", minimum, maximum)
	}
}

func TestCharacterRollDamageStaysInsideRange(t *testing.T) {
	character := Character{Damage: 10, DamageRatio: 1.1}
	random := rand.New(rand.NewSource(1))
	seenDifferentDamage := false
	previous := character.RollDamage(random)

	for range 1000 {
		damage := character.RollDamage(random)
		if damage < 9 || damage > 11 {
			t.Fatalf("damage outside 9..11: %f", damage)
		}
		if damage != previous {
			seenDifferentDamage = true
		}
		previous = damage
	}
	if !seenDifferentDamage {
		t.Fatal("expected damage rolls to vary")
	}
}

func TestCharacterRollDamageWithoutRatioIsFixed(t *testing.T) {
	character := Character{Damage: 10}
	random := rand.New(rand.NewSource(1))

	if damage := character.RollDamage(random); damage != 10 {
		t.Fatalf("expected fixed damage 10, got %f", damage)
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
