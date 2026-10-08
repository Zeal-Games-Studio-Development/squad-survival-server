package system

import (
	"math/rand"
	"testing"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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
		character.RegenRate != source.RegenRate || character.CritChance != source.CritChance ||
		character.CritMultiplier != source.CritMultiplier || character.DamageReduction != source.DamageReduction ||
		character.WeaponType != string(source.Weapon.Type) || character.RangeClass != string(source.RangeClass) || character.WeaponId != source.Weapon.ID ||
		character.AttackCount != int32(source.AttackCount) || character.CooldownScale != source.CooldownScale {
		t.Fatalf("unexpected character roster: %+v", character)
	}
}

func TestCharacterRosterFieldNumbers(t *testing.T) {
	descriptor := (&CharacterRoster{}).ProtoReflect().Descriptor()
	if descriptor.ReservedRanges().Len() != 0 || descriptor.ReservedNames().Len() != 0 {
		t.Fatal("character roster still reserves the old damage_ratio field")
	}
	for name, number := range map[string]protoreflect.FieldNumber{
		"weapon_type":      9,
		"range_class":      10,
		"weapon_id":        11,
		"crit_chance":      12,
		"crit_multiplier":  13,
		"damage_reduction": 14,
		"attack_count":     15,
		"cooldown_scale":   16,
	} {
		field := descriptor.Fields().ByName(protoreflect.Name(name))
		if field == nil || field.Number() != number {
			t.Fatalf("%s must use field number %d, got %v", name, number, field)
		}
	}
}
