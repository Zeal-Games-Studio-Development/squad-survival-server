package strategy

import "testing"

func TestDefaultStrategies(t *testing.T) {
	if MaxCharacters != 5 {
		t.Fatalf("expected maximum of 5 characters, got %d", MaxCharacters)
	}
	for _, strategyName := range []string{"x-type", "plus-type"} {
		definition, ok := DefaultDefinition(strategyName)
		if !ok {
			t.Fatalf("expected %q strategy", strategyName)
		}
		if definition.Capacity() != MaxCharacters {
			t.Fatalf("expected %q capacity %d, got %d", strategyName, MaxCharacters, definition.Capacity())
		}
		slots := definition.Slots()
		if slots[0].Row != GridCenter || slots[0].Column != GridCenter || slots[0].Offset.X != 0 || slots[0].Offset.Y != 0 {
			t.Fatalf("expected %q center slot first, got %+v", strategyName, slots[0])
		}
	}
}

func TestParseCatalogRejectsInvalidDefinitions(t *testing.T) {
	tests := []string{
		`{"strategies":[]}`,
		`{"strategies":[{"type":"","grid":[[1,1,1],[1,1,1],[1,1,1]]}]}`,
		`{"strategies":[{"type":"x","grid":[[1]]}]}`,
		`{"strategies":[{"type":"x","grid":[[2,0,0],[0,0,0],[0,0,0]]}]}`,
		`{"strategies":[{"type":"x","grid":[[0,0,0],[0,0,0],[0,0,0]]}]}`,
		`{"strategies":[{"type":"x","grid":[[1,0,0],[0,0,0],[0,0,0]]},{"type":"x","grid":[[1,0,0],[0,0,0],[0,0,0]]}]}`,
	}
	for _, data := range tests {
		if _, err := ParseCatalog([]byte(data)); err == nil {
			t.Fatalf("expected invalid catalog to fail: %s", data)
		}
	}
}
