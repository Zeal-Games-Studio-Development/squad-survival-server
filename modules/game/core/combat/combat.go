package combat

import (
	"math"
	"math/rand"
	"sort"
	"strconv"

	"squad-survival-be/modules/game/core/damage"
	"squad-survival-be/modules/game/core/entity"
)

type EventType uint8

const AttackMovementThreshold = 0.2
const BasicAttackHitDelayTicks int64 = 2
const spearLineRadius = 0.6

const (
	EventAttackStarted EventType = iota + 1
	EventDamageApplied
	EventCharacterDied
	EventProjectileSpawned
	EventProjectileHit
	EventProjectileExpired
	EventSkillStarted
)

type Event struct {
	Type                                      EventType
	AttackID, ProjectileID                    string
	HitID                                     string
	SkillID                                   string
	HitIndex                                  int
	Tags                                      []entity.ActionTag
	AttackCount                               int
	AttackerUserID, AttackerCharacterID       string
	TargetUserID, TargetCharacterID           string
	WeaponType                                entity.WeaponType
	WeaponID                                  string
	Position, Direction                       entity.Vector2
	Speed                                     float64
	StartTick, ImpactTick, CompleteTick, Tick int64
	Damage, RemainingHealth                   float64
	Critical                                  bool
}

type Projectile struct {
	ID, AttackID                                        string
	HitID                                               string
	HitIndex                                            int
	Tags                                                []entity.ActionTag
	AttackerUserID, AttackerCharacterID                 string
	TargetUserID, TargetCharacterID                     string
	WeaponType                                          entity.WeaponType
	WeaponID                                            string
	Position, Direction                                 entity.Vector2
	Speed, Damage                                       float64
	BaseDamage, CritChance, CritMultiplier, AttackRange float64
	Origin                                              entity.Vector2
	Critical                                            bool
}

type Simulation struct {
	projectiles map[string]*Projectile
	config      Config
}
type ownedCharacter struct {
	owner     *entity.Player
	character *entity.Character
}
type damageIntent struct {
	event      Event
	target     *entity.Character
	projectile *Projectile
}

func NewSimulation() *Simulation {
	return &Simulation{projectiles: make(map[string]*Projectile), config: DefaultConfig()}
}

func (s *Simulation) Projectiles() []*Projectile {
	if s == nil {
		return nil
	}
	projectiles := make([]*Projectile, 0, len(s.projectiles))
	for _, projectile := range s.projectiles {
		projectiles = append(projectiles, projectile)
	}
	sort.Slice(projectiles, func(i, j int) bool { return projectiles[i].ID < projectiles[j].ID })
	return projectiles
}

