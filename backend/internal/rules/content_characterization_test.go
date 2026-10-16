package rules

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The characterization of load and its own steps (loadData, indexLevels,
// loadEffects): the SRD's files, one of them broken at a time, and the exact
// error load answers. The order of the cases' errors is the order of load's
// steps, so a split cannot reorder them.

var (
	contentFilesOnce sync.Once
	contentFiles     map[string][]byte
)

// srdContentFiles reads every file of the SRD content once.
func srdContentFiles(t *testing.T) map[string][]byte {
	t.Helper()
	contentFilesOnce.Do(func() {
		contentFiles = map[string][]byte{}
		err := fs.WalkDir(srd51.Files, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := fs.ReadFile(srd51.Files, p)
			contentFiles[p] = b
			return err
		})
		if err != nil {
			panic(err)
		}
	})
	return contentFiles
}

// contentEdit changes the files of one case: a nil value removes a file.
type contentEdit map[string][]byte

// editContentJSON decodes a file of the SRD, lets edit change it, and encodes
// it back.
func editContentJSON(t *testing.T, name string, edit func(v any) any) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(srdContentFiles(t)[name], &v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	b, err := json.Marshal(edit(v))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// firstRow is the first object of a JSON list.
func firstRow(v any) map[string]any {
	rows, _ := v.([]any)
	row, _ := rows[0].(map[string]any)
	return row
}

// effectsOf is the "effects" object of an effects file.
func effectsOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	e, _ := m["effects"].(map[string]any)
	return e
}

