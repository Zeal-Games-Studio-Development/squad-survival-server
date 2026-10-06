package system

import (
	"errors"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

type CharacterBoxEvent struct {
	Type              CharacterBoxEventType
	Box               *entity.CharacterBox
	ID                string
	ClaimantSessionID string
	StartedAtTick     int64
	CompletesAtTick   int64
}

func CharacterBoxPickupStarted(id, sessionID string, startedAtTick, completesAtTick int64) CharacterBoxEvent {
	return CharacterBoxEvent{Type: CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_STARTED, ID: id, ClaimantSessionID: sessionID, StartedAtTick: startedAtTick, CompletesAtTick: completesAtTick}
}

func CharacterBoxPickupCancelled(id, sessionID string) CharacterBoxEvent {
	return CharacterBoxEvent{Type: CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_CANCELLED, ID: id, ClaimantSessionID: sessionID}
}

func SpawnedCharacterBox(box *entity.CharacterBox) CharacterBoxEvent {
	return CharacterBoxEvent{Type: CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_SPAWNED, Box: box}
}

func DespawnedCharacterBox(id string) CharacterBoxEvent {
	return CharacterBoxEvent{Type: CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_DESPAWNED, ID: id}
}

func EncodeCharacterBoxStateBatch(tick int64, events []CharacterBoxEvent) ([]byte, error) {
	batch := &CharacterBoxStateBatch{Tick: tick, Events: make([]*CharacterBoxStateEvent, 0, len(events))}
	for _, event := range events {
		snapshot, err := characterBoxStateEventSnapshot(event)
		if err != nil {
			return nil, err
		}
		batch.Events = append(batch.Events, snapshot)
	}
	return proto.Marshal(batch)
}

func characterBoxStateEventSnapshot(event CharacterBoxEvent) (*CharacterBoxStateEvent, error) {
	switch event.Type {
	case CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_SPAWNED:
		if event.Box == nil || event.Box.ID == "" || event.Box.Value == nil || event.Box.Value.WeaponType == "" || event.Box.Value.WeaponId == "" {
			return nil, errors.New("spawned character box is invalid")
		}
		return &CharacterBoxStateEvent{
			EventType: event.Type,
			BoxId:     event.Box.ID,
			Position:  vectorSnapshot(event.Box.Position),
			Value:     &entity.CharacterBoxValue{WeaponType: event.Box.Value.WeaponType, WeaponId: event.Box.Value.WeaponId},
		}, nil
	case CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_DESPAWNED:
		if event.ID == "" {
			return nil, errors.New("despawned character box id is required")
		}
		return &CharacterBoxStateEvent{EventType: event.Type, BoxId: event.ID}, nil
	case CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_STARTED:
		if event.ID == "" || event.ClaimantSessionID == "" || event.StartedAtTick < 0 || event.CompletesAtTick <= event.StartedAtTick {
			return nil, errors.New("character box pickup started event is invalid")
		}
		return &CharacterBoxStateEvent{EventType: event.Type, BoxId: event.ID, ClaimantSessionId: event.ClaimantSessionID, StartedAtTick: event.StartedAtTick, CompletesAtTick: event.CompletesAtTick}, nil
	case CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_CANCELLED:
		if event.ID == "" || event.ClaimantSessionID == "" {
			return nil, errors.New("character box pickup cancelled event is invalid")
		}
		return &CharacterBoxStateEvent{EventType: event.Type, BoxId: event.ID, ClaimantSessionId: event.ClaimantSessionID}, nil
	default:
		return nil, errors.New("unknown character box event type")
	}
}
