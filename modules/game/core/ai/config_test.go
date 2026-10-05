package ai

import (
	"math/rand"
	"strings"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	for _, data := range []string{
		`{`,
		`{"survival_count":-1}`,
		`{"battle_royale_count":-1}`,
		`{"survival_count":1,"prefixes":[],"suffixes":["Wolf"]}`,
		`{"battle_royale_count":1,"prefixes":["Iron"],"suffixes":[]}`,
		`{"survival_count":1,"prefixes":[" "],"suffixes":["Wolf"]}`,
		`{"disengage_difference_by_max_characters":{"0":1}}`,
		`{"disengage_difference_by_max_characters":{"10":1}}`,
		`{"disengage_difference_by_max_characters":{"3":0}}`,
		`{"disengage_difference_by_max_characters":{"3":-1}}`,
		`{"disengage_difference_by_max_characters":{"3":4}}`,
		`{"disengage_difference_by_max_characters":{"3":1.5}}`,
	} {
		if _, err := ParseConfig([]byte(data)); err == nil {
			t.Fatalf("accepted invalid config: %s", data)
		}
	}
	if _, err := ParseConfig([]byte(`{"survival_count":0,"battle_royale_count":0}`)); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	if config.SurvivalCount != 8 || config.BattleRoyaleCount != 16 {
		t.Fatalf("unexpected defaults: %+v", config)
	}
}

func TestDisengageDifferenceDefaultsAndOverrides(t *testing.T) {
	config, err := ParseConfig([]byte(`{"disengage_difference_by_max_characters":{"3":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	defaults := DefaultConfig()
	for maximum, want := range []int{1, 1, 1, 1, 1, 2, 2, 2, 3} {
		maximum++
		if got := defaults.DisengageDifferenceByMaxCharacters[maximum]; got != want {
			t.Fatalf("max %d: expected default %d, got %d", maximum, want, got)
		}
		if maximum == 3 {
			want = 2
		}
		if got := config.DisengageDifferenceByMaxCharacters[maximum]; got != want {
			t.Fatalf("max %d: expected merged value %d, got %d", maximum, want, got)
		}
	}
}

func TestNamesRemainUniqueAfterCombinationsExhausted(t *testing.T) {
	config := Config{Prefixes: []string{"Iron"}, Suffixes: []string{"Wolf"}}
	used := map[string]struct{}{}
	random := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		name := config.Name(random, used)
		if !strings.HasPrefix(name, "Iron Wolf") || len(used) != i+1 {
			t.Fatalf("duplicate or malformed name: %q", name)
		}
	}
}

func TestIDsSeparateMatchesAndEntityKinds(t *testing.T) {
	seen := map[string]bool{}
	for _, match := range []string{"one", "two"} {
		for index := 1; index <= 16; index++ {
			user, session := IDs(match, index)
			for _, id := range []string{user, session} {
				if seen[id] {
					t.Fatalf("duplicate identity: %s", id)
				}
				seen[id] = true
			}
		}
	}
}
