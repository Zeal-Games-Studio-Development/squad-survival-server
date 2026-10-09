package entity

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
)

type SkillCooldownSpec struct {
	Mode     CooldownMode `json:"mode"`
	Required float64      `json:"required"`
}

type SkillTiming struct {
	ImpactTicks   int64 `json:"impact_ticks"`
	CompleteTicks int64 `json:"complete_ticks"`
}

type SkillTarget struct {
	Relation     string  `json:"relation"`
	Shape        string  `json:"shape"`
	AngleDegrees float64 `json:"angle_degrees"`
	RangeScale   float64 `json:"range_scale"`
}

type SkillEffect struct {
	Kind             string  `json:"kind"`
	DamageMultiplier float64 `json:"damage_multiplier"`
	CanCrit          bool    `json:"can_crit"`
}

type SkillDefinition struct {
	ID        string            `json:"skill_id"`
	WeaponID  string            `json:"weapon_id"`
	WeaponIDs []string          `json:"weapon_ids"`
	Priority  int               `json:"priority"`
	Cooldown  SkillCooldownSpec `json:"cooldown"`
	Timing    SkillTiming       `json:"timing"`
	Target    SkillTarget       `json:"target"`
	Effect    SkillEffect       `json:"effect"`
}

//go:embed skills.json
var defaultSkillJSON []byte

var defaultSkillOnce sync.Once
var defaultSkills []SkillDefinition

func ParseSkillCatalog(data []byte, weapons []Weapon) ([]SkillDefinition, error) {
	var catalog struct {
		Skills []SkillDefinition `json:"skills"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	weaponIDs := make(map[string]bool, len(weapons))
	for _, weapon := range weapons {
		weaponIDs[weapon.ID] = true
	}
	seen := make(map[string]bool, len(catalog.Skills))
	for _, skill := range catalog.Skills {
		if skill.ID == "" || seen[skill.ID] {
			return nil, fmt.Errorf("missing or duplicate skill id %q", skill.ID)
		}
		seen[skill.ID] = true
		ids := skill.WeaponIDs
		if skill.WeaponID != "" {
			ids = append([]string{skill.WeaponID}, ids...)
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("skill %q has no weapon ids", skill.ID)
		}
		boundIDs := make(map[string]bool, len(ids))
		for _, id := range ids {
			if !weaponIDs[id] {
				return nil, fmt.Errorf("skill %q references unknown weapon id %q", skill.ID, id)
			}
			if boundIDs[id] {
				return nil, fmt.Errorf("skill %q repeats weapon id %q", skill.ID, id)
			}
			boundIDs[id] = true
		}
		if !validCooldownMode(skill.Cooldown.Mode) || !finitePositive(skill.Cooldown.Required) {
			return nil, fmt.Errorf("skill %q has invalid cooldown", skill.ID)
		}
		if skill.Timing.ImpactTicks < 1 || skill.Timing.CompleteTicks < skill.Timing.ImpactTicks {
			return nil, fmt.Errorf("skill %q has invalid timing", skill.ID)
		}
		if skill.Target.Relation != "self" && skill.Target.Relation != "ally" && skill.Target.Relation != "enemy" {
			return nil, fmt.Errorf("skill %q has invalid target relation", skill.ID)
		}
		if skill.Target.Shape != "cone" && skill.Target.Shape != "single" {
			return nil, fmt.Errorf("skill %q has unsupported target shape", skill.ID)
		}
		if !finitePositive(skill.Target.RangeScale) || skill.Target.Shape == "cone" && (!finitePositive(skill.Target.AngleDegrees) || skill.Target.AngleDegrees > 360) {
			return nil, fmt.Errorf("skill %q has invalid target geometry", skill.ID)
		}
		if skill.Effect.Kind != "damage" || !finitePositive(skill.Effect.DamageMultiplier) {
			return nil, fmt.Errorf("skill %q has unsupported effect", skill.ID)
		}
	}
	sort.Slice(catalog.Skills, func(i, j int) bool {
		if catalog.Skills[i].Priority != catalog.Skills[j].Priority {
			return catalog.Skills[i].Priority > catalog.Skills[j].Priority
		}
		return catalog.Skills[i].ID < catalog.Skills[j].ID
	})
	return catalog.Skills, nil
}

func validCooldownMode(mode CooldownMode) bool {
	return mode == CooldownTicks || mode == CooldownAttackActions || mode == CooldownDamageReceived
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func DefaultSkillCatalog() []SkillDefinition {
	defaultSkillOnce.Do(func() {
		var err error
		defaultSkills, err = ParseSkillCatalog(defaultSkillJSON, DefaultWeaponCatalog())
		if err != nil {
			panic("invalid embedded skill catalog: " + err.Error())
		}
	})
	return defaultSkills
}

func SkillsForWeapon(weaponID string) []SkillDefinition {
	var result []SkillDefinition
	for _, skill := range DefaultSkillCatalog() {
		if skill.WeaponID == weaponID {
			result = append(result, skill)
			continue
		}
		for _, id := range skill.WeaponIDs {
			if id == weaponID {
				result = append(result, skill)
				break
			}
		}
	}
	return result
}
