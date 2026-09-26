package survival

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/spatial"
	"squad-survival-be/modules/game/core/system"
	"squad-survival-be/modules/game/matchregistry"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/proto"
)

func TestJoinAttemptReservesAndExpiresSlot(t *testing.T) {
	match := &Match{}
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players:             make(map[string]*entity.Player),
		Reservations:        make(map[string]int64),
	}
	for i := 0; i < MaxPlayers; i++ {
		presence := testPresence{userID: fmt.Sprintf("user-%d", i), sessionID: fmt.Sprintf("session-%d", i)}
		_, allowed, _ := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 0, state, presence, nil)
		if !allowed {
			t.Fatalf("expected player %d to reserve a slot", i+1)
		}
	}

	second := testPresence{userID: "overflow-user", sessionID: "overflow-session"}
	_, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 1, state, second, nil)
	if allowed || reason != "match is full" {
		t.Fatalf("expected full match rejection, got allowed=%v reason=%q", allowed, reason)
	}

	_, allowed, _ = match.MatchJoinAttempt(nil, nil, nil, nil, nil, reservationTTLSeconds*tickRate, state, second, nil)
	if !allowed {
		t.Fatal("expected expired reservation to release the slot")
	}
}

func TestJoinAndLeaveUpdateCapacityLabel(t *testing.T) {
	match := &Match{}
	dispatcher := &testDispatcher{}
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players:             make(map[string]*entity.Player),
		Presences:           make(map[string]runtime.Presence),
		Reservations:        map[string]int64{presence.sessionID: 100},
		SpatialGrid:         spatial.NewGrid(spatialCellSize),
		random:              rand.New(rand.NewSource(1)),
	}

	match.MatchJoin(nil, nil, nil, nil, dispatcher, 0, state, []runtime.Presence{presence})
	if state.Players[presence.sessionID].DisplayName != presence.userID {
		t.Fatalf("expected username fallback, got %q", state.Players[presence.sessionID].DisplayName)
	}
	if dispatcher.label != `{"mode":"survival","status":"playing","player_count":1,"max_players":32,"joinable":true}` {
		t.Fatalf("unexpected full label: %s", dispatcher.label)
	}
	assertStateSnapshot(t, dispatcher, 0, 1)
	if state.Presences[presence.sessionID] == nil {
		t.Fatal("expected joined player presence to be tracked")
	}
	if err := state.SpatialGrid.Insert(state.Players[presence.sessionID]); err == nil {
		t.Fatal("expected joined player to already exist in spatial grid")
	}

	match.MatchLeave(nil, nil, nil, nil, dispatcher, 2, state, []runtime.Presence{presence})
	if dispatcher.label != `{"mode":"survival","status":"playing","player_count":0,"max_players":32,"joinable":true}` {
		t.Fatalf("unexpected empty label: %s", dispatcher.label)
	}
	assertStateSnapshot(t, dispatcher, 2, 0)
	if _, exists := state.Presences[presence.sessionID]; exists {
		t.Fatal("expected leaving player presence to be removed")
	}
	if state.SpatialGrid.Remove(presence.sessionID) {
		t.Fatal("expected leaving player to be absent from spatial grid")
	}
	stateSnapshots, rosters := 0, 0
	for _, broadcast := range dispatcher.broadcasts {
		switch broadcast.opCode {
		case system.OpStateSnapshot:
			stateSnapshots++
		case system.OpPlayerRosterBatch:
			rosters++
		}
	}
	if stateSnapshots != 2 || rosters != 1 {
		t.Fatalf("unexpected join/leave broadcasts: state=%d roster=%d", stateSnapshots, rosters)
	}
}

func TestJoinAndLeaveUpdateActiveMatchRegistry(t *testing.T) {
	registry := matchregistry.New()
	match := &Match{registry: registry}
	dispatcher := &testDispatcher{}
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	state := &State{
		MatchID:             "match-1",
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players:             make(map[string]*entity.Player),
		Presences:           make(map[string]runtime.Presence),
		Reservations:        make(map[string]int64),
		SpatialGrid:         spatial.NewGrid(spatialCellSize),
		random:              rand.New(rand.NewSource(1)),
	}

	match.MatchJoin(nil, nil, nil, nil, dispatcher, 0, state, []runtime.Presence{presence})
	if matchID, ok := registry.MatchForUser(presence.userID); !ok || matchID != state.MatchID {
		t.Fatalf("unexpected active match: %q, %v", matchID, ok)
	}

	match.MatchLeave(nil, nil, nil, nil, dispatcher, 1, state, []runtime.Presence{presence})
	if _, ok := registry.MatchForUser(presence.userID); ok {
		t.Fatal("expected leave to remove active match membership")
	}
}

