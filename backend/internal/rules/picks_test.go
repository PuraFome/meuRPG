package rules

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// resilientEffects is the shape of Resilient: the player picks an ability, which rises
// by 1 and whose saving throw the feat makes a proficiency; a note shows a DC from it.
func resilientEffects() []Effect {
	return []Effect{
		{Type: "choice", Choice: "ability", Name: "ability", Count: 1},
		{Type: "modifier", Target: "score.@ability", Mode: "add", Value: "1", Cap: 20},
		{Type: "proficiency", Proficiency: "save.@ability"},
		{Type: "note", Value: `8 + prof() + mod("@ability")`, TextPT: "CD {n}."},
	}
}

func pickFeat(key string, effects ...Effect) Overlay { return featWith(key, effects...) }

func TestAFeatMayNameItsPicksAndReadThem(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	const key = "feat:resistente" + tableSuffix
	c, err := srd.With(pickFeat(key, resilientEffects()...))
	if err != nil {
		t.Fatalf("a Resilient-shaped feat: %v", err)
	}
	// The compiled effects keep the tokens and know the picks that are theirs.
	got := c.c.effects[key]
	if len(got) != 4 || got[1].Target != "score.@ability" || got[2].Proficiency != "save.@ability" || got[3].Value != `8 + prof() + mod("@ability")` {
		t.Fatalf("effects = %+v", got)
	}
	if got[1].picks["ability"] != "ability" || got[0].Name != "ability" {
		t.Errorf("picks = %v, name = %q", got[1].picks, got[0].Name)
	}

	typed := []Effect{
		{Type: "choice", Choice: "damage_type", Name: "tipo", Count: 1, From: []string{"damage-type:fire", "damage-type:cold"}},
		{Type: "resistance", Kind: "resist", DamageTypes: []string{"damage-type:@tipo"}},
		{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "1", Weapon: ExtraDamageAnyWeapon, Frequency: ExtraDamageEachHit, DamageType: "damage-type:@tipo"},
	}
	if _, err := srd.With(pickFeat("feat:tipado"+tableSuffix, typed...)); err != nil {
		t.Errorf("a damage type pick: %v", err)
	}
}

func TestPickTokenRefusals(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	named := Effect{Type: "choice", Choice: "ability", Name: "ability", Count: 1}
	prof := Effect{Type: "proficiency", Proficiency: "save.@ability"}
	cases := []struct {
		name    string
		effects []Effect
		field   string
		reason  string
	}{
		{"unknown name", []Effect{named, {Type: "proficiency", Proficiency: "save.@outra"}}, "feats[0].effects[1].proficiency", ReasonReference},
		{"declared later", []Effect{prof, named}, "feats[0].effects[0].proficiency", ReasonReference},
		{"wrong kind: ability pick as a damage type", []Effect{named, {Type: "resistance", Kind: "resist", DamageTypes: []string{"damage-type:@ability"}}}, "feats[0].effects[1].damage_types", ReasonValue},
		{"wrong kind: damage type pick as an ability", []Effect{{Type: "choice", Choice: "damage_type", Name: "tipo", Count: 1}, {Type: "proficiency", Proficiency: "save.@tipo"}}, "feats[0].effects[1].proficiency", ReasonValue},
		{"wrong kind in a formula", []Effect{{Type: "choice", Choice: "damage_type", Name: "tipo", Count: 1}, {Type: "note", Value: `mod("@tipo")`, TextPT: "x {n}"}}, "feats[0].effects[1].value", ReasonValue},
		{"a name with bad characters", []Effect{{Type: "choice", Choice: "ability", Name: "Habilidade!", Count: 1}}, "feats[0].effects[0].name", ReasonValue},
		{"the table suffix as a name", []Effect{{Type: "choice", Choice: "ability", Name: "mesa", Count: 1}}, "feats[0].effects[0].name", ReasonValue},
		{"a duplicate name", []Effect{named, named}, "feats[0].effects[1].name", ReasonValue},
		{"a token the formula compiler does not take", []Effect{named, {Type: "note", Value: `level() + 0 * mod("@ability") + score("@outra")`, TextPT: "x {n}"}}, "feats[0].effects[1].value", ReasonReference},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := srd.With(pickFeat("feat:teste"+tableSuffix, tc.effects...))
			if err == nil {
				t.Fatal("the feat was accepted")
			}
			violationAt(t, err, tc.field, tc.reason)
		})
	}
}

