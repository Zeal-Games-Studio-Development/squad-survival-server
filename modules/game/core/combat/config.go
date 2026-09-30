package combat

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"squad-survival-be/modules/game/core/entity"
)

type Config struct {
	QueryBuffer float64 `json:"query_buffer"`
}

//go:embed config.json
var defaultConfigJSON []byte

func ParseConfig(data []byte) (Config, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	if math.IsNaN(c.QueryBuffer) || math.IsInf(c.QueryBuffer, 0) || c.QueryBuffer <= 0 {
		return errors.New("combat query buffer must be finite and greater than zero")
	}
	return nil
}

func DefaultConfig() Config {
	config, err := ParseConfig(defaultConfigJSON)
	if err != nil {
		panic("invalid embedded combat config: " + err.Error())
	}
	return config
}

func ValidateWeaponRanges(weapons []entity.Weapon, detectionRadius float64, config Config) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if math.IsNaN(detectionRadius) || math.IsInf(detectionRadius, 0) || detectionRadius <= 0 {
		return errors.New("detection radius must be finite and greater than zero")
	}
	for _, weapon := range weapons {
		if weapon.AttackRange+config.QueryBuffer > detectionRadius {
			return fmt.Errorf("weapon %q attack range %.2f plus combat query buffer %.2f exceeds detection radius %.2f", weapon.Type, weapon.AttackRange, config.QueryBuffer, detectionRadius)
		}
	}
	return nil
}
