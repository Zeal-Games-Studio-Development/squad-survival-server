package ai

import (
	"math"
	"math/rand"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/game/core/progression"
	"squad-survival-be/modules/game/core/spatial"
	"squad-survival-be/modules/game/core/world"
)

const searchRadius = 120.0
const wanderTicks = 30.0

type Controller struct {
	Player          *entity.Player
	TargetID        string
	TargetKind      string
	wanderUntil     int64
	wanderDirection entity.Vector2
}

func NewController(player *entity.Player) *Controller {
	return &Controller{Player: player}
}

// Step chooses a movement target. The regular player movement and combat systems
// apply the decision afterward, exactly as they do for a human player.
func (c *Controller) Step(tick int64, grid *spatial.Grid, random *rand.Rand) {
	if c == nil || c.Player == nil || c.Player.IsEliminated() || grid == nil || random == nil {
		if c != nil && c.Player != nil {
			c.Player.Direction = entity.Vector2{}
			c.TargetID, c.TargetKind = "", ""
		}
		return
	}
	p := c.Player
	if p.CharacterCount() < p.MaxCharacters() {
		if boxes := grid.QueryCharacterBoxes(p.Position, searchRadius); len(boxes) > 0 {
			c.follow("box", boxes[0].ID, boxes[0].Position)
			return
		}
	}
	if p.Level < progression.MaxLevel {
		if packages := grid.QueryExperiencePackages(p.Position, searchRadius); len(packages) > 0 {
			c.follow("experience", packages[0].ID, packages[0].Position)
			return
		}
	}
	for _, candidate := range grid.QueryPlayersWithinRadius(p, searchRadius) {
		if !candidate.IsEliminated() {
			c.follow("player", candidate.SessionID, candidate.Position)
			return
		}
	}
	c.TargetID, c.TargetKind = "", "wander"
	if tick >= c.wanderUntil || c.wanderDirection == (entity.Vector2{}) {
		angle := random.Float64() * 2 * math.Pi
		c.wanderDirection = entity.Vector2{X: math.Cos(angle), Y: math.Sin(angle)}
		c.wanderUntil = tick + wanderTicks
	}
	// Turn inward if the next wander step would leave the playable area.
	if p.Position.X*c.wanderDirection.X+p.Position.Y*c.wanderDirection.Y > 0 &&
		math.Hypot(p.Position.X, p.Position.Y) >= world.PlayAreaRadius-10 {
		c.wanderDirection = entity.NormalizeDirection(entity.Vector2{X: -p.Position.X, Y: -p.Position.Y})
		c.wanderUntil = tick + wanderTicks
	}
	c.setDirection(c.wanderDirection)
}

func (c *Controller) follow(kind, id string, position entity.Vector2) {
	c.TargetKind, c.TargetID = kind, id
	p := c.Player.Position
	direction := entity.Vector2{X: position.X - p.X, Y: position.Y - p.Y}
	length := math.Hypot(direction.X, direction.Y)
	if kind == "box" && length <= entity.CharacterBoxCollisionRadius {
		c.setDirection(entity.Vector2{})
		return
	}
	// Scale the last step to arrive without oscillating around the target.
	stepDistance := c.Player.MinMoveSpeed() / float64(entity.TickRate)
	if divisor := math.Max(length, stepDistance); divisor > 0 {
		direction.X /= divisor
		direction.Y /= divisor
	}
	c.setDirection(direction)
}

func (c *Controller) setDirection(direction entity.Vector2) {
	c.Player.Direction = direction
	if direction != (entity.Vector2{}) {
		c.Player.Facing = direction
	}
}
