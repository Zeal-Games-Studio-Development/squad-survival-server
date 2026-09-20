package matchregistry

import "sync"

// Registry tracks authoritative match membership for the lifetime of this module process.
type Registry struct {
	mu        sync.RWMutex
	byUser    map[string]map[string]string
	bySession map[string]membership
}

type membership struct {
	userID  string
	matchID string
}

func New() *Registry {
	return &Registry{
		byUser:    make(map[string]map[string]string),
		bySession: make(map[string]membership),
	}
}

// MatchForUser returns the active match for a user. All sessions for a user are
// constrained to the same match.
func (r *Registry) MatchForUser(userID string) (string, bool) {
	if r == nil || userID == "" {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, matchID := range r.byUser[userID] {
		return matchID, true
	}
	return "", false
}

// CanJoin reports whether the user has no active match or is joining the same match.
func (r *Registry) CanJoin(userID, matchID string) bool {
	activeMatchID, ok := r.MatchForUser(userID)
	return !ok || activeMatchID == matchID
}

// Add records a successful join. It returns false if the user is active in another match.
func (r *Registry) Add(userID, sessionID, matchID string) bool {
	if r == nil || userID == "" || sessionID == "" || matchID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, activeMatchID := range r.byUser[userID] {
		if activeMatchID != matchID {
			return false
		}
	}
	if previous, ok := r.bySession[sessionID]; ok {
		if previous.userID != userID || previous.matchID != matchID {
			return false
		}
		return true
	}
	if r.byUser[userID] == nil {
		r.byUser[userID] = make(map[string]string)
	}
	r.byUser[userID][sessionID] = matchID
	r.bySession[sessionID] = membership{userID: userID, matchID: matchID}
	return true
}

func (r *Registry) RemoveSession(sessionID string) bool {
	if r == nil || sessionID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.bySession[sessionID]
	if !ok {
		return false
	}
	delete(r.bySession, sessionID)
	delete(r.byUser[entry.userID], sessionID)
	if len(r.byUser[entry.userID]) == 0 {
		delete(r.byUser, entry.userID)
	}
	return true
}

func (r *Registry) RemoveMatch(matchID string) int {
	if r == nil || matchID == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	for sessionID, entry := range r.bySession {
		if entry.matchID != matchID {
			continue
		}
		delete(r.bySession, sessionID)
		delete(r.byUser[entry.userID], sessionID)
		if len(r.byUser[entry.userID]) == 0 {
			delete(r.byUser, entry.userID)
		}
		removed++
	}
	return removed
}
