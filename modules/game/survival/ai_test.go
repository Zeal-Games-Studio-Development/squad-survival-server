package survival

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"squad-survival-be/modules/game/core/ai"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/system"
	"squad-survival-be/modules/game/core/world"
	"squad-survival-be/modules/game/matchregistry"

	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/proto"
)

func newAITestMatch(t *testing.T) (*Match, *State) {
	t.Helper()
	match := &Match{registry: matchregistry.New()}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_MATCH_ID, "ai-test-match")
	raw, _, _ := match.MatchInit(ctx, aiTestLogger{}, nil, nil, nil)
	return match, raw.(*State)
}

func TestAIInitUsesConfiguredPopulation(t *testing.T) {
	match, state := newAITestMatch(t)
	want := ai.DefaultConfig().SurvivalCount
	if len(state.AIControllers) != want || len(state.Players) != want || state.humanPlayerCount() != 0 {
		t.Fatalf("unexpected initial counts: bots=%d entities=%d humans=%d", len(state.AIControllers), len(state.Players), state.humanPlayerCount())
	}
	names := map[string]bool{}
	for sessionID, controller := range state.AIControllers {
		player := state.Players[sessionID]
		if player == nil || controller.Player != player || !strings.HasPrefix(player.UserID, "ai-user:ai-test-match:") || !strings.HasPrefix(sessionID, "ai-session:ai-test-match:") {
			t.Fatalf("invalid bot identity: %s", sessionID)
		}
		if names[player.DisplayName] || player.DisplayName == "" {
			t.Fatalf("invalid bot name: %q", player.DisplayName)
		}
		names[player.DisplayName] = true
		if math.Hypot(player.Position.X, player.Position.Y) > world.SpawnRadius {
			t.Fatal("bot spawned outside spawn area")
		}
		if len(player.Characters) != 1 || player.Level != 1 || player.Experience != 0 {
			t.Fatal("bot did not use default player data")
		}
		if player.Characters[0].ID != player.UserID+":1" {
			t.Fatal("character identity does not belong to bot")
		}
		found := false
		for _, visible := range state.SpatialGrid.QueryPlayers(&entity.Player{SessionID: "probe", Position: player.Position, DetectionRadius: 0.001}) {
			if visible == player {
				found = true
			}
		}
		if !found {
			t.Fatal("bot missing from spatial grid")
		}
		if _, exists := match.registry.MatchForUser(player.UserID); exists {
			t.Fatal("bot registered as a Nakama member")
		}
	}
	if len(state.Presences) != 0 || len(state.Reservations) != 0 {
		t.Fatal("bot created human session state")
	}
}

func TestAILeavesAllHumanSlotsAvailable(t *testing.T) {
	match, state := newAITestMatch(t)
	dispatcher := &testDispatcher{}
	for i := 0; i < MaxPlayers; i++ {
		presence := testPresence{userID: fmt.Sprintf("user-%d", i), sessionID: fmt.Sprintf("session-%d", i)}
		if _, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, dispatcher, 1, state, presence, nil); !allowed {
			t.Fatalf("human %d rejected: %s", i, reason)
		}
		match.MatchJoin(nil, nil, nil, nil, dispatcher, 1, state, []runtime.Presence{presence})
		if i == WaitingPlayerLimit-1 {
			match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, 1, state, nil)
		}
	}
	if state.humanPlayerCount() != MaxPlayers || len(state.Players) != MaxPlayers+len(state.AIControllers) {
		t.Fatal("bots consumed human capacity")
	}
	overflow := testPresence{userID: "overflow", sessionID: "overflow"}
	if _, allowed, _ := match.MatchJoinAttempt(nil, nil, nil, nil, dispatcher, 2, state, overflow, nil); allowed {
		t.Fatal("33rd human accepted")
	}
	var label Label
	if err := json.Unmarshal([]byte(state.label()), &label); err != nil {
		t.Fatal(err)
	}
	if label.PlayerCount != MaxPlayers || label.Joinable {
		t.Fatalf("bad capacity label: %+v", label)
	}
	dispatcher.broadcasts = nil
	state.broadcastSnapshot(nil, dispatcher, 2)
	var snapshot system.StateSnapshot
	if err := proto.Unmarshal(dispatcher.broadcasts[0].data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.PlayerCount != MaxPlayers {
		t.Fatalf("snapshot counted bots: %d", snapshot.PlayerCount)
	}
}