func TestJoinAttemptRejectsUserActiveInAnotherMatch(t *testing.T) {
	registry := matchregistry.New()
	registry.Add("user-1", "session-1", "match-1")
	match := &Match{registry: registry}
	state := &State{
		MatchID:             "match-2",
		AllowJoinInProgress: true,
		Players:             make(map[string]*entity.Player),
		Reservations:        make(map[string]int64),
	}
	presence := testPresence{userID: "user-1", sessionID: "session-2"}

	_, allowed, reason := match.MatchJoinAttempt(nil, nil, nil, nil, nil, 0, state, presence, nil)
	if allowed || reason != "already in another match" {
		t.Fatalf("expected duplicate match rejection, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestResolveDisplayNamesUsesProfileAndUsernameFallback(t *testing.T) {
	presences := []runtime.Presence{
		testPresence{userID: "user-1", sessionID: "session-1", username: "username-1"},
		testPresence{userID: "user-2", sessionID: "session-2", username: "username-2"},
	}
	lookup := testUserLookup{users: []*api.User{
		{Id: "user-1", DisplayName: "Commander One"},
		{Id: "user-2"},
	}}

	displayNames, err := resolveDisplayNames(nil, lookup, presences)
	if err != nil {
		t.Fatal(err)
	}
	if displayNames["user-1"] != "Commander One" {
		t.Fatalf("expected profile display name, got %q", displayNames["user-1"])
	}
	if displayNames["user-2"] != "username-2" {
		t.Fatalf("expected username fallback, got %q", displayNames["user-2"])
	}
}

func TestMatchLoopIgnoresInvalidMessagesWithoutBroadcastingSnapshot(t *testing.T) {
	match := &Match{}
	dispatcher := &testDispatcher{}
	player := entity.NewPlayer("user-1", "session-1", "Player One", entity.Vector2{}, rand.New(rand.NewSource(1)))
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players:             map[string]*entity.Player{player.SessionID: player},
		Reservations:        make(map[string]int64),
		SpatialGrid:         spatial.NewGrid(spatialCellSize),
	}
	if err := state.SpatialGrid.Insert(player); err != nil {
		t.Fatal(err)
	}
	messages := []runtime.MatchData{
		testMatchData{testPresence: testPresence{userID: player.UserID, sessionID: player.SessionID}, opCode: 999, data: []byte(`{}`)},
		testMatchData{testPresence: testPresence{userID: player.UserID, sessionID: player.SessionID}, opCode: system.OpMovementInput, data: []byte{0x0a, 0x01}},
	}

	result := match.MatchLoop(nil, nil, nil, nil, dispatcher, 1, state, messages)
	if result == nil {
		t.Fatal("invalid messages must not stop the match")
	}
	if player.Position != (entity.Vector2{}) {
		t.Fatalf("invalid messages changed position: %+v", player.Position)
	}
	if dispatcher.broadcastCount != 0 {
		t.Fatalf("match loop unexpectedly broadcast %d messages", dispatcher.broadcastCount)
	}
}

func TestMatchLoopAppliesMovementWithoutBroadcastingSnapshot(t *testing.T) {
	match := &Match{}
	dispatcher := &testDispatcher{}
	player := entity.NewPlayer("user-1", "session-1", "Player One", entity.Vector2{}, rand.New(rand.NewSource(1)))
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players:             map[string]*entity.Player{player.SessionID: player},
		Reservations:        make(map[string]int64),
		SpatialGrid:         spatial.NewGrid(spatialCellSize),
	}
	if err := state.SpatialGrid.Insert(player); err != nil {
		t.Fatal(err)
	}
	message := testMatchData{
		testPresence: testPresence{userID: player.UserID, sessionID: player.SessionID},
		opCode:       system.OpMovementInput,
		data:         mustMarshalMovementInput(t, &entity.MovementInput{X: 1, Sequence: 1}),
	}

	match.MatchLoop(nil, nil, nil, nil, dispatcher, 1, state, []runtime.MatchData{message})
	expectedPosition := entity.Vector2{X: player.MinMoveSpeed() / float64(entity.TickRate)}
	if player.Position != expectedPosition {
		t.Fatalf("unexpected player position: %+v", player.Position)
	}

	if dispatcher.broadcastCount != 0 {
		t.Fatalf("movement unexpectedly broadcast %d state snapshots", dispatcher.broadcastCount)
	}
}