func (s *Simulation) Step(players map[string]*entity.Player, nearbyPlayers map[string][]*entity.Player, tick int64, random *rand.Rand) []Event {
	if s == nil {
		return nil
	}
	if s.projectiles == nil {
		s.projectiles = make(map[string]*Projectile)
	}
	if err := s.config.Validate(); err != nil {
		s.config = DefaultConfig()
	}
	if random == nil {
		random = rand.New(rand.NewSource(tick))
	}
	characters := sortedCharacters(players)
	for _, character := range characters {
		if character.character.Health > 0 && (!character.character.CooldownTickSet || character.character.LastCooldownTick != tick) {
			character.character.AdvanceCooldowns(entity.CooldownTicks, "")
			character.character.LastCooldownTick = tick
			character.character.CooldownTickSet = true
		}
	}
	lookup := characterLookup(characters)
	candidates := candidateCharactersBySession(nearbyPlayers)
	intents, events := s.stepProjectiles(lookup, tick)

	for _, attacker := range characters {
		character := attacker.character
		if character.ActiveSkillID != "" {
			if character.Health <= 0 {
				character.ResetSkill()
				continue
			}
			if !character.SkillImpacted && tick >= character.SkillImpactTick {
				if skill, ok := skillByID(character.Weapon.ID, character.ActiveSkillID); ok {
					result := resolveSkillImpact(skill, attacker, candidates[attacker.owner.SessionID], tick, random)
					intents = append(intents, result.damageIntents...)
					events = append(events, result.events...)
				}
				character.SkillImpacted = true
			}
			if tick >= character.SkillCompleteTick {
				character.ResetSkill()
			}
			continue
		}
		if !canAttack(attacker, s.config.QueryBuffer) {
			character.ResetAttack()
			continue
		}
		if character.TargetCharacterID == "" {
			continue
		}
		target, ok := lookup[targetKey(character.TargetUserID, character.TargetCharacterID)]
		if !ok || !isCandidatePlayer(attacker.owner, target.owner, nearbyPlayers) || !validTarget(attacker, target) || !withinRange(character, target.character) {
			character.ResetAttack()
			continue
		}
		if character.AttackHitsEmitted < actionAttackCount(character) && tick >= character.AttackImpactTick+int64(character.AttackHitsEmitted)*BasicAttackHitDelayTicks {
			character.AttackImpacted = true
			if character.RangeClass == entity.RangeRanged {
				hit := character.AttackHitsEmitted + 1
				projectile, event := s.spawnProjectile(attacker, target, tick, hit, random)
				s.projectiles[projectile.ID] = projectile
				events = append(events, event)
				character.AttackHitsEmitted++
			} else if character.Weapon.Type == entity.WeaponSpear {
				for index, candidate := range spearTargets(attacker, candidates[attacker.owner.SessionID]) {
					amount, critical := damage.RollAttackAgainst(character, candidate.character, random)
					if actionAttackCount(character) > 1 {
						amount *= 1.5 * float64(actionAttackCount(character))
					}
					intents = append(intents, newDamageIntent(attacker, candidate, attackID(character), index+1, "", actionTags(character), tick, amount, critical))
				}
				character.AttackHitsEmitted = actionAttackCount(character)
			} else {
				hit := character.AttackHitsEmitted + 1
				amount, critical := damage.RollAttackAgainst(character, target.character, random)
				intents = append(intents, newDamageIntent(attacker, target, attackID(character), hit, "", actionTags(character), tick, amount, critical))
				character.AttackHitsEmitted++
			}
		}
		if tick >= character.AttackCompleteTick && character.AttackHitsEmitted >= actionAttackCount(character) {
			character.ResetAttack()
		}
	}

	sortDamageIntents(intents)
	events = append(events, applyDamage(intents, tick, random)...)
	for _, player := range players {
		player.RemoveDeadCharacters()
	}
	characters = sortedCharacters(players)
	lookup = characterLookup(characters)
	for _, attacker := range characters {
		character := attacker.character
		if character.TargetCharacterID != "" {
			if _, ok := lookup[targetKey(character.TargetUserID, character.TargetCharacterID)]; !ok {
				character.ResetAttack()
			}
		}
		if character.ActiveSkillID != "" || !canAttack(attacker, s.config.QueryBuffer) || character.TargetCharacterID != "" {
			continue
		}
		if event, ok := tryStartSkill(attacker, candidates[attacker.owner.SessionID], tick); ok {
			events = append(events, event)
			continue
		}
		if target, ok := nearestTarget(attacker, candidates[attacker.owner.SessionID]); ok {
			events = append(events, startAttack(attacker, target, tick))
		}
	}
	return events
}

func skillByID(weaponID, skillID string) (entity.SkillDefinition, bool) {
	for _, skill := range entity.SkillsForWeapon(weaponID) {
		if skill.ID == skillID {
			return skill, true
		}
	}
	return entity.SkillDefinition{}, false
}

func skillTargets(skill entity.SkillDefinition, caster ownedCharacter, enemies []ownedCharacter) []ownedCharacter {
	switch skill.Target.Relation {
	case "self":
		return []ownedCharacter{caster}
	case "ally":
		allies := make([]ownedCharacter, 0, len(caster.owner.Characters))
		for _, character := range caster.owner.Characters {
			if character != nil && character != caster.character && character.Health > 0 {
				allies = append(allies, ownedCharacter{owner: caster.owner, character: character})
			}
		}
		sort.Slice(allies, func(i, j int) bool { return targetLess(allies[i], allies[j]) })
		return allies
	default:
		return enemies
	}
}

