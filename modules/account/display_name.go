package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

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
)

type ChangeDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
}

type ChangeDisplayNameResponse struct {
	DisplayName string `json:"display_name"`
}

type profanityChecker func(string) bool

func NewChangeDisplayNameRPC(isProfane profanityChecker) func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, string) (string, error) {
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
		if err := nk.AccountUpdateId(ctx, userID, "", nil, displayName, "", "", "", ""); err != nil {
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
