package royale

import (
	"math/rand"
	"testing"

	"google.golang.org/protobuf/proto"
	"squad-survival-be/modules/game/core/characterbox"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/system"
)

func TestRankedBoxSpawnAndRefill(t *testing.T) {
	state := ownedBoxState()
	player := entity.NewPlayerWithWeapon("owner", "owner", "Owner", entity.Vector2{X: 100}, state.WeaponCatalog[0])
	state.Players[player.SessionID] = player
	state.OwnedWeapons[player.SessionID] = characterbox.Owned(state.WeaponCatalog, []string{"archer", "novice_mage_uncommon", "bruiser_rare"})
	events := state.spawnCharacterBoxes(60)
	assertRankedBoxEvents(t, state, player.SessionID, events, [3]int{42, 12, 6})
	// Remove only common boxes: refill must allocate the new 20, not rebalance the map.
	removed := 0
	for id, box := range state.CharacterBoxes {
		if box.Rarity() == entity.RarityCommon && removed < 20 {
			state.SpatialGrid.RemoveCharacterBox(id)
			delete(state.CharacterBoxes, id)
			removed++
		}
	}
	existing := map[string]entity.CharacterBoxValue{}
	for id, box := range state.CharacterBoxes {
		existing[id] = entity.CharacterBoxValue{WeaponType: box.Value.WeaponType, Rarity: box.Value.Rarity}
	}
	events = state.spawnCharacterBoxes(60)
	assertRankedBoxEvents(t, state, player.SessionID, events, [3]int{14, 4, 2})
	for id, value := range existing {
		box := state.CharacterBoxes[id]
		if box == nil || box.Value.WeaponType != value.WeaponType || box.Value.Rarity != value.Rarity {
			t.Fatal("refill changed existing box")
		}
	}
	if len(state.spawnCharacterBoxes(60)) != 0 || len(state.spawnCharacterBoxes(59)) != 0 {
		t.Fatal("spawned above target")
	}
	counts := [3]int{}
	for _, box := range state.CharacterBoxes {
		switch box.Rarity() {
		case entity.RarityCommon:
			counts[0]++
		case entity.RarityUncommon:
			counts[1]++
		case entity.RarityRare:
			counts[2]++
		}
	}
	if counts != [3]int{36, 16, 8} {
		t.Fatalf("refill rebalanced existing boxes: %v", counts)
	}
	dispatcher := &testDispatcher{}
	state.sendCurrentCharacterBoxes(nil, dispatcher, 5, testPresence{userID: "viewer", sessionID: "viewer"})
	var batch system.CharacterBoxStateBatch
	if len(dispatcher.broadcasts) != 1 {
		t.Fatal("missing snapshot")
	}
	if err := proto.Unmarshal(dispatcher.broadcasts[0].data, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 60 {
		t.Fatalf("snapshot has %d boxes", len(batch.Events))
	}
	for _, event := range batch.Events {
		if event.Value.Rarity != state.CharacterBoxes[event.BoxId].Value.Rarity {
			t.Fatal("snapshot lost actual rarity")
		}
	}
}

func assertRankedBoxEvents(t *testing.T, state *State, sessionID string, events []system.CharacterBoxEvent, want [3]int) {
	t.Helper()
	got := [3]int{}
	for _, event := range events {
		box := event.Box
		if len(state.weaponsForBox(box, sessionID)) == 0 {
			t.Fatalf("unowned spawn: %+v", box)
		}
		switch box.Rarity() {
		case entity.RarityCommon:
			got[0]++
		case entity.RarityUncommon:
			got[1]++
		case entity.RarityRare:
			got[2]++
		default:
			t.Fatal("invalid rarity")
		}
	}
	if got != want {
		t.Fatalf("got %v boxes, want %v", got, want)
	}
	data, err := system.EncodeCharacterBoxStateBatch(1, events)
	if err != nil {
		t.Fatal(err)
	}
	var batch system.CharacterBoxStateBatch
	if err := proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != len(events) {
		t.Fatal("spawn events lost")
	}
	for _, event := range batch.Events {
		if event.Value.Rarity != state.CharacterBoxes[event.BoxId].Value.Rarity {
			t.Fatal("spawn event lost rarity")
		}
	}
}

func TestRankedBoxesAwardOnlyMatchingOwnedWeapon(t *testing.T) {
	for _, id := range []string{"archer_uncommon", "archer_rare"} {
		t.Run(id, func(t *testing.T) {
			state := ownedBoxState()
			weapon := characterbox.Owned(state.WeaponCatalog, []string{id})[0]
			near := entity.NewPlayerWithWeapon("near", "near", "Near", entity.Vector2{}, state.WeaponCatalog[0])
			far := entity.NewPlayerWithWeapon("far", "far", "Far", entity.Vector2{X: 0.5}, state.WeaponCatalog[0])
			near.Level = 2
			far.Level = 2
			state.Players[near.SessionID] = near
			state.Players[far.SessionID] = far
			state.OwnedWeapons[near.SessionID] = commonTestWeapons()
			state.OwnedWeapons[far.SessionID] = []entity.Weapon{weapon}
			box := entity.NewCharacterBox("ranked", entity.Vector2{}, weapon.Type, weapon.Rarity)
			state.CharacterBoxes[box.ID] = box
			if err := state.SpatialGrid.InsertCharacterBox(box); err != nil {
				t.Fatal(err)
			}
			events := state.collectCharacterBoxes(1, nil)
			if len(events) != 1 || events[0].ClaimantSessionID != far.SessionID {
				t.Fatal("player without matching rarity blocked eligible player")
			}
			events = state.collectCharacterBoxes(11, nil)
			if len(events) != 1 || events[0].Type != system.CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_DESPAWNED || near.CharacterCount() != 1 || far.CharacterCount() != 2 || far.Characters[1].Weapon != weapon {
				t.Fatal("wrong character awarded for ranked box")
			}
		})
	}
}

// Repeating the same point lets the first placement succeed and blocks every later attempt.
type fixedBoxSpawnSource struct{}

func (fixedBoxSpawnSource) Int63() int64 { return 0x456789abcdef1234 }
func (fixedBoxSpawnSource) Seed(int64)   {}

func TestRankedBoxPlacementStopsWithoutRemovingPlacedBoxes(t *testing.T) {
	state := ownedBoxState()
	player := entity.NewPlayerWithWeapon("owner", "owner", "Owner", entity.Vector2{X: 100}, state.WeaponCatalog[0])
	state.Players[player.SessionID] = player
	state.OwnedWeapons[player.SessionID] = append([]entity.Weapon(nil), state.WeaponCatalog...)
	state.random = rand.New(fixedBoxSpawnSource{})
	events := state.spawnCharacterBoxes(10)
	if len(events) != 1 || len(state.CharacterBoxes) != 1 {
		t.Fatalf("partial spawn not preserved: events=%d boxes=%d", len(events), len(state.CharacterBoxes))
	}
	placed := events[0].Box
	if !placed.Rarity().Valid() || len(state.weaponsForBox(placed, player.SessionID)) == 0 {
		t.Fatal("partial spawn has invalid type/rarity")
	}
	if len(state.spawnCharacterBoxes(10)) != 0 || state.CharacterBoxes[placed.ID] != placed {
		t.Fatal("exhausted spawn changed existing box")
	}
}

func TestBotsContributeAllBoxRarities(t *testing.T) {
	_, state := newAITestMatch(t)
	counts := map[entity.Rarity]int{}
	for _, box := range state.CharacterBoxes {
		counts[box.Rarity()]++
		eligible := false
		for sessionID := range state.AIControllers {
			if len(state.weaponsForBox(box, sessionID)) > 0 {
				eligible = true
				break
			}
		}
		if !eligible {
			t.Fatal("box has no eligible bot")
		}
	}
	for _, rarity := range []entity.Rarity{entity.RarityCommon, entity.RarityUncommon, entity.RarityRare} {
		if counts[rarity] == 0 {
			t.Fatalf("bots did not contribute %s boxes", rarity)
		}
	}
}
