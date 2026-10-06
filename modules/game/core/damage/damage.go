// Package damage calculates attacks and applies damage to characters.
package damage

import (
	"math"
	"math/rand"

	"squad-survival-be/modules/game/core/entity"
)

const MaxReduction = 0.6

// RollAttack uses the character's fixed base damage and rolls only critical chance.
func RollAttack(attacker *entity.Character, random *rand.Rand) (amount float64, critical bool) {
	if attacker == nil || math.IsNaN(attacker.Damage) || attacker.Damage <= 0 {
		return 0, false
	}
	amount = attacker.Damage
	chance := attacker.CritChance
	critical = chance >= 1 || chance > 0 && random != nil && random.Float64() < chance
	if critical {
		multiplier := attacker.CritMultiplier
		if multiplier < 1 || math.IsNaN(multiplier) {
			multiplier = entity.DefaultCritMultiplier
		}
		amount *= multiplier
	}
	return amount, critical
}

// Apply reduces the incoming amount using the target's reduction at impact time.
// It returns health actually lost, remaining health, and whether this hit killed the target.
func Apply(target *entity.Character, incoming float64) (dealt, remaining float64, died bool) {
	if target == nil || target.Health <= 0 || incoming <= 0 || math.IsNaN(incoming) {
		if target != nil {
			return 0, math.Max(0, target.Health), false
		}
		return 0, 0, false
	}
	reduction := target.DamageReduction
	if math.IsNaN(reduction) || reduction < 0 {
		reduction = 0
	} else if reduction > MaxReduction {
		reduction = MaxReduction
	}
	dealt = math.Min(target.Health, incoming*(1-reduction))
	target.Health -= dealt
	if target.Health < 0 {
		target.Health = 0
	}
	return dealt, target.Health, target.Health == 0
}
