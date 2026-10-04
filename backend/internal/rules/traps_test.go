package rules

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestTrapPresetsOfTheSRD(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)

	var keys []string
	var names []string
	for _, p := range c.TrapPresets() {
		keys = append(keys, p.Key)
		names = append(names, p.NamePT)
		if p.DescriptionPT == "" || p.FindDC == 0 {
			t.Errorf("%s: a preset has a description and a DC to find it", p.Key)
		}
	}
	// Every condition and damage type a preset names exists (as TestReferences
	// does for the snapshot): the loader checks it, and this keeps it checked.
	for _, p := range c.TrapPresets() {
		var types []string
		for _, d := range p.Damage {
			types = append(types, d.Type)
		}
		if p.Attack != nil {
			types = append(types, p.Attack.Damage.Type)
		}
		if p.Save != nil {
			for _, d := range p.Save.OnFail.Damage {
				types = append(types, d.Type)
			}
			types = append(types, p.Save.OnFail.Condition)
		}
		for _, cond := range p.Conditions {
			types = append(types, cond.Condition)
		}
		for _, k := range types {
			if k != "" && !c.c.exists(k) {
				t.Errorf("%s points at unknown %s", p.Key, k)
			}
		}
	}
	wantNames := "Fosso simples, Fosso escondido, Agulha envenenada, Dardos envenenados, Teto que desaba, Estátua que cospe fogo, Rede que cai, Esfera rolante"
	if got := strings.Join(names, ", "); got != wantNames {
		t.Errorf("names = %s\nwant    %s", got, wantNames)
	}
	if len(keys) != 8 || keys[0] != "trap:simple-pit" || keys[7] != "trap:rolling-sphere" {
		t.Errorf("keys = %v", keys)
	}

	dmg := func(dice, kind string) TrapDamage {
		d, _ := ParseDice(dice)
		return TrapDamage{Dice: d, Type: "damage-type:" + kind}
	}
	get := func(key string) TrapPreset {
		t.Helper()
		p, ok := c.TrapPreset(key)
		if !ok {
			t.Fatalf("no preset %s", key)
		}
		return p
	}

	// SRD numbers, trap by trap.
	pit := get("trap:simple-pit")
	if pit.NoticeDC != 10 || pit.FindDC != 10 || pit.Trigger != TrapTriggerEnter || pit.AreaSize != 1 || pit.FallFt != 10 ||
		len(pit.Damage) != 1 || pit.Damage[0] != dmg("1d6", "bludgeoning") || pit.Save != nil || pit.Attack != nil {
		t.Errorf("simple pit = %+v", pit)
	}
	// SRD falling: the creature lands prone.
	for _, k := range []string{"trap:simple-pit", "trap:hidden-pit"} {
		if p := get(k); len(p.Conditions) != 1 || p.Conditions[0].Condition != "condition:prone" {
			t.Errorf("%s: a fall lands the creature prone, got %+v", k, p.Conditions)
		}
	}
	// The find DC is ours (not an SRD number) for these four, and the SRD's for the rest.
	ours := map[string]bool{"trap:simple-pit": true, "trap:collapsing-roof": true, "trap:fire-breathing-statue": true, "trap:falling-net": true}
	for _, p := range c.TrapPresets() {
		if p.FindDCIsOurs != ours[p.Key] {
			t.Errorf("%s: FindDCIsOurs = %v", p.Key, p.FindDCIsOurs)
		}
	}
	hidden := get("trap:hidden-pit")
	if hidden.NoticeDC != 15 || hidden.FindDC != 15 || hidden.AreaSize != 2 || hidden.FallFt != 20 || hidden.Damage[0] != dmg("2d6", "bludgeoning") {
		t.Errorf("hidden pit (the cave's Fosso escondido: 15, 15, a fall of 6 m for 2d6) = %+v", hidden)
	}
	needle := get("trap:poison-needle")
	if needle.NoticeDC != 0 || needle.FindDC != 20 || needle.Trigger != TrapTriggerManual || len(needle.Damage) != 2 ||
		needle.Damage[0] != dmg("1", "piercing") || needle.Damage[1] != dmg("2d10", "poison") {
		t.Errorf("poison needle = %+v", needle)
	}
	if s := needle.Save; s == nil || s.Ability != CON || s.DC != 15 || s.OnFail.Condition != "condition:poisoned" || s.OnFail.DurationPT != "1 hora" ||
		len(s.OnFail.Damage) != 0 || s.OnPass != TrapPassNone || s.AppliesTo != TrapSaveCaught {
		t.Errorf("poison needle save = %+v", needle.Save)
	}
	darts := get("trap:poison-darts")
	if a := darts.Attack; a == nil || a.Bonus != 8 || a.Count != 4 || a.Damage != dmg("1d4", "piercing") || darts.Targets != TrapTargetsManual {
		t.Errorf("poison darts attack = %+v", darts.Attack)
	}
	if s := darts.Save; s == nil || s.Ability != CON || s.DC != 15 || s.AppliesTo != TrapSaveHit || s.OnPass != TrapPassHalf ||
		len(s.OnFail.Damage) != 1 || s.OnFail.Damage[0] != dmg("2d10", "poison") {
		t.Errorf("poison darts save = %+v", darts.Save)
	}
	roof := get("trap:collapsing-roof")
	if roof.NoticeDC != 10 || roof.Save == nil || roof.Save.Ability != DEX || roof.Save.DC != 15 || roof.Save.OnFail.Damage[0] != dmg("4d10", "bludgeoning") || roof.Save.OnPass != TrapPassHalf {
		t.Errorf("collapsing roof = %+v", roof)
	}
	statue := get("trap:fire-breathing-statue")
	if statue.Kind != TrapMagic || statue.NoticeDC != 15 || statue.Save == nil || statue.Save.DC != 13 || statue.Save.OnFail.Damage[0] != dmg("4d10", "fire") || statue.Targets != TrapTargetsManual {
		t.Errorf("fire-breathing statue = %+v", statue)
	}
	net := get("trap:falling-net")
	if len(net.Conditions) != 1 || net.Conditions[0].Condition != "condition:restrained" || net.Save == nil || net.Save.Ability != STR || net.Save.DC != 10 ||
		net.Save.OnFail.Condition != "condition:prone" || net.Save.OnPass != TrapPassNone {
		t.Errorf("falling net = %+v", net)
	}
	sphere := get("trap:rolling-sphere")
	if sphere.NoticeDC != 15 || sphere.Save == nil || sphere.Save.Ability != DEX || sphere.Save.DC != 15 || sphere.Save.OnFail.Damage[0] != dmg("10d10", "bludgeoning") ||
		sphere.Save.OnFail.Condition != "condition:prone" || sphere.Targets != TrapTargetsManual {
		t.Errorf("rolling sphere = %+v", sphere)
	}
	if _, ok := c.TrapPreset("trap:sphere-of-annihilation"); ok {
		t.Error("the sphere of annihilation is not one of the eight presets")
	}
}

