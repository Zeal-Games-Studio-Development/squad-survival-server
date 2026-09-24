package entity

import (
	"errors"
	"math"
	"sort"

	"squad-survival-be/modules/game/core/strategy"
	"squad-survival-be/modules/game/core/world"
)

const CharacterFollowSpeedMultiplier = 1.5

var (
	ErrNilCharacter     = errors.New("character is nil")
	ErrCharacterLimit   = errors.New("player character limit reached")
	ErrStrategyCapacity = errors.New("strategy does not have enough open slots")
	ErrInvalidStrategy  = errors.New("strategy is invalid")
)

func (p *Player) SetStrategy(definition strategy.Definition) error {
	if definition.Type == "" || definition.Capacity() == 0 {
		return ErrInvalidStrategy
	}
	if p.CharacterCount() > definition.Capacity() {
		return ErrStrategyCapacity
	}

	previous := p.Strategy
	p.Strategy = definition
	if err := AssignCharacterTargets(p); err != nil {
		p.Strategy = previous
		return err
	}
	return nil
}

func (p *Player) AddCharacter(character *Character) error {
	if character == nil {
		return ErrNilCharacter
	}
	if p.CharacterCount() >= strategy.MaxCharacters {
		return ErrCharacterLimit
	}
	if p.CharacterCount()+1 > p.Strategy.Capacity() {
		return ErrStrategyCapacity
	}

	p.Characters = append(p.Characters, character)
	p.assignCharacterID(character)
	if err := AssignCharacterTargets(p); err != nil {
		p.Characters = p.Characters[:len(p.Characters)-1]
		return err
	}
	character.Position = character.TargetPosition
	p.MarkRosterChanged()
	return nil
}

func (p *Player) RemoveCharacterAt(index int) bool {
	if index < 0 || index >= len(p.Characters) {
		return false
	}
	p.Characters = append(p.Characters[:index], p.Characters[index+1:]...)
	_ = AssignCharacterTargets(p)
	p.MarkRosterChanged()
	return true
}

func (p *Player) CharacterCount() int {
	count := 0
	for _, character := range p.Characters {
		if character != nil {
			count++
		}
	}
	return count
}

func (p *Player) RemoveDeadCharacters() int {
	survivors := p.Characters[:0]
	removed := 0
	for _, character := range p.Characters {
		if character != nil && character.Health <= 0 {
			removed++
			continue
		}
		survivors = append(survivors, character)
	}
	p.Characters = survivors
	if removed > 0 {
		_ = AssignCharacterTargets(p)
		p.MarkRosterChanged()
	}
	return removed
}

func AssignCharacterTargets(player *Player) error {
	if player == nil || player.Strategy.Type == "" {
		return ErrInvalidStrategy
	}
	slots := player.Strategy.Slots()
	type indexedCharacter struct {
		character *Character
	}
	characters := make([]indexedCharacter, 0, len(player.Characters))
	for _, character := range player.Characters {
		if character != nil {
			characters = append(characters, indexedCharacter{character: character})
		}
	}
	if len(characters) > len(slots) {
		return ErrStrategyCapacity
	}
	sort.SliceStable(characters, func(i, j int) bool {
		return rangeClassPriority(characters[i].character.RangeClass) < rangeClassPriority(characters[j].character.RangeClass)
	})

	facing := NormalizeDirection(player.Facing)
	if facing == (Vector2{}) {
		facing = Vector2{X: 1}
	}
	right := Vector2{X: facing.Y, Y: -facing.X}
	for index, item := range characters {
		offset := slots[index].Offset
		target := Vector2{
			X: player.Position.X + right.X*offset.X + facing.X*offset.Y,
			Y: player.Position.Y + right.Y*offset.X + facing.Y*offset.Y,
		}
		item.character.TargetPosition = world.ClampToPlayArea(target)
	}
	return nil
}

func rangeClassPriority(rangeClass RangeClass) int {
	switch rangeClass {
	case RangeRanged:
		return 0
	case RangeMelee:
		return 1
	default:
		return 2
	}
}

func StepCharacters(player *Player) error {
	if err := AssignCharacterTargets(player); err != nil {
		return err
	}
	deltaSeconds := 1.0 / float64(TickRate)
	for _, character := range player.Characters {
		if character == nil || !isFinite(character.MoveSpeed) || character.MoveSpeed <= 0 {
			continue
		}
		character.Position = moveTowards(
			character.Position,
			character.TargetPosition,
			character.MoveSpeed*CharacterFollowSpeedMultiplier*deltaSeconds,
		)
		character.Position = world.ClampToPlayArea(character.Position)
	}
	return nil
}

func moveTowards(position, target Vector2, maximumDistance float64) Vector2 {
	deltaX := target.X - position.X
	deltaY := target.Y - position.Y
	distance := math.Hypot(deltaX, deltaY)
	if distance == 0 || distance <= maximumDistance {
		return target
	}
	return Vector2{
		X: position.X + deltaX/distance*maximumDistance,
		Y: position.Y + deltaY/distance*maximumDistance,
	}
}
