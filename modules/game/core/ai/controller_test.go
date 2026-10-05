package ai

import (
	"math"
	"math/rand"
	"strconv"
	"testing"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/progression"
	"squad-survival-be/modules/game/core/spatial"
	"squad-survival-be/modules/game/core/world"
)

func TestControllerPrioritiesAndMissingTargets(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	player := entity.NewPlayer("ai-user:m:1", "ai-session:m:1", "Bot", entity.Vector2{}, random)
	player.Level = 2
	enemy := entity.NewPlayer("human", "human-session", "Human", entity.Vector2{X: -4}, random)
	enemy.Characters = append(enemy.Characters, &entity.Character{Health: 100})
	grid := spatial.NewGrid(20)
	if err := grid.Insert(enemy); err != nil {
		t.Fatal(err)
	}
	box := entity.NewCharacterBox("box", entity.Vector2{X: 10}, entity.WeaponBow)
	item := entity.NewExperiencePackage("xp", entity.Vector2{Y: 5}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 1)
	if err := grid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	if err := grid.InsertExperiencePackage(item); err != nil {
		t.Fatal(err)
	}
	controller := NewController(player)
	controller.Step(1, grid, random)
	if controller.TargetKind != "box" || player.Direction.X <= 0 {
		t.Fatalf("expected box first: %+v", controller)
	}
	player.Position = entity.Vector2{X: 9.5}
	controller.Step(2, grid, random)
	if player.Direction != (entity.Vector2{}) {
		t.Fatal("bot must stop inside box pickup radius")
	}
	grid.RemoveCharacterBox(box.ID)
	controller.Step(3, grid, random)
	if controller.TargetKind != "experience" || controller.TargetID != item.ID {
		t.Fatalf("expected XP after box disappeared: %+v", controller)
	}
	grid.RemoveExperiencePackage(item.ID)
	enemy.Characters = enemy.Characters[:1]
	controller.Step(4, grid, random)
	if controller.TargetKind != "player" || controller.TargetID != enemy.SessionID || player.Direction.X >= 0 {
		t.Fatalf("expected nearest opponent: %+v", controller)
	}
	enemy.Characters[0].Health = 0
	controller.Step(5, grid, random)
	if controller.TargetKind != "wander" || player.Direction == (entity.Vector2{}) {
		t.Fatal("expected wandering after opponent died")
	}
	player.Characters[0].Health = 0
	controller.Step(6, grid, random)
	if player.Direction != (entity.Vector2{}) || controller.TargetID != "" {
		t.Fatal("eliminated bot kept moving or targeting")
	}
}

func TestControllerSkipsFullRosterAndMaxLevelXP(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	player := entity.NewPlayer("bot", "bot-session", "Bot", entity.Vector2{}, random)
	grid := spatial.NewGrid(20)
	if err := grid.InsertCharacterBox(entity.NewCharacterBox("box", entity.Vector2{X: 2}, entity.WeaponBow)); err != nil {
		t.Fatal(err)
	}
	if err := grid.InsertExperiencePackage(entity.NewExperiencePackage("xp", entity.Vector2{Y: 2}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 1)); err != nil {
		t.Fatal(err)
	}
	controller := NewController(player)
	controller.Step(1, grid, random)
	if controller.TargetKind != "experience" {
		t.Fatal("full level-one roster should seek XP")
	}
	grid.RemoveCharacterBox("box")
	player.Level = progression.MaxLevel
	controller.Step(2, grid, random)
	if controller.TargetKind != "wander" {
		t.Fatal("max-level bot should not seek XP")
	}
}

func TestControllerWanderChangesDirectionAndStaysInWorld(t *testing.T) {
	random := rand.New(rand.NewSource(3))
	player := entity.NewPlayer("bot", "bot-session", "Bot", entity.Vector2{}, random)
	controller := NewController(player)
	grid := spatial.NewGrid(20)
	controller.Step(1, grid, random)
	first := player.Direction
	controller.Step(2, grid, random)
	if first != player.Direction {
		t.Fatal("wander direction changed before its deadline")
	}
	controller.Step(1+wanderTicks, grid, random)
	if first == player.Direction {
		t.Fatal("wander direction did not change at its deadline")
	}
	player.Position = entity.Vector2{X: world.PlayAreaRadius}
	controller.wanderDirection = entity.Vector2{X: 1}
	controller.wanderUntil = 1000
	controller.Step(100, grid, random)
	entity.StepMovement(player, 100)
	if player.Direction.X >= 0 || math.Hypot(player.Position.X, player.Position.Y) >= world.PlayAreaRadius {
		t.Fatal("bot did not turn away from boundary")
	}
}

