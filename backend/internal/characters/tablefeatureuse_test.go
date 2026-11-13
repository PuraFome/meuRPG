package characters

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

func storedUse() *rulesv1.FeatureUse {
	return &rulesv1.FeatureUse{
		LimitScope: rulesv1.UseScope_USE_SCOPE_COMBAT, LimitUses: "prof()",
		Targets: &rulesv1.FeatureTargets{Min: 1, Max: 2, Who: rulesv1.FeatureTargetWho_FEATURE_TARGET_WHO_ALLY, RangeFt: 30},
		Choices: []*rulesv1.FeatureChoice{{Key: "fogo", TextPt: "Fogo"}, {Key: "gelo", TextPt: "Gelo"}},
		Applies: &rulesv1.FeatureApplies{
			Heal: "1d6", TempHp: "level()", EffectRounds: 10,
			Effect: &rulesv1.TableSpellEffect{Modifiers: []*rulesv1.TableEffectModifier{{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_TEMP_HP, Value: 3}}},
		},
	}
}

// TestAFeatureUseSurvivesTheJSONOfAPackage: a table effect with `use` goes through the JSON of the package
// (export and import) and becomes the engine's use.
func TestAFeatureUseSurvivesTheJSONOfAPackage(t *testing.T) {
	t.Parallel()
	in := &rulesv1.TableEffect{Type: "grant_action", Economy: "bonus_action", Use: storedUse()}
	raw, err := protojson.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out := &rulesv1.TableEffect{}
	if err := protojson.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(in, out) {
		t.Fatalf("round trip = %v, want %v", out, in)
	}
	got := tableEffectsOf([]*rulesv1.TableEffect{out})[0].Use
	want := &rules.FeatureUse{
		LimitScope: rules.UseScopeCombat, LimitUses: "prof()",
		Targets: &rules.FeatureTargets{Min: 1, Max: 2, Who: rules.FeatureWhoAlly, RangeFt: 30},
		Choices: []rules.FeatureChoice{{Key: "fogo", TextPT: "Fogo"}, {Key: "gelo", TextPT: "Gelo"}},
		Applies: &rules.FeatureApplies{Heal: "1d6", TempHP: "level()", EffectRounds: 10, Effect: got.Applies.Effect},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("use = %+v, want %+v", got, want)
	}
	if e := got.Applies.Effect; e == nil || len(e.Modifiers) != 1 || e.Modifiers[0].Kind != rules.ModifierTempHP || e.Modifiers[0].Value != 3 {
		t.Errorf("effect = %+v", e)
	}
	if tableEffectsOf([]*rulesv1.TableEffect{{Type: "grant_action", Economy: "action"}})[0].Use != nil {
		t.Error("an effect without use got one")
	}
}

func TestAnUnknownScopeAndWhoReachTheRulesToBeRefused(t *testing.T) {
	t.Parallel()
	u := featureUseOf(&rulesv1.FeatureUse{LimitScope: 99, LimitUses: "1", Targets: &rulesv1.FeatureTargets{Max: 1, Who: 99}})
	if u.LimitScope != "unknown" || u.Targets.Who != "unknown" {
		t.Errorf("use = %+v, want unknown words, not the defaults", u)
	}
}

// TestAnActionThatDeclaresAUseReachesTheTurnOptions: the option says it is taken with UseFeature, with the
// scope, the targets and the choices; an action without a use is as before.
func TestAnActionThatDeclaresAUseReachesTheTurnOptions(t *testing.T) {
	t.Parallel()
	act := rules.Action{Key: "feature:x@mesa", Economy: rules.EconomyBonusAction, Use: &rules.ActionUse{
		Scope: rules.UseScopeRound, Uses: 2, Targets: rules.FeatureTargets{Min: 1, Max: 3, Who: rules.FeatureWhoEnemy, RangeFt: 60},
		Choices: []rules.FeatureChoice{{Key: "a", TextPT: "A"}}, Result: rules.ActionResult{TempHP: 4, Heal: &rules.DiceFormula{Count: 1, Sides: 6}},
		HasEffect: true, EffectRounds: 5,
	}}
	o := actionOptionToProto(combat.ActionOption{Action: act, Option: combat.Option{Enabled: true}})
	if !o.GetViaUseFeature() || o.GetUseScope() != rulesv1.UseScope_USE_SCOPE_ROUND || o.GetTargetMin() != 1 || o.GetTargetMax() != 3 ||
		o.GetTargetWho() != rulesv1.FeatureTargetWho_FEATURE_TARGET_WHO_ENEMY || o.GetTargetRangeFt() != 60 ||
		len(o.GetChoices()) != 1 || o.GetChoices()[0].GetKey() != "a" || o.GetChoices()[0].GetTextPt() != "A" {
		t.Errorf("option = %v", o)
	}
	if o.GetUsesLeft() != 0 {
		t.Errorf("uses_left = %d: the scope's uses are the play module's to fill", o.GetUsesLeft())
	}
	fa := featureActionOf(rules.Derived{}, act)
	if fa.Use == nil || fa.Use.Scope != "round" || fa.Use.Uses != 2 || fa.Use.TargetMax != 3 || fa.Use.TargetWho != "enemy" || fa.Use.TempHP != 4 ||
		fa.Use.Heal == nil || fa.Use.Heal.Sides != 6 || !fa.Use.HasEffect || fa.Use.EffectRounds != 5 || len(fa.Use.Choices) != 1 {
		t.Errorf("link use = %+v", fa.Use)
	}
	old := rules.Action{Key: "feature:y@mesa", Economy: rules.EconomyAction}
	plain := actionOptionToProto(combat.ActionOption{Action: old})
	if plain.GetViaUseFeature() || plain.GetUseScope() != rulesv1.UseScope_USE_SCOPE_UNSPECIFIED || len(plain.GetChoices()) != 0 || featureActionOf(rules.Derived{}, old).Use != nil || old.ViaUseFeature() {
		t.Errorf("an action without a use changed: %v", plain)
	}
}
