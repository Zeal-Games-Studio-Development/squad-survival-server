package economy

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/heroiclabs/nakama-common/runtime"
)

const CostChangeDisplayName = "change_display_name"

// GeneralConfig contains feature prices, separate from currency definitions.
type GeneralConfig struct {
	Costs map[string]map[string]int64 `json:"costs"`
}

//go:embed economy_general.json
var defaultGeneralJSON []byte

func (s *Service) ParseGeneralConfig(data []byte) (GeneralConfig, error) {
	var config GeneralConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return GeneralConfig{}, fmt.Errorf("decode economy general config: %w", err)
	}
	if len(config.Costs[CostChangeDisplayName]) == 0 {
		return GeneralConfig{}, fmt.Errorf("economy config requires cost %q", CostChangeDisplayName)
	}
	for feature, costs := range config.Costs {
		if _, err := s.changeset(costs, -1); err != nil {
			return GeneralConfig{}, fmt.Errorf("economy cost %q: %w", feature, err)
		}
	}
	return config, nil
}

func (s *Service) DefaultGeneralConfig() (GeneralConfig, error) {
	return s.ParseGeneralConfig(defaultGeneralJSON)
}

// PaymentUpdate builds a validated debit for a larger atomic MultiUpdate.
func (s *Service) PaymentUpdate(userID string, costs map[string]int64, metadata map[string]interface{}) (*runtime.WalletUpdate, error) {
	changeset, err := s.changeset(costs, -1)
	if err != nil {
		return nil, err
	}
	return &runtime.WalletUpdate{UserID: userID, Changeset: changeset, Metadata: withEvent(metadata, EventPayment)}, nil
}
