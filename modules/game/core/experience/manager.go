package experience

import (
	"math/rand"
	"sort"
	"strconv"

	"squad-survival-be/modules/game/core/combat"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/spatial"
	"squad-survival-be/modules/game/core/system"
	"squad-survival-be/modules/game/core/world"
)

type Collection struct {
	Package *entity.ExperiencePackage
	Player  *entity.Player
}

type Manager struct {
	Packages       map[string]*entity.ExperiencePackage
	Known          map[string]map[string]struct{}
	NextID         uint64
	NextRefillTick int64
	Config         Config
	grid           *spatial.Grid
	random         *rand.Rand
}

func NewManager(grid *spatial.Grid, random *rand.Rand, tickRate int) *Manager {
	config := DefaultConfig()
	manager := &Manager{Packages: make(map[string]*entity.ExperiencePackage), Known: make(map[string]map[string]struct{}), Config: config, grid: grid, random: random}
	manager.NextRefillTick = int64(config.RefillIntervalSeconds * tickRate)
	return manager
}

func (m *Manager) TargetCount() int {
	if m == nil {
		return 0
	}
	total := 0
	for _, definition := range m.Config.Packages {
		total += definition.TargetCount
	}
	return total
}

func (m *Manager) SpawnMissing(players map[string]*entity.Player) int {
	if m == nil || m.grid == nil || m.random == nil {
		return 0
	}
	spawned := 0
	for _, definition := range m.Config.Packages {
		tier, _ := Tier(definition.Tier)
		current := 0
		for _, item := range m.Packages {
			if item.Value != nil && item.Value.Tier == tier {
				current++
			}
		}
		for current < definition.TargetCount {
			position, ok := m.randomSpawn(players)
			if !ok {
				break
			}
			m.NextID++
			span := definition.MaxValue - definition.MinValue + 1
			value := definition.MinValue + uint64(m.random.Int63n(int64(span)))
			item := entity.NewExperiencePackage("xp:"+strconv.FormatUint(m.NextID, 10), position, tier, value)
			if err := m.grid.InsertExperiencePackage(item); err != nil {
				continue
			}
			m.Packages[item.ID] = item
			current++
			spawned++
		}
	}
	return spawned
}

func (m *Manager) RefillIfDue(tick int64, tickRate int, players map[string]*entity.Player) bool {
	if m == nil || tick < m.NextRefillTick {
		return false
	}
	m.SpawnMissing(players)
	m.NextRefillTick = tick + int64(m.Config.RefillIntervalSeconds*tickRate)
	return true
}

func (m *Manager) ResetRefill(tick int64, tickRate int) {
	if m != nil {
		m.NextRefillTick = tick + int64(m.Config.RefillIntervalSeconds*tickRate)
	}
}

func (m *Manager) randomSpawn(players map[string]*entity.Player) (entity.Vector2, bool) {
	separation := m.Config.SpawnSeparation
	for attempt := 0; attempt < m.Config.SpawnAttempts; attempt++ {
		position := world.RandomSpawn(m.random)
		if len(m.grid.QueryExperiencePackages(position, separation)) != 0 || len(m.grid.QueryCharacterBoxes(position, separation)) != 0 {
			continue
		}
		valid := true
		for _, player := range players {
			if player != nil && distanceSquared(position, player.Position) < separation*separation {
				valid = false
				break
			}
		}
		if valid {
			return position, true
		}
	}
	return entity.Vector2{}, false
}

type collision struct {
	player          *entity.Player
	distanceSquared float64
}

