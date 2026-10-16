package maps

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// cleanRules is the rules module as cleanTrap and cleanSave read it: one trap preset, one
// skill besides Perception and Investigation, and names for the keys of damage types and
// conditions.
type cleanRules struct{ noRules }

func (cleanRules) TrapPreset(key string) (rules.TrapPreset, bool) {
	return rules.TrapPreset{}, key == "trap:pit"
}

func (cleanRules) IsSkill(key string) bool { return key == "skill:arcana" || key == "skill:history" }

func (cleanRules) NamePT(key string) string {
	if key == "damage-type:fire" || key == "condition:poisoned" {
		return "nome"
	}
	return ""
}

// wantTrapSpecRefusal checks that a clean* function refused with invalid_argument and
// this exact message.
func wantTrapSpecRefusal(t *testing.T, call string, err error, msg string) {
	t.Helper()
	if connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.HasSuffix(err.Error(), ": "+msg) {
		t.Errorf("%s error = %v, want invalid_argument %q", call, err, msg)
	}
}

// validCleanTrap is a trap cleanTrap accepts, with a save that can fail.
func validCleanTrap() *mapsv1.TrapSpec {
	return &mapsv1.TrapSpec{
		NoticeDc: 12, FindDc: 14, AreaSize: 2, Trigger: rulesv1.TrapTrigger_TRAP_TRIGGER_ENTER,
		Effect: &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{{Dice: "2d6", DamageTypeKey: "damage-type:fire"}}},
	}
}

// cleanTrap checks the trap in a fixed order: the first fault is the one read, so the cases
// with two faults pin which check runs first.
func TestCleanTrapRefusals(t *testing.T) {
	t.Parallel()
	s := &Service{rules: cleanRules{}}
	armed, bogus := "armed", mapsv1.TrapState(99)
	cases := []struct {
		name string
		edit func(*mapsv1.TrapSpec)
		want string
	}{
		{"an unknown preset", func(in *mapsv1.TrapSpec) { in.PresetKey = "trap:nope" }, "trap.preset_key is not a trap preset"},
		{"a preset before the DCs", func(in *mapsv1.TrapSpec) { in.PresetKey, in.NoticeDc = "trap:nope", 31 }, "trap.preset_key is not a trap preset"},
		{"a negative notice DC", func(in *mapsv1.TrapSpec) { in.NoticeDc = -1 }, "trap.notice_dc must be 0 (none) or 1 to 30"},
		{"a notice DC of 31", func(in *mapsv1.TrapSpec) { in.NoticeDc = 31 }, "trap.notice_dc must be 0 (none) or 1 to 30"},
		{"the notice DC before the find DC", func(in *mapsv1.TrapSpec) { in.NoticeDc, in.FindDc = 31, 0 }, "trap.notice_dc must be 0 (none) or 1 to 30"},
		{"a find DC of 0", func(in *mapsv1.TrapSpec) { in.FindDc = 0 }, "trap.find_dc must be 1 to 30"},
		{"a find DC of 31", func(in *mapsv1.TrapSpec) { in.FindDc = 31 }, "trap.find_dc must be 1 to 30"},
		{"the find DC before the area", func(in *mapsv1.TrapSpec) { in.FindDc, in.AreaSize = 0, 0 }, "trap.find_dc must be 1 to 30"},
		{"an area of 0", func(in *mapsv1.TrapSpec) { in.AreaSize = 0 }, "trap.area_size must be 1 to 4"},
		{"an area of 5", func(in *mapsv1.TrapSpec) { in.AreaSize = 5 }, "trap.area_size must be 1 to 4"},
		{"the area before the trigger", func(in *mapsv1.TrapSpec) { in.AreaSize, in.Trigger = 5, 0 }, "trap.area_size must be 1 to 4"},
		{"no trigger", func(in *mapsv1.TrapSpec) { in.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_UNSPECIFIED }, "trap.trigger is required"},
		{"an unknown trigger", func(in *mapsv1.TrapSpec) { in.Trigger = 99 }, "trap.trigger is required"},
		{"an unknown state", func(in *mapsv1.TrapSpec) { in.State = bogus }, "trap.state is not a trap state"},
		{"the state before the effect", func(in *mapsv1.TrapSpec) {
			in.State, in.Effect = bogus, &rulesv1.TrapEffect{Targets: 99}
		}, "trap.state is not a trap state"},
		{"an effect with bad targets", func(in *mapsv1.TrapSpec) { in.Effect = &rulesv1.TrapEffect{Targets: 99} }, "trap.effect.targets is not a valid value"},
		{"the effect before the skills", func(in *mapsv1.TrapSpec) {
			in.Effect, in.AlsoFindSkillKeys = &rulesv1.TrapEffect{Targets: 99}, []string{"skill:nope"}
		}, "trap.effect.targets is not a valid value"},
		{"a skill every trap has", func(in *mapsv1.TrapSpec) { in.AlsoFindSkillKeys = []string{skillPerception} }, "trap.also_find_skill_keys[0] is a skill every trap can be found with"},
		{"a skill outside the SRD", func(in *mapsv1.TrapSpec) { in.AlsoFindSkillKeys = []string{"skill:arcana", "skill:nope"} }, "trap.also_find_skill_keys[1] is not a skill of the SRD"},
		{"a repeated skill", func(in *mapsv1.TrapSpec) { in.AlsoFindSkillKeys = []string{"skill:arcana", "skill:arcana"} }, "trap.also_find_skill_keys[1] repeats a skill"},
	}
	for _, c := range cases {
		in := validCleanTrap()
		c.edit(in)
		_, err := s.cleanTrap(in, &armed)
		wantTrapSpecRefusal(t, c.name, err, c.want)
	}
	if _, err := s.cleanTrap(nil, nil); connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.HasSuffix(err.Error(), "trap is required for a TRAP point") {
		t.Errorf("a missing trap error = %v, want invalid_argument", err)
	}
}

