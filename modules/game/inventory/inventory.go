package inventory

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"

	"squad-survival-be/modules/game/core/entity"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	Collection = "character_inventory"
	Key        = "inventory"
)

var (
	ErrNotInitialized   = errors.New("character inventory is not initialized")
	ErrInvalidInventory = errors.New("character inventory must contain unique, known weapon ids")
)

var requiredTypes = []entity.WeaponType{
	entity.WeaponBow, entity.WeaponStaff, entity.WeaponSpear, entity.WeaponSword,
	entity.WeaponAxe, entity.WeaponBlunt, entity.WeaponCrossbow,
	entity.WeaponShield,
}

//go:embed initial_character_inventory.json
var initialInventoryJSON []byte

type Data struct {
	WeaponIDs []string `json:"weapon_ids"`
}

type Store interface {
	StorageRead(context.Context, []*runtime.StorageRead) ([]*api.StorageObject, error)
	StorageWrite(context.Context, []*runtime.StorageWrite) ([]*api.StorageObjectAck, error)
}

type Service struct {
	byID          map[string]entity.Weapon
	initialIDs    []string
	initialByType map[entity.WeaponType]entity.Weapon
}

func NewService(weapons []entity.Weapon, initialIDs []string) (*Service, error) {
	if len(weapons) == 0 {
		return nil, errors.New("weapon catalog is empty")
	}
	service := &Service{byID: make(map[string]entity.Weapon, len(weapons)), initialIDs: append([]string(nil), initialIDs...), initialByType: make(map[entity.WeaponType]entity.Weapon, len(requiredTypes))}
	types := make(map[entity.WeaponType]bool)
	for _, weapon := range weapons {
		if weapon.ID == "" || weapon.Type == "" {
			return nil, errors.New("weapon id and type are required")
		}
		if _, exists := service.byID[weapon.ID]; exists {
			return nil, fmt.Errorf("duplicate weapon id %q", weapon.ID)
		}
		service.byID[weapon.ID] = weapon
		types[weapon.Type] = true
	}
	if len(types) != len(requiredTypes) {
		return nil, fmt.Errorf("weapon catalog must define exactly %d weapon types", len(requiredTypes))
	}
	for _, weaponType := range requiredTypes {
		if !types[weaponType] {
			return nil, fmt.Errorf("missing weapon type %q", weaponType)
		}
	}
	if len(initialIDs) != len(requiredTypes) {
		return nil, fmt.Errorf("initial inventory must contain exactly %d weapon ids", len(requiredTypes))
	}
	for _, id := range initialIDs {
		weapon, ok := service.byID[id]
		if !ok {
			return nil, fmt.Errorf("initial inventory has unknown weapon id %q", id)
		}
		if _, exists := service.initialByType[weapon.Type]; exists {
			return nil, fmt.Errorf("initial inventory has duplicate weapon type %q", weapon.Type)
		}
		service.initialByType[weapon.Type] = weapon
	}
	if err := service.Validate(Data{WeaponIDs: initialIDs}); err != nil {
		return nil, err
	}
	return service, nil
}

func DefaultService() *Service {
	var config struct {
		WeaponIDs []string `json:"weapon_ids"`
	}
	if err := json.Unmarshal(initialInventoryJSON, &config); err != nil {
		panic("invalid initial character inventory: " + err.Error())
	}
	service, err := NewService(entity.DefaultWeaponCatalog(), config.WeaponIDs)
	if err != nil {
		panic("invalid initial character inventory: " + err.Error())
	}
	return service
}

func (s *Service) InitialData() (Data, error) {
	return Data{WeaponIDs: append([]string(nil), s.initialIDs...)}, nil
}

func (s *Service) Validate(data Data) error {
	if len(data.WeaponIDs) == 0 {
		return ErrInvalidInventory
	}
	owned := make(map[string]bool, len(data.WeaponIDs))
	for _, id := range data.WeaponIDs {
		if _, ok := s.byID[id]; !ok || owned[id] {
			return ErrInvalidInventory
		}
		owned[id] = true
	}
	return nil
}

func (s *Service) Starter(random *rand.Rand) (entity.Weapon, error) {
	if random == nil {
		return entity.Weapon{}, errors.New("starter random source is required")
	}
	return s.byID[s.initialIDs[random.Intn(len(s.initialIDs))]], nil
}

func (s *Service) InitialWeapon(weaponType entity.WeaponType) (entity.Weapon, bool) {
	weapon, ok := s.initialByType[weaponType]
	return weapon, ok
}

func (s *Service) Load(ctx context.Context, store Store, userID string) (Data, error) {
	if store == nil || userID == "" {
		return Data{}, errors.New("inventory store and user id are required")
	}
	objects, err := store.StorageRead(ctx, []*runtime.StorageRead{{Collection: Collection, Key: Key, UserID: userID}})
	if err != nil {
		return Data{}, err
	}
	if len(objects) == 0 {
		return Data{}, ErrNotInitialized
	}
	if len(objects) != 1 || objects[0] == nil {
		return Data{}, errors.New("invalid character inventory storage response")
	}
	var data Data
	if err := json.Unmarshal([]byte(objects[0].Value), &data); err != nil {
		return Data{}, fmt.Errorf("decode character inventory: %w", err)
	}
	if err := s.Validate(data); err != nil {
		return Data{}, err
	}
	return data, nil
}

func (s *Service) InitializeNewAccount(ctx context.Context, store Store, userID string) error {
	if store == nil || userID == "" {
		return errors.New("inventory store and user id are required")
	}
	data, err := s.InitialData()
	if err != nil {
		return err
	}
	value, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = store.StorageWrite(ctx, []*runtime.StorageWrite{{Collection: Collection, Key: Key, UserID: userID, Value: string(value), PermissionRead: 1, PermissionWrite: 0}})
	return err
}