func TestMatchLoopRemovesDeadCharacterBeforeMovement(t *testing.T) {
	match := &Match{}
	dispatcher := &testDispatcher{}
	player := entity.NewPlayer("user-1", "session-1", "Player One", entity.Vector2{}, rand.New(rand.NewSource(1)))
	player.Characters[0].Health = 0
	grid := spatial.NewGrid(spatialCellSize)
	if err := grid.Insert(player); err != nil {
		t.Fatal(err)
	}
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players:             map[string]*entity.Player{player.SessionID: player},
		Reservations:        make(map[string]int64),
		SpatialGrid:         grid,
	}
	message := testMatchData{
		testPresence: testPresence{userID: player.UserID, sessionID: player.SessionID},
		opCode:       system.OpMovementInput,
		data:         mustMarshalMovementInput(t, &entity.MovementInput{X: 1, Sequence: 1}),
	}

	match.MatchLoop(nil, nil, nil, nil, dispatcher, 1, state, []runtime.MatchData{message})
	if len(player.Characters) != 0 {
		t.Fatalf("expected dead character removed, got %d characters", len(player.Characters))
	}
	if player.Position != (entity.Vector2{}) {
		t.Fatalf("player moved without living characters: %+v", player.Position)
	}
}

func TestMatchLoopMovesPlayerBetweenSpatialCells(t *testing.T) {
	match := &Match{}
	dispatcher := &testDispatcher{}
	random := rand.New(rand.NewSource(1))
	moving := entity.NewPlayer("moving-user", "moving", "Moving", entity.Vector2{X: 19.9}, random)
	detector := entity.NewPlayer("detector-user", "detector", "Detector", entity.Vector2{X: 30.2}, random)
	grid := spatial.NewGrid(spatialCellSize)
	if err := grid.Insert(moving); err != nil {
		t.Fatal(err)
	}
	if err := grid.Insert(detector); err != nil {
		t.Fatal(err)
	}
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players: map[string]*entity.Player{
			moving.SessionID:   moving,
			detector.SessionID: detector,
		},
		Reservations: make(map[string]int64),
		SpatialGrid:  grid,
	}
	if players := grid.QueryPlayers(detector); len(players) != 0 {
		t.Fatalf("expected moving player outside detection radius, got %+v", players)
	}
	message := testMatchData{
		testPresence: testPresence{userID: moving.UserID, sessionID: moving.SessionID},
		opCode:       system.OpMovementInput,
		data:         mustMarshalMovementInput(t, &entity.MovementInput{X: 1, Sequence: 1}),
	}

	match.MatchLoop(nil, nil, nil, nil, dispatcher, 1, state, []runtime.MatchData{message})
	players := grid.QueryPlayers(detector)
	if len(players) != 1 || players[0] != moving {
		t.Fatalf("expected moved player inside detection radius, got %+v", players)
	}
}

