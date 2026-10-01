package progression

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"squad-survival-be/modules/game/core/strategy"
)

const (
	MinLevel            = 1
	MaxLevel            = 10
)

type Level struct {
	Level                 int    `json:"level"`
	ExperienceToNextLevel uint64 `json:"experience_to_next_level"`
}

type Catalog struct {
	Levels []Level `json:"levels"`
}

//go:embed level_progression.json
var defaultCatalogJSON []byte

var embeddedCatalog = mustParseDefaultCatalog()

func ParseCatalog(data []byte) (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, err
	}
	if len(catalog.Levels) != MaxLevel-MinLevel+1 {
		return Catalog{}, fmt.Errorf("progression catalog must contain levels %d through %d", MinLevel, MaxLevel)
	}
	seen := make(map[int]struct{}, len(catalog.Levels))
	for index, definition := range catalog.Levels {
		expectedLevel := MinLevel + index
		if definition.Level != expectedLevel {
			return Catalog{}, fmt.Errorf("progression level at index %d must be %d", index, expectedLevel)
		}
		if _, exists := seen[definition.Level]; exists {
			return Catalog{}, fmt.Errorf("duplicate progression level %d", definition.Level)
		}
		seen[definition.Level] = struct{}{}
		if definition.Level < MaxLevel && definition.ExperienceToNextLevel == 0 {
			return Catalog{}, fmt.Errorf("experience to next level must be positive at level %d", definition.Level)
		}
		if definition.Level == MaxLevel && definition.ExperienceToNextLevel != 0 {
			return Catalog{}, errors.New("experience to next level must be zero at max level")
		}
	}
	if MaxCharactersForLevel(MaxLevel) > strategy.MaxCharacters {
		return Catalog{}, errors.New("max progression level exceeds formation character boundary")
	}
	return catalog, nil
}

func DefaultCatalog() Catalog {
	levels := make([]Level, len(embeddedCatalog.Levels))
	copy(levels, embeddedCatalog.Levels)
	return Catalog{Levels: levels}
}

func mustParseDefaultCatalog() Catalog {
	catalog, err := ParseCatalog(defaultCatalogJSON)
	if err != nil {
		panic("invalid embedded level progression catalog: " + err.Error())
	}
	return catalog
}

func ClampLevel(level int) int {
	if level < MinLevel {
		return MinLevel
	}
	if level > MaxLevel {
		return MaxLevel
	}
	return level
}

func MaxCharactersForLevel(level int) int {
	maximum := ClampLevel(level)
	if maximum > strategy.MaxCharacters {
		return strategy.MaxCharacters
	}
	return maximum
}
