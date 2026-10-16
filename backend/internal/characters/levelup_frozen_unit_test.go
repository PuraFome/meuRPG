package characters

import (
	"bytes"
	"encoding/json"
	"errors"
	goflag "flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The pure parts of the guided level-up, frozen before levelup.go is split: the
// shape check of the choices, error by error and in the order the checks run, and
// the copy of a level's offer into the message, every field of it.

var updateLevelUpGolden = goflag.Bool("update-levelup-golden", false, "write the golden files of the guided level-up again")

// levelUpGolden compares m, as indented JSON, with testdata/levelup_<name>.golden.json.
// Run with -update-levelup-golden to write the files again after a change of behavior
// that was meant.
func levelUpGolden(t *testing.T, name string, m proto.Message, labels map[string]string) {
	t.Helper()
	raw, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for id, label := range labels {
		raw = bytes.ReplaceAll(raw, []byte(id), []byte(label))
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "levelup_"+name+".golden.json")
	if *updateLevelUpGolden {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file: %v (write it with -update-levelup-golden)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from %s:\n--- got\n%s\n--- want\n%s", name, path, got, want)
	}
}

// levelUpChoicesShape is a set of choices that passes the shape check, for a case to
// break one thing of.
func levelUpChoicesShape() *charactersv1.LevelUpChoices {
	return &charactersv1.LevelUpChoices{
		ClassKey:          "class:wizard",
		CantripKeys:       []string{"spell:light"},
		KnownSpellKeys:    []string{"spell:shield"},
		FeatureChoiceText: map[string]string{},
		HitPoints:         &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_AVERAGE},
	}
}

// keysOfLength is n distinct keys.
func keysOfLength(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "k" + strings.Repeat("x", i)
	}
	return out
}

