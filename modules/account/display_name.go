package account

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"squad-survival-be/modules/economy"

	"github.com/heroiclabs/nakama-common/runtime"
	"golang.org/x/text/unicode/norm"
)

const (
	ChangeDisplayNameRPC = "change_display_name"

	minDisplayNameLength = 3
	maxDisplayNameLength = 12

	invalidPayloadMessage    = "invalid request payload"
	invalidLengthMessage     = "display name must be between 3 and 12 characters"
	invalidCharactersMessage = "display name contains invalid characters"
	profanityMessage         = "display name contains profanity"
	authenticationMessage    = "authentication required"
	updateDisplayNameMessage = "could not update display name"
	encodeDisplayNameMessage = "could not encode display name response"
	invalidArgumentCode      = 3
	internalCode             = 13
	unauthenticatedCode      = 16
	operationCollection      = "account_operations"
	maxChangeAttempts        = 3
	conflictMessage          = "display name change conflicted; retry"
	insufficientGemMessage   = "insufficient gem"
)

type ChangeDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
}

type ChangeDisplayNameResponse struct {
	DisplayName string `json:"display_name"`
}

type profanityChecker func(string) bool

func NewChangeDisplayNameRPC(isProfane profanityChecker, economyService *economy.Service, costs map[string]int64) func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, string) (string, error) {
	return func(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
		userID, _ := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
		if userID == "" {
			return "", runtime.NewError(authenticationMessage, unauthenticatedCode)
		}

		var request ChangeDisplayNameRequest
		decoder := json.NewDecoder(strings.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return "", runtime.NewError(invalidPayloadMessage, invalidArgumentCode)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return "", runtime.NewError(invalidPayloadMessage, invalidArgumentCode)
		}

		displayName, err := validateDisplayName(request.DisplayName, isProfane)
		if err != nil {
			return "", err
		}
		if err := changeDisplayName(ctx, nk, economyService, costs, userID, displayName); err != nil {
			var runtimeErr *runtime.Error
			if errors.As(err, &runtimeErr) {
				return "", runtimeErr
			}
			if logger != nil {
				logger.Error("Could not update display name for user %s: %v", userID, err)
			}
			return "", runtime.NewError(updateDisplayNameMessage, internalCode)
		}

		response, err := json.Marshal(ChangeDisplayNameResponse{DisplayName: displayName})
		if err != nil {
			return "", runtime.NewError(encodeDisplayNameMessage, internalCode)
		}
		return string(response), nil
	}
}

// Read the operation version before the account snapshot. The conditional write
// prevents a stale snapshot from receiving another free rename, across nodes.
func changeDisplayName(ctx context.Context, nk runtime.NakamaModule, economyService *economy.Service, costs map[string]int64, userID, displayName string) error {
	for attempt := 0; attempt < maxChangeAttempts; attempt++ {
		objects, err := nk.StorageRead(ctx, []*runtime.StorageRead{{Collection: operationCollection, Key: ChangeDisplayNameRPC, UserID: userID}})
		if err != nil {
			return err
		}
		version := "*"
		if len(objects) > 0 {
			version = objects[0].Version
		}
		account, err := nk.AccountGetId(ctx, userID)
		if err != nil {
			return err
		}
		if account == nil || account.User == nil {
			return errors.New("account lookup returned no user")
		}
		currentName := account.User.GetDisplayName()
		if norm.NFC.String(strings.TrimSpace(currentName)) == displayName {
			return nil
		}
		var walletUpdates []*runtime.WalletUpdate
		if currentName != "" {
			payment, err := economyService.PaymentUpdate(userID, costs, map[string]interface{}{economy.LedgerEventKey: economy.EventChangeDisplayName})
			if err != nil {
				return err
			}
			walletUpdates = []*runtime.WalletUpdate{payment}
		}
		// A fresh nonce changes the storage version even when revisiting an old name.
		value, err := json.Marshal(map[string]string{"nonce": rand.Text()})
		if err != nil {
			return err
		}
		_, _, err = nk.MultiUpdate(ctx,
			[]*runtime.AccountUpdate{{UserID: userID, DisplayName: displayName}},
			[]*runtime.StorageWrite{{Collection: operationCollection, Key: ChangeDisplayNameRPC, UserID: userID, Value: string(value), Version: version, PermissionRead: 0, PermissionWrite: 0}},
			nil, walletUpdates, true)
		if errors.Is(err, runtime.ErrStorageRejectedVersion) {
			continue
		}
		var negative *runtime.WalletNegativeError
		if errors.As(err, &negative) {
			return runtime.NewError(insufficientGemMessage, 9)
		}
		return err
	}
	return runtime.NewError(conflictMessage, 10)
}

func validateDisplayName(input string, isProfane profanityChecker) (string, error) {
	displayName := norm.NFC.String(strings.TrimSpace(input))
	length := utf8.RuneCountInString(displayName)
	if length < minDisplayNameLength || length > maxDisplayNameLength {
		return "", runtime.NewError(invalidLengthMessage, invalidArgumentCode)
	}

	previousWasSpace := false
	for _, r := range displayName {
		if r == ' ' {
			if previousWasSpace {
				return "", runtime.NewError(invalidCharactersMessage, invalidArgumentCode)
			}
			previousWasSpace = true
			continue
		}
		previousWasSpace = false
		if !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsNumber(r) {
			return "", runtime.NewError(invalidCharactersMessage, invalidArgumentCode)
		}
	}

	if isProfane != nil && isProfane(displayName) {
		return "", runtime.NewError(profanityMessage, invalidArgumentCode)
	}
	return displayName, nil
}