func TestControllerCanTargetBotsAndDoesNotOvershoot(t *testing.T) {
	random := rand.New(rand.NewSource(4))
	player := entity.NewPlayer("ai-user:m:1", "ai-session:m:1", "A", entity.Vector2{}, random)
	enemy := entity.NewPlayer("ai-user:m:2", "ai-session:m:2", "B", entity.Vector2{X: 0.01}, random)
	controller := NewController(player)
	grid := spatial.NewGrid(20)
	if err := grid.Insert(enemy); err != nil {
		t.Fatal(err)
	}
	controller.Step(1, grid, random)
	entity.StepMovement(player, 1)
	if controller.TargetID != enemy.SessionID || math.Abs(player.Position.X-enemy.Position.X) > 1e-9 {
		t.Fatalf("bot pursuit overshot target: %+v", player.Position)
	}
}

func TestControllerQueriesGridWithinSearchRadius(t *testing.T) {
	random := rand.New(rand.NewSource(5))
	bot := entity.NewPlayer("bot", "bot", "Bot", entity.Vector2{}, random)
	bot.Level = 2
	grid := spatial.NewGrid(20)
	near := entity.NewPlayer("near", "near", "Near", entity.Vector2{X: 100}, random)
	near.Characters[0].Health = 0
	far := entity.NewPlayer("far", "far", "Far", entity.Vector2{X: 120.1}, random)
	for _, candidate := range []*entity.Player{near, far} {
		if err := grid.Insert(candidate); err != nil {
			t.Fatal(err)
		}
	}
	item := entity.NewExperiencePackage("xp", entity.Vector2{Y: 120}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 1)
	if err := grid.InsertExperiencePackage(item); err != nil {
		t.Fatal(err)
	}
	controller := NewController(bot)
	controller.Step(1, grid, random)
	if controller.TargetKind != "experience" || controller.TargetID != item.ID {
		t.Fatalf("expected XP at radius boundary, got %s %s", controller.TargetKind, controller.TargetID)
	}
	grid.RemoveExperiencePackage(item.ID)
	near.Characters[0].Health = 100
	controller.Step(2, grid, random)
	if controller.TargetKind != "player" || controller.TargetID != near.SessionID {
		t.Fatalf("expected grid player beyond default detection radius, got %s %s", controller.TargetKind, controller.TargetID)
	}
	grid.Remove(near.SessionID)
	controller.Step(3, grid, random)
	if controller.TargetKind != "wander" {
		t.Fatalf("expected no target outside radius, got %s %s", controller.TargetKind, controller.TargetID)
	}
}

func setLivingCharacters(player *entity.Player, count int) {
	player.Characters = make([]*entity.Character, count)
	for i := range player.Characters {
		player.Characters[i] = &entity.Character{Health: 100}
	}
}

func TestControllerPrefersNearestEligibleOpponent(t *testing.T) {
	for _, enemyCount := range []int{1, 2} {
		t.Run(strconv.Itoa(enemyCount), func(t *testing.T) {
			random := rand.New(rand.NewSource(6))
			bot := entity.NewPlayer("bot", "bot", "Bot", entity.Vector2{}, random)
			bot.Level = 3
			setLivingCharacters(bot, 2)
			bot.Characters = append(bot.Characters, nil, &entity.Character{Health: 0})
			strong := entity.NewPlayer("strong", "strong", "Strong", entity.Vector2{X: 1}, random)
			setLivingCharacters(strong, 3)
			near := entity.NewPlayer("near", "near", "Near", entity.Vector2{X: 40}, random)
			setLivingCharacters(near, enemyCount)
			near.Characters = append(near.Characters, nil, &entity.Character{Health: 0})
			far := entity.NewPlayer("far", "far", "Far", entity.Vector2{X: 50}, random)
			grid := spatial.NewGrid(20)
			for _, candidate := range []*entity.Player{bot, strong, near, far} {
				if err := grid.Insert(candidate); err != nil {
					t.Fatal(err)
				}
			}
			if err := grid.InsertCharacterBox(entity.NewCharacterBox("box", entity.Vector2{X: 2}, entity.WeaponBow)); err != nil {
				t.Fatal(err)
			}
			if err := grid.InsertExperiencePackage(entity.NewExperiencePackage("xp", entity.Vector2{Y: 1}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 1)); err != nil {
				t.Fatal(err)
			}
			controller := NewController(bot)
			controller.Step(1, grid, random)
			if controller.TargetKind != "player" || controller.TargetID != near.SessionID {
				t.Fatalf("expected nearest eligible opponent, got %s %s", controller.TargetKind, controller.TargetID)
			}
		})
	}
}

