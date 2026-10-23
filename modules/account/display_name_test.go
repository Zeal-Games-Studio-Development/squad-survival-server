package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"squad-survival-be/modules/economy"

	goaway "github.com/TwiN/go-away"
	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func TestValidateDisplayNameAcceptsAndNormalizesUnicode(t *testing.T) {
	tests := map[string]string{
		"vietnamese and digits": "Nguyễn 123",
		"decomposed accent":     "Nguye\u0302̃n",
		"cjk":                   "玩家一二三",
		"unicode digits":        "Player ١٢",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := validateDisplayName("  "+input+"  ", func(string) bool { return false })
			if err != nil {
				t.Fatalf("validate display name: %v", err)
			}
			if got == "" || got[0] == ' ' || got[len(got)-1] == ' ' {
				t.Fatalf("expected a trimmed name, got %q", got)
			}
		})
	}
}

func TestValidateDisplayNameNormalizesNFC(t *testing.T) {
	got, err := validateDisplayName("  Nguye\u0302̃n  ", func(string) bool { return false })
	if err != nil {
		t.Fatalf("validate display name: %v", err)
	}
	if got != "Nguyễn" {
		t.Fatalf("expected NFC-normalized name, got %q", got)
	}
}

func TestDefaultProfanityCheckerAcceptsUnicodeName(t *testing.T) {
	got, err := validateDisplayName("Nguyễn 123", goaway.IsProfane)
	if err != nil {
		t.Fatalf("expected Unicode name to pass the default profanity checker: %v", err)
	}
	if got != "Nguyễn 123" {
		t.Fatalf("unexpected validated name: %q", got)
	}
}

func TestValidateDisplayNameChecksLengthBeforeProfanity(t *testing.T) {
	checkerCalled := false
	_, err := validateDisplayName("ab", func(string) bool {
		checkerCalled = true
		return true
	})
	assertRuntimeError(t, err, invalidArgumentCode, invalidLengthMessage)
	if checkerCalled {
		t.Fatal("profanity checker must not run before length validation succeeds")
	}

	_, err = validateDisplayName("1234567890123", func(string) bool {
		checkerCalled = true
		return true
	})
	assertRuntimeError(t, err, invalidArgumentCode, invalidLengthMessage)
	if checkerCalled {
		t.Fatal("profanity checker must not run for an overlong name")
	}
}

func TestValidateDisplayNameRejectsInvalidCharactersBeforeProfanity(t *testing.T) {
	tests := []string{"abc@", "abc%", "abc^", "abc_", "abc-", "abc🙂", "abc\tdef", "abc\ndef", "abc  def"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			checkerCalled := false
			_, err := validateDisplayName(input, func(string) bool {
				checkerCalled = true
				return false
			})
			assertRuntimeError(t, err, invalidArgumentCode, invalidCharactersMessage)
			if checkerCalled {
				t.Fatal("profanity checker must not run for invalid characters")
			}
		})
	}
}

func TestValidateDisplayNameRejectsProfanity(t *testing.T) {
	for _, input := range []string{"fuck", "FUCK", "sh1t"} {
		t.Run(input, func(t *testing.T) {
			_, err := validateDisplayName(input, goaway.IsProfane)
			assertRuntimeError(t, err, invalidArgumentCode, profanityMessage)
		})
	}
}

type accountUpdateCall struct {
	userID      string
	username    string
	metadata    map[string]interface{}
	displayName string
	timezone    string
	location    string
	langTag     string
	avatarURL   string
}

type fakeNakamaModule struct {
	runtime.NakamaModule
	mu            sync.Mutex
	call          *accountUpdateCall
	err           error
	name          string
	balance       int64
	version       string
	multiCalls    int
	conflictCount int
	beforeMulti   func()
	multiErr      error
	ledger        []*runtime.WalletUpdate
}

func testChangeDisplayNameRPC(check profanityChecker) func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, string) (string, error) {
	service, err := economy.NewService(economy.DefaultCurrencies())
	if err != nil {
		panic(err)
	}
	return NewChangeDisplayNameRPC(check, service, map[string]int64{"gem": 500})
}

func (m *fakeNakamaModule) StorageRead(_ context.Context, _ []*runtime.StorageRead) ([]*api.StorageObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.version == "" {
		return nil, nil
	}
	return []*api.StorageObject{{Version: m.version}}, nil
}

func (m *fakeNakamaModule) AccountGetId(_ context.Context, _ string) (*api.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &api.Account{User: &api.User{DisplayName: m.name}}, nil
}

