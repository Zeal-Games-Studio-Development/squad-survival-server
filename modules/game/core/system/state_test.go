package system

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestStateSnapshotContainsPlayerCount(t *testing.T) {
	data, err := EncodeStateSnapshot(7, 2)
	if err != nil {
		t.Fatal(err)
	}

	var snapshot StateSnapshot
	if err = proto.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Tick != 7 || snapshot.PlayerCount != 2 {
		t.Fatalf("unexpected snapshot: %+v", &snapshot)
	}
}

func TestMatchLifecycleStateContainsPhaseDeadlineAndTickRate(t *testing.T) {
	data, err := EncodeMatchLifecycleState(MatchPhase_MATCH_PHASE_WAITING, 10, 610, 10)
	if err != nil {
		t.Fatal(err)
	}

	var lifecycle MatchLifecycleState
	if err = proto.Unmarshal(data, &lifecycle); err != nil {
		t.Fatal(err)
	}
	if lifecycle.Phase != MatchPhase_MATCH_PHASE_WAITING || lifecycle.ServerTick != 10 || lifecycle.PhaseEndsAtTick != 610 || lifecycle.TickRate != 10 {
		t.Fatalf("unexpected lifecycle state: %+v", &lifecycle)
	}
}
