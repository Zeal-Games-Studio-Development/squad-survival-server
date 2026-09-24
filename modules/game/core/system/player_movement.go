package system

import (
	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func EncodePlayerMovementSnapshot(tick int64, self *entity.Player, players []*entity.Player) ([]byte, error) {
	snapshot := &PlayerMovementSnapshot{
		Tick: tick, Self: playerMovementSnapshot(self),
		Players: make([]*PlayerMovement, 0, len(players)),
	}
	for _, player := range players {
		if movement := playerMovementSnapshot(player); movement != nil {
			snapshot.Players = append(snapshot.Players, movement)
		}
	}
	return proto.Marshal(snapshot)
}

func playerMovementSnapshot(player *entity.Player) *PlayerMovement {
	if player == nil {
		return nil
	}
	characters := make([]*CharacterMovement, 0, len(player.Characters))
	for _, character := range player.Characters {
		if character == nil {
			continue
		}
		characters = append(characters, &CharacterMovement{
			CharacterId: character.ID,
			Position:    vectorSnapshot(character.Position),
		})
	}
	return &PlayerMovement{
		SessionId: player.SessionID, Position: vectorSnapshot(player.Position),
		Facing: vectorSnapshot(player.Facing), Direction: vectorSnapshot(player.Direction),
		Characters: characters,
	}
}
