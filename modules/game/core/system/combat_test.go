package system

import (
	"testing"

	corecombat "squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestEncodeCombatEventBatch(t *testing.T) {
	events := []corecombat.Event{
		{
			Type: corecombat.EventAttackStarted, AttackID: "a:1:1",
			AttackerUserID: "a", AttackerCharacterID: "a:1", TargetUserID: "b", TargetCharacterID: "b:1",
			WeaponType: entity.WeaponSword, StartTick: 10, ImpactTick: 12, CompleteTick: 15,
		},
		{
			Type: corecombat.EventDamageApplied, AttackID: "a:1:1",
			AttackerUserID: "a", AttackerCharacterID: "a:1", TargetUserID: "b", TargetCharacterID: "b:1",
			Damage: 11, RemainingHealth: 89, Tick: 12, Critical: true,
		},
		{
			Type: corecombat.EventCharacterDied, AttackID: "a:1:1",
			AttackerUserID: "a", AttackerCharacterID: "a:1", TargetUserID: "b", TargetCharacterID: "b:1", Tick: 12,
		},
	}

	data, err := EncodeCombatEventBatch(12, events)
	if err != nil {
		t.Fatal(err)
	}
	var batch CombatEventBatch
	if err = proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Tick != 12 || len(batch.Events) != 3 {
		t.Fatalf("unexpected combat batch: %+v", &batch)
	}
	if started := batch.Events[0].GetAttackStarted(); started == nil || started.AttackId != "a:1:1" || started.ImpactTick != 12 {
		t.Fatalf("unexpected attack started event: %+v", started)
	}
	if damage := batch.Events[1].GetDamageApplied(); damage == nil || damage.Damage != 11 || damage.RemainingHealth != 89 || !damage.Critical {
		t.Fatalf("unexpected damage event: %+v", damage)
	}
	if died := batch.Events[2].GetCharacterDied(); died == nil || died.KillerCharacterId != "a:1" || died.TargetCharacterId != "b:1" {
		t.Fatalf("unexpected death event: %+v", died)
	}
}

func TestEncodeCombatEventBatchRejectsUnknownEvent(t *testing.T) {
	if _, err := EncodeCombatEventBatch(1, []corecombat.Event{{}}); err == nil {
		t.Fatal("expected unknown combat event to fail encoding")
	}
}

func TestEncodeProjectileCombatEvents(t *testing.T) {
	events := []corecombat.Event{
		{Type: corecombat.EventProjectileSpawned, ProjectileID: "p1", AttackID: "a1", AttackerUserID: "u1", AttackerCharacterID: "c1", TargetUserID: "u2", TargetCharacterID: "c2", WeaponType: entity.WeaponBow, WeaponName: "bow", Position: entity.Vector2{X: 1}, Direction: entity.Vector2{Y: 1}, Speed: 12, Tick: 4},
		{Type: corecombat.EventProjectileHit, ProjectileID: "p1", AttackID: "a1", AttackerUserID: "u1", AttackerCharacterID: "c1", TargetUserID: "u2", TargetCharacterID: "c2", Position: entity.Vector2{X: 2}, Tick: 5},
		{Type: corecombat.EventProjectileExpired, ProjectileID: "p2", AttackID: "a2", AttackerUserID: "u1", AttackerCharacterID: "c1", TargetUserID: "u2", TargetCharacterID: "c2", Position: entity.Vector2{X: 3}, Tick: 6},
	}
	data, err := EncodeCombatEventBatch(6, events)
	if err != nil {
		t.Fatal(err)
	}
	var batch CombatEventBatch
	if err = proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if spawned := batch.Events[0].GetProjectileSpawned(); spawned == nil || spawned.ProjectileId != "p1" || spawned.WeaponName != "bow" || spawned.Speed != 12 {
		t.Fatalf("unexpected projectile spawned: %+v", spawned)
	}
	if hit := batch.Events[1].GetProjectileHit(); hit == nil || hit.ProjectileId != "p1" || hit.Position.X != 2 {
		t.Fatalf("unexpected projectile hit: %+v", hit)
	}
	if expired := batch.Events[2].GetProjectileExpired(); expired == nil || expired.ProjectileId != "p2" || expired.Position.X != 3 {
		t.Fatalf("unexpected projectile expired: %+v", expired)
	}
}