func (m *Manager) Collect(players map[string]*entity.Player) []Collection {
	if m == nil {
		return nil
	}
	candidates := make(map[string][]collision)
	for _, player := range players {
		if player == nil || player.IsEliminated() {
			continue
		}
		for _, item := range m.grid.QueryExperiencePackages(player.Position, m.Config.PickupRadius) {
			candidates[item.ID] = append(candidates[item.ID], collision{player, distanceSquared(player.Position, item.Position)})
		}
	}
	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	collections := make([]Collection, 0, len(ids))
	for _, id := range ids {
		item := m.Packages[id]
		if item == nil {
			continue
		}
		collisions := candidates[id]
		sort.Slice(collisions, func(i, j int) bool {
			if collisions[i].distanceSquared == collisions[j].distanceSquared {
				return collisions[i].player.SessionID < collisions[j].player.SessionID
			}
			return collisions[i].distanceSquared < collisions[j].distanceSquared
		})
		winner := collisions[0].player
		if item.Value != nil {
			winner.AddExperience(item.Value.Experience)
		}
		delete(m.Packages, id)
		m.grid.RemoveExperiencePackage(id)
		collections = append(collections, Collection{Package: item, Player: winner})
	}
	return collections
}

// VisibilityEvents returns reliable per-observer deltas after applying collections.
func (m *Manager) VisibilityEvents(players map[string]*entity.Player, collections []Collection) map[string][]system.ExperiencePackageEvent {
	result := make(map[string][]system.ExperiencePackageEvent)
	for sessionID := range m.Known {
		if players[sessionID] == nil {
			delete(m.Known, sessionID)
		}
	}
	for _, collection := range collections {
		for sessionID := range players {
			_, knew := m.Known[sessionID][collection.Package.ID]
			if knew || sessionID == collection.Player.SessionID {
				result[sessionID] = append(result[sessionID], system.CollectedExperiencePackage(collection.Package, collection.Player))
			}
			delete(m.Known[sessionID], collection.Package.ID)
		}
	}
	for sessionID, player := range players {
		if player == nil {
			continue
		}
		known := m.Known[sessionID]
		if known == nil {
			known = make(map[string]struct{})
			m.Known[sessionID] = known
		}
		visible := make(map[string]*entity.ExperiencePackage)
		for _, item := range m.grid.QueryExperiencePackages(player.Position, player.DetectionRadius) {
			visible[item.ID] = item
		}
		lost := make([]string, 0)
		for id := range known {
			if visible[id] == nil {
				lost = append(lost, id)
			}
		}
		sort.Strings(lost)
		for _, id := range lost {
			result[sessionID] = append(result[sessionID], system.LostExperiencePackage(id))
			delete(known, id)
		}
		detected := make([]string, 0)
		for id := range visible {
			if _, ok := known[id]; !ok {
				detected = append(detected, id)
			}
		}
		sort.Strings(detected)
		for _, id := range detected {
			result[sessionID] = append(result[sessionID], system.DetectedExperiencePackage(visible[id]))
			known[id] = struct{}{}
		}
	}
	return result
}

func (m *Manager) RemoveObserver(sessionID string) {
	if m != nil {
		delete(m.Known, sessionID)
	}
}

func CharacterCountsByUser(players map[string]*entity.Player) map[string]int {
	counts := make(map[string]int, len(players))
	for _, player := range players {
		if player != nil {
			counts[player.UserID] = player.CharacterCount()
		}
	}
	return counts
}

func AwardKillExperience(players map[string]*entity.Player, counts map[string]int, events []combat.Event, config Config) {
	byUser := make(map[string]*entity.Player, len(players))
	for _, player := range players {
		if player != nil {
			byUser[player.UserID] = player
		}
	}
	for _, event := range events {
		if event.Type != combat.EventCharacterDied {
			continue
		}
		count := counts[event.TargetUserID]
		if killer := byUser[event.AttackerUserID]; killer != nil {
			killer.AddExperience(config.KillExperienceByCharacterCount[count])
		}
		if count > 0 {
			counts[event.TargetUserID] = count - 1
		}
	}
}

func distanceSquared(a, b entity.Vector2) float64 { dx, dy := b.X-a.X, b.Y-a.Y; return dx*dx + dy*dy }
