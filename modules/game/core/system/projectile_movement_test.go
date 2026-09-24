package system

import (
	"testing"

	"squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestEncodeProjectileMovementSnapshot(t *testing.T) {
	projectile := &combat.Projectile{ID: "projectile-1", Position: entity.Vector2{X: 3, Y: 4}}
	data, err := EncodeProjectileMovementSnapshot(9, []*combat.Projectile{projectile, nil})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot ProjectileMovementSnapshot
	if err = proto.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Tick != 9 || len(snapshot.Projectiles) != 1 {
		t.Fatalf("unexpected projectile movement snapshot: %+v", &snapshot)
	}
	movement := snapshot.Projectiles[0]
	if movement.ProjectileId != projectile.ID || movement.Position.X != 3 || movement.Position.Y != 4 {
		t.Fatalf("unexpected projectile movement: %+v", movement)
	}
}