// A pick belongs to the feat that names it: another entry's effects cannot read it, and a
// class feature cannot name one.
func TestPicksBelongToTheirFeat(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := pickFeat("feat:dono"+tableSuffix, Effect{Type: "choice", Choice: "ability", Name: "ability", Count: 1})
	o.Feats = append(o.Feats, TableFeat{
		TableEntry: TableEntry{Key: "feat:outro" + tableSuffix, NamePT: "Outro"}, DescPT: []string{"Texto."},
		Effects: []Effect{{Type: "proficiency", Proficiency: "save.@ability"}},
	})
	o.Strict = nil
	_, err := srd.With(o)
	violationAt(t, err, "feats[1].effects[0].proficiency", ReasonReference)

	// A class feature: a token, and a name, are refused.
	for field, effects := range map[string][]Effect{
		"classes[0].levels[0].features[0].effects[0].proficiency": {{Type: "proficiency", Proficiency: "save.@ability"}},
		"classes[0].levels[0].features[0].effects[0].name":        {{Type: "choice", Choice: "ability", Name: "ability", Count: 1}},
	} {
		cl := classWithEffects(nil)
		cl.Levels[0].Features = []TableFeature{{Key: "feature:livre" + tableSuffix, NamePT: "Livre", DescPT: []string{"Texto."}, Effects: effects}}
		_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{cl}})
		violationAt(t, err, field, ReasonValue)
	}
}

func TestPickChoiceKinds(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cases := []struct {
		name   string
		choice Effect
		field  string // empty: accepted
		reason string
	}{
		{"every ability", Effect{Choice: "ability", Count: 1}, "", ""},
		{"some abilities", Effect{Choice: "ability", Count: 1, From: []string{"str", "dex"}}, "", ""},
		{"an unknown ability", Effect{Choice: "ability", Count: 1, From: []string{"luck"}}, "feats[0].effects[0].from", ReasonValue},
		{"an ability twice", Effect{Choice: "ability", Count: 1, From: []string{"str", "str"}}, "feats[0].effects[0].from", ReasonValue},
		{"more abilities than listed", Effect{Choice: "ability", Count: 3, From: []string{"str", "dex"}}, "feats[0].effects[0].count", ReasonValue},
		{"every damage type", Effect{Choice: "damage_type", Count: 1}, "", ""},
		{"a damage type", Effect{Choice: "damage_type", Count: 1, From: []string{"damage-type:fire"}}, "", ""},
		{"a key that is no damage type", Effect{Choice: "damage_type", Count: 1, From: []string{"condition:blinded"}}, "feats[0].effects[0].from", ReasonReference},
		{"no name", Effect{Choice: "ability", Count: 1, Name: " "}, "feats[0].effects[0].name", ReasonValue},
		{"a named choice of two", Effect{Choice: "ability", Count: 2}, "feats[0].effects[0].count", ReasonValue},
		{"a count of zero", Effect{Choice: "ability"}, "feats[0].effects[0].count", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := tc.choice
			e.Type = "choice"
			if e.Name == "" && tc.field != "feats[0].effects[0].name" {
				e.Name = "escolha"
			}
			_, err := srd.With(pickFeat("feat:escolha"+tableSuffix, e))
			if tc.field == "" {
				if err != nil {
					t.Fatalf("With() error = %v", err)
				}
				return
			}
			violationAt(t, err, tc.field, tc.reason)
		})
	}
}

// featWithOptions is a feat with n options and the choice of features that offers them.
func featWithOptions(n int, effects ...Effect) Overlay {
	o := featWith("feat:opcoes"+tableSuffix, effects...)
	for i := range n {
		o.Feats[0].Options = append(o.Feats[0].Options, TableFeature{
			Key: "feature:opcao-" + strconv.Itoa(i) + tableSuffix, NamePT: fmt.Sprintf("Opção %d", i), DescPT: []string{"Texto."},
			Effects: []Effect{{Type: "note", TextPT: "Vale quando escolhida."}},
		})
	}
	return o
}

