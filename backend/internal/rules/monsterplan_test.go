package rules

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

func planActionOf(t *testing.T, c *Content, creature, action string) ActionPlan {
	t.Helper()
	a, ok := c.MonsterActionPlan("monster:" + creature + "#" + slugOf(action))
	if !ok {
		t.Fatalf("%s has no action %q", creature, action)
	}
	return a
}

func dmgText(parts []ActionDamage) string {
	var out []string
	for _, d := range parts {
		out = append(out, formulaText(d.Dice)+" "+strings.TrimPrefix(d.TypeKey, "damage-type:"))
	}
	return strings.Join(out, " + ")
}

// formulaText writes a formula as "2d6+3", "1d4" or a flat "1".
func formulaText(f DiceFormula) string {
	text := ""
	if f.Count > 0 {
		text = fmt.Sprintf("%dd%d", f.Count, f.Sides)
	}
	switch {
	case text == "":
		return fmt.Sprint(f.Bonus)
	case f.Bonus > 0:
		return fmt.Sprintf("%s+%d", text, f.Bonus)
	case f.Bonus < 0:
		return fmt.Sprintf("%s%d", text, f.Bonus)
	}
	return text
}

// TestMonsterPlanReadsTheActionsAsWritten: the actions the master runs most are
// read as the SRD writes them (SRD 5.1, "Monsters": actions).
func TestMonsterPlanReadsTheActionsAsWritten(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)

	t.Run("a ghoul's claws paralyze on a failed save", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "ghoul", "Claws")
		if a.Kind != ActionKindAttack || a.Class != ActionStructured || a.Attack == nil || a.Attack.Bonus != 4 || !a.Attack.Melee || a.Attack.ReachFt != 5 {
			t.Fatalf("Claws = %+v, want a structured melee attack at +4, reach 5", a)
		}
		if got := dmgText(a.Damage); got != "2d4+2 slashing" {
			t.Errorf("damage = %q, want 2d4+2 slashing", got)
		}
		s := a.Save
		if s == nil || !s.OnHit || s.Ability != CON || s.DC != 10 || s.ConditionKey != "condition:paralyzed" || s.OnSuccess != "none" || len(s.Damage) != 0 {
			t.Errorf("save = %+v, want the rider: DC 10 Constitution on a hit, paralyzed on a failure", s)
		}
		if s != nil && (s.Duration != "1 minute" || !s.RepeatSave) {
			t.Errorf("duration %q repeat %v, want 1 minute with a repeat at the end of each turn", s.Duration, s.RepeatSave)
		}
	})
	t.Run("a poison bite: the save governs the poison, half on a success", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "giant-spider", "Bite")
		if got := dmgText(a.Damage); got != "1d8+3 piercing" {
			t.Errorf("hit damage = %q, want 1d8+3 piercing", got)
		}
		s := a.Save
		if s == nil || !s.OnHit || s.Ability != CON || s.DC != 11 || s.OnSuccess != "half" || dmgText(s.Damage) != "2d8 poison" {
			t.Errorf("save = %+v, want DC 11 Constitution, 2d8 poison, half on a success", s)
		}
	})
	t.Run("the assassin's poison is rolled once, with the save", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "assassin", "Shortsword")
		if got := dmgText(a.Damage); got != "1d6+3 piercing" {
			t.Errorf("hit damage = %q, want 1d6+3 piercing only", got)
		}
		if a.Save == nil || dmgText(a.Save.Damage) != "7d6 poison" || a.Save.OnSuccess != "half" {
			t.Errorf("save = %+v, want 7d6 poison, half on a success", a.Save)
		}
	})
	t.Run("a breath weapon: cone, save, damage, half on a success, recharge", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "adult-red-dragon", "Fire Breath")
		s := a.Save
		if a.Kind != ActionKindSave || a.Class != ActionStructured || s == nil || s.OnHit || s.Ability != DEX || s.DC != 21 || dmgText(s.Damage) != "18d6 fire" {
			t.Fatalf("Fire Breath = %+v, want a structured save: DC 21 Dexterity, 18d6 fire", a)
		}
		if s.OnSuccess != "half" {
			t.Errorf("OnSuccess = %q, want half (corrections.json)", s.OnSuccess)
		}
		if a.Area == nil || a.Area.Shape != AreaCone || a.Area.SizeFt != 60 {
			t.Errorf("area = %+v, want a 60-foot cone", a.Area)
		}
		if a.Usage.Kind != UsageRecharge || a.Usage.RechargeMin != 5 {
			t.Errorf("usage = %+v, want Recharge 5-6", a.Usage)
		}
	})
	t.Run("a line breath weapon keeps its width", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "adult-blue-dragon", "Lightning Breath")
		if a.Area == nil || a.Area.Shape != AreaLine || a.Area.SizeFt != 90 || a.Area.WidthFt != 5 || a.Save == nil || a.Save.OnSuccess != "half" {
			t.Errorf("Lightning Breath = %+v %+v, want a 90-foot line 5 feet wide, half on a success", a.Area, a.Save)
		}
	})
	t.Run("a bite that grapples", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "giant-toad", "Bite")
		if got := dmgText(a.Damage); got != "1d10+2 piercing + 1d10 poison" {
			t.Errorf("damage = %q, want both parts", got)
		}
		h := a.HitCondition
		if h == nil || h.ConditionKey != "condition:grappled" || h.EscapeDC != 13 || h.MaxSize != "" {
			t.Errorf("hit condition = %+v, want grappled, escape DC 13, any size", h)
		}
	})
	t.Run("a grapple keeps its escape DC and its biggest target", func(t *testing.T) {
		t.Parallel()
		h := planActionOf(t, c, "chuul", "Pincer").HitCondition
		if h == nil || h.MaxSize != "Large" || h.EscapeDC != 14 {
			t.Errorf("chuul hit condition = %+v, want escape DC 14 for a Large or smaller target", h)
		}
	})
	t.Run("a thrown weapon is an attack in melee and at range", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "goblin", "Shortbow")
		if a.Attack == nil || a.Attack.Melee || a.Attack.RangeFt != 80 || a.Attack.LongRangeFt != 320 {
			t.Errorf("Shortbow = %+v, want a ranged attack 80/320", a.Attack)
		}
		a = planActionOf(t, c, "bugbear", "Javelin")
		if a.Attack == nil || !a.Attack.Melee || a.Attack.ReachFt != 5 || a.Attack.RangeFt != 30 || a.Attack.LongRangeFt != 120 {
			t.Errorf("Javelin = %+v, want melee reach 5 and range 30/120", a.Attack)
		}
	})
	t.Run("a two-handed damage stays in the text, not in the plan", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "hobgoblin", "Longsword")
		if got := dmgText(a.Damage); got != "1d8+1 slashing" || a.Class != ActionStructured {
			t.Errorf("Longsword = %q %s, want 1d8+1 slashing, structured", got, a.Class)
		}
	})
	t.Run("an action with no attack roll and no save is a reminder", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "ancient-brass-dragon", "Change Shape")
		if a.Kind != ActionKindOther || a.Class != ActionText || a.Attack != nil || a.Save != nil || a.Area != nil {
			t.Errorf("Change Shape = %+v, want text only", a)
		}
	})
	t.Run("a Multiattack keeps its routines, with the action of each step", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "adult-red-dragon", "Multiattack")
		if a.Kind != ActionKindMultiattack || len(a.Routines) != 1 || len(a.Routines[0]) != 3 {
			t.Fatalf("Multiattack = %+v, want one routine of three steps", a.Routines)
		}
		step := a.Routines[0][2]
		if step.Name != "Claw" || step.Count != 2 || step.ActionKey != "monster:adult-red-dragon#claw" {
			t.Errorf("step 3 = %+v, want 2 claws", step)
		}
	})
	t.Run("a step that casts has no action of its own", func(t *testing.T) {
		t.Parallel()
		a := planActionOf(t, c, "glabrezu", "Multiattack")
		step := a.Routines[1][1]
		if step.Name != "Innate Spellcasting" || step.ActionKey != "" || step.Kind != "magic" {
			t.Errorf("step = %+v, want Innate Spellcasting as a magic step with no action", step)
		}
	})
	t.Run("the plan is found by the key of the action", func(t *testing.T) {
		t.Parallel()
		if _, ok := c.MonsterActionPlan("monster:ghoul#nope"); ok {
			t.Error("an unknown action was found")
		}
		if _, ok := c.MonsterActionPlan("monster:nope#claws"); ok {
			t.Error("an unknown creature was found")
		}
		if _, ok := c.MonsterActionPlan("monster:ghoul"); ok {
			t.Error("a key with no action was found")
		}
	})
}

