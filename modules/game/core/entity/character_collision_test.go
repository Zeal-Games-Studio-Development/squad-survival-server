package entity

import (
	"math"
	"testing"

	"squad-survival-be/modules/game/core/world"
)

func TestCharacterRadiusAndFriendlyOverlap(t *testing.T) {
	player := newTestPlayer(Vector2{})
	if player.Characters[0].Radius != 0.4 {
		t.Fatalf("unexpected default radius: %v", player.Characters[0].Radius)
	}
	player.Level = 2
	player.SessionID = "self"
	if err := player.AddCharacter(NewCharacter()); err != nil {
		t.Fatal(err)
	}
	if err := StepAllCharacters(map[string]*Player{"self": player}, 1); err != nil {
		t.Fatal(err)
	}
	if distance := math.Hypot(player.Characters[0].Position.X-player.Characters[1].Position.X, player.Characters[0].Position.Y-player.Characters[1].Position.Y); !almostEqual(distance, math.Sqrt(2)*1.5) {
		t.Fatalf("friendly formation spacing changed: %v", distance)
	}
}

func TestCharacterSlidesAroundEnemyWithoutExceedingStep(t *testing.T) {
	mover := newTestPlayer(Vector2{X: 3})
	enemy := newTestPlayer(Vector2{X: 1.0, Y: 0.35})
	mover.SessionID, enemy.SessionID = "a", "b"
	mover.Characters[0].Position = Vector2{}
	start := mover.Characters[0].Position
	if err := StepAllCharacters(map[string]*Player{"a": mover, "b": enemy}, 1); err != nil {
		t.Fatal(err)
	}
	got := mover.Characters[0].Position
	if got.Y >= 0 || got.X <= 0 {
		t.Fatalf("expected forward slide away from obstacle, got %+v", got)
	}
	if math.Hypot(got.X-start.X, got.Y-start.Y) > 0.75+collisionEpsilon {
		t.Fatalf("movement exceeded per-tick speed: %+v", got)
	}
	if math.Hypot(got.X-enemy.Characters[0].Position.X, got.Y-enemy.Characters[0].Position.Y) < 0.8-collisionEpsilon {
		t.Fatalf("enemy overlap: %+v", got)
	}
}

func TestCharacterSnapFindsFreePositionAndRespectsSkill(t *testing.T) {
	mover := newTestPlayer(Vector2{X: 10})
	enemy := newTestPlayer(Vector2{X: 10})
	mover.SessionID, enemy.SessionID = "a", "b"
	mover.Characters[0].Position = Vector2{}
	mover.Characters[0].ActiveSkillID = "skill"
	mover.Characters[0].SkillCompleteTick = 2
	players := map[string]*Player{"a": mover, "b": enemy}
	if err := StepAllCharacters(players, 2); err != nil {
		t.Fatal(err)
	}
	if mover.Characters[0].Position != (Vector2{}) {
		t.Fatal("casting character moved")
	}
	mover.Characters[0].ResetSkill()
	if err := StepAllCharacters(players, 3); err != nil {
		t.Fatal(err)
	}
	got := mover.Characters[0].Position
	if math.Hypot(got.X-10, got.Y) > 2 || math.Hypot(got.X-10, got.Y) < 0.8-collisionEpsilon {
		t.Fatalf("snap did not choose a free nearby position: %+v", got)
	}
}

func TestCharacterSnapThresholdAndExistingOverlap(t *testing.T) {
	character := NewCharacter()
	character.Position = Vector2{}
	character.TargetPosition = Vector2{X: CharacterSnapDistance}
	if got := moveCharacterWithCollisions(character, nil, 0.75); !almostEqual(got.X, 0.75) {
		t.Fatalf("character snapped at threshold: %+v", got)
	}
	character.TargetPosition.X = CharacterSnapDistance + 0.01
	if got := moveCharacterWithCollisions(character, nil, 0.75); got != character.TargetPosition {
		t.Fatalf("character did not snap beyond threshold: %+v", got)
	}
	character.TargetPosition = Vector2{X: 2}
	other := NewCharacter()
	other.Position = Vector2{}
	got := moveCharacterWithCollisions(character, []*Character{other}, 0.75)
	if !isFinite(got.X) || !isFinite(got.Y) || math.Hypot(got.X, got.Y) < 0.8-collisionEpsilon {
		t.Fatalf("existing overlap was not resolved: %+v", got)
	}
}

func TestMultipleBlockersKeepCharacterOutsideAllCircles(t *testing.T) {
	character := NewCharacter()
	character.TargetPosition = Vector2{X: 4}
	first, second := NewCharacter(), NewCharacter()
	first.Position = Vector2{X: 1.0, Y: 0.35}
	second.Position = Vector2{X: 1.0, Y: -0.35}
	got := moveCharacterWithCollisions(character, []*Character{first, second}, 0.75)
	if math.Hypot(got.X, got.Y) > 0.75+collisionEpsilon {
		t.Fatalf("movement exceeded step: %+v", got)
	}
	for _, other := range []*Character{first, second} {
		if math.Hypot(got.X-other.Position.X, got.Y-other.Position.Y) < 0.8-collisionEpsilon {
			t.Fatalf("character overlaps blocker: %+v", got)
		}
	}
}

func TestCharacterCollisionAtBoundaryAndDeterministicOrder(t *testing.T) {
	makePlayers := func() map[string]*Player {
		a := newTestPlayer(Vector2{X: world.PlayAreaRadius})
		b := newTestPlayer(Vector2{X: world.PlayAreaRadius - 1, Y: 1})
		a.SessionID, b.SessionID = "a", "b"
		a.Characters[0].Position = Vector2{X: world.PlayAreaRadius - 2}
		return map[string]*Player{"b": b, "a": a}
	}
	var expected Vector2
	for run := 0; run < 10; run++ {
		players := makePlayers()
		if err := StepAllCharacters(players, 1); err != nil {
			t.Fatal(err)
		}
		got := players["a"].Characters[0].Position
		if !isFinite(got.X) || !isFinite(got.Y) || math.Hypot(got.X, got.Y) > world.PlayAreaRadius+collisionEpsilon {
			t.Fatalf("invalid boundary position: %+v", got)
		}
		if run > 0 && got != expected {
			t.Fatalf("non-deterministic movement: %+v versus %+v", got, expected)
		}
		expected = got
	}
}
