package entity

import "math/rand"

const (
	DefaultHealth         = 100.0
	DefaultDamage         = 10.0
	DefaultMoveSpeed      = 5.0
	DefaultAttackSpeed    = 1.2
	DefaultRegenRate      = 0.0
	DefaultCritMultiplier = 1.5
)

type Character struct {
	ID                 string
	Skin               Skin
	Weapon             Weapon
	RangeClass         RangeClass
	Position           Vector2
	TargetPosition     Vector2
	Health             float64
	MaxHealth          float64
	Damage             float64
	MoveSpeed          float64 // World units per second.
	AttackSpeed        float64
	AttackRange        float64
	ImpactRatio        float64
	RegenRate          float64
	CritChance         float64
	CritMultiplier     float64
	DamageReduction    float64
	TargetUserID       string
	TargetCharacterID  string
	AttackSequence     uint64
	AttackStartTick    int64
	AttackImpactTick   int64
	AttackCompleteTick int64
	AttackImpacted     bool
}

func CreateCharacter(random *rand.Rand, weapons []Weapon) *Character {
	character := NewCharacter()
	character.ApplyWeapon(RandomWeapon(random, weapons))
	return character
}

func (c *Character) ApplyWeapon(weapon Weapon) {
	c.Weapon = weapon
	c.RangeClass = weapon.RangeClass
	c.MaxHealth = weapon.Health
	c.Health = c.MaxHealth
	c.Damage = weapon.Damage
	c.MoveSpeed = weapon.MoveSpeed
	c.AttackSpeed = weapon.AttackSpeed
	c.AttackRange = weapon.AttackRange
	c.ImpactRatio = weapon.ImpactRatio
	c.RegenRate = weapon.RegenRate
	c.CritChance = weapon.CritChance
	c.CritMultiplier = weapon.CritMultiplier
	if c.CritMultiplier == 0 {
		c.CritMultiplier = DefaultCritMultiplier
	}
	c.DamageReduction = weapon.DamageReduction
}

func (c *Character) ResetAttack() {
	c.TargetUserID = ""
	c.TargetCharacterID = ""
	c.AttackStartTick = 0
	c.AttackImpactTick = 0
	c.AttackCompleteTick = 0
	c.AttackImpacted = false
}

func NewCharacter() *Character {
	return &Character{
		Health:         DefaultHealth,
		MaxHealth:      DefaultHealth,
		Damage:         DefaultDamage,
		MoveSpeed:      DefaultMoveSpeed,
		AttackSpeed:    DefaultAttackSpeed,
		RegenRate:      DefaultRegenRate,
		CritMultiplier: DefaultCritMultiplier,
	}
}
