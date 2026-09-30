package royale

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/system"
	"squad-survival-be/modules/game/matchregistry"

	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/proto"
)

func TestMatchInitStartsWaiting(t *testing.T) {
	match := &Match{registry: matchregistry.New()}
	rawState, rate, label := match.MatchInit(context.Background(), testLogger{}, nil, nil, map[string]interface{}{})
	state := rawState.(*State)

	if rate != tickRate || state.Phase != PhaseWaiting || state.WaitingEndsAtTick != 0 {
		t.Fatalf("unexpected initial lifecycle: rate=%d state=%+v", rate, state)
	}
	want := `{"mode":"battle-royale","status":"waiting","player_count":0,"max_players":32,"joinable":true}`
	if label != want {
		t.Fatalf("unexpected initial label: got %s want %s", label, want)
	}
}

func TestWaitingStartsOnFirstJoinAndPausesGameplay(t *testing.T) {
	match, state := newTestMatchState(t)
	dispatcher := &testDispatcher{}
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	match.MatchJoin(context.Background(), nil, nil, nil, dispatcher, 10, state, []runtime.Presence{presence})

	if state.WaitingEndsAtTick != 10+waitingDurationTicks {
		t.Fatalf("unexpected waiting deadline: %d", state.WaitingEndsAtTick)
	}
	assertLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_WAITING, 10, state.WaitingEndsAtTick, true)

	player := state.Players[presence.sessionID]
	box := entity.NewCharacterBox("waiting-box", player.Position, entity.WeaponBow)
	state.CharacterBoxes[box.ID] = box
	if err := state.SpatialGrid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	match.MatchLoop(nil, nil, nil, nil, dispatcher, state.WaitingEndsAtTick-1, state, nil)
	if len(player.Characters) != 1 || state.CharacterBoxes[box.ID] == nil {
		t.Fatal("waiting phase ran character-box gameplay")
	}

	dispatcher.broadcasts = nil
	match.MatchLoop(nil, nil, nil, nil, dispatcher, state.WaitingEndsAtTick, state, nil)
	if state.Phase != PhasePlaying || state.PlayingEndsAtTick != state.WaitingEndsAtTick+playingDurationTicks {
		t.Fatalf("match did not start on deadline: %+v", state)
	}
	wantPlayingLabel := `{"mode":"battle-royale","status":"playing","player_count":1,"max_players":32,"joinable":false}`
	if dispatcher.label != wantPlayingLabel {
		t.Fatalf("unexpected playing label: got %s want %s", dispatcher.label, wantPlayingLabel)
	}
	assertLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_PLAYING, state.WaitingEndsAtTick, state.PlayingEndsAtTick, false)
}

func TestWaitingStartsImmediatelyWhenFull(t *testing.T) {
	match, state := newTestMatchState(t)
	state.WaitingEndsAtTick = waitingDurationTicks
	for i := 0; i < MaxPlayers; i++ {
		player := entity.NewPlayer(fmt.Sprintf("user-%d", i), fmt.Sprintf("session-%d", i), "", entity.Vector2{X: float64(i)}, rand.New(rand.NewSource(int64(i+1))))
		state.Players[player.SessionID] = player
		if err := state.SpatialGrid.Insert(player); err != nil {
			t.Fatal(err)
		}
	}
	match.MatchLoop(nil, nil, nil, nil, &testDispatcher{}, 1, state, nil)
	if state.Phase != PhasePlaying {
		t.Fatalf("full lobby remained in phase %q", state.Phase)
	}
}

func TestJoinIsAllowedOnlyWhileWaiting(t *testing.T) {
	match, state := newTestMatchState(t)
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	if _, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 1, state, presence, nil); !allowed || reason != "" {
		t.Fatalf("waiting join rejected: allowed=%v reason=%q", allowed, reason)
	}
	delete(state.Reservations, presence.sessionID)

	for _, phase := range []Phase{PhasePlaying, PhaseEnded} {
		state.Phase = phase
		if _, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 2, state, presence, nil); allowed || reason != "match already started" {
			t.Fatalf("join accepted in %s: allowed=%v reason=%q", phase, allowed, reason)
		}
	}
}

func TestEmptyStartedLobbyTerminates(t *testing.T) {
	match, state := newTestMatchState(t)
	state.WaitingEndsAtTick = waitingDurationTicks
	if result := match.MatchLoop(nil, nil, nil, nil, &testDispatcher{}, 1, state, nil); result != nil {
		t.Fatal("empty started lobby did not terminate")
	}
}

