package damage

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func TestDamageValues(t *testing.T) {
	for _, test := range []struct {
		name      string
		chance    float64
		reduction float64
		wantRaw   float64
		wantDealt float64
		wantCrit  bool
	}{
		{"normal", 0, 0, 10, 10, false},
		{"critical", 1, 0, 15, 15, true},
		{"reduced", 0, 0.6, 10, 4, false},
		{"critical reduced", 1, 0.6, 15, 6, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			attacker := &entity.Character{Damage: 10, CritChance: test.chance, CritMultiplier: 1.5}
			target := &entity.Character{Health: 100, DamageReduction: test.reduction}
			raw, critical := RollAttack(attacker, rand.New(rand.NewSource(1)))
			dealt, remaining, died := Apply(target, raw)
			if raw != test.wantRaw || critical != test.wantCrit || dealt != test.wantDealt || remaining != 100-test.wantDealt || died {
				t.Fatalf("raw=%v critical=%v dealt=%v remaining=%v died=%v", raw, critical, dealt, remaining, died)
			}
		})
	}
}

func TestCritChanceBoundaryUsesInjectedRandom(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	wantRandom := rand.New(rand.NewSource(7))
	first := wantRandom.Float64()
	attacker := &entity.Character{Damage: 10, CritMultiplier: 1.5, CritChance: first}
	amount, critical := RollAttack(attacker, random)
	if critical || amount != 10 {
		t.Fatalf("equal roll should not crit: amount=%v critical=%v", amount, critical)
	}
	attacker.CritChance = 1
	if amount, critical = RollAttack(attacker, random); !critical || amount != 15 {
		t.Fatalf("100%% chance should crit: amount=%v critical=%v", amount, critical)
	}
}

func TestApplyCapsReductionAndHealth(t *testing.T) {
	target := &entity.Character{Health: 5, DamageReduction: 0.9}
	dealt, remaining, died := Apply(target, 20)
	if dealt != 5 || remaining != 0 || target.Health != 0 || !died {
		t.Fatalf("overkill should consume five health: dealt=%v remaining=%v died=%v", dealt, remaining, died)
	}
	if dealt, remaining, died = Apply(target, 20); dealt != 0 || remaining != 0 || died {
		t.Fatalf("dead target took another hit: dealt=%v remaining=%v died=%v", dealt, remaining, died)
	}
	target = &entity.Character{Health: 100, DamageReduction: 0.9}
	if dealt, _, _ = Apply(target, 10); dealt != 4 {
		t.Fatalf("reduction above cap must apply as 60%%: dealt=%v", dealt)
	}
}
