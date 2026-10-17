package characterbox

import (
	"math/rand"
	"sort"

	"squad-survival-be/modules/game/core/entity"
)

// SpawnRarities apportions new boxes using 7:2:1 weights among non-empty pools.
// Largest remainders receive leftover boxes; ties favor common, uncommon, rare.
// Shuffling avoids favoring a rarity if placement runs out of space partway through.
func SpawnRarities(random *rand.Rand, count int, pools map[entity.Rarity][]entity.WeaponType) []entity.Rarity {
	if count <= 0 || random == nil {
		return nil
	}
	allocations := []struct {
		rarity    entity.Rarity
		weight    int
		count     int
		remainder int
	}{
		{rarity: entity.RarityCommon, weight: 7},
		{rarity: entity.RarityUncommon, weight: 2},
		{rarity: entity.RarityRare, weight: 1},
	}
	totalWeight := 0
	for _, allocation := range allocations {
		if len(pools[allocation.rarity]) > 0 {
			totalWeight += allocation.weight
		}
	}
	if totalWeight == 0 {
		return nil
	}
	assigned := 0
	for index := range allocations {
		allocation := &allocations[index]
		if len(pools[allocation.rarity]) == 0 {
			allocation.remainder = -1
			continue
		}
		allocation.count = count * allocation.weight / totalWeight
		allocation.remainder = count * allocation.weight % totalWeight
		assigned += allocation.count
	}
	sort.SliceStable(allocations, func(i, j int) bool { return allocations[i].remainder > allocations[j].remainder })
	for index := 0; index < count-assigned; index++ {
		allocations[index].count++
	}
	rarities := make([]entity.Rarity, 0, count)
	for _, allocation := range allocations {
		for n := 0; n < allocation.count; n++ {
			rarities = append(rarities, allocation.rarity)
		}
	}
	random.Shuffle(len(rarities), func(i, j int) { rarities[i], rarities[j] = rarities[j], rarities[i] })
	return rarities
}
