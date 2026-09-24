package system

import (
	"squad-survival-be/modules/game/core/combat"

	"google.golang.org/protobuf/proto"
)

func EncodeProjectileMovementSnapshot(tick int64, projectiles []*combat.Projectile) ([]byte, error) {
	snapshot := &ProjectileMovementSnapshot{
		Tick: tick, Projectiles: make([]*ProjectileMovement, 0, len(projectiles)),
	}
	for _, projectile := range projectiles {
		if projectile == nil {
			continue
		}
		snapshot.Projectiles = append(snapshot.Projectiles, &ProjectileMovement{
			ProjectileId: projectile.ID,
			Position:     vectorSnapshot(projectile.Position),
		})
	}
	return proto.Marshal(snapshot)
}
