package characterbox

import (
	"math/rand"
	"reflect"
	"squad-survival-be/modules/game/core/entity"
	"testing"
)

func TestOwnedSelectionFiltersIDTypeAndRarity(t *testing.T) {
	catalog := []entity.Weapon{
		{ID: "one", Type: entity.WeaponBow, Rarity: entity.RarityCommon},
		{ID: "two", Type: entity.WeaponBow, Rarity: entity.RarityCommon},
		{ID: "unowned", Type: entity.WeaponBow, Rarity: entity.RarityCommon},
		{ID: "rare", Type: entity.WeaponBow, Rarity: entity.RarityRare},
		{ID: "sword", Type: entity.WeaponSword, Rarity: entity.RarityCommon},
	}
	owned := Owned(catalog, []string{"one", "two", "rare", "sword", "two"})
	matches := Filter(owned, entity.WeaponBow, entity.RarityCommon)
	random := rand.New(rand.NewSource(3))
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		weapon, ok := Choose(random, matches)
		if !ok || (weapon.ID != "one" && weapon.ID != "two") {
			t.Fatalf("invalid selection: %+v", weapon)
		}
		seen[weapon.ID] = true
	}
	if len(seen) != 2 {
		t.Fatalf("did not select both common variants: %v", seen)
	}
	if _, ok := Choose(random, Filter(owned, entity.WeaponStaff, entity.RarityCommon)); ok {
		t.Fatal("empty pool selected a weapon")
	}
	if _, ok := Choose(nil, matches); ok {
		t.Fatal("nil RNG selected a weapon")
	}
	if types := SpawnTypes(catalog, [][]entity.Weapon{owned}, entity.RarityCommon); !reflect.DeepEqual(types, []entity.WeaponType{entity.WeaponBow, entity.WeaponSword}) {
		t.Fatalf("wrong type pool: %v", types)
	}
	if types := SpawnTypes(catalog, [][]entity.Weapon{{catalog[3]}}, entity.RarityCommon); len(types) != 0 {
		t.Fatalf("rare entered spawn pool: %v", types)
	}
	if types := SpawnTypes(catalog, nil, entity.RarityCommon); len(types) != 0 {
		t.Fatal("empty inventories entered pool")
	}
}

func TestSpawnTypesFiltersEachRarity(t *testing.T) {
	catalog := entity.DefaultWeaponCatalog()
	owned := Owned(catalog, []string{"archer", "archer_uncommon", "bruiser_rare"})
	for rarity, want := range map[entity.Rarity]entity.WeaponType{entity.RarityCommon: entity.WeaponBow, entity.RarityUncommon: entity.WeaponBow, entity.RarityRare: entity.WeaponBlunt} {
		got := SpawnTypes(catalog, [][]entity.Weapon{owned, owned}, rarity)
		if !reflect.DeepEqual(got, []entity.WeaponType{want}) {
			t.Fatalf("wrong %s types: %v", rarity, got)
		}
	}
}
