package account

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestBeforeUpdateAccountRejectsEveryProfileField(t *testing.T) {
	tests := map[string]func(*api.UpdateAccountRequest){
		"username":     func(in *api.UpdateAccountRequest) { in.Username = wrapperspb.String("") },
		"display_name": func(in *api.UpdateAccountRequest) { in.DisplayName = wrapperspb.String("") },
		"avatar_url":   func(in *api.UpdateAccountRequest) { in.AvatarUrl = wrapperspb.String("") },
		"lang_tag":     func(in *api.UpdateAccountRequest) { in.LangTag = wrapperspb.String("") },
		"location":     func(in *api.UpdateAccountRequest) { in.Location = wrapperspb.String("") },
		"timezone":     func(in *api.UpdateAccountRequest) { in.Timezone = wrapperspb.String("") },
	}

	for name, setField := range tests {
		t.Run(name, func(t *testing.T) {
			in := &api.UpdateAccountRequest{}
			setField(in)
			out, err := beforeUpdateAccount(context.Background(), nil, nil, nil, in)
			if out != nil {
				t.Fatalf("expected no request output, got %#v", out)
			}
			assertPermissionDenied(t, err, profileUpdateDeniedMessage)
		})
	}
}

func TestBeforeUpdateAccountAllowsEmptyRequest(t *testing.T) {
	in := &api.UpdateAccountRequest{}
	out, err := beforeUpdateAccount(context.Background(), nil, nil, nil, in)
	if err != nil {
		t.Fatalf("expected empty update to pass: %v", err)
	}
	if out != in {
		t.Fatal("expected the original request to be returned")
	}
}

func TestBeforeAuthenticateRejectsCustomUsernameForEveryProvider(t *testing.T) {
	tests := map[string]func(string) error{
		"apple": func(username string) error {
			_, err := beforeAuthenticateApple(context.Background(), nil, nil, nil, &api.AuthenticateAppleRequest{Username: username})
			return err
		},
		"custom": func(username string) error {
			_, err := beforeAuthenticateCustom(context.Background(), nil, nil, nil, &api.AuthenticateCustomRequest{Username: username})
			return err
		},
		"device": func(username string) error {
			_, err := beforeAuthenticateDevice(context.Background(), nil, nil, nil, &api.AuthenticateDeviceRequest{Username: username})
			return err
		},
		"email": func(username string) error {
			_, err := beforeAuthenticateEmail(context.Background(), nil, nil, nil, &api.AuthenticateEmailRequest{Username: username})
			return err
		},
		"facebook": func(username string) error {
			_, err := beforeAuthenticateFacebook(context.Background(), nil, nil, nil, &api.AuthenticateFacebookRequest{Username: username})
			return err
		},
		"facebook_instant_game": func(username string) error {
			_, err := beforeAuthenticateFacebookInstantGame(context.Background(), nil, nil, nil, &api.AuthenticateFacebookInstantGameRequest{Username: username})
			return err
		},
		"game_center": func(username string) error {
			_, err := beforeAuthenticateGameCenter(context.Background(), nil, nil, nil, &api.AuthenticateGameCenterRequest{Username: username})
			return err
		},
		"google": func(username string) error {
			_, err := beforeAuthenticateGoogle(context.Background(), nil, nil, nil, &api.AuthenticateGoogleRequest{Username: username})
			return err
		},
		"steam": func(username string) error {
			_, err := beforeAuthenticateSteam(context.Background(), nil, nil, nil, &api.AuthenticateSteamRequest{Username: username})
			return err
		},
	}

	for name, call := range tests {
		t.Run(name+"_rejects_username", func(t *testing.T) {
			assertPermissionDenied(t, call("player-name"), customUsernameDeniedMessage)
		})
		t.Run(name+"_allows_empty_username", func(t *testing.T) {
			if err := call(""); err != nil {
				t.Fatalf("expected empty username to pass: %v", err)
			}
		})
	}
}

func assertPermissionDenied(t *testing.T, err error, message string) {
	t.Helper()
	runtimeErr, ok := err.(*runtime.Error)
	if !ok {
		t.Fatalf("expected *runtime.Error, got %T (%v)", err, err)
	}
	if runtimeErr.Code != permissionDeniedCode || runtimeErr.Message != message {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

type recordingInitializer struct {
	runtime.Initializer
	calls  []string
	failAt string
	err    error
}

func (i *recordingInitializer) register(name string) error {
	i.calls = append(i.calls, name)
	if name == i.failAt {
		return i.err
	}
	return nil
}

func (i *recordingInitializer) RegisterBeforeUpdateAccount(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.UpdateAccountRequest) (*api.UpdateAccountRequest, error)) error {
	return i.register("update_account")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateApple(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateAppleRequest) (*api.AuthenticateAppleRequest, error)) error {
	return i.register("apple")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateCustom(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateCustomRequest) (*api.AuthenticateCustomRequest, error)) error {
	return i.register("custom")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateDevice(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateDeviceRequest) (*api.AuthenticateDeviceRequest, error)) error {
	return i.register("device")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateEmail(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateEmailRequest) (*api.AuthenticateEmailRequest, error)) error {
	return i.register("email")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateFacebook(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateFacebookRequest) (*api.AuthenticateFacebookRequest, error)) error {
	return i.register("facebook")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateFacebookInstantGame(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateFacebookInstantGameRequest) (*api.AuthenticateFacebookInstantGameRequest, error)) error {
	return i.register("facebook_instant_game")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateGameCenter(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateGameCenterRequest) (*api.AuthenticateGameCenterRequest, error)) error {
	return i.register("game_center")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateGoogle(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateGoogleRequest) (*api.AuthenticateGoogleRequest, error)) error {
	return i.register("google")
}
func (i *recordingInitializer) RegisterBeforeAuthenticateSteam(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.AuthenticateSteamRequest) (*api.AuthenticateSteamRequest, error)) error {
	return i.register("steam")
}
func (i *recordingInitializer) RegisterRpc(id string, _ func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, string) (string, error)) error {
	return i.register("rpc:" + id)
}

func TestRegisterInstallsAllHooks(t *testing.T) {
	initializer := &recordingInitializer{}
	if err := Register(initializer); err != nil {
		t.Fatalf("register hooks: %v", err)
	}
	want := []string{"update_account", "apple", "custom", "device", "email", "facebook", "facebook_instant_game", "game_center", "google", "steam", "rpc:" + ChangeDisplayNameRPC}
	if !reflect.DeepEqual(initializer.calls, want) {
		t.Fatalf("unexpected registrations: got %v want %v", initializer.calls, want)
	}
}

func TestRegisterReturnsRPCRegistrationError(t *testing.T) {
	wantErr := errors.New("rpc registration failed")
	initializer := &recordingInitializer{failAt: "rpc:" + ChangeDisplayNameRPC, err: wantErr}
	if err := Register(initializer); !errors.Is(err, wantErr) {
		t.Fatalf("expected RPC registration error, got %v", err)
	}
}

func TestRegisterReturnsRegistrationError(t *testing.T) {
	wantErr := errors.New("registration failed")
	initializer := &recordingInitializer{failAt: "google", err: wantErr}
	if err := Register(initializer); !errors.Is(err, wantErr) {
		t.Fatalf("expected registration error, got %v", err)
	}
	wantCalls := []string{"update_account", "apple", "custom", "device", "email", "facebook", "facebook_instant_game", "game_center", "google"}
	if !reflect.DeepEqual(initializer.calls, wantCalls) {
		t.Fatalf("expected registration to stop on failure: got %v want %v", initializer.calls, wantCalls)
	}
}
