package damage

import (
	"math"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func TestAxeAndBluntBonusUsesTargetRangeClass(t *testing.T) {
	attacker := &entity.Character{Damage: 10, CritMultiplier: 1.5}
	for _, test := range []struct {
		weapon     entity.WeaponType
		rangeClass entity.RangeClass
		want       float64
	}{
		{entity.WeaponAxe, entity.RangeRanged, 13.5},
		{entity.WeaponAxe, entity.RangeMelee, 10},
		{entity.WeaponBlunt, entity.RangeMelee, 13.5},
		{entity.WeaponBlunt, entity.RangeRanged, 10},
	} {
		attacker.Weapon.Type = test.weapon
		amount, _ := RollAttackAgainst(attacker, &entity.Character{RangeClass: test.rangeClass}, nil)
		if amount != test.want {
			t.Fatalf("%s versus %s: got %f, want %f", test.weapon, test.rangeClass, amount, test.want)
		}
	}
}

func TestCrossbowCritBonusIsTargetSpecific(t *testing.T) {
	attacker := &entity.Character{Weapon: entity.Weapon{Type: entity.WeaponCrossbow}, Damage: 10, CritChance: 0, CritMultiplier: 1.5}
	for _, test := range []struct {
		health   float64
		critical bool
	}{
		{100, false}, {75, false}, {50, true}, {20, true},
	} {
		target := &entity.Character{Health: test.health, MaxHealth: 100}
		_, critical := RollAttackAgainst(attacker, target, rand.New(fixedSource{}))
		if critical != test.critical {
			t.Fatalf("health=%f critical=%v, want %v", test.health, critical, test.critical)
		}
	}
}

// Float64 from this source is close to 0.25.
type fixedSource struct{}

func (fixedSource) Seed(int64)   {}
func (fixedSource) Int63() int64 { return 1 << 61 }

func TestShieldProjectileReductionIsMultiplicative(t *testing.T) {
	shield := &entity.Character{Weapon: entity.Weapon{Type: entity.WeaponShield}, Health: 100, DamageReduction: 0.1}
	dealt, _, _ := ApplyTagged(shield, 100, []entity.ActionTag{entity.TagBasicAttack, entity.TagProjectile})
	if math.Abs(dealt-54) > 1e-9 {
		t.Fatalf("shield projectile damage=%f, want 54", dealt)
	}
	dealt, _, _ = ApplyTagged(shield, 10, []entity.ActionTag{entity.TagBasicAttack})
	if math.Abs(dealt-9) > 1e-9 {
		t.Fatalf("shield reduced non-projectile damage: %f", dealt)
	}
}
