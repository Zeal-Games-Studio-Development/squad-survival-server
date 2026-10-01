package strategy

import (
	"math"
	"testing"
)

func TestDefaultStrategies(t *testing.T) {
	if GridSize != 5 || GridCenter != 2 || MaxCharacters != 9 {
		t.Fatalf("unexpected formation constants: size=%d center=%d max=%d", GridSize, GridCenter, MaxCharacters)
	}
	for _, strategyName := range []string{"x-type", "plus-type"} {
		definition, ok := DefaultDefinition(strategyName)
		if !ok {
			t.Fatalf("expected %q strategy", strategyName)
		}
		if definition.Capacity() != MaxCharacters {
			t.Fatalf("expected %q capacity %d, got %d", strategyName, MaxCharacters, definition.Capacity())
		}
		var expectedGrid [GridSize][GridSize]uint8
		for index := 0; index < GridSize; index++ {
			if strategyName == "x-type" {
				expectedGrid[index][index] = 1
				expectedGrid[index][GridSize-1-index] = 1
			} else {
				expectedGrid[index][GridCenter] = 1
				expectedGrid[GridCenter][index] = 1
			}
		}
		if definition.Grid != expectedGrid {
			t.Fatalf("unexpected %q grid: %v", strategyName, definition.Grid)
		}
		slots := definition.Slots()
		if slots[0].Row != GridCenter || slots[0].Column != GridCenter || slots[0].Offset.X != 0 || slots[0].Offset.Y != 0 {
			t.Fatalf("expected %q center slot first, got %+v", strategyName, slots[0])
		}
		maximumDistance := math.Sqrt(slots[len(slots)-1].DistanceSquared)
		wantMaximumDistance := 3.0
		if strategyName == "x-type" {
			wantMaximumDistance = math.Sqrt(18)
		}
		if math.Abs(maximumDistance-wantMaximumDistance) > 1e-9 {
			t.Fatalf("expected %q maximum offset %f, got %f", strategyName, wantMaximumDistance, maximumDistance)
		}
	}
}

func TestParseCatalogRejectsInvalidDefinitions(t *testing.T) {
	tests := []string{
		`{"strategies":[]}`,
		`{"strategies":[{"type":"","grid":[[1,1,1,1,1],[1,1,1,1,1],[1,1,1,1,1],[1,1,1,1,1],[1,1,1,1,1]]}]}`,
		`{"strategies":[{"type":"x","grid":[[1]]}]}`,
		`{"strategies":[{"type":"x","grid":[[2,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]]}]}`,
		`{"strategies":[{"type":"x","grid":[[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]]}]}`,
		`{"strategies":[{"type":"x","grid":[[1,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]]},{"type":"x","grid":[[1,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]]}]}`,
	}
	for _, data := range tests {
		if _, err := ParseCatalog([]byte(data)); err == nil {
			t.Fatalf("expected invalid catalog to fail: %s", data)
		}
	}
}
