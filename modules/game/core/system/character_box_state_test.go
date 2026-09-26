package system

import (
	"testing"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestEncodeCharacterBoxStateBatch(t *testing.T) {
	box := entity.NewCharacterBox("box:1", entity.Vector2{X: 3, Y: -4}, entity.WeaponWand)
	data, err := EncodeCharacterBoxStateBatch(42, []CharacterBoxEvent{
		SpawnedCharacterBox(box),
		DespawnedCharacterBox(box.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	var batch CharacterBoxStateBatch
	if err := proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Tick != 42 || len(batch.Events) != 2 {
		t.Fatalf("unexpected batch: %+v", &batch)
	}
	spawned := batch.Events[0]
	if spawned.EventType != CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_SPAWNED || spawned.BoxId != box.ID || spawned.Position.X != 3 || spawned.Position.Y != -4 || spawned.Value.WeaponType != "wand" {
		t.Fatalf("unexpected spawn event: %+v", spawned)
	}
	despawned := batch.Events[1]
	if despawned.EventType != CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_DESPAWNED || despawned.BoxId != box.ID || despawned.Position != nil || despawned.Value != nil {
		t.Fatalf("unexpected despawn event: %+v", despawned)
	}
}

func TestEncodeCharacterBoxStateBatchRejectsInvalidEvents(t *testing.T) {
	if _, err := EncodeCharacterBoxStateBatch(1, []CharacterBoxEvent{{}}); err == nil {
		t.Fatal("expected unspecified event error")
	}
	if _, err := EncodeCharacterBoxStateBatch(1, []CharacterBoxEvent{DespawnedCharacterBox("")}); err == nil {
		t.Fatal("expected empty despawn ID error")
	}
}
