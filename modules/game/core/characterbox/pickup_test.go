package characterbox

import "testing"

func TestDefaultPickupDelays(t *testing.T) {
	delays := DefaultDelays()
	for count := 1; count <= 4; count++ {
		ticks, ok := DelayTicks(delays, count, 10)
		if !ok || ticks != int64(count*10) {
			t.Fatalf("unexpected delay for %d characters: ticks=%d ok=%v", count, ticks, ok)
		}
	}
	if _, ok := DelayTicks(delays, 5, 10); ok {
		t.Fatal("expected a full roster to have no pickup delay")
	}
}

func TestParsePickupDelaysRejectsInvalidConfig(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"delays_seconds":{"1":1,"2":2,"3":3}}`),
		[]byte(`{"delays_seconds":{"1":1,"2":0,"3":3,"4":4}}`),
		[]byte(`{"delays_seconds":{"1":1,"2":2,"3":3,"4":4,"5":5}}`),
	} {
		if _, err := ParseDelays(data); err == nil {
			t.Fatalf("expected invalid config to fail: %s", data)
		}
	}
}
