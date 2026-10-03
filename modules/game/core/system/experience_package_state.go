package system

import (
	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

type ExperiencePackageEvent struct {
	Type               ExperiencePackageEventType
	Package            *entity.ExperiencePackage
	PackageID          string
	CollectorUserID    string
	CollectorSessionID string
}

func DetectedExperiencePackage(item *entity.ExperiencePackage) ExperiencePackageEvent {
	return ExperiencePackageEvent{Type: ExperiencePackageEventType_EXPERIENCE_PACKAGE_EVENT_TYPE_DETECTED, Package: item, PackageID: item.ID}
}

func LostExperiencePackage(id string) ExperiencePackageEvent {
	return ExperiencePackageEvent{Type: ExperiencePackageEventType_EXPERIENCE_PACKAGE_EVENT_TYPE_LOST, PackageID: id}
}

func CollectedExperiencePackage(item *entity.ExperiencePackage, player *entity.Player) ExperiencePackageEvent {
	return ExperiencePackageEvent{Type: ExperiencePackageEventType_EXPERIENCE_PACKAGE_EVENT_TYPE_COLLECTED, Package: item, PackageID: item.ID, CollectorUserID: player.UserID, CollectorSessionID: player.SessionID}
}

func EncodeExperiencePackageStateBatch(tick int64, events []ExperiencePackageEvent) ([]byte, error) {
	batch := &ExperiencePackageStateBatch{Tick: tick, Events: make([]*ExperiencePackageStateEvent, 0, len(events))}
	for _, event := range events {
		state := &ExperiencePackageStateEvent{EventType: event.Type, PackageId: event.PackageID, CollectorUserId: event.CollectorUserID, CollectorSessionId: event.CollectorSessionID}
		if event.Package != nil {
			state.Position = vectorSnapshot(event.Package.Position)
			state.Value = event.Package.Value
		}
		batch.Events = append(batch.Events, state)
	}
	return proto.Marshal(batch)
}