func TestAIVisibleToHumansWithoutNetworkObservers(t *testing.T) {
	match, state := newAITestMatch(t)
	dispatcher := &testDispatcher{}
	presence := testPresence{userID: "human", sessionID: "human-session"}
	match.MatchJoin(nil, nil, nil, nil, dispatcher, 1, state, []runtime.Presence{presence})
	human := state.Players[presence.sessionID]
	var bot *entity.Player
	for _, controller := range state.AIControllers {
		bot = controller.Player
		break
	}
	bot.Position = entity.Vector2{X: human.Position.X + 1, Y: human.Position.Y}
	if err := state.SpatialGrid.Move(bot); err != nil {
		t.Fatal(err)
	}
	dispatcher.broadcasts = nil
	nearby := state.queryNearbyPlayers()
	state.broadcastRosterUpdates(nil, dispatcher, 2, nearby)
	state.broadcastPlayerMovementSnapshots(nil, dispatcher, 2, nearby)
	state.broadcastExperiencePackageEvents(nil, dispatcher, 2, nil)
	rosterFound, movementFound := false, false
	for _, broadcast := range dispatcher.broadcasts {
		if len(broadcast.presences) != 1 || broadcast.presences[0].GetSessionId() != human.SessionID {
			t.Fatal("sent data to a bot observer")
		}
		switch broadcast.opCode {
		case system.OpPlayerRosterBatch:
			var batch system.PlayerRosterBatch
			if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
				t.Fatal(err)
			}
			for _, item := range batch.Players {
				if item.SessionId == bot.SessionID && item.UserId == bot.UserID && item.DisplayName == bot.DisplayName {
					rosterFound = true
				}
			}
		case system.OpPlayerMovementSnapshot:
			var snapshot system.PlayerMovementSnapshot
			if err := proto.Unmarshal(broadcast.data, &snapshot); err != nil {
				t.Fatal(err)
			}
			for _, item := range snapshot.Players {
				if item.SessionId == bot.SessionID {
					movementFound = true
				}
			}
		}
	}
	if !rosterFound || !movementFound {
		t.Fatalf("bot missing from replication: roster=%v movement=%v", rosterFound, movementFound)
	}
	for sessionID := range state.AIControllers {
		if state.Experience.Known[sessionID] != nil || state.RosterVersions[sessionID] != nil || state.ProgressionVersions[sessionID] != nil {
			t.Fatal("bot allocated observer tracking")
		}
	}
}

func TestAIOnlyWaitingMatchExpiresWithoutMovingBots(t *testing.T) {
	match, state := newAITestMatch(t)
	state.EmptyTicks = emptyMatchTTLSeconds*tickRate - 1
	if next := match.MatchLoop(nil, aiTestLogger{}, nil, nil, &testDispatcher{}, 1, state, nil); next != nil {
		t.Fatal("AI kept an empty match alive")
	}
	for _, controller := range state.AIControllers {
		if controller.Player.Direction != (entity.Vector2{}) {
			t.Fatal("bot moved during waiting")
		}
	}
}

func TestAIUsesNormalPickupProgressionAndCombat(t *testing.T) {
	match, state := newAITestMatch(t)
	state.Phase = PhasePlaying
	state.PlayingEndsAtTick = playingDurationTicks

	var bot *entity.Player
	for _, controller := range state.AIControllers {
		bot = controller.Player
		break
	}
	for id := range state.Players {
		if id != bot.SessionID {
			state.SpatialGrid.Remove(id)
			delete(state.Players, id)
			delete(state.AIControllers, id)
		}
	}
	for id := range state.CharacterBoxes {
		state.SpatialGrid.RemoveCharacterBox(id)
		delete(state.CharacterBoxes, id)
	}
	for id := range state.Experience.Packages {
		state.SpatialGrid.RemoveExperiencePackage(id)
		delete(state.Experience.Packages, id)
	}
	bot.Position = entity.Vector2{}
	bot.Level = 2
	if err := state.SpatialGrid.Move(bot); err != nil {
		t.Fatal(err)
	}
	box := entity.NewCharacterBox("ai-box", bot.Position, entity.WeaponBow)
	state.CharacterBoxes[box.ID] = box
	if err := state.SpatialGrid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	item := entity.NewExperiencePackage("ai-xp", bot.Position, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 1)
	state.Experience.Packages[item.ID] = item
	if err := state.SpatialGrid.InsertExperiencePackage(item); err != nil {
		t.Fatal(err)
	}
	dispatcher := &testDispatcher{}
	for tick := int64(1); tick <= 20; tick++ {
		match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, tick, state, nil)
	}
	if state.CharacterBoxes[box.ID] != nil || bot.CharacterCount() != 2 {
		t.Fatal("AI failed to finish timed character pickup")
	}
	if state.Experience.Packages[item.ID] != nil || bot.Experience != 1 {
		t.Fatal("AI failed to collect normal experience")
	}
	opponent := entity.NewPlayer("ai-user:opponent:1", "ai-session:opponent:1", "Opponent", bot.Position, state.random)
	opponent.Level = bot.Level
	// Equal rosters make both bots choose combat under the strength policy.
	for opponent.CharacterCount() < bot.CharacterCount() {
		if err := opponent.AddCharacter(entity.CreateCharacter(state.random, state.WeaponCatalog)); err != nil {
			t.Fatal(err)
		}
	}
	state.Players[opponent.SessionID] = opponent
	state.AIControllers[opponent.SessionID] = ai.NewController(opponent)
	if err := state.SpatialGrid.Insert(opponent); err != nil {
		t.Fatal(err)
	}
	initialHealth := make(map[*entity.Character]float64, len(opponent.Characters))
	for _, character := range opponent.Characters {
		initialHealth[character] = character.Health
	}
	for tick := int64(21); tick <= 50; tick++ {
		match.MatchLoop(nil, aiTestLogger{}, nil, nil, dispatcher, tick, state, nil)
		for character, health := range initialHealth {
			if character.Health < health {
				return
			}
		}
	}
	t.Fatal("AI did not participate in automatic combat")
}

type aiTestLogger struct{}

func (aiTestLogger) Debug(string, ...interface{})                       {}
func (aiTestLogger) Info(string, ...interface{})                        {}
func (aiTestLogger) Warn(string, ...interface{})                        {}
func (aiTestLogger) Error(string, ...interface{})                       {}
func (l aiTestLogger) WithField(string, interface{}) runtime.Logger     { return l }
func (l aiTestLogger) WithFields(map[string]interface{}) runtime.Logger { return l }
func (aiTestLogger) Fields() map[string]interface{}                     { return nil }
