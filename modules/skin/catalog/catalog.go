package catalog

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// PartType identifies the character or equipment part to which an item belongs.
type PartType string

const (
	PartHair     PartType = "hair"
	PartBeard    PartType = "beard"
	PartAxe      PartType = "axe"
	PartArrow    PartType = "arrow"
	PartBlunt    PartType = "blunt"
	PartBow      PartType = "bow"
	PartChest    PartType = "chest"
	PartCrossbow PartType = "crossbow"
	PartEye      PartType = "eye"
	PartHelmet   PartType = "helmet"
	PartShield   PartType = "shield"
	PartSpear    PartType = "spear"
	PartStaff    PartType = "staff"
	PartSword    PartType = "sword"
	PartWand     PartType = "wand"
)

// PartDefinition describes the valid numeric item IDs for one part type.
type PartDefinition struct {
	Type         PartType `json:"type"`
	Prefix       string   `json:"prefix"`
	IDs          []int    `json:"ids"`
	ExclusiveIDs []int    `json:"exclusive_ids"`
}

// Item is a resolved catalog entry with its canonical ID.
type Item struct {
	ID        string   `json:"id"`
	PartType  PartType `json:"type"`
	NumericID int      `json:"numeric_id"`
	Prefix    string   `json:"prefix"`
	Exclusive bool     `json:"exclusive"`
}

type catalogFile struct {
	Parts []PartDefinition `json:"parts"`
}

type partItemKey struct {
	partType  PartType
	numericID int
}

// Catalog provides validated item lookups by canonical ID and part/numeric ID.
type Catalog struct {
	parts  map[PartType]PartDefinition
	byID   map[string]Item
	byPart map[partItemKey]Item
	items  []Item
}

var (
	partTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	prefixPattern   = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
)

//go:embed skins.json
var defaultCatalogJSON []byte

// ParseCatalog parses and validates an item catalog.
func ParseCatalog(data []byte) (*Catalog, error) {
	var file catalogFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode item catalog: %w", err)
	}
	if len(file.Parts) == 0 {
		return nil, errors.New("item catalog has no parts")
	}

	catalog := &Catalog{
		parts:  make(map[PartType]PartDefinition, len(file.Parts)),
		byID:   make(map[string]Item),
		byPart: make(map[partItemKey]Item),
	}
	prefixes := make(map[string]PartType, len(file.Parts))

	for _, definition := range file.Parts {
		if !partTypePattern.MatchString(string(definition.Type)) {
			return nil, fmt.Errorf("part type %q must match %s", definition.Type, partTypePattern)
		}
		if !prefixPattern.MatchString(definition.Prefix) {
			return nil, fmt.Errorf("part prefix %q must match %s", definition.Prefix, prefixPattern)
		}
		if len(definition.IDs) == 0 {
			return nil, fmt.Errorf("part %q has no item ids", definition.Type)
		}
		if _, exists := catalog.parts[definition.Type]; exists {
			return nil, fmt.Errorf("duplicate part type %q", definition.Type)
		}
		if otherType, exists := prefixes[definition.Prefix]; exists {
			return nil, fmt.Errorf("duplicate part prefix %q for %q and %q", definition.Prefix, otherType, definition.Type)
		}

		seenNumericIDs := make(map[int]struct{}, len(definition.IDs))
		exclusiveIDs := make(map[int]struct{}, len(definition.ExclusiveIDs))
		for _, numericID := range definition.ExclusiveIDs {
			if numericID <= 0 {
				return nil, fmt.Errorf("part %q exclusive item id must be greater than zero", definition.Type)
			}
			if _, exists := exclusiveIDs[numericID]; exists {
				return nil, fmt.Errorf("part %q has duplicate exclusive item id %d", definition.Type, numericID)
			}
			exclusiveIDs[numericID] = struct{}{}
		}
		for _, numericID := range definition.IDs {
			if numericID <= 0 {
				return nil, fmt.Errorf("part %q item id must be greater than zero", definition.Type)
			}
			if _, exists := seenNumericIDs[numericID]; exists {
				return nil, fmt.Errorf("part %q has duplicate item id %d", definition.Type, numericID)
			}
			seenNumericIDs[numericID] = struct{}{}

			canonicalID := definition.Prefix + strconv.Itoa(numericID)
			if other, exists := catalog.byID[canonicalID]; exists {
				return nil, fmt.Errorf("canonical item id %q collides between %q and %q", canonicalID, other.PartType, definition.Type)
			}
			_, exclusive := exclusiveIDs[numericID]
			item := Item{ID: canonicalID, PartType: definition.Type, NumericID: numericID, Prefix: definition.Prefix, Exclusive: exclusive}
			catalog.byID[canonicalID] = item
			catalog.byPart[partItemKey{partType: definition.Type, numericID: numericID}] = item
			catalog.items = append(catalog.items, item)
		}
		for numericID := range exclusiveIDs {
			if _, exists := seenNumericIDs[numericID]; !exists {
				return nil, fmt.Errorf("part %q exclusive item id %d is not present in ids", definition.Type, numericID)
			}
		}

		definition.IDs = append([]int(nil), definition.IDs...)
		definition.ExclusiveIDs = append([]int(nil), definition.ExclusiveIDs...)
		catalog.parts[definition.Type] = definition
		prefixes[definition.Prefix] = definition.Type
	}

	return catalog, nil
}

// DefaultCatalog returns the validated catalog embedded in this package.
func DefaultCatalog() *Catalog {
	catalog, err := ParseCatalog(defaultCatalogJSON)
	if err != nil {
		panic("invalid embedded item catalog: " + err.Error())
	}
	return catalog
}

// Lookup finds an item by its canonical ID, such as "wn3".
func (c *Catalog) Lookup(itemID string) (Item, bool) {
	if c == nil {
		return Item{}, false
	}
	item, ok := c.byID[itemID]
	return item, ok
}

// LookupPart finds an item by its part type and numeric ID.
func (c *Catalog) LookupPart(partType PartType, numericID int) (Item, bool) {
	if c == nil {
		return Item{}, false
	}
	item, ok := c.byPart[partItemKey{partType: partType, numericID: numericID}]
	return item, ok
}

// CanonicalID returns the canonical ID for a valid part/numeric ID pair.
func (c *Catalog) CanonicalID(partType PartType, numericID int) (string, bool) {
	item, ok := c.LookupPart(partType, numericID)
	if !ok {
		return "", false
	}
	return item.ID, true
}

// Part returns a copy of a part definition.
func (c *Catalog) Part(partType PartType) (PartDefinition, bool) {
	if c == nil {
		return PartDefinition{}, false
	}
	definition, ok := c.parts[partType]
	if !ok {
		return PartDefinition{}, false
	}
	definition.IDs = append([]int(nil), definition.IDs...)
	definition.ExclusiveIDs = append([]int(nil), definition.ExclusiveIDs...)
	return definition, true
}

// DrawableItems returns all non-exclusive items in their catalog order.
func (c *Catalog) DrawableItems() []Item {
	if c == nil {
		return nil
	}
	items := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !item.Exclusive {
			items = append(items, item)
		}
	}
	return items
}

// Len returns the number of concrete items in the catalog.
func (c *Catalog) Len() int {
	if c == nil {
		return 0
	}
	return len(c.byID)
}