func TestCheckLevelUpChoicesFrozen(t *testing.T) {
	t.Parallel()
	texts := func(n int) map[string]string {
		out := map[string]string{}
		for i := range n {
			out["feature:f"+strings.Repeat("x", i)] = "texto"
		}
		return out
	}
	tests := []struct {
		name  string
		mod   func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices
		field string // "" for no error
		msg   string
	}{
		{"nil", func(*charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices { return nil }, "choices", "choices is required"},
		{"the shape passes", nil, "", ""},
		{"no hit points at all passes", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices { c.HitPoints = nil; return c }, "", ""},
		{"cantrips over", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.CantripKeys = keysOfLength(rules.MaxCantrips + 1)
			return c
		}, "choices.cantrip_keys", "choices.cantrip_keys must have at most 30 entries"},
		{"cantrips at the limit", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.CantripKeys = keysOfLength(rules.MaxCantrips)
			return c
		}, "", ""},
		{"known spells over", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.KnownSpellKeys = keysOfLength(rules.MaxKnownSpells + 1)
			return c
		}, "choices.known_spell_keys", "choices.known_spell_keys must have at most 200 entries"},
		{"prepared over", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.PreparedSpellKeys = keysOfLength(rules.MaxPreparedSpells + 1)
			return c
		}, "choices.prepared_spell_keys", "choices.prepared_spell_keys must have at most 100 entries"},
		{"feature choices over", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatureChoiceKeys = keysOfLength(rules.MaxListLength + 1)
			return c
		}, "choices.feature_choice_keys", "choices.feature_choice_keys must have at most 100 entries"},
		{"late choices over", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.LateChoiceKeys = keysOfLength(rules.MaxListLength + 1)
			return c
		}, "choices.late_choice_keys", "choices.late_choice_keys must have at most 100 entries"},
		{"skills over", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.SkillProficiencyKeys = keysOfLength(rules.MaxSkillKeys + 1)
			return c
		}, "choices.skill_proficiency_keys", "choices.skill_proficiency_keys must have at most 18 entries"},
		{"expertise over", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.ExpertiseSkillKeys = keysOfLength(rules.MaxSkillKeys + 1)
			return c
		}, "choices.expertise_skill_keys", "choices.expertise_skill_keys must have at most 18 entries"},
		// Every size comes before every repeat: a repeat in an earlier list loses to a size
		// in a later one.
		{"a size beats an earlier repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.CantripKeys = []string{"spell:light", "spell:light"}
			c.ExpertiseSkillKeys = keysOfLength(rules.MaxSkillKeys + 1)
			return c
		}, "choices.expertise_skill_keys", "choices.expertise_skill_keys must have at most 18 entries"},
		{"cantrips repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.CantripKeys = []string{"a", "b", "a"}
			return c
		}, "choices.cantrip_keys", "choices.cantrip_keys repeats an entry"},
		{"known repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.KnownSpellKeys = []string{"a", "a"}
			return c
		}, "choices.known_spell_keys", "choices.known_spell_keys repeats an entry"},
		{"prepared repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.PreparedSpellKeys = []string{"a", "a"}
			return c
		}, "choices.prepared_spell_keys", "choices.prepared_spell_keys repeats an entry"},
		{"feature choices repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatureChoiceKeys = []string{"a", "a"}
			return c
		}, "choices.feature_choice_keys", "choices.feature_choice_keys repeats an entry"},
		{"late choices repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.LateChoiceKeys = []string{"a", "a"}
			return c
		}, "choices.late_choice_keys", "choices.late_choice_keys repeats an entry"},
		{"skills repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.SkillProficiencyKeys = []string{"a", "a"}
			return c
		}, "choices.skill_proficiency_keys", "choices.skill_proficiency_keys repeats an entry"},
		{"expertise repeat", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.ExpertiseSkillKeys = []string{"a", "a"}
			return c
		}, "choices.expertise_skill_keys", "choices.expertise_skill_keys repeats an entry"},
		{"a repeat beats the texts", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.ExpertiseSkillKeys = []string{"a", "a"}
			c.FeatureChoiceText = texts(rules.MaxChoiceTexts + 1)
			return c
		}, "choices.expertise_skill_keys", "choices.expertise_skill_keys repeats an entry"},
		{"too many texts", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatureChoiceText = texts(rules.MaxChoiceTexts + 1)
			return c
		}, "choices.feature_choice_text", "choices.feature_choice_text must have at most 12 entries"},
		{"texts at the limit", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatureChoiceText = texts(rules.MaxChoiceTexts)
			return c
		}, "", ""},
		{"a text too long", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatureChoiceText = map[string]string{"feature:a": strings.Repeat("a", rules.MaxChoiceTextLength+1)}
			return c
		}, "choices.feature_choice_text", "choices.feature_choice_text must be at most 40 characters, got 41"},
		{"a text with a control character", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatureChoiceText = map[string]string{"feature:a": "a\x07b"}
			return c
		}, "choices.feature_choice_text", "choices.feature_choice_text must not contain the character U+0007"},
		{"a text beats the swap key", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatureChoiceText = map[string]string{"feature:a": strings.Repeat("a", rules.MaxChoiceTextLength+1)}
			c.SwapInvocationKey = strings.Repeat("k", maxChoiceKeyLength+1)
			return c
		}, "choices.feature_choice_text", "choices.feature_choice_text must be at most 40 characters, got 41"},
		{"the swap key too long", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.SwapInvocationKey = strings.Repeat("k", maxChoiceKeyLength+1)
			return c
		}, "choices.swap_invocation_key", "choices.swap_invocation_key must be at most 120 characters"},
		{"the swap key at the limit", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.SwapInvocationKey = strings.Repeat("k", maxChoiceKeyLength)
			return c
		}, "", ""},
		{"the swap key beats the feat key", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.SwapInvocationKey = strings.Repeat("k", maxChoiceKeyLength+1)
			c.FeatKey = strings.Repeat("k", maxContentKeyLength+1)
			return c
		}, "choices.swap_invocation_key", "choices.swap_invocation_key must be at most 120 characters"},
		{"the feat key too long", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatKey = strings.Repeat("k", maxContentKeyLength+1)
			return c
		}, "choices.feat_key", "choices.feat_key must be at most 128 characters"},
		{"the feat key at the limit", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatKey = strings.Repeat("k", maxContentKeyLength)
			return c
		}, "", ""},
		{"the feat key beats the method", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.FeatKey = strings.Repeat("k", maxContentKeyLength+1)
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: 99}
			return c
		}, "choices.feat_key", "choices.feat_key must be at most 128 characters"},
		{"an unknown method", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: 99}
			return c
		}, "choices.hit_points.method", "choices.hit_points.method is not a known method"},
		{"the method beats the value", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: 99, Value: -1}
			return c
		}, "choices.hit_points.method", "choices.hit_points.method is not a known method"},
		{"the unspecified method passes", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.HitPoints = &charactersv1.LevelUpHitPoints{}
			return c
		}, "", ""},
		{"a negative value", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.HitPoints.Value = -1
			return c
		}, "choices.hit_points.value", "choices.hit_points.value must be 0 to 1000"},
		{"a value over 1000", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.HitPoints.Value = 1001
			return c
		}, "choices.hit_points.value", "choices.hit_points.value must be 0 to 1000"},
		{"a value of 1000", func(c *charactersv1.LevelUpChoices) *charactersv1.LevelUpChoices {
			c.HitPoints.Value = 1000
			return c
		}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := levelUpChoicesShape()
			if tt.mod != nil {
				c = tt.mod(c)
			}
			err := checkLevelUpChoices(c)
			if tt.field == "" {
				if err != nil {
					t.Fatalf("checkLevelUpChoices() = %v, want nil", err)
				}
				return
			}
			fe, ok := errors.AsType[*fieldError](err)
			if !ok || fe.field != tt.field || err.Error() != tt.msg {
				t.Errorf("checkLevelUpChoices() = %#v (%v), want %s: %q", err, err, tt.field, tt.msg)
			}
		})
	}
}

