package world

import (
	"math"
	"math/rand"
)

const (
	PlayAreaRadius = 500.0
	SpawnRadius    = 450.0
)

type Vector2 struct {
	X float64
	Y float64
}

func ClampToPlayArea(position Vector2) Vector2 {
	distanceSquared := position.X*position.X + position.Y*position.Y
	radiusSquared := PlayAreaRadius * PlayAreaRadius
	if distanceSquared <= radiusSquared {
		return position
	}

	distance := math.Sqrt(distanceSquared)
	return Vector2{
		X: position.X / distance * PlayAreaRadius,
		Y: position.Y / distance * PlayAreaRadius,
	}
}

func RandomSpawn(random *rand.Rand) Vector2 {
	angle := random.Float64() * 2 * math.Pi
	radius := math.Sqrt(random.Float64()) * SpawnRadius
	return Vector2{
		X: math.Cos(angle) * radius,
		Y: math.Sin(angle) * radius,
	}
}
