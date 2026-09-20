package matchmaking

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"squad-survival-be/modules/game/matchregistry"
	"squad-survival-be/modules/game/survival"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

var (
	modePattern       = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)
	findOrCreateMutex sync.Mutex
)

type Request struct {
	Mode                string `json:"mode"`
	AllowJoinInProgress *bool  `json:"allow_join_in_progress,omitempty"`
}

type Response struct {
	MatchID       string `json:"match_id"`
	Created       bool   `json:"created"`
	AlreadyJoined bool   `json:"already_joined"`
}

func NewFindOrCreateRPC(registry *matchregistry.Registry) func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule, string) (string, error) {
	return func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
		return findOrCreateRPC(ctx, logger, db, nk, payload, registry)
	}
}

func findOrCreateRPC(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, payload string, registry *matchregistry.Registry) (string, error) {
	request, err := parseRequest(payload)
	if err != nil {
		return "", err
	}
	if userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string); ok {
		if matchID, active := registry.MatchForUser(userID); active {
			return encodeResponse(Response{MatchID: matchID, AlreadyJoined: true})
		}
	}

	matchID, created, err := findOrCreate(ctx, logger, nk, request, 1)
	if err != nil {
		return "", err
	}

	return encodeResponse(Response{MatchID: matchID, Created: created})
}

func encodeResponse(value Response) (string, error) {
	response, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(response), nil
}

func Matched(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, entries []runtime.MatchmakerEntry) (string, error) {
	if len(entries) == 0 {
		return "", errors.New("matchmaker returned no entries")
	}

	request := requestFromProperties(entries[0].GetProperties())
	matchID, _, err := findOrCreate(ctx, logger, nk, request, len(entries))
	return matchID, err
}

func findOrCreate(ctx context.Context, logger runtime.Logger, nk runtime.NakamaModule, request Request, requiredSlots int) (string, bool, error) {
	findOrCreateMutex.Lock()
	defer findOrCreateMutex.Unlock()

	allowJoin := request.AllowJoinInProgress == nil || *request.AllowJoinInProgress
	if allowJoin {
		matches, err := findJoinable(ctx, nk, request, requiredSlots)
		if err != nil {
			return "", false, err
		}
		if len(matches) > 0 {
			logger.Info("Backfilling survival match: match_id=%s players=%d", matches[0].GetMatchId(), requiredSlots)
			return matches[0].GetMatchId(), false, nil
		}
	}

	matchID, err := nk.MatchCreate(ctx, survival.ModuleName, map[string]interface{}{
		"mode":                   request.Mode,
		"allow_join_in_progress": allowJoin,
	})
	if err != nil {
		return "", false, err
	}

	logger.Info("Created survival match: match_id=%s players=%d", matchID, requiredSlots)
	return matchID, true, nil
}

func findJoinable(ctx context.Context, nk runtime.NakamaModule, request Request, requiredSlots int) ([]*api.Match, error) {
	maxCurrentSize := survival.MaxPlayers - requiredSlots
	if maxCurrentSize < 0 {
		return nil, errors.New("matched party exceeds match capacity")
	}

	query := matchQuery(request)
	matches, err := nk.MatchList(ctx, 20, true, "", nil, &maxCurrentSize, query)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].GetSize() > matches[j].GetSize()
	})
	return matches, nil
}

func matchQuery(request Request) string {
	return fmt.Sprintf("+label.mode:%s +label.joinable:T +label.max_players:%d", request.Mode, survival.MaxPlayers)
}

func parseRequest(payload string) (Request, error) {
	request := defaultRequest()
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			return Request{}, errors.New("invalid matchmaking payload")
		}
	}
	if !modePattern.MatchString(request.Mode) {
		return Request{}, errors.New("mode must contain 1-32 letters, numbers, underscores, or hyphens")
	}
	return request, nil
}

func requestFromProperties(properties map[string]interface{}) Request {
	request := defaultRequest()
	if mode, ok := properties["mode"].(string); ok && modePattern.MatchString(mode) {
		request.Mode = mode
	}
	return request
}

func defaultRequest() Request {
	return Request{Mode: survival.DefaultMode}
}