// TestMonsterPlanSavesFollowTheText: every action of the SRD that deals damage
// on a failed save halves it on a success when the text says so; the snapshot's
// four exceptions are corrected (effects/corrections.json).
func TestMonsterPlanSavesFollowTheText(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	checked := 0
	for _, e := range mustList(t, c, CreatureFilter{}) {
		m := c.c.monsters[e.Key]
		p, _ := c.MonsterPlan(e.Key)
		for i, a := range p.Actions {
			if a.Kind != ActionKindSave || a.Save == nil || len(a.Save.Damage) == 0 {
				continue
			}
			text := m.Actions[i].Desc
			halves := strings.Contains(text, "half as much damage on a success") || strings.Contains(text, "takes half as much damage") ||
				strings.Contains(text, "takes half the") || strings.Contains(text, "only half the damage")
			if halves != (a.Save.OnSuccess == "half") {
				t.Errorf("%s %s: OnSuccess = %q but the text %s halves", e.Key, a.Name, a.Save.OnSuccess, map[bool]string{true: "says it", false: "does not say it"}[halves])
			}
			checked++
		}
	}
	if checked < 50 {
		t.Errorf("checked %d save actions with damage; want the 60 or so of the SRD", checked)
	}
}

// The classes of the 334 creatures' actions: a change to a pattern that moves
// an action from one class to another fails TestMonsterPlanEveryCreature, so
// the move is seen and chosen.
const (
	wantActions    = 884
	wantStructured = 721
	wantPartial    = 94
	wantText       = 69
	wantMultiatk   = 148
)

