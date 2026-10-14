package entity

import "google.golang.org/protobuf/proto"

func DecodeMovementInput(data []byte) (*MovementInput, error) {
	input := &MovementInput{}
	err := proto.Unmarshal(data, input)
	return input, err
}

func DecodeActivateSkillInput(data []byte) (*ActivateSkillInput, error) {
	input := &ActivateSkillInput{}
	err := proto.Unmarshal(data, input)
	return input, err
}
