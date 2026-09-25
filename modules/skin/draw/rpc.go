package draw

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

func Register(initializer runtime.Initializer, service *Service) error {
	if service == nil {
		return errors.New("skin lucky draw service must not be nil")
	}
	return initializer.RegisterRpc(RPCName, NewRPC(service))
}

func NewRPC(service *Service) func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, string) (string, error) {
	return func(ctx context.Context, _ runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
		userID, _ := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
		if userID == "" {
			return "", runtime.NewError("authentication required", 16)
		}

		var request Request
		decoder := json.NewDecoder(strings.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return "", runtime.NewError("invalid request payload", 3)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return "", runtime.NewError("invalid request payload", 3)
		}

		response, err := service.Draw(ctx, nk, userID, request)
		if err != nil {
			return "", rpcError(err)
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			return "", runtime.NewError("encode skin lucky draw response", 13)
		}
		return string(encoded), nil
	}
}

func rpcError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return runtime.NewError(err.Error(), 3)
	case errors.Is(err, ErrInsufficientGems), errors.Is(err, ErrInsufficientPool):
		return runtime.NewError(err.Error(), 9)
	case errors.Is(err, ErrConcurrentUpdate):
		return runtime.NewError(err.Error(), 10)
	default:
		return runtime.NewError(fmt.Sprintf("skin lucky draw failed: %v", err), 13)
	}
}
