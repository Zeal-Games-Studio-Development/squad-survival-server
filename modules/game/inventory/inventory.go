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
	ErrNotInitialized  = errors.New("character inventory is not initialized")
	ErrInvalidLoadout  = errors.New("squad loadout must contain one owned weapon id for each weapon type")
	ErrVersionConflict = errors.New("character inventory version conflict")
)

var requiredTypes = []entity.WeaponType{
	entity.WeaponBow, entity.WeaponStaff, entity.WeaponSpear, entity.WeaponSword,
	entity.WeaponAxe, entity.WeaponBlunt, entity.WeaponCrossbow,
	entity.WeaponShield,
}

var renamedStarterIDs = map[string]string{
	"bow":         "acher",
	"staff":       "novice_mage",
	"spear":       "farmer",
	"sword":       "swordman",
	"axe":         "axeman",
	"blunt":       "bruiser",
	"crossbow":    "hunter",
	"crossbowman": "hunter",
	"shield":      "shieldbearer",
}

//go:embed initial_character_inventory.json
var initialInventoryJSON []byte

type Data struct {
	WeaponIDs    []string          `json:"weapon_ids"`
	SquadLoadout map[string]string `json:"squad_loadout"`
}

type Snapshot struct {
	Data
	Version string `json:"version"`
}

type Store interface {
	StorageRead(context.Context, []*runtime.StorageRead) ([]*api.StorageObject, error)
	StorageWrite(context.Context, []*runtime.StorageWrite) ([]*api.StorageObjectAck, error)
}

type Service struct {
	byID       map[string]entity.Weapon
	initialIDs []string
}

func NewService(weapons []entity.Weapon, initialIDs []string) (*Service, error) {
	if len(weapons) == 0 {
		return nil, errors.New("weapon catalog is empty")
	}
	service := &Service{byID: make(map[string]entity.Weapon, len(weapons)), initialIDs: append([]string(nil), initialIDs...)}
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
	if _, err := service.InitialData(); err != nil {
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
	data := Data{WeaponIDs: append([]string(nil), s.initialIDs...), SquadLoadout: make(map[string]string, len(requiredTypes))}
	for _, id := range data.WeaponIDs {
		weapon, ok := s.byID[id]
		if !ok {
			return Data{}, fmt.Errorf("initial inventory has unknown weapon id %q", id)
		}
		if _, exists := data.SquadLoadout[string(weapon.Type)]; exists {
			return Data{}, fmt.Errorf("initial inventory has duplicate weapon type %q", weapon.Type)
		}
		data.SquadLoadout[string(weapon.Type)] = id
	}
	if err := s.Validate(data); err != nil {
		return Data{}, fmt.Errorf("invalid initial inventory: %w", err)
	}
	return data, nil
}

func (s *Service) Validate(data Data) error {
	owned := make(map[string]bool, len(data.WeaponIDs))
	for _, id := range data.WeaponIDs {
		if _, ok := s.byID[id]; !ok || owned[id] {
			return ErrInvalidLoadout
		}
		owned[id] = true
	}
	if len(data.SquadLoadout) != len(requiredTypes) {
		return ErrInvalidLoadout
	}
	for _, weaponType := range requiredTypes {
		id, ok := data.SquadLoadout[string(weaponType)]
		if !ok || !owned[id] || s.byID[id].Type != weaponType {
			return ErrInvalidLoadout
		}
	}
	return nil
}

func (s *Service) Starter(snapshot Snapshot, random *rand.Rand) (entity.Weapon, error) {
	if err := s.Validate(snapshot.Data); err != nil {
		return entity.Weapon{}, err
	}
	if random == nil {
		return entity.Weapon{}, errors.New("starter random source is required")
	}
	typeName := requiredTypes[random.Intn(len(requiredTypes))]
	return s.byID[snapshot.SquadLoadout[string(typeName)]], nil
}

func (s *Service) Load(ctx context.Context, store Store, userID string) (Snapshot, error) {
	if store == nil || userID == "" {
		return Snapshot{}, errors.New("inventory store and user id are required")
	}
	objects, err := store.StorageRead(ctx, []*runtime.StorageRead{{Collection: Collection, Key: Key, UserID: userID}})
	if err != nil {
		return Snapshot{}, err
	}
	if len(objects) == 0 {
		return Snapshot{}, ErrNotInitialized
	}
	if len(objects) != 1 || objects[0] == nil {
		return Snapshot{}, errors.New("invalid character inventory storage response")
	}
	var data Data
	if err := json.Unmarshal([]byte(objects[0].Value), &data); err != nil {
		return Snapshot{}, fmt.Errorf("decode character inventory: %w", err)
	}
	for index, id := range data.WeaponIDs {
		data.WeaponIDs[index] = s.currentID(id)
	}
	for weaponType, id := range data.SquadLoadout {
		data.SquadLoadout[weaponType] = s.currentID(id)
	}
	if err := s.Validate(data); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Data: data, Version: objects[0].Version}, nil
}

func (s *Service) currentID(id string) string {
	if _, exists := s.byID[id]; exists {
		return id
	}
	if replacement, renamed := renamedStarterIDs[id]; renamed {
		if _, exists := s.byID[replacement]; exists {
			return replacement
		}
	}
	return id
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

func (s *Service) SetLoadout(ctx context.Context, store Store, userID, version string, loadout map[string]string) (Snapshot, error) {
	if version == "" {
		return Snapshot{}, ErrVersionConflict
	}
	current, err := s.Load(ctx, store, userID)
	if err != nil {
		return Snapshot{}, err
	}
	if current.Version != version {
		return Snapshot{}, ErrVersionConflict
	}
	current.SquadLoadout = loadout
	if err := s.Validate(current.Data); err != nil {
		return Snapshot{}, err
	}
	value, err := json.Marshal(current.Data)
	if err != nil {
		return Snapshot{}, err
	}
	acks, err := store.StorageWrite(ctx, []*runtime.StorageWrite{{Collection: Collection, Key: Key, UserID: userID, Value: string(value), Version: version, PermissionRead: 1, PermissionWrite: 0}})
	if err != nil {
		if errors.Is(err, runtime.ErrStorageRejectedVersion) {
			return Snapshot{}, ErrVersionConflict
		}
		return Snapshot{}, err
	}
	if len(acks) != 1 || acks[0] == nil {
		return Snapshot{}, errors.New("invalid character inventory write response")
	}
	current.Version = acks[0].Version
	return current, nil
}