func TestTrapSeverityTables(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	sev := c.TrapSeverities()
	want := []TrapSeverity{
		{Key: "setback", NamePT: "Revés", SaveDC: IntRange{10, 11}, AttackBonus: IntRange{3, 5}},
		{Key: "dangerous", NamePT: "Perigosa", SaveDC: IntRange{12, 15}, AttackBonus: IntRange{6, 8}},
		{Key: "deadly", NamePT: "Mortal", SaveDC: IntRange{16, 20}, AttackBonus: IntRange{9, 12}},
	}
	if len(sev) != 3 || sev[0] != want[0] || sev[1] != want[1] || sev[2] != want[2] {
		t.Errorf("TrapSeverities() = %+v, want %+v", sev, want)
	}
	rows := c.TrapDamageByLevel()
	d := func(s string) DiceFormula { f, _ := ParseDice(s); return f }
	if len(rows) != 4 || rows[0] != (TrapDamageRow{1, 4, d("1d10"), d("2d10"), d("4d10")}) || rows[3] != (TrapDamageRow{17, 20, d("10d10"), d("18d10"), d("24d10")}) ||
		rows[1].Dangerous != d("4d10") || rows[2].Deadly != d("18d10") {
		t.Errorf("TrapDamageByLevel() = %+v", rows)
	}
	// Every preset's DC sits in a severity's range, or is an SRD one-off the
	// table does not cover (the pits and the net are below "Revés").
	for _, p := range c.TrapPresets() {
		if p.Save != nil && p.Save.DC > 20 {
			t.Errorf("%s: DC %d is above the SRD's deadliest", p.Key, p.Save.DC)
		}
	}
}