func TestMatchLoopSendsPersonalizedPlayerMovementSnapshots(t *testing.T) {
	match := &Match{}
	dispatcher := &testDispatcher{}
	random := rand.New(rand.NewSource(1))
	playerA := entity.NewPlayer("user-a", "session-a", "Player A", entity.Vector2{}, random)
	playerB := entity.NewPlayer("user-b", "session-b", "Player B", entity.Vector2{X: 5}, random)
	playerC := entity.NewPlayer("user-c", "session-c", "Player C", entity.Vector2{X: 30}, random)
	playerB.DetectionRadius = 30
	grid := spatial.NewGrid(spatialCellSize)
	for _, player := range []*entity.Player{playerA, playerB, playerC} {
		if err := grid.Insert(player); err != nil {
			t.Fatal(err)
		}
	}
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players: map[string]*entity.Player{
			playerA.SessionID: playerA,
			playerB.SessionID: playerB,
			playerC.SessionID: playerC,
		},
		Presences: map[string]runtime.Presence{
			playerA.SessionID: testPresence{userID: playerA.UserID, sessionID: playerA.SessionID},
			playerB.SessionID: testPresence{userID: playerB.UserID, sessionID: playerB.SessionID},
			playerC.SessionID: testPresence{userID: playerC.UserID, sessionID: playerC.SessionID},
		},
		Reservations: make(map[string]int64),
		SpatialGrid:  grid,
	}

	movement := testMatchData{
		testPresence: testPresence{userID: playerA.UserID, sessionID: playerA.SessionID},
		opCode:       system.OpMovementInput,
		data:         mustMarshalMovementInput(t, &entity.MovementInput{X: 1, Sequence: 1}),
	}
	match.MatchLoop(nil, nil, nil, nil, dispatcher, 9, state, []runtime.MatchData{movement})
	movementBroadcasts := make([]testBroadcast, 0, 3)
	for _, broadcast := range dispatcher.broadcasts {
		if broadcast.opCode == system.OpPlayerMovementSnapshot {
			movementBroadcasts = append(movementBroadcasts, broadcast)
		}
	}
	if len(movementBroadcasts) != 3 {
		t.Fatalf("expected one player movement snapshot per player, got %d", len(movementBroadcasts))
	}
	want := map[string][]string{
		playerA.SessionID: {playerB.SessionID},
		playerB.SessionID: {playerA.SessionID, playerC.SessionID},
		playerC.SessionID: {},
	}
	for _, broadcast := range movementBroadcasts {
		if broadcast.reliable {
			t.Fatalf("unexpected detection broadcast: opcode=%d reliable=%v", broadcast.opCode, broadcast.reliable)
		}
		if len(broadcast.presences) != 1 {
			t.Fatalf("expected one recipient, got %d", len(broadcast.presences))
		}
		recipient := broadcast.presences[0].GetSessionId()
		var snapshot system.PlayerMovementSnapshot
		if err := proto.Unmarshal(broadcast.data, &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Tick != 9 {
			t.Fatalf("unexpected tick for %s: %d", recipient, snapshot.Tick)
		}
		if snapshot.Self == nil || snapshot.Self.SessionId != recipient {
			t.Fatalf("recipient %s received unexpected self: %+v", recipient, snapshot.Self)
		}
		expected := want[recipient]
		if len(snapshot.Players) != len(expected) {
			t.Fatalf("recipient %s: expected %v, got %+v", recipient, expected, snapshot.Players)
		}
		for index, sessionID := range expected {
			if snapshot.Players[index].SessionId != sessionID {
				t.Fatalf("recipient %s result %d: expected %s, got %s", recipient, index, sessionID, snapshot.Players[index].SessionId)
			}
		}
		if recipient == playerB.SessionID {
			detectedA := snapshot.Players[0]
			if detectedA.Characters[0].Position.X != playerA.Position.X {
				t.Fatalf("character position was not updated before snapshot: character=%+v player=%+v", detectedA.Characters[0].Position, playerA.Position)
			}
		}
	}
}

