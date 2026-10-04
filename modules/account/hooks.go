package account

import (
	"context"
	"database/sql"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	profileUpdateDeniedMessage  = "account profile updates are not allowed"
	customUsernameDeniedMessage = "custom usernames are not allowed"
	permissionDeniedCode        = 7
)

// Register installs client-facing account restrictions. Server-side account
// updates made through NakamaModule do not pass through these API hooks.
func Register(initializer runtime.Initializer) error {
	if err := initializer.RegisterBeforeUpdateAccount(beforeUpdateAccount); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateApple(beforeAuthenticateApple); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateCustom(beforeAuthenticateCustom); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateDevice(beforeAuthenticateDevice); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateEmail(beforeAuthenticateEmail); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateFacebook(beforeAuthenticateFacebook); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateFacebookInstantGame(beforeAuthenticateFacebookInstantGame); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateGameCenter(beforeAuthenticateGameCenter); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateGoogle(beforeAuthenticateGoogle); err != nil {
		return err
	}
	return initializer.RegisterBeforeAuthenticateSteam(beforeAuthenticateSteam)
}

func beforeUpdateAccount(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.UpdateAccountRequest) (*api.UpdateAccountRequest, error) {
	if in != nil && (in.Username != nil || in.DisplayName != nil || in.AvatarUrl != nil || in.LangTag != nil || in.Location != nil || in.Timezone != nil) {
		return nil, runtime.NewError(profileUpdateDeniedMessage, permissionDeniedCode)
	}
	return in, nil
}

func rejectCustomUsername(username string) error {
	if username != "" {
		return runtime.NewError(customUsernameDeniedMessage, permissionDeniedCode)
	}
	return nil
}

func beforeAuthenticateApple(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateAppleRequest) (*api.AuthenticateAppleRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateCustom(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateCustomRequest) (*api.AuthenticateCustomRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateDevice(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateDeviceRequest) (*api.AuthenticateDeviceRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateEmail(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateEmailRequest) (*api.AuthenticateEmailRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateFacebook(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateFacebookRequest) (*api.AuthenticateFacebookRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateFacebookInstantGame(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateFacebookInstantGameRequest) (*api.AuthenticateFacebookInstantGameRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateGameCenter(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateGameCenterRequest) (*api.AuthenticateGameCenterRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateGoogle(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateGoogleRequest) (*api.AuthenticateGoogleRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func beforeAuthenticateSteam(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, in *api.AuthenticateSteamRequest) (*api.AuthenticateSteamRequest, error) {
	if in != nil {
		if err := rejectCustomUsername(in.Username); err != nil {
			return nil, err
		}
	}
	return in, nil
}
