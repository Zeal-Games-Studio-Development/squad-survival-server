package progression

import (
	"fmt"
	"testing"
)

func TestDefaultCatalog(t *testing.T) {
	catalog := DefaultCatalog()
	if len(catalog.Levels) != 10 {
		t.Fatalf("expected 10 levels, got %d", len(catalog.Levels))
	}
	for index, definition := range catalog.Levels {
		expectedLevel := index + 1
		expectedExperience := uint64(expectedLevel * 100)
		if expectedLevel == MaxLevel {
			expectedExperience = 0
		}
		if definition.Level != expectedLevel || definition.ExperienceToNextLevel != expectedExperience {
			t.Fatalf("unexpected level definition at index %d: %+v", index, definition)
		}
	}
}

func TestParseCatalogRejectsInvalidDefinitions(t *testing.T) {
	tests := []string{
		`not json`,
		`{"levels":[]}`,
		validJSONWith(3, 2, 300),
		validJSONWith(5, 5, 0),
		validJSONWith(10, 10, 1000),
	}
	for _, data := range tests {
		if _, err := ParseCatalog([]byte(data)); err == nil {
			t.Fatalf("expected invalid catalog to fail: %s", data)
		}
	}
}

func TestMaxCharactersForLevelClampsLevel(t *testing.T) {
	tests := map[int]int{-1: 1, 1: 1, 2: 2, 8: 8, 9: 9, 10: 9, 99: 9}
	for level, expected := range tests {
		if actual := MaxCharactersForLevel(level); actual != expected {
			t.Fatalf("level %d: expected %d characters, got %d", level, expected, actual)
		}
	}
}

func validJSONWith(index, level int, experience uint64) string {
	result := `{"levels":[`
	for current := 1; current <= 10; current++ {
		if current > 1 {
			result += `,`
		}
		currentLevel := current
		currentExperience := uint64(current * 100)
		if current == 10 {
			currentExperience = 0
		}
		if current == index {
			currentLevel = level
			currentExperience = experience
		}
		result += fmt.Sprintf(`{"level":%d,"experience_to_next_level":%d}`, currentLevel, currentExperience)
	}
	return result + `]}`
}
