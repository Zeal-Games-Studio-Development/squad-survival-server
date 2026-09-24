package system

import "squad-survival-be/modules/game/core/entity"

func vectorSnapshot(vector entity.Vector2) *Vector2 {
	return &Vector2{X: vector.X, Y: vector.Y}
}
