package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/heroiclabs/nakama-common/runtime"
)

type setLoadoutRequest struct {
	SquadLoadout map[string]string `json:"squad_loadout"`
	Version      string            `json:"version"`
}

func (s *Service) GetRPC(ctx context.Context, _ runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, _ string) (string, error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return "", err
	}
	snapshot, err := s.Load(ctx, nk, userID)
	if err != nil {
		return "", rpcError(err)
	}
	return encodeSnapshot(snapshot)
}

func (s *Service) SetLoadoutRPC(ctx context.Context, _ runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return "", err
	}
	var request setLoadoutRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		return "", runtime.NewError("invalid squad loadout payload", 3)
	}
	snapshot, err := s.SetLoadout(ctx, nk, userID, request.Version, request.SquadLoadout)
	if err != nil {
		return "", rpcError(err)
	}
	return encodeSnapshot(snapshot)
}

func userIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("authentication required", 16)
	}
	return userID, nil
}

func rpcError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidLoadout):
		return runtime.NewError(err.Error(), 3)
	case errors.Is(err, ErrNotInitialized):
		return runtime.NewError(err.Error(), 9)
	case errors.Is(err, ErrVersionConflict):
		return runtime.NewError(err.Error(), 10)
	default:
		return runtime.NewError("character inventory unavailable", 13)
	}
}

func encodeSnapshot(snapshot Snapshot) (string, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
