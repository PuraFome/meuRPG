package leaktest

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// These tests need no database. They test the checker itself: a leak test that cannot fail
// proves nothing, so each layer of inspect is given an answer with a planted leak and must
// find it, and an answer with none, which it must let through.

func unitWorld() (*world, *person, *person) {
	ana, caio := &person{name: "Ana"}, &person{name: "Caio"}
	s := newSecrets()
	w := &world{secrets: s}
	w.ana, w.caio = ana, caio
	return w, ana, caio
}

func asReply(m proto.Message) reply {
	body, _ := protojson.Marshal(m)
	return reply{status: 200, body: body, msg: m}
}

func TestInspectFindsAMarker(t *testing.T) {
	w, ana, caio := unitWorld()
	secret := w.secrets.marker("trap-name")
	mine := w.secrets.marker("clue", ana)
	r := asReply(&mapsv1.MapPoint{Name: secret + " e " + mine})
	if got := w.inspect(ana, r, nil); len(got) != 1 || !strings.Contains(got[0], "trap-name") {
		t.Errorf("Ana: found %v, want only the trap's name (the clue is hers)", got)
	}
	if got := w.inspect(caio, r, nil); len(got) != 2 {
		t.Errorf("Caio: found %v, want both markers", got)
	}
	if got := w.inspect(caio, r, []string{"trap-name", "clue"}); len(got) != 0 {
		t.Errorf("with both kinds ignored: found %v, want none", got)
	}
}

func TestInspectFindsAnIDInAnErrorBody(t *testing.T) {
	w, ana, _ := unitWorld()
	id := w.secrets.id("hidden-map", "0b8f3a52-7d6e-4f1b-9a52-3c7d1e2f4a60")
	r := reply{status: 404, body: []byte(`{"code":"not_found","message":"map ` + id + ` is hidden"}`)}
	if got := w.inspect(ana, r, nil); len(got) != 1 {
		t.Errorf("found %v, want the id in an error message", got)
	}
}

func TestInspectFindsASecretNumberOnlyInANumericField(t *testing.T) {
	w, ana, _ := unitWorld()
	w.secrets.number("npc-hit-points", 4321)
	w.secrets.number("trap-dc", 29, "dc")
	// 4321 in any number, 29 only in a field named *dc*: an x_bp of 29 is a position.
	got := w.inspect(ana, asReply(&mapsv1.MapPoint{XBp: 29, YBp: 4321}), nil)
	if len(got) != 1 || !strings.Contains(got[0], "y_bp") {
		t.Fatalf("found %v, want y_bp (4321) only: an x_bp of 29 is a position", got)
	}
	if got := w.inspect(ana, asReply(&rulesv1.TrapSaveEffect{Dc: 29}), nil); len(got) != 1 {
		t.Errorf("a save's dc of 29: found %v, want it", got)
	}
	// the text "4321" in a string is nothing: the number is searched as a number
	if got := w.inspect(ana, asReply(&mapsv1.MapPoint{Name: "Rua 4321"}), nil); len(got) != 0 {
		t.Errorf("a text with the digits is not the number: found %v", got)
	}
}

func TestInspectFindsAMasterOnlyField(t *testing.T) {
	w, ana, _ := unitWorld()
	// whatever the value: a renamed secret, a field filled with something no canary knows
	r := asReply(&mapsv1.MapPoint{Hooks: "algo que nenhum canário conhece", Trap: &mapsv1.TrapSpec{AreaSize: 2}})
	got := w.inspect(ana, r, nil)
	if len(got) != 1 || !strings.Contains(got[0], "hooks") {
		t.Errorf("found %v, want the hooks only (the trap's area is the player's)", got)
	}
	// a combatant: an NPC's hit points and armor class are the master's, a player's character's are not
	npc := &playv1.Combatant{Kind: playv1.CombatantKind_COMBATANT_KIND_NPC, HitPointsCurrent: proto.Int32(5), ArmorClass: proto.Int32(12)}
	pc := &playv1.Combatant{Kind: playv1.CombatantKind_COMBATANT_KIND_PLAYER, HitPointsCurrent: proto.Int32(5), Mine: true, DeathFailures: 1}
	if got := w.inspect(ana, asReply(&playv1.Encounter{Combatants: []*playv1.Combatant{npc}}), nil); len(got) != 2 {
		t.Errorf("an NPC: found %v, want hit_points_current and armor_class", got)
	}
	if got := w.inspect(ana, asReply(&playv1.Encounter{Combatants: []*playv1.Combatant{pc}}), nil); len(got) != 0 {
		t.Errorf("the player's own character: found %v, want none", got)
	}
	other := &playv1.Combatant{Kind: playv1.CombatantKind_COMBATANT_KIND_PLAYER, DeathFailures: 2}
	if got := w.inspect(ana, asReply(&playv1.Encounter{Combatants: []*playv1.Combatant{other}}), nil); len(got) != 1 {
		t.Errorf("another player's death saves: found %v, want death_failures", got)
	}
}

// TestMasterOnlyFieldsExist keeps the list of master-only fields honest: a field renamed or
// removed from the .proto files makes its entry name nothing, and the layer would check nothing.
func TestMasterOnlyFieldsExist(t *testing.T) {
	for _, r := range masterOnly {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(r.message)
		if err != nil {
			t.Errorf("%s.%s: no such message (%v)", r.message, r.field, err)
			continue
		}
		md, ok := d.(protoreflect.MessageDescriptor)
		if !ok || md.Fields().ByName(r.field) == nil {
			t.Errorf("%s has no field %s any more: update masteronly_test.go", r.message, r.field)
		}
		if strings.TrimSpace(r.why) == "" {
			t.Errorf("%s.%s has no reason", r.message, r.field)
		}
	}
}
