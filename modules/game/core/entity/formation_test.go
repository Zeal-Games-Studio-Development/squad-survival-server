package entity

import (
	"errors"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/strategy"
)

func TestNewPlayerSnapsCharacterToCenterSlot(t *testing.T) {
	player := NewPlayer("user", "session", "Player", Vector2{X: 10, Y: 20}, rand.New(rand.NewSource(1)))
	character := player.Characters[0]
	if character.Position != player.Position || character.TargetPosition != player.Position {
		t.Fatalf("expected character at strategy center, got position=%+v target=%+v", character.Position, character.TargetPosition)
	}
}

func TestAssignCharacterTargetsPrioritizesRangeClass(t *testing.T) {
	player := newTestPlayer(Vector2{})
	firstMelee := &Character{RangeClass: RangeMelee, AttackRange: 100, MoveSpeed: 5}
	ranged := &Character{RangeClass: RangeRanged, AttackRange: 1, MoveSpeed: 5}
	secondMelee := &Character{RangeClass: RangeMelee, AttackRange: 50, MoveSpeed: 5}
	player.Characters = []*Character{firstMelee, ranged, secondMelee}
	if err := AssignCharacterTargets(player); err != nil {
		t.Fatal(err)
	}

	if ranged.TargetPosition != (Vector2{}) {
		t.Fatalf("expected ranged character at center, got %+v", ranged.TargetPosition)
	}
	if firstMelee.TargetPosition != (Vector2{X: strategy.SlotSpacing, Y: strategy.SlotSpacing}) {
		t.Fatalf("unexpected first melee target: %+v", firstMelee.TargetPosition)
	}
	if secondMelee.TargetPosition != (Vector2{X: strategy.SlotSpacing, Y: -strategy.SlotSpacing}) {
		t.Fatalf("unexpected second melee target: %+v", secondMelee.TargetPosition)
	}
}

func TestEqualRangeClassKeepsCharacterIndexAndRotatesWithFacing(t *testing.T) {
	player := newTestPlayer(Vector2{})
	first := &Character{RangeClass: RangeRanged, AttackRange: 1, MoveSpeed: 5}
	second := &Character{RangeClass: RangeRanged, AttackRange: 10, MoveSpeed: 5}
	player.Characters = []*Character{first, second}
	player.Facing = Vector2{Y: 1}
	if err := AssignCharacterTargets(player); err != nil {
		t.Fatal(err)
	}

	if first.TargetPosition != (Vector2{}) {
		t.Fatalf("expected first ranged character at center, got %+v", first.TargetPosition)
	}
	if second.TargetPosition != (Vector2{X: -strategy.SlotSpacing, Y: strategy.SlotSpacing}) {
		t.Fatalf("expected formation to rotate with facing, got %+v", second.TargetPosition)
	}
}

func TestStepCharactersMovesSoftlyWithoutOvershoot(t *testing.T) {
	player := newTestPlayer(Vector2{X: 10})
	character := player.Characters[0]
	character.Position = Vector2{}
	character.MoveSpeed = 5
	if err := StepCharacters(player); err != nil {
		t.Fatal(err)
	}
	expectedStep := character.MoveSpeed * CharacterFollowSpeedMultiplier / float64(TickRate)
	if !almostEqual(character.Position.X, expectedStep) {
		t.Fatalf("expected soft movement step %f, got %+v", expectedStep, character.Position)
	}

	player.Position = Vector2{X: character.Position.X + 0.1}
	if err := StepCharacters(player); err != nil {
		t.Fatal(err)
	}
	if character.Position != player.Position {
		t.Fatalf("expected character to stop at target, got %+v", character.Position)
	}
}

func TestSetStrategyRejectsInsufficientCapacity(t *testing.T) {
	player := newTestPlayer(Vector2{})
	if err := player.AddCharacter(NewCharacter()); err != nil {
		t.Fatal(err)
	}
	previous := player.Strategy
	limited := strategy.Definition{Type: "limited"}
	limited.Grid[strategy.GridCenter][strategy.GridCenter] = 1

	if err := player.SetStrategy(limited); !errors.Is(err, ErrStrategyCapacity) {
		t.Fatalf("expected capacity error, got %v", err)
	}
	if player.Strategy.Type != previous.Type {
		t.Fatalf("strategy changed after rejection: %q", player.Strategy.Type)
	}
}

func TestAddCharacterEnforcesMaximum(t *testing.T) {
	player := newTestPlayer(Vector2{})
	player.Characters = append(player.Characters, nil)
	if player.CharacterCount() != 1 {
		t.Fatalf("nil character consumed capacity: %d", player.CharacterCount())
	}
	for player.CharacterCount() < strategy.MaxCharacters {
		if err := player.AddCharacter(NewCharacter()); err != nil {
			t.Fatal(err)
		}
	}
	if err := player.AddCharacter(NewCharacter()); !errors.Is(err, ErrCharacterLimit) {
		t.Fatalf("expected character limit error, got %v", err)
	}
}

func TestRemoveDeadCharactersCompactsFormation(t *testing.T) {
	player := newTestPlayer(Vector2{})
	alive := &Character{Health: 1, MoveSpeed: 5}
	dead := &Character{Health: 0, MoveSpeed: 5}
	overKilled := &Character{Health: -10, MoveSpeed: 5}
	player.Characters = []*Character{dead, alive, nil, overKilled}

	if removed := player.RemoveDeadCharacters(); removed != 2 {
		t.Fatalf("expected 2 dead characters removed, got %d", removed)
	}
	if len(player.Characters) != 2 || player.Characters[0] != alive || player.Characters[1] != nil {
		t.Fatalf("unexpected survivors: %+v", player.Characters)
	}
}