func TestRosterBroadcastsOnEncounterReentryAndVersionChange(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	playerA := entity.NewPlayer("user-a", "session-a", "Player A", entity.Vector2{}, random)
	playerB := entity.NewPlayer("user-b", "session-b", "Player B", entity.Vector2{X: 5}, random)
	playerC := entity.NewPlayer("user-c", "session-c", "Player C", entity.Vector2{X: 30}, random)
	state := &State{
		Players: map[string]*entity.Player{
			playerA.SessionID: playerA, playerB.SessionID: playerB, playerC.SessionID: playerC,
		},
		Presences: map[string]runtime.Presence{
			playerA.SessionID: testPresence{userID: playerA.UserID, sessionID: playerA.SessionID},
			playerB.SessionID: testPresence{userID: playerB.UserID, sessionID: playerB.SessionID},
			playerC.SessionID: testPresence{userID: playerC.UserID, sessionID: playerC.SessionID},
		},
		RosterVersions: map[string]map[string]uint64{
			playerA.SessionID: {playerA.SessionID: playerA.RosterVersion},
			playerB.SessionID: {playerB.SessionID: playerB.RosterVersion},
			playerC.SessionID: {playerC.SessionID: playerC.RosterVersion},
		},
	}
	nearby := map[string][]*entity.Player{
		playerA.SessionID: {playerB},
		playerB.SessionID: {playerA},
		playerC.SessionID: {},
	}
	dispatcher := &testDispatcher{}

	state.broadcastRosterUpdates(nil, dispatcher, 1, nearby)
	assertRosterRecipients(t, dispatcher.broadcasts, map[string][]string{
		playerA.SessionID: {playerB.SessionID},
		playerB.SessionID: {playerA.SessionID},
	})

	dispatcher.broadcasts = nil
	state.broadcastRosterUpdates(nil, dispatcher, 2, nearby)
	if len(dispatcher.broadcasts) != 0 {
		t.Fatalf("expected no unchanged roster broadcasts, got %d", len(dispatcher.broadcasts))
	}

	state.broadcastRosterUpdates(nil, dispatcher, 3, map[string][]*entity.Player{
		playerA.SessionID: {}, playerB.SessionID: {}, playerC.SessionID: {},
	})
	playerB.Characters[0].Health = 37
	dispatcher.broadcasts = nil
	state.broadcastRosterUpdates(nil, dispatcher, 4, nearby)
	assertRosterRecipients(t, dispatcher.broadcasts, map[string][]string{
		playerA.SessionID: {playerB.SessionID},
		playerB.SessionID: {playerA.SessionID},
	})
	for _, broadcast := range dispatcher.broadcasts {
		if broadcast.presences[0].GetSessionId() != playerA.SessionID {
			continue
		}
		var batch system.PlayerRosterBatch
		if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
			t.Fatal(err)
		}
		if len(batch.Players) != 1 || batch.Players[0].Characters[0].Health != 37 {
			t.Fatalf("expected current health on re-entry, got %+v", &batch)
		}
	}

	if err := playerB.AddCharacter(entity.NewCharacter()); err != nil {
		t.Fatal(err)
	}
	dispatcher.broadcasts = nil
	state.broadcastRosterUpdates(nil, dispatcher, 5, nearby)
	assertRosterRecipients(t, dispatcher.broadcasts, map[string][]string{
		playerA.SessionID: {playerB.SessionID},
		playerB.SessionID: {playerB.SessionID},
	})
	for _, broadcast := range dispatcher.broadcasts {
		if len(broadcast.presences) != 1 || broadcast.presences[0].GetSessionId() == playerC.SessionID {
			t.Fatalf("outside observer received roster update: %+v", broadcast)
		}
	}
}

func TestRemoveRosterTrackingCleansObserverAndTarget(t *testing.T) {
	state := &State{RosterVersions: map[string]map[string]uint64{
		"session-a": {"session-a": 1, "session-b": 1},
		"session-b": {"session-a": 1, "session-b": 1},
	}}
	state.removeRosterTracking("session-b")
	if _, ok := state.RosterVersions["session-b"]; ok {
		t.Fatal("expected observer roster cache to be removed")
	}
	if _, ok := state.RosterVersions["session-a"]["session-b"]; ok {
		t.Fatal("expected target roster cache to be removed")
	}
}

func TestMatchLoopSendsReliableCombatEventsToRelevantViewers(t *testing.T) {
	match := &Match{}
	dispatcher := &testDispatcher{}
	random := rand.New(rand.NewSource(2))
	playerA := entity.NewPlayer("user-a", "session-a", "Player A", entity.Vector2{}, random)
	playerB := entity.NewPlayer("user-b", "session-b", "Player B", entity.Vector2{X: 1}, random)
	playerC := entity.NewPlayer("user-c", "session-c", "Player C", entity.Vector2{X: 30}, random)
	for _, player := range []*entity.Player{playerA, playerB} {
		character := player.Characters[0]
		character.RangeClass = entity.RangeMelee
		character.AttackRange = 2
		character.AttackSpeed = 2
		character.ImpactRatio = 0.5
		character.Position = player.Position
	}
	playerC.Characters[0].RangeClass = entity.RangeRanged
	grid := spatial.NewGrid(spatialCellSize)
	for _, player := range []*entity.Player{playerA, playerB, playerC} {
		if err := grid.Insert(player); err != nil {
			t.Fatal(err)
		}
	}
	state := &State{
		Mode:                DefaultMode,
		AllowJoinInProgress: true,
		Players: map[string]*entity.Player{
			playerA.SessionID: playerA,
			playerB.SessionID: playerB,
			playerC.SessionID: playerC,
		},
		Presences: map[string]runtime.Presence{
			playerA.SessionID: testPresence{userID: playerA.UserID, sessionID: playerA.SessionID},
			playerB.SessionID: testPresence{userID: playerB.UserID, sessionID: playerB.SessionID},
			playerC.SessionID: testPresence{userID: playerC.UserID, sessionID: playerC.SessionID},
		},
		Reservations: make(map[string]int64),
		SpatialGrid:  grid,
		random:       random,
	}

	match.MatchLoop(nil, nil, nil, nil, dispatcher, 1, state, nil)
	combatRecipients := make(map[string]bool)
	movementCount := 0
	projectileMovementCount := 0
	for _, broadcast := range dispatcher.broadcasts {
		switch broadcast.opCode {
		case system.OpCombatEventBatch:
			if !broadcast.reliable || len(broadcast.presences) != 1 {
				t.Fatalf("unexpected combat broadcast: %+v", broadcast)
			}
			recipient := broadcast.presences[0].GetSessionId()
			combatRecipients[recipient] = true
			var batch system.CombatEventBatch
			if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
				t.Fatal(err)
			}
			if batch.Tick != 1 || len(batch.Events) != 2 {
				t.Fatalf("unexpected combat batch for %s: %+v", recipient, &batch)
			}
		case system.OpPlayerMovementSnapshot:
			movementCount++
		case system.OpProjectileMovementSnapshot:
			projectileMovementCount++
		}
	}
	if !combatRecipients[playerA.SessionID] || !combatRecipients[playerB.SessionID] || combatRecipients[playerC.SessionID] {
		t.Fatalf("unexpected combat recipients: %+v", combatRecipients)
	}
	if movementCount != 3 || projectileMovementCount != 3 {
		t.Fatalf("unexpected movement snapshots: players=%d projectiles=%d", movementCount, projectileMovementCount)
	}
}

