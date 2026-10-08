package entity

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestDefaultWeaponCatalogContainsAllWeaponTypes(t *testing.T) {
	weapons := DefaultWeaponCatalog()
	if len(weapons) != 8 {
		t.Fatalf("expected 8 weapons, got %d", len(weapons))
	}

	found := make(map[WeaponType]bool, len(weapons))
	for _, weapon := range weapons {
		found[weapon.Type] = true
		if weapon.ID == "" || weapon.ID == "wand" {
			t.Fatalf("weapon %q has missing or removed id %q", weapon.Type, weapon.ID)
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
		WeaponSword, WeaponAxe, WeaponBlunt, WeaponCrossbow, WeaponShield,
	} {
		if !found[weaponType] {
			t.Fatalf("missing weapon type %q", weaponType)
		}
	}
}

func TestParseWeaponCatalogRejectsRemovedWandType(t *testing.T) {
	data := []byte(`{"weapons":[{"type":"wand","weapon_id":"wand","range_class":"ranged","attack_speed":1,"impact_ratio":0.5,"projectile_speed":10}]}`)
	if _, err := ParseWeaponCatalog(data); err == nil || !strings.Contains(err.Error(), "unsupported weapon type") {
		t.Fatalf("removed wand type was accepted: %v", err)
	}
}

func TestParseWeaponCatalogAllowsVariantOfSupportedType(t *testing.T) {
	data := []byte(`{"weapons":[{"type":"bow","weapon_id":"bow","range_class":"ranged","attack_speed":1,"impact_ratio":0.5,"projectile_speed":10},{"type":"bow","weapon_id":"bow_rare","range_class":"ranged","attack_speed":1,"impact_ratio":0.5,"projectile_speed":10}]}`)
	weapons, err := ParseWeaponCatalog(data)
	if err != nil || len(weapons) != 2 {
		t.Fatalf("supported type variant was rejected: weapons=%+v err=%v", weapons, err)
	}
}

func TestParseWeaponCatalogRejectsDuplicateWeaponID(t *testing.T) {
	data := []byte(`{"weapons":[{"type":"bow","weapon_id":"same","range_class":"ranged","attack_speed":1,"impact_ratio":0.5,"projectile_speed":10},{"type":"crossbow","weapon_id":"same","range_class":"ranged","attack_speed":1,"impact_ratio":0.5,"projectile_speed":10}]}`)
	if _, err := ParseWeaponCatalog(data); err == nil || !strings.Contains(err.Error(), "duplicate weapon id") {
		t.Fatalf("expected duplicate weapon id error, got %v", err)
	}
}

func TestWeaponReplacesAllCharacterStats(t *testing.T) {
	weapon := Weapon{
		Type: WeaponAxe, ID: "axe", RangeClass: RangeMelee, Health: 120, Damage: 15,
		MoveSpeed: 4, AttackSpeed: 0.8, AttackRange: 2, ImpactRatio: 0.5,
		RegenRate: 0.5, CritChance: 0.14, CritMultiplier: 1.5, DamageReduction: 0.2,
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
		character.RegenRate != 0.5 || character.CritChance != 0.14 || character.CritMultiplier != 1.5 || character.DamageReduction != 0.2 {
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

func TestParseWeaponCatalogRejectsInvalidAttackRange(t *testing.T) {
	data := `{"weapons":[{"type":"sword","name":"sword","range_class":"melee","attack_speed":1,"attack_range":-1,"impact_ratio":0.5,"projectile_speed":0}]}`
	if _, err := ParseWeaponCatalog([]byte(data)); err == nil {
		t.Fatal("expected negative attack range to fail")
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

func TestDefaultWeaponDamageStats(t *testing.T) {
	want := map[WeaponType]float64{
		WeaponBow: 0.12, WeaponStaff: 0.12, WeaponSpear: 0.12,
		WeaponSword: 0.10, WeaponAxe: 0.14, WeaponBlunt: 0.18, WeaponCrossbow: 0.12, WeaponShield: 0.08,
	}
	for _, weapon := range DefaultWeaponCatalog() {
		reduction := 0.0
		if weapon.Type == WeaponShield {
			reduction = 0.1
		}
		if weapon.CritChance != want[weapon.Type] || weapon.CritMultiplier != 1.5 || weapon.DamageReduction != reduction {
			t.Fatalf("unexpected damage stats for %q: %+v", weapon.Type, weapon)
		}
	}
}

func TestWeaponAttackAndCooldownStats(t *testing.T) {
	for _, weapon := range DefaultWeaponCatalog() {
		if weapon.AttackCount != 1 {
			t.Fatalf("%s attack count=%d", weapon.Type, weapon.AttackCount)
		}
		wantScale := 1.0
		if weapon.Type == WeaponStaff {
			wantScale = 1.25
		}
		if weapon.CooldownScale != wantScale {
			t.Fatalf("%s cooldown scale=%f", weapon.Type, weapon.CooldownScale)
		}
	}
	for _, field := range []string{`"attack_count":0`, `"attack_count":-1`, `"cooldown_scale":0`, `"cooldown_scale":-1`} {
		data := fmt.Sprintf(`{"weapons":[{"type":"sword","weapon_id":"sword","range_class":"melee","attack_speed":1,"impact_ratio":0.5,%s}]}`, field)
		if _, err := ParseWeaponCatalog([]byte(data)); err == nil {
			t.Fatalf("invalid stat accepted: %s", field)
		}
	}
}

func TestWeaponWithoutMultiplierUsesDefault(t *testing.T) {
	character := NewCharacter()
	character.ApplyWeapon(Weapon{Damage: 10})
	if character.CritMultiplier != 1.5 {
		t.Fatalf("missing multiplier should default to 1.5: %+v", character)
	}
}

func TestParseWeaponCatalogValidatesDamageStats(t *testing.T) {
	for _, test := range []struct {
		name, field string
	}{
		{"negative chance", `"crit_chance":-0.1`},
		{"chance above one", `"crit_chance":1.1`},
		{"multiplier below one", `"crit_multiplier":0.9`},
		{"negative reduction", `"damage_reduction":-0.1`},
		{"reduction above cap", `"damage_reduction":0.61`},
		{"negative damage", `"damage":-1`},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := fmt.Sprintf(`{"weapons":[{"type":"sword","name":"sword","range_class":"melee","attack_speed":1,"impact_ratio":0.5,%s}]}`, test.field)
			if _, err := ParseWeaponCatalog([]byte(data)); err == nil {
				t.Fatalf("invalid stat accepted: %s", test.field)
			}
		})
	}
}