func TestPlayingEndsAfterTenMinutesAndTerminatesAfterGracePeriod(t *testing.T) {
	match, state := newTestMatchState(t)
	dispatcher := &testDispatcher{}
	state.Phase = PhasePlaying
	state.PlayingEndsAtTick = playingDurationTicks

	result := match.MatchLoop(nil, nil, nil, nil, dispatcher, playingDurationTicks, state, nil)
	if result == nil || state.Phase != PhaseEnded || state.EndedAtTick != playingDurationTicks+endedDurationTicks {
		t.Fatalf("unexpected ended transition: result=%v state=%+v", result, state)
	}
	wantEndedLabel := `{"mode":"battle-royale","status":"ended","player_count":0,"max_players":32,"joinable":false}`
	if dispatcher.label != wantEndedLabel {
		t.Fatalf("unexpected ended label: got %s want %s", dispatcher.label, wantEndedLabel)
	}
	assertLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_ENDED, playingDurationTicks, state.EndedAtTick, false)
	if result = match.MatchLoop(nil, nil, nil, nil, dispatcher, state.EndedAtTick-1, state, nil); result == nil {
		t.Fatal("ended match terminated before grace period")
	}
	if result = match.MatchLoop(nil, nil, nil, nil, dispatcher, state.EndedAtTick, state, nil); result != nil {
		t.Fatal("ended match did not terminate after grace period")
	}
}

func newTestMatchState(t *testing.T) (*Match, *State) {
	t.Helper()
	match := &Match{registry: matchregistry.New()}
	rawState, _, _ := match.MatchInit(context.Background(), testLogger{}, nil, nil, map[string]interface{}{})
	return match, rawState.(*State)
}

func assertLifecycle(t *testing.T, dispatcher *testDispatcher, phase system.MatchPhase, serverTick, endsAt int64, targeted bool) {
	t.Helper()
	for i := len(dispatcher.broadcasts) - 1; i >= 0; i-- {
		broadcast := dispatcher.broadcasts[i]
		if broadcast.opCode != system.OpMatchLifecycleState {
			continue
		}
		var lifecycle system.MatchLifecycleState
		if err := proto.Unmarshal(broadcast.data, &lifecycle); err != nil {
			t.Fatal(err)
		}
		if lifecycle.Phase != phase || lifecycle.ServerTick != serverTick || lifecycle.PhaseEndsAtTick != endsAt || lifecycle.TickRate != int32(tickRate) || !broadcast.reliable {
			t.Fatalf("unexpected lifecycle: %+v broadcast=%+v", &lifecycle, broadcast)
		}
		if targeted != (len(broadcast.presences) > 0) {
			t.Fatalf("unexpected lifecycle targeting: %+v", broadcast.presences)
		}
		return
	}
	t.Fatal("lifecycle broadcast not found")
}

type testPresence struct{ userID, sessionID string }

func (p testPresence) GetHidden() bool                   { return false }
func (p testPresence) GetPersistence() bool              { return false }
func (p testPresence) GetUsername() string               { return p.userID }
func (p testPresence) GetStatus() string                 { return "" }
func (p testPresence) GetReason() runtime.PresenceReason { return runtime.PresenceReasonUnknown }
func (p testPresence) GetUserId() string                 { return p.userID }
func (p testPresence) GetSessionId() string              { return p.sessionID }
func (p testPresence) GetNodeId() string                 { return "node-1" }

type testBroadcast struct {
	opCode    int64
	data      []byte
	presences []runtime.Presence
	reliable  bool
}

type testDispatcher struct {
	label      string
	broadcasts []testBroadcast
}

func (d *testDispatcher) BroadcastMessage(opCode int64, data []byte, presences []runtime.Presence, _ runtime.Presence, reliable bool) error {
	d.broadcasts = append(d.broadcasts, testBroadcast{opCode: opCode, data: append([]byte(nil), data...), presences: append([]runtime.Presence(nil), presences...), reliable: reliable})
	return nil
}
func (*testDispatcher) BroadcastMessageDeferred(int64, []byte, []runtime.Presence, runtime.Presence, bool) error {
	return nil
}
func (*testDispatcher) MatchKick([]runtime.Presence) error { return nil }
func (d *testDispatcher) MatchLabelUpdate(label string) error {
	d.label = label
	return nil
}

type testLogger struct{}

func (testLogger) Debug(string, ...interface{})                       {}
func (testLogger) Info(string, ...interface{})                        {}
func (testLogger) Warn(string, ...interface{})                        {}
func (testLogger) Error(string, ...interface{})                       {}
func (l testLogger) WithField(string, interface{}) runtime.Logger     { return l }
func (l testLogger) WithFields(map[string]interface{}) runtime.Logger { return l }
func (testLogger) Fields() map[string]interface{}                     { return nil }