func contentLoadCases(t *testing.T) map[string]contentEdit {
	t.Helper()
	files := srdContentFiles(t)
	broken := []byte(`{`)
	cases := map[string]contentEdit{"ok: the SRD": {}}
	// A missing file stops load at the step that reads it, which freezes the
	// order of the steps.
	for name := range files {
		if strings.HasSuffix(name, ".go") {
			continue
		}
		cases["missing: "+name] = contentEdit{name: nil}
	}
	cases["broken: data/manifest.json"] = contentEdit{"data/manifest.json": broken}
	cases["bad: no snapshot version"] = contentEdit{"data/manifest.json": []byte(`{"snapshot_version":""}`)}
	cases["bad: an unknown ability"] = contentEdit{"data/abilities.json": editContentJSON(t, "data/abilities.json", func(v any) any {
		firstRow(v)["key"] = "luck"
		return v
	})}
	cases["bad: five abilities"] = contentEdit{"data/abilities.json": editContentJSON(t, "data/abilities.json", func(v any) any {
		return v.([]any)[1:]
	})}
	cases["bad: a duplicate skill"] = contentEdit{"data/skills.json": editContentJSON(t, "data/skills.json", func(v any) any {
		return append(v.([]any), v.([]any)[0])
	})}
	cases["bad: a duplicate condition"] = contentEdit{"data/conditions.json": editContentJSON(t, "data/conditions.json", func(v any) any {
		return append(v.([]any), v.([]any)[0])
	})}
	cases["bad: a skill and a race duplicated"] = contentEdit{
		"data/skills.json": editContentJSON(t, "data/skills.json", func(v any) any { return append(v.([]any), v.([]any)[0]) }),
		"data/races.json":  editContentJSON(t, "data/races.json", func(v any) any { return append(v.([]any), v.([]any)[0]) }),
	}
	cases["bad: a feat of an unknown ability"] = contentEdit{"data/feats.json": editContentJSON(t, "data/feats.json", func(v any) any {
		firstRow(v)["minimums"] = map[string]any{"luck": 13}
		return v
	})}
	spellArea := func(area string, size int) []byte {
		return editContentJSON(t, "data/spells.json", func(v any) any {
			row := firstRow(v)
			row["area_type"], row["area_size_ft"] = area, size
			if area == "" {
				delete(row, "area_type")
			}
			return v
		})
	}
	cases["bad: an area of no size"] = contentEdit{"data/spells.json": spellArea("cone", 0)}
	cases["bad: a size of no area"] = contentEdit{"data/spells.json": spellArea("", 10)}
	cases["bad: an area of no shape"] = contentEdit{"data/spells.json": spellArea("star", 10)}
	cases["bad: an area of 7 ft"] = contentEdit{"data/spells.json": spellArea("cone", 7)}
	cases["bad: an area under 5 ft"] = contentEdit{"data/spells.json": spellArea("cone", -5)}
	cases["ok: an area of 5 ft"] = contentEdit{"data/spells.json": spellArea("sphere", 5)}
	cases["bad: a bad monster"] = contentEdit{"data/monsters.json": editContentJSON(t, "data/monsters.json", func(v any) any {
		firstRow(v)["challenge_rating"] = "x"
		return v
	})}
	cases["bad: a magic item fewer"] = contentEdit{"data/magic-items.json": editContentJSON(t, "data/magic-items.json", func(v any) any {
		return v.([]any)[1:]
	})}
	cases["bad: a spell text of an unknown spell"] = contentEdit{"effects/spells_pt.json": []byte(`{"spell:nope":{}}`)}
	level := func(edit func(rows []any) []any) []byte {
		return editContentJSON(t, "data/levels.json", func(v any) any { return edit(v.([]any)) })
	}
	cases["bad: a level 0 row"] = contentEdit{"data/levels.json": level(func(rows []any) []any {
		rows[0].(map[string]any)["level"] = 0
		return rows
	})}
	cases["bad: a level 21 row"] = contentEdit{"data/levels.json": level(func(rows []any) []any {
		rows[0].(map[string]any)["level"] = 21
		return rows
	})}
	cases["bad: a class missing a level"] = contentEdit{"data/levels.json": level(func(rows []any) []any { return rows[1:] })}
	cases["bad: a class with no rows"] = contentEdit{"data/levels.json": level(func(rows []any) []any {
		var out []any
		for _, r := range rows {
			if r.(map[string]any)["class"] != "class:monk" {
				out = append(out, r)
			}
		}
		return out
	})}
	cases["ok: a row of a level 1 subclass"] = contentEdit{"data/levels.json": level(func(rows []any) []any {
		return append(rows, map[string]any{"class": "class:wizard", "subclass": "subclass:evocation", "level": 1})
	})}
	cases["bad: a correction of an unknown class"] = contentEdit{"effects/corrections.json": []byte(`{"spell_list_corrections":[{"class":"class:nope"}]}`)}
	cases["bad: revision 0"] = contentEdit{"effects/revision.json": []byte(`{"revision":0,"sha256":"x"}`)}
	cases["ok: revision 1"] = contentEdit{"effects/revision.json": []byte(`{"revision":1,"sha256":"x"}`)}
	names := func(add map[string]string) []byte {
		return editContentJSON(t, "effects/names_pt.json", func(v any) any {
			n, _ := v.(map[string]any)["names"].(map[string]any)
			for k, val := range add {
				n[k] = val
			}
			return v
		})
	}
	cases["bad: a name of an unknown key"] = contentEdit{"effects/names_pt.json": names(map[string]string{"spell:nope": "Nada"})}
	cases["bad: a name of an unknown attack"] = contentEdit{"effects/names_pt.json": names(map[string]string{"attack:nope-nope": "Nada"})}
	cases["ok: names of every free prefix"] = contentEdit{"effects/names_pt.json": names(map[string]string{
		"sense:x": "x", "resource:x": "x", "creature-type:x": "x", "terrain:x": "x", "str": "Força",
	})}
	cases["bad: an attunement name of no item"] = contentEdit{"effects/names_pt.json": names(map[string]string{"attunement:x": "x"})}
	cases["bad: a light name of no light"] = contentEdit{"effects/names_pt.json": names(map[string]string{"light:x": "x"})}
	cases["bad: an effects file broken"] = contentEdit{"effects/wizard.json": broken}
	cases["bad: an effect of an unknown key"] = contentEdit{"effects/wizard.json": editContentJSON(t, "effects/wizard.json", func(v any) any {
		effectsOf(v)["feature:nope"] = []any{}
		return v
	})}
	cases["bad: an effect of an unknown background feature"] = contentEdit{"effects/wizard.json": editContentJSON(t, "effects/wizard.json", func(v any) any {
		effectsOf(v)["background-feature:nope"] = []any{}
		return v
	})}
	cases["bad: effects in two files"] = contentEdit{"effects/wizard.json": editContentJSON(t, "effects/wizard.json", func(v any) any {
		effectsOf(v)["feature:rage"] = []any{}
		return v
	})}
	cases["bad: an effect that does not compile"] = contentEdit{"effects/wizard.json": editContentJSON(t, "effects/wizard.json", func(v any) any {
		effectsOf(v)["feature:arcane-recovery"] = []any{map[string]any{"type": "fly"}}
		return v
	})}
	cases["ok: a new effects file"] = contentEdit{"effects/extra.json": []byte(`{"effects":{"feature:additional-fighting-style":[{"type":"note","text_pt":"Mais um estilo."}]}}`)}
	cases["bad: a new effects file broken"] = contentEdit{"effects/extra.json": broken}
	cases["bad: two spellcasting effects"] = contentEdit{"effects/wizard.json": editContentJSON(t, "effects/wizard.json", func(v any) any {
		e := effectsOf(v)
		e["feature:arcane-recovery"] = append(e["feature:arcane-recovery"].([]any), e["feature:spellcasting-wizard"].([]any)[0])
		return v
	})}
	return cases
}

