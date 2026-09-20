package main

import (
	"context"
	"database/sql"

	"squad-survival-be/modules/game/matchmaking"
	"squad-survival-be/modules/game/matchregistry"
	"squad-survival-be/modules/game/survival"

	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(_ context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, initializer runtime.Initializer) error {
	registry := matchregistry.New()
	if err := initializer.RegisterMatch(survival.ModuleName, survival.NewMatchHandler(registry)); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("find_or_create_match", matchmaking.NewFindOrCreateRPC(registry)); err != nil {
		return err
	}
	if err := initializer.RegisterMatchmakerMatched(matchmaking.Matched); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("healthcheck", healthcheck); err != nil {
		return err
	}

	logger.Info("Squad Survival Go runtime module loaded")
	return nil
}

func healthcheck(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, _ string) (string, error) {
	return `{"status":"ok"}`, nil
}
