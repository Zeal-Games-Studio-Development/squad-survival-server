package system

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func TestEncodePlayerRosterBatch(t *testing.T) {
	player := entity.NewPlayer("user-1", "session-1", "Player One", entity.Vector2{}, rand.New(rand.NewSource(1)))
	player.Characters = append(player.Characters, nil)
	data, err := EncodePlayerRosterBatch(12, []*entity.Player{player, nil})
	if err != nil {
		t.Fatal(err)
	}
	var batch PlayerRosterBatch
	if err = proto.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Tick != 12 || len(batch.Players) != 1 {
		t.Fatalf("unexpected roster batch: %+v", &batch)
	}
	roster := batch.Players[0]
	if roster.UserId != player.UserID || roster.SessionId != player.SessionID || roster.DisplayName != player.DisplayName || roster.RosterVersion != player.RosterVersion {
		t.Fatalf("unexpected player roster: %+v", roster)
	}
	if len(roster.Characters) != 1 {
		t.Fatalf("expected nil character to be skipped, got %d", len(roster.Characters))
	}
	character, source := roster.Characters[0], player.Characters[0]
	if character.CharacterId != source.ID || character.Health != source.Health || character.MaxHealth != source.MaxHealth ||
		character.Damage != source.Damage || character.MoveSpeed != source.MoveSpeed ||
		character.AttackSpeed != source.AttackSpeed || character.AttackRange != source.AttackRange ||
		character.RegenRate != source.RegenRate || character.DamageRatio != source.DamageRatio ||
		character.WeaponType != string(source.Weapon.Type) || character.RangeClass != string(source.RangeClass) || character.WeaponName != source.Weapon.Name {
		t.Fatalf("unexpected character roster: %+v", character)
	}
}