func nearestSkillTarget(skill entity.SkillDefinition, caster ownedCharacter, enemies []ownedCharacter) (ownedCharacter, bool) {
	var selected ownedCharacter
	best, found := math.Inf(1), false
	maximum := caster.character.AttackRange * skill.Target.RangeScale
	for _, candidate := range skillTargets(skill, caster, enemies) {
		if candidate.character == nil || candidate.character.ID == "" || candidate.character.Health <= 0 {
			continue
		}
		distance := distanceSquared(caster.character.Position, candidate.character.Position)
		if distance > maximum*maximum {
			continue
		}
		if !found || distance < best || distance == best && targetLess(candidate, selected) {
			selected, best, found = candidate, distance, true
		}
	}
	return selected, found
}

func tryStartSkill(caster ownedCharacter, enemies []ownedCharacter, tick int64) (Event, bool) {
	character := caster.character
	for _, skill := range entity.SkillsForWeapon(character.Weapon.ID) {
		if !character.CooldownReady(skill.ID) {
			continue
		}
		target, ok := nearestSkillTarget(skill, caster, enemies)
		if !ok || !character.ResetCooldown(skill.ID) {
			continue
		}
		character.AttackSequence++
		character.ActiveSkillID = skill.ID
		character.SkillStateVersion++
		character.SkillActionID = attackID(character)
		character.SkillDirection = directionTo(character.Position, target.character.Position)
		if character.SkillDirection == (entity.Vector2{}) {
			character.SkillDirection = entity.NormalizeDirection(caster.owner.Facing)
		}
		if character.SkillDirection == (entity.Vector2{}) {
			character.SkillDirection = entity.Vector2{X: 1}
		}
		character.SkillStartTick = tick
		character.SkillImpactTick = tick + skill.Timing.ImpactTicks
		character.SkillCompleteTick = tick + skill.Timing.CompleteTicks
		character.SkillImpacted = false
		tags := []entity.ActionTag{entity.TagSkill}
		if skill.Target.Shape == "cone" {
			tags = append(tags, entity.TagAOE)
		}
		return Event{Type: EventSkillStarted, AttackID: character.SkillActionID, SkillID: skill.ID,
			Tags: tags, AttackerUserID: caster.owner.UserID, AttackerCharacterID: character.ID,
			TargetUserID: target.owner.UserID, TargetCharacterID: target.character.ID,
			WeaponType: character.Weapon.Type, WeaponID: character.Weapon.ID, Direction: character.SkillDirection,
			StartTick: character.SkillStartTick, ImpactTick: character.SkillImpactTick,
			CompleteTick: character.SkillCompleteTick, Tick: tick}, true
	}
	return Event{}, false
}

type skillEffectResult struct {
	damageIntents []damageIntent
	events        []Event
}

type skillEffectHandler func(entity.SkillDefinition, ownedCharacter, []ownedCharacter, int64, *rand.Rand) skillEffectResult

var skillEffectHandlers = map[string]skillEffectHandler{"damage": skillDamageIntents}

func resolveSkillImpact(skill entity.SkillDefinition, caster ownedCharacter, enemies []ownedCharacter, tick int64, random *rand.Rand) skillEffectResult {
	handler := skillEffectHandlers[skill.Effect.Kind]
	if handler == nil {
		return skillEffectResult{}
	}
	return handler(skill, caster, skillTargets(skill, caster, enemies), tick, random)
}