// What cleanTrap keeps: the canonical copy without the output-only parts, the state a new
// trap starts with, the one an edit keeps, and the one an edit names.
func TestCleanTrapKeepsTheCanonicalCopyAndTheState(t *testing.T) {
	t.Parallel()
	s := &Service{rules: cleanRules{}}
	triggered := "triggered"
	cases := []struct {
		name    string
		in      func(*mapsv1.TrapSpec)
		current *string
		state   string
	}{
		{"a new trap starts armed", func(*mapsv1.TrapSpec) {}, nil, "armed"},
		{"an edit that names no state keeps the current one", func(*mapsv1.TrapSpec) {}, &triggered, "triggered"},
		{"an edit names armed", func(in *mapsv1.TrapSpec) { in.State = mapsv1.TrapState_TRAP_STATE_ARMED }, &triggered, "armed"},
		{"an edit names disarmed", func(in *mapsv1.TrapSpec) { in.State = mapsv1.TrapState_TRAP_STATE_DISARMED }, nil, "disarmed"},
		{"an edit names triggered", func(in *mapsv1.TrapSpec) { in.State = mapsv1.TrapState_TRAP_STATE_TRIGGERED }, nil, "triggered"},
	}
	for _, c := range cases {
		in := validCleanTrap()
		c.in(in)
		in.PresetKey = "trap:pit"
		in.AlsoFindSkillKeys = []string{"skill:history", "skill:arcana"}
		in.Effect.Damage[0].Dice = "2d6"
		got, err := s.cleanTrap(in, c.current)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.state != c.state {
			t.Errorf("%s: state = %q, want %q", c.name, got.state, c.state)
		}
		var back mapsv1.TrapSpec
		if err := protojson.Unmarshal(got.json, &back); err != nil {
			t.Fatalf("%s: the stored trap is not JSON: %v", c.name, err)
		}
		want := &mapsv1.TrapSpec{
			PresetKey: "trap:pit", NoticeDc: 12, FindDc: 14, AreaSize: 2, Trigger: rulesv1.TrapTrigger_TRAP_TRIGGER_ENTER,
			AlsoFindSkillKeys: []string{"skill:history", "skill:arcana"},
			Effect: &rulesv1.TrapEffect{
				Targets: rulesv1.TrapTargets_TRAP_TARGETS_AREA,
				Damage:  []*rulesv1.TrapDamage{{Dice: "2d6", DamageTypeKey: "damage-type:fire"}},
			},
		}
		if !proto.Equal(&back, want) {
			t.Errorf("%s: stored trap = %v, want %v", c.name, &back, want)
		}
		if strings.Contains(string(got.json), "state") {
			t.Errorf("%s: the stored trap carries its state: %s", c.name, got.json)
		}
	}
	// A manual trap, a notice DC of none and the largest values are accepted.
	edge := validCleanTrap()
	edge.NoticeDc, edge.FindDc, edge.AreaSize, edge.Trigger = 0, 30, 4, rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL
	if _, err := s.cleanTrap(edge, nil); err != nil {
		t.Errorf("the edge values were refused: %v", err)
	}
}

