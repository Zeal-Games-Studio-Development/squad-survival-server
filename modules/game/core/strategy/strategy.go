package strategy

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"squad-survival-be/modules/game/core/world"
)

const (
	GridSize      = 3
	GridCenter    = GridSize / 2
	MaxCharacters = 5
	SlotSpacing   = 1.5

	DefaultStrategyName = "x-type"
)

type Definition struct {
	Type string
	Grid [GridSize][GridSize]uint8
}

type Slot struct {
	Row             int
	Column          int
	Offset          world.Vector2
	DistanceSquared float64
}

type rawCatalog struct {
	Strategies []rawDefinition `json:"strategies"`
}

type rawDefinition struct {
	Type string  `json:"type"`
	Grid [][]int `json:"grid"`
}

//go:embed strategies.json
var defaultStrategyJSON []byte

func ParseCatalog(data []byte) (map[string]Definition, error) {
	var raw rawCatalog
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if len(raw.Strategies) == 0 {
		return nil, errors.New("strategy catalog has no strategies")
	}

	catalog := make(map[string]Definition, len(raw.Strategies))
	for _, item := range raw.Strategies {
		if item.Type == "" {
			return nil, errors.New("strategy type is required")
		}
		if _, exists := catalog[item.Type]; exists {
			return nil, fmt.Errorf("duplicate strategy type %q", item.Type)
		}
		if len(item.Grid) != GridSize {
			return nil, fmt.Errorf("strategy %q must have %d rows", item.Type, GridSize)
		}

		definition := Definition{Type: item.Type}
		capacity := 0
		for row := 0; row < GridSize; row++ {
			if len(item.Grid[row]) != GridSize {
				return nil, fmt.Errorf("strategy %q row %d must have %d columns", item.Type, row, GridSize)
			}
			for column, value := range item.Grid[row] {
				if value != 0 && value != 1 {
					return nil, fmt.Errorf("strategy %q cell [%d][%d] must be 0 or 1", item.Type, row, column)
				}
				definition.Grid[row][column] = uint8(value)
				capacity += value
			}
		}
		if capacity == 0 {
			return nil, fmt.Errorf("strategy %q has no open slots", item.Type)
		}
		catalog[item.Type] = definition
	}
	return catalog, nil
}

func DefaultDefinition(strategyType string) (Definition, bool) {
	catalog, err := ParseCatalog(defaultStrategyJSON)
	if err != nil {
		panic("invalid embedded strategy catalog: " + err.Error())
	}
	definition, ok := catalog[strategyType]
	return definition, ok
}

func (d Definition) Capacity() int {
	capacity := 0
	for row := range GridSize {
		for column := range GridSize {
			capacity += int(d.Grid[row][column])
		}
	}
	return capacity
}

func (d Definition) Slots() []Slot {
	slots := make([]Slot, 0, d.Capacity())
	for row := range GridSize {
		for column := range GridSize {
			if d.Grid[row][column] == 0 {
				continue
			}
			x := float64(column-GridCenter) * SlotSpacing
			y := float64(GridCenter-row) * SlotSpacing
			slots = append(slots, Slot{
				Row:             row,
				Column:          column,
				Offset:          world.Vector2{X: x, Y: y},
				DistanceSquared: x*x + y*y,
			})
		}
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].DistanceSquared != slots[j].DistanceSquared {
			return slots[i].DistanceSquared < slots[j].DistanceSquared
		}
		if slots[i].Row != slots[j].Row {
			return slots[i].Row < slots[j].Row
		}
		return slots[i].Column < slots[j].Column
	})
	return slots
}
