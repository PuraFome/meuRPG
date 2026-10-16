package rules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The small loaders of effects/ (spell targets, damage resistances, the
// Portuguese texts, the lights) and three reads of the whole content (the
// effect menu, the table defaults, the spell list), frozen before they are
// split.

// jsonText is v as JSON, or the marshaling error.
func jsonText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// runEffectLoaderCases loads each file of cases into a fresh content and
// records the error, and the state on success.
func runEffectLoaderCases(t *testing.T, golden, path string, cases map[string]string, fresh func() *content, load func(c *content, fsys fstest.MapFS) error, state func(c *content) any) {
	t.Helper()
	got := map[string]string{}
	for name, doc := range cases {
		c := fresh()
		fsys := fstest.MapFS{}
		if doc != "<missing>" {
			fsys[path] = &fstest.MapFile{Data: []byte(doc)}
		}
		err := load(c, fsys)
		text := errText(err)
		if err == nil {
			text += " " + jsonText(state(c))
		}
		if strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = text
	}
	goldenText(t, golden, got)
}

func TestLoadSpellTargetsCharacterization(t *testing.T) {
	t.Parallel()
	one := func(body string) string { return `{"spells":{"spell:a":` + body + `}}` }
	cases := map[string]string{
		"bad: missing":                 "<missing>",
		"bad: not json":                `{"spells":`,
		"bad: unknown field":           `{"spells":{},"x":1}`,
		"bad: unknown entry field":     one(`{"kind":"self","radius":1}`),
		"ok: nothing":                  `{"_comment":"x"}`,
		"bad: unknown spell":           `{"spells":{"spell:nope":{"kind":"self"}}}`,
		"ok: every kind":               `{"spells":{"spell:a":{"kind":"self"},"spell:b":{"kind":"none","label_pt":"Um ponto"},"spell:c":{"kind":"creature","per_slot_level":1},"spell:d":{"kind":"creatures","count":20,"per_slot_level":10},"spell:e":{"kind":"area","shape":"cone","size_ft":15}}}`,
		"ok: creatures 1 and 0 more":   one(`{"kind":"creatures","count":1}`),
		"ok: every area shape":         `{"spells":{"spell:a":{"kind":"area","shape":"cube","size_ft":5},"spell:b":{"kind":"area","shape":"cylinder","size_ft":10},"spell:c":{"kind":"area","shape":"line","size_ft":100},"spell:d":{"kind":"area","shape":"sphere","size_ft":20}}}`,
		"bad: self with a count":       one(`{"kind":"self","count":1}`),
		"bad: self with a shape":       one(`{"kind":"self","shape":"cone"}`),
		"bad: none with a size":        one(`{"kind":"none","size_ft":5}`),
		"bad: none more per slot":      one(`{"kind":"none","per_slot_level":1}`),
		"bad: self more per slot":      one(`{"kind":"self","per_slot_level":1}`),
		"bad: creature with a count":   one(`{"kind":"creature","count":2}`),
		"bad: creatures count 0":       one(`{"kind":"creatures"}`),
		"bad: creatures count 21":      one(`{"kind":"creatures","count":21}`),
		"bad: creatures per slot -1":   one(`{"kind":"creatures","count":1,"per_slot_level":-1}`),
		"bad: creatures per slot 11":   one(`{"kind":"creatures","count":1,"per_slot_level":11}`),
		"bad: creatures with a shape":  one(`{"kind":"creatures","count":1,"shape":"cone"}`),
		"bad: creatures with a size":   one(`{"kind":"creatures","count":1,"size_ft":5}`),
		"bad: area of no shape":        one(`{"kind":"area","size_ft":5}`),
		"bad: area of a bad shape":     one(`{"kind":"area","shape":"star","size_ft":5}`),
		"bad: area of size 0":          one(`{"kind":"area","shape":"cone"}`),
		"bad: area of size 7":          one(`{"kind":"area","shape":"cone","size_ft":7}`),
		"bad: area with a count":       one(`{"kind":"area","shape":"cone","size_ft":5,"count":1}`),
		"bad: area more per slot":      one(`{"kind":"area","shape":"cone","size_ft":5,"per_slot_level":1}`),
		"bad: empty kind":              one(`{}`),
		"bad: unknown kind":            one(`{"kind":"object"}`),
		"ok: label of 80":              one(`{"kind":"self","label_pt":"` + strings.Repeat("é", 80) + `"}`),
		"bad: label of 81":             one(`{"kind":"self","label_pt":"` + strings.Repeat("a", 81) + `"}`),
		"bad: label of two lines":      one(`{"kind":"self","label_pt":"a\nb"}`),
		"bad: label hidden rune":       one(`{"kind":"self","label_pt":"a` + "\u200b" + `b"}`),
		"order: keys in order":         `{"spells":{"spell:b":{"kind":"x"},"spell:a":{"kind":"y"}}}`,
		"order: unknown before kind":   `{"spells":{"spell:nope":{"kind":"x"}}}`,
		"order: kind before label":     one(`{"kind":"self","count":1,"label_pt":"a\nb"}`),
		"order: a good one then a bad": `{"spells":{"spell:a":{"kind":"self"},"spell:b":{"kind":"x"}}}`,
	}
	fresh := func() *content {
		c := &content{spells: map[string]*srd51.Spell{}}
		for _, k := range []string{"a", "b", "c", "d", "e"} {
			c.spells["spell:"+k] = &srd51.Spell{Key: "spell:" + k}
		}
		return c
	}
	runEffectLoaderCases(t, "spelltargets-refusals", "effects/spell_targets.json", cases, fresh,
		func(c *content, fsys fstest.MapFS) error { return c.loadSpellTargets(fsys) },
		func(c *content) any { return c.srdTargets })
}

func TestLoadDamageResistancesCharacterization(t *testing.T) {
	t.Parallel()
	one := func(key, body string) string {
		return `{"source":"SRD","resistances":{"` + key + `":` + body + `}}`
	}
	cases := map[string]string{
		"bad: missing":                   "<missing>",
		"bad: not json":                  `{`,
		"bad: unknown field":             `{"source":"SRD","x":1}`,
		"bad: no source":                 `{"resistances":{}}`,
		"bad: blank source":              `{"source":"  "}`,
		"ok: nothing":                    `{"source":"SRD"}`,
		"ok: a feature and a trait":      `{"source":"SRD","resistances":{"feature:rage":{"damage_types":["damage-type:fire","damage-type:cold"],"while":"rage"},"trait:hot":{"damage_types":["damage-type:fire"]}}}`,
		"bad: a spell key":               one("spell:x", `{"damage_types":["damage-type:fire"]}`),
		"bad: an unknown feature":        one("feature:nope", `{"damage_types":["damage-type:fire"]}`),
		"bad: an unknown trait":          one("trait:nope", `{"damage_types":["damage-type:fire"]}`),
		"bad: no damage type":            one("trait:hot", `{}`),
		"bad: a condition as a type":     one("trait:hot", `{"damage_types":["damage-type:fire","condition:prone"]}`),
		"bad: an unknown damage type":    one("trait:hot", `{"damage_types":["damage-type:fun"]}`),
		"bad: while unknown":             one("trait:hot", `{"damage_types":["damage-type:fire"],"while":"sleep"}`),
		"bad: unknown entry field":       one("trait:hot", `{"damage_types":["damage-type:fire"],"x":1}`),
		"order: key before types":        one("trait:nope", `{"while":"x"}`),
		"order: types before while":      one("trait:hot", `{"damage_types":["damage-type:fun"],"while":"x"}`),
		"order: empty before while":      one("trait:hot", `{"while":"x"}`),
		"order: keys in order":           `{"source":"SRD","resistances":{"trait:hot":{},"feature:rage":{}}}`,
		"order: source before entries":   `{"resistances":{"trait:nope":{}}}`,
		"order: a good one then a bad 1": `{"source":"SRD","resistances":{"feature:rage":{"damage_types":["damage-type:fire"]},"trait:hot":{}}}`,
	}
	fresh := func() *content {
		return &content{
			namesEN: map[string]string{"feature:rage": "Rage", "trait:hot": "Hot", "damage-type:fire": "Fire", "damage-type:cold": "Cold", "condition:prone": "Prone", "spell:x": "X"},
			namesPT: map[string]string{"feature:rage": "Fúria"},
		}
	}
	runEffectLoaderCases(t, "resistances-refusals", "effects/damage_resistances.json", cases, fresh,
		func(c *content, fsys fstest.MapFS) error { return c.loadDamageResistances(fsys) },
		func(c *content) any { return c.resistances })
}

func TestLoadTextsPTCharacterization(t *testing.T) {
	t.Parallel()
	const items = `{"magic-item:ring":{"desc":["Anel."]}}`
	type files struct{ spells, items string }
	cases := map[string]files{
		"bad: no spells file":            {"<missing>", items},
		"bad: no items file":             {`{}`, "<missing>"},
		"bad: spells not json":           {`{`, items},
		"bad: items not json":            {`{}`, `[`},
		"bad: unknown spell field":       {`{"spell:plain":{"desc":["a"],"x":1}}`, items},
		"bad: unknown item field":        {`{}`, `{"magic-item:ring":{"desc":["a"],"x":1}}`},
		"ok: nothing":                    {`{}`, `{}`},
		"ok: everything":                 {`{"spell:plain":{"desc":["Um.","Dois."],"higher_level":["Mais."]},"spell:mat":{"desc":["Pó."],"material":"Um pó."},"spell:gap":{"desc":["A.","",""]}}`, items},
		"bad: unknown spell":             {`{"spell:nope":{"desc":["a"]}}`, items},
		"bad: too few paragraphs":        {`{"spell:plain":{"desc":["Um."],"higher_level":["Mais."]}}`, items},
		"bad: too many paragraphs":       {`{"spell:plain":{"desc":["Um.","Dois.","Três."],"higher_level":["Mais."]}}`, items},
		"bad: empty paragraph":           {`{"spell:plain":{"desc":["Um."," "],"higher_level":["Mais."]}}`, items},
		"bad: higher level missing":      {`{"spell:plain":{"desc":["Um.","Dois."]}}`, items},
		"bad: higher level empty":        {`{"spell:plain":{"desc":["Um.","Dois."],"higher_level":[""]}}`, items},
		"bad: material without one":      {`{"spell:plain":{"desc":["Um.","Dois."],"higher_level":["Mais."],"material":"Pó."}}`, items},
		"bad: material missing":          {`{"spell:mat":{"desc":["Pó."]}}`, items},
		"bad: material blank":            {`{"spell:mat":{"desc":["Pó."],"material":" "}}`, items},
		"bad: unknown item":              {`{}`, `{"magic-item:nope":{"desc":["a"]}}`},
		"bad: item paragraphs":           {`{}`, `{"magic-item:ring":{"desc":[]}}`},
		"bad: item empty paragraph":      {`{}`, `{"magic-item:ring":{"desc":[""]}}`},
		"order: desc before higher":      {`{"spell:plain":{"desc":["Um."]}}`, items},
		"order: higher before material":  {`{"spell:plain":{"desc":["Um.","Dois."],"material":"x"}}`, items},
		"order: count before emptiness":  {`{"spell:plain":{"desc":[""],"higher_level":["Mais."]}}`, items},
		"order: spells before items":     {`{"spell:nope":{}}`, `{"magic-item:nope":{}}`},
		"order: spell keys in order":     {`{"spell:plain":{"desc":[]},"spell:mat":{"desc":[]}}`, items},
		"order: item keys in order":      {`{}`, `{"magic-item:z":{},"magic-item:a":{}}`},
		"order: spells refused, no read": {`{"spell:nope":{}}`, "<missing>"},
	}
	got := map[string]string{}
	for name, f := range cases {
		c := &content{
			spells: map[string]*srd51.Spell{
				"spell:plain": {Desc: []string{"One.", "Two."}, HigherLevel: []string{"More."}},
				"spell:mat":   {Desc: []string{"Dust."}, Material: "A dust."},
				"spell:gap":   {Desc: []string{"A.", "", " "}},
			},
			magicItems: map[string]*srd51.MagicItem{"magic-item:ring": {Desc: []string{"Ring."}}},
		}
		fsys := fstest.MapFS{}
		if f.spells != "<missing>" {
			fsys["effects/spells_pt.json"] = &fstest.MapFile{Data: []byte(f.spells)}
		}
		if f.items != "<missing>" {
			fsys["effects/magic_items_pt.json"] = &fstest.MapFile{Data: []byte(f.items)}
		}
		err := c.loadTextsPT(fsys)
		text := errText(err)
		if err == nil {
			text += " " + jsonText([]any{c.spellTextsPT, c.itemTextsPT})
		}
		if strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = text
	}
	goldenText(t, "textspt-refusals", got)
}

func TestLoadLightsCharacterization(t *testing.T) {
	t.Parallel()
	light := func(key string, bright, dim int, dur string) string {
		return fmt.Sprintf(`{"key":%q,"bright_ft":%d,"dim_ft":%d,"duration_pt":%q}`, key, bright, dim, dur)
	}
	file := func(lights ...string) string { return `{"lights":[` + strings.Join(lights, ",") + `]}` }
	good := light("torch", 20, 20, "1 hora")
	cases := map[string]string{
		"bad: missing":              "<missing>",
		"bad: not json":             `{`,
		"bad: unknown field":        `{"lights":[],"x":1}`,
		"bad: unknown light field":  `{"lights":[{"key":"torch","bright_ft":20,"dim_ft":20,"duration_pt":"x","cone":true}]}`,
		"bad: nothing, a name left": file(),
		"ok: two lights in order":   file(good, light("candle", 0, 5, "1 hora")),
		"ok: only bright":           file(good, light("candle", 5, 0, "x")),
		"bad: empty key":            file(light("", 5, 5, "x")),
		"bad: key with a space":     file(light("a b", 5, 5, "x")),
		"bad: key with a colon":     file(light("light:torch", 5, 5, "x")),
		"bad: key repeated":         file(good, good),
		"bad: bright negative":      file(light("torch", -5, 10, "x")),
		"bad: dim negative":         file(light("torch", 10, -5, "x")),
		"bad: no radius":            file(light("torch", 0, 0, "x")),
		"bad: bright not of 5":      file(light("torch", 7, 5, "x")),
		"bad: dim not of 5":         file(light("torch", 5, 7, "x")),
		"bad: no duration":          file(light("torch", 5, 5, " ")),
		"bad: no Portuguese name":   file(good, light("lamp", 5, 5, "x")),
		"order: key before radius":  file(light("", -5, 0, "")),
		"order: radius before five": file(light("torch", -3, 0, "")),
		"order: five before dur":    file(light("torch", 3, 0, "")),
		"order: dur before name":    file(light("lamp", 5, 0, "")),
	}
	fresh := func() *content {
		return &content{namesPT: map[string]string{"light:torch": "Tocha", "light:candle": "Vela", "spell:x": "X"}}
	}
	runEffectLoaderCases(t, "lights-refusals", "effects/lights.json", cases, fresh,
		func(c *content, fsys fstest.MapFS) error { return c.loadLights(fsys) },
		func(c *content) any { return c.lights })
}

// TestEffectMenuCharacterization freezes the whole menu of the SRD content.
func TestEffectMenuCharacterization(t *testing.T) {
	t.Parallel()
	goldenText(t, "effectmenu", map[string]string{"srd": jsonText(loadForTest(t).EffectMenu())})
}

// TestTableDefaultsCharacterization freezes the defaults of the 20-level table.
func TestTableDefaultsCharacterization(t *testing.T) {
	t.Parallel()
	goldenText(t, "tabledefaults", map[string]string{"srd": jsonText(loadForTest(t).TableDefaults())})
}

// TestListSpellsCharacterization freezes the keys ListSpells returns for each
// filter, alone and together.
func TestListSpellsCharacterization(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	filters := map[string]SpellFilter{
		"everything":              {},
		"query pt accents":        {Query: "  BOLA  "},
		"query en":                {Query: "missile"},
		"query nothing":           {Query: "zzzz"},
		"class wizard":            {Class: "class:wizard"},
		"class fighter, no list":  {Class: "class:fighter"},
		"class unknown":           {Class: "class:nope"},
		"a subclass with no list": {Class: "subclass:eldritch-knight"},
		"levels 0 and 9":          {Levels: []int{0, 9}},
		"school evocation":        {Schools: []string{"school:evocation"}},
		"learnable none":          {Learnable: []SpellAccess{}},
		"learnable two lists":     {Learnable: []SpellAccess{{List: "class:bard", MaxLevel: 1}, {List: "class:paladin", MaxLevel: 2}}},
		"hidden all":              {Hidden: func(string) bool { return true }},
		"hidden one":              {Hidden: func(k string) bool { return k == "spell:fireball" }, Query: "bola"},
		"together":                {Query: "a", Class: "class:cleric", Levels: []int{1, 2}, Schools: []string{"school:abjuration", "school:necromancy"}, Learnable: []SpellAccess{{List: "class:cleric", MaxLevel: 1}}},
	}
	got := map[string]string{}
	for name, f := range filters {
		var keys []string
		for _, e := range c.ListSpells(f) {
			keys = append(keys, e.Key)
		}
		got[name] = strings.Join(keys, " ")
	}
	goldenText(t, "listspells", got)
}
