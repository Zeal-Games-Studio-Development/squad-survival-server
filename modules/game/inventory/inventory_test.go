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
	objects       map[string]*api.StorageObject
	writes        int
	rejectVersion bool
}

func (m *memoryStore) StorageRead(_ context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	if object := m.objects[reads[0].UserID]; object != nil {
		return []*api.StorageObject{object}, nil
	}
	return nil, nil
}

func (m *memoryStore) StorageWrite(_ context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error) {
	write := writes[0]
	if m.rejectVersion {
		return nil, runtime.ErrStorageRejectedVersion
	}
	if m.objects == nil {
		m.objects = make(map[string]*api.StorageObject)
	}
	if existing := m.objects[write.UserID]; existing != nil && write.Version != existing.Version {
		return nil, ErrVersionConflict
	}
	if write.PermissionRead != 1 || write.PermissionWrite != 0 {
		return nil, errors.New("storage object must be owner-readable and server-writable only")
	}
	m.writes++
	version := string(rune('0' + m.writes))
	m.objects[write.UserID] = &api.StorageObject{Value: write.Value, Version: version}
	return []*api.StorageObjectAck{{Version: version}}, nil
}

func TestInitialInventoryAndLoadout(t *testing.T) {
	service := DefaultService()
	store := &memoryStore{}
	if err := service.InitializeNewAccount(context.Background(), store, "new-user"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Load(context.Background(), store, "new-user")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.WeaponIDs) != 8 || len(snapshot.SquadLoadout) != 8 {
		t.Fatalf("unexpected initial inventory: %+v", snapshot)
	}
	if _, exists := snapshot.SquadLoadout["wand"]; exists {
		t.Fatal("new account received removed wand slot")
	}
	for weaponType, id := range map[string]string{
		"bow": "archer", "staff": "novice_mage", "spear": "farmer", "sword": "swordman",
		"axe": "axeman", "blunt": "bruiser", "crossbow": "hunter", "shield": "shieldbearer",
	} {
		if snapshot.SquadLoadout[weaponType] != id {
			t.Fatalf("missing loadout slot for %s: %+v", weaponType, snapshot.SquadLoadout)
		}
	}
	if _, err := service.Load(context.Background(), store, "old-user"); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("unexpected old account fallback: %v", err)
	}
	legacy := snapshot.Data
	legacy.WeaponIDs = append(append([]string(nil), snapshot.WeaponIDs...), "wand")
	legacy.SquadLoadout = copyLoadout(snapshot.SquadLoadout)
	legacy.SquadLoadout["wand"] = "wand"
	value, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	store.objects["legacy-user"] = &api.StorageObject{Value: string(value), Version: "legacy"}
	if _, err := service.Load(context.Background(), store, "legacy-user"); !errors.Is(err, ErrInvalidLoadout) {
		t.Fatalf("legacy wand inventory was not rejected: %v", err)
	}
}

