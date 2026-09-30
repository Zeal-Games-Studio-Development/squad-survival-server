package loadout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"squad-survival-be/modules/game/core/entity"
	"squad-survival-be/modules/skin/catalog"
	"squad-survival-be/modules/skin/draw"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type StorageReader interface {
	StorageRead(ctx context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error)
}

// Owned groups the numeric IDs a player owns by catalog part.
type Owned map[catalog.PartType][]int

type LoadError struct {
	UserID string
	Err    error
}

func (e LoadError) Error() string {
	return fmt.Sprintf("load skin inventory for user %s: %v", e.UserID, e.Err)
}

// Load reads one inventory snapshot for each user. Every requested user is
// present in the result; an empty Owned value means the selector uses ID 1.
func Load(ctx context.Context, reader StorageReader, userIDs []string, itemCatalog *catalog.Catalog) (map[string]Owned, []error) {
	result := make(map[string]Owned)
	reads := make([]*runtime.StorageRead, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		if _, exists := result[userID]; exists {
			continue
		}
		result[userID] = Owned{}
		reads = append(reads, &runtime.StorageRead{Collection: draw.InventoryCollection, Key: draw.InventoryKey, UserID: userID})
	}
	if len(reads) == 0 {
		return result, nil
	}
	if reader == nil || itemCatalog == nil {
		err := errors.New("skin storage reader and catalog are required")
		return result, errorsForUsers(result, err)
	}

	objects, err := reader.StorageRead(ctx, reads)
	if err != nil {
		return result, errorsForUsers(result, err)
	}
	seen := make(map[string]struct{}, len(objects))
	loadErrors := make([]error, 0)
	for _, object := range objects {
		if object == nil {
			continue
		}
		userID := object.GetUserId()
		if _, requested := result[userID]; !requested {
			continue
		}
		if _, duplicate := seen[userID]; duplicate {
			result[userID] = Owned{}
			loadErrors = append(loadErrors, LoadError{UserID: userID, Err: errors.New("multiple skin inventory objects returned")})
			continue
		}
		seen[userID] = struct{}{}
		owned, parseErr := parseInventory(object.GetValue(), itemCatalog)
		if parseErr != nil {
			loadErrors = append(loadErrors, LoadError{UserID: userID, Err: parseErr})
			continue
		}
		result[userID] = owned
	}
	for userID := range result {
		if _, ok := seen[userID]; !ok {
			loadErrors = append(loadErrors, LoadError{UserID: userID, Err: errors.New("skin inventory not found")})
		}
	}
	return result, loadErrors
}

func parseInventory(value string, itemCatalog *catalog.Catalog) (Owned, error) {
	var inventory draw.Inventory
	if err := json.Unmarshal([]byte(value), &inventory); err != nil {
		return nil, fmt.Errorf("decode skin inventory: %w", err)
	}
	owned := make(Owned)
	seen := make(map[partID]struct{}, len(inventory.Items))
	for _, stored := range inventory.Items {
		item, ok := itemCatalog.LookupPart(stored.Key, stored.ID)
		if !ok {
			return nil, fmt.Errorf("unknown skin item key=%q id=%d", stored.Key, stored.ID)
		}
		key := partID{partType: stored.Key, numericID: stored.ID}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("duplicate skin item key=%q id=%d", stored.Key, stored.ID)
		}
		seen[key] = struct{}{}
		owned[item.PartType] = append(owned[item.PartType], item.NumericID)
	}
	for partType := range owned {
		sort.Ints(owned[partType])
	}
	return owned, nil
}

type partID struct {
	partType  catalog.PartType
	numericID int
}

func errorsForUsers(users map[string]Owned, err error) []error {
	result := make([]error, 0, len(users))
	for userID := range users {
		result = append(result, LoadError{UserID: userID, Err: err})
	}
	return result
}

// RandomSkin selects appearance IDs and the IDs relevant to the character's weapon.
func RandomSkin(random *rand.Rand, owned Owned, weaponType entity.WeaponType) entity.Skin {
	skin := entity.Skin{
		HairID:   chooseAppearance(random, owned[catalog.PartHair]),
		BeardID:  chooseAppearance(random, owned[catalog.PartBeard]),
		ChestID:  chooseAppearance(random, owned[catalog.PartChest]),
		EyeID:    chooseAppearance(random, owned[catalog.PartEye]),
		HelmetID: chooseAppearance(random, owned[catalog.PartHelmet]),
	}
	if partType, ok := weaponPart(weaponType); ok {
		skin.WeaponID = choose(random, owned[partType])
	}
	if weaponType == entity.WeaponBow {
		skin.ProjectileID = choose(random, owned[catalog.PartArrow])
	}
	return skin
}

func chooseAppearance(random *rand.Rand, ids []int) int {
	if len(ids) == 0 {
		ids = []int{1}
	}
	if random == nil {
		return 0
	}
	offset := random.Intn(len(ids) + 1)
	if offset == 0 {
		return 0
	}
	return ids[offset-1]
}

func choose(random *rand.Rand, ids []int) int {
	if len(ids) == 0 {
		return 1
	}
	if random == nil || len(ids) == 1 {
		return ids[0]
	}
	return ids[random.Intn(len(ids))]
}

func weaponPart(weaponType entity.WeaponType) (catalog.PartType, bool) {
	switch weaponType {
	case entity.WeaponAxe:
		return catalog.PartAxe, true
	case entity.WeaponBlunt:
		return catalog.PartBlunt, true
	case entity.WeaponBow:
		return catalog.PartBow, true
	case entity.WeaponSpear:
		return catalog.PartSpear, true
	case entity.WeaponStaff:
		return catalog.PartStaff, true
	case entity.WeaponSword:
		return catalog.PartSword, true
	case entity.WeaponWand:
		return catalog.PartWand, true
	default:
		return "", false
	}
}