// cleanSave checks the save in a fixed order too.
func TestCleanSaveRefusalsAndResult(t *testing.T) {
	t.Parallel()
	s := &Service{rules: cleanRules{}}
	fire := []*rulesv1.TrapDamage{{Dice: "1d8", DamageTypeKey: "damage-type:fire"}}
	valid := func() *rulesv1.TrapSaveEffect {
		return &rulesv1.TrapSaveEffect{
			Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 12, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
			OnFail: &rulesv1.TrapOnFail{Damage: fire}, OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF,
		}
	}
	cases := []struct {
		name      string
		edit      func(*rulesv1.TrapSaveEffect)
		hasAttack bool
		want      string
	}{
		{"no ability", func(sv *rulesv1.TrapSaveEffect) { sv.Ability = rulesv1.Ability_ABILITY_UNSPECIFIED }, false, "trap.effect.save.ability is required"},
		{"the ability before the DC", func(sv *rulesv1.TrapSaveEffect) { sv.Ability, sv.Dc = 0, 0 }, false, "trap.effect.save.ability is required"},
		{"a DC of 0", func(sv *rulesv1.TrapSaveEffect) { sv.Dc = 0 }, false, "trap.effect.save.dc must be 1 to 30"},
		{"a DC of 31", func(sv *rulesv1.TrapSaveEffect) { sv.Dc = 31 }, false, "trap.effect.save.dc must be 1 to 30"},
		{"the DC before applies_to", func(sv *rulesv1.TrapSaveEffect) { sv.Dc, sv.AppliesTo = 0, 0 }, false, "trap.effect.save.dc must be 1 to 30"},
		{"no applies_to", func(sv *rulesv1.TrapSaveEffect) { sv.AppliesTo = rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_UNSPECIFIED }, true, "trap.effect.save.applies_to is required"},
		{"HIT with no attack", func(sv *rulesv1.TrapSaveEffect) { sv.AppliesTo = rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT }, false, "trap.effect.save.applies_to HIT needs an attack"},
		{"HIT before on_pass", func(sv *rulesv1.TrapSaveEffect) {
			sv.AppliesTo, sv.OnPass = rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT, 0
		}, false, "trap.effect.save.applies_to HIT needs an attack"},
		{"no on_pass", func(sv *rulesv1.TrapSaveEffect) { sv.OnPass = rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_UNSPECIFIED }, false, "trap.effect.save.on_pass is required"},
		{"on_pass before the damage", func(sv *rulesv1.TrapSaveEffect) {
			sv.OnPass, sv.OnFail = 0, &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "bad"}}}
		}, false, "trap.effect.save.on_pass is required"},
		{"bad damage on a failure", func(sv *rulesv1.TrapSaveEffect) {
			sv.OnFail = &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "bad", DamageTypeKey: "damage-type:fire"}}}
		}, false, "trap.effect.save.on_fail.damage[0].dice is not dice such as \"2d6\" or a flat number"},
		{"a bad condition on a failure", func(sv *rulesv1.TrapSaveEffect) {
			sv.OnFail = &rulesv1.TrapOnFail{Damage: fire, Condition: &rulesv1.TrapCondition{ConditionKey: "condition:nope"}}
		}, false, "trap.effect.save.on_fail.condition.condition_key is not a condition"},
		{"a failure that does nothing", func(sv *rulesv1.TrapSaveEffect) {
			sv.OnFail, sv.OnPass = &rulesv1.TrapOnFail{}, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE
		}, false, "trap.effect.save.on_fail must do damage or give a condition"},
		{"nothing on a failure before half", func(sv *rulesv1.TrapSaveEffect) { sv.OnFail = nil }, false, "trap.effect.save.on_fail must do damage or give a condition"},
		{"half with only a condition", func(sv *rulesv1.TrapSaveEffect) {
			sv.OnFail = &rulesv1.TrapOnFail{Condition: &rulesv1.TrapCondition{ConditionKey: "condition:poisoned"}}
		}, false, "trap.effect.save.on_pass HALF needs damage on a failure"},
	}
	for _, c := range cases {
		sv := valid()
		c.edit(sv)
		_, err := s.cleanSave(sv, c.hasAttack)
		wantTrapSpecRefusal(t, c.name, err, c.want)
	}

	// What it accepts: the canonical copy, with the duration trimmed.
	sv := valid()
	sv.AppliesTo = rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT
	sv.OnFail = &rulesv1.TrapOnFail{Damage: fire, Condition: &rulesv1.TrapCondition{ConditionKey: "condition:poisoned", DurationPt: "  1 hora  "}}
	got, err := s.cleanSave(sv, true)
	want := &rulesv1.TrapSaveEffect{
		Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 12, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT, OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF,
		OnFail: &rulesv1.TrapOnFail{Damage: fire, Condition: &rulesv1.TrapCondition{ConditionKey: "condition:poisoned", DurationPt: "1 hora"}},
	}
	if err != nil || !proto.Equal(got, want) {
		t.Errorf("cleanSave() = %v, %v, want %v", got, err, want)
	}
	// A condition alone is a failure that does something, and none passes with no damage.
	sv = valid()
	sv.OnFail = &rulesv1.TrapOnFail{Condition: &rulesv1.TrapCondition{ConditionKey: "condition:poisoned"}}
	sv.OnPass = rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE
	got, err = s.cleanSave(sv, false)
	if err != nil || got.GetOnFail().GetCondition().GetConditionKey() != "condition:poisoned" || len(got.GetOnFail().GetDamage()) != 0 {
		t.Errorf("a save that gives a condition = %v, %v", got, err)
	}
	// Both of the edge values: DC 1 and 30.
	for _, dc := range []int32{1, 30} {
		sv = valid()
		sv.Dc = dc
		if _, err := s.cleanSave(sv, false); err != nil {
			t.Errorf("DC %d was refused: %v", dc, err)
		}
	}
}
