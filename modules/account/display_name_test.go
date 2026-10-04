package account

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	goaway "github.com/TwiN/go-away"
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
	call *accountUpdateCall
	err  error
}

func (m *fakeNakamaModule) AccountUpdateId(_ context.Context, userID, username string, metadata map[string]interface{}, displayName, timezone, location, langTag, avatarURL string) error {
	m.call = &accountUpdateCall{userID, username, metadata, displayName, timezone, location, langTag, avatarURL}
	return m.err
}

func TestChangeDisplayNameRPCUpdatesAuthenticatedUser(t *testing.T) {
	nk := &fakeNakamaModule{}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	rpc := NewChangeDisplayNameRPC(func(string) bool { return false })

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
	rpc := NewChangeDisplayNameRPC(func(string) bool { return false })
	_, err := rpc(context.Background(), nil, nil, &fakeNakamaModule{}, `{"display_name":"Player"}`)
	assertRuntimeError(t, err, unauthenticatedCode, authenticationMessage)
}

func TestChangeDisplayNameRPCRejectsMalformedPayload(t *testing.T) {
	rpc := NewChangeDisplayNameRPC(func(string) bool { return false })
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
	_, err := NewChangeDisplayNameRPC(func(string) bool { return false })(ctx, nil, nil, nk, `{"display_name":"Player"}`)
	assertRuntimeError(t, err, internalCode, updateDisplayNameMessage)
}

func TestChangeDisplayNameRPCDoesNotUpdateInvalidName(t *testing.T) {
	nk := &fakeNakamaModule{}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	_, err := NewChangeDisplayNameRPC(func(string) bool { return true })(ctx, nil, nil, nk, `{"display_name":"Player"}`)
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
