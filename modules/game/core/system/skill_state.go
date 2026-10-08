package system

import (
	"sort"

	"squad-survival-be/modules/game/core/entity"

	"google.golang.org/protobuf/proto"
)

func skillCooldownSnapshots(character *entity.Character) []*SkillCooldown {
	if character == nil || len(character.Cooldowns) == 0 {
		return nil
	}
	ids := make([]string, 0, len(character.Cooldowns))
	for id := range character.Cooldowns {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]*SkillCooldown, 0, len(ids))
	for _, id := range ids {
		cooldown := character.Cooldowns[id]
		if cooldown == nil {
			continue
		}
		state := &SkillCooldown{SkillId: id, Mode: string(cooldown.Mode), Progress: cooldown.Progress, Required: cooldown.Required}
		if character.ActiveSkillID == id {
			state.ActiveActionId = character.SkillActionID
			state.CompleteTick = character.SkillCompleteTick
		}
		result = append(result, state)
	}
	return result
}

func SkillStateSnapshot(player *entity.Player, character *entity.Character) *SkillState {
	if player == nil || character == nil || len(character.Cooldowns) == 0 {
		return nil
	}
	return &SkillState{UserId: player.UserID, CharacterId: character.ID, Version: character.SkillStateVersion, Cooldowns: skillCooldownSnapshots(character)}
}

func EncodeSkillStateBatch(tick int64, states []*SkillState) ([]byte, error) {
	return proto.Marshal(&SkillStateBatch{Tick: tick, States: states})
}

func ChangedSkillStates(players []*entity.Player, seen map[string]uint64) []*SkillState {
	visible := make(map[string]bool)
	states := make([]*SkillState, 0)
	for _, player := range players {
		if player == nil {
			continue
		}
		for _, character := range player.Characters {
			if character == nil || len(character.Cooldowns) == 0 {
				continue
			}
			key := player.UserID + "\x00" + character.ID
			if visible[key] {
				continue
			}
			visible[key] = true
			if version, ok := seen[key]; !ok || version != character.SkillStateVersion {
				states = append(states, SkillStateSnapshot(player, character))
			}
		}
	}
	for key := range seen {
		if !visible[key] {
			delete(seen, key)
		}
	}
	sort.Slice(states, func(i, j int) bool {
		return SkillStateKey(states[i]) < SkillStateKey(states[j])
	})
	return states
}

func SkillStateKey(state *SkillState) string {
	return state.UserId + "\x00" + state.CharacterId
}
