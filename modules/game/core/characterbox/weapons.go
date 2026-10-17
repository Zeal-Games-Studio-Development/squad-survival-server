package characterbox

import (
	"math/rand"

	"squad-survival-be/modules/game/core/entity"
)

// Owned resolves a storage snapshot in catalog order, independent of bootstrap config.
func Owned(catalog []entity.Weapon, ids []string) []entity.Weapon {
	owned := make(map[string]bool, len(ids))
	for _, id := range ids {
		owned[id] = true
	}
	var weapons []entity.Weapon
	for _, weapon := range catalog {
		if owned[weapon.ID] {
			weapons = append(weapons, weapon)
		}
	}
	return weapons
}

func Filter(weapons []entity.Weapon, weaponType entity.WeaponType, rarity entity.Rarity) []entity.Weapon {
	var matches []entity.Weapon
	for _, weapon := range weapons {
		if (weaponType == "" || weapon.Type == weaponType) && weapon.Rarity == rarity {
			matches = append(matches, weapon)
		}
	}
	return matches
}

func Choose(random *rand.Rand, weapons []entity.Weapon) (entity.Weapon, bool) {
	if random == nil || len(weapons) == 0 {
		return entity.Weapon{}, false
	}
	return weapons[random.Intn(len(weapons))], true
}

// SpawnTypes preserves catalog order so map iteration cannot affect gameplay RNG.
func SpawnTypes(catalog []entity.Weapon, inventories [][]entity.Weapon) []entity.WeaponType {
	eligible := make(map[entity.WeaponType]bool)
	for _, weapons := range inventories {
		for _, weapon := range Filter(weapons, "", entity.RarityCommon) {
			eligible[weapon.Type] = true
		}
	}
	var types []entity.WeaponType
	for _, weapon := range catalog {
		if weapon.Rarity == entity.RarityCommon && eligible[weapon.Type] {
			types = append(types, weapon.Type)
			delete(eligible, weapon.Type)
		}
	}
	return types
}
