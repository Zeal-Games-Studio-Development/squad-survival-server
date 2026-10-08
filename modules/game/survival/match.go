package survival

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"time"

	"squad-survival-be/modules/game/core/ai"
	"squad-survival-be/modules/game/core/characterbox"
	"squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/experience"
	"squad-survival-be/modules/game/core/spatial"
	"squad-survival-be/modules/game/core/system"
	"squad-survival-be/modules/game/core/world"
	"squad-survival-be/modules/game/inventory"
	"squad-survival-be/modules/game/matchregistry"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

// Config của mode survival, có thể thay đổi được thông qua params khi tạo match.
const (
	ModuleName                  = "survival"
	DefaultMode                 = "survival"
	MaxPlayers                  = 32
	tickRate                    = entity.TickRate
	reservationTTLSeconds       = 10
	emptyMatchTTLSeconds        = 60
	spatialCellSize             = 20.0
	characterBoxMinCount        = 60
	characterBoxMaxCount        = 80
	characterBoxRefillTicks     = 60 * tickRate
	characterBoxSpawnSeparation = 4.0
	characterBoxSpawnAttempts   = 100
)

type Match struct {
	registry  *matchregistry.Registry
	inventory *inventory.Service
}

type State struct {
	MatchID             string
	Mode                string
	AllowJoinInProgress bool
	Players             map[string]*entity.Player
	AIControllers       map[string]*ai.Controller
	Presences           map[string]runtime.Presence
	Reservations        map[string]int64
	SpatialGrid         *spatial.Grid
	Combat              *combat.Simulation
	RosterVersions      map[string]map[string]uint64
	ProgressionVersions map[string]map[string]uint64
	SkillStateVersions  map[string]map[string]uint64
	Experience          *experience.Manager
	CharacterBoxes      map[string]*entity.CharacterBox
	WeaponCatalog       []entity.Weapon
	NextCharacterBoxID  uint64
	NextBoxRefillTick   int64
	EmptyTicks          int64
	BoxClaims           map[string]characterbox.Claim
	ClaimedBoxBySession map[string]string
	BoxPickupDelays     map[int]int64
	random              *rand.Rand // Gameplay RNG: spawn, weapon, combat, and damage.
	aiRandom            *rand.Rand
}

type Label struct {
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	Joinable    bool   `json:"joinable"`
}

func NewMatchHandler(registry *matchregistry.Registry, inventoryService ...*inventory.Service) func(context.Context, runtime.Logger, *sql.DB, runtime.NakamaModule) (runtime.Match, error) {
	return func(_ context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule) (runtime.Match, error) {
		match := &Match{registry: registry}
		if len(inventoryService) > 0 {
			match.inventory = inventoryService[0]
		}
		return match, nil
	}
}

func (m *Match) MatchInit(ctx context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, params map[string]interface{}) (interface{}, int, string) {
	randomSeed := time.Now().UnixNano()
	weaponCatalog := entity.DefaultWeaponCatalog()
	if err := combat.ValidateWeaponRanges(weaponCatalog, entity.DefaultDetectionRadius, combat.DefaultConfig()); err != nil {
		panic("invalid survival combat configuration: " + err.Error())
	}
	state := &State{
		MatchID:             matchIDFromContext(ctx),
		Mode:                stringParam(params, "mode", DefaultMode),
		AllowJoinInProgress: true,
		Players:             make(map[string]*entity.Player),
		AIControllers:       make(map[string]*ai.Controller),
		Presences:           make(map[string]runtime.Presence),
		Reservations:        make(map[string]int64),
		SpatialGrid:         spatial.NewGrid(spatialCellSize),
		Combat:              combat.NewSimulation(),
		RosterVersions:      make(map[string]map[string]uint64),
		ProgressionVersions: make(map[string]map[string]uint64),
		SkillStateVersions:  make(map[string]map[string]uint64),
		CharacterBoxes:      make(map[string]*entity.CharacterBox),
		WeaponCatalog:       weaponCatalog,
		NextBoxRefillTick:   characterBoxRefillTicks,
		BoxClaims:           make(map[string]characterbox.Claim),
		ClaimedBoxBySession: make(map[string]string),
		BoxPickupDelays:     characterbox.DefaultDelays(),
		random:              rand.New(rand.NewSource(randomSeed)),
		aiRandom:            rand.New(rand.NewSource(randomSeed + 2)),
	}
	initialBoxTarget := state.randomCharacterBoxTarget()
	state.spawnCharacterBoxes(initialBoxTarget)
	if len(state.CharacterBoxes) < initialBoxTarget {
		logger.Warn("Could not place all initial character boxes: target=%d spawned=%d", initialBoxTarget, len(state.CharacterBoxes))
	}
	state.Experience = experience.NewManager(state.SpatialGrid, state.random, tickRate)
	if spawned := state.Experience.SpawnMissing(state.Players); spawned < state.Experience.TargetCount() {
		logger.Warn("Could not place all initial experience packages: spawned=%d target=%d", spawned, state.Experience.TargetCount())
	}
	state.spawnAI()
	logger.Info("Survival match initialized: mode=%s max_players=%d ai_count=%d", state.Mode, MaxPlayers, len(state.AIControllers))
	return state, tickRate, state.label()
}

