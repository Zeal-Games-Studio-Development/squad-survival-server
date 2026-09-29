package economy

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type fakeInitialStorage struct {
	write *runtime.StorageWrite
	err   error
}

func (s fakeInitialStorage) InitialStorageWrite(string) (*runtime.StorageWrite, error) {
	return s.write, s.err
}

type fakeNewAccountStore struct {
	calls         int
	writes        []*runtime.StorageWrite
	walletUpdates []*runtime.WalletUpdate
	updateLedger  bool
	err           error
}

func (s *fakeNewAccountStore) MultiUpdate(_ context.Context, _ []*runtime.AccountUpdate, writes []*runtime.StorageWrite, _ []*runtime.StorageDelete, walletUpdates []*runtime.WalletUpdate, updateLedger bool) ([]*api.StorageObjectAck, []*runtime.WalletUpdateResult, error) {
	s.calls++
	s.writes = writes
	s.walletUpdates = walletUpdates
	s.updateLedger = updateLedger
	return nil, nil, s.err
}

func TestInitializeNewAccountCommitsCurrencyAndStorageTogether(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold", InitValue: 1000}, {ID: "gem", InitValue: 50}, {ID: "token"}})
	write := &runtime.StorageWrite{Collection: "player_inventory", Key: "skins", UserID: "new-user", Version: "*"}
	store := &fakeNewAccountStore{}

	if err := service.InitializeNewAccount(context.Background(), store, "new-user", fakeInitialStorage{write: write}); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 || len(store.writes) != 1 || store.writes[0] != write || len(store.walletUpdates) != 1 || !store.updateLedger {
		t.Fatalf("unexpected transaction: %#v", store)
	}
	update := store.walletUpdates[0]
	if update.UserID != "new-user" || !reflect.DeepEqual(update.Changeset, map[string]int64{"gold": 1000, "gem": 50}) || update.Metadata[LedgerEventKey] != EventInitialCurrencyGrant {
		t.Fatalf("unexpected initial wallet update: %#v", update)
	}
}

func TestInitializeAuthenticatedAccountOnlyHandlesCreatedSessions(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold", InitValue: 1000}})
	store := &fakeNewAccountStore{}
	storage := fakeInitialStorage{write: &runtime.StorageWrite{}}

	if err := initializeAuthenticatedAccount(context.Background(), nil, store, &api.Session{Created: false}, service, storage); err != nil {
		t.Fatal(err)
	}
	if store.calls != 0 {
		t.Fatalf("existing account was initialized: calls=%d", store.calls)
	}
	if err := initializeAuthenticatedAccount(context.Background(), nil, store, &api.Session{Created: true}, service, storage); err == nil {
		t.Fatal("expected missing user id to fail")
	}

	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "new-user")
	store.err = errors.New("storage unavailable")
	if err := initializeAuthenticatedAccount(ctx, nil, store, &api.Session{Created: true}, service, storage); err == nil {
		t.Fatal("expected transaction error to fail authentication hook")
	}
}

func TestInitializeNewAccountRejectsInvalidStorageCatalog(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold", InitValue: 1000}})
	store := &fakeNewAccountStore{}
	if err := service.InitializeNewAccount(context.Background(), store, "new-user", fakeInitialStorage{err: errors.New("missing numeric id 1")}); err == nil {
		t.Fatal("expected invalid initial storage to fail")
	}
	if store.calls != 0 {
		t.Fatal("transaction ran after initial storage failed")
	}
}
