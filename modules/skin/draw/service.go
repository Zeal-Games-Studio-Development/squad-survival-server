package draw

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"squad-survival-be/modules/skin/catalog"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	RPCName              = "skin_lucky_draw"
	InventoryCollection  = "player_inventory"
	InventoryKey         = "skins"
	InventorySource      = "skin_lucky_draw"
	GemCurrency          = "gem"
	SingleDrawCost       = int64(100)
	TenDrawCost          = int64(900)
	MaxProcessedRequests = 50
	maxConflictRetries   = 5
)

var (
	ErrInvalidRequest   = errors.New("invalid skin lucky draw request")
	ErrInsufficientGems = errors.New("insufficient gems")
	ErrInsufficientPool = errors.New("not enough drawable skins remain")
	ErrInvalidInventory = errors.New("invalid player skin inventory")
	ErrConcurrentUpdate = errors.New("skin inventory update conflict")
	uuidPattern         = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Request struct {
	RequestID string `json:"request_id"`
	DrawCount int    `json:"draw_count"`
}

type Response struct {
	RequestID       string   `json:"request_id"`
	DrawCount       int      `json:"draw_count"`
	ItemIDs         []string `json:"item_ids"`
	GemSpent        int64    `json:"gem_spent"`
	GemBalanceAfter int64    `json:"gem_balance_after"`
	Replayed        bool     `json:"replayed"`
}

type InventoryItem struct {
	AcquiredAt int64  `json:"acquired_at"`
	Source     string `json:"source"`
}

type ProcessedRequest struct {
	RequestID       string   `json:"request_id"`
	DrawCount       int      `json:"draw_count"`
	ItemIDs         []string `json:"item_ids"`
	GemSpent        int64    `json:"gem_spent"`
	GemBalanceAfter int64    `json:"gem_balance_after,omitempty"`
	CreatedAt       int64    `json:"created_at"`
}

type Inventory struct {
	Items             map[string]InventoryItem `json:"items"`
	ProcessedRequests []ProcessedRequest       `json:"processed_requests"`
}

type NakamaStore interface {
	AccountGetId(ctx context.Context, userID string) (*api.Account, error)
	StorageRead(ctx context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error)
	MultiUpdate(ctx context.Context, accountUpdates []*runtime.AccountUpdate, storageWrites []*runtime.StorageWrite, storageDeletes []*runtime.StorageDelete, walletUpdates []*runtime.WalletUpdate, updateLedger bool) ([]*api.StorageObjectAck, []*runtime.WalletUpdateResult, error)
}

type Random interface {
	Intn(max int) (int, error)
}

type cryptoRandom struct{}

func (cryptoRandom) Intn(max int) (int, error) {
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

type Service struct {
	catalog *catalog.Catalog
	random  Random
	now     func() time.Time
}

func NewService(itemCatalog *catalog.Catalog) (*Service, error) {
	return NewServiceWithDependencies(itemCatalog, cryptoRandom{}, time.Now)
}

func NewServiceWithDependencies(itemCatalog *catalog.Catalog, random Random, now func() time.Time) (*Service, error) {
	if itemCatalog == nil || len(itemCatalog.DrawableItems()) == 0 {
		return nil, errors.New("skin draw catalog has no drawable items")
	}
	if random == nil || now == nil {
		return nil, errors.New("skin draw dependencies must not be nil")
	}
	return &Service{catalog: itemCatalog, random: random, now: now}, nil
}

func (s *Service) Draw(ctx context.Context, store NakamaStore, userID string, request Request) (Response, error) {
	cost, err := validateRequest(userID, request)
	if err != nil {
		return Response{}, err
	}

	for attempt := 0; attempt <= maxConflictRetries; attempt++ {
		inventory, version, err := s.readInventory(ctx, store, userID)
		if err != nil {
			return Response{}, err
		}
		if receipt, ok := findProcessedRequest(inventory.ProcessedRequests, request.RequestID); ok {
			balance, err := currentGemBalance(ctx, store, userID)
			if err != nil {
				return Response{}, err
			}
			receipt.GemBalanceAfter = balance
			return responseFromReceipt(receipt, true), nil
		}

		eligible := s.eligibleItems(inventory)
		if len(eligible) < request.DrawCount {
			return Response{}, fmt.Errorf("%w: requested %d, available %d", ErrInsufficientPool, request.DrawCount, len(eligible))
		}
		selected, err := s.selectItems(eligible, request.DrawCount)
		if err != nil {
			return Response{}, fmt.Errorf("select drawable skins: %w", err)
		}
		gemBalance, err := currentGemBalance(ctx, store, userID)
		if err != nil {
			return Response{}, err
		}
		if gemBalance < cost {
			return Response{}, ErrInsufficientGems
		}

		createdAt := s.now().UTC().Unix()
		itemIDs := make([]string, len(selected))
		for i, item := range selected {
			itemIDs[i] = item.ID
			inventory.Items[item.ID] = InventoryItem{AcquiredAt: createdAt, Source: InventorySource}
		}
		receipt := ProcessedRequest{
			RequestID: request.RequestID, DrawCount: request.DrawCount, ItemIDs: itemIDs,
			GemSpent: cost, GemBalanceAfter: gemBalance - cost, CreatedAt: createdAt,
		}
		inventory.ProcessedRequests = append(inventory.ProcessedRequests, receipt)
		if len(inventory.ProcessedRequests) > MaxProcessedRequests {
			inventory.ProcessedRequests = append([]ProcessedRequest(nil), inventory.ProcessedRequests[len(inventory.ProcessedRequests)-MaxProcessedRequests:]...)
		}

		value, err := json.Marshal(inventory)
		if err != nil {
			return Response{}, fmt.Errorf("encode player skin inventory: %w", err)
		}
		metadata := map[string]interface{}{
			"event": InventorySource, "request_id": request.RequestID,
			"draw_count": request.DrawCount, "item_ids": itemIDs,
		}
		_, walletResults, err := store.MultiUpdate(ctx, nil, []*runtime.StorageWrite{{
			Collection: InventoryCollection, Key: InventoryKey, UserID: userID, Value: string(value),
			Version: version, PermissionRead: 1, PermissionWrite: 0,
		}}, nil, []*runtime.WalletUpdate{{
			UserID: userID, Changeset: map[string]int64{GemCurrency: -cost}, Metadata: metadata,
		}}, true)
		if err != nil {
			var walletNegative *runtime.WalletNegativeError
			if errors.As(err, &walletNegative) {
				return Response{}, ErrInsufficientGems
			}
			if attempt < maxConflictRetries {
				continue
			}
			return Response{}, fmt.Errorf("%w: %v", ErrConcurrentUpdate, err)
		}
		if len(walletResults) != 1 {
			return Response{}, errors.New("skin lucky draw transaction returned no wallet result")
		}
		receipt.GemBalanceAfter = walletResults[0].Updated[GemCurrency]
		return responseFromReceipt(receipt, false), nil
	}

	return Response{}, ErrConcurrentUpdate
}

func validateRequest(userID string, request Request) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("%w: user id is required", ErrInvalidRequest)
	}
	if !uuidPattern.MatchString(request.RequestID) {
		return 0, fmt.Errorf("%w: request_id must be a canonical UUID", ErrInvalidRequest)
	}
	switch request.DrawCount {
	case 1:
		return SingleDrawCost, nil
	case 10:
		return TenDrawCost, nil
	default:
		return 0, fmt.Errorf("%w: draw_count must be 1 or 10", ErrInvalidRequest)
	}
}

