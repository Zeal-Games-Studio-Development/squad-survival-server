package entity

import (
	"math"
	"math/rand"
	"strconv"

	"squad-survival-be/modules/game/core/progression"
	"squad-survival-be/modules/game/core/strategy"
	"squad-survival-be/modules/game/core/world"
)

const (
	InputTimeoutTicks      = int64(3)
	TickRate               = 10
	DefaultDetectionRadius = 20.0
)

type Vector2 = world.Vector2

type Player struct {
	UserID                string
	SessionID             string
	DisplayName           string
	Position              Vector2
	Facing                Vector2
	Direction             Vector2
	LastSequence          uint64
	HasSequence           bool
	LastInputTick         int64
	Characters            []*Character
	DetectionRadius       float64
	Strategy              strategy.Definition
	RosterVersion         uint64
	Level                 int
	Experience            uint64
	ProgressionVersion    uint64
	nextCharacterSequence uint64
}

func NewPlayer(userID, sessionID, displayName string, position Vector2, random *rand.Rand) *Player {
	return NewPlayerWithWeapon(userID, sessionID, displayName, position, RandomWeapon(random, DefaultWeaponCatalog()))
}

func NewPlayerWithWeapon(userID, sessionID, displayName string, position Vector2, weapon Weapon) *Player {
	definition, ok := strategy.DefaultDefinition(strategy.DefaultStrategyName)
	if !ok {
		panic("default strategy not found: " + strategy.DefaultStrategyName)
	}
	character := NewCharacter()
	character.ApplyWeapon(weapon)
	player := &Player{
		UserID:             userID,
		SessionID:          sessionID,
		DisplayName:        displayName,
		Position:           world.ClampToPlayArea(position),
		Facing:             Vector2{X: 1},
		Characters:         []*Character{character},
		DetectionRadius:    DefaultDetectionRadius,
		Strategy:           definition,
		RosterVersion:      1,
		Level:              progression.MinLevel,
		Experience:         0,
		ProgressionVersion: 1,
	}
	player.assignCharacterID(character)

	if err := AssignCharacterTargets(player); err != nil {
		panic("could not initialize player strategy: " + err.Error())
	}
	for _, character := range player.Characters {
		if character != nil {
			character.Position = character.TargetPosition
		}
	}
	return player
}

func (p *Player) MaxCharacters() int {
	if p == nil {
		return 0
	}
	return progression.MaxCharactersForLevel(p.Level)
}

func (p *Player) MarkProgressionChanged() {
	p.ProgressionVersion++
}

// AddExperience applies experience, carrying overflow through multiple levels.
// It reports whether the player's public progression state changed.
func (p *Player) AddExperience(amount uint64) bool {
	if p == nil || amount == 0 || p.Level >= progression.MaxLevel {
		return false
	}
	p.Experience += amount
	for p.Level < progression.MaxLevel {
		required := progression.MaxExperienceForLevel(p.Level)
		if required == 0 || p.Experience < required {
			break
		}
		p.Experience -= required
		p.Level++
	}
	if p.Level >= progression.MaxLevel {
		p.Level = progression.MaxLevel
		p.Experience = 0
	}
	p.MarkProgressionChanged()
	return true
}

func ApplyMovementInput(player *Player, input *MovementInput, tick int64) bool {
	if player.IsEliminated() {
		return false
	}
	if player.HasSequence && input.Sequence <= player.LastSequence {
		return false
	}
	if !isFinite(input.X) || !isFinite(input.Y) {
		return false
	}

	direction := NormalizeDirection(Vector2{X: input.X, Y: input.Y})
	player.Direction = direction
	if direction.X != 0 || direction.Y != 0 {
		player.Facing = direction
	}
	player.LastSequence = input.Sequence
	player.HasSequence = true
	player.LastInputTick = tick
	return true
}

func StepMovement(player *Player, tick int64) {
	if player.IsEliminated() {
		player.Direction = Vector2{}
		return
	}
	if player.HasSequence && tick-player.LastInputTick >= InputTimeoutTicks {
		player.Direction = Vector2{}
	}

	deltaSeconds := 1.0 / float64(TickRate)
	moveSpeed := player.MinMoveSpeed()
	player.Position.X += player.Direction.X * moveSpeed * deltaSeconds
	player.Position.Y += player.Direction.Y * moveSpeed * deltaSeconds
	player.Position = world.ClampToPlayArea(player.Position)
}

// IsEliminated reports whether the player has no living characters and can no
// longer perform gameplay actions. The player entity itself remains in-world.
func (p *Player) IsEliminated() bool {
	if p == nil {
		return true
	}
	for _, character := range p.Characters {
		if character != nil && character.Health > 0 {
			return false
		}
	}
	return true
}

func (p *Player) MinMoveSpeed() float64 {
	minimum := math.Inf(1)
	for _, character := range p.Characters {
		if character == nil {
			continue
		}
		if !isFinite(character.MoveSpeed) || character.MoveSpeed <= 0 {
			return 0
		}
		if character.MoveSpeed < minimum {
			minimum = character.MoveSpeed
		}
	}
	if math.IsInf(minimum, 1) {
		return 0
	}
	return minimum
}

func (p *Player) CharactersByRangeClass(rangeClass RangeClass) []*Character {
	characters := make([]*Character, 0)
	for _, character := range p.Characters {
		if character != nil && character.RangeClass == rangeClass {
			characters = append(characters, character)
		}
	}
	return characters
}

func (p *Player) assignCharacterID(character *Character) {
	if character == nil || character.ID != "" {
		return
	}
	p.nextCharacterSequence++
	character.ID = p.UserID + ":" + strconv.FormatUint(p.nextCharacterSequence, 10)
}

// MarkRosterChanged must be called after changing character loadout or static stats.
func (p *Player) MarkRosterChanged() {
	p.RosterVersion++
}

func NormalizeDirection(direction Vector2) Vector2 {
	lengthSquared := direction.X*direction.X + direction.Y*direction.Y
	if lengthSquared == 0 || lengthSquared <= 1 {
		return direction
	}

	length := math.Sqrt(lengthSquared)
	return Vector2{X: direction.X / length, Y: direction.Y / length}
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