func TestFallDice(t *testing.T) {
	t.Parallel()
	for feet, want := range map[int]DiceFormula{
		-5: {}, 0: {}, 9: {}, 10: {Count: 1, Sides: 6}, 19: {Count: 1, Sides: 6}, 20: {Count: 2, Sides: 6}, 60: {Count: 6, Sides: 6},
		200: {Count: 20, Sides: 6}, 500: {Count: 20, Sides: 6},
	} {
		if got := FallDice(feet); got != want {
			t.Errorf("FallDice(%d) = %+v, want %+v", feet, got, want)
		}
	}
}

func TestTrapPresetsAreDeepCopies(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	a := c.TrapPresets()
	a[0].NamePT = "x"
	if b := c.TrapPresets(); b[0].NamePT == "x" {
		t.Error("TrapPresets shares its slice")
	}
	// Edit every nested field of a copy: the content must stay as it was.
	p, _ := c.TrapPreset("trap:poison-darts")
	p.Attack.Bonus, p.Save.DC = 99, 99
	p.Save.OnFail.Damage[0].Type = "x"
	n, _ := c.TrapPreset("trap:poison-needle")
	n.Damage[1].Type = "x"
	n.Save.OnFail.Condition = "x"
	f, _ := c.TrapPreset("trap:falling-net")
	f.Conditions[0].Condition = "x"
	for _, all := range [][]TrapPreset{c.TrapPresets()} {
		for _, q := range all {
			switch q.Key {
			case "trap:poison-darts":
				if q.Attack.Bonus != 8 || q.Save.DC != 15 || q.Save.OnFail.Damage[0].Type != "damage-type:poison" {
					t.Errorf("a copy changed the darts: %+v", q)
				}
			case "trap:poison-needle":
				if q.Damage[1].Type != "damage-type:poison" || q.Save.OnFail.Condition != "condition:poisoned" {
					t.Errorf("a copy changed the needle: %+v", q)
				}
			case "trap:falling-net":
				if q.Conditions[0].Condition != "condition:restrained" {
					t.Errorf("a copy changed the net: %+v", q)
				}
			}
		}
	}
	if q, _ := c.TrapPreset("trap:poison-darts"); q.Attack.Bonus != 8 {
		t.Error("TrapPreset shares its Attack")
	}
}

// trapsContent is a bare content that knows what the loader looks up.
func trapsContent() *content {
	return &content{
		namesEN: map[string]string{"damage-type:poison": "", "damage-type:piercing": "", "damage-type:bludgeoning": "", "condition:poisoned": "", "condition:prone": "", "spell:x": ""},
		namesPT: map[string]string{"trap:a": "A"},
	}
}

const severityOK = `"severity":{"levels":[
 {"key":"setback","name_pt":"Revés","save_dc":{"min":10,"max":11},"attack_bonus":{"min":3,"max":5}},
 {"key":"dangerous","name_pt":"Perigosa","save_dc":{"min":12,"max":15},"attack_bonus":{"min":6,"max":8}},
 {"key":"deadly","name_pt":"Mortal","save_dc":{"min":16,"max":20},"attack_bonus":{"min":9,"max":12}}],
 "damage_by_level":[{"from":1,"to":10,"setback":"1d10","dangerous":"2d10","deadly":"4d10"},{"from":11,"to":20,"setback":"2d10","dangerous":"4d10","deadly":"10d10"}]}`

