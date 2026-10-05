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
