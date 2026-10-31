package rules

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The table's options for the SRD's option sets. Every name is invented; the sets, the pact and
// the spells are the SRD's.

const (
	crowGaze    = "option:invocation:olhar-do-corvo@mesa"
	farGaze     = "option:invocation:olhar-distante@mesa"
	doubleGuard = "option:fighting_style:guarda-dupla@mesa"
	tome        = "feature:pact-of-the-tome"
	chain       = "feature:pact-of-the-chain"
	invocations = "feature:eldritch-invocations"
	fighting    = "feature:fighter-fighting-style"
)

func crowGazeOption() TableOption {
	return TableOption{
		TableEntry: TableEntry{Key: crowGaze, NamePT: "Olhar do Corvo"}, Set: "invocation", DescPT: []string{"Você enxerga no escuro."},
		Prerequisite: TableOptionPrerequisite{ClassKey: "class:warlock", ClassLevel: 5, RequiresPact: tome},
		Effects:      []Effect{{Type: "sense", Sense: "darkvision", RangeFt: 60}},
	}
}

func doubleGuardOption() TableOption {
	return TableOption{
		TableEntry: TableEntry{Key: doubleGuard, NamePT: "Guarda Dupla"}, Set: "fighting_style", DescPT: []string{"Escudo e arma na mão."},
		Effects: []Effect{{Type: "modifier", Target: "ac", Mode: "add", Value: "1", When: "shield()"}},
	}
}

func optionsOverlay(opts ...TableOption) Overlay { return Overlay{Revision: 1, Options: opts} }

func warlock(level int, picks ...string) Build {
	return choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: level}, picks...)
}

func TestATableOptionJoinsThePoolOfItsSet(t *testing.T) {
	c := withOverlay(t, optionsOverlay(crowGazeOption(), doubleGuardOption()))

	inv := choiceOf(t, c.Choices(warlock(5, tome)), invocations)
	if len(inv.Options) != 33 {
		t.Fatalf("%d invocations listed, want the 32 of the SRD and the table's", len(inv.Options))
	}
	o := optionOf(t, inv, crowGaze)
	if o.NamePT != "Olhar do Corvo" || o.Blocked() || o.SummaryPT != "Você enxerga no escuro." {
		t.Errorf("option = %+v, want the table's invocation, unblocked, with its text", o)
	}
	// The other offerers of the set carry it too, and nothing leaks into another set.
	style := choiceOf(t, c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1})), fighting)
	if !slices.Contains(optionKeys(style), doubleGuard) || slices.Contains(optionKeys(style), crowGaze) {
		t.Errorf("fighting styles = %v, want the table's style and no invocation", optionKeys(style))
	}
	for _, k := range []string{"feature:paladin-fighting-style", "feature:ranger-fighting-style", "feature:additional-fighting-style"} {
		if !slices.Contains(c.c.features[k].Options, doubleGuard) {
			t.Errorf("%s does not offer the table's style", k)
		}
	}
}

func TestATableOptionAsksForItsPrerequisites(t *testing.T) {
	c := withOverlay(t, optionsOverlay(crowGazeOption()))
	cases := []struct {
		name  string
		build Build
		want  string // "" met; else a word of the reason
	}{
		{"qualified", warlock(5, tome), ""},
		{"a lower level", warlock(3, tome), "nível 5 de Bruxo"},
		{"another pact", warlock(5, chain), "Exige o"},
		{"no pact yet", warlock(5), "ainda não escolheu a Dádiva"},
	}
	for _, tc := range cases {
		inv := choiceOf(t, c.Choices(tc.build), invocations)
		o := optionOf(t, inv, crowGaze)
		if tc.want == "" {
			if o.Blocked() {
				t.Errorf("%s: blocked: %q", tc.name, o.ReasonPT)
			}
			continue
		}
		if !o.Blocked() || !strings.Contains(o.ReasonPT, tc.want) {
			t.Errorf("%s: reason = %q (blocked %v), want %q", tc.name, o.ReasonPT, o.Blocked(), tc.want)
		}
	}
}

func TestATableOptionRequiresAnotherOptionAndASpell(t *testing.T) {
	far := TableOption{
		TableEntry: TableEntry{Key: farGaze, NamePT: "Olhar Distante"}, Set: "invocation", DescPT: []string{"Ainda mais longe."},
		Prerequisite: TableOptionPrerequisite{RequiresOptions: []string{crowGaze}, RequiresSpell: "spell:eldritch-blast"},
	}
	c := withOverlay(t, optionsOverlay(crowGazeOption(), far))
	reason := func(b Build) string {
		o := optionOf(t, choiceOf(t, c.Choices(b), invocations), farGaze)
		return o.ReasonPT
	}
	if got := reason(warlock(5, tome)); !strings.Contains(got, "Olhar do Corvo") || !strings.Contains(got, "Rajada Mística") {
		t.Errorf("reason = %q, want both the option and the spell named", got)
	}
	b := warlock(5, tome, crowGaze)
	b.Cantrips = []string{"spell:eldritch-blast"}
	if got := reason(b); got != "" {
		t.Errorf("reason = %q, want none once the option is picked and the cantrip known", got)
	}
	// A spell picked in the known list counts too, and an SRD option is a valid requirement.
	d := TableOption{
		TableEntry: TableEntry{Key: "option:fighting_style:guarda-firme@mesa", NamePT: "Guarda Firme"}, Set: "fighting_style",
		Prerequisite: TableOptionPrerequisite{RequiresOptions: []string{"feature:fighter-fighting-style-defense"}},
	}
	c2 := withOverlay(t, optionsOverlay(d))
	style := choiceOf(t, c2.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1})), fighting)
	if o := optionOf(t, style, d.Key); !o.Blocked() || !strings.Contains(o.ReasonPT, "Defesa") {
		t.Errorf("option = %+v, want blocked until Defense is picked", o)
	}
}

