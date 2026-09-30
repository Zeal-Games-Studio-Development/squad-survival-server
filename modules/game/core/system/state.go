package system

import "google.golang.org/protobuf/proto"

func EncodeStateSnapshot(tick int64, playerCount int) ([]byte, error) {
	return proto.Marshal(&StateSnapshot{Tick: tick, PlayerCount: int32(playerCount)})
}

func EncodeMatchLifecycleState(phase MatchPhase, serverTick, phaseEndsAtTick int64, tickRate int) ([]byte, error) {
	return proto.Marshal(&MatchLifecycleState{
		Phase: phase, ServerTick: serverTick, PhaseEndsAtTick: phaseEndsAtTick, TickRate: int32(tickRate),
	})
}
