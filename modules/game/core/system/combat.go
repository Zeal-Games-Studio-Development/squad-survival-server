package system

import (
	"fmt"

	corecombat "squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func EncodeCombatEventBatch(tick int64, events []corecombat.Event) ([]byte, error) {
	batch := &CombatEventBatch{
		Tick:   tick,
		Events: make([]*CombatEvent, 0, len(events)),
	}
	for _, event := range events {
		encoded, err := combatEventSnapshot(event)
		if err != nil {
			return nil, err
		}
		batch.Events = append(batch.Events, encoded)
	}
	return proto.Marshal(batch)
}

func combatEventSnapshot(event corecombat.Event) (*CombatEvent, error) {
	switch event.Type {
	case corecombat.EventSkillStarted:
		return &CombatEvent{Event: &CombatEvent_SkillStarted{SkillStarted: &SkillStarted{
			ActionId: event.AttackID, SkillId: event.SkillID,
			AttackerUserId: event.AttackerUserID, AttackerCharacterId: event.AttackerCharacterID,
			TargetUserId: event.TargetUserID, TargetCharacterId: event.TargetCharacterID,
			WeaponId: event.WeaponID, Direction: vectorSnapshot(event.Direction),
			StartTick: event.StartTick, ImpactTick: event.ImpactTick, CompleteTick: event.CompleteTick,
			Tags: tagNames(event.Tags),
		}}}, nil
	case corecombat.EventAttackStarted:
		return &CombatEvent{Event: &CombatEvent_AttackStarted{AttackStarted: &AttackStarted{
			ActionId: event.AttackID, AttackerUserId: event.AttackerUserID, AttackerCharacterId: event.AttackerCharacterID,
			TargetUserId: event.TargetUserID, TargetCharacterId: event.TargetCharacterID, WeaponType: string(event.WeaponType),
			StartTick: event.StartTick, ImpactTick: event.ImpactTick, CompleteTick: event.CompleteTick, WeaponId: event.WeaponID,
			AttackCount: int32(event.AttackCount), Tags: tagNames(event.Tags),
		}}}, nil
	case corecombat.EventDamageApplied:
		return &CombatEvent{Event: &CombatEvent_DamageApplied{DamageApplied: &DamageApplied{
			ActionId: event.AttackID, AttackerUserId: event.AttackerUserID, AttackerCharacterId: event.AttackerCharacterID,
			TargetUserId: event.TargetUserID, TargetCharacterId: event.TargetCharacterID,
			Damage: event.Damage, RemainingHealth: event.RemainingHealth, Tick: event.Tick, ProjectileId: event.ProjectileID,
			Critical: event.Critical, HitId: event.HitID, Tags: tagNames(event.Tags),
		}}}, nil
	case corecombat.EventCharacterDied:
		return &CombatEvent{Event: &CombatEvent_CharacterDied{CharacterDied: &CharacterDied{
			ActionId: event.AttackID, KillerUserId: event.AttackerUserID, KillerCharacterId: event.AttackerCharacterID,
			TargetUserId: event.TargetUserID, TargetCharacterId: event.TargetCharacterID, Tick: event.Tick, ProjectileId: event.ProjectileID,
			HitId: event.HitID, Tags: tagNames(event.Tags),
		}}}, nil
	case corecombat.EventProjectileSpawned:
		return &CombatEvent{Event: &CombatEvent_ProjectileSpawned{ProjectileSpawned: &ProjectileSpawned{
			ProjectileId: event.ProjectileID, ActionId: event.AttackID,
			AttackerUserId: event.AttackerUserID, AttackerCharacterId: event.AttackerCharacterID,
			TargetUserId: event.TargetUserID, TargetCharacterId: event.TargetCharacterID,
			WeaponType: string(event.WeaponType), WeaponId: event.WeaponID,
			Position: vectorSnapshot(event.Position), Direction: vectorSnapshot(event.Direction), Speed: event.Speed, Tick: event.Tick,
			HitId: event.HitID, Tags: tagNames(event.Tags),
		}}}, nil
	case corecombat.EventProjectileHit:
		return &CombatEvent{Event: &CombatEvent_ProjectileHit{ProjectileHit: &ProjectileHit{
			ProjectileId: event.ProjectileID, ActionId: event.AttackID,
			AttackerUserId: event.AttackerUserID, AttackerCharacterId: event.AttackerCharacterID,
			TargetUserId: event.TargetUserID, TargetCharacterId: event.TargetCharacterID,
			Position: vectorSnapshot(event.Position), Tick: event.Tick, HitId: event.HitID, Tags: tagNames(event.Tags),
		}}}, nil
	case corecombat.EventProjectileExpired:
		return &CombatEvent{Event: &CombatEvent_ProjectileExpired{ProjectileExpired: &ProjectileExpired{
			ProjectileId: event.ProjectileID, ActionId: event.AttackID,
			AttackerUserId: event.AttackerUserID, AttackerCharacterId: event.AttackerCharacterID,
			TargetUserId: event.TargetUserID, TargetCharacterId: event.TargetCharacterID,
			Position: vectorSnapshot(event.Position), Tick: event.Tick, HitId: event.HitID, Tags: tagNames(event.Tags),
		}}}, nil
	default:
		return nil, fmt.Errorf("unknown combat event type %d", event.Type)
	}
}

func tagNames(tags []entity.ActionTag) []string {
	names := make([]string, len(tags))
	for index, tag := range tags {
		names[index] = string(tag)
	}
	return names
}
