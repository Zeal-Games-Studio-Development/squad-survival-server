package economy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func Register(initializer runtime.Initializer, service *Service) error {
	hook := func(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, session *api.Session) error {
		if session == nil || !session.Created {
			return nil
		}
		userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
		if !ok || userID == "" {
			return errors.New("new-user currency grant: user id missing from runtime context")
		}
		if err := service.GrantInitialCurrency(ctx, nk, userID); err != nil {
			return fmt.Errorf("grant initial currency to user %s: %w", userID, err)
		}
		logger.Info("Granted initial currency to new user %s", userID)
		return nil
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
