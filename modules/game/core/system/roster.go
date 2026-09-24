package system

import (
	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func EncodePlayerRosterBatch(tick int64, players []*entity.Player) ([]byte, error) {
	batch := &PlayerRosterBatch{Tick: tick, Players: make([]*PlayerRoster, 0, len(players))}
	for _, player := range players {
		if roster := playerRoster(player); roster != nil {
			batch.Players = append(batch.Players, roster)
		}
	}
	return proto.Marshal(batch)
}

func playerRoster(player *entity.Player) *PlayerRoster {
	if player == nil {
		return nil
	}
	characters := make([]*CharacterRoster, 0, len(player.Characters))
	for _, character := range player.Characters {
		if character == nil {
			continue
		}
		characters = append(characters, &CharacterRoster{
			CharacterId: character.ID,
			Health:      character.Health, MaxHealth: character.MaxHealth,
			Damage: character.Damage, MoveSpeed: character.MoveSpeed,
			AttackSpeed: character.AttackSpeed, AttackRange: character.AttackRange,
			RegenRate: character.RegenRate, DamageRatio: character.DamageRatio,
			WeaponType: string(character.Weapon.Type), RangeClass: string(character.RangeClass),
			WeaponName: character.Weapon.Name,
		})
	}
	return &PlayerRoster{
		UserId: player.UserID, SessionId: player.SessionID, DisplayName: player.DisplayName,
		RosterVersion: player.RosterVersion, Characters: characters,
	}
}