func skillDamageIntents(skill entity.SkillDefinition, caster ownedCharacter, candidates []ownedCharacter, tick int64, random *rand.Rand) skillEffectResult {
	character := caster.character
	maximum := character.AttackRange * skill.Target.RangeScale
	cosHalfAngle := math.Cos(skill.Target.AngleDegrees * math.Pi / 360)
	targets := make([]ownedCharacter, 0)
	for _, candidate := range candidates {
		if candidate.character == nil || candidate.character.ID == "" || candidate.character.Health <= 0 {
			continue
		}
		dx := candidate.character.Position.X - character.Position.X
		dy := candidate.character.Position.Y - character.Position.Y
		distance := math.Hypot(dx, dy)
		if distance > maximum {
			continue
		}
		if skill.Target.Shape == "cone" && distance > 0 && (dx*character.SkillDirection.X+dy*character.SkillDirection.Y)/distance < cosHalfAngle {
			continue
		}
		targets = append(targets, candidate)
	}
	if skill.Target.Shape == "single" && len(targets) > 1 {
		sort.Slice(targets, func(i, j int) bool {
			left := distanceSquared(character.Position, targets[i].character.Position)
			right := distanceSquared(character.Position, targets[j].character.Position)
			return left < right || left == right && targetLess(targets[i], targets[j])
		})
		targets = targets[:1]
	}
	sort.Slice(targets, func(i, j int) bool { return targetLess(targets[i], targets[j]) })
	tags := []entity.ActionTag{entity.TagSkill}
	if skill.Target.Shape == "cone" {
		tags = append(tags, entity.TagAOE)
	}
	intents := make([]damageIntent, 0, len(targets))
	for index, target := range targets {
		amount, critical := character.Damage, false
		if skill.Effect.CanCrit {
			amount, critical = damage.RollAttack(character, random)
		}
		amount *= skill.Effect.DamageMultiplier
		intents = append(intents, newDamageIntent(caster, target, character.SkillActionID, index+1, "", tags, tick, amount, critical))
	}
	return skillEffectResult{damageIntents: intents}
}

func (s *Simulation) stepProjectiles(lookup map[string]ownedCharacter, tick int64) ([]damageIntent, []Event) {
	intents := make([]damageIntent, 0)
	events := make([]Event, 0)
	for _, projectile := range s.Projectiles() {
		target, ok := lookup[targetKey(projectile.TargetUserID, projectile.TargetCharacterID)]
		if !ok || target.character.Health <= 0 {
			events = append(events, projectileEvent(EventProjectileExpired, projectile, tick))
			delete(s.projectiles, projectile.ID)
			continue
		}
		dx := target.character.Position.X - projectile.Position.X
		dy := target.character.Position.Y - projectile.Position.Y
		distance := math.Hypot(dx, dy)
		maximumDistance := projectile.Speed / float64(entity.TickRate)
		if distance == 0 || distance <= maximumDistance {
			projectile.Position = target.character.Position
			events = append(events, projectileEvent(EventProjectileHit, projectile, tick))
			amount, critical := projectile.Damage, projectile.Critical
			if projectile.WeaponType == entity.WeaponBow && projectile.AttackRange > 0 {
				factor := 1 + math.Min(1, math.Hypot(target.character.Position.X-projectile.Origin.X, target.character.Position.Y-projectile.Origin.Y)/projectile.AttackRange)
				amount *= factor
			}
			intents = append(intents, damageIntent{target: target.character, projectile: projectile, event: Event{
				Type: EventDamageApplied, AttackID: projectile.AttackID, ProjectileID: projectile.ID, HitID: projectile.HitID, HitIndex: projectile.HitIndex, Tags: projectile.Tags,
				AttackerUserID: projectile.AttackerUserID, AttackerCharacterID: projectile.AttackerCharacterID,
				TargetUserID: projectile.TargetUserID, TargetCharacterID: projectile.TargetCharacterID,
				Tick: tick, Damage: amount, Critical: critical,
			}})
			delete(s.projectiles, projectile.ID)
			continue
		}
		projectile.Direction = entity.Vector2{X: dx / distance, Y: dy / distance}
		projectile.Position.X += projectile.Direction.X * maximumDistance
		projectile.Position.Y += projectile.Direction.Y * maximumDistance
	}
	return intents, events
}

