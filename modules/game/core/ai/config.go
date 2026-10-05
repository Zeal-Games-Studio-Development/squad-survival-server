package ai

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"squad-survival-be/modules/game/core/strategy"
)

type Config struct {
	SurvivalCount                      int         `json:"survival_count"`
	BattleRoyaleCount                  int         `json:"battle_royale_count"`
	Prefixes                           []string    `json:"prefixes"`
	Suffixes                           []string    `json:"suffixes"`
	DisengageDifferenceByMaxCharacters map[int]int `json:"disengage_difference_by_max_characters"`
}

//go:embed config.json
var defaultConfigJSON []byte

func ParseConfig(data []byte) (Config, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if config.SurvivalCount < 0 || config.BattleRoyaleCount < 0 {
		return Config{}, errors.New("AI counts must not be negative")
	}
	for maximum, difference := range config.DisengageDifferenceByMaxCharacters {
		if maximum < 1 || maximum > strategy.MaxCharacters || difference < 1 || difference > maximum {
			return Config{}, fmt.Errorf("invalid disengage difference %d for max characters %d", difference, maximum)
		}
	}
	if config.DisengageDifferenceByMaxCharacters == nil {
		config.DisengageDifferenceByMaxCharacters = make(map[int]int)
	}
	for maximum := 1; maximum <= strategy.MaxCharacters; maximum++ {
		config.DisengageDifferenceByMaxCharacters[maximum] = config.disengageDifference(maximum)
	}
	if config.SurvivalCount > 0 || config.BattleRoyaleCount > 0 {
		if len(config.Prefixes) == 0 || len(config.Suffixes) == 0 {
			return Config{}, errors.New("AI names require prefixes and suffixes")
		}
		for _, name := range append(append([]string(nil), config.Prefixes...), config.Suffixes...) {
			if strings.TrimSpace(name) == "" {
				return Config{}, errors.New("AI name parts must not be blank")
			}
		}
	}
	return config, nil
}

func (c Config) disengageDifference(maximum int) int {
	if difference := c.DisengageDifferenceByMaxCharacters[maximum]; difference >= 1 && difference <= maximum {
		return difference
	}
	return max(1, maximum/3)
}

func DefaultConfig() Config {
	config, err := ParseConfig(defaultConfigJSON)
	if err != nil {
		panic("invalid embedded AI config: " + err.Error())
	}
	return config
}

// Name draws from both lists and adds a number if that combination was used already.
func (c Config) Name(random *rand.Rand, used map[string]struct{}) string {
	base := c.Prefixes[random.Intn(len(c.Prefixes))] + " " + c.Suffixes[random.Intn(len(c.Suffixes))]
	name := base
	for number := 2; ; number++ {
		if _, exists := used[name]; !exists {
			used[name] = struct{}{}
			return name
		}
		name = base + " " + strconv.Itoa(number)
	}
}

func IDs(matchID string, index int) (string, string) {
	identifier := fmt.Sprintf("%s:%d", matchID, index)
	return "ai-user:" + identifier, "ai-session:" + identifier
}
