package economy

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Currency describes a wallet currency and the balance granted to a new user.
type Currency struct {
	ID        string `json:"id"`
	InitValue int64  `json:"init_value"`
}

type currencyFile struct {
	Currencies []Currency `json:"currencies"`
}

var currencyIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

//go:embed currency.json
var defaultCurrencyJSON []byte

func ParseCurrencies(data []byte) ([]Currency, error) {
	var file currencyFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode currency config: %w", err)
	}
	if len(file.Currencies) == 0 {
		return nil, errors.New("currency config has no currencies")
	}

	seen := make(map[string]struct{}, len(file.Currencies))
	for _, currency := range file.Currencies {
		if !currencyIDPattern.MatchString(currency.ID) {
			return nil, fmt.Errorf("currency id %q must match %s", currency.ID, currencyIDPattern)
		}
		if currency.InitValue < 0 {
			return nil, fmt.Errorf("currency %q init_value must not be negative", currency.ID)
		}
		if _, exists := seen[currency.ID]; exists {
			return nil, fmt.Errorf("duplicate currency id %q", currency.ID)
		}
		seen[currency.ID] = struct{}{}
	}

	return file.Currencies, nil
}

func DefaultCurrencies() []Currency {
	currencies, err := ParseCurrencies(defaultCurrencyJSON)
	if err != nil {
		panic("invalid embedded currency config: " + err.Error())
	}
	return currencies
}
