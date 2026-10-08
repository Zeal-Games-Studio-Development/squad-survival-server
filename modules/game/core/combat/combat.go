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

const (
	EventAttackStarted EventType = iota + 1
	EventDamageApplied
	EventCharacterDied
	EventProjectileSpawned
	EventProjectileHit
	EventProjectileExpired
)

type Event struct {
	Type                                      EventType
	AttackID, ProjectileID                    string
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
	ID, AttackID                        string
	AttackerUserID, AttackerCharacterID string
	TargetUserID, TargetCharacterID     string
	WeaponType                          entity.WeaponType
	WeaponID                            string
	Position, Direction                 entity.Vector2
	Speed, Damage                       float64
	Critical                            bool
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
	event  Event
	target *entity.Character
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
	lookup := characterLookup(characters)
	candidates := candidateCharactersBySession(nearbyPlayers)
	intents, events := s.stepProjectiles(lookup, tick)

	for _, attacker := range characters {
		character := attacker.character
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
		if !character.AttackImpacted && tick >= character.AttackImpactTick {
			character.AttackImpacted = true
			if character.RangeClass == entity.RangeRanged {
				projectile, event := s.spawnProjectile(attacker, target, tick, random)
				s.projectiles[projectile.ID] = projectile
				events = append(events, event)
			} else {
				amount, critical := damage.RollAttack(character, random)
				intents = append(intents, newDamageIntent(attacker, target, attackID(character), "", tick, amount, critical))
			}
		}
		if tick >= character.AttackCompleteTick {
			character.ResetAttack()
		}
	}

	sortDamageIntents(intents)
	events = append(events, applyDamage(intents, tick)...)
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
		if !canAttack(attacker, s.config.QueryBuffer) || character.TargetCharacterID != "" {
			continue
		}
		if target, ok := nearestTarget(attacker, candidates[attacker.owner.SessionID]); ok {
			events = append(events, startAttack(attacker, target, tick))
		}
	}
	return events
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
			intents = append(intents, damageIntent{target: target.character, event: Event{
				Type: EventDamageApplied, AttackID: projectile.AttackID, ProjectileID: projectile.ID,
				AttackerUserID: projectile.AttackerUserID, AttackerCharacterID: projectile.AttackerCharacterID,
				TargetUserID: projectile.TargetUserID, TargetCharacterID: projectile.TargetCharacterID,
				Tick: tick, Damage: projectile.Damage, Critical: projectile.Critical,
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

func (s *Simulation) spawnProjectile(attacker, target ownedCharacter, tick int64, random *rand.Rand) (*Projectile, Event) {
	character := attacker.character
	id := attackID(character) + ":projectile"
	amount, critical := damage.RollAttack(character, random)
	projectile := &Projectile{
		ID: id, AttackID: attackID(character),
		AttackerUserID: attacker.owner.UserID, AttackerCharacterID: character.ID,
		TargetUserID: target.owner.UserID, TargetCharacterID: target.character.ID,
		WeaponType: character.Weapon.Type, WeaponID: character.Weapon.ID,
		Position: character.Position, Speed: character.Weapon.ProjectileSpeed,
		Damage: amount, Critical: critical,
	}
	projectile.Direction = directionTo(projectile.Position, target.character.Position)
	return projectile, projectileEvent(EventProjectileSpawned, projectile, tick)
}

func projectileEvent(eventType EventType, projectile *Projectile, tick int64) Event {
	return Event{
		Type: eventType, AttackID: projectile.AttackID, ProjectileID: projectile.ID,
		AttackerUserID: projectile.AttackerUserID, AttackerCharacterID: projectile.AttackerCharacterID,
		TargetUserID: projectile.TargetUserID, TargetCharacterID: projectile.TargetCharacterID,
		WeaponType: projectile.WeaponType, WeaponID: projectile.WeaponID,
		Position: projectile.Position, Direction: projectile.Direction, Speed: projectile.Speed, Tick: tick,
	}
}

func newDamageIntent(attacker, target ownedCharacter, attackID, projectileID string, tick int64, amount float64, critical bool) damageIntent {
	return damageIntent{target: target.character, event: Event{
		Type: EventDamageApplied, AttackID: attackID, ProjectileID: projectileID,
		AttackerUserID: attacker.owner.UserID, AttackerCharacterID: attacker.character.ID,
		TargetUserID: target.owner.UserID, TargetCharacterID: target.character.ID,
		Tick: tick, Damage: amount, Critical: critical,
	}}
}

func sortDamageIntents(intents []damageIntent) {
	sort.Slice(intents, func(i, j int) bool {
		if intents[i].event.AttackerCharacterID == intents[j].event.AttackerCharacterID {
			return intents[i].event.AttackID < intents[j].event.AttackID
		}
		return intents[i].event.AttackerCharacterID < intents[j].event.AttackerCharacterID
	})
}

func applyDamage(intents []damageIntent, tick int64) []Event {
	events := make([]Event, 0, len(intents)*2)
	for _, intent := range intents {
		if intent.target == nil || intent.target.Health <= 0 {
			continue
		}
		var died bool
		intent.event.Damage, intent.event.RemainingHealth, died = damage.Apply(intent.target, intent.event.Damage)
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
	character.TargetUserID, character.TargetCharacterID = target.owner.UserID, target.character.ID
	character.AttackStartTick, character.AttackImpactTick = tick, tick+impactOffset
	character.AttackCompleteTick, character.AttackImpacted = tick+cycleTicks, false
	return Event{
		Type: EventAttackStarted, AttackID: attackID(character),
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
