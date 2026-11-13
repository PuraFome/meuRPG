package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// UseFeature end to end (#743, docs/architecture/play/feature-state.md "Using a feature"). The names are invented: a Fighter 5 of
// the table's subclass "Mestre dos Brados" has features whose grant_action declares a use. These tests need the database.

const callsKey = "chamados"

// featureTable is the table of this file: Pensantus (Ana), Toren (Caio) and Brisa (Bia) fight the Capitão and a goblin.
type featureTable struct {
	*armed
	// keys by the name of the feature.
	keys map[string]string
}

func useOf(scope rulesv1.UseScope, uses string, targets *rulesv1.FeatureTargets, applies *rulesv1.FeatureApplies, choices ...string) *rulesv1.FeatureUse {
	u := &rulesv1.FeatureUse{LimitScope: scope, LimitUses: uses, Targets: targets, Applies: applies}
	for _, c := range choices {
		u.Choices = append(u.Choices, &rulesv1.FeatureChoice{Key: c, TextPt: "Opção " + c})
	}
	return u
}

func newFeatureTable(t *testing.T) *featureTable {
	t.Helper()
	m := &featureTable{keys: map[string]string{}}
	tempHP := func(v int32) *rulesv1.TableSpellEffect {
		return &rulesv1.TableSpellEffect{Modifiers: []*rulesv1.TableEffectModifier{{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_TEMP_HP, Value: v}}}
	}
	grant := func(name, economy, resource string, u *rulesv1.FeatureUse) *rulesv1.TableFeature {
		return &rulesv1.TableFeature{NamePt: name, DescPt: []string{name + "."}, Effects: []*rulesv1.TableEffect{{Type: "grant_action", Economy: economy, Resource: resource, TextPt: name + ".", Use: u}}}
	}
	ally := &rulesv1.FeatureTargets{Min: 1, Max: 2, Who: rulesv1.FeatureTargetWho_FEATURE_TARGET_WHO_ALLY, RangeFt: 30}
	enemy := &rulesv1.FeatureTargets{Min: 1, Max: 1, Who: rulesv1.FeatureTargetWho_FEATURE_TARGET_WHO_ENEMY, RangeFt: 10}
	self := &rulesv1.FeatureTargets{Who: rulesv1.FeatureTargetWho_FEATURE_TARGET_WHO_SELF}
	m.armed = newArmedWith(t, func(a *armed) {
		features := []*rulesv1.TableFeature{
			grant("Brado", "bonus_action", "", useOf(rulesv1.UseScope_USE_SCOPE_COMBAT, "2", ally, &rulesv1.FeatureApplies{Heal: "1d6", TempHp: "2"})),
			grant("Postura", "action", "", useOf(rulesv1.UseScope_USE_SCOPE_TURN, "1", self, &rulesv1.FeatureApplies{Effect: tempHP(3), EffectRounds: 3}, "fogo", "gelo")),
			grant("Reflexo", "reaction", "", useOf(rulesv1.UseScope_USE_SCOPE_ROUND, "1", nil, &rulesv1.FeatureApplies{TempHp: "4"})),
			grant("Marca", "free", "", useOf(rulesv1.UseScope_USE_SCOPE_COMBAT, "1", enemy, nil)),
			grant("Chamado", "bonus_action", callsKey, useOf(rulesv1.UseScope_USE_SCOPE_UNSPECIFIED, "", self, &rulesv1.FeatureApplies{TempHp: "1"})),
			{NamePt: "Chamados", DescPt: []string{"Uma reserva."}, Effects: []*rulesv1.TableEffect{{Type: "resource", Resource: callsKey, Max: "2", Recharge: "short_rest"}}},
		}
		sub := &rulesv1.TableSubclass{
			NamePt: "Mestre dos Brados", ClassKey: "class:fighter", DescPt: []string{"Brados."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: features}},
		}
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: sub},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		entry := res.Msg.GetEntry()
		for _, f := range entry.GetTableSubclass().GetLevels()[0].GetFeatures() {
			m.keys[f.GetNamePt()] = f.GetKey()
		}
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, RaceKey: "race:human",
			Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
			Classes: []*charactersv1.ClassLevel{{
				ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: entry.GetKey()},
			}},
			Languages:            []string{"Anão", "Élfico", "Gnômico"},
			SkillProficiencyKeys: []string{"skill:athletics", "skill:perception"},
			FeatureChoiceKeys:    []string{"feature:fighter-fighting-style-defense"},
			WeaponKeys:           []string{longsword},
			HitPoints:            &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}, ExperiencePoints: 6500,
		}}}
		made, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Pensantus) error = %v", err)
		}
		a.pens = made.Msg.GetCharacter()
		if issues := a.pens.GetDerived().GetIssues(); len(issues) != 0 {
			t.Fatalf("the sheet has issues: %v", issues)
		}
		scores := &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores, []string{battleaxe}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores, []string{rapier}, nil)
	})
	return m
}

