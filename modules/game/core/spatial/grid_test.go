package spatial

import (
	"errors"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func TestQueryPlayersFiltersRadiusSelfAndSorts(t *testing.T) {
	grid := NewGrid(20)
	origin := player("origin", 0, 0)
	nearB := player("b", 3, 4)
	nearA := player("a", -3, -4)
	edge := player("edge", entity.DefaultDetectionRadius, 0)
	outside := player("outside", entity.DefaultDetectionRadius+0.01, 0)
	for _, candidate := range []*entity.Player{origin, nearB, nearA, edge, outside} {
		if err := grid.Insert(candidate); err != nil {
			t.Fatal(err)
		}
	}

	got := grid.QueryPlayers(origin)
	want := []string{"a", "b", "edge"}
	if len(got) != len(want) {
		t.Fatalf("expected %d players, got %d", len(want), len(got))
	}
	for index, sessionID := range want {
		if got[index].SessionID != sessionID {
			t.Fatalf("result %d: expected %q, got %q", index, sessionID, got[index].SessionID)
		}
	}
}

func TestQueryPlayersFindsPlayersAcrossNegativeCells(t *testing.T) {
	grid := NewGrid(20)
	left := player("left", -0.1, 0)
	right := player("right", 0.1, 0)
	if err := grid.Insert(left); err != nil {
		t.Fatal(err)
	}
	if err := grid.Insert(right); err != nil {
		t.Fatal(err)
	}

	got := grid.QueryPlayers(left)
	if len(got) != 1 || got[0] != right {
		t.Fatalf("unexpected players across negative cell boundary: %+v", got)
	}
}

func TestQueryPlayersWithinRadiusOverridesDetectionRadius(t *testing.T) {
	grid := NewGrid(20)
	origin := player("origin", 0, 0)
	edge := player("edge", 120, 0)
	outside := player("outside", 120.1, 0)
	for _, candidate := range []*entity.Player{origin, edge, outside} {
		if err := grid.Insert(candidate); err != nil {
			t.Fatal(err)
		}
	}
	if got := grid.QueryPlayersWithinRadius(origin, 120); len(got) != 1 || got[0] != edge {
		t.Fatalf("unexpected players within radius 120: %+v", got)
	}
	if got := grid.QueryPlayers(origin); len(got) != 0 {
		t.Fatalf("default detection radius changed: %+v", got)
	}
}

func TestMoveUpdatesPlayerCell(t *testing.T) {
	grid := NewGrid(20)
	origin := player("origin", 0, 0)
	moving := player("moving", 30, 0)
	if err := grid.Insert(origin); err != nil {
		t.Fatal(err)
	}
	if err := grid.Insert(moving); err != nil {
		t.Fatal(err)
	}
	if got := grid.QueryPlayers(origin); len(got) != 0 {
		t.Fatalf("expected moving player outside radius, got %+v", got)
	}

	moving.Position = entity.Vector2{X: 5}
	if err := grid.Move(moving); err != nil {
		t.Fatal(err)
	}
	if got := grid.QueryPlayers(origin); len(got) != 1 || got[0] != moving {
		t.Fatalf("expected moved player in radius, got %+v", got)
	}
}

func TestRemoveAndLifecycleErrors(t *testing.T) {
	grid := NewGrid(20)
	tracked := player("tracked", 0, 0)
	if err := grid.Insert(tracked); err != nil {
		t.Fatal(err)
	}
	if err := grid.Insert(tracked); !errors.Is(err, ErrPlayerExists) {
		t.Fatalf("expected duplicate insert error, got %v", err)
	}
	if err := grid.Move(player("missing", 0, 0)); !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected missing move error, got %v", err)
	}
	if !grid.Remove(tracked.SessionID) {
		t.Fatal("expected tracked player to be removed")
	}
	if grid.Remove(tracked.SessionID) {
		t.Fatal("expected second remove to return false")
	}
}

func TestQueryCharacterBoxesFiltersRadiusSortsAndRemoves(t *testing.T) {
	grid := NewGrid(20)
	boxes := []*entity.CharacterBox{
		entity.NewCharacterBox("box:b", entity.Vector2{X: 0.6, Y: 0.8}, entity.WeaponBow),
		entity.NewCharacterBox("box:a", entity.Vector2{X: -0.6, Y: -0.8}, entity.WeaponSword),
		entity.NewCharacterBox("box:outside", entity.Vector2{X: 1.01}, entity.WeaponAxe),
		entity.NewCharacterBox("box:negative", entity.Vector2{X: -0.1}, entity.WeaponCrossbow),
	}
	for _, box := range boxes {
		if err := grid.InsertCharacterBox(box); err != nil {
			t.Fatal(err)
		}
	}
	got := grid.QueryCharacterBoxes(entity.Vector2{}, 1)
	want := []string{"box:negative", "box:a", "box:b"}
	if len(got) != len(want) {
		t.Fatalf("expected %d boxes, got %#v", len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("result %d: got %q, want %q", i, got[i].ID, id)
		}
	}
	if !grid.RemoveCharacterBox("box:a") || grid.RemoveCharacterBox("box:a") {
		t.Fatal("unexpected character box remove result")
	}
	if got := grid.QueryCharacterBoxes(entity.Vector2{}, 1); len(got) != 2 {
		t.Fatalf("removed box remains indexed: %#v", got)
	}
}

func TestCharacterBoxLifecycleErrors(t *testing.T) {
	grid := NewGrid(20)
	if err := grid.InsertCharacterBox(nil); !errors.Is(err, ErrNilCharacterBox) {
		t.Fatalf("expected nil box error, got %v", err)
	}
	if err := grid.InsertCharacterBox(&entity.CharacterBox{}); !errors.Is(err, ErrEmptyBoxID) {
		t.Fatalf("expected empty box ID error, got %v", err)
	}
	box := entity.NewCharacterBox("box:1", entity.Vector2{}, entity.WeaponBow)
	if err := grid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	if err := grid.InsertCharacterBox(box); !errors.Is(err, ErrBoxExists) {
		t.Fatalf("expected duplicate box error, got %v", err)
	}
}

func TestExperiencePackageQueryAndRemove(t *testing.T) {
	grid := NewGrid(20)
	a := entity.NewExperiencePackage("xp:a", entity.Vector2{X: 0.5}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 10)
	b := entity.NewExperiencePackage("xp:b", entity.Vector2{X: 2}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_LARGE, 20)
	if err := grid.InsertExperiencePackage(a); err != nil {
		t.Fatal(err)
	}
	if err := grid.InsertExperiencePackage(b); err != nil {
		t.Fatal(err)
	}
	got := grid.QueryExperiencePackages(entity.Vector2{}, 1)
	if len(got) != 1 || got[0] != a {
		t.Fatalf("unexpected query: %+v", got)
	}
	if !grid.RemoveExperiencePackage(a.ID) || grid.RemoveExperiencePackage(a.ID) {
		t.Fatal("unexpected remove result")
	}
	if err := grid.InsertExperiencePackage(nil); !errors.Is(err, ErrNilExperiencePackage) {
		t.Fatalf("unexpected nil error: %v", err)
	}
}

func player(sessionID string, x, y float64) *entity.Player {
	return entity.NewPlayer(sessionID, sessionID, sessionID, entity.Vector2{X: x, Y: y}, rand.New(rand.NewSource(1)))
}
