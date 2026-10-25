package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A table option for an SRD option set (MR-025): an invented invocation, "Olhar do Corvo", that asks
// for warlock level 5 and the Pact of the Tome. The SRD warlock, the pact and the sets are the SRD's;
// these tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	tomePact    = "feature:pact-of-the-tome"
	invocations = "feature:eldritch-invocations"
)

// crowGaze is the master's option: a darkvision of 60 ft.
func crowGaze() *rulesv1.TableOption {
	return &rulesv1.TableOption{
		NamePt: "Olhar do Corvo", DescPt: []string{"Você enxerga no escuro."}, Set: "invocation",
		Prerequisite: &rulesv1.OptionPrerequisite{
			ClassLevel: &rulesv1.OptionClassLevel{ClassKey: "class:warlock", Level: 5}, RequiresPact: tomePact,
		},
		Effects: []*rulesv1.TableEffect{{Type: "sense", Sense: "darkvision", RangeFt: 60}},
	}
}

// warlockSheet is a valid sheet of a warlock of this level with the pact and the picks given: the
// three cantrips of the class, the Tome's three, six spells known and the invocations.
func warlockSheet(level int32, pact string, picks ...string) *charactersv1.CharacterSheet {
	keys := []string{pact}
	if pact == tomePact {
		for _, s := range []string{"spell:guidance", "spell:shillelagh", "spell:light"} {
			keys = append(keys, tomePact+"#cantrips="+s)
		}
	}
	s := choiceSheet("race:human", "", "class:warlock", "", level, append(keys, picks...)...)
	full := s.GetFull()
	full.CantripKeys = []string{"spell:mage-hand", "spell:chill-touch", "spell:eldritch-blast"}
	full.KnownSpellKeys = []string{"spell:comprehend-languages", "spell:protection-from-evil-and-good", "spell:hold-person", "spell:misty-step", "spell:counterspell", "spell:hellish-rebuke"}
	full.SkillProficiencyKeys = []string{"skill:arcana", "skill:deception"}
	return s
}

func optionOfChoice(res *charactersv1.PreviewChoicesResponse, key string) *charactersv1.ChoiceOption {
	ch := choiceByKey(res, invocations)
	if ch == nil {
		return nil
	}
	for _, o := range ch.GetOptions() {
		if o.GetKey() == key {
			return o
		}
	}
	return nil
}

func TestMR025_ATableOptionIsOfferedOnlyToWhoQualifiesAndAppliesWhenPicked(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	key := tb.master.addEntry(t, tb.campaign, crowGaze()).GetKey()
	if key != "option:invocation:olhar-do-corvo@mesa" {
		t.Fatalf("key = %q, want one that names the set", key)
	}
	invoke := []string{"feature:eldritch-invocation-armor-of-shadows", "feature:eldritch-invocation-devils-sight"}

	// The choice lists it for a warlock of level 5 with the Tome, unblocked; for a lower level or another
	// pact it stays in the list, blocked, with what it lacks.
	for _, tc := range []struct {
		name    string
		sheet   *charactersv1.CharacterSheet
		blocked bool
	}{
		{"level 5 with the Tome", warlockSheet(5, tomePact, invoke...), false},
		{"level 5 with the Chain", warlockSheet(5, "feature:pact-of-the-chain", invoke...), true},
		{"level 3 with the Tome", warlockSheet(3, tomePact, invoke[0]), true},
	} {
		res, err := tb.preview(tb.ana, "", tc.sheet)
		if err != nil {
			t.Fatalf("%s: PreviewChoices() error = %v", tc.name, err)
		}
		o := optionOfChoice(res, key)
		if o == nil || o.GetNamePt() != "Olhar do Corvo" || (o.GetReasonPt() != "") != tc.blocked {
			t.Errorf("%s: option = %v, want it listed, blocked = %v", tc.name, o, tc.blocked)
		}
	}

	// A pick it does not qualify for is refused, as an invocation's own prerequisite is.
	_, err := tb.create(tb.ana, warlockSheet(3, tomePact, key, invoke[0]))
	if r := choiceRefusal(t, "the option below its level", err); r.GetReason() != charactersv1.ChoiceRefusalReason_CHOICE_REFUSAL_REASON_PREREQUISITE_UNMET ||
		!slices.Contains(issueKeys(r), invocations+"|"+key) {
		t.Errorf("refusal = %v, want the unmet prerequisite of the option", r)
	}

	// At level 5 with the Tome it is saved, and its effect is on the sheet.
	c, err := tb.create(tb.ana, warlockSheet(5, tomePact, key, invoke[0], invoke[1]))
	if err != nil {
		t.Fatalf("a warlock 5 with the Tome and the option: %v", err)
	}
	d := tb.ana.get(t, tb.campaign, c.GetId()).GetDerived()
	if i := slices.IndexFunc(d.GetSenses(), func(s *rulesv1.Sense) bool { return s.GetKey() == "darkvision" && s.GetRangeFt() == 60 }); i < 0 {
		t.Errorf("senses = %v, want the 60 ft darkvision of the option", d.GetSenses())
	}
	if i := slices.IndexFunc(d.GetFeatures(), func(f *rulesv1.Feature) bool { return f.GetKey() == key }); i < 0 {
		t.Errorf("features do not list the option %s", key)
	}
}

