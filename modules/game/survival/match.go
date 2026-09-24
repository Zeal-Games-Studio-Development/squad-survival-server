package survival

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/rand"
	"sort"
	"time"

	"squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/spatial"
	"squad-survival-be/modules/game/core/system"
	"squad-survival-be/modules/game/core/world"
	"squad-survival-be/modules/game/matchregistry"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

// Config của mode survival, có thể thay đổi được thông qua params khi tạo match.
const (
	ModuleName            = "survival"
	DefaultMode           = "survival"
	MaxPlayers            = 32
	tickRate              = entity.TickRate
	reservationTTLSeconds = 10
	emptyMatchTTLSeconds  = 60
	spatialCellSize       = 20.0
)

type Match struct {
	registry *matchregistry.Registry
}

type State struct {
	MatchID             string
	Mode                string
	AllowJoinInProgress bool
	Players             map[string]*entity.Player
	Presences           map[string]runtime.Presence
	Reservations        map[string]int64
	SpatialGrid         *spatial.Grid
	Combat              *combat.Simulation
	RosterVersions      map[string]map[string]uint64
	EmptyTicks          int64
	random              *rand.Rand
}

type Label struct {
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	Joinable    bool   `json:"joinable"`
}

func NewMatchHandler(registry *matchregistry.Registry) func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule) (runtime.Match, error) {
	return func(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule) (runtime.Match, error) {
		return &Match{registry: registry}, nil
	}
}

