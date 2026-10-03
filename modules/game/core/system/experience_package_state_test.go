package system

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"squad-survival-be/modules/game/core/entity"
)

func TestEncodeExperiencePackageStateBatch(t *testing.T) {
	item := entity.NewExperiencePackage("xp:1", entity.Vector2{X: 2, Y: 3}, entity.ExperiencePackageTier_EXPERIENCE_PACKAGE_TIER_MEDIUM, 17)
	data, err := EncodeExperiencePackageStateBatch(9, []ExperiencePackageEvent{DetectedExperiencePackage(item), CollectedExperiencePackage(item, &entity.Player{UserID: "u", SessionID: "s"})})
	if err != nil {
		t.Fatal(err)
	}
	var batch ExperiencePackageStateBatch
	if err := proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Tick != 9 || len(batch.Events) != 2 || batch.Events[0].Value.Experience != 17 || batch.Events[1].CollectorSessionId != "s" {
		t.Fatalf("unexpected batch: %+v", &batch)
	}
}
