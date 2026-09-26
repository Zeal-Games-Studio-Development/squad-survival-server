package spatial

import (
	"errors"
	"math"
	"sort"

	"squad-survival-be/modules/game/core/entity"
)

var (
	ErrNilPlayer       = errors.New("player is nil")
	ErrEmptySessionID  = errors.New("player session ID is empty")
	ErrPlayerExists    = errors.New("player already exists in spatial grid")
	ErrPlayerNotFound  = errors.New("player does not exist in spatial grid")
	ErrNilCharacterBox = errors.New("character box is nil")
	ErrEmptyBoxID      = errors.New("character box ID is empty")
	ErrBoxExists       = errors.New("character box already exists in spatial grid")
)

type cell struct {
	X int
	Y int
}

type Grid struct {
	cellSize     float64
	cells        map[cell]map[string]*entity.Player
	locations    map[string]cell
	boxCells     map[cell]map[string]*entity.CharacterBox
	boxLocations map[string]cell
}

func NewGrid(cellSize float64) *Grid {
	if cellSize <= 0 || math.IsNaN(cellSize) || math.IsInf(cellSize, 0) {
		panic("spatial grid cell size must be finite and greater than zero")
	}
	return &Grid{
		cellSize:     cellSize,
		cells:        make(map[cell]map[string]*entity.Player),
		locations:    make(map[string]cell),
		boxCells:     make(map[cell]map[string]*entity.CharacterBox),
		boxLocations: make(map[string]cell),
	}
}

func (g *Grid) InsertCharacterBox(box *entity.CharacterBox) error {
	if box == nil {
		return ErrNilCharacterBox
	}
	if box.ID == "" {
		return ErrEmptyBoxID
	}
	if _, exists := g.boxLocations[box.ID]; exists {
		return ErrBoxExists
	}
	location := g.cellAt(box.Position)
	if g.boxCells[location] == nil {
		g.boxCells[location] = make(map[string]*entity.CharacterBox)
	}
	g.boxCells[location][box.ID] = box
	g.boxLocations[box.ID] = location
	return nil
}

func (g *Grid) RemoveCharacterBox(boxID string) bool {
	location, exists := g.boxLocations[boxID]
	if !exists {
		return false
	}
	delete(g.boxLocations, boxID)
	delete(g.boxCells[location], boxID)
	if len(g.boxCells[location]) == 0 {
		delete(g.boxCells, location)
	}
	return true
}

// QueryCharacterBoxes returns boxes within radius ordered by distance then ID.
func (g *Grid) QueryCharacterBoxes(position entity.Vector2, radius float64) []*entity.CharacterBox {
	if radius < 0 || math.IsNaN(radius) || math.IsInf(radius, 0) {
		return nil
	}
	radiusSquared := radius * radius
	minCell := g.cellAt(entity.Vector2{X: position.X - radius, Y: position.Y - radius})
	maxCell := g.cellAt(entity.Vector2{X: position.X + radius, Y: position.Y + radius})
	type result struct {
		box             *entity.CharacterBox
		distanceSquared float64
	}
	results := make([]result, 0)
	for x := minCell.X; x <= maxCell.X; x++ {
		for y := minCell.Y; y <= maxCell.Y; y++ {
			for _, box := range g.boxCells[cell{X: x, Y: y}] {
				deltaX := box.Position.X - position.X
				deltaY := box.Position.Y - position.Y
				distanceSquared := deltaX*deltaX + deltaY*deltaY
				if distanceSquared <= radiusSquared {
					results = append(results, result{box: box, distanceSquared: distanceSquared})
				}
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].distanceSquared == results[j].distanceSquared {
			return results[i].box.ID < results[j].box.ID
		}
		return results[i].distanceSquared < results[j].distanceSquared
	})
	boxes := make([]*entity.CharacterBox, len(results))
	for index, result := range results {
		boxes[index] = result.box
	}
	return boxes
}

func (g *Grid) Insert(player *entity.Player) error {
	if err := validatePlayer(player); err != nil {
		return err
	}
	if _, exists := g.locations[player.SessionID]; exists {
		return ErrPlayerExists
	}

	location := g.cellAt(player.Position)
	if g.cells[location] == nil {
		g.cells[location] = make(map[string]*entity.Player)
	}
	g.cells[location][player.SessionID] = player
	g.locations[player.SessionID] = location
	return nil
}

func (g *Grid) Move(player *entity.Player) error {
	if err := validatePlayer(player); err != nil {
		return err
	}
	previous, exists := g.locations[player.SessionID]
	if !exists {
		return ErrPlayerNotFound
	}

	next := g.cellAt(player.Position)
	if previous == next {
		g.cells[previous][player.SessionID] = player
		return nil
	}

	delete(g.cells[previous], player.SessionID)
	if len(g.cells[previous]) == 0 {
		delete(g.cells, previous)
	}
	if g.cells[next] == nil {
		g.cells[next] = make(map[string]*entity.Player)
	}
	g.cells[next][player.SessionID] = player
	g.locations[player.SessionID] = next
	return nil
}

func (g *Grid) Remove(sessionID string) bool {
	location, exists := g.locations[sessionID]
	if !exists {
		return false
	}

	delete(g.locations, sessionID)
	delete(g.cells[location], sessionID)
	if len(g.cells[location]) == 0 {
		delete(g.cells, location)
	}
	return true
}

func (g *Grid) QueryPlayers(player *entity.Player) []*entity.Player {
	if player == nil || player.DetectionRadius < 0 {
		return nil
	}

	radius := player.DetectionRadius
	radiusSquared := radius * radius
	minCell := g.cellAt(entity.Vector2{X: player.Position.X - radius, Y: player.Position.Y - radius})
	maxCell := g.cellAt(entity.Vector2{X: player.Position.X + radius, Y: player.Position.Y + radius})
	type result struct {
		player          *entity.Player
		distanceSquared float64
	}
	results := make([]result, 0)

	for x := minCell.X; x <= maxCell.X; x++ {
		for y := minCell.Y; y <= maxCell.Y; y++ {
			for sessionID, candidate := range g.cells[cell{X: x, Y: y}] {
				if sessionID == player.SessionID {
					continue
				}
				deltaX := candidate.Position.X - player.Position.X
				deltaY := candidate.Position.Y - player.Position.Y
				distanceSquared := deltaX*deltaX + deltaY*deltaY
				if distanceSquared <= radiusSquared {
					results = append(results, result{player: candidate, distanceSquared: distanceSquared})
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].distanceSquared == results[j].distanceSquared {
			return results[i].player.SessionID < results[j].player.SessionID
		}
		return results[i].distanceSquared < results[j].distanceSquared
	})
	players := make([]*entity.Player, len(results))
	for index, result := range results {
		players[index] = result.player
	}
	return players
}

func (g *Grid) cellAt(position entity.Vector2) cell {
	return cell{
		X: int(math.Floor(position.X / g.cellSize)),
		Y: int(math.Floor(position.Y / g.cellSize)),
	}
}

func validatePlayer(player *entity.Player) error {
	if player == nil {
		return ErrNilPlayer
	}
	if player.SessionID == "" {
		return ErrEmptySessionID
	}
	return nil
}
