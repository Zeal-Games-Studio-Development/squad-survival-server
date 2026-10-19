package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"squad-survival-be/modules/game/core/entity"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type memoryStore struct {
	objects map[string]*api.StorageObject
}

func (m *memoryStore) StorageRead(_ context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	if object := m.objects[reads[0].UserID]; object != nil {
		return []*api.StorageObject{object}, nil
	}
	return nil, nil
}

func (m *memoryStore) StorageWrite(_ context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error) {
	write := writes[0]
	if m.objects == nil {
		m.objects = make(map[string]*api.StorageObject)
	}
	if write.PermissionRead != 1 || write.PermissionWrite != 0 {
		return nil, errors.New("storage object must be owner-readable and server-writable only")
	}
	m.objects[write.UserID] = &api.StorageObject{Value: write.Value, Version: "v1"}
	return []*api.StorageObjectAck{{Version: "v1"}}, nil
}

func TestInitialInventoryStoresOnlyWeaponIDs(t *testing.T) {
	service := DefaultService()
	store := &memoryStore{}
	if err := service.InitializeNewAccount(context.Background(), store, "new-user"); err != nil {
		t.Fatal(err)
	}
	data, err := service.Load(context.Background(), store, "new-user")
	if err != nil {
		t.Fatal(err)
	}
	bases := []string{"archer", "mage", "farmer", "swordman", "axeman", "bruiser", "hunter", "shieldbearer"}
	var want []string
	for _, suffix := range []string{"", "_uncommon", "_rare"} {
		for _, name := range bases {
			want = append(want, name+suffix)
		}
	}
	if !reflect.DeepEqual(data.WeaponIDs, want) {
		t.Fatalf("unexpected initial inventory: %+v", data)
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal([]byte(store.objects["new-user"].Value), &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored["weapon_ids"] == nil {
		t.Fatalf("unexpected storage shape: %s", store.objects["new-user"].Value)
	}
	var storedIDs []string
	if err := json.Unmarshal(stored["weapon_ids"], &storedIDs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(storedIDs, want) {
		t.Fatalf("wrong stored IDs: %v", storedIDs)
	}
	catalog := entity.DefaultWeaponCatalog()
	if len(data.WeaponIDs) != 24 || len(catalog) != 24 {
		t.Fatal("default inventory must cover all 24 catalog weapons")
	}
	counts := map[entity.Rarity]int{}
	for _, weapon := range catalog {
		if !containsID(data.WeaponIDs, weapon.ID) {
			t.Fatalf("default inventory missing %s", weapon.ID)
		}
		counts[weapon.Rarity]++
	}
	for _, rarity := range []entity.Rarity{entity.RarityCommon, entity.RarityUncommon, entity.RarityRare} {
		if counts[rarity] != 8 {
			t.Fatalf("expected eight %s weapons, got %d", rarity, counts[rarity])
		}
	}
	if _, err := service.Load(context.Background(), store, "missing-user"); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("missing inventory: %v", err)
	}
}

func TestInventoryValidationAllowsPartialOwnership(t *testing.T) {
	service := DefaultService()
	if err := service.Validate(Data{WeaponIDs: []string{"archer"}}); err != nil {
		t.Fatalf("single owned weapon rejected: %v", err)
	}
	for _, ids := range [][]string{nil, {}, {"archer", "archer"}, {"archer", "wand"}} {
		if err := service.Validate(Data{WeaponIDs: ids}); !errors.Is(err, ErrInvalidInventory) {
			t.Fatalf("invalid ids %v accepted: %v", ids, err)
		}
	}
}

func TestInitialInventoryAllowsPartialAndMultipleWeaponsPerType(t *testing.T) {
	weapons := entity.DefaultWeaponCatalog()
	for _, initial := range [][]string{
		{"archer"},
		{"archer", "archer_uncommon", "archer_rare"},
		{"archer", "mage", "farmer", "swordman", "axeman", "bruiser", "hunter", "shieldbearer"},
	} {
		service, err := NewService(weapons, initial)
		if err != nil {
			t.Fatalf("valid initial inventory rejected: %v: %v", initial, err)
		}
		data, err := service.InitialData()
		if err != nil || !reflect.DeepEqual(data.WeaponIDs, initial) {
			t.Fatalf("wrong bootstrap data: %+v %v", data, err)
		}
	}
	for _, ids := range [][]string{nil, {}, {"archer", "archer"}, {"archer", "missing"}} {
		if _, err := NewService(weapons, ids); !errors.Is(err, ErrInvalidInventory) {
			t.Fatalf("invalid initial list %v: %v", ids, err)
		}
	}
}

func TestCatalogExpansionDoesNotGrantAdditionalInitialWeapons(t *testing.T) {
	weapons := entity.DefaultWeaponCatalog()
	extra := weapons[0]
	extra.ID = "future_common"
	weapons = append(weapons, extra)
	service, err := NewService(weapons, []string{"archer"})
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	if err := service.InitializeNewAccount(context.Background(), store, "new-user"); err != nil {
		t.Fatal(err)
	}
	data, err := service.Load(context.Background(), store, "new-user")
	if err != nil || !reflect.DeepEqual(data.WeaponIDs, []string{"archer"}) {
		t.Fatalf("catalog expansion granted unconfigured weapons: %+v %v", data, err)
	}
}

func containsID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func TestCatalogServiceDoesNotRequireBootstrapInventory(t *testing.T) {
	service, err := NewCatalogService(entity.DefaultWeaponCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Validate(Data{WeaponIDs: []string{"archer_rare"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InitialData(); err == nil {
		t.Fatal("catalog-only service allowed account initialization")
	}
}
