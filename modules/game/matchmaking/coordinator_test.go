package matchmaking

import (
	"context"
	"encoding/json"
	"testing"

	"squad-survival-be/modules/game/matchregistry"

	"github.com/heroiclabs/nakama-common/runtime"
)

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
