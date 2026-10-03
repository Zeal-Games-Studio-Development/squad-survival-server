package experience

import "testing"

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	wantCounts := map[string]int{"small": 900, "medium": 450, "large": 150}
	wantRanges := map[string][2]uint64{"small": {10, 12}, "medium": {15, 18}, "large": {20, 22}}
	for _, definition := range config.Packages {
		if definition.TargetCount != wantCounts[definition.Tier] || [2]uint64{definition.MinValue, definition.MaxValue} != wantRanges[definition.Tier] {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	for count := 1; count <= 9; count++ {
		if got := config.KillExperienceByCharacterCount[count]; got != uint64(count*5) {
			t.Fatalf("count %d: got %d", count, got)
		}
	}
	if config.RefillIntervalSeconds != 60 || config.PickupRadius != 4 || config.SpawnSeparation != 4 || config.SpawnAttempts != 100 {
		t.Fatalf("unexpected world config: %+v", config)
	}
}

func TestParseConfigRejectsInvalidDefinitions(t *testing.T) {
	cases := []string{
		`{}`,
		`{"packages":[{"tier":"small","min_value":12,"max_value":10,"target_count":1},{"tier":"medium","min_value":1,"max_value":1,"target_count":1},{"tier":"large","min_value":1,"max_value":1,"target_count":1}],"kill_experience_by_character_count":{"1":1,"2":1,"3":1,"4":1,"5":1,"6":1,"7":1,"8":1,"9":1},"refill_interval_seconds":1,"pickup_radius":1,"spawn_separation":1,"spawn_attempts":1}`,
		`{"packages":[{"tier":"small","min_value":1,"max_value":1,"target_count":1},{"tier":"medium","min_value":1,"max_value":1,"target_count":1},{"tier":"large","min_value":1,"max_value":1,"target_count":1}],"kill_experience_by_character_count":{"1":1},"refill_interval_seconds":1,"pickup_radius":1,"spawn_separation":1,"spawn_attempts":1}`,
	}
	for _, data := range cases {
		if _, err := ParseConfig([]byte(data)); err == nil {
			t.Fatalf("expected invalid config: %s", data)
		}
	}
}
