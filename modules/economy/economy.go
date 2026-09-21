package economy

import (
	"context"
	"errors"
	"fmt"
)

const (
	LedgerEventKey            = "event"
	EventInitialCurrencyGrant = "initial_currency_grant"
	EventPayment              = "payment"
	EventCurrencyGrant        = "currency_grant"
)

var ErrInvalidAmount = errors.New("currency amount must be greater than zero")

// WalletUpdater is the subset of Nakama used by the economy service.
type WalletUpdater interface {
	WalletUpdate(ctx context.Context, userID string, changeset map[string]int64, metadata map[string]interface{}, updateLedger bool) (updated, previous map[string]int64, err error)
}

type Service struct {
	currencies map[string]Currency
}

func NewService(currencies []Currency) (*Service, error) {
	if len(currencies) == 0 {
		return nil, errors.New("at least one currency is required")
	}

	known := make(map[string]Currency, len(currencies))
	for _, currency := range currencies {
		if !currencyIDPattern.MatchString(currency.ID) {
			return nil, fmt.Errorf("invalid currency id %q", currency.ID)
		}
		if currency.InitValue < 0 {
			return nil, fmt.Errorf("currency %q init value must not be negative", currency.ID)
		}
		if _, exists := known[currency.ID]; exists {
			return nil, fmt.Errorf("duplicate currency id %q", currency.ID)
		}
		known[currency.ID] = currency
	}
	return &Service{currencies: known}, nil
}

// Pay atomically subtracts the requested amounts from a Nakama wallet.
// Nakama rejects the complete update when any resulting balance is negative.
func (s *Service) Pay(ctx context.Context, wallet WalletUpdater, userID string, costs map[string]int64, metadata map[string]interface{}) (map[string]int64, error) {
	changeset, err := s.changeset(costs, -1)
	if err != nil {
		return nil, err
	}
	updated, _, err := wallet.WalletUpdate(ctx, userID, changeset, withEvent(metadata, EventPayment), true)
	return updated, err
}

// Grant atomically adds currency to a Nakama wallet.
func (s *Service) Grant(ctx context.Context, wallet WalletUpdater, userID string, amounts map[string]int64, metadata map[string]interface{}) (map[string]int64, error) {
	changeset, err := s.changeset(amounts, 1)
	if err != nil {
		return nil, err
	}
	updated, _, err := wallet.WalletUpdate(ctx, userID, changeset, withEvent(metadata, EventCurrencyGrant), true)
	return updated, err
}

func (s *Service) GrantInitialCurrency(ctx context.Context, wallet WalletUpdater, userID string) error {
	changeset := make(map[string]int64, len(s.currencies))
	for id, currency := range s.currencies {
		if currency.InitValue > 0 {
			changeset[id] = currency.InitValue
		}
	}
	if len(changeset) == 0 {
		return nil
	}
	_, _, err := wallet.WalletUpdate(ctx, userID, changeset, map[string]interface{}{LedgerEventKey: EventInitialCurrencyGrant}, true)
	return err
}

func (s *Service) changeset(amounts map[string]int64, sign int64) (map[string]int64, error) {
	if len(amounts) == 0 {
		return nil, errors.New("currency amounts must not be empty")
	}
	changeset := make(map[string]int64, len(amounts))
	for id, amount := range amounts {
		if _, exists := s.currencies[id]; !exists {
			return nil, fmt.Errorf("unknown currency %q", id)
		}
		if amount <= 0 {
			return nil, fmt.Errorf("currency %q: %w", id, ErrInvalidAmount)
		}
		changeset[id] = amount * sign
	}
	return changeset, nil
}

func withEvent(metadata map[string]interface{}, event string) map[string]interface{} {
	result := make(map[string]interface{}, len(metadata)+1)
	for key, value := range metadata {
		result[key] = value
	}
	result[LedgerEventKey] = event
	return result
}