func TestControllerRetainsOrDropsOpponent(t *testing.T) {
	cases := []struct {
		name   string
		change func(*entity.Player, *entity.Player, *spatial.Grid)
		config Config
		want   string
	}{
		{"below threshold", func(_ *entity.Player, enemy *entity.Player, _ *spatial.Grid) { setLivingCharacters(enemy, 4) }, Config{}, "player"},
		{"at threshold", func(_ *entity.Player, enemy *entity.Player, _ *spatial.Grid) { setLivingCharacters(enemy, 5) }, Config{}, "box"},
		{"above threshold", func(_ *entity.Player, enemy *entity.Player, _ *spatial.Grid) { setLivingCharacters(enemy, 6) }, Config{}, "box"},
		{"AI loses character", func(bot *entity.Player, _ *entity.Player, _ *spatial.Grid) {
			bot.Level = 3
			bot.Characters[0].Health = 0
		}, Config{}, "box"},
		{"level changes threshold", func(bot *entity.Player, enemy *entity.Player, _ *spatial.Grid) {
			bot.Level = 3
			setLivingCharacters(enemy, 4)
		}, Config{}, "experience"},
		{"configured threshold", func(_ *entity.Player, enemy *entity.Player, _ *spatial.Grid) { setLivingCharacters(enemy, 4) }, Config{DisengageDifferenceByMaxCharacters: map[int]int{6: 1}}, "box"},
		{"dead opponent", func(_ *entity.Player, enemy *entity.Player, _ *spatial.Grid) {
			for _, character := range enemy.Characters {
				character.Health = 0
			}
		}, Config{}, "box"},
		{"outside radius", func(_ *entity.Player, enemy *entity.Player, grid *spatial.Grid) {
			enemy.Position.X = 120.1
			if err := grid.Move(enemy); err != nil {
				t.Fatal(err)
			}
		}, Config{}, "box"},
		{"removed opponent", func(_ *entity.Player, enemy *entity.Player, grid *spatial.Grid) { grid.Remove(enemy.SessionID) }, Config{}, "box"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			random := rand.New(rand.NewSource(7))
			bot := entity.NewPlayer("bot", "bot", "Bot", entity.Vector2{}, random)
			bot.Level = 6
			setLivingCharacters(bot, 3)
			enemy := entity.NewPlayer("enemy", "enemy", "Enemy", entity.Vector2{X: 120}, random)
			setLivingCharacters(enemy, 3)
			grid := spatial.NewGrid(20)
			if err := grid.Insert(enemy); err != nil {
				t.Fatal(err)
			}
			if err := grid.InsertCharacterBox(entity.NewCharacterBox("box", entity.Vector2{X: 2}, entity.WeaponBow)); err != nil {
				t.Fatal(err)
			}
			if err := grid.InsertExperiencePackage(entity.NewExperiencePackage("xp", entity.Vector2{Y: 1}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 1)); err != nil {
				t.Fatal(err)
			}
			controller := NewControllerWithConfig(bot, test.config)
			controller.Step(1, grid, random)
			if controller.TargetID != enemy.SessionID {
				t.Fatal("expected to engage opponent at radius boundary")
			}
			test.change(bot, enemy, grid)
			controller.Step(2, grid, random)
			if controller.TargetKind != test.want {
				t.Fatalf("expected %s, got %s %s", test.want, controller.TargetKind, controller.TargetID)
			}
			// An eligible, closer opponent must not interrupt an existing pursuit.
			other := entity.NewPlayer("other", "other", "Other", entity.Vector2{X: 10}, random)
			if err := grid.Insert(other); err != nil {
				t.Fatal(err)
			}
			controller.Step(3, grid, random)
			wantID := other.SessionID
			if test.want == "player" {
				wantID = enemy.SessionID
			}
			if controller.TargetKind != "player" || controller.TargetID != wantID {
				t.Fatalf("expected opponent %s, got %s %s", wantID, controller.TargetKind, controller.TargetID)
			}
		})
	}
}

func TestControllerCollectsOrWandersAgainstStrongerOpponent(t *testing.T) {
	random := rand.New(rand.NewSource(8))
	bot := entity.NewPlayer("bot", "bot", "Bot", entity.Vector2{}, random)
	bot.Level = 3
	enemy := entity.NewPlayer("enemy", "enemy", "Enemy", entity.Vector2{X: 1}, random)
	setLivingCharacters(enemy, 3)
	grid := spatial.NewGrid(20)
	if err := grid.Insert(enemy); err != nil {
		t.Fatal(err)
	}
	if err := grid.InsertCharacterBox(entity.NewCharacterBox("box", entity.Vector2{X: 2}, entity.WeaponBow)); err != nil {
		t.Fatal(err)
	}
	if err := grid.InsertExperiencePackage(entity.NewExperiencePackage("xp", entity.Vector2{Y: 1}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, 1)); err != nil {
		t.Fatal(err)
	}
	controller := NewController(bot)
	setLivingCharacters(enemy, 1)
	controller.Step(0, grid, random)
	if controller.TargetKind != "player" || controller.TargetID != enemy.SessionID {
		t.Fatal("expected to engage equal opponent before it gains characters")
	}
	setLivingCharacters(enemy, 3)
	for tick, want := range []string{"box", "experience", "wander"} {
		controller.Step(int64(tick+1), grid, random)
		if controller.TargetKind != want {
			t.Fatalf("expected %s, got %s", want, controller.TargetKind)
		}
		if tick == 0 {
			grid.RemoveCharacterBox("box")
		}
		if tick == 1 {
			grid.RemoveExperiencePackage("xp")
		}
	}
}
