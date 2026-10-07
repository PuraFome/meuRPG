package leaktest

import (
	"maps"
	"sort"
	"strings"
	"testing"
	"uuid"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// probeRequest builds a request for any procedure, by the names of its fields: every id
// field gets the id of a hidden thing of the fixture (the boss NPC, the hidden trap, the
// puzzle never shown...), every other field stays empty. A call that the master only may
// make must be refused for a player whatever else is in the request; the others are
// asked to answer without a word of what is hidden, so the target of the probe is always
// the hidden one. The request is rarely valid: what the probe proves is the refusal,
// and that no refusal says too much.
func (w *world) probeRequest(p *person, method protoreflect.MethodDescriptor) *dynamicpb.Message {
	m := dynamicpb.NewMessage(method.Input())
	byName := map[protoreflect.Name]string{
		"campaign_id":       w.campaign,
		"map_id":            w.fogMap,
		"point_id":          w.pts["trap-hidden"].GetId(),
		"character_id":      w.boss.GetId(),
		"encounter_id":      w.encounter.GetId(),
		"combatant_id":      w.combatant(w.boss).GetId(),
		"attacker_id":       w.combatant(w.pens).GetId(),
		"target_id":         w.combatant(w.boss).GetId(),
		"puzzle_id":         w.puzzles["riddle-hidden"].GetId(),
		"image_id":          w.imgUnshown,
		"generation_id":     w.generation,
		"game_session_id":   w.session,
		"clue_id":           w.clueUnrevealed,
		"map_point_id":      w.pts["battle"].GetId(),
		"idempotency_key":   uuid.New().String(),
		"token":             "x",
		"as_character_id":   w.boss.GetId(),
		"creature_id":       w.creatureNotOf(p),
		"note_id":           w.notes[otherName(p)], // a note that is not the caller's
		"milestone_id":      w.milestoneUnreached,
		"key":               w.keyOf("race-archived"),
		"scene_point_id":    w.pts["scene-closed"].GetId(),
		"portrait_image_id": w.imgPortrait,
	}
	for i := range method.Input().Fields().Len() {
		fd := method.Input().Fields().Get(i)
		if v, ok := byName[fd.Name()]; ok && fd.Kind() == protoreflect.StringKind && !fd.IsList() {
			m.Set(fd, protoreflect.ValueOfString(v))
		}
	}
	return m
}

func otherName(p *person) string {
	if p.name == "Ana" {
		return "Caio"
	}
	return "Ana"
}

// creatureNotOf is a creature that is not p's: Ana's, for anyone but Ana (who gets her
// character's id, which is no creature).
func (w *world) creatureNotOf(p *person) string {
	if p == w.ana {
		return w.boss.GetId()
	}
	return w.creature
}

// checkNotReads aims every call that is not a read at what the master hid, as each person
// who is not the master, and checks two things. A masterWrite is refused: it must never
// succeed for a player, a pending member, a stranger or someone with no session, whatever
// else the request says (the .proto comment says "only the master"). And no call, master-only or
// the player's own, answers with a word of what the request aimed at: the refusal of a
// hidden thing says what it says for a thing that does not exist.
//
// The requests are built from the names of the fields (probeRequest) and are rarely valid
// in the rest, so a call that is a player's own usually fails its validation: that is why
// the player's own actions are also scripted (actions_test.go), where the answers are real.
func checkNotReads(t *testing.T, w *world, got *answers) {
	t.Helper()
	anon := w.anonymous()
	people := append(w.players(), anon)
	all := map[string]classified{}
	maps.Copy(all, notReads)
	for _, a := range actions { // the scripted actions are the player's own calls too: probe them at what is hidden
		all[a.procedure] = classified{playerAction, a.why}
	}
	var procedures []string
	for p := range all {
		procedures = append(procedures, p)
	}
	sort.Strings(procedures)
	for _, procedure := range procedures {
		c := all[procedure]
		if c.class == notACampaignCall || c.class == streamed {
			continue
		}
		method := methodOf(t, procedure)
		for _, p := range people {
			r := p.call(procedure, w.probeRequest(p, method))
			got.keep(p, r)
			if c.class == masterWrite && r.ok() {
				t.Errorf("%s: %s got an answer from a master-only call (%s): %s", strings.TrimPrefix(procedure, "/meurpg."), p.name, c.why, shorten(r.body))
			}
			if r.status >= 500 {
				t.Errorf("%s: %s made the server fail (HTTP %d): %s", strings.TrimPrefix(procedure, "/meurpg."), p.name, r.status, shorten(r.body))
			}
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s: %s\n\tanswer: %s", strings.TrimPrefix(procedure, "/meurpg."), p.name, f, shorten(r.body))
			}
		}
	}
}