func (m *Match) MatchInit(ctx context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, params map[string]interface{}) (interface{}, int, string) {
	state := &State{
		MatchID:             matchIDFromContext(ctx),
		Mode:                stringParam(params, "mode", DefaultMode),
		AllowJoinInProgress: boolParam(params, "allow_join_in_progress", true),
		Players:             make(map[string]*entity.Player),
		Presences:           make(map[string]runtime.Presence),
		Reservations:        make(map[string]int64),
		SpatialGrid:         spatial.NewGrid(spatialCellSize),
		Combat:              combat.NewSimulation(),
		RosterVersions:      make(map[string]map[string]uint64),
		random:              rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	logger.Info("Survival match initialized: mode=%s max_players=%d", state.Mode, MaxPlayers)
	return state, tickRate, state.label()
}

func (m *Match) MatchJoinAttempt(ctx context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, _ runtime.MatchDispatcher, tick int64, rawState interface{}, presence runtime.Presence, _ map[string]string) (interface{}, bool, string) {
	state := rawState.(*State)
	state.expireReservations(tick)

	if _, exists := state.Players[presence.GetSessionId()]; exists {
		return state, true, ""
	}
	if state.MatchID != "" && !m.registry.CanJoin(presence.GetUserId(), state.MatchID) {
		return state, false, "already in another match"
	}
	if !state.AllowJoinInProgress && len(state.Players) > 0 {
		return state, false, "match already started"
	}
	if state.occupiedSlots() >= MaxPlayers {
		return state, false, "match is full"
	}

	state.Reservations[presence.GetSessionId()] = tick + reservationTTLSeconds*tickRate
	return state, true, ""
}

func (m *Match) MatchJoin(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, rawState interface{}, presences []runtime.Presence) interface{} {
	state := rawState.(*State)
	playerCount := len(state.Players)
	displayNames, err := resolveDisplayNames(ctx, nk, presences)
	if err != nil && logger != nil {
		logger.Error("Could not load player display names: %v", err)
	}
	for _, presence := range presences {
		delete(state.Reservations, presence.GetSessionId())
		if _, exists := state.Players[presence.GetSessionId()]; exists {
			continue
		}
		if state.MatchID != "" && !m.registry.Add(presence.GetUserId(), presence.GetSessionId(), state.MatchID) {
			if logger != nil {
				logger.Error("Could not register active match membership: user_id=%s session_id=%s match_id=%s", presence.GetUserId(), presence.GetSessionId(), state.MatchID)
			}
			if err = dispatcher.MatchKick([]runtime.Presence{presence}); err != nil && logger != nil {
				logger.Error("Could not kick duplicate match presence: session_id=%s error=%v", presence.GetSessionId(), err)
			}
			continue
		}
		player := entity.NewPlayer(
			presence.GetUserId(),
			presence.GetSessionId(),
			displayNames[presence.GetUserId()],
			world.RandomSpawn(state.random),
			state.random,
		)
		if err = state.SpatialGrid.Insert(player); err != nil {
			m.registry.RemoveSession(presence.GetSessionId())
			if logger != nil {
				logger.Error("Could not insert player into spatial grid: session_id=%s error=%v", presence.GetSessionId(), err)
			}
			continue
		}
		state.Players[presence.GetSessionId()] = player
		state.Presences[presence.GetSessionId()] = presence
		state.sendInitialRoster(logger, dispatcher, tick, presence, player)
	}
	state.EmptyTicks = 0
	state.updateLabel(dispatcher)
	if len(state.Players) != playerCount {
		state.broadcastSnapshot(logger, dispatcher, tick)
	}
	return state
}

type userLookup interface {
	UsersGetId(ctx context.Context, userIDs []string, facebookIDs []string) ([]*api.User, error)
}

func resolveDisplayNames(ctx context.Context, lookup userLookup, presences []runtime.Presence) (map[string]string, error) {
	displayNames := make(map[string]string, len(presences))
	userIDs := make([]string, 0, len(presences))
	for _, presence := range presences {
		displayNames[presence.GetUserId()] = presence.GetUsername()
		userIDs = append(userIDs, presence.GetUserId())
	}
	if lookup == nil || len(userIDs) == 0 {
		return displayNames, nil
	}

	users, err := lookup.UsersGetId(ctx, userIDs, nil)
	if err != nil {
		return displayNames, err
	}
	for _, user := range users {
		if user.GetDisplayName() != "" {
			displayNames[user.GetId()] = user.GetDisplayName()
		}
	}
	return displayNames, nil
}

func (m *Match) MatchLeave(_ context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, rawState interface{}, presences []runtime.Presence) interface{} {
	state := rawState.(*State)
	playerCount := len(state.Players)
	for _, presence := range presences {
		delete(state.Reservations, presence.GetSessionId())
		if _, exists := state.Players[presence.GetSessionId()]; exists && !state.SpatialGrid.Remove(presence.GetSessionId()) && logger != nil {
			logger.Error("Could not remove player from spatial grid: session_id=%s", presence.GetSessionId())
		}
		delete(state.Players, presence.GetSessionId())
		delete(state.Presences, presence.GetSessionId())
		state.removeRosterTracking(presence.GetSessionId())
		m.registry.RemoveSession(presence.GetSessionId())
	}
	state.updateLabel(dispatcher)
	if len(state.Players) != playerCount {
		state.broadcastSnapshot(logger, dispatcher, tick)
	}
	return state
}

func (m *Match) MatchLoop(_ context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, rawState interface{}, messages []runtime.MatchData) interface{} {
	state := rawState.(*State)
	if state.expireReservations(tick) {
		state.updateLabel(dispatcher)
	}

	for _, message := range messages {
		if message.GetOpCode() != system.OpMovementInput {
			continue
		}
		player, ok := state.Players[message.GetSessionId()]
		if !ok {
			continue
		}
		input, err := entity.DecodeMovementInput(message.GetData())
		if err != nil {
			continue
		}
		entity.ApplyMovementInput(player, input, tick)
	}

	for _, player := range state.Players {
		player.RemoveDeadCharacters()
		entity.StepMovement(player, tick)
		if err := entity.StepCharacters(player); err != nil && logger != nil {
			logger.Error("Could not update character strategy: session_id=%s error=%v", player.SessionID, err)
		}
		if err := state.SpatialGrid.Move(player); err != nil && logger != nil {
			logger.Error("Could not move player in spatial grid: session_id=%s error=%v", player.SessionID, err)
		}
	}
	nearbyPlayers := state.queryNearbyPlayers()
	state.broadcastRosterUpdates(logger, dispatcher, tick, nearbyPlayers)
	if state.Combat == nil {
		state.Combat = combat.NewSimulation()
	}
	combatEvents := state.Combat.Step(state.Players, tick, state.random)
	state.broadcastCombatEvents(logger, dispatcher, tick, combatEvents, nearbyPlayers)
	state.broadcastPlayerMovementSnapshots(logger, dispatcher, tick, nearbyPlayers)
	state.broadcastProjectileMovementSnapshots(logger, dispatcher, tick, nearbyPlayers, state.Combat.Projectiles())

	if len(state.Players) == 0 && len(state.Reservations) == 0 {
		state.EmptyTicks++
		if state.EmptyTicks >= emptyMatchTTLSeconds*tickRate {
			logger.Info("Stopping empty survival match")
			m.registry.RemoveMatch(state.MatchID)
			return nil
		}
	} else {
		state.EmptyTicks = 0
	}

	return state
}

func (m *Match) MatchTerminate(ctx context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, _ runtime.MatchDispatcher, _ int64, rawState interface{}, _ int) interface{} {
	matchID := matchIDFromContext(ctx)
	if state, ok := rawState.(*State); ok && state.MatchID != "" {
		matchID = state.MatchID
	}
	m.registry.RemoveMatch(matchID)
	return nil
}

func (m *Match) MatchSignal(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, _ runtime.MatchDispatcher, _ int64, rawState interface{}, _ string) (interface{}, string) {
	state := rawState.(*State)
	return state, state.label()
}

func (s *State) occupiedSlots() int {
	return len(s.Players) + len(s.Reservations)
}

func (s *State) expireReservations(tick int64) bool {
	expired := false
	for sessionID, expiresAt := range s.Reservations {
		if tick >= expiresAt {
			delete(s.Reservations, sessionID)
			expired = true
		}
	}
	return expired
}

func (s *State) updateLabel(dispatcher runtime.MatchDispatcher) {
	_ = dispatcher.MatchLabelUpdate(s.label())
}

func (s *State) broadcastSnapshot(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64) {
	snapshot, err := system.EncodeStateSnapshot(tick, len(s.Players))
	if err == nil {
		err = dispatcher.BroadcastMessage(system.OpStateSnapshot, snapshot, nil, nil, true)
	}
	if err != nil && logger != nil {
		logger.Error("Could not broadcast state snapshot: %v", err)
	}
}

func (s *State) queryNearbyPlayers() map[string][]*entity.Player {
	nearbyPlayers := make(map[string][]*entity.Player, len(s.Players))
	for sessionID, player := range s.Players {
		nearbyPlayers[sessionID] = s.SpatialGrid.QueryPlayers(player)
	}
	return nearbyPlayers
}

func (s *State) broadcastCombatEvents(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, events []combat.Event, nearbyPlayers map[string][]*entity.Player) {
	if len(events) == 0 {
		return
	}
	for sessionID, player := range s.Players {
		presence, ok := s.Presences[sessionID]
		if !ok {
			continue
		}
		relevant := relevantCombatEvents(player, nearbyPlayers[sessionID], events)
		if len(relevant) == 0 {
			continue
		}
		payload, err := system.EncodeCombatEventBatch(tick, relevant)
		if err == nil {
			err = dispatcher.BroadcastMessage(system.OpCombatEventBatch, payload, []runtime.Presence{presence}, nil, true)
		}
		if err != nil && logger != nil {
			logger.Error("Could not send combat events: session_id=%s error=%v", sessionID, err)
		}
	}
}

func relevantCombatEvents(player *entity.Player, nearby []*entity.Player, events []combat.Event) []combat.Event {
	visibleUsers := make(map[string]struct{}, len(nearby)+1)
	visibleUsers[player.UserID] = struct{}{}
	for _, detected := range nearby {
		if detected != nil {
			visibleUsers[detected.UserID] = struct{}{}
		}
	}
	relevant := make([]combat.Event, 0)
	for _, event := range events {
		_, seesAttacker := visibleUsers[event.AttackerUserID]
		_, seesTarget := visibleUsers[event.TargetUserID]
		if seesAttacker || seesTarget {
			relevant = append(relevant, event)
		}
	}
	return relevant
}

func relevantProjectiles(player *entity.Player, nearby []*entity.Player, projectiles []*combat.Projectile) []*combat.Projectile {
	visibleUsers := make(map[string]struct{}, len(nearby)+1)
	visibleUsers[player.UserID] = struct{}{}
	for _, detected := range nearby {
		if detected != nil {
			visibleUsers[detected.UserID] = struct{}{}
		}
	}
	relevant := make([]*combat.Projectile, 0)
	for _, projectile := range projectiles {
		_, seesAttacker := visibleUsers[projectile.AttackerUserID]
		_, seesTarget := visibleUsers[projectile.TargetUserID]
		if seesAttacker || seesTarget {
			relevant = append(relevant, projectile)
		}
	}
	return relevant
}

func (s *State) broadcastPlayerMovementSnapshots(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, nearbyPlayers map[string][]*entity.Player) {
	for sessionID, player := range s.Players {
		presence, ok := s.Presences[sessionID]
		if !ok {
			if logger != nil {
				logger.Error("Could not send player movement snapshot: presence not found for session_id=%s", sessionID)
			}
			continue
		}

		payload, err := system.EncodePlayerMovementSnapshot(tick, player, nearbyPlayers[sessionID])
		if err == nil {
			err = dispatcher.BroadcastMessage(system.OpPlayerMovementSnapshot, payload, []runtime.Presence{presence}, nil, false)
		}
		if err != nil && logger != nil {
			logger.Error("Could not send player movement snapshot: session_id=%s error=%v", sessionID, err)
		}
	}
}

func (s *State) broadcastProjectileMovementSnapshots(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, nearbyPlayers map[string][]*entity.Player, projectiles []*combat.Projectile) {
	for sessionID, player := range s.Players {
		presence, ok := s.Presences[sessionID]
		if !ok {
			if logger != nil {
				logger.Error("Could not send projectile movement snapshot: presence not found for session_id=%s", sessionID)
			}
			continue
		}
		payload, err := system.EncodeProjectileMovementSnapshot(tick, relevantProjectiles(player, nearbyPlayers[sessionID], projectiles))
		if err == nil {
			err = dispatcher.BroadcastMessage(system.OpProjectileMovementSnapshot, payload, []runtime.Presence{presence}, nil, false)
		}
		if err != nil && logger != nil {
			logger.Error("Could not send projectile movement snapshot: session_id=%s error=%v", sessionID, err)
		}
	}
}

func (s *State) sendInitialRoster(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, presence runtime.Presence, player *entity.Player) {
	payload, err := system.EncodePlayerRosterBatch(tick, []*entity.Player{player})
	if err == nil {
		err = dispatcher.BroadcastMessage(system.OpPlayerRosterBatch, payload, []runtime.Presence{presence}, nil, true)
	}
	if err != nil {
		if logger != nil {
			logger.Error("Could not send initial player roster: session_id=%s error=%v", player.SessionID, err)
		}
		return
	}
	if s.RosterVersions == nil {
		s.RosterVersions = make(map[string]map[string]uint64)
	}
	s.RosterVersions[player.SessionID] = map[string]uint64{player.SessionID: player.RosterVersion}
}

func (s *State) broadcastRosterUpdates(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, nearbyPlayers map[string][]*entity.Player) {
	if s.RosterVersions == nil {
		s.RosterVersions = make(map[string]map[string]uint64)
	}
	for observerSessionID, observer := range s.Players {
		presence, ok := s.Presences[observerSessionID]
		if !ok {
			continue
		}
		sent := s.RosterVersions[observerSessionID]
		if sent == nil {
			sent = make(map[string]uint64)
			s.RosterVersions[observerSessionID] = sent
		}

		visible := make(map[string]*entity.Player, len(nearbyPlayers[observerSessionID])+1)
		visible[observerSessionID] = observer
		for _, player := range nearbyPlayers[observerSessionID] {
			if player != nil {
				visible[player.SessionID] = player
			}
		}
		for targetSessionID := range sent {
			if _, ok := visible[targetSessionID]; !ok {
				delete(sent, targetSessionID)
			}
		}

		pending := make([]*entity.Player, 0, len(visible))
		for targetSessionID, player := range visible {
			if version, ok := sent[targetSessionID]; !ok || version != player.RosterVersion {
				pending = append(pending, player)
			}
		}
		if len(pending) == 0 {
			continue
		}
		sort.Slice(pending, func(i, j int) bool { return pending[i].SessionID < pending[j].SessionID })
		payload, err := system.EncodePlayerRosterBatch(tick, pending)
		if err == nil {
			err = dispatcher.BroadcastMessage(system.OpPlayerRosterBatch, payload, []runtime.Presence{presence}, nil, true)
		}
		if err != nil {
			if logger != nil {
				logger.Error("Could not send player roster: session_id=%s error=%v", observerSessionID, err)
			}
			continue
		}
		for _, player := range pending {
			sent[player.SessionID] = player.RosterVersion
		}
	}
}

func (s *State) removeRosterTracking(sessionID string) {
	delete(s.RosterVersions, sessionID)
	for _, sent := range s.RosterVersions {
		delete(sent, sessionID)
	}
}

func (s *State) label() string {
	label, _ := json.Marshal(Label{
		Mode:        s.Mode,
		Status:      "playing",
		PlayerCount: len(s.Players),
		MaxPlayers:  MaxPlayers,
		Joinable:    s.AllowJoinInProgress && s.occupiedSlots() < MaxPlayers,
	})
	return string(label)
}

func stringParam(params map[string]interface{}, key, fallback string) string {
	if value, ok := params[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func boolParam(params map[string]interface{}, key string, fallback bool) bool {
	if value, ok := params[key].(bool); ok {
		return value
	}
	return fallback
}

func matchIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	matchID, _ := ctx.Value(runtime.RUNTIME_CTX_MATCH_ID).(string)
	return matchID
}
