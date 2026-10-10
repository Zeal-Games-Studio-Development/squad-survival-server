package matchmaking

import (
	"context"
	"encoding/json"
	"testing"

	"squad-survival-be/modules/game/inventory"
	"squad-survival-be/modules/game/matchregistry"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type inventoryModule struct{ runtime.NakamaModule }

type matchListModule struct {
	runtime.NakamaModule
	matches []*api.Match
}

func (m matchListModule) MatchList(context.Context, int, bool, string, *int, *int, string) ([]*api.Match, error) {
	return m.matches, nil
}

func (inventoryModule) StorageRead(_ context.Context, _ []*runtime.StorageRead) ([]*api.StorageObject, error) {
	return nil, nil
}

type matchmakerEntry struct {
	runtime.MatchmakerEntry
	presence runtime.Presence
}

func (e matchmakerEntry) GetPresence() runtime.Presence { return e.presence }

type matchmakerPresence struct {
	runtime.Presence
	userID string
}

func (p matchmakerPresence) GetUserId() string { return p.userID }

func TestInventoryGatesBothMatchmakingPaths(t *testing.T) {
	service := inventory.DefaultService()
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	if _, err := findOrCreateRPC(ctx, nil, nil, inventoryModule{}, "", matchregistry.New(), service); err == nil {
		t.Fatal("RPC accepted missing inventory")
	}
	entry := matchmakerEntry{presence: matchmakerPresence{userID: "user-1"}}
	if _, err := matched(context.Background(), nil, inventoryModule{}, []runtime.MatchmakerEntry{entry}, service); err == nil {
		t.Fatal("matchmaker accepted missing inventory")
	}
}

func TestMatchQueryUsesNakamaBooleanToken(t *testing.T) {
	query := matchQuery(Request{Mode: "survival"})
	want := "+label.mode:survival +label.joinable:T +label.max_players:32"
	if query != want {
		t.Fatalf("unexpected query: got %q want %q", query, want)
	}
}

func TestBattleRoyaleMatchQueryOnlyFindsWaitingLobbies(t *testing.T) {
	query := matchQuery(Request{Mode: "battle-royale"})
	want := "+label.mode:battle-royale +label.joinable:T +label.max_players:32 +label.status:waiting"
	if query != want {
		t.Fatalf("unexpected query: got %q want %q", query, want)
	}
}

func TestSurvivalMatchmakingRespectsPhaseCapacity(t *testing.T) {
	matches := []*api.Match{
		{MatchId: "waiting-one-slot", Size: 2, Label: wrapperspb.String(`{"status":"waiting","available_slots":1}`)},
		{MatchId: "playing-two-slots", Size: 30, Label: wrapperspb.String(`{"status":"playing","available_slots":2}`)},
		{MatchId: "ended", Size: 1, Label: wrapperspb.String(`{"status":"ended","available_slots":31}`)},
	}
	module := matchListModule{matches: matches}
	found, err := findJoinable(context.Background(), module, Request{Mode: "survival"}, 2)
	if err != nil || len(found) != 1 || found[0].GetMatchId() != "playing-two-slots" {
		t.Fatalf("wrong matches for two-player party: %+v, %v", found, err)
	}
	if _, _, err := findOrCreate(context.Background(), nil, matchListModule{}, Request{Mode: "survival"}, 4); err == nil {
		t.Fatal("party larger than waiting capacity created an unusable lobby")
	}
}

func TestMaxPlayersForMode(t *testing.T) {
	if got := maxPlayersForMode("battle-royale"); got != 32 {
		t.Fatalf("unexpected battle royale capacity: %d", got)
	}
	if got := maxPlayersForMode("survival"); got != 32 {
		t.Fatalf("unexpected survival capacity: %d", got)
	}
}

func TestParseRequestDefaults(t *testing.T) {
	request, err := parseRequest("")
	if err != nil {
		t.Fatal(err)
	}
	if request.Mode != "survival" {
		t.Fatalf("unexpected defaults: %+v", request)
	}
}

func TestModuleNameForMode(t *testing.T) {
	if got := moduleNameForMode("battle-royale"); got != "battle-royale" {
		t.Fatalf("battle-royale routed to %q", got)
	}
	if got := moduleNameForMode("survival"); got != "survival" {
		t.Fatalf("survival routed to %q", got)
	}
}

func TestFindOrCreateReturnsActiveMatch(t *testing.T) {
	registry := matchregistry.New()
	if !registry.Add("user-1", "session-1", "match-1") {
		t.Fatal("expected active match membership")
	}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "user-1")
	payload, err := findOrCreateRPC(ctx, nil, nil, nil, "", registry)
	if err != nil {
		t.Fatal(err)
	}
	var response Response
	if err = json.Unmarshal([]byte(payload), &response); err != nil {
		t.Fatal(err)
	}
	if response.MatchID != "match-1" || response.Created || !response.AlreadyJoined {
		t.Fatalf("unexpected response: %+v", response)
	}
}
