package experience

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/spatial"
)

func TestSpawnCollectAndVisibility(t *testing.T) {
	grid := spatial.NewGrid(20)
	manager := NewManager(grid, rand.New(rand.NewSource(1)), entity.TickRate)
	manager.Config.Packages = []PackageDefinition{{Tier: "small", MinValue: 10, MaxValue: 12, TargetCount: 1}, {Tier: "medium", MinValue: 15, MaxValue: 18}, {Tier: "large", MinValue: 20, MaxValue: 22}}
	if got := manager.SpawnMissing(nil); got != 1 {
		t.Fatalf("spawned %d", got)
	}
	var item *entity.ExperiencePackage
	for _, candidate := range manager.Packages {
		item = candidate
	}
	if item.Value.Experience < 10 || item.Value.Experience > 12 {
		t.Fatalf("value outside range: %d", item.Value.Experience)
	}

	player := entity.NewPlayer("user", "session", "", item.Position, rand.New(rand.NewSource(2)))
	players := map[string]*entity.Player{player.SessionID: player}
	events := manager.VisibilityEvents(players, nil)[player.SessionID]
	if len(events) != 1 {
		t.Fatalf("expected detection, got %+v", events)
	}
	collections := manager.Collect(players)
	if len(collections) != 1 || player.Experience != item.Value.Experience {
		t.Fatalf("unexpected collection: %+v xp=%d", collections, player.Experience)
	}
	events = manager.VisibilityEvents(players, collections)[player.SessionID]
	if len(events) != 1 || events[0].Type.String() != "EXPERIENCE_PACKAGE_EVENT_TYPE_COLLECTED" {
		t.Fatalf("expected collected event: %+v", events)
	}
}

func TestCollectionTieBreaksBySessionAndMaxLevelConsumes(t *testing.T) {
	grid := spatial.NewGrid(20)
	manager := NewManager(grid, rand.New(rand.NewSource(1)), entity.TickRate)
	manager.Packages = make(map[string]*entity.ExperiencePackage)
	item := entity.NewExperiencePackage("xp:1", entity.Vector2{}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_LARGE, 22)
	manager.Packages[item.ID] = item
	if err := grid.InsertExperiencePackage(item); err != nil {
		t.Fatal(err)
	}
	a := entity.NewPlayer("a", "a", "", entity.Vector2{}, rand.New(rand.NewSource(2)))
	b := entity.NewPlayer("b", "b", "", entity.Vector2{}, rand.New(rand.NewSource(3)))
	a.Level = 10
	collections := manager.Collect(map[string]*entity.Player{"b": b, "a": a})
	if len(collections) != 1 || collections[0].Player != a || a.Experience != 0 || len(manager.Packages) != 0 {
		t.Fatalf("unexpected winner/max level result: %+v", collections)
	}
}

func TestCollectUsesConfiguredFourUnitPickupRadius(t *testing.T) {
	grid := spatial.NewGrid(20)
	manager := NewManager(grid, rand.New(rand.NewSource(1)), entity.TickRate)
	inside := entity.NewExperiencePackage("xp:inside", entity.Vector2{X: 3.99}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 10)
	outside := entity.NewExperiencePackage("xp:outside", entity.Vector2{X: 4.01}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 10)
	for _, item := range []*entity.ExperiencePackage{inside, outside} {
		manager.Packages[item.ID] = item
		if err := grid.InsertExperiencePackage(item); err != nil {
			t.Fatal(err)
		}
	}
	player := entity.NewPlayer("user", "session", "", entity.Vector2{}, rand.New(rand.NewSource(2)))
	collections := manager.Collect(map[string]*entity.Player{player.SessionID: player})
	if len(collections) != 1 || collections[0].Package.ID != inside.ID {
		t.Fatalf("unexpected collections: %+v", collections)
	}
	if manager.Packages[outside.ID] == nil {
		t.Fatal("package outside pickup radius was collected")
	}
}

func TestAwardKillExperienceUsesVictimCountBeforeEachDeath(t *testing.T) {
	killer := entity.NewPlayer("killer", "killer-session", "", entity.Vector2{}, rand.New(rand.NewSource(1)))
	victim := entity.NewPlayer("victim", "victim-session", "", entity.Vector2{}, rand.New(rand.NewSource(2)))
	players := map[string]*entity.Player{killer.SessionID: killer, victim.SessionID: victim}
	events := []combat.Event{{Type: combat.EventCharacterDied, AttackerUserID: killer.UserID, TargetUserID: victim.UserID}, {Type: combat.EventCharacterDied, AttackerUserID: killer.UserID, TargetUserID: victim.UserID}}
	AwardKillExperience(players, map[string]int{victim.UserID: 3}, events, DefaultConfig())
	if killer.Experience != 25 {
		t.Fatalf("got %d XP, want 25", killer.Experience)
	}
}