func (s *Simulation) spawnProjectile(attacker, target ownedCharacter, tick int64, hit int, random *rand.Rand) (*Projectile, Event) {
	character := attacker.character
	id := attackID(character) + ":projectile:" + strconv.Itoa(hit)
	amount, critical := character.Damage, false
	if character.Weapon.Type != entity.WeaponCrossbow {
		amount, critical = damage.RollAttack(character, random)
	}
	projectile := &Projectile{
		ID: id, AttackID: attackID(character), HitID: hitID(attackID(character), hit), HitIndex: hit, Tags: actionTags(character),
		AttackerUserID: attacker.owner.UserID, AttackerCharacterID: character.ID,
		TargetUserID: target.owner.UserID, TargetCharacterID: target.character.ID,
		WeaponType: character.Weapon.Type, WeaponID: character.Weapon.ID,
		Position: character.Position, Origin: character.Position, Speed: character.Weapon.ProjectileSpeed,
		Damage: amount, BaseDamage: character.Damage, CritChance: character.CritChance, CritMultiplier: character.CritMultiplier, AttackRange: character.AttackRange, Critical: critical,
	}
	projectile.Direction = directionTo(projectile.Position, target.character.Position)
	return projectile, projectileEvent(EventProjectileSpawned, projectile, tick)
}

func projectileEvent(eventType EventType, projectile *Projectile, tick int64) Event {
	return Event{
		Type: eventType, AttackID: projectile.AttackID, ProjectileID: projectile.ID, HitID: projectile.HitID, HitIndex: projectile.HitIndex, Tags: projectile.Tags,
		AttackerUserID: projectile.AttackerUserID, AttackerCharacterID: projectile.AttackerCharacterID,
		TargetUserID: projectile.TargetUserID, TargetCharacterID: projectile.TargetCharacterID,
		WeaponType: projectile.WeaponType, WeaponID: projectile.WeaponID,
		Position: projectile.Position, Direction: projectile.Direction, Speed: projectile.Speed, Tick: tick,
	}
}

func newDamageIntent(attacker, target ownedCharacter, attackID string, hitIndex int, projectileID string, tags []entity.ActionTag, tick int64, amount float64, critical bool) damageIntent {
	return damageIntent{target: target.character, event: Event{
		Type: EventDamageApplied, AttackID: attackID, HitID: hitID(attackID, hitIndex), HitIndex: hitIndex, ProjectileID: projectileID, Tags: tags,
		AttackerUserID: attacker.owner.UserID, AttackerCharacterID: attacker.character.ID,
		TargetUserID: target.owner.UserID, TargetCharacterID: target.character.ID,
		Tick: tick, Damage: amount, Critical: critical,
	}}
}

func sortDamageIntents(intents []damageIntent) {
	sort.Slice(intents, func(i, j int) bool {
		if intents[i].event.AttackerCharacterID == intents[j].event.AttackerCharacterID {
			if intents[i].event.AttackID == intents[j].event.AttackID {
				return intents[i].event.HitIndex < intents[j].event.HitIndex
			}
			return intents[i].event.AttackID < intents[j].event.AttackID
		}
		return intents[i].event.AttackerCharacterID < intents[j].event.AttackerCharacterID
	})
}

func applyDamage(intents []damageIntent, tick int64, random *rand.Rand) []Event {
	events := make([]Event, 0, len(intents)*2)
	for _, intent := range intents {
		if intent.target == nil || intent.target.Health <= 0 {
			continue
		}
		if projectile := intent.projectile; projectile != nil && projectile.WeaponType == entity.WeaponCrossbow {
			attacker := &entity.Character{Weapon: entity.Weapon{Type: entity.WeaponCrossbow}, Damage: projectile.BaseDamage, CritChance: projectile.CritChance, CritMultiplier: projectile.CritMultiplier}
			intent.event.Damage, intent.event.Critical = damage.RollAttackAgainst(attacker, intent.target, random)
		}
		var died bool
		intent.event.Damage, intent.event.RemainingHealth, died = damage.ApplyTagged(intent.target, intent.event.Damage, intent.event.Tags)
		if intent.event.Damage > 0 {
			intent.target.AdvanceCooldowns(entity.CooldownDamageReceived, intent.event.AttackID)
		}
		events = append(events, intent.event)
		if died {
			death := intent.event
			death.Type, death.Tick, death.Damage, death.RemainingHealth = EventCharacterDied, tick, 0, 0
			events = append(events, death)
		}
	}
	return events
}

