package inventory

import (
	"context"
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
	want := []string{"archer", "novice_mage", "farmer", "swordman", "axeman", "bruiser", "hunter", "shieldbearer"}
	if !reflect.DeepEqual(data.WeaponIDs, want) {
		t.Fatalf("unexpected initial inventory: %+v", data)
	}
	if store.objects["new-user"].Value != `{"weapon_ids":["archer","novice_mage","farmer","swordman","axeman","bruiser","hunter","shieldbearer"]}` {
		t.Fatalf("unexpected storage shape: %s", store.objects["new-user"].Value)
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

func TestInitialCharactersRequireOneWeaponPerType(t *testing.T) {
	weapons := entity.DefaultWeaponCatalog()
	initial := []string{"archer", "novice_mage", "farmer", "swordman", "axeman", "bruiser", "hunter", "shieldbearer"}
	for _, ids := range [][]string{initial[:7], append(append([]string(nil), initial[:7]...), "archer_uncommon"), append(append([]string(nil), initial[:7]...), "wand")} {
		if _, err := NewService(weapons, ids); err == nil {
			t.Fatalf("invalid initial list accepted: %v", ids)
		}
	}
	service, err := NewService(weapons, initial)
	if err != nil {
		t.Fatal(err)
	}
	data, err := service.InitialData()
	if err != nil || !reflect.DeepEqual(data.WeaponIDs, initial) {
		t.Fatalf("wrong bootstrap data: %+v %v", data, err)
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
