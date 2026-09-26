package system

import (
	"errors"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

type CharacterBoxEvent struct {
	Type CharacterBoxEventType
	Box  *entity.CharacterBox
	ID   string
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
		if event.Box == nil || event.Box.ID == "" || event.Box.Value == nil || event.Box.Value.WeaponType == "" {
			return nil, errors.New("spawned character box is invalid")
		}
		return &CharacterBoxStateEvent{
			EventType: event.Type,
			BoxId:     event.Box.ID,
			Position:  vectorSnapshot(event.Box.Position),
			Value:     &entity.CharacterBoxValue{WeaponType: event.Box.Value.WeaponType},
		}, nil
	case CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_DESPAWNED:
		if event.ID == "" {
			return nil, errors.New("despawned character box id is required")
		}
		return &CharacterBoxStateEvent{EventType: event.Type, BoxId: event.ID}, nil
	default:
		return nil, errors.New("unknown character box event type")
	}
}
