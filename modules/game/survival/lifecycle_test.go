package survival

import (
	"fmt"
	"testing"

	"squad-survival-be/modules/game/core/characterbox"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/system"

	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/proto"
)

func TestSurvivalWaitingStartsOnFirstJoinAndTimesOut(t *testing.T) {
	match, state := newAITestMatch(t)
	if state.Phase != PhaseWaiting || state.WaitingEndsAtTick != 0 || state.label() != `{"mode":"survival","status":"waiting","player_count":0,"max_players":32,"available_slots":3,"joinable":true}` {
		t.Fatalf("unexpected initial lifecycle: %+v label=%s", state, state.label())
	}
	dispatcher := &testDispatcher{}
	match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, 9, state, nil)
	if state.WaitingEndsAtTick != 0 {
		t.Fatal("waiting deadline started before first join")
	}
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	match.MatchJoin(nil, nil, nil, nil, dispatcher, 10, state, []runtime.Presence{presence})
	if state.WaitingEndsAtTick != 10+waitingDurationTicks {
		t.Fatalf("wrong waiting deadline: %d", state.WaitingEndsAtTick)
	}
	assertSurvivalLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_WAITING, 10, state.WaitingEndsAtTick, true)
	player := state.Players[presence.sessionID]
	position := player.Position
	message := testMatchData{testPresence: presence, opCode: system.OpMovementInput, data: mustMarshalMovementInput(t, &entity.MovementInput{X: 1, Sequence: 1})}
	match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, state.WaitingEndsAtTick-1, state, []runtime.MatchData{message})
	if state.Phase != PhaseWaiting || player.Position != position {
		t.Fatal("gameplay ran during waiting")
	}
	startTick := state.WaitingEndsAtTick
	match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, startTick, state, nil)
	if state.Phase != PhasePlaying || state.PlayingEndsAtTick != startTick+playingDurationTicks || state.NextBoxRefillTick != startTick+characterBoxRefillTicks {
		t.Fatalf("wrong playing transition: phase=%s deadline=%d refill=%d", state.Phase, state.PlayingEndsAtTick, state.NextBoxRefillTick)
	}
	assertSurvivalLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_PLAYING, startTick, state.PlayingEndsAtTick, false)
	late := testPresence{userID: "late", sessionID: "late"}
	if _, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, startTick+1, state, late, nil); !allowed {
		t.Fatalf("playing join rejected: %s", reason)
	}
	match.MatchJoin(nil, nil, nil, nil, dispatcher, startTick+1, state, []runtime.Presence{late})
	assertSurvivalLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_PLAYING, startTick+1, state.PlayingEndsAtTick, true)
}

func TestSurvivalWaitingCapacityAndThreePlayerStart(t *testing.T) {
	match, state := newAITestMatch(t)
	dispatcher := &testDispatcher{}
	for index := 1; index <= WaitingPlayerLimit; index++ {
		presence := testPresence{userID: fmt.Sprintf("user-%d", index), sessionID: fmt.Sprintf("session-%d", index)}
		if _, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 10, state, presence, nil); !allowed {
			t.Fatalf("waiting reservation %d rejected: %s", index, reason)
		}
	}
	fourth := testPresence{userID: "user-4", sessionID: "session-4"}
	if _, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 11, state, fourth, nil); allowed || reason != "match is full" || state.label() != `{"mode":"survival","status":"waiting","player_count":0,"max_players":32,"available_slots":0,"joinable":false}` {
		t.Fatalf("waiting capacity ignored reservations: allowed=%v reason=%s label=%s", allowed, reason, state.label())
	}
	for index := 1; index <= WaitingPlayerLimit; index++ {
		presence := testPresence{userID: fmt.Sprintf("user-%d", index), sessionID: fmt.Sprintf("session-%d", index)}
		match.MatchJoin(nil, nil, nil, nil, dispatcher, 12, state, []runtime.Presence{presence})
	}
	if state.Phase != PhasePlaying || state.PlayingEndsAtTick != 12+playingDurationTicks || state.label() != `{"mode":"survival","status":"playing","player_count":3,"max_players":32,"available_slots":29,"joinable":true}` {
		t.Fatalf("three players did not start playing: phase=%s label=%s", state.Phase, state.label())
	}
	assertSurvivalLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_PLAYING, 12, state.PlayingEndsAtTick, false)
	if _, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 13, state, fourth, nil); !allowed {
		t.Fatalf("fourth player was rejected during playing: %s", reason)
	}
}