func TestCharacterBoxInitialSpawnIsValid(t *testing.T) {
	state := characterBoxTestState(7)
	events := state.spawnCharacterBoxes(24)
	if len(events) != 24 || len(state.CharacterBoxes) != 24 {
		t.Fatalf("unexpected box count: events=%d boxes=%d", len(events), len(state.CharacterBoxes))
	}
	seen := make(map[string]bool)
	boxes := make([]*entity.CharacterBox, 0, len(state.CharacterBoxes))
	for id, box := range state.CharacterBoxes {
		if seen[id] || box.WeaponType() == "" {
			t.Fatalf("invalid character box: %#v", box)
		}
		seen[id] = true
		if _, ok := state.weaponByType(box.WeaponType()); !ok {
			t.Fatalf("box has unknown weapon: %#v", box)
		}
		boxes = append(boxes, box)
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			if distanceSquared(boxes[i].Position, boxes[j].Position) < characterBoxSpawnSeparation*characterBoxSpawnSeparation {
				t.Fatalf("boxes overlap: %s and %s", boxes[i].ID, boxes[j].ID)
			}
		}
	}
}

func TestCharacterBoxCollisionAwardsNearestPlayerAndDespawnsGlobally(t *testing.T) {
	state := characterBoxTestState(1)
	far := entity.NewPlayer("user-far", "session-far", "Far", entity.Vector2{X: 0.8}, rand.New(rand.NewSource(2)))
	near := entity.NewPlayer("user-near", "session-near", "Near", entity.Vector2{X: 0.2}, rand.New(rand.NewSource(3)))
	state.Players[far.SessionID] = far
	state.Players[near.SessionID] = near
	box := entity.NewCharacterBox("box:1", entity.Vector2{}, entity.WeaponBow)
	state.CharacterBoxes[box.ID] = box
	if err := state.SpatialGrid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	initialVersion := near.RosterVersion
	events := state.collectCharacterBoxes(nil)
	if len(events) != 1 || events[0].ID != box.ID || events[0].Type != system.CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_DESPAWNED {
		t.Fatalf("unexpected collision events: %#v", events)
	}
	if len(far.Characters) != 1 || len(near.Characters) != 2 || near.RosterVersion != initialVersion+1 {
		t.Fatalf("wrong player received box: far=%d near=%d version=%d", len(far.Characters), len(near.Characters), near.RosterVersion)
	}
	awarded := near.Characters[len(near.Characters)-1]
	weapon, _ := state.weaponByType(entity.WeaponBow)
	if awarded.Weapon.Type != entity.WeaponBow || awarded.MaxHealth != weapon.Health || awarded.Damage != weapon.Damage {
		t.Fatalf("character did not receive weapon stats: %#v", awarded)
	}
	if _, exists := state.CharacterBoxes[box.ID]; exists || len(state.SpatialGrid.QueryCharacterBoxes(entity.Vector2{}, 1)) != 0 {
		t.Fatal("consumed box remains in state or spatial grid")
	}

	dispatcher := &testDispatcher{}
	state.broadcastCharacterBoxEvents(nil, dispatcher, 9, events, nil)
	if len(dispatcher.broadcasts) != 1 || dispatcher.broadcasts[0].opCode != system.OpCharacterBoxState || !dispatcher.broadcasts[0].reliable || dispatcher.broadcasts[0].presences != nil {
		t.Fatalf("despawn was not globally broadcast: %#v", dispatcher.broadcasts)
	}
}

