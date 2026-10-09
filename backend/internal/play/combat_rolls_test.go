package play

import playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"

// rollRPCs are the calls of the attack rolls' modes, the damage extras and the states of
// a combatant. They have their own matrix in this file.
var rollRPCs = []string{"RequestRollMode", "AnswerRollModeRequest", "CancelRollModeRequest", "RemoveDamagePart", "AnswerRageEnd", "EndRage"}

// The two d20 of a roll with advantage or disadvantage, for the tests that type them:
// the face that counts and a die that never does.

func advantage(face int32) func(*playv1.RollAttackRequest) {
	return func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{face, 1} }
}

func disadvantage(face int32) func(*playv1.RollAttackRequest) {
	return func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{face, 20} }
}

func castDisadvantage(face int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.D20Faces = []int32{face, 20} }
}
