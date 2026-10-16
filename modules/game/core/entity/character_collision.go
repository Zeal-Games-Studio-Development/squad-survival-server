package entity

import (
	"math"

	"squad-survival-be/modules/game/core/world"
)

const collisionEpsilon = 1e-6

func characterRadius(character *Character) float64 {
	if character != nil && isFinite(character.Radius) && character.Radius > 0 {
		return character.Radius
	}
	return DefaultCharacterRadius
}

func freeCharacterPosition(position Vector2, character *Character, obstacles []*Character) bool {
	if !isFinite(position.X) || !isFinite(position.Y) || math.Hypot(position.X, position.Y) > world.PlayAreaRadius+collisionEpsilon {
		return false
	}
	for _, other := range obstacles {
		minimum := characterRadius(character) + characterRadius(other)
		if math.Hypot(position.X-other.Position.X, position.Y-other.Position.Y) < minimum-collisionEpsilon {
			return false
		}
	}
	return true
}

// nearestFreeCharacterPosition searches concentric rings in a stable order.
func nearestFreeCharacterPosition(target Vector2, character *Character, obstacles []*Character) (Vector2, bool) {
	target = world.ClampToPlayArea(target)
	if freeCharacterPosition(target, character, obstacles) {
		return target, true
	}
	for ring := 1; ring <= 160; ring++ {
		distance := float64(ring) * 0.1
		for angle := 0; angle < 64; angle++ {
			radians := 2 * math.Pi * float64(angle) / 64
			candidate := Vector2{X: target.X + distance*math.Cos(radians), Y: target.Y + distance*math.Sin(radians)}
			if freeCharacterPosition(candidate, character, obstacles) {
				return candidate, true
			}
		}
	}
	return Vector2{}, false
}

// safeSegment returns the first safe point on a segment and its blocking unit.
func safeSegment(start, end Vector2, character *Character, obstacles []*Character) (Vector2, *Character) {
	dx, dy := end.X-start.X, end.Y-start.Y
	first := 1.0
	var blocker *Character
	for _, other := range obstacles {
		radius := characterRadius(character) + characterRadius(other)
		ox, oy := start.X-other.Position.X, start.Y-other.Position.Y
		c := ox*ox + oy*oy - radius*radius
		if c < -collisionEpsilon {
			return start, other
		}
		a := dx*dx + dy*dy
		if a == 0 {
			continue
		}
		b := 2 * (ox*dx + oy*dy)
		if c <= collisionEpsilon && b >= 0 {
			continue
		}
		discriminant := b*b - 4*a*c
		if discriminant < 0 {
			continue
		}
		t := (-b - math.Sqrt(discriminant)) / (2 * a)
		if t >= 0 && t < first {
			first, blocker = t, other
		}
	}
	if blocker == nil {
		return end, nil
	}
	first = math.Max(0, first-collisionEpsilon)
	return Vector2{X: start.X + dx*first, Y: start.Y + dy*first}, blocker
}

func moveCharacterWithCollisions(character *Character, obstacles []*Character, maximumDistance float64) Vector2 {
	start := character.Position
	target := character.TargetPosition
	if math.Hypot(target.X-start.X, target.Y-start.Y) > CharacterSnapDistance {
		if destination, ok := nearestFreeCharacterPosition(target, character, obstacles); ok {
			return destination
		}
		return start
	}
	if !freeCharacterPosition(start, character, obstacles) {
		if destination, ok := nearestFreeCharacterPosition(start, character, obstacles); ok {
			start = destination
		} else {
			return start
		}
	}
	direct := world.ClampToPlayArea(moveTowards(start, target, maximumDistance))
	contact, blocker := safeSegment(start, direct, character, obstacles)
	if blocker == nil {
		return direct
	}
	remaining := maximumDistance - math.Hypot(contact.X-start.X, contact.Y-start.Y)
	if remaining <= collisionEpsilon {
		return contact
	}
	nx, ny := contact.X-blocker.Position.X, contact.Y-blocker.Position.Y
	length := math.Hypot(nx, ny)
	if length <= collisionEpsilon {
		return contact
	}
	tx, ty := -ny/length, nx/length
	best := contact
	bestDistance := math.Hypot(target.X-contact.X, target.Y-contact.Y)
	for _, sign := range []float64{1, -1} {
		end := world.ClampToPlayArea(Vector2{X: contact.X + sign*tx*remaining, Y: contact.Y + sign*ty*remaining})
		candidate, _ := safeSegment(contact, end, character, obstacles)
		if !freeCharacterPosition(candidate, character, obstacles) {
			continue
		}
		distance := math.Hypot(target.X-candidate.X, target.Y-candidate.Y)
		if distance < bestDistance-collisionEpsilon || best == contact && math.Hypot(candidate.X-contact.X, candidate.Y-contact.Y) > collisionEpsilon {
			best, bestDistance = candidate, distance
		}
	}
	return best
}
