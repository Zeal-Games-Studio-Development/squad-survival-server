package system

import (
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/progression"

	"google.golang.org/protobuf/proto"
)

func EncodePlayerProgressionBatch(tick int64, players []*entity.Player) ([]byte, error) {
	batch := &PlayerProgressionBatch{Tick: tick, Players: make([]*PlayerProgression, 0, len(players))}
	for _, player := range players {
		if player == nil {
			continue
		}
		batch.Players = append(batch.Players, &PlayerProgression{
			UserId: player.UserID, SessionId: player.SessionID,
			Level: int32(player.Level), Experience: player.Experience,
			MaxExperience: progression.MaxExperienceForLevel(player.Level),
		})
	}
	return proto.Marshal(batch)
}
