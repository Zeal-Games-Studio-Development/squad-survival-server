package entity

import (
	"math/rand"
	"testing"
)

func TestDefaultWeaponCatalogContainsAllWeaponTypes(t *testing.T) {
	weapons := DefaultWeaponCatalog()
	if len(weapons) != 7 {
		t.Fatalf("expected 7 weapons, got %d", len(weapons))
	}

	found := make(map[WeaponType]bool, len(weapons))
	for _, weapon := range weapons {
		found[weapon.Type] = true
		if weapon.Name == "" {
			t.Fatalf("weapon %q has no name", weapon.Type)
		}
		if !weapon.RangeClass.Valid() {
			t.Fatalf("weapon %q has invalid range class %q", weapon.Type, weapon.RangeClass)
		}
		if weapon.AttackSpeed <= 0 || weapon.ImpactRatio <= 0 || weapon.ImpactRatio > 1 {
			t.Fatalf("weapon %q has invalid attack timing: speed=%f impact_ratio=%f", weapon.Type, weapon.AttackSpeed, weapon.ImpactRatio)
		}
	}
	for _, weaponType := range []WeaponType{
		WeaponBow, WeaponStaff, WeaponSpear,
		WeaponSword, WeaponWand, WeaponAxe, WeaponBlunt,
	} {
		if !found[weaponType] {
			t.Fatalf("missing weapon type %q", weaponType)
		}
	}
}

func TestWeaponReplacesAllCharacterStats(t *testing.T) {
	weapon := Weapon{
		Type: WeaponAxe, Name: "axe", RangeClass: RangeMelee, Health: 120, Damage: 15,
		MoveSpeed: 4, AttackSpeed: 0.8, AttackRange: 2, ImpactRatio: 0.5,
		RegenRate: 0.5, DamageRatio: 1.2,
	}
	character := NewCharacter()
	character.ApplyWeapon(weapon)

	if character.Weapon != weapon {
		t.Fatalf("unexpected weapon: %+v", character.Weapon)
	}
	if character.RangeClass != RangeMelee {
		t.Fatalf("expected melee character, got %q", character.RangeClass)
	}
	if character.Health != 120 || character.MaxHealth != 120 || character.Damage != 15 || character.MoveSpeed != 4 ||
		character.AttackSpeed != 0.8 || character.AttackRange != 2 || character.ImpactRatio != 0.5 ||
		character.RegenRate != 0.5 || character.DamageRatio != 1.2 {
		t.Fatalf("weapon stats were not fully applied: %+v", character)
	}
}

func TestParseWeaponCatalogRejectsInvalidRangeClass(t *testing.T) {
	_, err := ParseWeaponCatalog([]byte(`{"weapons":[{"type":"sword","name":"sword","range_class":"short","attack_speed":1,"impact_ratio":0.5}]}`))
	if err == nil {
		t.Fatal("expected invalid range class to be rejected")
	}
}

func TestParseWeaponCatalogRejectsMediumRangeClass(t *testing.T) {
	_, err := ParseWeaponCatalog([]byte(`{"weapons":[{"type":"spear","name":"spear","range_class":"medium","attack_speed":1,"impact_ratio":0.5}]}`))
	if err == nil {
		t.Fatal("expected medium range class to be rejected")
	}
}

func TestParseWeaponCatalogRejectsRemovedReachRangeClass(t *testing.T) {
	_, err := ParseWeaponCatalog([]byte(`{"weapons":[{"type":"spear","name":"spear","range_class":"reach","attack_speed":1,"impact_ratio":0.5}]}`))
	if err == nil {
		t.Fatal("expected removed reach range class to be rejected")
	}
}

func TestParseWeaponCatalogRejectsInvalidAttackTiming(t *testing.T) {
	tests := []string{
		`{"weapons":[{"type":"sword","name":"sword","range_class":"melee","attack_speed":0,"impact_ratio":0.5}]}`,
		`{"weapons":[{"type":"sword","name":"sword","range_class":"melee","attack_speed":1,"impact_ratio":0}]}`,
		`{"weapons":[{"type":"sword","name":"sword","range_class":"melee","attack_speed":1,"impact_ratio":1.1}]}`,
	}
	for _, data := range tests {
		if _, err := ParseWeaponCatalog([]byte(data)); err == nil {
			t.Fatalf("expected invalid attack timing to be rejected: %s", data)
		}
	}
}

func TestParseWeaponCatalogRejectsRangedWeaponWithoutProjectileSpeed(t *testing.T) {
	data := `{"weapons":[{"type":"bow","name":"bow","range_class":"ranged","attack_speed":1,"impact_ratio":0.5,"projectile_speed":0}]}`
	if _, err := ParseWeaponCatalog([]byte(data)); err == nil {
		t.Fatal("expected ranged weapon without projectile speed to be rejected")
	}
}

func TestCreateCharacterSelectsWeaponFromCatalog(t *testing.T) {
	weapons := DefaultWeaponCatalog()
	character := CreateCharacter(rand.New(rand.NewSource(1)), weapons)

	found := false
	for _, weapon := range weapons {
		if character.Weapon == weapon {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("character received unknown weapon: %+v", character.Weapon)
	}
}