func startAttack(attacker, target ownedCharacter, tick int64) Event {
	character := attacker.character
	cycleTicks := int64(math.Ceil(float64(entity.TickRate) / character.AttackSpeed))
	if cycleTicks < 1 {
		cycleTicks = 1
	}
	impactOffset := int64(math.Ceil(float64(cycleTicks) * character.ImpactRatio))
	if impactOffset < 1 {
		impactOffset = 1
	}
	if impactOffset > cycleTicks {
		impactOffset = cycleTicks
	}
	character.AttackSequence++
	character.AttackActionCount = character.AttackCount
	if character.AttackActionCount < 1 {
		character.AttackActionCount = 1
	}
	character.AttackDirection = directionTo(character.Position, target.character.Position)
	character.AttackHitsEmitted = 0
	character.AdvanceCooldowns(entity.CooldownAttackActions, attackID(character))
	character.TargetUserID, character.TargetCharacterID = target.owner.UserID, target.character.ID
	character.AttackStartTick, character.AttackImpactTick = tick, tick+impactOffset
	character.AttackCompleteTick, character.AttackImpacted = tick+cycleTicks, false
	if character.Weapon.Type != entity.WeaponSpear {
		lastHitTick := character.AttackImpactTick + int64(character.AttackActionCount-1)*BasicAttackHitDelayTicks
		if lastHitTick > character.AttackCompleteTick {
			character.AttackCompleteTick = lastHitTick
		}
	}
	return Event{
		Type: EventAttackStarted, AttackID: attackID(character), Tags: actionTags(character), AttackCount: character.AttackActionCount,
		AttackerUserID: attacker.owner.UserID, AttackerCharacterID: character.ID,
		TargetUserID: target.owner.UserID, TargetCharacterID: target.character.ID,
		WeaponType: character.Weapon.Type, WeaponID: character.Weapon.ID,
		StartTick: character.AttackStartTick, ImpactTick: character.AttackImpactTick,
		CompleteTick: character.AttackCompleteTick, Tick: tick,
	}
}

func nearestTarget(attacker ownedCharacter, characters []ownedCharacter) (ownedCharacter, bool) {
	var selected ownedCharacter
	selectedDistance, found := math.Inf(1), false
	for _, candidate := range characters {
		if !validTarget(attacker, candidate) || !withinRange(attacker.character, candidate.character) {
			continue
		}
		distance := distanceSquared(attacker.character.Position, candidate.character.Position)
		if !found || distance < selectedDistance || distance == selectedDistance && targetLess(candidate, selected) {
			selected, selectedDistance, found = candidate, distance, true
		}
	}
	return selected, found
}

func canAttack(attacker ownedCharacter, queryBuffer float64) bool {
	character := attacker.character
	if character == nil || character.ID == "" || character.Health <= 0 || character.AttackSpeed <= 0 || character.AttackRange < 0 || character.AttackRange+queryBuffer > attacker.owner.DetectionRadius || character.ImpactRatio <= 0 || character.ImpactRatio > 1 {
		return false
	}
	if math.Abs(attacker.owner.Direction.X) >= AttackMovementThreshold || math.Abs(attacker.owner.Direction.Y) >= AttackMovementThreshold {
		return false
	}
	if character.RangeClass == entity.RangeRanged {
		return character.Weapon.ProjectileSpeed > 0
	}
	return character.RangeClass == entity.RangeMelee
}

func candidateCharactersBySession(nearbyPlayers map[string][]*entity.Player) map[string][]ownedCharacter {
	result := make(map[string][]ownedCharacter, len(nearbyPlayers))
	for sessionID, players := range nearbyPlayers {
		characterCount := 0
		for _, player := range players {
			if player != nil {
				characterCount += len(player.Characters)
			}
		}
		characters := make([]ownedCharacter, 0, characterCount)
		for _, player := range players {
			if player == nil || player.SessionID == sessionID {
				continue
			}
			for _, character := range player.Characters {
				if character != nil {
					characters = append(characters, ownedCharacter{owner: player, character: character})
				}
			}
		}
		result[sessionID] = characters
	}
	return result
}

func isCandidatePlayer(attacker, target *entity.Player, nearbyPlayers map[string][]*entity.Player) bool {
	if attacker == nil || target == nil {
		return false
	}
	for _, candidate := range nearbyPlayers[attacker.SessionID] {
		if candidate != nil && candidate.SessionID == target.SessionID {
			return true
		}
	}
	return false
}

