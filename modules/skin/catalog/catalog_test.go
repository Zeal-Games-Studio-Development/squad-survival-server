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
		partType  PartType
		numericID int
	}{
		{PartHair, 1}, {PartHair, 26}, {PartHelmet, 40},
		{PartSword, 19}, {PartWand, 9}, {PartArrow, 8},
	}

	for _, test := range tests {
		byPart, ok := catalog.LookupPart(test.partType, test.numericID)
		if !ok {
			t.Errorf("LookupPart(%q, %d) failed", test.partType, test.numericID)
			continue
		}
		if byPart.PartType != test.partType || byPart.NumericID != test.numericID {
			t.Errorf("lookup mismatch: %#v", byPart)
		}
	}
}

func TestCatalogLookupRejectsUnknownItems(t *testing.T) {
	catalog := DefaultCatalog()
	for _, numericID := range []int{-1, 0, 10} {
		if _, ok := catalog.LookupPart(PartWand, numericID); ok {
			t.Errorf("LookupPart(wand, %d) unexpectedly succeeded", numericID)
		}
	}
	if _, ok := catalog.LookupPart(PartType("unknown"), 1); ok {
		t.Fatal("unknown part unexpectedly succeeded")
	}
}

func TestParseCatalogRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		message string
	}{
		{"empty catalog", `{"parts":[]}`, "no parts"},
		{"empty type", `{"parts":[{"type":"","ids":[1]}]}`, "part type"},
		{"duplicate type", `{"parts":[{"type":"hat","ids":[1]},{"type":"hat","ids":[2]}]}`, "duplicate part type"},
		{"empty IDs", `{"parts":[{"type":"hat","ids":[]}]}`, "no item ids"},
		{"zero ID", `{"parts":[{"type":"hat","ids":[0]}]}`, "greater than zero"},
		{"negative ID", `{"parts":[{"type":"hat","ids":[-1]}]}`, "greater than zero"},
		{"duplicate numeric ID", `{"parts":[{"type":"hat","ids":[1,1]}]}`, "duplicate item id"},
		{"zero exclusive ID", `{"parts":[{"type":"hat","ids":[1],"exclusive_ids":[0]}]}`, "exclusive item id must be greater than zero"},
		{"duplicate exclusive ID", `{"parts":[{"type":"hat","ids":[1],"exclusive_ids":[1,1]}]}`, "duplicate exclusive item id"},
		{"unknown exclusive ID", `{"parts":[{"type":"hat","ids":[1],"exclusive_ids":[2]}]}`, "is not present in ids"},
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
	catalog, err := ParseCatalog([]byte(`{"parts":[{"type":"hat","ids":[1,2,3],"exclusive_ids":[2]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	items := catalog.DrawableItems()
	if len(items) != 2 || items[0].NumericID != 1 || items[1].NumericID != 3 {
		t.Fatalf("unexpected drawable items: %#v", items)
	}
	exclusive, ok := catalog.LookupPart(PartType("hat"), 2)
	if !ok || !exclusive.Exclusive {
		t.Fatalf("exclusive catalog item was not retained: %#v", exclusive)
	}
}

func TestCatalogItemsPreserveOrderAndReturnDefensiveCopy(t *testing.T) {
	catalog, err := ParseCatalog([]byte(`{"parts":[{"type":"hat","ids":[2,1],"exclusive_ids":[2]},{"type":"hair","ids":[1]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	items := catalog.Items()
	if len(items) != 3 || items[0].NumericID != 2 || items[1].NumericID != 1 || items[2].PartType != PartHair || !items[0].Exclusive {
		t.Fatalf("unexpected catalog items: %#v", items)
	}
	items[0].NumericID = 999
	if unchanged := catalog.Items(); unchanged[0].NumericID != 2 {
		t.Fatalf("catalog items were mutated through returned slice: %#v", unchanged)
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
