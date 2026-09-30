package combat

import (
	"math"
	"testing"

	"squad-survival-be/modules/game/core/entity"
)

func TestDefaultConfigUsesFiveUnitQueryBuffer(t *testing.T) {
	if got := DefaultConfig().QueryBuffer; got != 5 {
		t.Fatalf("unexpected query buffer: %f", got)
	}
}

func TestCombatConfigRejectsInvalidQueryBuffer(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{}`), []byte(`{"query_buffer":0}`), []byte(`{"query_buffer":-1}`), []byte(`not json`)} {
		if _, err := ParseConfig(data); err == nil {
			t.Fatalf("expected invalid config to fail: %s", data)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := (Config{QueryBuffer: value}).Validate(); err == nil {
			t.Fatalf("expected invalid query buffer to fail: %f", value)
		}
	}
}

func TestValidateWeaponRangesIncludesBuffer(t *testing.T) {
	config := Config{QueryBuffer: 5}
	if err := ValidateWeaponRanges([]entity.Weapon{{Type: entity.WeaponBow, AttackRange: 15}}, 20, config); err != nil {
		t.Fatalf("expected boundary range to be valid: %v", err)
	}
	if err := ValidateWeaponRanges([]entity.Weapon{{Type: entity.WeaponBow, AttackRange: 15.01}}, 20, config); err == nil {
		t.Fatal("expected attack range plus buffer above detection radius to fail")
	}
}
