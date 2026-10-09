package leaktest

import "google.golang.org/protobuf/reflect/protoreflect"

// The fields a player always reads empty (layer 3 of inspect). Each entry names the
// message and the field, and why: the comment in the .proto file says the same. A field
// is listed here when no value of it is ever a player's; a field that is the player's in
// some cases (their own character's) carries an `unless`. TestMasterOnlyFieldsExist fails
// when a name here is not in the descriptors, so a rename cannot hide a field.

// own is true when the message is the caller's own: `mine` says it in map tokens and in
// combatants.
func own(_ *world, _ *person, m protoreflect.Message) bool {
	fd := m.Descriptor().Fields().ByName("mine")
	return fd != nil && m.Get(fd).Bool()
}

// notAPlayerCharacter is true for a combatant that is an NPC or a creature.
func playerCharacter(_ *world, _ *person, m protoreflect.Message) bool {
	fd := m.Descriptor().Fields().ByName("kind")
	return fd != nil && m.Get(fd).Enum() == 1 // COMBATANT_KIND_PLAYER
}

func ownOrAnyPlayerCharacter(w *world, p *person, m protoreflect.Message) bool {
	return own(w, p, m) || playerCharacter(w, p, m)
}

var masterOnly = []masterOnlyField{
	// --- maps
	{"meurpg.maps.v1.Map", "base_light", "the light of an unlit square is the master's setting", nil},
	{"meurpg.maps.v1.Map", "generated_dungeon", "only the master reads a dungeon's rooms", nil},
	{"meurpg.maps.v1.MapImage", "name", "the gallery is the master's preparation", nil},
	{"meurpg.maps.v1.MapPoint", "hooks", "the master's private text on a scene (RN-20)", nil},
	{"meurpg.maps.v1.MapPoint", "clues", "a clue not revealed is the master's; revealed ones are in the notes", nil},
	{"meurpg.maps.v1.MapPoint", "light", "a player never receives a light point", nil},
	{"meurpg.maps.v1.MapPoint", "trap_revealed_to", "who knows a trap is the master's card", nil},
	{"meurpg.maps.v1.MapPoint", "treasure_converted", "XP bookkeeping", nil},
	{"meurpg.maps.v1.TrapSpec", "preset_key", "RN-10: a player gets a trap's area and state only", nil},
	{"meurpg.maps.v1.TrapSpec", "notice_dc", "RN-10", nil},
	{"meurpg.maps.v1.TrapSpec", "find_dc", "RN-10", nil},
	{"meurpg.maps.v1.TrapSpec", "trigger", "RN-10", nil},
	{"meurpg.maps.v1.TrapSpec", "effect", "RN-10", nil},
	{"meurpg.maps.v1.SceneAction", "dc", "the fixture never turns \"Mostrar a CD\" on (RN-20)", nil},
	{"meurpg.maps.v1.MapToken", "hidden", "a hidden token is not sent to a player at all", nil},
	{"meurpg.maps.v1.MapToken", "carried_light", "only the master and the character's own player", func(w *world, p *person, m protoreflect.Message) bool { return own(w, p, m) }},

	// --- play: the open scene and the stage
	{"meurpg.play.v1.OpenSceneInfo", "hooks", "the master's private text (RN-20)", nil},
	{"meurpg.play.v1.OpenSceneInfo", "clues", "the clues are in the player's notes, not on the scene", nil},
	{"meurpg.play.v1.StageNpc", "character_id", "a player gets an NPC's name and portrait only (RN-20)", nil},
	{"meurpg.play.v1.SceneActionView", "dc", "the fixture never turns \"Mostrar a CD\" on (RN-20)", nil},

	// --- play: the combat (RN-20)
	{"meurpg.play.v1.Encounter", "map_point_id", "the battle point the master started from", nil},
	{"meurpg.play.v1.Combatant", "hidden", "a hidden combatant is not sent to a player", nil},
	{"meurpg.play.v1.Combatant", "tie_unresolved", "the master's bookkeeping", nil},
	{"meurpg.play.v1.Combatant", "armor_class", "a player never learns an armor class", nil},
	{"meurpg.play.v1.Combatant", "xp_value", "what an NPC gives is the master's", nil},
	{"meurpg.play.v1.Combatant", "portrait_url", "a player sees a portrait only on the stage", nil},
	{"meurpg.play.v1.Combatant", "bestiary_creature_key", "which creature an NPC is", nil},
	{"meurpg.play.v1.Combatant", "challenge_rating", "which creature an NPC is", nil},
	{"meurpg.play.v1.Combatant", "character_id", "an NPC's character id is the master's", func(w *world, p *person, m protoreflect.Message) bool { return playerCharacter(w, p, m) }},
	{"meurpg.play.v1.Combatant", "hit_points_current", "an NPC's numbers", func(w *world, p *person, m protoreflect.Message) bool { return ownOrAnyPlayerCharacter(w, p, m) }},
	{"meurpg.play.v1.Combatant", "hit_points_max", "an NPC's numbers", func(w *world, p *person, m protoreflect.Message) bool { return ownOrAnyPlayerCharacter(w, p, m) }},
	{"meurpg.play.v1.Combatant", "hit_points_temporary", "an NPC's numbers", func(w *world, p *person, m protoreflect.Message) bool { return ownOrAnyPlayerCharacter(w, p, m) }},
	{"meurpg.play.v1.Combatant", "initiative_bonus", "only the master and the combatant's player", func(w *world, p *person, m protoreflect.Message) bool { return own(w, p, m) }},
	{"meurpg.play.v1.Combatant", "initiative_face", "only the master and the combatant's player", func(w *world, p *person, m protoreflect.Message) bool { return own(w, p, m) }},
	{"meurpg.play.v1.Combatant", "death_successes", "the table keeps the death saves to the owner and the master (RN-24)", func(w *world, p *person, m protoreflect.Message) bool { return own(w, p, m) }},
	{"meurpg.play.v1.ReactionWindow", "trigger", "the numbers of what happened (total, armor class, damage) are the master's (RN-20)", nil},
	{"meurpg.play.v1.ReactionWindow", "answer_now", "the order the master answers in is his", nil},
	{"meurpg.play.v1.Combatant", "death_failures", "the table keeps the death saves to the owner and the master (RN-24)", func(w *world, p *person, m protoreflect.Message) bool { return own(w, p, m) }},
}