func TestSetLoadoutValidatesOwnershipTypeCompletenessAndVersion(t *testing.T) {
	weapons := entity.DefaultWeaponCatalog()
	variant := weapons[0]
	variant.ID = "bow_rare"
	service, err := NewService(append(weapons, variant), []string{"archer", "novice_mage", "farmer", "swordman", "axeman", "bruiser", "hunter", "shieldbearer", "bow_rare"})
	if err == nil {
		t.Fatal("initial config must not contain duplicate type")
	}
	service, err = NewService(append(weapons, variant), []string{"archer", "novice_mage", "farmer", "swordman", "axeman", "bruiser", "hunter", "shieldbearer"})
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	if err := service.InitializeNewAccount(context.Background(), store, "user"); err != nil {
		t.Fatal(err)
	}
	current, err := service.Load(context.Background(), store, "user")
	if err != nil {
		t.Fatal(err)
	}
	withWand := current.Data
	withWand.WeaponIDs = append(append([]string(nil), current.WeaponIDs...), "wand")
	withWand.SquadLoadout = copyLoadout(current.SquadLoadout)
	withWand.SquadLoadout["wand"] = "wand"
	if err := service.Validate(withWand); !errors.Is(err, ErrInvalidLoadout) {
		t.Fatalf("removed wand loadout was accepted: %v", err)
	}
	wrongType := copyLoadout(current.SquadLoadout)
	wrongType["bow"] = "novice_mage"
	if _, err := service.SetLoadout(context.Background(), store, "user", current.Version, wrongType); !errors.Is(err, ErrInvalidLoadout) {
		t.Fatalf("wrong type: %v", err)
	}
	missing := copyLoadout(current.SquadLoadout)
	delete(missing, "bow")
	if _, err := service.SetLoadout(context.Background(), store, "user", current.Version, missing); !errors.Is(err, ErrInvalidLoadout) {
		t.Fatalf("missing slot: %v", err)
	}
	unowned := copyLoadout(current.SquadLoadout)
	unowned["bow"] = "bow_rare"
	if _, err := service.SetLoadout(context.Background(), store, "user", current.Version, unowned); !errors.Is(err, ErrInvalidLoadout) {
		t.Fatalf("unowned id: %v", err)
	}
	withVariant := current.Data
	withVariant.WeaponIDs = append(append([]string(nil), current.WeaponIDs...), "bow_rare")
	withVariant.SquadLoadout = copyLoadout(current.SquadLoadout)
	withVariant.SquadLoadout["bow"] = "bow_rare"
	if err := service.Validate(withVariant); err != nil {
		t.Fatalf("owned variant of the same type was rejected: %v", err)
	}
	if _, err := service.SetLoadout(context.Background(), store, "user", "old", current.SquadLoadout); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale version: %v", err)
	}
	updated, err := service.SetLoadout(context.Background(), store, "user", current.Version, current.SquadLoadout)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version == current.Version || !reflect.DeepEqual(updated.SquadLoadout, current.SquadLoadout) {
		t.Fatalf("unexpected update: %+v", updated)
	}
	if store.writes != 2 {
		t.Fatalf("invalid writes occurred: %d", store.writes)
	}
	store.rejectVersion = true
	if _, err := service.SetLoadout(context.Background(), store, "user", updated.Version, updated.SquadLoadout); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("storage CAS conflict not reported: %v", err)
	}
}

func TestLoadRejectsObsoleteWeaponIDs(t *testing.T) {
	service := DefaultService()
	for _, old := range []struct {
		id         string
		weaponType string
		index      int
	}{
		{id: "bow", weaponType: "bow", index: 0},
		{id: "acher", weaponType: "bow", index: 0},
		{id: "crossbowman", weaponType: "crossbow", index: 6},
	} {
		t.Run(old.id, func(t *testing.T) {
			data, err := service.InitialData()
			if err != nil {
				t.Fatal(err)
			}
			data.WeaponIDs[old.index] = old.id
			data.SquadLoadout[old.weaponType] = old.id
			value, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			store := &memoryStore{objects: map[string]*api.StorageObject{
				"user": {Value: string(value), Version: "legacy"},
			}}
			if _, err := service.Load(context.Background(), store, "user"); !errors.Is(err, ErrInvalidLoadout) {
				t.Fatalf("obsolete ID %q was accepted: %v", old.id, err)
			}
		})
	}
}

func TestRPCRejectsInvalidPayload(t *testing.T) {
	service := DefaultService()
	store := &rpcStore{memoryStore: memoryStore{}}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user")
	if err := service.InitializeNewAccount(ctx, store, "user"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetLoadoutRPC(ctx, nil, nil, store, `{`); err == nil {
		t.Fatal("malformed payload accepted")
	}
	response, err := service.GetRPC(ctx, nil, nil, store, "")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(response), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Version == "" || len(snapshot.WeaponIDs) != 8 {
		t.Fatalf("invalid RPC response: %+v", snapshot)
	}
}

type rpcStore struct {
	runtime.NakamaModule
	memoryStore
}

func (r *rpcStore) StorageRead(ctx context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	return r.memoryStore.StorageRead(ctx, reads)
}

func (r *rpcStore) StorageWrite(ctx context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error) {
	return r.memoryStore.StorageWrite(ctx, writes)
}

func copyLoadout(source map[string]string) map[string]string {
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