// fight starts the combat with Pensantus on turn.
func (m *featureTable) fight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return m.passTo(t, m.castersFight(t, 1), "Pensantus")
}

// row is the option of the feature as u reads it for the combatant.
func (m *featureTable) row(t *testing.T, u *user, e *playv1.Encounter, who, name string) *rulesv1.ActionOption {
	t.Helper()
	for _, f := range m.mustOptions(t, u, e, who).GetOptions().GetFeatureActions() {
		if f.GetAction().GetKey() == m.keys[name] {
			return f
		}
	}
	t.Fatalf("%s has no option %q", who, name)
	return nil
}

// use calls UseFeature as u for the combatant.
func (m *featureTable) use(t *testing.T, u *user, e *playv1.Encounter, who, name string, edit func(*playv1.UseFeatureRequest)) (*playv1.UseFeatureResponse, error) {
	t.Helper()
	req := &playv1.UseFeatureRequest{CampaignId: m.campaignID, EncounterId: e.GetId(), CombatantId: m.id(t, who), ActionKey: m.keys[name], IdempotencyKey: newKey()}
	if edit != nil {
		edit(req)
	}
	res, err := u.feature.UseFeature(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func targets(labels ...string) func(m *featureTable, t *testing.T) func(*playv1.UseFeatureRequest) {
	return func(m *featureTable, t *testing.T) func(*playv1.UseFeatureRequest) {
		return func(r *playv1.UseFeatureRequest) {
			for _, l := range labels {
				r.TargetIds = append(r.TargetIds, m.id(t, l))
			}
		}
	}
}

func (m *featureTable) aim(t *testing.T, labels ...string) func(*playv1.UseFeatureRequest) {
	return targets(labels...)(m, t)
}

// A use that heals: the dice are the server's, the bonus action is spent (the action stays), the uses of the combat go down, the
// options say so, and the log names the feature for the table with the numbers for the master and the user's player.
func TestUseFeatureHealsWithTheServersDiceAndSpendsTheBonusAction(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.hurt(t, m.toren, 5)
	toren := m.vitals(t, m.toren).GetHitPointsCurrent()

	row := m.row(t, m.ana, e, "Pensantus", "Brado")
	if !row.GetEnabled() || !row.GetViaUseFeature() || row.GetUseTotal() != 2 || row.GetUseLeft() != 2 {
		t.Fatalf("the option before = %v, want enabled, 2 uses of 2", row)
	}
	m.h.roller.queue(4)
	res, err := m.use(t, m.ana, e, "Pensantus", "Brado", m.aim(t, "Toren"))
	if err != nil {
		t.Fatalf("UseFeature() error = %v", err)
	}
	if res.GetHealed() != 4 || res.GetTempHp() != 2 {
		t.Errorf("healed %d and gave %d temporary hit points, want 4 and 2", res.GetHealed(), res.GetTempHp())
	}
	if got := m.vitals(t, m.toren).GetHitPointsCurrent(); got != toren+4 {
		t.Errorf("Toren has %d hit points, want %d", got, toren+4)
	}
	if c := byLabel(t, res.GetEncounter(), "Pensantus"); !c.GetBonusActionUsed() || c.GetActionUsed() {
		t.Errorf("Pensantus has the bonus action used = %v and the action used = %v, want the bonus action only", c.GetBonusActionUsed(), c.GetActionUsed())
	}
	after := m.row(t, m.ana, e, "Pensantus", "Brado")
	if after.GetUseLeft() != 1 || after.GetEnabled() || after.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_BONUS_ACTION_USED {
		t.Errorf("the option after = enabled %v with %d left (%v), want the bonus action used and 1 use left", after.GetEnabled(), after.GetUseLeft(), after.GetReason().GetCode())
	}
	for name, u := range map[string]*user{"the master": m.master, "the player": m.ana, "another player": m.caio} {
		line := m.logEntries(t, u, e)[0]
		if line.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED || line.GetActorLabel() != "Pensantus" || line.GetFeatureUsed().GetNamePt() != "Brado" {
			t.Fatalf("%s reads the line %v, want Pensantus's use of Brado", name, line)
		}
		wantHealed := int32(4)
		if u == m.caio {
			wantHealed = 0 // the numbers are the master's and the user's player's
		}
		if got := line.GetFeatureUsed().GetHealed(); got != wantHealed {
			t.Errorf("%s reads %d healed, want %d", name, got, wantHealed)
		}
	}
}