// The texts are cleaned in place: a blank one is dropped, the rest trimmed and
// normalized, in the request itself (the plan and the record read the cleaned map).
func TestCheckLevelUpChoicesCleansTheTextsInPlace(t *testing.T) {
	t.Parallel()
	c := levelUpChoicesShape()
	c.FeatureChoiceText = map[string]string{"feature:a": "  Dragões  ", "feature:b": "   ", "feature:c": "e\u0301"}
	if err := checkLevelUpChoices(c); err != nil {
		t.Fatalf("checkLevelUpChoices() = %v", err)
	}
	want := map[string]string{"feature:a": "Dragões", "feature:c": "\u00e9"}
	if len(c.GetFeatureChoiceText()) != len(want) {
		t.Fatalf("texts = %q, want %q", c.GetFeatureChoiceText(), want)
	}
	for k, v := range want {
		if c.GetFeatureChoiceText()[k] != v {
			t.Errorf("texts[%s] = %q, want %q", k, c.GetFeatureChoiceText()[k], v)
		}
	}
	// A nil map with nothing in it is left nil.
	c = levelUpChoicesShape()
	c.FeatureChoiceText = nil
	if err := checkLevelUpChoices(c); err != nil || c.FeatureChoiceText != nil {
		t.Errorf("checkLevelUpChoices(no texts) = %v, texts %v; want nil, nil", err, c.FeatureChoiceText)
	}
}

