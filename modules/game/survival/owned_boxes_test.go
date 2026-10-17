package survival

import (
	"context"
	"math/rand"
	"testing"

	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/proto"
	"squad-survival-be/modules/game/core/characterbox"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/spatial"
	"squad-survival-be/modules/game/core/system"
	"squad-survival-be/modules/game/inventory"
	"squad-survival-be/modules/game/matchregistry"
)

func ownedBoxState() *State {
	return &State{
		Phase: PhaseWaiting, Players: map[string]*entity.Player{},
		Presences: map[string]runtime.Presence{}, Reservations: map[string]int64{},
		SpatialGrid: spatial.NewGrid(spatialCellSize), CharacterBoxes: map[string]*entity.CharacterBox{},
		WeaponCatalog: entity.DefaultWeaponCatalog(), OwnedWeapons: map[string][]entity.Weapon{},
		random: rand.New(rand.NewSource(41)),
	}
}

func TestOwnedInventoryControlsStarterSnapshotAndLeave(t *testing.T) {
	ctx := context.Background()
	state := ownedBoxState()
	variant := state.WeaponCatalog[0]
	variant.ID = "owned_common_variant"
	state.WeaponCatalog = append(state.WeaponCatalog, variant)
	// A different bootstrap list must not affect gameplay selection.
	initial := []string{"archer_uncommon", "novice_mage", "farmer", "swordman", "axeman", "bruiser", "hunter", "shieldbearer"}
	service, err := inventory.NewService(state.WeaponCatalog, initial)
	if err != nil {
		t.Fatal(err)
	}
	match := &Match{registry: matchregistry.New(), inventory: service}
	presence := testPresence{userID: "owned-user", sessionID: "owned-session"}
	uncommon := inventoryTestModule{value: `{"weapon_ids":["archer_uncommon","archer_rare"]}`}
	if _, allowed, _ := match.MatchJoinAttempt(ctx, nil, nil, uncommon, nil, 0, state, presence, nil); allowed {
		t.Fatal("joined without common")
	}
	module := inventoryTestModule{value: `{"weapon_ids":["owned_common_variant","archer_rare"]}`}
	if _, allowed, reason := match.MatchJoinAttempt(ctx, nil, nil, module, nil, 0, state, presence, nil); !allowed {
		t.Fatalf("valid common rejected: %s", reason)
	}
	dispatcher := &testDispatcher{}
	// Inventory is checked again when actual join happens.
	match.MatchJoin(ctx, nil, nil, uncommon, dispatcher, 0, state, []runtime.Presence{presence})
	if state.Players[presence.sessionID] != nil {
		t.Fatal("actual join accepted inventory without common")
	}
	match.MatchJoin(ctx, nil, nil, module, dispatcher, 1, state, []runtime.Presence{presence})
	player := state.Players[presence.sessionID]
	if player == nil || player.Characters[0].Weapon.ID != variant.ID {
		t.Fatalf("starter not owned common: %+v", player)
	}
	module.value = `{"weapon_ids":["archer"]}`
	box := entity.NewCharacterBox("owned-box", player.Position, entity.WeaponBow, entity.RarityCommon)
	matches := state.weaponsForBox(box, presence.sessionID)
	if len(matches) != 1 || matches[0].ID != variant.ID {
		t.Fatalf("snapshot changed with storage: %+v", matches)
	}
	match.MatchLeave(ctx, nil, nil, nil, dispatcher, 2, state, []runtime.Presence{presence})
	if _, exists := state.OwnedWeapons[presence.sessionID]; exists {
		t.Fatal("inventory snapshot survived leave")
	}
}

