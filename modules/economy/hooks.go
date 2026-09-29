package economy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func Register(initializer runtime.Initializer, service *Service, storage InitialStorageWriter) error {
	hook := func(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, session *api.Session) error {
		return initializeAuthenticatedAccount(ctx, logger, nk, session, service, storage)
	}

	if err := initializer.RegisterAfterAuthenticateApple(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateAppleRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateCustom(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateCustomRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateDevice(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateDeviceRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateEmail(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateEmailRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateFacebook(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateFacebookRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateFacebookInstantGame(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateFacebookInstantGameRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateGameCenter(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateGameCenterRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateGoogle(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateGoogleRequest) error {
		return hook(ctx, l, db, nk, out)
	}); err != nil {
		return err
	}
	return initializer.RegisterAfterAuthenticateSteam(func(ctx context.Context, l runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateSteamRequest) error {
		return hook(ctx, l, db, nk, out)
	})
}

func initializeAuthenticatedAccount(ctx context.Context, logger runtime.Logger, store NewAccountStore, session *api.Session, service *Service, storage InitialStorageWriter) error {
	if session == nil || !session.Created {
		return nil
	}
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return errors.New("new-account initialization: user id missing from runtime context")
	}
	if err := service.InitializeNewAccount(ctx, store, userID, storage); err != nil {
		if logger != nil {
			logger.Error("Could not initialize new user %s: %v", userID, err)
		}
		return fmt.Errorf("initialize new user %s: %w", userID, err)
	}
	if logger != nil {
		logger.Info("Initialized currency and skin inventory for new user %s", userID)
	}
	return nil
}
