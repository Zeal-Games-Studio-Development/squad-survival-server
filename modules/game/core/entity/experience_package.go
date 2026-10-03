package entity

const ExperiencePackageCollisionRadius = 1.0

// ExperiencePackage is a static, instantly collectible world entity.
type ExperiencePackage struct {
	ID       string
	Position Vector2
	Value    *ExperiencePackageValue
}

func NewExperiencePackage(id string, position Vector2, tier ExperiencePackageTier, value uint64) *ExperiencePackage {
	return &ExperiencePackage{ID: id, Position: position, Value: &ExperiencePackageValue{Tier: tier, Experience: value}}
}