// TestMonsterPlanEveryCreature: the invariants of the plan over the 334
// creatures, and how many actions fall in each class.
func TestMonsterPlanEveryCreature(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	classes := map[string]int{}
	var multi, total int
	for _, e := range mustList(t, c, CreatureFilter{}) {
		p, ok := c.MonsterPlan(e.Key)
		if !ok {
			t.Fatalf("%s has no plan", e.Key)
		}
		m := c.c.monsters[e.Key]
		if len(p.Actions) != len(m.Actions) {
			t.Errorf("%s: %d planned actions for %d", e.Key, len(p.Actions), len(m.Actions))
		}
		for _, a := range p.Actions {
			total++
			classes[a.Class]++
			if a.Kind == ActionKindMultiattack {
				multi++
			}
			if (a.Kind == ActionKindAttack) != (a.Attack != nil) {
				t.Errorf("%s %s: kind %s with attack %v", e.Key, a.Name, a.Kind, a.Attack)
			}
			if (a.Kind == ActionKindSave) && a.Save == nil {
				t.Errorf("%s %s: a save action with no save", e.Key, a.Name)
			}
			if (a.Kind == ActionKindOther) != (a.Class == ActionText) {
				t.Errorf("%s %s: kind %s with class %s", e.Key, a.Name, a.Kind, a.Class)
			}
			if got, ok := c.MonsterActionPlan(a.Key); !ok || got.Name != a.Name {
				t.Errorf("%s: key %s is not found", e.Key, a.Key)
			}
		}
	}
	t.Logf("%d actions: structured %d, partial %d, text %d (multiattack %d)", total, classes[ActionStructured], classes[ActionPartial], classes[ActionText], multi)
	if total != wantActions || classes[ActionStructured] != wantStructured || classes[ActionPartial] != wantPartial || classes[ActionText] != wantText || multi != wantMultiatk {
		t.Errorf("%d actions: structured %d, partial %d, text %d, multiattack %d; want %d: %d, %d, %d, %d",
			total, classes[ActionStructured], classes[ActionPartial], classes[ActionText], multi,
			wantActions, wantStructured, wantPartial, wantText, wantMultiatk)
	}
}

