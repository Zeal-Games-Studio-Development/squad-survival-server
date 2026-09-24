package entity

import (
	"math"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/world"
)

func TestDiagonalMovementIsNormalized(t *testing.T) {
	player := newTestPlayer(Vector2{})
	if player.DetectionRadius != DefaultDetectionRadius {
		t.Fatalf("expected detection radius %f, got %f", DefaultDetectionRadius, player.DetectionRadius)
	}
	if len(player.Characters) != 1 || player.Characters[0].Weapon.Type == "" {
		t.Fatalf("unexpected default characters: %+v", player.Characters)
	}
	player.Characters = []*Character{NewCharacter()}
	if !ApplyMovementInput(player, &MovementInput{X: 1, Y: 1, Sequence: 1}, 0) {
		t.Fatal("expected movement input to be accepted")
	}

	StepMovement(player, 0)
	if !almostEqual(math.Hypot(player.Position.X, player.Position.Y), 0.5) {
		t.Fatalf("expected 0.5 units per tick, got %+v", player.Position)
	}
	if !almostEqual(player.Facing.X, math.Sqrt(0.5)) || !almostEqual(player.Facing.Y, math.Sqrt(0.5)) {
		t.Fatalf("unexpected facing: %+v", player.Facing)
	}
}

func TestPlayerUsesSlowestCharacterMoveSpeed(t *testing.T) {
	player := newTestPlayer(Vector2{})
	player.Characters = []*Character{
		{MoveSpeed: 7},
		{MoveSpeed: 3},
		{MoveSpeed: 5},
	}
	ApplyMovementInput(player, &MovementInput{X: 1, Sequence: 1}, 0)
	StepMovement(player, 0)

	if !almostEqual(player.Position.X, 0.3) {
		t.Fatalf("expected movement at slowest speed, got x=%f", player.Position.X)
	}
}

func TestPlayerAssignsStableUniqueCharacterIDs(t *testing.T) {
	player := newTestPlayer(Vector2{})
	initialVersion := player.RosterVersion
	firstID := player.Characters[0].ID
	second := NewCharacter()
	if err := player.AddCharacter(second); err != nil {
		t.Fatal(err)
	}
	if firstID != player.UserID+":1" || second.ID != player.UserID+":2" {
		t.Fatalf("unexpected character IDs: first=%q second=%q", firstID, second.ID)
	}
	if player.RosterVersion != initialVersion+1 {
		t.Fatalf("expected add to increment roster version, got %d", player.RosterVersion)
	}
	player.RemoveCharacterAt(0)
	if second.ID != player.UserID+":2" {
		t.Fatalf("character ID changed after compaction: %q", second.ID)
	}
	if player.RosterVersion != initialVersion+2 {
		t.Fatalf("expected remove to increment roster version, got %d", player.RosterVersion)
	}
}

func TestPlayerWithoutCharactersCannotMove(t *testing.T) {
	player := newTestPlayer(Vector2{})
	player.Characters = nil
	ApplyMovementInput(player, &MovementInput{X: 1, Sequence: 1}, 0)
	StepMovement(player, 0)

	if player.Position != (Vector2{}) {
		t.Fatalf("expected player without characters to remain still, got %+v", player.Position)
	}
}

func TestZeroInputStopsWithoutChangingFacing(t *testing.T) {
	player := newTestPlayer(Vector2{})
	player.Characters = []*Character{NewCharacter()}
	ApplyMovementInput(player, &MovementInput{Y: 1, Sequence: 1}, 0)
	ApplyMovementInput(player, &MovementInput{Sequence: 2}, 1)
	StepMovement(player, 1)

	if player.Position != (Vector2{}) {
		t.Fatalf("expected player to remain still, got %+v", player.Position)
	}
	if player.Facing != (Vector2{Y: 1}) {
		t.Fatalf("expected facing to remain unchanged, got %+v", player.Facing)
	}
}

func TestStaleSequenceIsIgnored(t *testing.T) {
	player := newTestPlayer(Vector2{})
	player.Characters = []*Character{NewCharacter()}
	ApplyMovementInput(player, &MovementInput{X: 1, Sequence: 5}, 0)

	if ApplyMovementInput(player, &MovementInput{Y: 1, Sequence: 5}, 1) {
		t.Fatal("expected duplicate sequence to be ignored")
	}
	if ApplyMovementInput(player, &MovementInput{Y: 1, Sequence: 4}, 1) {
		t.Fatal("expected stale sequence to be ignored")
	}
	if player.Direction != (Vector2{X: 1}) {
		t.Fatalf("stale input changed direction: %+v", player.Direction)
	}
}

func TestInputTimesOutAfterThreeTicks(t *testing.T) {
	player := newTestPlayer(Vector2{})
	player.Characters = []*Character{NewCharacter()}
	ApplyMovementInput(player, &MovementInput{X: 1, Sequence: 1}, 0)

	for tick := int64(0); tick <= InputTimeoutTicks; tick++ {
		StepMovement(player, tick)
	}
	if !almostEqual(player.Position.X, 1.5) {
		t.Fatalf("expected movement for three ticks, got x=%f", player.Position.X)
	}
	if player.Direction != (Vector2{}) {
		t.Fatalf("expected timed out input to stop, got %+v", player.Direction)
	}
}

func TestMovementCannotLeavePlayArea(t *testing.T) {
	player := newTestPlayer(Vector2{X: world.PlayAreaRadius - 0.1})
	player.Characters = []*Character{NewCharacter()}
	ApplyMovementInput(player, &MovementInput{X: 1, Sequence: 1}, 0)
	StepMovement(player, 0)

	if !almostEqual(math.Hypot(player.Position.X, player.Position.Y), world.PlayAreaRadius) {
		t.Fatalf("expected player on play-area edge, got %+v", player.Position)
	}
}

func newTestPlayer(position Vector2) *Player {
	return NewPlayer("user-1", "session-1", "Player One", position, rand.New(rand.NewSource(1)))
}

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
