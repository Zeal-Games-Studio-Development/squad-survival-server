package characterbox

import (
	"math/rand"
	"reflect"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func TestSpawnRarityAllocation(t *testing.T) {
	for _, test := range []struct {
		name      string
		count     int
		available [3]bool
		want      [3]int
	}{
		{"sixty", 60, [3]bool{true, true, true}, [3]int{42, 12, 6}},
		{"seventy", 70, [3]bool{true, true, true}, [3]int{49, 14, 7}},
		{"eighty", 80, [3]bool{true, true, true}, [3]int{56, 16, 8}},
		{"sixty-one", 61, [3]bool{true, true, true}, [3]int{43, 12, 6}},
		{"one", 1, [3]bool{true, true, true}, [3]int{1, 0, 0}},
		{"two", 2, [3]bool{true, true, true}, [3]int{2, 0, 0}},
		{"three", 3, [3]bool{true, true, true}, [3]int{2, 1, 0}},
		{"tie-common-rare", 5, [3]bool{true, true, true}, [3]int{4, 1, 0}},
		{"no-rare", 9, [3]bool{true, true, false}, [3]int{7, 2, 0}},
		{"no-common", 10, [3]bool{false, true, true}, [3]int{0, 7, 3}},
		{"no-uncommon-tie", 4, [3]bool{true, false, true}, [3]int{4, 0, 0}},
		{"only-common", 10, [3]bool{true, false, false}, [3]int{10, 0, 0}},
		{"only-uncommon", 10, [3]bool{false, true, false}, [3]int{0, 10, 0}},
		{"only-rare", 10, [3]bool{false, false, true}, [3]int{0, 0, 10}},
		{"empty", 10, [3]bool{}, [3]int{}},
		{"zero", 0, [3]bool{true, true, true}, [3]int{}},
		{"negative", -1, [3]bool{true, true, true}, [3]int{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rarities := []entity.Rarity{entity.RarityCommon, entity.RarityUncommon, entity.RarityRare}
			pools := map[entity.Rarity][]entity.WeaponType{}
			for i, available := range test.available {
				if available {
					pools[rarities[i]] = []entity.WeaponType{entity.WeaponBow}
				}
			}
			got := SpawnRarities(rand.New(rand.NewSource(12)), test.count, pools)
			counts := [3]int{}
			for _, rarity := range got {
				for i, value := range rarities {
					if rarity == value {
						counts[i]++
					}
				}
			}
			if counts != test.want {
				t.Fatalf("got %v, want %v", counts, test.want)
			}
			if len(got) != counts[0]+counts[1]+counts[2] {
				t.Fatal("unknown rarity")
			}
		})
	}
}

func TestSpawnRarityShuffleIsReproducible(t *testing.T) {
	pools := map[entity.Rarity][]entity.WeaponType{
		entity.RarityCommon: {entity.WeaponBow}, entity.RarityUncommon: {entity.WeaponStaff}, entity.RarityRare: {entity.WeaponSword},
	}
	a := SpawnRarities(rand.New(rand.NewSource(7)), 60, pools)
	b := SpawnRarities(rand.New(rand.NewSource(7)), 60, pools)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed produced different spawn order")
	}
	if reflect.DeepEqual(a, SpawnRarities(rand.New(rand.NewSource(8)), 60, pools)) {
		t.Fatal("spawn order was not shuffled")
	}
	if got := SpawnRarities(nil, 60, pools); len(got) != 0 {
		t.Fatal("nil RNG generated spawn plan")
	}
}