func TestAFeatOffersOptions(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	offer := Effect{Type: "choice", Choice: "feature", Count: 2}
	c, err := srd.With(featWithOptions(MaxFeatureOptions, offer))
	if err != nil {
		t.Fatalf("%d options: %v", MaxFeatureOptions, err)
	}
	const key = "feat:opcoes" + tableSuffix
	feat := c.c.feats[key]
	if len(feat.Options) != MaxFeatureOptions || feat.OptionsChoose != 2 || feat.Options[0] != "feature:opcao-0"+tableSuffix {
		t.Errorf("the feat offers %d options (choose %d)", len(feat.Options), feat.OptionsChoose)
	}
	if got := c.c.offeredBy["feature:opcao-3"+tableSuffix]; len(got) != 1 || got[0] != key {
		t.Errorf("offeredBy = %v", got)
	}
	if opt := c.c.features["feature:opcao-3"+tableSuffix]; opt == nil || opt.Parent != key {
		t.Errorf("option = %+v", opt)
	}

	_, err = srd.With(featWithOptions(MaxFeatureOptions+1, offer))
	violationAt(t, err, "feats[0].options", ReasonLimit)

	// Options need the choice that offers them, and may not nest.
	_, err = srd.With(featWithOptions(2))
	violationAt(t, err, "feats[0].effects", ReasonEffect)
	nested := featWithOptions(2, offer)
	nested.Feats[0].Options[0].Options = []TableFeature{{Key: "feature:fundo" + tableSuffix, NamePT: "Fundo"}}
	_, err = srd.With(nested)
	violationAt(t, err, "feats[0].options[0].options", ReasonValue)
}

func TestAChoiceMayOfferAnOptionSet(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	const key = "feat:estilo" + tableSuffix
	c, err := srd.With(featWith(key, Effect{Type: "choice", Choice: "feature", Count: 1, Set: "fighting_style"}))
	if err != nil {
		t.Fatalf("a set: %v", err)
	}
	if f := c.c.feats[key]; f.OptionSet != "fighting_style" || len(f.Options) != 0 || f.OptionsChoose != 1 {
		t.Errorf("feat = %+v", f)
	}
	for name, tc := range map[string]struct {
		e      Effect
		field  string
		reason string
	}{
		"an unknown set":          {Effect{Choice: "feature", Count: 1, Set: "nao_existe"}, "feats[0].effects[0].set", ReasonValue},
		"a set and a list":        {Effect{Choice: "feature", Count: 1, Set: "fighting_style", From: []string{"feature:fighter-fighting-style-defense"}}, "feats[0].effects[0].set", ReasonValue},
		"a set of the wrong kind": {Effect{Choice: "skill", Count: 1, Set: "fighting_style"}, "feats[0].effects[0].set", ReasonValue},
		"the table feature set":   {Effect{Choice: "feature", Count: 1, Set: "table_feature"}, "feats[0].effects[0].set", ReasonValue},
	} {
		e := tc.e
		e.Type = "choice"
		_, err := srd.With(featWith("feat:erro"+tableSuffix, e))
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		violationAt(t, err, tc.field, tc.reason)
	}
}

func TestSubstitutePick(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ field, name, local, want string }{
		{"save.@ability", "ability", "dex", "save.dex"},
		{"score.@ability", "ability", "con", "score.con"},
		{"damage-type:@type", "type", "fire", "damage-type:fire"},
		{"@a and @a", "a", "x", "x and x"},
		{"save.@ability_two", "ability", "dex", "save.@ability_two"}, // another name
		{"save.@other", "ability", "dex", "save.@other"},
		{"proficiency:x@mesa", "mesa", "dex", "proficiency:x@mesa"}, // the table suffix is no token
		{"str", "ability", "dex", "str"},
		{"", "ability", "dex", ""},
	} {
		if got := substitutePick(tc.field, tc.name, tc.local); got != tc.want {
			t.Errorf("substitutePick(%q, %q, %q) = %q, want %q", tc.field, tc.name, tc.local, got, tc.want)
		}
	}
	for pick, want := range map[string]string{"ability:dex": "dex", "dex": "dex", "damage-type:fire": "fire", "": ""} {
		if got := pickLocal(pick); got != want {
			t.Errorf("pickLocal(%q) = %q, want %q", pick, got, want)
		}
	}
}