func TestBoxOwnershipClaimAndAward(t *testing.T) {
	state := ownedBoxState()
	common := state.WeaponCatalog[0]
	near := entity.NewPlayerWithWeapon("near", "near", "Near", entity.Vector2{}, common)
	far := entity.NewPlayerWithWeapon("far", "far", "Far", entity.Vector2{X: 0.5}, common)
	near.Level = 2
	far.Level = 2
	state.Players[near.SessionID] = near
	state.Players[far.SessionID] = far
	// Nearest player owns only another type. The farther eligible player must win.
	state.OwnedWeapons[near.SessionID] = []entity.Weapon{state.WeaponCatalog[1]}
	variant := common
	variant.ID = "owned_bow"
	variant.Damage = 99
	state.WeaponCatalog = append(state.WeaponCatalog, variant)
	state.OwnedWeapons[far.SessionID] = []entity.Weapon{variant}
	box := entity.NewCharacterBox("box", entity.Vector2{}, entity.WeaponBow, entity.RarityCommon)
	state.CharacterBoxes[box.ID] = box
	if err := state.SpatialGrid.InsertCharacterBox(box); err != nil {
		t.Fatal(err)
	}
	events := state.collectCharacterBoxes(1, nil)
	if len(events) != 1 || events[0].ClaimantSessionID != far.SessionID {
		t.Fatalf("unowned player blocked eligible player: %+v", events)
	}
	// Invalidated eligibility cancels an existing claim without consuming the box.
	state.OwnedWeapons[far.SessionID] = nil
	events = state.collectCharacterBoxes(2, nil)
	if len(events) != 1 || events[0].Type != system.CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_CANCELLED || state.CharacterBoxes[box.ID] == nil {
		t.Fatalf("invalid claim survived: %+v", events)
	}
	state.OwnedWeapons[far.SessionID] = []entity.Weapon{variant}
	state.collectCharacterBoxes(3, nil)
	state.collectCharacterBoxes(13, nil)
	if far.CharacterCount() != 2 || far.Characters[1].Weapon.ID != variant.ID || far.Characters[1].Damage != 99 || near.CharacterCount() != 1 || state.CharacterBoxes[box.ID] != nil {
		t.Fatal("box did not award exact owned variant")
	}
	// Rarity mismatch does not yield an eligible weapon.
	box.Value.Rarity = "rare"
	if len(state.weaponsForBox(box, far.SessionID)) != 0 {
		t.Fatal("common satisfied rare box")
	}
}

func TestOwnedBoxSpawnPoolAndJoinRarity(t *testing.T) {
	state := ownedBoxState()
	if len(state.spawnCharacterBoxes(4)) != 0 {
		t.Fatal("spawned without participants")
	}
	player := entity.NewPlayerWithWeapon("pool", "pool", "Pool", entity.Vector2{X: 100}, state.WeaponCatalog[0])
	state.Players[player.SessionID] = player
	state.OwnedWeapons[player.SessionID] = characterbox.Owned(state.WeaponCatalog, []string{"archer_rare", "novice_mage_uncommon"})
	if len(state.spawnCharacterBoxes(4)) != 0 {
		t.Fatal("spawned from non-common inventory")
	}
	state.OwnedWeapons[player.SessionID] = characterbox.Owned(state.WeaponCatalog, []string{"archer", "archer_rare"})
	if len(state.spawnCharacterBoxes(4)) != 4 {
		t.Fatal("failed to spawn owned common")
	}
	for _, box := range state.CharacterBoxes {
		if box.Rarity() != entity.RarityCommon || box.WeaponType() != entity.WeaponBow {
			t.Fatalf("wrong box: %+v", box)
		}
	}
	dispatcher := &testDispatcher{}
	state.sendCurrentCharacterBoxes(nil, dispatcher, 1, testPresence{userID: "snapshot", sessionID: "snapshot"})
	var batch system.CharacterBoxStateBatch
	if len(dispatcher.broadcasts) != 1 {
		t.Fatal("missing snapshot")
	}
	if err := proto.Unmarshal(dispatcher.broadcasts[0].data, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 4 {
		t.Fatalf("wrong snapshot size: %d", len(batch.Events))
	}
	for _, event := range batch.Events {
		if event.Value.Rarity != "common" {
			t.Fatal("snapshot lost rarity")
		}
	}
	// Eliminated participants no longer contribute new spawn types; existing boxes stay.
	player.Characters = nil
	if len(state.spawnCharacterBoxes(5)) != 0 || len(state.CharacterBoxes) != 4 {
		t.Fatal("eliminated participant contributed to spawn pool")
	}
}
