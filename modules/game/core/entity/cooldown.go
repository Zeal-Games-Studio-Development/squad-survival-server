package entity

import (
	"errors"
	"math"
)

type CooldownMode string

const (
	CooldownTicks          CooldownMode = "ticks"
	CooldownAttackActions  CooldownMode = "attack_actions"
	CooldownDamageReceived CooldownMode = "damage_received"
)

// Cooldown tracks fractional progress so cooldown_scale works for all three modes.
type Cooldown struct {
	Mode        CooldownMode
	Required    float64
	Progress    float64
	seenActions map[string]struct{}
}

func (c *Character) StartCooldown(skillID string, mode CooldownMode, required float64) error {
	if c == nil || skillID == "" || math.IsNaN(required) || math.IsInf(required, 0) || required <= 0 || mode != CooldownTicks && mode != CooldownAttackActions && mode != CooldownDamageReceived {
		return errors.New("invalid skill cooldown")
	}
	if c.Cooldowns == nil {
		c.Cooldowns = make(map[string]*Cooldown)
	}
	c.Cooldowns[skillID] = &Cooldown{Mode: mode, Required: required}
	c.SkillStateVersion++
	return nil
}

func (c *Character) CooldownReady(skillID string) bool {
	if c == nil {
		return false
	}
	state, exists := c.Cooldowns[skillID]
	return exists && state.Progress >= state.Required
}

func (c *Character) ResetCooldown(skillID string) bool {
	if c == nil {
		return false
	}
	state, exists := c.Cooldowns[skillID]
	if !exists || state == nil || state.Progress < state.Required {
		return false
	}
	state.Progress = 0
	c.SkillStateVersion++
	return true
}

func (c *Character) AdvanceCooldowns(mode CooldownMode, actionID string) {
	if c == nil {
		return
	}
	scale := c.CooldownScale
	if scale <= 0 {
		scale = 1
	}
	for _, state := range c.Cooldowns {
		if state == nil || state.Mode != mode || state.Progress >= state.Required {
			continue
		}
		switch mode {
		case CooldownAttackActions:
			if actionID == "" {
				continue
			}
		case CooldownDamageReceived:
			if actionID == "" {
				continue
			}
		case CooldownTicks:
		default:
			continue
		}
		if mode != CooldownTicks {
			if _, seen := state.seenActions[actionID]; seen {
				continue
			}
			if state.seenActions == nil {
				state.seenActions = make(map[string]struct{})
			}
			state.seenActions[actionID] = struct{}{}
		}
		state.Progress += scale
		c.SkillStateVersion++
		if state.Progress > state.Required {
			state.Progress = state.Required
		}
	}
}
