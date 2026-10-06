package economy

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func TestInitializeNewAccountGrantsCurrency(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold", InitValue: 1000}, {ID: "gem", InitValue: 50}, {ID: "token"}})
	wallet := &fakeWallet{}
	if err := service.InitializeNewAccount(context.Background(), wallet, "new-user"); err != nil {
		t.Fatal(err)
	}
	if wallet.call.userID != "new-user" || !wallet.call.updateLedger || !reflect.DeepEqual(wallet.call.changeset, map[string]int64{"gold": 1000, "gem": 50}) || wallet.call.metadata[LedgerEventKey] != EventInitialCurrencyGrant {
		t.Fatalf("unexpected initial wallet update: %#v", wallet.call)
	}
}

func TestInitializeAuthenticatedAccountOnlyHandlesCreatedSessions(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold", InitValue: 1000}})
	wallet := &fakeWallet{}
	if err := initializeAuthenticatedAccount(context.Background(), nil, wallet, &api.Session{Created: false}, service); err != nil {
		t.Fatal(err)
	}
	if wallet.call.userID != "" {
		t.Fatalf("existing account was initialized: %#v", wallet.call)
	}
	if err := initializeAuthenticatedAccount(context.Background(), nil, wallet, &api.Session{Created: true}, service); err == nil {
		t.Fatal("expected missing user id to fail")
	}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "new-user")
	wallet.err = errors.New("wallet unavailable")
	if err := initializeAuthenticatedAccount(ctx, nil, wallet, &api.Session{Created: true}, service); err == nil {
		t.Fatal("expected wallet error to fail authentication hook")
	}
}

func TestInitializeNewAccountRejectsMissingUserOrWallet(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold", InitValue: 1000}})
	if err := service.InitializeNewAccount(context.Background(), &fakeWallet{}, ""); err == nil {
		t.Fatal("expected missing user id to fail")
	}
	if err := service.InitializeNewAccount(context.Background(), nil, "new-user"); err == nil {
		t.Fatal("expected missing wallet to fail")
	}
}