func validTarget(attacker, target ownedCharacter) bool {
	return target.character != nil && target.character.ID != "" && target.character.Health > 0 && attacker.owner != target.owner
}
func withinRange(attacker, target *entity.Character) bool {
	return distanceSquared(attacker.Position, target.Position) <= attacker.AttackRange*attacker.AttackRange
}
func directionTo(from, to entity.Vector2) entity.Vector2 {
	dx, dy := to.X-from.X, to.Y-from.Y
	distance := math.Hypot(dx, dy)
	if distance == 0 {
		return entity.Vector2{}
	}
	return entity.Vector2{X: dx / distance, Y: dy / distance}
}
func distanceSquared(a, b entity.Vector2) float64 { dx, dy := b.X-a.X, b.Y-a.Y; return dx*dx + dy*dy }
func targetLess(a, b ownedCharacter) bool {
	if a.owner.UserID == b.owner.UserID {
		return a.character.ID < b.character.ID
	}
	return a.owner.UserID < b.owner.UserID
}

func sortedCharacters(players map[string]*entity.Player) []ownedCharacter {
	owners := make([]*entity.Player, 0, len(players))
	for _, player := range players {
		if player != nil {
			owners = append(owners, player)
		}
	}
	sort.Slice(owners, func(i, j int) bool {
		if owners[i].UserID == owners[j].UserID {
			return owners[i].SessionID < owners[j].SessionID
		}
		return owners[i].UserID < owners[j].UserID
	})
	characters := make([]ownedCharacter, 0)
	for _, owner := range owners {
		for _, character := range owner.Characters {
			if character != nil {
				characters = append(characters, ownedCharacter{owner, character})
			}
		}
	}
	sort.SliceStable(characters, func(i, j int) bool {
		if characters[i].owner.UserID == characters[j].owner.UserID {
			return characters[i].character.ID < characters[j].character.ID
		}
		return characters[i].owner.UserID < characters[j].owner.UserID
	})
	return characters
}
func characterLookup(characters []ownedCharacter) map[string]ownedCharacter {
	lookup := make(map[string]ownedCharacter, len(characters))
	for _, character := range characters {
		lookup[targetKey(character.owner.UserID, character.character.ID)] = character
	}
	return lookup
}
func targetKey(userID, characterID string) string { return userID + "\x00" + characterID }
func attackID(character *entity.Character) string {
	return character.ID + ":" + strconv.FormatUint(character.AttackSequence, 10)
}

func hitID(actionID string, index int) string {
	return actionID + ":hit:" + strconv.Itoa(index)
}

func actionAttackCount(character *entity.Character) int {
	if character.AttackActionCount > 0 {
		return character.AttackActionCount
	}
	return 1
}

func actionTags(character *entity.Character) []entity.ActionTag {
	tags := []entity.ActionTag{entity.TagBasicAttack}
	if character.Weapon.Type == entity.WeaponSpear {
		return append(tags, entity.TagAOE)
	}
	if character.RangeClass == entity.RangeRanged {
		return append(tags, entity.TagProjectile)
	}
	return tags
}

func spearTargets(attacker ownedCharacter, candidates []ownedCharacter) []ownedCharacter {
	character := attacker.character
	direction := character.AttackDirection
	if direction.X == 0 && direction.Y == 0 {
		direction = attacker.owner.Facing
		if direction.X == 0 && direction.Y == 0 {
			direction = entity.Vector2{X: 1}
		}
	}
	result := make([]ownedCharacter, 0)
	for _, candidate := range candidates {
		if !validTarget(attacker, candidate) {
			continue
		}
		dx := candidate.character.Position.X - character.Position.X
		dy := candidate.character.Position.Y - character.Position.Y
		along := dx*direction.X + dy*direction.Y
		across := dx*direction.Y - dy*direction.X
		if along >= 0 && along <= character.AttackRange && math.Abs(across) <= spearLineRadius {
			result = append(result, candidate)
		}
	}
	sort.Slice(result, func(i, j int) bool { return targetLess(result[i], result[j]) })
	return result
}
