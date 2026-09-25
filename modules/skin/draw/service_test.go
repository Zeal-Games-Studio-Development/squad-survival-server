package draw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	skincatalog "squad-survival-be/modules/skin/catalog"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

const testRequestID = "123e4567-e89b-42d3-a456-426614174000"

type zeroRandom struct{}

func (zeroRandom) Intn(int) (int, error) { return 0, nil }

type fakeStore struct {
	object          *api.StorageObject
	wallet          map[string]int64
	multiCalls      int
	readCalls       int
	failMultiCount  int
	lastWrite       *runtime.StorageWrite
	lastWallet      *runtime.WalletUpdate
	versionSequence int
}

func newFakeStore(gems int64) *fakeStore {
	return &fakeStore{wallet: map[string]int64{GemCurrency: gems}, versionSequence: 1}
}

func (s *fakeStore) AccountGetId(context.Context, string) (*api.Account, error) {
	wallet, _ := json.Marshal(s.wallet)
	return &api.Account{Wallet: string(wallet)}, nil
}

func (s *fakeStore) StorageRead(context.Context, []*runtime.StorageRead) ([]*api.StorageObject, error) {
	s.readCalls++
	if s.object == nil {
		return nil, nil
	}
	return []*api.StorageObject{{
		Collection: s.object.Collection, Key: s.object.Key, UserId: s.object.UserId,
		Value: s.object.Value, Version: s.object.Version,
		PermissionRead: s.object.PermissionRead, PermissionWrite: s.object.PermissionWrite,
	}}, nil
}

func (s *fakeStore) MultiUpdate(_ context.Context, _ []*runtime.AccountUpdate, writes []*runtime.StorageWrite, _ []*runtime.StorageDelete, walletUpdates []*runtime.WalletUpdate, _ bool) ([]*api.StorageObjectAck, []*runtime.WalletUpdateResult, error) {
	s.multiCalls++
	if s.failMultiCount > 0 {
		s.failMultiCount--
		return nil, nil, errors.New("storage version conflict")
	}
	if len(writes) != 1 || len(walletUpdates) != 1 {
		return nil, nil, errors.New("unexpected transaction shape")
	}
	write := writes[0]
	update := walletUpdates[0]
	if s.object == nil {
		if write.Version != "*" {
			return nil, nil, errors.New("new object was not create-only")
		}
	} else if write.Version != s.object.Version {
		return nil, nil, errors.New("storage version conflict")
	}
	previous := cloneWallet(s.wallet)
	for currency, amount := range update.Changeset {
		if s.wallet[currency]+amount < 0 {
			return nil, nil, &runtime.WalletNegativeError{UserID: update.UserID, Path: currency, Current: s.wallet[currency], Amount: amount}
		}
	}
	for currency, amount := range update.Changeset {
		s.wallet[currency] += amount
	}
	s.versionSequence++
	version := fmt.Sprintf("v%d", s.versionSequence)
	s.object = &api.StorageObject{
		Collection: write.Collection, Key: write.Key, UserId: write.UserID,
		Value: write.Value, Version: version, PermissionRead: int32(write.PermissionRead), PermissionWrite: int32(write.PermissionWrite),
	}
	s.lastWrite = write
	s.lastWallet = update
	return []*api.StorageObjectAck{{Collection: write.Collection, Key: write.Key, UserId: write.UserID, Version: version}}, []*runtime.WalletUpdateResult{{
		UserID: update.UserID, Previous: previous, Updated: cloneWallet(s.wallet),
	}}, nil
}

func cloneWallet(wallet map[string]int64) map[string]int64 {
	copy := make(map[string]int64, len(wallet))
	for key, value := range wallet {
		copy[key] = value
	}
	return copy
}

