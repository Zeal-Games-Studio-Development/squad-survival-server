package main

import (
	"context"
	"database/sql"

	"squad-survival-be/modules/economy"
	"squad-survival-be/modules/game/matchmaking"
	"squad-survival-be/modules/game/matchregistry"
	"squad-survival-be/modules/game/survival"
	skincatalog "squad-survival-be/modules/skin/catalog"
	skindraw "squad-survival-be/modules/skin/draw"

	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(_ context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, initializer runtime.Initializer) error {
	economyService, err := economy.NewService(economy.DefaultCurrencies())
	if err != nil {
		return err
	}
	if err := economy.Register(initializer, economyService); err != nil {
		return err
	}
	itemCatalog := skincatalog.DefaultCatalog()
	skinDrawService, err := skindraw.NewService(itemCatalog)
	if err != nil {
		return err
	}
	if err := skindraw.Register(initializer, skinDrawService); err != nil {
		return err
	}

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
