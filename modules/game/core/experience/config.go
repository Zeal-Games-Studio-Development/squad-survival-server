package experience

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/strategy"
)

type PackageDefinition struct {
	Tier        string `json:"tier"`
	MinValue    uint64 `json:"min_value"`
	MaxValue    uint64 `json:"max_value"`
	TargetCount int    `json:"target_count"`
}

type Config struct {
	Packages                       []PackageDefinition `json:"packages"`
	KillExperienceByCharacterCount map[int]uint64      `json:"kill_experience_by_character_count"`
	RefillIntervalSeconds          int                 `json:"refill_interval_seconds"`
	PickupRadius                   float64             `json:"pickup_radius"`
	SpawnSeparation                float64             `json:"spawn_separation"`
	SpawnAttempts                  int                 `json:"spawn_attempts"`
}

//go:embed config.json
var defaultConfigJSON []byte

var embeddedConfig = mustParseDefaultConfig()

func ParseConfig(data []byte) (Config, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	want := map[string]bool{"small": false, "medium": false, "large": false}
	if len(config.Packages) != len(want) {
		return Config{}, errors.New("experience config must define small, medium, and large exactly once")
	}
	for _, definition := range config.Packages {
		seen, ok := want[definition.Tier]
		if !ok || seen {
			return Config{}, fmt.Errorf("invalid or duplicate experience package tier %q", definition.Tier)
		}
		want[definition.Tier] = true
		if definition.MinValue == 0 || definition.MaxValue < definition.MinValue || definition.TargetCount < 0 {
			return Config{}, fmt.Errorf("invalid experience package definition for tier %q", definition.Tier)
		}
	}
	if len(config.KillExperienceByCharacterCount) != strategy.MaxCharacters {
		return Config{}, fmt.Errorf("kill experience must define character counts 1 through %d", strategy.MaxCharacters)
	}
	for count := 1; count <= strategy.MaxCharacters; count++ {
		if config.KillExperienceByCharacterCount[count] == 0 {
			return Config{}, fmt.Errorf("kill experience for character count %d must be positive", count)
		}
	}
	if config.RefillIntervalSeconds <= 0 || config.PickupRadius <= 0 || config.SpawnSeparation <= 0 || config.SpawnAttempts <= 0 {
		return Config{}, errors.New("experience timing, radius, separation, and attempts must be positive")
	}
	return config, nil
}

func DefaultConfig() Config {
	result := embeddedConfig
	result.Packages = append([]PackageDefinition(nil), embeddedConfig.Packages...)
	result.KillExperienceByCharacterCount = make(map[int]uint64, len(embeddedConfig.KillExperienceByCharacterCount))
	for count, value := range embeddedConfig.KillExperienceByCharacterCount {
		result.KillExperienceByCharacterCount[count] = value
	}
	return result
}

func Tier(value string) (entity.ExperiencePackageTier, bool) {
	switch value {
	case "small":
		return entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_SMALL, true
	case "medium":
		return entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_MEDIUM, true
	case "large":
		return entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_LARGE, true
	default:
		return entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_UNSPECIFIED, false
	}
}

func mustParseDefaultConfig() Config {
	config, err := ParseConfig(defaultConfigJSON)
	if err != nil {
		panic("invalid embedded experience config: " + err.Error())
	}
	return config
}
