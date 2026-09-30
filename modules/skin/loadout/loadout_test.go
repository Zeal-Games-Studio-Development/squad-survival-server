package loadout

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/skin/catalog"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func TestLoadBatchesUsersAndFallsBackForMissingInventory(t *testing.T) {
	reader := &fakeReader{objects: []*api.StorageObject{{
		Collection: "player_inventory", Key: "skins", UserId: "user-1",
		Value: `{"items":[{"key":"hair","id":3},{"key":"hair","id":2},{"key":"bow","id":4},{"key":"arrow","id":5},{"key":"chest","id":7},{"key":"beard","id":6},{"key":"eye","id":8},{"key":"helmet","id":9}]}`,
	}}}
	owned, loadErrors := Load(context.Background(), reader, []string{"user-1", "user-2", "user-1"}, catalog.DefaultCatalog())
	if reader.calls != 1 || len(reader.reads) != 2 {
		t.Fatalf("inventory was not batch-read once: calls=%d reads=%d", reader.calls, len(reader.reads))
	}
	if len(loadErrors) != 1 {
		t.Fatalf("expected missing-user fallback error, got %v", loadErrors)
	}
	if got := owned["user-1"][catalog.PartHair]; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("owned hair IDs were not sorted: %v", got)
	}
	if len(owned["user-2"]) != 0 {
		t.Fatalf("missing inventory did not fall back: %v", owned["user-2"])
	}
}

func TestLoadFallsBackForInvalidInventoryAndReadFailure(t *testing.T) {
	reader := &fakeReader{objects: []*api.StorageObject{{UserId: "user-1", Value: `{"items":[{"key":"unknown","id":1}]}`}}}
	owned, loadErrors := Load(context.Background(), reader, []string{"user-1"}, catalog.DefaultCatalog())
	if len(loadErrors) != 1 || len(owned["user-1"]) != 0 {
		t.Fatalf("invalid inventory did not fall back: owned=%v errors=%v", owned, loadErrors)
	}

	reader = &fakeReader{err: errors.New("storage unavailable")}
	owned, loadErrors = Load(context.Background(), reader, []string{"user-1"}, catalog.DefaultCatalog())
	if len(loadErrors) != 1 || len(owned["user-1"]) != 0 {
		t.Fatalf("read failure did not fall back: owned=%v errors=%v", owned, loadErrors)
	}
}

func TestRandomSkinUsesOwnedAppearanceAndWeaponParts(t *testing.T) {
	owned := Owned{
		catalog.PartHair: {2}, catalog.PartBeard: {3}, catalog.PartChest: {4},
		catalog.PartEye: {5}, catalog.PartHelmet: {6}, catalog.PartBow: {7}, catalog.PartArrow: {8},
	}
	skin := RandomSkin(rand.New(rand.NewSource(1)), owned, entity.WeaponBow)
	if !oneOf(skin.HairID, 0, 2) || !oneOf(skin.BeardID, 0, 3) || !oneOf(skin.ChestID, 0, 4) ||
		!oneOf(skin.EyeID, 0, 5) || !oneOf(skin.HelmetID, 0, 6) || skin.WeaponID != 7 || skin.ProjectileID != 8 {
		t.Fatalf("unexpected bow skin: %+v", skin)
	}
}

func TestRandomSkinDefaultsMissingPartsAndLeavesUnknownWeaponUnset(t *testing.T) {
	skin := RandomSkin(rand.New(rand.NewSource(1)), nil, entity.WeaponType("unknown"))
	if !oneOf(skin.HairID, 0, 1) || !oneOf(skin.BeardID, 0, 1) || !oneOf(skin.ChestID, 0, 1) ||
		!oneOf(skin.EyeID, 0, 1) || !oneOf(skin.HelmetID, 0, 1) {
		t.Fatalf("appearance did not use ID 0/1 fallback: %+v", skin)
	}
	if skin.WeaponID != 0 || skin.ProjectileID != 0 {
		t.Fatalf("unknown weapon unexpectedly received a catalog skin: %+v", skin)
	}
}

func TestRandomSkinCanSelectNoAppearance(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	owned := Owned{catalog.PartHair: {2}}
	seenNone, seenOwned := false, false
	for range 100 {
		hairID := RandomSkin(random, owned, entity.WeaponSword).HairID
		seenNone = seenNone || hairID == 0
		seenOwned = seenOwned || hairID == 2
	}
	if !seenNone || !seenOwned {
		t.Fatalf("appearance random did not include both no-skin and owned skin: none=%v owned=%v", seenNone, seenOwned)
	}
}

func oneOf(value int, allowed ...int) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

type fakeReader struct {
	objects []*api.StorageObject
	err     error
	calls   int
	reads   []*runtime.StorageRead
}

func (r *fakeReader) StorageRead(_ context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	r.calls++
	r.reads = append([]*runtime.StorageRead(nil), reads...)
	return r.objects, r.err
}
