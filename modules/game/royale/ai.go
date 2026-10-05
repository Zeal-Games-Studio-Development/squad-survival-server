package royale

import (
	"math/rand"
	"sort"
	"strconv"
	"time"

	"squad-survival-be/modules/game/core/ai"
	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/skin/loadout"
)

func (s *State) spawnAI() {
	config := ai.DefaultConfig()
	count := config.BattleRoyaleCount
	namespace := s.MatchID
	if namespace == "" {
		namespace = "local-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	usedNames := make(map[string]struct{}, count)
	for index := 1; index <= count; index++ {
		userID, sessionID := ai.IDs(namespace, index)
		player := entity.NewPlayer(userID, sessionID, config.Name(s.cosmeticRandom, usedNames), s.randomPlayerSpawn(), s.random)
		for _, character := range player.Characters {
			if character != nil {
				character.Skin = loadout.RandomSkin(s.cosmeticRandom, nil, character.Weapon.Type)
			}
		}
		if err := s.SpatialGrid.Insert(player); err != nil {
			panic("could not initialize AI player: " + err.Error())
		}
		s.Players[sessionID] = player
		s.AIControllers[sessionID] = ai.NewController(player)
	}
}

func (s *State) stepAI(tick int64) {
	if len(s.AIControllers) == 0 {
		return
	}
	if s.aiRandom == nil {
		s.aiRandom = rand.New(rand.NewSource(tick))
	}
	ids := make([]string, 0, len(s.AIControllers))
	for sessionID := range s.AIControllers {
		ids = append(ids, sessionID)
	}
	sort.Strings(ids)
	for _, sessionID := range ids {
		s.AIControllers[sessionID].Step(tick, s.SpatialGrid, s.aiRandom)
	}
}

// Human counts are independent of both bot entities and pending reservations.
func (s *State) humanPlayerCount() int {
	count := 0
	for sessionID := range s.Players {
		if s.AIControllers[sessionID] == nil {
			count++
		}
	}
	return count
}

func (s *State) humanObservers() map[string]*entity.Player {
	observers := make(map[string]*entity.Player, len(s.Presences))
	for sessionID := range s.Presences {
		if player := s.Players[sessionID]; player != nil && s.AIControllers[sessionID] == nil {
			observers[sessionID] = player
		}
	}
	return observers
}