// TestLoadContentCharacterization freezes load's refusals and their order.
func TestLoadContentCharacterization(t *testing.T) {
	t.Parallel()
	files := srdContentFiles(t)
	got := map[string]string{}
	for name, edit := range contentLoadCases(t) {
		fsys := fstest.MapFS{}
		for p, b := range files {
			fsys[p] = &fstest.MapFile{Data: b}
		}
		for p, b := range edit {
			if b == nil {
				delete(fsys, p)
				continue
			}
			fsys[p] = &fstest.MapFile{Data: b}
		}
		c, err := load(fsys)
		text := errText(err)
		if err == nil {
			text += fmt.Sprintf(" %s, %d spells, %d effects, %d names, %d casters", c.version, len(c.spells), len(c.effects), len(c.namesPT), len(c.casting))
		}
		// A missing file may be one load does not need: either answer is frozen.
		if !strings.HasPrefix(name, "missing:") && strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = text
	}
	goldenText(t, "content-load-refusals", got)
}

// TestCorrectCreatureCharacterization freezes correctCreature and
// correctCreatureLists field by field: each refusal and what each write
// leaves on the stat block.
func TestCorrectCreatureCharacterization(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ field, raw string }{
		"ok: armor class":             {"armor_class", `15`},
		"bad: armor class 0":          {"armor_class", `0`},
		"bad: armor class 101":        {"armor_class", `101`},
		"bad: armor class text":       {"armor_class", `"x"`},
		"ok: hit points max":          {"hit_points", `65536`},
		"bad: hit points over":        {"hit_points", `65537`},
		"ok: speed walk 0":            {"speed_walk", `0`},
		"bad: speed fly 501":          {"speed_fly", `501`},
		"ok: speed swim":              {"speed_swim", `30`},
		"ok: speed climb":             {"speed_climb", `500`},
		"ok: speed burrow":            {"speed_burrow", `10`},
		"ok: darkvision":              {"darkvision", `60`},
		"ok: blindsight":              {"blindsight", `10`},
		"ok: tremorsense":             {"tremorsense", `30`},
		"ok: truesight":               {"truesight", `120`},
		"bad: truesight negative":     {"truesight", `-5`},
		"ok: passive perception":      {"passive_perception", `100`},
		"bad: passive perception 0":   {"passive_perception", `0`},
		"ok: attacks per action":      {"attacks_per_action", `20`},
		"bad: attacks per action 21":  {"attacks_per_action", `21`},
		"bad: attacks per action 0":   {"attacks_per_action", `0`},
		"ok: hit points roll":         {"hit_points_roll", `"6d8+6"`},
		"ok: hit points roll bare":    {"hit_points_roll", `"6d8"`},
		"ok: hit points roll minus":   {"hit_points_roll", `"6d8-1"`},
		"bad: hit points roll number": {"hit_points_roll", `12`},
		"bad: hit points roll dice":   {"hit_points_roll", `"5d8+6"`},
		"bad: hit points roll text":   {"hit_points_roll", `"lots"`},
		"ok: skills":                  {"skills", `{"skill:perception":5}`},
		"bad: skills empty":           {"skills", `{}`},
		"bad: skills list":            {"skills", `["skill:perception"]`},
		"bad: skills unknown":         {"skills", `{"skill:nope":1}`},
		"ok: damage immunities":       {"damage_immunities", `["damage-type:fire","damage-type:cold"]`},
		"bad: damage immunities none": {"damage_immunities", `[]`},
		"bad: damage immunities map":  {"damage_immunities", `{}`},
		"bad: damage immunity twice":  {"damage_immunities", `["damage-type:fire","damage-type:fire"]`},
		"bad: damage immunity nil":    {"damage_immunities", `["damage-type:ghost"]`},
		"bad: damage immunity cond":   {"damage_immunities", `["condition:deafened"]`},
		"ok: condition immunities":    {"condition_immunities", `["condition:deafened"]`},
		"bad: condition is damage":    {"condition_immunities", `["damage-type:fire"]`},
		"bad: condition unknown":      {"condition_immunities", `["condition:nope"]`},
		"bad: an unknown field":       {"challenge_rating", `"1"`},
		"bad: an empty field":         {"", `1`},
	}
	got := map[string]string{}
	run := func(name, key, field, raw string) {
		c := correctionsContent()
		c.attacksPerAction = map[string]int{}
		m := c.monsters[key]
		err := c.correctCreature(key, m, field, json.RawMessage(raw))
		text := errText(err)
		if err == nil {
			text += " " + contentJSONText([]any{m, c.attacksPerAction})
		}
		if strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = text
	}
	for name, tc := range cases {
		run(name, "monster:veteran", tc.field, tc.raw)
	}
	run("bad: attacks per action with no Multiattack", "monster:rat", "attacks_per_action", `2`)
	run("ok: skills on a block of none", "monster:rat", "skills", `{"skill:perception":2}`)
	// An immunity with a note would be lost by a corrected list.
	c := correctionsContent()
	m := c.monsters["monster:veteran"]
	m.Immunities = []srd51.MonsterDamageMod{{Types: []string{"damage-type:fire"}, Note: "x"}}
	got["bad: an immunity with a note"] = errText(c.correctCreature("monster:veteran", m, "damage_immunities", json.RawMessage(`["damage-type:cold"]`)))
	goldenText(t, "correctcreature", got)
}

// TestCorrectSubclassFeaturesRowOfANewSubclass freezes the row made for a
// subclass the snapshot has no row of at all.
func TestCorrectSubclassFeaturesRowOfANewSubclass(t *testing.T) {
	t.Parallel()
	c := correctionsContent()
	c.features["feature:e"] = &srd51.Feature{Key: "feature:e", Subclass: "subclass:empty"}
	err := c.correctSubclassFeatures("f.json", []subclassRowCorrection{{Subclass: "subclass:empty", Level: 2, Add: []string{"feature:e"}, Source: "SRD"}})
	goldenText(t, "correctsubclass-new", map[string]string{"empty": errText(err) + " " + contentJSONText(c.subclassLevels["subclass:empty"])})
}

// contentJSONText is v as JSON, or the error.
func contentJSONText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}
