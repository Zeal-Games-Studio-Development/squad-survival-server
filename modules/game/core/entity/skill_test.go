package entity

import (
	"strings"
	"testing"
)

func TestDefaultSkillCatalogAndWeaponBinding(t *testing.T) {
	skills := DefaultSkillCatalog()
	if len(skills) != 1 || skills[0].ID != "sword_cone" || len(skills[0].WeaponIDs) != 4 {
		t.Fatalf("unexpected default skills: %+v", skills)
	}
	for _, weapon := range DefaultWeaponCatalog() {
		character := NewCharacter()
		character.ApplyWeapon(weapon)
		want := 0
		if weapon.Type == WeaponSword {
			want = 1
		}
		if got := len(character.Cooldowns); got != want || len(SkillsForWeapon(weapon.ID)) != want {
			t.Fatalf("weapon %s has %d cooldowns, want %d", weapon.ID, got, want)
		}
	}
	variant := DefaultWeaponCatalog()[0]
	variant.ID = "sword_variant"
	variant.Type = WeaponSword
	character := NewCharacter()
	character.ApplyWeapon(variant)
	if len(character.Cooldowns) != 0 {
		t.Fatal("skill leaked from weapon type to another weapon ID")
	}
}

func TestParseSkillCatalogRejectsInvalidWeaponBindings(t *testing.T) {
	for _, change := range []struct{ old, replacement string }{
		{`"sword_sturdy"`, `"missing"`},
		{`"sword_sturdy"`, `"sword"`},
		{`"sword_sturdy"`, `""`},
	} {
		data := strings.Replace(string(defaultSkillJSON), change.old, change.replacement, 1)
		if _, err := ParseSkillCatalog([]byte(data), DefaultWeaponCatalog()); err == nil {
			t.Fatalf("accepted invalid weapon binding: %s", data)
		}
	}
}

func TestParseSkillCatalogRejectsInvalidDefinitions(t *testing.T) {
	valid := `{"skills":[{"skill_id":"one","weapon_id":"sword","cooldown":{"mode":"ticks","required":30},"timing":{"impact_ticks":2,"complete_ticks":5},"target":{"relation":"enemy","shape":"cone","angle_degrees":90,"range_scale":1},"effect":{"kind":"damage","damage_multiplier":1,"can_crit":true}}]}`
	for _, mutation := range []struct{ old, replacement string }{
		{`"weapon_id":"sword"`, `"weapon_id":"missing"`},
		{`"mode":"ticks"`, `"mode":"invalid"`},
		{`"required":30`, `"required":0`},
		{`"impact_ticks":2`, `"impact_ticks":6`},
		{`"relation":"enemy"`, `"relation":"team"`},
		{`"shape":"cone"`, `"shape":"circle"`},
		{`"angle_degrees":90`, `"angle_degrees":0`},
		{`"kind":"damage"`, `"kind":"buff"`},
	} {
		data := strings.Replace(valid, mutation.old, mutation.replacement, 1)
		if _, err := ParseSkillCatalog([]byte(data), DefaultWeaponCatalog()); err == nil {
			t.Fatalf("accepted invalid skill: %s", data)
		}
	}
	duplicate := strings.Replace(valid, `]}`, `,`+valid[len(`{"skills":[`):len(valid)-len(`]}`)]+`]}`, 1)
	if _, err := ParseSkillCatalog([]byte(duplicate), DefaultWeaponCatalog()); err == nil {
		t.Fatal("accepted duplicate skill ID")
	}
}