func TestCharacterBoxRemainsWhenPlayerIsAtCapacity(t *testing.T) {
	state := characterBoxTestState(1)
	player := entity.NewPlayer("user-1", "session-1", "Full", entity.Vector2{}, rand.New(rand.NewSource(2)))
	for player.CharacterCount() < 5 {
		if err := player.AddCharacter(entity.NewCharacter()); err != nil {
			t.Fatal(err)
		}
	}
	state.Players[player.SessionID] = player
	box := entity.NewCharacterBox("box:1", entity.Vector2{}, entity.WeaponAxe)
	state.CharacterBoxes[box.ID] = box
	if err := state.SpatialGrid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	if events := state.collectCharacterBoxes(nil); len(events) != 0 {
		t.Fatalf("full player consumed box: %#v", events)
	}
	if state.CharacterBoxes[box.ID] == nil {
		t.Fatal("box was removed for full player")
	}
}

func TestCharacterBoxCollisionTieBreaksBySessionID(t *testing.T) {
	state := characterBoxTestState(1)
	playerB := entity.NewPlayer("user-b", "session-b", "B", entity.Vector2{X: 0.5}, rand.New(rand.NewSource(2)))
	playerA := entity.NewPlayer("user-a", "session-a", "A", entity.Vector2{X: -0.5}, rand.New(rand.NewSource(3)))
	state.Players[playerB.SessionID] = playerB
	state.Players[playerA.SessionID] = playerA
	box := entity.NewCharacterBox("box:1", entity.Vector2{}, entity.WeaponStaff)
	state.CharacterBoxes[box.ID] = box
	if err := state.SpatialGrid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	state.collectCharacterBoxes(nil)
	if len(playerA.Characters) != 2 || len(playerB.Characters) != 1 {
		t.Fatalf("session ID tie-break failed: player-a=%d player-b=%d", len(playerA.Characters), len(playerB.Characters))
	}
}

