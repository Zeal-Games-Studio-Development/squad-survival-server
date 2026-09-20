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

func TestParseRequestDefaults(t *testing.T) {
	request, err := parseRequest("")
	if err != nil {
		t.Fatal(err)
	}
	if request.Mode != "survival" {
		t.Fatalf("unexpected defaults: %+v", request)
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