func testService(t *testing.T, itemCatalog *skincatalog.Catalog) *Service {
	t.Helper()
	service, err := NewServiceWithDependencies(itemCatalog, zeroRandom{}, func() time.Time {
		return time.Unix(1_790_265_600, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestDrawSingleAtomicallyChargesAndStoresItem(t *testing.T) {
	service := testService(t, skincatalog.DefaultCatalog())
	store := newFakeStore(1_000)

	response, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.ItemIDs, []string{"r1"}) || response.GemSpent != 100 || response.GemBalanceAfter != 900 || response.Replayed {
		t.Fatalf("unexpected response: %#v", response)
	}
	if store.multiCalls != 1 || store.lastWrite.Collection != InventoryCollection || store.lastWrite.Key != InventoryKey {
		t.Fatalf("unexpected storage transaction: %#v", store.lastWrite)
	}
	if store.lastWrite.PermissionRead != 1 || store.lastWrite.PermissionWrite != 0 || store.lastWrite.Version != "*" {
		t.Fatalf("unexpected storage write controls: %#v", store.lastWrite)
	}
	if store.lastWallet.Changeset[GemCurrency] != -100 || store.lastWallet.Metadata["event"] != InventorySource || store.lastWallet.Metadata["request_id"] != testRequestID {
		t.Fatalf("unexpected wallet update: %#v", store.lastWallet)
	}

	var inventory Inventory
	if err := json.Unmarshal([]byte(store.object.Value), &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.Items["r1"].Source != InventorySource || inventory.Items["r1"].AcquiredAt != 1_790_265_600 {
		t.Fatalf("unexpected inventory: %#v", inventory)
	}
	if len(inventory.ProcessedRequests) != 1 || inventory.ProcessedRequests[0].GemBalanceAfter != 900 {
		t.Fatalf("unexpected processed request: %#v", inventory.ProcessedRequests)
	}
}

func TestDrawTenReturnsUniqueItemsAndUsesDiscount(t *testing.T) {
	service := testService(t, skincatalog.DefaultCatalog())
	store := newFakeStore(1_000)
	response, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ItemIDs) != 10 || response.GemSpent != 900 || response.GemBalanceAfter != 100 {
		t.Fatalf("unexpected response: %#v", response)
	}
	seen := make(map[string]struct{}, len(response.ItemIDs))
	for _, itemID := range response.ItemIDs {
		if _, duplicate := seen[itemID]; duplicate {
			t.Fatalf("duplicate result %q in %#v", itemID, response.ItemIDs)
		}
		seen[itemID] = struct{}{}
	}
}

func TestDrawExcludesOwnedAndExclusiveItems(t *testing.T) {
	itemCatalog, err := skincatalog.ParseCatalog([]byte(`{"parts":[{"type":"hat","prefix":"h","ids":[1,2,3],"exclusive_ids":[2]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	service := testService(t, itemCatalog)
	store := newFakeStore(1_000)
	setInventory(t, store, Inventory{Items: map[string]InventoryItem{"h1": {AcquiredAt: 1, Source: "other"}}})

	response, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.ItemIDs, []string{"h3"}) {
		t.Fatalf("unexpected result: %#v", response.ItemIDs)
	}
}

func TestDrawRejectsInsufficientGemsAndPoolWithoutMutation(t *testing.T) {
	service := testService(t, skincatalog.DefaultCatalog())
	store := newFakeStore(99)
	_, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1})
	if !errors.Is(err, ErrInsufficientGems) || store.object != nil || store.wallet[GemCurrency] != 99 {
		t.Fatalf("insufficient gems result: err=%v object=%#v wallet=%#v", err, store.object, store.wallet)
	}

	smallCatalog, _ := skincatalog.ParseCatalog([]byte(`{"parts":[{"type":"hat","prefix":"h","ids":[1,2,3]}]}`))
	service = testService(t, smallCatalog)
	store = newFakeStore(1_000)
	_, err = service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 10})
	if !errors.Is(err, ErrInsufficientPool) || store.multiCalls != 0 || store.wallet[GemCurrency] != 1_000 {
		t.Fatalf("insufficient pool result: err=%v calls=%d wallet=%#v", err, store.multiCalls, store.wallet)
	}
}

func TestDrawIsIdempotent(t *testing.T) {
	service := testService(t, skincatalog.DefaultCatalog())
	store := newFakeStore(1_000)
	first, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if store.multiCalls != 1 || store.wallet[GemCurrency] != 900 || !second.Replayed || !reflect.DeepEqual(first.ItemIDs, second.ItemIDs) {
		t.Fatalf("idempotency failed: first=%#v second=%#v store=%#v", first, second, store)
	}
}

func TestDrawRetriesVersionConflict(t *testing.T) {
	service := testService(t, skincatalog.DefaultCatalog())
	store := newFakeStore(1_000)
	store.failMultiCount = 1
	if _, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1}); err != nil {
		t.Fatal(err)
	}
	if store.multiCalls != 2 || store.readCalls != 2 || store.wallet[GemCurrency] != 900 {
		t.Fatalf("unexpected retry behavior: calls=%d reads=%d wallet=%#v", store.multiCalls, store.readCalls, store.wallet)
	}
}

func TestDrawTrimsProcessedRequestHistory(t *testing.T) {
	service := testService(t, skincatalog.DefaultCatalog())
	store := newFakeStore(1_000)
	inventory := Inventory{Items: make(map[string]InventoryItem)}
	for i := 0; i < MaxProcessedRequests; i++ {
		inventory.ProcessedRequests = append(inventory.ProcessedRequests, ProcessedRequest{RequestID: fmt.Sprintf("old-%d", i)})
	}
	setInventory(t, store, inventory)
	if _, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1}); err != nil {
		t.Fatal(err)
	}
	var updated Inventory
	if err := json.Unmarshal([]byte(store.object.Value), &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.ProcessedRequests) != MaxProcessedRequests || updated.ProcessedRequests[0].RequestID != "old-1" || updated.ProcessedRequests[MaxProcessedRequests-1].RequestID != testRequestID {
		t.Fatalf("unexpected processed request history: %#v", updated.ProcessedRequests)
	}
}

func TestDrawRejectsInvalidRequestAndInventory(t *testing.T) {
	service := testService(t, skincatalog.DefaultCatalog())
	store := newFakeStore(1_000)
	invalidRequests := []Request{{RequestID: "not-a-uuid", DrawCount: 1}, {RequestID: testRequestID, DrawCount: 2}}
	for _, request := range invalidRequests {
		if _, err := service.Draw(context.Background(), store, "user-1", request); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("request %#v returned %v", request, err)
		}
	}
	store.object = &api.StorageObject{Collection: InventoryCollection, Key: InventoryKey, UserId: "user-1", Value: `{"items":{"unknown1":{"acquired_at":1,"source":"other"}}}`, Version: "v1"}
	if _, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1}); !errors.Is(err, ErrInvalidInventory) {
		t.Fatalf("expected invalid inventory error, got %v", err)
	}
	store.object.Value = `{invalid`
	if _, err := service.Draw(context.Background(), store, "user-1", Request{RequestID: testRequestID, DrawCount: 1}); !errors.Is(err, ErrInvalidInventory) {
		t.Fatalf("expected malformed inventory error, got %v", err)
	}
}

func setInventory(t *testing.T, store *fakeStore, inventory Inventory) {
	t.Helper()
	value, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	store.object = &api.StorageObject{
		Collection: InventoryCollection, Key: InventoryKey, UserId: "user-1",
		Value: string(value), Version: "v1", PermissionRead: 1, PermissionWrite: 0,
	}
}
