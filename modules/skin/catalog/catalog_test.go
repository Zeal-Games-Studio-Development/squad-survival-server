package catalog

import (
	"strings"
	"testing"
)

func TestDefaultCatalogCounts(t *testing.T) {
	catalog := DefaultCatalog()
	expected := map[PartType]int{
		PartHair: 26, PartBeard: 15, PartAxe: 12, PartArrow: 8, PartBlunt: 9,
		PartBow: 10, PartChest: 29, PartCrossbow: 9, PartEye: 18, PartHelmet: 40,
		PartShield: 10, PartSpear: 10, PartStaff: 10, PartSword: 19, PartWand: 9,
	}

	total := 0
	for partType, count := range expected {
		definition, ok := catalog.Part(partType)
		if !ok {
			t.Fatalf("missing part %q", partType)
		}
		if len(definition.IDs) != count {
			t.Errorf("part %q has %d IDs, want %d", partType, len(definition.IDs), count)
		}
		total += count
	}
	if catalog.Len() != total {
		t.Fatalf("catalog has %d items, want %d", catalog.Len(), total)
	}
	if _, ok := catalog.Part(PartType("skin")); ok {
		t.Fatal("skin must not be present before its IDs are defined")
	}
}

func TestCatalogBoundaryLookups(t *testing.T) {
	catalog := DefaultCatalog()
	tests := []struct {
		itemID    string
		partType  PartType
		numericID int
	}{
		{"r1", PartHair, 1}, {"r26", PartHair, 26}, {"h40", PartHelmet, 40},
		{"sw19", PartSword, 19}, {"wn9", PartWand, 9}, {"ar8", PartArrow, 8},
	}

	for _, test := range tests {
		byID, ok := catalog.Lookup(test.itemID)
		if !ok {
			t.Errorf("Lookup(%q) failed", test.itemID)
			continue
		}
		byPart, ok := catalog.LookupPart(test.partType, test.numericID)
		if !ok {
			t.Errorf("LookupPart(%q, %d) failed", test.partType, test.numericID)
			continue
		}
		canonicalID, ok := catalog.CanonicalID(test.partType, test.numericID)
		if !ok || canonicalID != test.itemID {
			t.Errorf("CanonicalID(%q, %d) = %q, %v", test.partType, test.numericID, canonicalID, ok)
		}
		if byID != byPart || byID.ID != test.itemID {
			t.Errorf("lookup mismatch: by ID %#v, by part %#v", byID, byPart)
		}
	}
}

func TestCatalogLookupRejectsUnknownItems(t *testing.T) {
	catalog := DefaultCatalog()
	for _, itemID := range []string{"", "r0", "r27", "h41", "xx1", "wn01"} {
		if _, ok := catalog.Lookup(itemID); ok {
			t.Errorf("Lookup(%q) unexpectedly succeeded", itemID)
		}
	}
	for _, numericID := range []int{-1, 0, 10} {
		if _, ok := catalog.LookupPart(PartWand, numericID); ok {
			t.Errorf("LookupPart(wand, %d) unexpectedly succeeded", numericID)
		}
		if _, ok := catalog.CanonicalID(PartWand, numericID); ok {
			t.Errorf("CanonicalID(wand, %d) unexpectedly succeeded", numericID)
		}
	}
}

func TestParseCatalogRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		message string
	}{
		{"empty catalog", `{"parts":[]}`, "no parts"},
		{"empty type", `{"parts":[{"type":"","prefix":"x","ids":[1]}]}`, "part type"},
		{"empty prefix", `{"parts":[{"type":"hat","prefix":"","ids":[1]}]}`, "part prefix"},
		{"duplicate type", `{"parts":[{"type":"hat","prefix":"h","ids":[1]},{"type":"hat","prefix":"x","ids":[1]}]}`, "duplicate part type"},
		{"duplicate prefix", `{"parts":[{"type":"hat","prefix":"h","ids":[1]},{"type":"hair","prefix":"h","ids":[2]}]}`, "duplicate part prefix"},
		{"empty IDs", `{"parts":[{"type":"hat","prefix":"h","ids":[]}]}`, "no item ids"},
		{"zero ID", `{"parts":[{"type":"hat","prefix":"h","ids":[0]}]}`, "greater than zero"},
		{"negative ID", `{"parts":[{"type":"hat","prefix":"h","ids":[-1]}]}`, "greater than zero"},
		{"duplicate numeric ID", `{"parts":[{"type":"hat","prefix":"h","ids":[1,1]}]}`, "duplicate item id"},
		{"zero exclusive ID", `{"parts":[{"type":"hat","prefix":"h","ids":[1],"exclusive_ids":[0]}]}`, "exclusive item id must be greater than zero"},
		{"duplicate exclusive ID", `{"parts":[{"type":"hat","prefix":"h","ids":[1],"exclusive_ids":[1,1]}]}`, "duplicate exclusive item id"},
		{"unknown exclusive ID", `{"parts":[{"type":"hat","prefix":"h","ids":[1],"exclusive_ids":[2]}]}`, "is not present in ids"},
		{"canonical collision", `{"parts":[{"type":"first","prefix":"a","ids":[11]},{"type":"second","prefix":"a1","ids":[1]}]}`, "collides"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseCatalog([]byte(test.data))
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("ParseCatalog() error = %v, want message containing %q", err, test.message)
			}
		})
	}
}

func TestCatalogDrawableItemsExcludeExclusive(t *testing.T) {
	catalog, err := ParseCatalog([]byte(`{"parts":[{"type":"hat","prefix":"h","ids":[1,2,3],"exclusive_ids":[2]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	items := catalog.DrawableItems()
	if len(items) != 2 || items[0].ID != "h1" || items[1].ID != "h3" {
		t.Fatalf("unexpected drawable items: %#v", items)
	}
	exclusive, ok := catalog.Lookup("h2")
	if !ok || !exclusive.Exclusive {
		t.Fatalf("exclusive catalog item was not retained: %#v", exclusive)
	}
}

func TestPartReturnsDefensiveIDCopy(t *testing.T) {
	catalog := DefaultCatalog()
	definition, _ := catalog.Part(PartHair)
	definition.IDs[0] = 999

	unchanged, _ := catalog.Part(PartHair)
	if unchanged.IDs[0] != 1 {
		t.Fatalf("catalog IDs were mutated through returned definition: %#v", unchanged.IDs)
	}
}
