package economy

import "testing"

func TestGeneralConfig(t *testing.T) {
	service, err := NewService(DefaultCurrencies())
	if err != nil {
		t.Fatal(err)
	}
	config, err := service.DefaultGeneralConfig()
	if err != nil || config.Costs[CostChangeDisplayName]["gem"] != 500 {
		t.Fatalf("unexpected default: %+v, %v", config, err)
	}
	for _, data := range []string{
		`{`, `{}`, `{"costs":{"change_display_name":{}}}`,
		`{"costs":{"change_display_name":{"unknown":500}}}`,
		`{"costs":{"change_display_name":{"gem":0}}}`,
		`{"costs":{"change_display_name":{"gem":-1}}}`,
		`{"costs":{"change_display_name":{"gem":1.5}}}`,
		`{"costs":{"change_display_name":{"gem":500},"gacha":{}}}`,
	} {
		if _, err := service.ParseGeneralConfig([]byte(data)); err == nil {
			t.Errorf("accepted invalid config: %s", data)
		}
	}
	if _, err := service.ParseGeneralConfig([]byte(`{"costs":{"change_display_name":{"gem":500},"gacha_character":{"gold":100,"gem":10}}}`)); err != nil {
		t.Fatal(err)
	}
}