func (m *fakeNakamaModule) MultiUpdate(_ context.Context, accounts []*runtime.AccountUpdate, writes []*runtime.StorageWrite, deletes []*runtime.StorageDelete, wallets []*runtime.WalletUpdate, ledger bool) ([]*api.StorageObjectAck, []*runtime.WalletUpdateResult, error) {
	if m.beforeMulti != nil {
		m.beforeMulti()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.multiCalls++
	if m.multiErr != nil {
		return nil, nil, m.multiErr
	}
	if len(accounts) != 1 || len(writes) != 1 || len(deletes) != 0 || !ledger {
		panic("unexpected transaction")
	}
	write := writes[0]
	if write.Collection != operationCollection || write.Key != ChangeDisplayNameRPC || write.PermissionRead != 0 || write.PermissionWrite != 0 || write.UserID != accounts[0].UserID {
		panic("unexpected operation guard")
	}
	if m.conflictCount > 0 {
		m.conflictCount--
		return nil, nil, runtime.ErrStorageRejectedVersion
	}
	expected := m.version
	if expected == "" {
		expected = "*"
	}
	if write.Version != expected {
		return nil, nil, runtime.ErrStorageRejectedVersion
	}
	balance := m.balance
	for _, wallet := range wallets {
		balance += wallet.Changeset["gem"]
		if balance < 0 {
			return nil, nil, &runtime.WalletNegativeError{UserID: wallet.UserID, Path: "gem", Current: m.balance, Amount: wallet.Changeset["gem"]}
		}
	}
	m.name = accounts[0].DisplayName
	m.balance = balance
	m.version = fmt.Sprint(m.multiCalls)
	m.ledger = append(m.ledger, wallets...)
	m.call = &accountUpdateCall{userID: accounts[0].UserID, displayName: m.name}
	return nil, nil, nil
}

func TestChangeDisplayNameRPCUpdatesAuthenticatedUser(t *testing.T) {
	nk := &fakeNakamaModule{}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	rpc := testChangeDisplayNameRPC(func(string) bool { return false })

	payload, err := rpc(ctx, nil, nil, nk, `{"display_name":"  Nguye\u0302̃n  "}`)
	if err != nil {
		t.Fatalf("change display name: %v", err)
	}
	if nk.call == nil || nk.call.userID != "user-1" || nk.call.displayName != "Nguyễn" {
		t.Fatalf("unexpected account update: %#v", nk.call)
	}
	if nk.call.username != "" || nk.call.metadata != nil || nk.call.timezone != "" || nk.call.location != "" || nk.call.langTag != "" || nk.call.avatarURL != "" {
		t.Fatalf("RPC changed unrelated account fields: %#v", nk.call)
	}
	var response ChangeDisplayNameResponse
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.DisplayName != "Nguyễn" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestChangeDisplayNameRPCRejectsUnauthenticatedRequest(t *testing.T) {
	rpc := testChangeDisplayNameRPC(func(string) bool { return false })
	_, err := rpc(context.Background(), nil, nil, &fakeNakamaModule{}, `{"display_name":"Player"}`)
	assertRuntimeError(t, err, unauthenticatedCode, authenticationMessage)
}

func TestChangeDisplayNameRPCRejectsMalformedPayload(t *testing.T) {
	rpc := testChangeDisplayNameRPC(func(string) bool { return false })
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	for _, payload := range []string{
		`{"display_name":"Player","unknown":true}`,
		`{"display_name":"Player"} {}`,
		`not-json`,
	} {
		_, err := rpc(ctx, nil, nil, &fakeNakamaModule{}, payload)
		assertRuntimeError(t, err, invalidArgumentCode, invalidPayloadMessage)
	}
}

func TestChangeDisplayNameRPCMapsAccountUpdateFailure(t *testing.T) {
	nk := &fakeNakamaModule{err: errors.New("database unavailable")}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	_, err := testChangeDisplayNameRPC(func(string) bool { return false })(ctx, nil, nil, nk, `{"display_name":"Player"}`)
	assertRuntimeError(t, err, internalCode, updateDisplayNameMessage)
}

func TestChangeDisplayNameRPCDoesNotUpdateInvalidName(t *testing.T) {
	nk := &fakeNakamaModule{}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	_, err := testChangeDisplayNameRPC(func(string) bool { return true })(ctx, nil, nil, nk, `{"display_name":"Player"}`)
	assertRuntimeError(t, err, invalidArgumentCode, profanityMessage)
	if nk.call != nil {
		t.Fatalf("invalid name must not be persisted: %#v", nk.call)
	}
}

func assertRuntimeError(t *testing.T, err error, code int, message string) {
	t.Helper()
	runtimeErr, ok := err.(*runtime.Error)
	if !ok {
		t.Fatalf("expected *runtime.Error, got %T (%v)", err, err)
	}
	if runtimeErr.Code != code || runtimeErr.Message != message {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

func TestChangeDisplayNameChargesOnlyForActualRename(t *testing.T) {
	for _, tc := range []struct {
		label, current, requested, want string
		balance, remaining              int64
		calls, ledgerCount              int
		code                            int
	}{
		{label: "first name without gems", requested: "Player", want: "Player", calls: 1},
		{label: "exact funds", current: "Old", requested: "Player", want: "Player", balance: 500, calls: 1, ledgerCount: 1},
		{label: "insufficient funds", current: "Old", requested: "Player", want: "Old", balance: 499, remaining: 499, calls: 1, code: 9},
		{label: "same name normalized", current: "Nguyễn", requested: "  Nguye\u0302̃n  ", want: "Nguyễn", balance: 500, remaining: 500},
		{label: "imported name", current: "Google Name", requested: "Player", want: "Player", balance: 600, remaining: 100, calls: 1, ledgerCount: 1},
	} {
		t.Run(tc.label, func(t *testing.T) {
			nk := &fakeNakamaModule{name: tc.current, balance: tc.balance}
			ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
			request, _ := json.Marshal(ChangeDisplayNameRequest{DisplayName: tc.requested})
			_, err := testChangeDisplayNameRPC(nil)(ctx, nil, nil, nk, string(request))
			if tc.code != 0 {
				assertRuntimeError(t, err, tc.code, insufficientGemMessage)
			} else if err != nil {
				t.Fatal(err)
			}
			if nk.name != tc.want || nk.balance != tc.remaining || nk.multiCalls != tc.calls || len(nk.ledger) != tc.ledgerCount {
				t.Fatalf("unexpected result: name=%q balance=%d calls=%d ledger=%d", nk.name, nk.balance, nk.multiCalls, len(nk.ledger))
			}
			if len(nk.ledger) > 0 {
				entry := nk.ledger[0]
				if entry.UserID != "user-1" || entry.Changeset["gem"] != -500 || entry.Metadata[economy.LedgerEventKey] != economy.EventChangeDisplayName || len(entry.Metadata) != 1 {
					t.Fatalf("unexpected ledger: %+v", entry)
				}
			}
		})
	}
}

func TestChangeDisplayNameTransactionErrors(t *testing.T) {
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	for _, failure := range []error{errors.New("database unavailable"), runtime.ErrStorageRejectedPermission} {
		nk := &fakeNakamaModule{name: "Old", balance: 500, multiErr: failure}
		_, err := testChangeDisplayNameRPC(nil)(ctx, nil, nil, nk, `{"display_name":"Player"}`)
		assertRuntimeError(t, err, internalCode, updateDisplayNameMessage)
		if nk.name != "Old" || nk.balance != 500 || nk.version != "" || len(nk.ledger) != 0 {
			t.Fatal("failed transaction changed state")
		}
	}
	nk := &fakeNakamaModule{conflictCount: maxChangeAttempts}
	_, err := testChangeDisplayNameRPC(nil)(ctx, nil, nil, nk, `{"display_name":"Player"}`)
	assertRuntimeError(t, err, 10, conflictMessage)
	if nk.multiCalls != maxChangeAttempts || nk.name != "" {
		t.Fatal("unexpected conflict retry result")
	}
	nk = &fakeNakamaModule{conflictCount: 1}
	if _, err := testChangeDisplayNameRPC(nil)(ctx, nil, nil, nk, `{"display_name":"Player"}`); err != nil || nk.multiCalls != 2 {
		t.Fatalf("retry failed: %v", err)
	}
}

func TestConcurrentDisplayNameChanges(t *testing.T) {
	for _, sameName := range []bool{false, true} {
		t.Run(fmt.Sprint("sameName=", sameName), func(t *testing.T) {
			var arrivals atomic.Int32
			ready := make(chan struct{})
			nk := &fakeNakamaModule{balance: 500}
			nk.beforeMulti = func() {
				n := arrivals.Add(1)
				if n == 2 {
					close(ready)
				}
				if n <= 2 {
					<-ready
				}
			}
			ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
			rpc := testChangeDisplayNameRPC(nil)
			results := make(chan error, 2)
			for _, name := range []string{"PlayerOne", map[bool]string{false: "PlayerTwo", true: "PlayerOne"}[sameName]} {
				go func(name string) {
					request, _ := json.Marshal(ChangeDisplayNameRequest{DisplayName: name})
					_, err := rpc(ctx, nil, nil, nk, string(request))
					results <- err
				}(name)
			}
			for range 2 {
				if err := <-results; err != nil {
					t.Fatal(err)
				}
			}
			wantBalance, wantLedger := int64(0), 1
			if sameName {
				wantBalance, wantLedger = 500, 0
			}
			if nk.balance != wantBalance || len(nk.ledger) != wantLedger {
				t.Fatalf("unexpected concurrent result: balance=%d ledger=%d", nk.balance, len(nk.ledger))
			}
		})
	}
}