func TestAClassLevelPrerequisiteReadsTheClassItNames(t *testing.T) {
	o := doubleGuardOption()
	o.Prerequisite = TableOptionPrerequisite{ClassKey: "class:fighter", ClassLevel: 4}
	c := withOverlay(t, optionsOverlay(o))
	for level, blocked := range map[int]bool{3: true, 4: false} {
		style := choiceOf(t, c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: level})), fighting)
		if got := optionOf(t, style, doubleGuard).Blocked(); got != blocked {
			t.Errorf("fighter %d: blocked = %v, want %v", level, got, blocked)
		}
	}
}

func TestAPickedTableOptionAppliesItsEffects(t *testing.T) {
	c := withOverlay(t, optionsOverlay(crowGazeOption(), doubleGuardOption()))
	d := Derive(warlock(5, tome, crowGaze), c)
	if i := slices.IndexFunc(d.Senses, func(s Sense) bool { return s.Key == "darkvision" && s.RangeFt == 60 }); i < 0 {
		t.Errorf("senses = %+v, want the darkvision of the option", d.Senses)
	}
	if i := slices.IndexFunc(d.Features, func(f Feature) bool { return f.Key == crowGaze }); i < 0 {
		t.Errorf("features do not list the option")
	}
	for _, is := range d.Issues {
		if strings.Contains(is.Message, "Olhar do Corvo") {
			t.Errorf("issue about the option: %+v", is)
		}
	}
	// The option picked below its level is a problem of the pick, as an invocation's is.
	problems := c.Choices(warlock(3, tome, crowGaze)).ChoiceProblems(false)
	if !slices.ContainsFunc(problems, func(p ChoiceProblem) bool { return p.Code == ChoiceProblemPrerequisite && p.OptionKey == crowGaze }) {
		t.Errorf("problems = %+v, want a prerequisite problem on the option", problems)
	}
	if problems := c.Choices(warlock(5, tome, crowGaze)).ChoiceProblems(false); slices.ContainsFunc(problems, func(p ChoiceProblem) bool { return p.OptionKey == crowGaze }) {
		t.Errorf("problems = %+v, want none for the option once it qualifies", problems)
	}
	// A fighting style gives its effect only with the condition: a shield.
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1}, doubleGuard)
	b.Shield = true
	with := Derive(b, c).ArmorClass
	b.Shield = false
	without := Derive(b, c).ArmorClass
	if with-without != 3 { // the shield is +2 and the style +1
		t.Errorf("AC with a shield %d, without %d, want 3 more with it", with, without)
	}
}

func TestAPoolWithManyTableOptionsIsNotBoundByTheListLimit(t *testing.T) {
	var opts []TableOption
	for i := range 30 {
		o := doubleGuardOption()
		o.Key = fmt.Sprintf("option:fighting_style:estilo-%d@mesa", i)
		o.NamePT = fmt.Sprintf("Estilo %d", i)
		opts = append(opts, o)
	}
	c := withOverlay(t, optionsOverlay(opts...))
	style := choiceOf(t, c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1})), fighting)
	if len(style.Options) != 6+30 {
		t.Errorf("%d options, want the SRD's 6 and the table's 30 (a listed `from` stops at 20 keys)", len(style.Options))
	}
}

func TestAnArchivedOrSwitchedOffOptionIsNotOffered(t *testing.T) {
	o := doubleGuardOption()
	o.Archived = true
	c := withOverlay(t, optionsOverlay(o))
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1})
	if slices.Contains(optionKeys(choiceOf(t, c.Choices(b), fighting)), doubleGuard) {
		t.Error("an archived option is offered as a new choice")
	}
	picked := choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1}, doubleGuard)
	if !slices.Contains(optionKeys(choiceOf(t, c.Choices(picked), fighting)), doubleGuard) {
		t.Error("a sheet that has the option lost it")
	}
	offOverlay := optionsOverlay(doubleGuardOption())
	offOverlay.Off = []string{doubleGuard}
	off := withOverlay(t, offOverlay)
	if slices.Contains(optionKeys(choiceOf(t, off.Choices(b), fighting)), doubleGuard) {
		t.Error("a switched-off option is offered")
	}
	if got := off.Options(); len(got) != 1 || !got[0].Off || got[0].Set != "fighting_style" {
		t.Errorf("Options() = %+v, want the one option, off", got)
	}
}

func TestATableOptionRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cases := []struct {
		name  string
		edit  func(o *TableOption)
		field string
	}{
		{"an unknown set", func(o *TableOption) { o.Set = "estilo" }, ".set"},
		{"a set that is not the key's", func(o *TableOption) { o.Set = "metamagic" }, ".set"},
		{"an unknown class", func(o *TableOption) { o.Prerequisite.ClassKey = "class:nao-existe" }, ".prerequisite.class_level.class_key"},
		{"a level above 20", func(o *TableOption) { o.Prerequisite.ClassLevel = 21 }, ".prerequisite.class_level.level"},
		{"a class with no level", func(o *TableOption) { o.Prerequisite.ClassLevel = 0 }, ".prerequisite.class_level"},
		{"an option that does not exist", func(o *TableOption) { o.Prerequisite.RequiresOptions = []string{"feature:nada"} }, ".prerequisite.requires_option_keys[0]"},
		{"an option that requires itself", func(o *TableOption) { o.Prerequisite.RequiresOptions = []string{crowGaze} }, ".prerequisite.requires_option_keys[0]"},
		{"a pact that is none", func(o *TableOption) { o.Prerequisite.RequiresPact = "feature:fighter-fighting-style-defense" }, ".prerequisite.requires_pact"},
		{"a spell that does not exist", func(o *TableOption) { o.Prerequisite.RequiresSpell = "spell:nao-existe" }, ".prerequisite.requires_spell_key"},
		{"a choice effect", func(o *TableOption) { o.Effects = []Effect{{Type: "choice", Choice: "skill", Count: 1}} }, ".effects[0].type"},
	}
	for _, tc := range cases {
		o := crowGazeOption()
		tc.edit(&o)
		_, err := srd.With(optionsOverlay(o))
		oe, ok := errors.AsType[*OverlayError](err)
		if !ok {
			t.Errorf("%s: err = %v, want an overlay error", tc.name, err)
			continue
		}
		found := false
		for _, v := range oe.Violations() {
			found = found || strings.HasSuffix(v.Field, tc.field)
		}
		if !found {
			t.Errorf("%s: refused at %+v, want the field %s", tc.name, oe.Violations()[0], tc.field)
		}
	}
	// A key that names no set is refused at the key.
	o := crowGazeOption()
	o.Key = "option:estilo:x@mesa"
	if _, err := srd.With(optionsOverlay(o)); err == nil {
		t.Error("a key with an unknown set was accepted")
	}
}

func TestATableOptionOfAPactCanBeRequired(t *testing.T) {
	pact := TableOption{
		TableEntry: TableEntry{Key: "option:pact_boon:pacto-da-lanterna@mesa", NamePT: "Pacto da Lanterna"}, Set: "pact_boon", DescPT: []string{"Uma luz."},
	}
	needs := crowGazeOption()
	needs.Prerequisite.RequiresPact = pact.Key
	c := withOverlay(t, optionsOverlay(pact, needs))
	inv := choiceOf(t, c.Choices(warlock(5, pact.Key)), invocations)
	if o := optionOf(t, inv, crowGaze); o.Blocked() {
		t.Errorf("blocked with the table's pact: %q", o.ReasonPT)
	}
	boons := choiceOf(t, c.Choices(warlock(5)), "feature:pact-boon")
	if !slices.Contains(optionKeys(boons), pact.Key) {
		t.Errorf("the pact boons = %v, want the table's pact too", optionKeys(boons))
	}
}

// TestAWarlockSeesEveryInvocationWhenTheTableAddsItsOwn: the Eldritch Invocations choice lists the SRD's
// and all the table's invocations, past 20 keys.
func TestAWarlockSeesEveryInvocationWhenTheTableAddsItsOwn(t *testing.T) {
	t.Parallel()
	var opts []TableOption
	for i := range 30 {
		o := doubleGuardOption()
		o.Key = fmt.Sprintf("option:invocation:invocacao-%d@mesa", i)
		o.NamePT = fmt.Sprintf("Invocação %d", i)
		o.Set = "invocation"
		opts = append(opts, o)
	}
	c := withOverlay(t, optionsOverlay(opts...))
	srd := choiceOf(t, loadForTest(t).Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 2})), invocationsFeature)
	got := choiceOf(t, c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 2})), invocationsFeature)
	if len(got.Options) != len(srd.Options)+30 || len(got.Options) <= 20 {
		t.Errorf("the warlock sees %d invocations, want the SRD's %d and the table's 30", len(got.Options), len(srd.Options))
	}
	// An invocation past the 20th key is a valid pick, with no problem on the sheet.
	last := "option:invocation:invocacao-29@mesa"
	if problems := c.Choices(warlock(2, "", last)).ChoiceProblems(false); slices.ContainsFunc(problems, func(p ChoiceProblem) bool { return p.OptionKey == last }) {
		t.Errorf("problems = %+v, want none for the 30th table invocation", problems)
	}
}
