package system

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestEncodePlayerProgressionBatch(t *testing.T) {
	player := entity.NewPlayer("user-1", "session-1", "Player", entity.Vector2{}, rand.New(rand.NewSource(1)))
	player.Level = 3
	player.Experience = 175
	data, err := EncodePlayerProgressionBatch(12, []*entity.Player{player, nil})
	if err != nil {
		t.Fatal(err)
	}
	var batch PlayerProgressionBatch
	if err = proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Tick != 12 || len(batch.Players) != 1 {
		t.Fatalf("unexpected progression batch: %+v", &batch)
	}
	state := batch.Players[0]
	if state.UserId != player.UserID || state.SessionId != player.SessionID || state.Level != 3 || state.Experience != 175 {
		t.Fatalf("unexpected player progression: %+v", state)
	}
}