// TestMonsterPlanNamesWhatTheMasterReads pins the partial and text-only actions
// of the creatures of challenge rating 0 to 5 (the ones the first floors use):
// the actions whose text the master still reads. A change in the plan that moves
// an action between classes shows here by name.
func TestMonsterPlanNamesWhatTheMasterReads(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	list, err := c.ListCreatures(CreatureFilter{MaxCR: "5"})
	if err != nil {
		t.Fatal(err)
	}
	partial, text := map[string]bool{}, map[string]bool{}
	for _, e := range list {
		p, _ := c.MonsterPlan(e.Key)
		for _, a := range p.Actions {
			switch a.Class {
			case ActionPartial:
				partial[a.Name] = true
			case ActionText:
				text[a.Name] = true
			}
		}
	}
	const wantPartial = "Acid Spray|Antennae|Beaks|Bite|Bites|Claws|Club|Constrict|Crush|Death Glare|Draining Kiss|Dreadful Glare|Engulf|Glaive|Hand Crossbow|Hooves|Javelin|Luring Song|Petrifying Breath|Possession|Quarterstaff|Shortbow|Smother|Sting|Stomp|Tail|Tendril|Tentacle|Tentacle Slam|Tentacles|Touch|War Pick|Web|Whelm|Whirlwind"
	const wantText = "Breath Weapons|Change Shape|Create Specter|Darkness Aura|Enlarge|Ethereal Stride|Etherealness|Healing Touch|Illusory Appearance|Ink Cloud|Invisibility|Invisible Passage|Leadership|Nightmare Haunting|Read Thoughts|Reel|Swallow|Teleport"
	if got := strings.Join(slices.Sorted(maps.Keys(partial)), "|"); got != wantPartial {
		t.Errorf("partial actions of CR 0 to 5:\n got %s\nwant %s", got, wantPartial)
	}
	if got := strings.Join(slices.Sorted(maps.Keys(text)), "|"); got != wantText {
		t.Errorf("text-only actions of CR 0 to 5:\n got %s\nwant %s", got, wantText)
	}
}

// TestActionSaveSuccessCorrection: the loader refuses a correction of an action's
// save that cannot apply, and writes a good one over the action.
func TestActionSaveSuccessCorrection(t *testing.T) {
	t.Parallel()
	newWyrmContent := func() *content {
		return &content{
			classLevels: map[string][]*srd51.Level{},
			monsters: map[string]*srd51.Monster{"monster:wyrm": {Key: "monster:wyrm", Actions: []srd51.MonsterAction{
				{Name: "Breath", Save: &srd51.MonsterSave{Ability: "dex", DC: 12}}, {Name: "Bite"},
			}}},
		}
	}
	one := func(action, value string) string {
		return `{"creature_corrections":[{"creature":"monster:wyrm","action":"` + action + `","field":"action_save_success","value":` + value + `,"source":"SRD"}]}`
	}
	for name, doc := range map[string]string{
		"unknown action":                    one("Roar", `"half"`),
		"action with no save":               one("Bite", `"half"`),
		"not an outcome":                    one("Breath", `"most"`),
		"not a string":                      one("Breath", `1`),
		"no action named":                   one("", `"half"`),
		"action on a field of the creature": `{"creature_corrections":[{"creature":"monster:wyrm","action":"Breath","field":"hit_points","value":5,"source":"SRD"}]}`,
	} {
		c := newWyrmContent()
		if err := c.applyCorrections(fstest.MapFS{"effects/corrections.json": {Data: []byte(doc)}}); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	c := newWyrmContent()
	if err := c.applyCorrections(fstest.MapFS{"effects/corrections.json": {Data: []byte(one("Breath", `"half"`))}}); err != nil {
		t.Fatal(err)
	}
	if got := c.monsters["monster:wyrm"].Actions[0].Save.OnSuccess; got != "half" {
		t.Errorf("OnSuccess = %q, want half", got)
	}
}
