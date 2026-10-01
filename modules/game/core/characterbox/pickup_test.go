package characterbox

import (
	"testing"

	"squad-survival-be/modules/game/core/strategy"
)

func TestDefaultPickupDelays(t *testing.T) {
	delays := DefaultDelays()
	wantSeconds := map[int]int64{1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 5, 7: 5, 8: 6}
	for count := 1; count < strategy.MaxCharacters; count++ {
		ticks, ok := DelayTicks(delays, count, 10)
		if !ok || ticks != wantSeconds[count]*10 {
			t.Fatalf("unexpected delay for %d characters: ticks=%d ok=%v", count, ticks, ok)
		}
	}
	if _, ok := DelayTicks(delays, strategy.MaxCharacters, 10); ok {
		t.Fatal("expected a full roster to have no pickup delay")
	}
}

func TestParsePickupDelaysRejectsInvalidConfig(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"delays_seconds":{"1":1,"2":2,"3":3}}`),
		[]byte(`{"delays_seconds":{"1":1,"2":0,"3":3,"4":4}}`),
		[]byte(`{"delays_seconds":{"1":1,"2":2,"3":3,"4":4,"5":5,"6":5,"7":5,"8":6,"9":6,"10":6,"11":7,"12":7,"13":7}}`),
	} {
		if _, err := ParseDelays(data); err == nil {
			t.Fatalf("expected invalid config to fail: %s", data)
		}
	}
}
