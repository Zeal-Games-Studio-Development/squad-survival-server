package strategy

import "testing"

func TestDefaultCompactStrategy(t *testing.T) {
	if MaxCharacters != 5 {
		t.Fatalf("expected maximum of 5 characters, got %d", MaxCharacters)
	}
	definition, ok := DefaultDefinition(Compact)
	if !ok {
		t.Fatal("expected compact strategy")
	}
	if definition.Capacity() < MaxCharacters {
		t.Fatalf("expected capacity of at least %d, got %d", MaxCharacters, definition.Capacity())
	}
	slots := definition.Slots()
	if slots[0].Row != GridCenter || slots[0].Column != GridCenter || slots[0].Offset.X != 0 || slots[0].Offset.Y != 0 {
		t.Fatalf("expected center slot first, got %+v", slots[0])
	}
	if slots[1].Row != GridCenter-1 || slots[1].Column != GridCenter {
		t.Fatalf("expected row/column tie-break, got %+v", slots[1])
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
