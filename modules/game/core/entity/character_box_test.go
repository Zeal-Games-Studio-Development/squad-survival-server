package entity

import "testing"

func TestNewCharacterBoxStoresWeaponValue(t *testing.T) {
	box := NewCharacterBox("box:1", Vector2{X: 2, Y: 3}, WeaponBow)
	if box.ID != "box:1" || box.Position.X != 2 || box.Position.Y != 3 || box.WeaponType() != WeaponBow {
		t.Fatalf("unexpected character box: %#v", box)
	}
	if (*CharacterBox)(nil).WeaponType() != "" {
		t.Fatal("nil character box returned a weapon type")
	}
}
