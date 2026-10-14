package royale

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/spatial"
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
	dispatcher.broadcasts = nil
	character := players["b"].Characters[0]
	character.ReadySkillIDs = []string{"sword_cone"}
	character.SkillStateVersion++
	state.broadcastSkillStates(nil, dispatcher, 4, visible)
	if len(dispatcher.broadcasts) != 2 {
		t.Fatalf("ready state did not reach owner and observer: %d", len(dispatcher.broadcasts))
	}
	for _, broadcast := range dispatcher.broadcasts {
		var batch system.SkillStateBatch
		if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
			t.Fatal(err)
		}
		if len(batch.States) != 1 || len(batch.States[0].ReadySkillIds) != 1 || batch.States[0].ReadySkillIds[0] != "sword_cone" {
			t.Fatalf("wrong ready payload: %+v", &batch)
		}
	}
	dispatcher.broadcasts = nil
	character.ReadySkillIDs = nil
	character.SkillStateVersion++
	state.broadcastSkillStates(nil, dispatcher, 5, visible)
	if len(dispatcher.broadcasts) != 2 {
		t.Fatalf("empty ready state did not reach owner and observer: %d", len(dispatcher.broadcasts))
	}
	for _, broadcast := range dispatcher.broadcasts {
		var batch system.SkillStateBatch
		if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
			t.Fatal(err)
		}
		if len(batch.States) != 1 || len(batch.States[0].ReadySkillIds) != 0 {
			t.Fatalf("ready list was not cleared: %+v", &batch)
		}
	}
}

type skillTestMatchData struct {
	testPresence
	opCode int64
	data   []byte
}

func (m skillTestMatchData) GetOpCode() int64      { return m.opCode }
func (m skillTestMatchData) GetData() []byte       { return m.data }
func (m skillTestMatchData) GetReliable() bool     { return true }
func (m skillTestMatchData) GetReceiveTime() int64 { return 0 }

func TestMatchLoopActivateSkillInput(t *testing.T) {
	weapon := entity.DefaultWeaponCatalog()[0]
	for _, candidate := range entity.DefaultWeaponCatalog() {
		if candidate.ID == "swordman" {
			weapon = candidate
			break
		}
	}
	a := entity.NewPlayerWithWeapon("a", "a", "", entity.Vector2{}, weapon)
	b := entity.NewPlayerWithWeapon("b", "b", "", entity.Vector2{X: 1}, weapon)
	a.Characters[0].AttackRange = 10
	b.Characters[0].AttackRange = 10
	a.Characters[0].Cooldowns["sword_cone"].Progress = 30
	grid := spatial.NewGrid(spatialCellSize)
	for _, player := range []*entity.Player{a, b} {
		if err := grid.Insert(player); err != nil {
			t.Fatal(err)
		}
	}
	state := &State{Phase: PhasePlaying, PlayingEndsAtTick: 100, NextBoxRefillTick: 100, Players: map[string]*entity.Player{"a": a, "b": b}, Presences: map[string]runtime.Presence{"a": testPresence{userID: "a", sessionID: "a"}, "b": testPresence{userID: "b", sessionID: "b"}}, SpatialGrid: grid, random: rand.New(rand.NewSource(1))}
	dispatcher := &testDispatcher{}
	match := &Match{}
	match.MatchLoop(nil, nil, nil, nil, dispatcher, 1, state, nil)
	if len(a.Characters[0].ReadySkillIDs) != 1 || a.Characters[0].ActiveSkillID != "" {
		t.Fatalf("human skill auto started or did not become ready: %+v", a.Characters[0])
	}
	data, err := proto.Marshal(&entity.ActivateSkillInput{CharacterId: a.Characters[0].ID, SkillId: "sword_cone"})
	if err != nil {
		t.Fatal(err)
	}
	messages := []runtime.MatchData{
		skillTestMatchData{testPresence: testPresence{userID: "b", sessionID: "b"}, opCode: system.OpActivateSkillInput, data: data},
		skillTestMatchData{testPresence: testPresence{userID: "a", sessionID: "a"}, opCode: system.OpActivateSkillInput, data: data},
	}
	match.MatchLoop(nil, nil, nil, nil, dispatcher, 2, state, messages)
	if a.Characters[0].ActiveSkillID != "sword_cone" || len(a.Characters[0].ReadySkillIDs) != 0 {
		t.Fatalf("owner input did not activate skill: %+v", a.Characters[0])
	}
}
