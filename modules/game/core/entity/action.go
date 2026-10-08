package entity

type ActionTag string

const (
	TagBasicAttack ActionTag = "basic_attack"
	TagAOE         ActionTag = "aoe"
	TagProjectile  ActionTag = "projectile"
	TagSkill       ActionTag = "skill"
)

func HasActionTag(tags []ActionTag, wanted ActionTag) bool {
	for _, tag := range tags {
		if tag == wanted {
			return true
		}
	}
	return false
}