func trapFile(trap string) string {
	return `{` + severityOK + `,"traps":[` + trap + `]}`
}

const trapHead = `"key":"a","kind":"mechanical","description_pt":"Um teste.","find_dc":12,"trigger":"enter","area_size":1,"targets":"area"`

func loadTrapFile(body string) error {
	return trapsContent().loadTraps(fstest.MapFS{"effects/traps.json": {Data: []byte(body)}})
}

// The loader refuses what the closed schema does not allow.
func TestLoadTrapsRefuses(t *testing.T) {
	t.Parallel()
	good := `{` + trapHead + `,"damage":[{"dice":"2d6","type":"damage-type:bludgeoning"}],"fall_ft":20,
	  "save":{"ability":"con","dc":15,"applies_to":"caught","on_fail":{"damage":[{"dice":"2d10","type":"damage-type:poison"}],"condition":"condition:poisoned","duration_pt":"1 hora"},"on_pass":"half"}}`
	if err := loadTrapFile(trapFile(good)); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	bad := map[string]string{
		"an unknown field":             `{` + trapHead + `,"loud":true}`,
		"an unknown kind":              `{"key":"a","kind":"divine","description_pt":"x","find_dc":12,"trigger":"enter","area_size":1,"targets":"area"}`,
		"no description":               `{"key":"a","kind":"magic","description_pt":" ","find_dc":12,"trigger":"enter","area_size":1,"targets":"area"}`,
		"a DC of 0 to find":            `{"key":"a","kind":"magic","description_pt":"x","find_dc":0,"trigger":"enter","area_size":1,"targets":"area"}`,
		"a notice DC above 30":         `{` + trapHead + `,"notice_dc":31}`,
		"an unknown trigger":           `{"key":"a","kind":"magic","description_pt":"x","find_dc":12,"trigger":"touch","area_size":1,"targets":"area"}`,
		"an area of 5":                 `{"key":"a","kind":"magic","description_pt":"x","find_dc":12,"trigger":"enter","area_size":5,"targets":"area"}`,
		"unknown targets":              `{"key":"a","kind":"magic","description_pt":"x","find_dc":12,"trigger":"enter","area_size":1,"targets":"all"}`,
		"a repeated key":               `{` + trapHead + `},{` + trapHead + `}`,
		"a key with a colon":           `{"key":"trap:a","kind":"magic","description_pt":"x","find_dc":12,"trigger":"enter","area_size":1,"targets":"area"}`,
		"a trap with no name":          `{"key":"b","kind":"magic","description_pt":"x","find_dc":12,"trigger":"enter","area_size":1,"targets":"area"}`,
		"an unknown damage type":       `{` + trapHead + `,"damage":[{"dice":"1d6","type":"damage-type:love"}]}`,
		"a damage that is a condition": `{` + trapHead + `,"damage":[{"dice":"1d6","type":"condition:poisoned"}]}`,
		"damage that is not dice":      `{` + trapHead + `,"damage":[{"dice":"lots","type":"damage-type:poison"}]}`,
		"dice that add a modifier":     `{` + trapHead + `,"damage":[{"dice":"1d6 + MOD","type":"damage-type:poison"}]}`,
		"a pit that does not fall":     `{` + trapHead + `,"fall_ft":20,"damage":[{"dice":"1d6","type":"damage-type:bludgeoning"}]}`,
		"a pit of 25 ft":               `{` + trapHead + `,"fall_ft":25}`,
		"an unknown condition":         `{` + trapHead + `,"conditions":[{"condition":"condition:sleepy"}]}`,
		"a condition that is a spell":  `{` + trapHead + `,"conditions":[{"condition":"spell:x"}]}`,
		"an attack with no count":      `{` + trapHead + `,"attack":{"bonus":8,"count":0,"damage":{"dice":"1d4","type":"damage-type:piercing"}}}`,
		"a save with no ability":       `{` + trapHead + `,"save":{"dc":15,"applies_to":"caught","on_fail":{"condition":"condition:prone"},"on_pass":"none"}}`,
		"a save of an unknown ability": `{` + trapHead + `,"save":{"ability":"luck","dc":15,"applies_to":"caught","on_fail":{"condition":"condition:prone"},"on_pass":"none"}}`,
		"a save with no DC":            `{` + trapHead + `,"save":{"ability":"dex","applies_to":"caught","on_fail":{"condition":"condition:prone"},"on_pass":"none"}}`,
		"a save of the hit, no attack": `{` + trapHead + `,"save":{"ability":"dex","dc":15,"applies_to":"hit","on_fail":{"condition":"condition:prone"},"on_pass":"none"}}`,
		"a failure that does nothing":  `{` + trapHead + `,"save":{"ability":"dex","dc":15,"applies_to":"caught","on_fail":{},"on_pass":"none"}}`,
		"half of no damage":            `{` + trapHead + `,"save":{"ability":"dex","dc":15,"applies_to":"caught","on_fail":{"condition":"condition:prone"},"on_pass":"half"}}`,
		"an unknown pass":              `{` + trapHead + `,"save":{"ability":"dex","dc":15,"applies_to":"caught","on_fail":{"condition":"condition:prone"},"on_pass":"double"}}`,
		"a duration with no condition": `{` + trapHead + `,"save":{"ability":"dex","dc":15,"applies_to":"caught","on_fail":{"damage":[{"dice":"1d6","type":"damage-type:poison"}],"duration_pt":"1 hora"},"on_pass":"none"}}`,
		"a fail condition that is not": `{` + trapHead + `,"save":{"ability":"dex","dc":15,"applies_to":"caught","on_fail":{"condition":"damage-type:poison"},"on_pass":"none"}}`,
	}
	for name, trap := range bad {
		body := trapFile(trap)
		// Names for the keys the cases use.
		c := trapsContent()
		if name != "a trap with no name" {
			c.namesPT["trap:b"] = "B"
		} else {
			delete(c.namesPT, "trap:a")
			trap = `{` + trapHead + `}`
			body = trapFile(trap)
		}
		if err := c.loadTraps(fstest.MapFS{"effects/traps.json": {Data: []byte(body)}}); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	if err := loadTrapFile(trapFile(`{"key":"a","kind":"mechanical","description_pt":"x","find_dc":12,"trigger":"enter","area_size":1,"targets":"area"}`)); err != nil {
		t.Errorf("an effect-less trap (\"só descrição\") is allowed: %v", err)
	}
	// A name with no trap is a leftover.
	c := trapsContent()
	c.namesPT["trap:gone"] = "Sumiu"
	if err := c.loadTraps(fstest.MapFS{"effects/traps.json": {Data: []byte(trapFile(`{` + trapHead + `}`))}}); err == nil {
		t.Error("a Portuguese name of a trap that is not in traps.json must be refused")
	}
}

func TestLoadTrapSeverityRefuses(t *testing.T) {
	t.Parallel()
	trap := `{` + trapHead + `}`
	bad := map[string]string{
		"a missing level":         strings.Replace(trapFile(trap), `{"key":"deadly","name_pt":"Mortal","save_dc":{"min":16,"max":20},"attack_bonus":{"min":9,"max":12}}`, ``, 1),
		"levels out of order":     strings.Replace(trapFile(trap), `"key":"setback"`, `"key":"deadly"`, 1),
		"an empty range":          strings.Replace(trapFile(trap), `"save_dc":{"min":10,"max":11}`, `"save_dc":{"min":12,"max":11}`, 1),
		"a gap in the levels":     strings.Replace(trapFile(trap), `"from":11`, `"from":12`, 1),
		"levels that stop at 19":  strings.Replace(trapFile(trap), `"to":20`, `"to":19`, 1),
		"damage that is not dice": strings.Replace(trapFile(trap), `"deadly":"4d10"`, `"deadly":"deadly"`, 1),
	}
	for name, body := range bad {
		if err := loadTrapFile(body); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
}
