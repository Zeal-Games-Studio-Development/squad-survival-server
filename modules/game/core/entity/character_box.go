package entity

const CharacterBoxCollisionRadius = 1.0

// CharacterBox is a static, collectible world entity.
type CharacterBox struct {
	ID       string
	Position Vector2
	Value    *CharacterBoxValue
}

func NewCharacterBox(id string, position Vector2, weaponType WeaponType) *CharacterBox {
	return &CharacterBox{
		ID:       id,
		Position: position,
		Value:    &CharacterBoxValue{WeaponType: string(weaponType)},
	}
}

func (b *CharacterBox) WeaponType() WeaponType {
	if b == nil || b.Value == nil {
		return ""
	}
	return WeaponType(b.Value.WeaponType)
}