// levelUpOfferEveryField is an offer with something in every field
// levelUpOptionsToProto copies, and every kind of nested part.
func levelUpOfferEveryField() rules.LevelUpOffer {
	named := func(prefix string, n int) []rules.NamedKey {
		var out []rules.NamedKey
		for i := range n {
			out = append(out, rules.NamedKey{Key: prefix + ":" + strings.Repeat("i", i+1), NamePT: "Nome " + prefix + strings.Repeat("i", i+1)})
		}
		return out
	}
	choice := rules.LevelUpFeatureChoice{
		Feature: rules.NamedKey{Key: "feature:style", NamePT: "Estilo"}, Subclass: "subclass:x", Choose: 2,
		Options: named("feature:opt", 2),
		Blocked: []rules.LevelUpBlockedOption{{Key: "feature:no", NamePT: "Não", ReasonPT: "Já tem"}, {Key: "feature:no2", NamePT: "Não 2", ReasonPT: "Outra razão"}},
	}
	return rules.LevelUpOffer{
		Class: "class:warlock", ClassNamePT: "Bruxo", FromLevel: 2, ToLevel: 3, TotalFrom: 5, TotalTo: 6,
		NewClass: true,
		Multiclass: &rules.MulticlassOffer{
			Proficiencies: []rules.MulticlassProficiency{{Key: "armor:light", NamePT: "Leve", Kind: "armor", AlreadyHave: true}, {Key: "weapon:simple", NamePT: "Simples", Kind: "weapon"}},
			Skills:        &rules.MulticlassPick{Choose: 1, From: []rules.MulticlassOption{{Key: "skill:arcana", NamePT: "Arcanismo", AlreadyHave: true}, {Key: "skill:history", NamePT: "História"}}},
			Instruments:   &rules.MulticlassPick{Choose: 1, From: []rules.MulticlassOption{{Key: "instrument:lute", NamePT: "Alaúde"}}},
		},
		HitDie: 8, HitPointAverage: 5, AbilityScoreImprovement: true,
		SubclassDue: true,
		Subclasses: []rules.LevelUpSubclass{
			{
				Key: "subclass:fiend", NamePT: "Corruptor", FeatureChoices: []rules.LevelUpFeatureChoice{choice},
				Cantrips: 1, SkillChoices: 2, ExpertiseChoices: 3, Spells: 4, SpellsKind: rules.PreparationKnown,
				SpellList: "class:wizard", MaxSpellLevel: 2, Prepares: true, PreparedMaxAfter: 7, Archived: true, Off: true,
			},
			{Key: "subclass:archfey", NamePT: "Arquifada", SpellsKind: rules.PreparationSpellbook},
		},
		Cantrips: 1, Spells: 2, SpellsKind: rules.PreparationSpellbook, SpellList: "class:warlock", MaxSpellLevel: 2,
		AnyClassSpells: 1, Prepares: true, PreparedMax: 3, PreparedMaxAfter: 4,
		FeatureChoices: []rules.LevelUpFeatureChoice{choice, {Feature: rules.NamedKey{Key: "feature:bare"}, Choose: 1}},
		SkillChoices:   1, ExpertiseChoices: 2,
		ProficiencyBonusBefore: 3, ProficiencyBonusAfter: 3,
		SlotsBefore: [9]int{4, 3, 2}, SlotsAfter: [9]int{4, 3, 3},
		PactBefore: &rules.PactMagic{SlotLevel: 1, Slots: 1}, PactAfter: &rules.PactMagic{SlotLevel: 2, Slots: 2},
		NewFeatures: named("feature:new", 2), MasterAdds: named("feature:master", 1),
		LateChoices:       []rules.ChoiceGroup{{Origin: rules.ChoiceOriginRace, SourceKey: "race:elf", SourceNamePT: "Elfo", Level: 1}},
		NewChoices:        []rules.ChoiceGroup{{Origin: rules.ChoiceOriginRace, SourceKey: "race:dwarf", SourceNamePT: "Anão", Level: 2}},
		CanSwapInvocation: true,
	}
}

// TestLevelUpOptionsToProtoFrozen: every field of the offer reaches the message,
// and an offer with nothing in it gives the empty lists and nil parts it gives today.
func TestLevelUpOptionsToProtoFrozen(t *testing.T) {
	t.Parallel()
	levelUpGolden(t, "options_to_proto_every_field", levelUpOptionsToProto(levelUpOfferEveryField()), nil)
	levelUpGolden(t, "options_to_proto_empty", levelUpOptionsToProto(rules.LevelUpOffer{}), nil)

	// The empty offer: nil lists (not empty ones), nil pact magic, and nine zero slots each side.
	empty := levelUpOptionsToProto(rules.LevelUpOffer{})
	if empty.NewFeatures != nil || empty.MasterAdds != nil || empty.FeatureChoices != nil || empty.Subclasses != nil ||
		empty.ProficiencyGains != nil || empty.ProficiencyChoices != nil || empty.PactMagicBefore != nil || empty.PactMagicAfter != nil {
		t.Errorf("empty offer = %v, want nil lists and parts", empty)
	}
	if !slices.Equal(empty.GetSpellSlotsBefore(), make([]int32, 9)) || !slices.Equal(empty.GetSpellSlotsAfter(), make([]int32, 9)) {
		t.Errorf("empty slots = %v, %v, want nine zeros each", empty.GetSpellSlotsBefore(), empty.GetSpellSlotsAfter())
	}
	// A multiclass offer without picks gains only the proficiencies.
	o := levelUpOfferEveryField()
	o.Multiclass.Skills, o.Multiclass.Instruments = nil, nil
	if got := levelUpOptionsToProto(o); len(got.GetProficiencyChoices()) != 0 || len(got.GetProficiencyGains()) != 2 {
		t.Errorf("no picks = %v choices, %v gains; want none and 2", got.GetProficiencyChoices(), got.GetProficiencyGains())
	}
}