// Every key field and formula field that takes a token is accepted through With.
func TestPickTokensInEveryField(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	ability := Effect{Type: "choice", Choice: "ability", Name: "ab", Count: 1}
	damage := Effect{Type: "choice", Choice: "damage_type", Name: "tp", Count: 1}
	cases := map[string][]Effect{
		"when":           {ability, {Type: "sense", Sense: "darkvision", RangeFt: 30, When: `mod("@ab") >= 2 and score("@ab") > 10`}},
		"except":         {damage, {Type: "resistance", Kind: "resist", Except: "damage-type:@tp"}},
		"damage_types":   {damage, {Type: "resistance", Kind: "resist", DamageTypes: []string{"damage-type:@tp"}}},
		"damage_type":    {damage, {Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "1", Weapon: ExtraDamageAnyWeapon, Frequency: ExtraDamageEachHit, DamageType: "damage-type:@tp"}},
		"attack_ability": {ability, {Type: "weapon_rule", Weapon: WeaponAny, AttackAbility: "@ab"}},
		"damage_ability": {ability, {Type: "weapon_rule", Weapon: WeaponAny, DamageAbility: "@ab"}},
		"ability":        {ability, {Type: "grant_action", Economy: "action", Heal: "1d4", Ability: "@ab"}},
		"targets":        {ability, {Type: "roll_mode", Roll: "advantage", Targets: []string{"save.@ab"}}},
		"save modifier":  {ability, {Type: "modifier", Target: "save.@ab", Mode: "add", Value: "mod(\"@ab\")"}},
	}
	for name, effects := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := srd.With(pickFeat("feat:campo"+tableSuffix, effects...)); err != nil {
				t.Errorf("With() error = %v", err)
			}
		})
	}
}

// An option of a feat is not the feat: its effects cannot read the feat's picks.
func TestAFeatOptionCannotReadPicks(t *testing.T) {
	t.Parallel()
	o := featWithOptions(1, Effect{Type: "choice", Choice: "ability", Name: "ab", Count: 1}, Effect{Type: "choice", Choice: "feature", Count: 1})
	o.Feats[0].Options[0].Effects = []Effect{{Type: "proficiency", Proficiency: "save.@ab"}}
	_, err := loadForTest(t).With(o)
	violationAt(t, err, "feats[0].options[0].effects[0].proficiency", ReasonValue)
}

// A table key ends in "@mesa", which is not a token, and a pick cannot be named that.
func TestTheTableSuffixIsNoToken(t *testing.T) {
	t.Parallel()
	if got := pickNames("proficiency:x" + tableSuffix); len(got) != 0 {
		t.Errorf("pickNames = %v", got)
	}
	_, err := loadForTest(t).With(pickFeat("feat:chave"+tableSuffix, Effect{Type: "proficiency", Proficiency: "proficiency:x" + tableSuffix}))
	if err == nil {
		t.Fatal("an unknown proficiency was accepted")
	}
	if msg := violationText(err); strings.Contains(msg, "earlier in this feat") {
		t.Errorf("the table suffix was read as a token: %s", msg)
	}
}

// Checking an effect never writes into the lists the caller gave.
func TestSamplingPicksLeavesTheInputAlone(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	damage := Effect{Type: "choice", Choice: "damage_type", Name: "tp", Count: 1}
	res := Effect{Type: "resistance", Kind: "resist", DamageTypes: []string{"damage-type:@tp"}}
	roll := Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.@ab"}}
	o := pickFeat("feat:intacto"+tableSuffix, Effect{Type: "choice", Choice: "ability", Name: "ab", Count: 1}, damage, res, roll)
	if _, err := srd.With(o); err != nil {
		t.Fatalf("With() error = %v", err)
	}
	if got := o.Feats[0].Effects[2].DamageTypes[0]; got != "damage-type:@tp" {
		t.Errorf("DamageTypes = %q", got)
	}
	if got := o.Feats[0].Effects[3].Targets[0]; got != "save.@ab" {
		t.Errorf("Targets = %q", got)
	}
}

// Only a feat picks an ability or a damage type, or offers a set.
func TestOnlyAFeatPicksAndOffersSets(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	for name, tc := range map[string]struct {
		e     Effect
		field string
	}{
		"ability":     {Effect{Type: "choice", Choice: "ability", Count: 1}, ".choice"},
		"damage_type": {Effect{Type: "choice", Choice: "damage_type", Count: 1}, ".choice"},
		"set":         {Effect{Type: "choice", Choice: "feature", Count: 1, Set: "fighting_style"}, ".set"},
	} {
		cl := classWithEffects(nil)
		cl.Levels[0].Features = []TableFeature{{Key: "feature:livre" + tableSuffix, NamePT: "Livre", DescPT: []string{"Texto."}, Effects: []Effect{tc.e}}}
		_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{cl}})
		if err == nil {
			t.Errorf("%s: a class feature was accepted", name)
			continue
		}
		violationAt(t, err, "classes[0].levels[0].features[0].effects[0]"+tc.field, ReasonValue)
	}
}
