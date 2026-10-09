package royale

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/system"

	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/proto"
)

func TestSkillStateBroadcastUsesDetection(t *testing.T) {
	players := make(map[string]*entity.Player)
	presences := make(map[string]runtime.Presence)
	for _, id := range []string{"a", "b", "c"} {
		player := entity.NewPlayer(id, id, "", entity.Vector2{}, rand.New(rand.NewSource(1)))
		for _, weapon := range entity.DefaultWeaponCatalog() {
			if weapon.ID == "swordman" {
				player.Characters[0].ApplyWeapon(weapon)
			}
		}
		players[id] = player
		presences[id] = testPresence{userID: id, sessionID: id}
	}
	state := &State{Players: players, Presences: presences}
	dispatcher := &testDispatcher{}
	visible := map[string][]*entity.Player{"a": {players["b"]}, "b": {players["a"]}}
	state.broadcastSkillStates(nil, dispatcher, 1, visible)
	if len(dispatcher.broadcasts) != 3 {
		t.Fatalf("expected one state batch per observer, got %d", len(dispatcher.broadcasts))
	}
	for _, broadcast := range dispatcher.broadcasts {
		if broadcast.opCode != system.OpSkillStateBatch || !broadcast.reliable || len(broadcast.presences) != 1 {
			t.Fatalf("wrong skill state broadcast: %+v", broadcast)
		}
		var batch system.SkillStateBatch
		if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
			t.Fatal(err)
		}
		want := 2
		if broadcast.presences[0].GetSessionId() == "c" {
			want = 1
		}
		if len(batch.States) != want {
			t.Fatalf("observer %s received %d states, expected %d", broadcast.presences[0].GetSessionId(), len(batch.States), want)
		}
	}
	dispatcher.broadcasts = nil
	state.broadcastSkillStates(nil, dispatcher, 2, visible)
	if len(dispatcher.broadcasts) != 0 {
		t.Fatal("unchanged state was resent")
	}
	players["b"].Characters[0].AdvanceCooldowns(entity.CooldownTicks, "")
	state.broadcastSkillStates(nil, dispatcher, 3, visible)
	if len(dispatcher.broadcasts) != 2 {
		t.Fatalf("expected changed b state for a and b only, got %d", len(dispatcher.broadcasts))
	}
}