func TestMR025_ATableOptionHasItsOwnSwitchForThePlayers(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	key := tb.master.addEntry(t, tb.campaign, crowGaze()).GetKey()
	invoke := []string{"feature:eldritch-invocation-armor-of-shadows", "feature:eldritch-invocation-devils-sight"}
	mine, err := tb.create(tb.ana, warlockSheet(5, tomePact, key, invoke[0], invoke[1]))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	tb.master.setOff(t, tb.campaign, true, key)

	// The master's list has the option with its mark, and says one character uses it.
	list, err := tb.master.table.ListOptionSwitches(t.Context(), connect.NewRequest(&rulesv1.ListOptionSwitchesRequest{CampaignId: tb.campaign}))
	if err != nil {
		t.Fatalf("ListOptionSwitches() error = %v", err)
	}
	var entry *rulesv1.OptionSwitchEntry
	for _, o := range list.Msg.GetOptions() {
		if o.GetKey() == key {
			entry = o
		}
	}
	if entry == nil || !entry.GetOff() || entry.GetKind() != rulesv1.TableContentKind_TABLE_CONTENT_KIND_OPTION || entry.GetCharactersUsing() != 1 {
		t.Fatalf("switch entry = %v, want the option, off, used by one character", entry)
	}
	// A new choice of it is refused for a player, and not offered; the sheet that has it keeps it.
	_, err = tb.create(tb.caio, warlockSheet(5, tomePact, key, invoke[0], invoke[1]))
	if b := charBlocked(t, err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_SWITCHED_OFF_CONTENT || b.GetContentKey() != key {
		t.Errorf("a new pick of an off option: %v, want SWITCHED_OFF_CONTENT with the key", b)
	}
	res, err := tb.preview(tb.caio, "", warlockSheet(5, tomePact, invoke...))
	if err != nil {
		t.Fatalf("PreviewChoices() error = %v", err)
	}
	if optionOfChoice(res, key) != nil {
		t.Error("an option that is off is offered to a player")
	}
	if d := tb.ana.get(t, tb.campaign, mine.GetId()).GetDerived(); len(d.GetSenses()) == 0 {
		t.Error("the sheet that had the option lost its effect")
	}
	// The master turns it back on.
	tb.master.setOff(t, tb.campaign, false, key)
	if _, err := tb.create(tb.caio, warlockSheet(5, tomePact, key, invoke[0], invoke[1])); err != nil {
		t.Errorf("a pick of the option once it is on again: %v", err)
	}
}
