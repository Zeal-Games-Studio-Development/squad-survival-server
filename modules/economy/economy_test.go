package economy

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type walletCall struct {
	userID       string
	changeset    map[string]int64
	metadata     map[string]interface{}
	updateLedger bool
}

type fakeWallet struct {
	call walletCall
	err  error
}

func (w *fakeWallet) WalletUpdate(_ context.Context, userID string, changeset map[string]int64, metadata map[string]interface{}, updateLedger bool) (map[string]int64, map[string]int64, error) {
	w.call = walletCall{userID: userID, changeset: changeset, metadata: metadata, updateLedger: updateLedger}
	return map[string]int64{"gold": 90}, map[string]int64{"gold": 100}, w.err
}

func TestParseCurrencies(t *testing.T) {
	currencies, err := ParseCurrencies([]byte(`{"currencies":[{"id":"gold","init_value":1000},{"id":"gem","init_value":50}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(currencies) != 2 || currencies[0].ID != "gold" || currencies[0].InitValue != 1000 {
		t.Fatalf("unexpected currencies: %#v", currencies)
	}
}

func TestParseCurrenciesRejectsInvalidConfiguration(t *testing.T) {
	tests := []string{
		`{"currencies":[]}`,
		`{"currencies":[{"id":"Gold","init_value":1}]}`,
		`{"currencies":[{"id":"gold","init_value":-1}]}`,
		`{"currencies":[{"id":"gold","init_value":1},{"id":"gold","init_value":2}]}`,
	}
	for _, data := range tests {
		if _, err := ParseCurrencies([]byte(data)); err == nil {
			t.Errorf("expected error for %s", data)
		}
	}
}

func TestPayUsesNegativeAtomicWalletChangeset(t *testing.T) {
	service, err := NewService([]Currency{{ID: "gold"}, {ID: "gem"}})
	if err != nil {
		t.Fatal(err)
	}
	wallet := &fakeWallet{}
	metadata := map[string]interface{}{"item_id": "sword"}

	updated, err := service.Pay(context.Background(), wallet, "user-1", map[string]int64{"gold": 10, "gem": 2}, metadata)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wallet.call.changeset, map[string]int64{"gold": -10, "gem": -2}) {
		t.Fatalf("unexpected changeset: %#v", wallet.call.changeset)
	}
	if wallet.call.userID != "user-1" || !wallet.call.updateLedger || wallet.call.metadata[LedgerEventKey] != EventPayment {
		t.Fatalf("unexpected wallet call: %#v", wallet.call)
	}
	if metadata[LedgerEventKey] != nil {
		t.Fatal("Pay mutated caller metadata")
	}
	if updated["gold"] != 90 {
		t.Fatalf("unexpected updated wallet: %#v", updated)
	}
}

func TestPayRejectsUnknownCurrencyAndInvalidAmount(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold"}})
	wallet := &fakeWallet{}
	if _, err := service.Pay(context.Background(), wallet, "user-1", map[string]int64{"coin": 1}, nil); err == nil {
		t.Fatal("expected unknown currency error")
	}
	if _, err := service.Pay(context.Background(), wallet, "user-1", map[string]int64{"gold": 0}, nil); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestGrantInitialCurrency(t *testing.T) {
	service, _ := NewService([]Currency{{ID: "gold", InitValue: 1000}, {ID: "gem", InitValue: 50}, {ID: "token", InitValue: 0}})
	wallet := &fakeWallet{}

	if err := service.GrantInitialCurrency(context.Background(), wallet, "new-user"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wallet.call.changeset, map[string]int64{"gold": 1000, "gem": 50}) {
		t.Fatalf("unexpected initial changeset: %#v", wallet.call.changeset)
	}
	if wallet.call.metadata[LedgerEventKey] != EventInitialCurrencyGrant || !wallet.call.updateLedger {
		t.Fatalf("unexpected wallet call: %#v", wallet.call)
	}
}
