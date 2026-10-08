package entity

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
)

type WeaponType string

type RangeClass string

const (
	WeaponBow      WeaponType = "bow"
	WeaponStaff    WeaponType = "staff"
	WeaponSpear    WeaponType = "spear"
	WeaponSword    WeaponType = "sword"
	WeaponWand     WeaponType = "wand"
	WeaponAxe      WeaponType = "axe"
	WeaponBlunt    WeaponType = "blunt"
	WeaponCrossbow WeaponType = "crossbow"
)

const (
	RangeMelee  RangeClass = "melee"
	RangeRanged RangeClass = "ranged"
)

type Weapon struct {
	Type            WeaponType `json:"type"`
	ID              string     `json:"weapon_id"`
	RangeClass      RangeClass `json:"range_class"`
	Health          float64    `json:"health"`
	Damage          float64    `json:"damage"`
	MoveSpeed       float64    `json:"move_speed"` // World units per second.
	AttackSpeed     float64    `json:"attack_speed"`
	AttackRange     float64    `json:"attack_range"`
	ImpactRatio     float64    `json:"impact_ratio"`
	ProjectileSpeed float64    `json:"projectile_speed"`
	RegenRate       float64    `json:"regen_rate"`
	CritChance      float64    `json:"crit_chance"`
	CritMultiplier  float64    `json:"crit_multiplier"`
	DamageReduction float64    `json:"damage_reduction"`
}

type WeaponCatalog struct {
	Weapons []Weapon `json:"weapons"`
}

//go:embed weapons.json
var defaultWeaponJSON []byte

func ParseWeaponCatalog(data []byte) ([]Weapon, error) {
	var catalog WeaponCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	if len(catalog.Weapons) == 0 {
		return nil, errors.New("weapon catalog has no weapons")
	}
	seenIDs := make(map[string]struct{}, len(catalog.Weapons))
	for index := range catalog.Weapons {
		weapon := &catalog.Weapons[index]
		if weapon.Type == "" {
			return nil, errors.New("weapon type is required")
		}
		if weapon.ID == "" {
			return nil, errors.New("weapon id is required")
		}
		if _, exists := seenIDs[weapon.ID]; exists {
			return nil, fmt.Errorf("duplicate weapon id %q", weapon.ID)
		}
		seenIDs[weapon.ID] = struct{}{}
		if !weapon.RangeClass.Valid() {
			return nil, errors.New("weapon range class must be melee or ranged")
		}
		if !isFinite(weapon.AttackRange) || weapon.AttackRange < 0 {
			return nil, errors.New("weapon attack range must be finite and non-negative")
		}
		if !isFinite(weapon.AttackSpeed) || weapon.AttackSpeed <= 0 {
			return nil, errors.New("weapon attack speed must be finite and greater than zero")
		}
		if !isFinite(weapon.ImpactRatio) || weapon.ImpactRatio <= 0 || weapon.ImpactRatio > 1 {
			return nil, errors.New("weapon impact ratio must be finite and within (0, 1]")
		}
		if !isFinite(weapon.ProjectileSpeed) || weapon.ProjectileSpeed < 0 || weapon.RangeClass == RangeRanged && weapon.ProjectileSpeed == 0 {
			return nil, errors.New("ranged weapon projectile speed must be finite and greater than zero")
		}
		if !isFinite(weapon.Damage) || weapon.Damage < 0 {
			return nil, fmt.Errorf("weapon %q damage must be finite and non-negative", weapon.Type)
		}
		if !isFinite(weapon.CritChance) || weapon.CritChance < 0 || weapon.CritChance > 1 {
			return nil, fmt.Errorf("weapon %q crit chance must be within [0, 1]", weapon.Type)
		}
		if weapon.CritMultiplier == 0 {
			weapon.CritMultiplier = DefaultCritMultiplier
		}
		if !isFinite(weapon.CritMultiplier) || weapon.CritMultiplier < 1 {
			return nil, fmt.Errorf("weapon %q crit multiplier must be at least 1", weapon.Type)
		}
		if !isFinite(weapon.DamageReduction) || weapon.DamageReduction < 0 || weapon.DamageReduction > 0.6 {
			return nil, fmt.Errorf("weapon %q damage reduction must be within [0, 0.6]", weapon.Type)
		}
	}
	return catalog.Weapons, nil
}

func (r RangeClass) Valid() bool {
	return r == RangeMelee || r == RangeRanged
}

func DefaultWeaponCatalog() []Weapon {
	weapons, err := ParseWeaponCatalog(defaultWeaponJSON)
	if err != nil {
		panic("invalid embedded weapon catalog: " + err.Error())
	}
	return weapons
}

func RandomWeapon(random *rand.Rand, weapons []Weapon) Weapon {
	if len(weapons) == 0 {
		weapons = DefaultWeaponCatalog()
	}
	return weapons[random.Intn(len(weapons))]
}