func (s *Service) readInventory(ctx context.Context, store NakamaStore, userID string) (Inventory, string, error) {
	objects, err := store.StorageRead(ctx, []*runtime.StorageRead{{Collection: InventoryCollection, Key: InventoryKey, UserID: userID}})
	if err != nil {
		return Inventory{}, "", fmt.Errorf("read player skin inventory: %w", err)
	}
	if len(objects) == 0 {
		return Inventory{Items: make(map[string]InventoryItem)}, "*", nil
	}
	if len(objects) != 1 {
		return Inventory{}, "", fmt.Errorf("%w: expected one object, got %d", ErrInvalidInventory, len(objects))
	}
	var inventory Inventory
	if err := json.Unmarshal([]byte(objects[0].Value), &inventory); err != nil {
		return Inventory{}, "", fmt.Errorf("%w: %v", ErrInvalidInventory, err)
	}
	if inventory.Items == nil {
		inventory.Items = make(map[string]InventoryItem)
	}
	for itemID := range inventory.Items {
		if _, ok := s.catalog.Lookup(itemID); !ok {
			return Inventory{}, "", fmt.Errorf("%w: unknown item %q", ErrInvalidInventory, itemID)
		}
	}
	return inventory, objects[0].Version, nil
}

func currentGemBalance(ctx context.Context, store NakamaStore, userID string) (int64, error) {
	account, err := store.AccountGetId(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("read player wallet: %w", err)
	}
	if account == nil {
		return 0, errors.New("read player wallet: account not found")
	}
	var wallet map[string]int64
	if err := json.Unmarshal([]byte(account.Wallet), &wallet); err != nil {
		return 0, fmt.Errorf("decode player wallet: %w", err)
	}
	return wallet[GemCurrency], nil
}

func (s *Service) eligibleItems(inventory Inventory) []catalog.Item {
	items := s.catalog.DrawableItems()
	eligible := make([]catalog.Item, 0, len(items))
	for _, item := range items {
		if _, owned := inventory.Items[item.ID]; !owned {
			eligible = append(eligible, item)
		}
	}
	return eligible
}

func (s *Service) selectItems(items []catalog.Item, count int) ([]catalog.Item, error) {
	pool := append([]catalog.Item(nil), items...)
	for i := 0; i < count; i++ {
		offset, err := s.random.Intn(len(pool) - i)
		if err != nil {
			return nil, err
		}
		j := i + offset
		pool[i], pool[j] = pool[j], pool[i]
	}
	return pool[:count], nil
}

func findProcessedRequest(requests []ProcessedRequest, requestID string) (ProcessedRequest, bool) {
	for i := len(requests) - 1; i >= 0; i-- {
		if requests[i].RequestID == requestID {
			return requests[i], true
		}
	}
	return ProcessedRequest{}, false
}

func responseFromReceipt(receipt ProcessedRequest, replayed bool) Response {
	return Response{
		RequestID: receipt.RequestID, DrawCount: receipt.DrawCount,
		ItemIDs: append([]string(nil), receipt.ItemIDs...), GemSpent: receipt.GemSpent,
		GemBalanceAfter: receipt.GemBalanceAfter, Replayed: replayed,
	}
}