func TestSurvivalEndedStopsGameplayAndTerminatesAfterGracePeriod(t *testing.T) {
	match, state := newAITestMatch(t)
	dispatcher := &testDispatcher{}
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	match.MatchJoin(nil, nil, nil, nil, dispatcher, 1, state, []runtime.Presence{presence})
	state.startPlaying(nil, dispatcher, 2)
	player := state.Players[presence.sessionID]
	position := player.Position
	boxID := "box:claim"
	state.BoxClaims[boxID] = characterbox.Claim{BoxID: boxID, SessionID: presence.sessionID, StartedAtTick: 3, CompletesAtTick: state.PlayingEndsAtTick + 1}
	state.ClaimedBoxBySession[presence.sessionID] = boxID
	endTick := state.PlayingEndsAtTick
	match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, endTick, state, nil)
	if state.Phase != PhaseEnded || state.EndedAtTick != endTick+endedDurationTicks || len(state.BoxClaims) != 0 || len(state.ClaimedBoxBySession) != 0 {
		t.Fatalf("wrong ended transition: phase=%s deadline=%d claims=%v", state.Phase, state.EndedAtTick, state.BoxClaims)
	}
	if state.label() != `{"mode":"survival","status":"ended","player_count":1,"max_players":32,"available_slots":0,"joinable":false}` {
		t.Fatalf("ended label is joinable: %s", state.label())
	}
	assertSurvivalLifecycle(t, dispatcher, system.MatchPhase_MATCH_PHASE_ENDED, endTick, state.EndedAtTick, false)
	if !hasSurvivalBoxEvent(t, dispatcher, system.CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_CANCELLED) {
		t.Fatal("active box claim was not cancelled")
	}
	late := testPresence{userID: "late", sessionID: "late"}
	if _, allowed, _ := match.MatchJoinAttempt(nil, nil, nil, nil, nil, endTick+1, state, late, nil); allowed {
		t.Fatal("ended match accepted a new join")
	}
	match.MatchJoin(nil, nil, nil, nil, dispatcher, endTick+1, state, []runtime.Presence{late})
	if state.Players[late.sessionID] != nil {
		t.Fatal("ended match created a new player")
	}
	message := testMatchData{testPresence: presence, opCode: system.OpMovementInput, data: mustMarshalMovementInput(t, &entity.MovementInput{X: 1, Sequence: 1})}
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, state.EndedAtTick-1, state, []runtime.MatchData{message}); result == nil || player.Position != position {
		t.Fatal("ended match stopped early or ran gameplay")
	}
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, state.EndedAtTick, state, nil); result != nil {
		t.Fatal("ended match did not stop after one minute")
	}
}

func TestSurvivalWaitingAndPlayingEmptyMatchPolicy(t *testing.T) {
	match, state := newAITestMatch(t)
	state.EmptyTicks = waitingEmptyTTLSeconds*tickRate - 1
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 1, state, nil); result != nil {
		t.Fatal("unused waiting match survived empty TTL")
	}
	match, state = newAITestMatch(t)
	state.WaitingEndsAtTick = waitingDurationTicks
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 1, state, nil); result != nil {
		t.Fatal("empty started waiting match did not stop")
	}
	match, state = newAITestMatch(t)
	state.Phase = PhasePlaying
	state.PlayingEndsAtTick = playingDurationTicks
	state.EmptyTicks = playingEmptyTTLSeconds*tickRate - 2
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 1, state, nil); result == nil || state.EmptyTicks != playingEmptyTTLSeconds*tickRate-1 {
		t.Fatal("empty playing match stopped before 15 seconds")
	}
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 2, state, nil); result != nil {
		t.Fatal("empty playing match survived 15 seconds")
	}

	match, state = newAITestMatch(t)
	state.Phase = PhasePlaying
	state.PlayingEndsAtTick = playingDurationTicks
	state.EmptyTicks = playingEmptyTTLSeconds*tickRate - 1
	state.Reservations["pending"] = 100
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 1, state, nil); result == nil || state.EmptyTicks != 0 {
		t.Fatal("reservation did not reset empty playing TTL")
	}
	delete(state.Reservations, "pending")
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 2, state, nil); result == nil || state.EmptyTicks != 1 {
		t.Fatal("empty playing TTL did not restart after reservation disappeared")
	}

	state.EmptyTicks = playingEmptyTTLSeconds*tickRate - 1
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	match.MatchJoin(nil, nil, nil, nil, &testDispatcher{}, 3, state, []runtime.Presence{presence})
	if state.EmptyTicks != 0 {
		t.Fatal("joining player did not reset empty playing TTL")
	}
	if result := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 3, state, nil); result == nil || state.EmptyTicks != 0 {
		t.Fatal("playing match with a player stopped as empty")
	}
}

func assertSurvivalLifecycle(t *testing.T, dispatcher *testDispatcher, phase system.MatchPhase, serverTick, deadline int64, targeted bool) {
	t.Helper()
	for index := len(dispatcher.broadcasts) - 1; index >= 0; index-- {
		broadcast := dispatcher.broadcasts[index]
		if broadcast.opCode != system.OpMatchLifecycleState {
			continue
		}
		var state system.MatchLifecycleState
		if err := proto.Unmarshal(broadcast.data, &state); err != nil {
			t.Fatal(err)
		}
		if state.Phase != phase || state.ServerTick != serverTick || state.PhaseEndsAtTick != deadline || state.TickRate != int32(tickRate) || !broadcast.reliable || (len(broadcast.presences) > 0) != targeted {
			t.Fatalf("unexpected lifecycle state: %+v broadcast=%+v", &state, broadcast)
		}
		return
	}
	t.Fatal("lifecycle state was not broadcast")
}

func hasSurvivalBoxEvent(t *testing.T, dispatcher *testDispatcher, eventType system.CharacterBoxEventType) bool {
	t.Helper()
	for _, broadcast := range dispatcher.broadcasts {
		if broadcast.opCode != system.OpCharacterBoxState {
			continue
		}
		var batch system.CharacterBoxStateBatch
		if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
			t.Fatal(err)
		}
		for _, event := range batch.Events {
			if event.EventType == eventType {
				return true
			}
		}
	}
	return false
}
