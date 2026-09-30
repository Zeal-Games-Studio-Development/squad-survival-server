package characterbox

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"squad-survival-be/modules/game/core/strategy"
)

type Claim struct {
	BoxID           string
	SessionID       string
	StartedAtTick   int64
	CompletesAtTick int64
}

type rawConfig struct {
	DelaysSeconds map[string]int64 `json:"delays_seconds"`
}

//go:embed pickup_delays.json
var defaultConfigJSON []byte

func ParseDelays(data []byte) (map[int]int64, error) {
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.DelaysSeconds == nil {
		return nil, errors.New("character box pickup delays are required")
	}
	delays := make(map[int]int64, strategy.MaxCharacters-1)
	for count := 1; count < strategy.MaxCharacters; count++ {
		seconds, ok := raw.DelaysSeconds[strconv.Itoa(count)]
		if !ok {
			return nil, fmt.Errorf("character box pickup delay for %d characters is required", count)
		}
		if seconds <= 0 {
			return nil, fmt.Errorf("character box pickup delay for %d characters must be positive", count)
		}
		delays[count] = seconds
	}
	if len(raw.DelaysSeconds) != len(delays) {
		return nil, errors.New("character box pickup delays contain unsupported character counts")
	}
	return delays, nil
}

func DefaultDelays() map[int]int64 {
	delays, err := ParseDelays(defaultConfigJSON)
	if err != nil {
		panic("invalid embedded character box pickup delays: " + err.Error())
	}
	return delays
}

func DelayTicks(delays map[int]int64, characterCount, tickRate int) (int64, bool) {
	seconds, ok := delays[characterCount]
	if !ok || seconds <= 0 || tickRate <= 0 {
		return 0, false
	}
	return seconds * int64(tickRate), true
}
