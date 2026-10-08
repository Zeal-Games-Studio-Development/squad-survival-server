package system

import (
	"testing"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestEncodeCharacterBoxStateBatch(t *testing.T) {
	box := entity.NewCharacterBox("box:1", entity.Vector2{X: 3, Y: -4}, entity.WeaponCrossbow)
	data, err := EncodeCharacterBoxStateBatch(42, []CharacterBoxEvent{
		SpawnedCharacterBox(box),
		CharacterBoxPickupStarted(box.ID, "session-1", 42, 52),
		CharacterBoxPickupCancelled(box.ID, "session-1"),
		DespawnedCharacterBox(box.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	var batch CharacterBoxStateBatch
	if err := proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Tick != 42 || len(batch.Events) != 4 {
		t.Fatalf("unexpected batch: %+v", &batch)
	}
	spawned := batch.Events[0]
	if spawned.EventType != CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_SPAWNED || spawned.BoxId != box.ID || spawned.Position.X != 3 || spawned.Position.Y != -4 || spawned.Value.WeaponType != "crossbow" {
		t.Fatalf("unexpected spawn event: %+v", spawned)
	}
	started := batch.Events[1]
	if started.EventType != CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_STARTED || started.BoxId != box.ID || started.ClaimantSessionId != "session-1" || started.StartedAtTick != 42 || started.CompletesAtTick != 52 {
		t.Fatalf("unexpected pickup started event: %+v", started)
	}
	cancelled := batch.Events[2]
	if cancelled.EventType != CharacterBoxEventType_CHARACTER_BOX_EVENT_TYPE_PICKUP_CANCELLED || cancelled.BoxId != box.ID || cancelled.ClaimantSessionId != "session-1" {
		t.Fatalf("unexpected pickup cancelled event: %+v", cancelled)
	}
	despawned := batch.Events[3]
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
	if _, err := EncodeCharacterBoxStateBatch(1, []CharacterBoxEvent{CharacterBoxPickupStarted("box:1", "", 1, 2)}); err == nil {
		t.Fatal("expected invalid pickup started error")
	}
	if _, err := EncodeCharacterBoxStateBatch(1, []CharacterBoxEvent{CharacterBoxPickupCancelled("", "session-1")}); err == nil {
		t.Fatal("expected invalid pickup cancelled error")
	}
}
