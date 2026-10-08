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
	Weapon             Weapon
	RangeClass         RangeClass
	Position           Vector2
	TargetPosition     Vector2
	Health             float64
	MaxHealth          float64
	Damage             float64
	MoveSpeed          float64 // World units per second.
	AttackSpeed        float64
	AttackCount        int
	CooldownScale      float64
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
	AttackActionCount  int
	AttackHitsEmitted  int
	AttackDirection    Vector2
	ActiveSkillID      string
	SkillActionID      string
	SkillDirection     Vector2
	SkillStartTick     int64
	SkillImpactTick    int64
	SkillCompleteTick  int64
	SkillImpacted      bool
	Cooldowns          map[string]*Cooldown
	SkillStateVersion  uint64
	LastCooldownTick   int64
	CooldownTickSet    bool
}

func CreateCharacter(random *rand.Rand, weapons []Weapon) *Character {
	character := NewCharacter()
	character.ApplyWeapon(RandomWeapon(random, weapons))
	return character
}

func (c *Character) ApplyWeapon(weapon Weapon) {
	c.Weapon = weapon
	c.Cooldowns = nil
	for _, skill := range SkillsForWeapon(weapon.ID) {
		if err := c.StartCooldown(skill.ID, skill.Cooldown.Mode, skill.Cooldown.Required); err != nil {
			panic("invalid skill cooldown: " + err.Error())
		}
	}
	c.RangeClass = weapon.RangeClass
	c.MaxHealth = weapon.Health
	c.Health = c.MaxHealth
	c.Damage = weapon.Damage
	c.MoveSpeed = weapon.MoveSpeed
	c.AttackSpeed = weapon.AttackSpeed
	c.AttackCount = weapon.AttackCount
	if c.AttackCount < 1 {
		c.AttackCount = 1
	}
	c.CooldownScale = weapon.CooldownScale
	if c.CooldownScale <= 0 {
		c.CooldownScale = 1
	}
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

func (c *Character) ResetSkill() {
	if c.ActiveSkillID != "" {
		c.SkillStateVersion++
	}
	c.ActiveSkillID = ""
	c.SkillActionID = ""
	c.SkillDirection = Vector2{}
	c.SkillStartTick = 0
	c.SkillImpactTick = 0
	c.SkillCompleteTick = 0
	c.SkillImpacted = false
}

func (c *Character) ResetAttack() {
	c.TargetUserID = ""
	c.TargetCharacterID = ""
	c.AttackStartTick = 0
	c.AttackImpactTick = 0
	c.AttackCompleteTick = 0
	c.AttackImpacted = false
	c.AttackActionCount = 0
	c.AttackHitsEmitted = 0
	c.AttackDirection = Vector2{}
}

func NewCharacter() *Character {
	return &Character{
		Health:         DefaultHealth,
		MaxHealth:      DefaultHealth,
		Damage:         DefaultDamage,
		MoveSpeed:      DefaultMoveSpeed,
		AttackSpeed:    DefaultAttackSpeed,
		AttackCount:    1,
		CooldownScale:  1,
		RegenRate:      DefaultRegenRate,
		CritMultiplier: DefaultCritMultiplier,
	}
}
