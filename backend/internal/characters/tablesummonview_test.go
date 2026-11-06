package characters

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// fillSummon sets every field of a message to a value that is not the zero one: a string, a number, a flag, the first
// enum value past the unspecified one, a list of one and a nested message.
func fillSummon(t *testing.T, m protoreflect.Message) {
	t.Helper()
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		switch {
		case fd.IsList():
			l := m.Mutable(fd).List()
			switch fd.Kind() {
			case protoreflect.StringKind:
				l.Append(protoreflect.ValueOfString("monster:wolf"))
			default:
				t.Fatalf("field %s: a list of %s is not filled by this test: add it", fd.Name(), fd.Kind())
			}
		case fd.Kind() == protoreflect.StringKind:
			m.Set(fd, protoreflect.ValueOfString("x"))
		case fd.Kind() == protoreflect.Int32Kind:
			m.Set(fd, protoreflect.ValueOfInt32(3))
		case fd.Kind() == protoreflect.BoolKind:
			m.Set(fd, protoreflect.ValueOfBool(true))
		case fd.Kind() == protoreflect.EnumKind:
			m.Set(fd, protoreflect.ValueOfEnum(fd.Enum().Values().Get(1).Number()))
		case fd.Kind() == protoreflect.MessageKind:
			fillSummon(t, m.Mutable(fd).Message())
		default:
			t.Fatalf("field %s of kind %s is not filled by this test: add it", fd.Name(), fd.Kind())
		}
	}
}

// The player's summon view lists what may pass and nothing else: a field the message gets later is hidden until it is
// named in playerSummonFields, and this test fails until the field is either named there or knowingly left hidden.
func TestThePlayersSummonViewListsOnlyTheNamedFields(t *testing.T) {
	t.Parallel()
	m := &rulesv1.TableSummon{}
	fillSummon(t, m.ProtoReflect())
	m.CreatureKeys = []string{"monster:wolf"} // an SRD creature, which stays
	playerView{held: map[string]bool{}}.stripSummon(m)
	fields := m.ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		kept := m.ProtoReflect().Has(fd)
		named := playerSummonFields[string(fd.Name())]
		switch {
		case kept && !named:
			t.Errorf("TableSummon.%s reaches a player and is not named in playerSummonFields: name it there, or clear it in stripSummon", fd.Name())
		case !kept && named:
			t.Errorf("TableSummon.%s is named in playerSummonFields and a player does not read it", fd.Name())
		}
	}
	for name := range playerSummonFields {
		if fields.ByName(protoreflect.Name(name)) == nil {
			t.Errorf("playerSummonFields names %q, which TableSummon has not", name)
		}
	}
}

func TestThePlayersSummonViewKeepsOnlyTheHeldTableCreatures(t *testing.T) {
	t.Parallel()
	const held, unheld = "creature:lobo-de-prata@mesa", "creature:urso-de-ferro@mesa"
	m := &rulesv1.TableSummon{CreatureKeys: []string{"monster:wolf", held, unheld}, Count: "1 + prof()", MaxLive: 4}
	playerView{held: map[string]bool{held: true}}.stripSummon(m)
	if got := m.GetCreatureKeys(); len(got) != 2 || got[0] != "monster:wolf" || got[1] != held || m.GetCount() != "" || m.GetMaxLive() != 0 {
		t.Errorf("summon = %v, want the SRD creature and the held one, with no count and no limit", m)
	}
}