func TestCharacterBoxRefillAndJoinSnapshot(t *testing.T) {
	state := characterBoxTestState(4)
	state.spawnCharacterBoxes(18)
	dispatcher := &testDispatcher{}
	presence := testPresence{userID: "user-1", sessionID: "session-1"}
	state.sendCurrentCharacterBoxes(nil, dispatcher, 10, presence)
	if len(dispatcher.broadcasts) != 1 || len(dispatcher.broadcasts[0].presences) != 1 || dispatcher.broadcasts[0].presences[0].GetSessionId() != presence.sessionID {
		t.Fatalf("join snapshot was not targeted: %#v", dispatcher.broadcasts)
	}
	var batch system.CharacterBoxStateBatch
	if err := proto.Unmarshal(dispatcher.broadcasts[0].data, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 18 {
		t.Fatalf("join snapshot has %d boxes, want 18", len(batch.Events))
	}

	state.NextBoxRefillTick = 600
	dispatcher.broadcasts = nil
	match := &Match{}
	match.MatchLoop(nil, nil, nil, nil, dispatcher, 600, state, nil)
	if len(state.CharacterBoxes) < characterBoxMinCount || len(state.CharacterBoxes) > characterBoxMaxCount {
		t.Fatalf("refill produced invalid count: %d", len(state.CharacterBoxes))
	}
	count := len(state.CharacterBoxes)
	match.MatchLoop(nil, nil, nil, nil, dispatcher, 601, state, nil)
	if len(state.CharacterBoxes) != count {
		t.Fatalf("boxes refilled before next interval: before=%d after=%d", count, len(state.CharacterBoxes))
	}
}

func characterBoxTestState(seed int64) *State {
	return &State{
		Players:           make(map[string]*entity.Player),
		Presences:         make(map[string]runtime.Presence),
		Reservations:      make(map[string]int64),
		SpatialGrid:       spatial.NewGrid(spatialCellSize),
		Combat:            combat.NewSimulation(),
		RosterVersions:    make(map[string]map[string]uint64),
		CharacterBoxes:    make(map[string]*entity.CharacterBox),
		WeaponCatalog:     entity.DefaultWeaponCatalog(),
		NextBoxRefillTick: characterBoxRefillTicks,
		random:            rand.New(rand.NewSource(seed)),
	}
}

func mustMarshalMovementInput(t *testing.T, input *entity.MovementInput) []byte {
	t.Helper()
	data, err := proto.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertRosterRecipients(t *testing.T, broadcasts []testBroadcast, expected map[string][]string) {
	t.Helper()
	actual := make(map[string][]string, len(broadcasts))
	for _, broadcast := range broadcasts {
		if broadcast.opCode != system.OpPlayerRosterBatch || !broadcast.reliable || len(broadcast.presences) != 1 {
			t.Fatalf("unexpected roster broadcast: %+v", broadcast)
		}
		var batch system.PlayerRosterBatch
		if err := proto.Unmarshal(broadcast.data, &batch); err != nil {
			t.Fatal(err)
		}
		recipient := broadcast.presences[0].GetSessionId()
		for _, player := range batch.Players {
			actual[recipient] = append(actual[recipient], player.SessionId)
		}
	}
	if len(actual) != len(expected) {
		t.Fatalf("unexpected roster recipients: got=%v want=%v", actual, expected)
	}
	for recipient, want := range expected {
		got := actual[recipient]
		if len(got) != len(want) {
			t.Fatalf("recipient %s: got=%v want=%v", recipient, got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("recipient %s: got=%v want=%v", recipient, got, want)
			}
		}
	}
}

func assertStateSnapshot(t *testing.T, dispatcher *testDispatcher, tick int64, playerCount int) {
	t.Helper()
	if dispatcher.broadcastOpCode != system.OpStateSnapshot || !dispatcher.broadcastReliable {
		t.Fatalf("unexpected broadcast: opcode=%d reliable=%v", dispatcher.broadcastOpCode, dispatcher.broadcastReliable)
	}

	var snapshot system.StateSnapshot
	if err := proto.Unmarshal(dispatcher.broadcastData, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Tick != tick || snapshot.PlayerCount != int32(playerCount) {
		t.Fatalf("unexpected state snapshot: %+v", &snapshot)
	}
}

type testPresence struct {
	userID    string
	sessionID string
	username  string
}

func (p testPresence) GetHidden() bool      { return false }
func (p testPresence) GetPersistence() bool { return false }
func (p testPresence) GetUsername() string {
	if p.username != "" {
		return p.username
	}
	return p.userID
}
func (p testPresence) GetStatus() string                 { return "" }
func (p testPresence) GetReason() runtime.PresenceReason { return runtime.PresenceReasonUnknown }
func (p testPresence) GetUserId() string                 { return p.userID }
func (p testPresence) GetSessionId() string              { return p.sessionID }
func (p testPresence) GetNodeId() string                 { return "node-1" }

type testUserLookup struct {
	users []*api.User
}

func (l testUserLookup) UsersGetId(context.Context, []string, []string) ([]*api.User, error) {
	return l.users, nil
}

type testMatchData struct {
	testPresence
	opCode int64
	data   []byte
}

func (m testMatchData) GetOpCode() int64      { return m.opCode }
func (m testMatchData) GetData() []byte       { return m.data }
func (m testMatchData) GetReliable() bool     { return false }
func (m testMatchData) GetReceiveTime() int64 { return 0 }

type testDispatcher struct {
	label             string
	broadcastOpCode   int64
	broadcastData     []byte
	broadcastReliable bool
	broadcastCount    int
	broadcasts        []testBroadcast
}

type testBroadcast struct {
	opCode    int64
	data      []byte
	presences []runtime.Presence
	reliable  bool
}

func (d *testDispatcher) BroadcastMessage(opCode int64, data []byte, presences []runtime.Presence, _ runtime.Presence, reliable bool) error {
	d.broadcastOpCode = opCode
	d.broadcastData = data
	d.broadcastReliable = reliable
	d.broadcastCount++
	d.broadcasts = append(d.broadcasts, testBroadcast{
		opCode: opCode, data: append([]byte(nil), data...), presences: append([]runtime.Presence(nil), presences...), reliable: reliable,
	})
	return nil
}
func (d *testDispatcher) BroadcastMessageDeferred(int64, []byte, []runtime.Presence, runtime.Presence, bool) error {
	return nil
}
func (d *testDispatcher) MatchKick([]runtime.Presence) error { return nil }
func (d *testDispatcher) MatchLabelUpdate(label string) error {
	d.label = label
	return nil
}
