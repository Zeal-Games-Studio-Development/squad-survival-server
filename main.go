package main

import (
	"context"
	"database/sql"

	"squad-survival-be/modules/account"
	"squad-survival-be/modules/economy"
	"squad-survival-be/modules/game/inventory"
	"squad-survival-be/modules/game/matchmaking"
	"squad-survival-be/modules/game/matchregistry"
	"squad-survival-be/modules/game/royale"
	"squad-survival-be/modules/game/survival"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(_ context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, initializer runtime.Initializer) error {
	if err := account.Register(initializer); err != nil {
		return err
	}

	economyService, err := economy.NewService(economy.DefaultCurrencies())
	if err != nil {
		return err
	}
	inventoryService := inventory.DefaultService()
	if err := economy.Register(initializer, economyService, func(ctx context.Context, logger runtime.Logger, nk runtime.NakamaModule, _ *api.Session) error {
		userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
		if !ok || userID == "" {
			return inventory.ErrNotInitialized
		}
		return inventoryService.InitializeNewAccount(ctx, nk, userID)
	}); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("get_character_inventory", inventoryService.GetRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("set_squad_loadout", inventoryService.SetLoadoutRPC); err != nil {
		return err
	}

	registry := matchregistry.New()
	if err := initializer.RegisterMatch(survival.ModuleName, survival.NewMatchHandler(registry, inventoryService)); err != nil {
		return err
	}
	if err := initializer.RegisterMatch(royale.ModuleName, royale.NewMatchHandler(registry, inventoryService)); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("find_or_create_match", matchmaking.NewFindOrCreateRPC(registry, inventoryService)); err != nil {
		return err
	}
	if err := initializer.RegisterMatchmakerMatched(matchmaking.NewMatched(inventoryService)); err != nil {
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
