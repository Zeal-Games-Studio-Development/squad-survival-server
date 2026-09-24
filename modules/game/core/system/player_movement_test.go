package system

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestEncodePlayerMovementSnapshot(t *testing.T) {
	player := entity.NewPlayer("user-1", "session-1", "Player One", entity.Vector2{X: 1, Y: 2}, rand.New(rand.NewSource(1)))
	player.Facing = entity.Vector2{Y: 1}
	player.Direction = entity.Vector2{X: -1}
	player.Characters = append(player.Characters, nil)
	data, err := EncodePlayerMovementSnapshot(42, player, []*entity.Player{player, nil})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot PlayerMovementSnapshot
	if err = proto.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Tick != 42 || len(snapshot.Players) != 1 || snapshot.Self == nil {
		t.Fatalf("unexpected movement envelope: %+v", &snapshot)
	}
	detected := snapshot.Players[0]
	if detected.SessionId != player.SessionID || detected.Position.X != 1 || detected.Position.Y != 2 || detected.Facing.Y != 1 || detected.Direction.X != -1 {
		t.Fatalf("unexpected player movement: %+v", detected)
	}
	if len(detected.Characters) != 1 || detected.Characters[0].CharacterId != player.Characters[0].ID {
		t.Fatalf("unexpected character movement: %+v", detected.Characters)
	}
	if detected.Characters[0].Position.X != player.Characters[0].Position.X || detected.Characters[0].Position.Y != player.Characters[0].Position.Y {
		t.Fatalf("unexpected character position: %+v", detected.Characters[0].Position)
	}
}

func TestEncodeEmptyPlayerMovementSnapshot(t *testing.T) {
	data, err := EncodePlayerMovementSnapshot(7, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot PlayerMovementSnapshot
	if err = proto.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Tick != 7 || snapshot.Self != nil || len(snapshot.Players) != 0 {
		t.Fatalf("unexpected empty movement snapshot: %+v", &snapshot)
	}
}
