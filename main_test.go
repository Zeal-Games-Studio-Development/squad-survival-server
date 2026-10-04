package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type failingAccountInitializer struct {
	runtime.Initializer
	err error
}

func (i *failingAccountInitializer) RegisterBeforeUpdateAccount(func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, *api.UpdateAccountRequest) (*api.UpdateAccountRequest, error)) error {
	return i.err
}

func TestInitModuleReturnsAccountHookRegistrationError(t *testing.T) {
	wantErr := errors.New("account hook registration failed")
	err := InitModule(context.Background(), nil, nil, nil, &failingAccountInitializer{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected account registration error, got %v", err)
	}
}
