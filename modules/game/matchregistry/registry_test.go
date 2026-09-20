package matchregistry

import "testing"

func TestRegistryLifecycle(t *testing.T) {
	registry := New()
	if _, ok := registry.MatchForUser("user-1"); ok {
		t.Fatal("expected user to start without an active match")
	}
	if !registry.Add("user-1", "session-1", "match-1") {
		t.Fatal("expected first membership to be added")
	}
	if !registry.Add("user-1", "session-2", "match-1") {
		t.Fatal("expected another session to join the same match")
	}
	if registry.Add("user-1", "session-3", "match-2") {
		t.Fatal("expected a different match to be rejected")
	}
	if matchID, ok := registry.MatchForUser("user-1"); !ok || matchID != "match-1" {
		t.Fatalf("unexpected active match: %q, %v", matchID, ok)
	}
	if !registry.RemoveSession("session-1") {
		t.Fatal("expected session removal")
	}
	if _, ok := registry.MatchForUser("user-1"); !ok {
		t.Fatal("expected the second session to keep the user active")
	}
	if removed := registry.RemoveMatch("match-1"); removed != 1 {
		t.Fatalf("expected one remaining session removed, got %d", removed)
	}
	if _, ok := registry.MatchForUser("user-1"); ok {
		t.Fatal("expected match cleanup to remove the user")
	}
}