func (m *Match) MatchJoinAttempt(ctx context.Context, _ runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, _ runtime.MatchDispatcher, tick int64, rawState interface{}, presence runtime.Presence, _ map[string]string) (interface{}, bool, string) {
	state := rawState.(*State)
	state.expireReservations(tick)

	if _, exists := state.Players[presence.GetSessionId()]; exists {
		return state, true, ""
	}
	if state.MatchID != "" && !m.registry.CanJoin(presence.GetUserId(), state.MatchID) {
		return state, false, "already in another match"
	}
	if !state.AllowJoinInProgress && state.humanPlayerCount() > 0 {
		return state, false, "match already started"
	}
	if state.occupiedSlots() >= MaxPlayers {
		return state, false, "match is full"
	}
	if m.inventory != nil {
		if _, err := m.inventory.Load(ctx, nk, presence.GetUserId()); err != nil {
			return state, false, "invalid squad loadout"
		}
	}

	state.Reservations[presence.GetSessionId()] = tick + reservationTTLSeconds*tickRate
	return state, true, ""
}

func (m *Match) MatchJoin(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, rawState interface{}, presences []runtime.Presence) interface{} {
	state := rawState.(*State)
	playerCount := state.humanPlayerCount()
	displayNames, err := resolveDisplayNames(ctx, nk, presences)
	if err != nil && logger != nil {
		logger.Error("Could not load player display names: %v", err)
	}
	for _, presence := range presences {
		delete(state.Reservations, presence.GetSessionId())
		if _, exists := state.Players[presence.GetSessionId()]; exists {
			continue
		}
		var starter entity.Weapon
		var loadout map[string]string
		if m.inventory != nil {
			snapshot, loadErr := m.inventory.Load(ctx, nk, presence.GetUserId())
			if loadErr == nil {
				starter, loadErr = m.inventory.Starter(snapshot, state.random)
				loadout = snapshot.SquadLoadout
			}
			if loadErr != nil {
				if logger != nil {
					logger.Error("Could not load squad loadout: user_id=%s error=%v", presence.GetUserId(), loadErr)
				}
				if kickErr := dispatcher.MatchKick([]runtime.Presence{presence}); kickErr != nil && logger != nil {
					logger.Error("Could not kick presence with invalid loadout: %v", kickErr)
				}
				continue
			}
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
		position := state.randomPlayerSpawn()
		var player *entity.Player
		if m.inventory != nil {
			player = entity.NewPlayerWithWeapon(presence.GetUserId(), presence.GetSessionId(), displayNames[presence.GetUserId()], position, starter)
			player.SetSquadLoadout(loadout)
		} else {
			player = entity.NewPlayer(presence.GetUserId(), presence.GetSessionId(), displayNames[presence.GetUserId()], position, state.random)
		}
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
		state.sendInitialProgression(logger, dispatcher, tick, presence, player)
		state.sendCurrentCharacterBoxes(logger, dispatcher, tick, presence)
	}
	state.broadcastExperiencePackageEvents(logger, dispatcher, tick, nil)
	state.EmptyTicks = 0
	state.updateLabel(dispatcher)
	if state.humanPlayerCount() != playerCount {
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
	playerCount := state.humanPlayerCount()
	boxEvents := make([]system.CharacterBoxEvent, 0, len(presences))
	for _, presence := range presences {
		boxEvents = append(boxEvents, state.cancelBoxClaimForSession(presence.GetSessionId())...)
		delete(state.Reservations, presence.GetSessionId())
		if _, exists := state.Players[presence.GetSessionId()]; exists && !state.SpatialGrid.Remove(presence.GetSessionId()) && logger != nil {
			logger.Error("Could not remove player from spatial grid: session_id=%s", presence.GetSessionId())
		}
		delete(state.Players, presence.GetSessionId())
		delete(state.Presences, presence.GetSessionId())
		state.removeRosterTracking(presence.GetSessionId())
		state.removeProgressionTracking(presence.GetSessionId())
		if state.Experience != nil {
			state.Experience.RemoveObserver(presence.GetSessionId())
		}
		m.registry.RemoveSession(presence.GetSessionId())
	}
	state.broadcastCharacterBoxEvents(logger, dispatcher, tick, boxEvents, nil)
	state.updateLabel(dispatcher)
	if state.humanPlayerCount() != playerCount {
		state.broadcastSnapshot(logger, dispatcher, tick)
	}
	return state
}

func (m *Match) MatchLoop(_ context.Context, logger runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, rawState interface{}, messages []runtime.MatchData) interface{} {
	state := rawState.(*State)
	state.ensureCharacterBoxState(tick)
	if state.expireReservations(tick) {
		state.updateLabel(dispatcher)
	}

	for _, message := range messages {
		if message.GetOpCode() != system.OpMovementInput {
			continue
		}
		player, ok := state.Players[message.GetSessionId()]
		if !ok || state.AIControllers[message.GetSessionId()] != nil || player.IsEliminated() {
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
	}
	state.stepAI(tick)
	for _, player := range state.Players {
		entity.StepMovement(player, tick)
		if err := entity.StepCharactersAtTick(player, tick); err != nil && logger != nil {
			logger.Error("Could not update character strategy: session_id=%s error=%v", player.SessionID, err)
		}
		if err := state.SpatialGrid.Move(player); err != nil && logger != nil {
			logger.Error("Could not move player in spatial grid: session_id=%s error=%v", player.SessionID, err)
		}
	}
	boxEvents := state.collectCharacterBoxes(tick, logger)
	if tick >= state.NextBoxRefillTick {
		target := state.randomCharacterBoxTarget()
		boxEvents = append(boxEvents, state.spawnCharacterBoxes(target)...)
		if len(state.CharacterBoxes) < target && logger != nil {
			logger.Warn("Could not refill all character boxes: target=%d current=%d", target, len(state.CharacterBoxes))
		}
		state.NextBoxRefillTick = tick + characterBoxRefillTicks
	}
	state.broadcastCharacterBoxEvents(logger, dispatcher, tick, boxEvents, nil)
	collections := []experience.Collection(nil)
	if state.Experience != nil {
		collections = state.Experience.Collect(state.Players)
		if state.Experience.RefillIfDue(tick, tickRate, state.Players) && len(state.Experience.Packages) < state.Experience.TargetCount() && logger != nil {
			logger.Warn("Could not refill all experience packages: current=%d target=%d", len(state.Experience.Packages), state.Experience.TargetCount())
		}
	}
	nearbyPlayers := state.queryNearbyPlayers()
	state.broadcastRosterUpdates(logger, dispatcher, tick, nearbyPlayers)
	if state.Combat == nil {
		state.Combat = combat.NewSimulation()
	}
	characterCounts := experience.CharacterCountsByUser(state.Players)
	combatEvents := state.Combat.Step(state.Players, nearbyPlayers, tick, state.random)
	if state.Experience != nil {
		experience.AwardKillExperience(state.Players, characterCounts, combatEvents, state.Experience.Config)
		state.broadcastExperiencePackageEvents(logger, dispatcher, tick, collections)
	}
	state.broadcastProgressionUpdates(logger, dispatcher, tick, nearbyPlayers)
	for _, player := range state.Players {
		if player.IsEliminated() {
			player.Direction = entity.Vector2{}
		}
	}
	state.broadcastCombatEvents(logger, dispatcher, tick, combatEvents, nearbyPlayers)
	state.broadcastSkillStates(logger, dispatcher, tick, nearbyPlayers)
	state.broadcastPlayerMovementSnapshots(logger, dispatcher, tick, nearbyPlayers)
	state.broadcastProjectileMovementSnapshots(logger, dispatcher, tick, nearbyPlayers, state.Combat.Projectiles())

	if state.humanPlayerCount() == 0 && len(state.Reservations) == 0 {
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

func (s *State) broadcastSkillStates(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, nearbyPlayers map[string][]*entity.Player) {
	if s.SkillStateVersions == nil {
		s.SkillStateVersions = make(map[string]map[string]uint64)
	}
	for sessionID, player := range s.Players {
		presence, ok := s.Presences[sessionID]
		if !ok || player == nil {
			continue
		}
		seen := s.SkillStateVersions[sessionID]
		if seen == nil {
			seen = make(map[string]uint64)
			s.SkillStateVersions[sessionID] = seen
		}
		visible := append([]*entity.Player{player}, nearbyPlayers[sessionID]...)
		states := system.ChangedSkillStates(visible, seen)
		if len(states) == 0 {
			continue
		}
		payload, err := system.EncodeSkillStateBatch(tick, states)
		if err == nil {
			err = dispatcher.BroadcastMessage(system.OpSkillStateBatch, payload, []runtime.Presence{presence}, nil, true)
		}
		if err != nil {
			if logger != nil {
				logger.Error("Could not send skill state: session_id=%s error=%v", sessionID, err)
			}
			continue
		}
		for _, state := range states {
			seen[system.SkillStateKey(state)] = state.Version
		}
	}
}

func (s *State) ensureCharacterBoxState(tick int64) {
	if s.CharacterBoxes == nil {
		s.CharacterBoxes = make(map[string]*entity.CharacterBox)
	}
	if s.BoxClaims == nil {
		s.BoxClaims = make(map[string]characterbox.Claim)
	}
	if s.ClaimedBoxBySession == nil {
		s.ClaimedBoxBySession = make(map[string]string)
	}
	if s.BoxPickupDelays == nil {
		s.BoxPickupDelays = characterbox.DefaultDelays()
	}
	if len(s.WeaponCatalog) == 0 {
		s.WeaponCatalog = entity.DefaultWeaponCatalog()
	}
	if s.random == nil {
		s.random = rand.New(rand.NewSource(tick))
	}
	if s.NextBoxRefillTick == 0 {
		s.NextBoxRefillTick = tick + characterBoxRefillTicks
	}
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
	return s.humanPlayerCount() + len(s.Reservations)
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
	snapshot, err := system.EncodeStateSnapshot(tick, s.humanPlayerCount())
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

func (s *State) randomCharacterBoxTarget() int {
	return characterBoxMinCount + s.random.Intn(characterBoxMaxCount-characterBoxMinCount+1)
}

func (s *State) spawnCharacterBoxes(target int) []system.CharacterBoxEvent {
	if target <= len(s.CharacterBoxes) || len(s.WeaponCatalog) == 0 {
		return nil
	}
	weaponTypes := make([]entity.WeaponType, 0, len(s.WeaponCatalog))
	seenTypes := make(map[entity.WeaponType]bool, len(s.WeaponCatalog))
	for _, weapon := range s.WeaponCatalog {
		if !seenTypes[weapon.Type] {
			weaponTypes = append(weaponTypes, weapon.Type)
			seenTypes[weapon.Type] = true
		}
	}
	events := make([]system.CharacterBoxEvent, 0, target-len(s.CharacterBoxes))
	for len(s.CharacterBoxes) < target {
		position, ok := s.randomCharacterBoxSpawn()
		if !ok {
			break
		}
		s.NextCharacterBoxID++
		weaponType := weaponTypes[s.random.Intn(len(weaponTypes))]
		box := entity.NewCharacterBox("box:"+strconv.FormatUint(s.NextCharacterBoxID, 10), position, weaponType)
		if err := s.SpatialGrid.InsertCharacterBox(box); err != nil {
			continue
		}
		s.CharacterBoxes[box.ID] = box
		events = append(events, system.SpawnedCharacterBox(box))
	}
	return events
}

func (s *State) randomCharacterBoxSpawn() (entity.Vector2, bool) {
	for range characterBoxSpawnAttempts {
		position := world.RandomSpawn(s.random)
		if len(s.SpatialGrid.QueryCharacterBoxes(position, characterBoxSpawnSeparation)) != 0 {
			continue
		}
		if s.Experience != nil && len(s.SpatialGrid.QueryExperiencePackages(position, characterBoxSpawnSeparation)) != 0 {
			continue
		}
		blocked := false
		for _, player := range s.Players {
			if player != nil && distanceSquared(position, player.Position) < characterBoxSpawnSeparation*characterBoxSpawnSeparation {
				blocked = true
				break
			}
		}
		if !blocked {
			return position, true
		}
	}
	return entity.Vector2{}, false
}

func (s *State) randomPlayerSpawn() entity.Vector2 {
	for range characterBoxSpawnAttempts {
		position := world.RandomSpawn(s.random)
		if len(s.SpatialGrid.QueryCharacterBoxes(position, characterBoxSpawnSeparation)) == 0 && (s.Experience == nil || len(s.SpatialGrid.QueryExperiencePackages(position, characterBoxSpawnSeparation)) == 0) {
			return position
		}
	}
	return world.RandomSpawn(s.random)
}

type boxCollisionCandidate struct {
	player          *entity.Player
	distanceSquared float64
}

func (s *State) collectCharacterBoxes(tick int64, logger runtime.Logger) []system.CharacterBoxEvent {
	s.ensureCharacterBoxState(tick)
	candidates := make(map[string][]boxCollisionCandidate)
	for _, player := range s.Players {
		if player == nil || player.IsEliminated() {
			continue
		}
		for _, box := range s.SpatialGrid.QueryCharacterBoxes(player.Position, entity.CharacterBoxCollisionRadius) {
			candidates[box.ID] = append(candidates[box.ID], boxCollisionCandidate{
				player: player, distanceSquared: distanceSquared(player.Position, box.Position),
			})
		}
	}
	events := make([]system.CharacterBoxEvent, 0, len(candidates))
	claimedBoxIDs := make([]string, 0, len(s.BoxClaims))
	for boxID := range s.BoxClaims {
		claimedBoxIDs = append(claimedBoxIDs, boxID)
	}
	sort.Strings(claimedBoxIDs)
	completedSessions := make(map[string]struct{})
	for _, boxID := range claimedBoxIDs {
		claim := s.BoxClaims[boxID]
		box := s.CharacterBoxes[boxID]
		player := s.Players[claim.SessionID]
		valid := box != nil && player != nil && !player.IsEliminated() && player.CharacterCount() < player.MaxCharacters() && distanceSquared(player.Position, box.Position) <= entity.CharacterBoxCollisionRadius*entity.CharacterBoxCollisionRadius
		if !valid {
			events = append(events, s.cancelBoxClaim(boxID)...)
			continue
		}
		if tick < claim.CompletesAtTick {
			continue
		}
		weapon, ok := s.weaponForBox(player, box)
		if !ok {
			if logger != nil {
				logger.Error("Character box has no loadout weapon: box_id=%s user_id=%s weapon_type=%s", box.ID, player.UserID, box.WeaponType())
			}
			events = append(events, s.cancelBoxClaim(boxID)...)
			continue
		}
		character := entity.NewCharacter()
		character.ApplyWeapon(weapon)
		if err := player.AddCharacter(character); err != nil {
			events = append(events, s.cancelBoxClaim(boxID)...)
			continue
		}
		delete(s.BoxClaims, boxID)
		delete(s.ClaimedBoxBySession, player.SessionID)
		delete(s.CharacterBoxes, boxID)
		s.SpatialGrid.RemoveCharacterBox(boxID)
		completedSessions[player.SessionID] = struct{}{}
		events = append(events, system.DespawnedCharacterBox(boxID))
	}

	boxIDs := make([]string, 0, len(candidates))
	for boxID := range candidates {
		if _, claimed := s.BoxClaims[boxID]; !claimed {
			boxIDs = append(boxIDs, boxID)
		}
	}
	sort.Strings(boxIDs)
	for _, boxID := range boxIDs {
		box := s.CharacterBoxes[boxID]
		if box == nil {
			continue
		}
		collisions := candidates[boxID]
		sort.Slice(collisions, func(i, j int) bool {
			if collisions[i].distanceSquared == collisions[j].distanceSquared {
				return collisions[i].player.SessionID < collisions[j].player.SessionID
			}
			return collisions[i].distanceSquared < collisions[j].distanceSquared
		})
		for _, collision := range collisions {
			if collision.player.CharacterCount() >= collision.player.MaxCharacters() {
				continue
			}
			if _, claimed := s.ClaimedBoxBySession[collision.player.SessionID]; claimed {
				continue
			}
			if _, completed := completedSessions[collision.player.SessionID]; completed {
				continue
			}
			delay, ok := characterbox.DelayTicks(s.BoxPickupDelays, collision.player.CharacterCount(), tickRate)
			if !ok {
				continue
			}
			claim := characterbox.Claim{BoxID: box.ID, SessionID: collision.player.SessionID, StartedAtTick: tick, CompletesAtTick: tick + delay}
			s.BoxClaims[box.ID] = claim
			s.ClaimedBoxBySession[collision.player.SessionID] = box.ID
			events = append(events, system.CharacterBoxPickupStarted(box.ID, claim.SessionID, claim.StartedAtTick, claim.CompletesAtTick))
			break
		}
	}
	return events
}

func (s *State) cancelBoxClaim(boxID string) []system.CharacterBoxEvent {
	claim, ok := s.BoxClaims[boxID]
	if !ok {
		return nil
	}
	delete(s.BoxClaims, boxID)
	delete(s.ClaimedBoxBySession, claim.SessionID)
	return []system.CharacterBoxEvent{system.CharacterBoxPickupCancelled(boxID, claim.SessionID)}
}

func (s *State) cancelBoxClaimForSession(sessionID string) []system.CharacterBoxEvent {
	boxID, ok := s.ClaimedBoxBySession[sessionID]
	if !ok {
		return nil
	}
	return s.cancelBoxClaim(boxID)
}

func (s *State) weaponByID(weaponID string) (entity.Weapon, bool) {
	for _, weapon := range s.WeaponCatalog {
		if weapon.ID == weaponID {
			return weapon, true
		}
	}
	return entity.Weapon{}, false
}

func (s *State) weaponForBox(player *entity.Player, box *entity.CharacterBox) (entity.Weapon, bool) {
	if player == nil || box == nil {
		return entity.Weapon{}, false
	}
	weapon, ok := s.weaponByID(player.SquadLoadout[box.WeaponType()])
	return weapon, ok && weapon.Type == box.WeaponType()
}

func (s *State) sendCurrentCharacterBoxes(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, presence runtime.Presence) {
	boxIDs := make([]string, 0, len(s.CharacterBoxes))
	for boxID := range s.CharacterBoxes {
		boxIDs = append(boxIDs, boxID)
	}
	sort.Strings(boxIDs)
	events := make([]system.CharacterBoxEvent, 0, len(boxIDs)+len(s.BoxClaims))
	for _, boxID := range boxIDs {
		events = append(events, system.SpawnedCharacterBox(s.CharacterBoxes[boxID]))
	}
	for _, boxID := range boxIDs {
		if claim, ok := s.BoxClaims[boxID]; ok {
			events = append(events, system.CharacterBoxPickupStarted(boxID, claim.SessionID, claim.StartedAtTick, claim.CompletesAtTick))
		}
	}
	s.broadcastCharacterBoxEvents(logger, dispatcher, tick, events, []runtime.Presence{presence})
}

func (s *State) broadcastCharacterBoxEvents(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, events []system.CharacterBoxEvent, presences []runtime.Presence) {
	if len(events) == 0 {
		return
	}
	payload, err := system.EncodeCharacterBoxStateBatch(tick, events)
	if err == nil {
		err = dispatcher.BroadcastMessage(system.OpCharacterBoxState, payload, presences, nil, true)
	}
	if err != nil && logger != nil {
		logger.Error("Could not send character box state: %v", err)
	}
}

func (s *State) broadcastExperiencePackageEvents(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, collections []experience.Collection) {
	if s.Experience == nil {
		return
	}
	for sessionID, events := range s.Experience.VisibilityEvents(s.humanObservers(), collections) {
		if len(events) == 0 {
			continue
		}
		presence, ok := s.Presences[sessionID]
		if !ok {
			continue
		}
		payload, err := system.EncodeExperiencePackageStateBatch(tick, events)
		if err == nil {
			err = dispatcher.BroadcastMessage(system.OpExperiencePackageState, payload, []runtime.Presence{presence}, nil, true)
		}
		if err != nil && logger != nil {
			logger.Error("Could not send experience package state: session_id=%s error=%v", sessionID, err)
		}
	}
}

func distanceSquared(a, b entity.Vector2) float64 {
	deltaX := b.X - a.X
	deltaY := b.Y - a.Y
	return math.FMA(deltaX, deltaX, deltaY*deltaY)
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
			if logger != nil && s.AIControllers[sessionID] == nil {
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
			if logger != nil && s.AIControllers[sessionID] == nil {
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
	delete(s.SkillStateVersions, sessionID)
	for _, sent := range s.RosterVersions {
		delete(sent, sessionID)
	}
}

func (s *State) sendInitialProgression(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, presence runtime.Presence, player *entity.Player) {
	payload, err := system.EncodePlayerProgressionBatch(tick, []*entity.Player{player})
	if err == nil {
		err = dispatcher.BroadcastMessage(system.OpPlayerProgressionBatch, payload, []runtime.Presence{presence}, nil, true)
	}
	if err != nil {
		if logger != nil {
			logger.Error("Could not send initial player progression: session_id=%s error=%v", player.SessionID, err)
		}
		return
	}
	if s.ProgressionVersions == nil {
		s.ProgressionVersions = make(map[string]map[string]uint64)
	}
	s.ProgressionVersions[player.SessionID] = map[string]uint64{player.SessionID: player.ProgressionVersion}
}

func (s *State) broadcastProgressionUpdates(logger runtime.Logger, dispatcher runtime.MatchDispatcher, tick int64, nearbyPlayers map[string][]*entity.Player) {
	if s.ProgressionVersions == nil {
		s.ProgressionVersions = make(map[string]map[string]uint64)
	}
	for observerSessionID, observer := range s.Players {
		presence, ok := s.Presences[observerSessionID]
		if !ok {
			continue
		}
		sent := s.ProgressionVersions[observerSessionID]
		if sent == nil {
			sent = make(map[string]uint64)
			s.ProgressionVersions[observerSessionID] = sent
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
			if version, ok := sent[targetSessionID]; !ok || version != player.ProgressionVersion {
				pending = append(pending, player)
			}
		}
		if len(pending) == 0 {
			continue
		}
		sort.Slice(pending, func(i, j int) bool { return pending[i].SessionID < pending[j].SessionID })
		payload, err := system.EncodePlayerProgressionBatch(tick, pending)
		if err == nil {
			err = dispatcher.BroadcastMessage(system.OpPlayerProgressionBatch, payload, []runtime.Presence{presence}, nil, true)
		}
		if err != nil {
			if logger != nil {
				logger.Error("Could not send player progression: session_id=%s error=%v", observerSessionID, err)
			}
			continue
		}
		for _, player := range pending {
			sent[player.SessionID] = player.ProgressionVersion
		}
	}
}

func (s *State) removeProgressionTracking(sessionID string) {
	delete(s.ProgressionVersions, sessionID)
	for _, sent := range s.ProgressionVersions {
		delete(sent, sessionID)
	}
}

func (s *State) label() string {
	label, _ := json.Marshal(Label{
		Mode:        s.Mode,
		Status:      "playing",
		PlayerCount: s.humanPlayerCount(),
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

func matchIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	matchID, _ := ctx.Value(runtime.RUNTIME_CTX_MATCH_ID).(string)
	return matchID
}
