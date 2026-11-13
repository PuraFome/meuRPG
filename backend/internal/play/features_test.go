package play

import (
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
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
	key, ok := m.keys[name]
	if !ok {
		key = name
	}
	req := &playv1.UseFeatureRequest{CampaignId: m.campaignID, EncounterId: e.GetId(), CombatantId: m.id(t, who), ActionKey: key, IdempotencyKey: newKey()}
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

// moveTo puts the combatant on the square, as the master.
func (m *featureTable) moveTo(t *testing.T, e *playv1.Encounter, label string, col, row int32) {
	t.Helper()
	if _, err := m.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
		CampaignId: m.campaignID, EncounterId: e.GetId(), CombatantId: m.id(t, label), IdempotencyKey: newKey(), Col: col, Row: row,
	})); err != nil {
		t.Fatalf("MoveCombatant(%s) error = %v", label, err)
	}
}

// uses is the feature used by u, which must work.
func (m *featureTable) mustUse(t *testing.T, u *user, e *playv1.Encounter, who, name string, edit func(*playv1.UseFeatureRequest)) *playv1.UseFeatureResponse {
	t.Helper()
	res, err := m.use(t, u, e, who, name, edit)
	if err != nil {
		t.Fatalf("UseFeature(%s, %s) error = %v", who, name, err)
	}
	return res
}

// Each economy a feature may spend: the action, the bonus action, the reaction and none (free) spend that and nothing else;
// a feature with a rest resource spends one use of it.
func TestUseFeatureSpendsTheEconomyTheFeatureDeclares(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.mustUse(t, m.ana, e, "Pensantus", "Postura", func(r *playv1.UseFeatureRequest) { r.Choice = "fogo" })
	if pens := byLabel(t, m.get(t, m.master), "Pensantus"); !pens.GetActionUsed() || pens.GetBonusActionUsed() {
		t.Errorf("after Postura: action used %v, bonus action used %v, want the action only", pens.GetActionUsed(), pens.GetBonusActionUsed())
	}
	m.mustUse(t, m.ana, e, "Pensantus", "Chamado", nil)
	m.mustUse(t, m.ana, e, "Pensantus", "Marca", m.aim(t, "Capitão Goblin"))
	pens := byLabel(t, m.get(t, m.master), "Pensantus")
	if !pens.GetActionUsed() || !pens.GetBonusActionUsed() || pens.GetReactionUsed() {
		t.Errorf("Pensantus has used action %v, bonus action %v, reaction %v; want the action (Postura) and the bonus action (Chamado) and no reaction (Marca is free)", pens.GetActionUsed(), pens.GetBonusActionUsed(), pens.GetReactionUsed())
	}
	if left, _ := poolOf(m.vitals(t, m.pens), callsKey); left != 1 {
		t.Errorf("chamados left = %d, want 1", left)
	}
	// A reaction is spent on another creature's turn: Toren's, once Pensantus's is over.
	m.passTo(t, m.get(t, m.master), "Toren")
	m.mustUse(t, m.ana, e, "Pensantus", "Reflexo", nil)
	if pens := byLabel(t, m.get(t, m.master), "Pensantus"); !pens.GetReactionUsed() {
		t.Error("the reaction is not spent")
	}
	if _, temp, _ := m.hp(t, "Pensantus"); temp != 4 {
		t.Errorf("Pensantus has %d temporary hit points, want 4", temp)
	}
}

// What is not a reaction waits for the combatant's turn, and a reaction the combatant already spent is refused.
func TestUseFeatureOffTurn(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.passTo(t, m.fight(t), "Toren")
	_, err := m.use(t, m.ana, e, "Pensantus", "Chamado", nil)
	wantBlockedBy(t, "a bonus action off turn", err, blockedNotYourTurn)
	if row := m.row(t, m.ana, e, "Pensantus", "Chamado"); row.GetEnabled() || row.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN {
		t.Errorf("the option off turn = enabled %v (%v), want NOT_YOUR_TURN", row.GetEnabled(), row.GetReason().GetCode())
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Reflexo"); !row.GetEnabled() {
		t.Errorf("the reaction off turn = %v, want it enabled", row)
	}
	m.mustUse(t, m.ana, e, "Pensantus", "Reflexo", nil)
	_, err = m.use(t, m.ana, e, "Pensantus", "Reflexo", nil)
	wantBlockedBy(t, "the reaction a second time", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_USED)
	if row := m.row(t, m.ana, e, "Pensantus", "Reflexo"); row.GetEnabled() || row.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_USED {
		t.Errorf("the option after = enabled %v (%v), want REACTION_USED", row.GetEnabled(), row.GetReason().GetCode())
	}
}

// The uses of a scope: the master may act again with the economy spent, but not past the uses; a turn's use comes back on the
// next turn, a combat's never does.
func TestUseFeatureUsesOfAScopeAndTheirExpiry(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	choose := func(r *playv1.UseFeatureRequest) { r.Choice = "gelo" }
	m.mustUse(t, m.ana, e, "Pensantus", "Postura", choose)
	_, err := m.use(t, m.ana, e, "Pensantus", "Postura", choose)
	wantBlockedBy(t, "Postura with the action spent", err, blockedActionUsed)
	_, err = m.use(t, m.master, e, "Pensantus", "Postura", choose)
	wantBlockedBy(t, "Postura with the turn's use spent", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	if row := m.row(t, m.master, e, "Pensantus", "Postura"); row.GetUseTotal() != 1 || row.GetUseLeft() != 0 || row.GetUseScope() != rulesv1.UseScope_USE_SCOPE_TURN {
		t.Errorf("the option = %v, want 0 of 1 turn use left", row)
	}
	// Brado is two uses in the combat: one now, one on the next turn, none after that.
	m.h.roller.queue(3)
	m.mustUse(t, m.ana, e, "Pensantus", "Brado", m.aim(t, "Toren"))
	e = m.passTo(t, m.get(t, m.master), "Toren")
	e = m.passTo(t, e, "Pensantus")
	if row := m.row(t, m.ana, e, "Pensantus", "Postura"); !row.GetEnabled() || row.GetUseLeft() != 1 {
		t.Errorf("Postura on the next turn = enabled %v with %d left, want its turn's use back", row.GetEnabled(), row.GetUseLeft())
	}
	m.h.roller.queue(3)
	m.mustUse(t, m.ana, e, "Pensantus", "Brado", m.aim(t, "Brisa"))
	_, err = m.use(t, m.master, e, "Pensantus", "Brado", m.aim(t, "Toren"))
	wantBlockedBy(t, "Brado past its two uses", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	e = m.passTo(t, m.passTo(t, e, "Toren"), "Pensantus")
	if row := m.row(t, m.ana, e, "Pensantus", "Brado"); row.GetEnabled() || row.GetUseLeft() != 0 || row.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_USES {
		t.Errorf("Brado in the third turn = enabled %v with %d left (%v), want NO_USES", row.GetEnabled(), row.GetUseLeft(), row.GetReason().GetCode())
	}
}

// A feature with a rest resource and no scope: the resource is the limit.
func TestUseFeatureWithARestResource(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	if row := m.row(t, m.ana, e, "Pensantus", "Chamado"); row.GetResourceTotal() != 2 || row.GetUsesLeft() != 2 || row.GetUseTotal() != 0 {
		t.Fatalf("the option = %v, want the resource's 2 of 2 and no scope uses", row)
	}
	m.mustUse(t, m.ana, e, "Pensantus", "Chamado", nil)
	m.mustUse(t, m.master, e, "Pensantus", "Chamado", nil)
	_, err := m.use(t, m.master, e, "Pensantus", "Chamado", nil)
	wantBlockedBy(t, "Chamado with the resource spent", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	if left, _ := poolOf(m.vitals(t, m.pens), callsKey); left != 0 {
		t.Errorf("chamados left = %d, want 0", left)
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Chamado"); row.GetUsesLeft() != 0 || row.GetEnabled() {
		t.Errorf("the option = %v, want it off with 0 left", row)
	}
}

// The targets: how many, who, how far, and a creature the player does not see is refused like one that does not exist.
func TestUseFeatureTargets(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	refused := func(call string, edit func(*playv1.UseFeatureRequest), name string) *playv1.EncounterBlocked {
		t.Helper()
		_, err := m.use(t, m.ana, e, "Pensantus", name, edit)
		return wantBlockedBy(t, call, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_FEATURE_TARGET)
	}
	refused("no target for an ally feature", nil, "Brado")
	refused("three targets for two", m.aim(t, "Toren", "Brisa", "Pensantus"), "Brado")
	refused("an enemy for an ally feature", m.aim(t, "Capitão Goblin"), "Brado")
	refused("an ally for an enemy feature", m.aim(t, "Toren"), "Marca")
	refused("two enemies for one", m.aim(t, "Capitão Goblin", "Goblin"), "Marca")
	refused("a target for a feature that picks none", m.aim(t, "Toren"), "Chamado")
	if got := m.get(t, m.master); byLabel(t, got, "Pensantus").GetBonusActionUsed() || byLabel(t, got, "Pensantus").GetActionUsed() {
		t.Error("a refused use spent the economy")
	}
	// "Ally" is another creature of the user's side: the user itself is picked only when it has no ally in sight.
	refused("the user among the targets of an ally feature", m.aim(t, "Pensantus", "Toren"), "Brado")
	m.h.roller.queue(2)
	m.mustUse(t, m.ana, e, "Pensantus", "Brado", m.aim(t, "Brisa", "Toren"))
	// A creature the player does not see, and one that does not exist, are the same answer.
	hideAgain(t, m.armed, e, "Goblin")
	hidden := refused("a hidden creature", m.aim(t, "Goblin"), "Marca")
	gone := refused("a creature that is not in the combat", func(r *playv1.UseFeatureRequest) { r.TargetIds = []string{newKey()} }, "Marca")
	if len(hidden.GetCombatantIds()) != 1 || len(gone.GetCombatantIds()) != 1 {
		t.Errorf("hidden = %v, gone = %v, want the same answer, naming the one id given", hidden, gone)
	}
	if hidden.GetCombatantIds()[0] != m.id(t, "Goblin") {
		t.Errorf("the refusal names %v, want the id the caller sent", hidden.GetCombatantIds())
	}
	// The master sees it: the use is his to make.
	m.mustUse(t, m.master, e, "Pensantus", "Marca", m.aim(t, "Goblin"))
}

// A target out of reach is refused, and the options say why in words.
func TestUseFeatureRange(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.moveTo(t, e, "Toren", 18, 5) // 13 squares away: 65 ft, past the 30 ft of Brado
	_, err := m.use(t, m.ana, e, "Pensantus", "Brado", m.aim(t, "Toren"))
	wantBlockedBy(t, "Toren out of reach", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_FEATURE_TARGET)
	var list *playv1.ResourceTargets
	for _, ft := range m.mustOptions(t, m.ana, e, "Pensantus").GetFeatureTargets() {
		if ft.GetActionKey() == m.keys["Brado"] {
			list = ft
		}
	}
	if list == nil {
		t.Fatal("feature_targets has no entry for Brado")
	}
	for _, c := range list.GetTargets() {
		reason := c.GetDisabledReasonPt()
		switch c.GetTarget().GetLabel() {
		case "Toren":
			if reason != "Fora de alcance: até 9 m" || !c.GetTarget().GetTooFar() {
				t.Errorf("Toren = %v, want out of reach", c)
			}
		case "Pensantus":
			if reason != "É você" {
				t.Errorf("Pensantus = %q, want that the user is not an ally", reason)
			}
		case "Brisa":
			if reason != "" {
				t.Errorf("%s = %q, want a target", c.GetTarget().GetLabel(), reason)
			}
		case "Capitão Goblin", "Goblin":
			if reason != "Lado errado: só pega aliados" {
				t.Errorf("%s = %q, want the wrong side", c.GetTarget().GetLabel(), reason)
			}
		}
	}
	m.h.roller.queue(1)
	m.mustUse(t, m.master, e, "Pensantus", "Brado", m.aim(t, "Toren")) // the master is not held to the reach
}

// The choice: one of the feature's keys, and none for a feature that offers none.
func TestUseFeatureChoice(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	for name, edit := range map[string]func(*playv1.UseFeatureRequest){
		"no choice":      nil,
		"an unknown one": func(r *playv1.UseFeatureRequest) { r.Choice = "raio" },
	} {
		_, err := m.use(t, m.ana, e, "Pensantus", "Postura", edit)
		wantBlockedBy(t, "Postura with "+name, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_FEATURE_CHOICE)
	}
	_, err := m.use(t, m.ana, e, "Pensantus", "Chamado", func(r *playv1.UseFeatureRequest) { r.Choice = "fogo" })
	wantBlockedBy(t, "a choice for a feature that offers none", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_FEATURE_CHOICE)
	if pens := byLabel(t, m.get(t, m.master), "Pensantus"); pens.GetActionUsed() || pens.GetBonusActionUsed() {
		t.Error("a refused choice spent the economy")
	}
	m.mustUse(t, m.ana, e, "Pensantus", "Postura", func(r *playv1.UseFeatureRequest) { r.Choice = "fogo" })
	for _, u := range []*user{m.master, m.ana, m.caio} {
		if got := m.logEntries(t, u, e)[0].GetFeatureUsed().GetChoiceTextPt(); got != "Opção fogo" {
			t.Errorf("the line's choice = %q, want the label of the choice", got)
		}
	}
}

// Who may use it: the character's own player and the master; not another player, not an NPC, and not a creature the player
// does not see.
func TestUseFeaturePermissions(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	_, err := m.use(t, m.caio, e, "Pensantus", "Chamado", nil)
	wantCode(t, "another player's use", err, connect.CodePermissionDenied)
	_, err = m.use(t, m.master, e, "Goblin", "Chamado", nil)
	wantCode(t, "an NPC's use", err, connect.CodeInvalidArgument)
	hideAgain(t, m.armed, e, "Goblin")
	_, err = m.use(t, m.ana, e, "Goblin", "Chamado", nil)
	wantCode(t, "a hidden NPC for a player", err, connect.CodeNotFound)
	_, err = m.use(t, m.h.anonymous(), e, "Pensantus", "Chamado", nil)
	if err == nil {
		t.Error("a stranger used a feature")
	}
	_, err = m.use(t, m.ana, e, "Pensantus", "feature:nao-existe", nil)
	wantCode(t, "a feature the character does not have", err, connect.CodeNotFound)
	m.mustUse(t, m.master, e, "Pensantus", "Chamado", nil)
	if line := m.logEntries(t, m.ana, e)[0]; !line.GetByMaster() {
		t.Errorf("the line = %v, want it marked as the master's act for the character", line)
	}
}

// A feature that spends the reaction is used while an opportunity offer waits for the user, on another creature's turn: the
// options list it as enabled (the user's reaction is free and it can act) and UseFeature spends the reaction, which answers
// the offer by taking away what it needed (the offer is passed over).
func TestUseFeatureReactionWhileAnOpportunityOfferWaits(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.passTo(t, m.castersFight(t, 1), "Capitão Goblin")
	m.moveTo(t, e, "Capitão Goblin", 2, 2) // out of Pensantus's reach, on its own turn
	offers := m.get(t, m.ana).GetOpportunityOffers()
	if len(offers) != 1 || offers[0].GetReactorId() != m.id(t, "Pensantus") {
		t.Fatalf("the offers = %v, want one for Pensantus", offers)
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Reflexo"); !row.GetEnabled() || row.GetReason() != nil {
		t.Fatalf("the reaction while the offer waits = %v, want it enabled", row)
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Chamado"); row.GetEnabled() {
		t.Errorf("a bonus action off turn = %v, want it off", row)
	}
	m.mustUse(t, m.ana, e, "Pensantus", "Reflexo", nil)
	if pens := byLabel(t, m.get(t, m.master), "Pensantus"); !pens.GetReactionUsed() {
		t.Error("the reaction is not spent")
	}
	if got := m.get(t, m.master).GetOpportunityOffers(); len(got) != 0 {
		t.Errorf("the offers after the reaction = %v, want none (nobody can answer them)", got)
	}
}

// validArgs are the arguments that make each feature of the table a valid call.
func (m *featureTable) validArgs(t *testing.T, name string) func(*playv1.UseFeatureRequest) {
	t.Helper()
	switch name {
	case "Brado":
		return m.aim(t, "Toren")
	case "Marca":
		return m.aim(t, "Capitão Goblin")
	case "Postura":
		return func(r *playv1.UseFeatureRequest) { r.Choice = "fogo" }
	}
	return nil
}

// What GetTurnOptions calls enabled is what UseFeature accepts, in every state the table goes through: on turn, with the
// economy and the uses spent, off turn, with the reaction spent.
func TestUseFeatureOptionsAgreeWithTheCall(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	names := []string{"Brado", "Postura", "Reflexo", "Marca", "Chamado"}
	check := func(state string) {
		t.Helper()
		for _, name := range names {
			m.h.roller.queue(3)
			enabled := m.row(t, m.ana, e, "Pensantus", name).GetEnabled()
			_, err := m.use(t, m.ana, e, "Pensantus", name, m.validArgs(t, name))
			if (err == nil) != enabled {
				t.Errorf("%s, %s: the option says enabled = %v and the call answers %v", state, name, enabled, err)
			}
			if err != nil {
				wantCode(t, state+", "+name, err, connect.CodeFailedPrecondition)
			}
		}
	}
	check("on turn")
	check("with everything spent")
	e = m.passTo(t, m.get(t, m.master), "Toren")
	check("off turn")
	e = m.passTo(t, e, "Pensantus")
	check("the next turn")
	check("the next turn, again")
}

// A use is replayed with the same key: one use, one event, the same answer.
func TestUseFeatureIsIdempotent(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.h.roller.queue(3, 5)
	key := newKey()
	req := func(r *playv1.UseFeatureRequest) { m.aim(t, "Toren")(r); r.IdempotencyKey = key }
	first := m.mustUse(t, m.ana, e, "Pensantus", "Brado", req)
	again := m.mustUse(t, m.ana, e, "Pensantus", "Brado", req)
	if first.GetHealed() != again.GetHealed() || first.GetTempHp() != again.GetTempHp() {
		t.Errorf("the replay healed %d and gave %d, want %d and %d", again.GetHealed(), again.GetTempHp(), first.GetHealed(), first.GetTempHp())
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Brado"); row.GetUseLeft() != 1 {
		t.Errorf("uses left = %d, want 1: a replay spends nothing", row.GetUseLeft())
	}
	lines := logLinesOf(t, m.armed, m.master, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED)
	if len(lines) != 1 {
		t.Errorf("the log has %d lines, want 1", len(lines))
	}
}

// Two calls at once on two connections: with one use in the combat, one of them wins and the other is told no uses are left;
// with the same key, both get the one answer.
func TestUseFeatureConcurrent(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 2)
	m := newFeatureTable(t)
	e := m.fight(t)
	race := func(keys [2]string) [2]error {
		var errs [2]error
		var wg sync.WaitGroup
		for i := range errs {
			wg.Go(func() {
				_, errs[i] = m.use(t, m.ana, e, "Pensantus", "Marca", func(r *playv1.UseFeatureRequest) {
					m.aim(t, "Capitão Goblin")(r)
					r.IdempotencyKey = keys[i]
				})
			})
		}
		wg.Wait()
		return errs
	}
	errs := race([2]string{newKey(), newKey()})
	won := 0
	for _, err := range errs {
		if err == nil {
			won++
			continue
		}
		wantBlockedBy(t, "the loser", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	}
	if won != 1 {
		t.Fatalf("%d of two calls won, want exactly one (%v)", won, errs)
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Marca"); row.GetUseLeft() != 0 {
		t.Errorf("uses left = %d, want 0", row.GetUseLeft())
	}
	// The same key twice at once is one use.
	m2 := newFeatureTable(t)
	e2 := m2.fight(t)
	same := newKey()
	var wg sync.WaitGroup
	var got [2]error
	for i := range got {
		wg.Go(func() {
			_, got[i] = m2.use(t, m2.ana, e2, "Pensantus", "Marca", func(r *playv1.UseFeatureRequest) {
				m2.aim(t, "Capitão Goblin")(r)
				r.IdempotencyKey = same
			})
		})
	}
	wg.Wait()
	if got[0] != nil || got[1] != nil {
		t.Errorf("the same key twice = %v, %v, want both to answer", got[0], got[1])
	}
	if lines := logLinesOf(t, m2.armed, m2.master, e2, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED); len(lines) != 1 {
		t.Errorf("the log has %d lines, want 1", len(lines))
	}
}

// A use with an effect: the lasting effect and the temporary hit points it gives, the choice, and an undo that takes all of it
// back and gives the use back.
func TestUseFeatureEffectAndUndo(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	count := func() (n int) {
		if err := m.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM combatant_states WHERE source_key = $1`, m.keys["Postura"]).Scan(&n); err != nil {
			t.Fatalf("count the effects: %v", err)
		}
		return n
	}
	m.mustUse(t, m.ana, e, "Pensantus", "Postura", func(r *playv1.UseFeatureRequest) { r.Choice = "fogo" })
	if _, temp, _ := m.hp(t, "Pensantus"); temp != 3 || count() != 1 {
		t.Fatalf("temporary hit points = %d and effects = %d, want 3 and 1", temp, count())
	}
	if line := m.logEntries(t, m.caio, e)[0]; !line.GetFeatureUsed().GetEffectApplied() {
		t.Errorf("the line = %v, want effect_applied", line)
	}
	m.undoLast(t, e)
	if _, temp, _ := m.hp(t, "Pensantus"); temp != 0 || count() != 0 {
		t.Errorf("after the undo: temporary hit points = %d and effects = %d, want 0 and 0", temp, count())
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Postura"); !row.GetEnabled() || row.GetUseLeft() != 1 {
		t.Errorf("the option after the undo = %v, want the use back", row)
	}
	pens := byLabel(t, m.get(t, m.master), "Pensantus")
	if pens.GetActionUsed() {
		t.Error("the action is still spent after the undo")
	}
	if lines := logLinesOf(t, m.armed, m.master, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED); len(lines) != 0 {
		t.Errorf("the log keeps %d lines of the undone use", len(lines))
	}
}

// The undo of a heal, of the resource and of the economy.
func TestUseFeatureUndoPutsEverythingBack(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.hurt(t, m.toren, 5)
	before := m.vitals(t, m.toren).GetHitPointsCurrent()
	m.h.roller.queue(6)
	m.mustUse(t, m.ana, e, "Pensantus", "Brado", m.aim(t, "Toren", "Brisa"))
	m.mustUse(t, m.master, e, "Pensantus", "Chamado", nil) // the master may use the bonus action again
	m.undoLast(t, e)                                       // the Chamado
	if left, _ := poolOf(m.vitals(t, m.pens), callsKey); left != 2 {
		t.Errorf("chamados left = %d after the undo, want 2", left)
	}
	if _, temp, _ := m.hp(t, "Pensantus"); temp != 0 {
		t.Errorf("Pensantus has %d temporary hit points, want the Chamado's taken back", temp)
	}
	m.undoLast(t, e) // the Brado
	if got := m.vitals(t, m.toren).GetHitPointsCurrent(); got != before {
		t.Errorf("Toren has %d hit points, want %d", got, before)
	}
	if _, temp, _ := m.hp(t, "Brisa"); temp != 0 {
		t.Errorf("Brisa has %d temporary hit points, want 0", temp)
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Brado"); !row.GetEnabled() || row.GetUseLeft() != 2 {
		t.Errorf("the option after = %v, want 2 uses left and enabled", row)
	}
}

// The line of a use that touched a creature the players do not see is the master's alone, and nothing the players read says
// the creature is there.
func TestUseFeatureLogAudienceAndNoLeak(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	hideAgain(t, m.armed, e, "Goblin")
	m.mustUse(t, m.master, e, "Pensantus", "Marca", m.aim(t, "Goblin"))
	if lines := logLinesOf(t, m.armed, m.master, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED); len(lines) != 1 {
		t.Fatalf("the master reads %d lines, want 1", len(lines))
	}
	for name, u := range map[string]*user{"Pensantus's player": m.ana, "another player": m.caio} {
		if lines := logLinesOf(t, m.armed, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED); len(lines) != 0 {
			t.Errorf("%s reads %v, want nothing of a use on a creature they do not see", name, lines)
		}
		raw, err := protojson.Marshal(m.log(t, u, e))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "Goblin\"") && strings.Contains(string(raw), m.id(t, "Goblin")) {
			t.Errorf("the log of %s names the hidden creature", name)
		}
		if strings.Contains(string(raw), m.id(t, "Goblin")) {
			t.Errorf("the log of %s holds the id of the hidden creature", name)
		}
		if u != m.ana {
			continue // only Pensantus's player reads Pensantus's options
		}
		for _, ft := range m.mustOptions(t, u, e, "Pensantus").GetFeatureTargets() {
			for _, c := range ft.GetTargets() {
				if c.GetTarget().GetCombatantId() == m.id(t, "Goblin") {
					t.Errorf("%s reads the hidden creature among the targets", name)
				}
			}
		}
	}
	// A use on a creature everybody sees is everybody's line.
	m.mustUse(t, m.ana, e, "Pensantus", "Chamado", nil)
	for name, u := range map[string]*user{"the master": m.master, "Pensantus's player": m.ana, "another player": m.caio} {
		if lines := logLinesOf(t, m.armed, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED); len(lines) < 1 {
			t.Errorf("%s reads no line of the Chamado", name)
		}
	}
}

// TakeAction and the other calls by key refuse a feature that declares a use.
func TestTakeActionRefusesAFeatureThatIsUsedWithUseFeature(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	_, err := m.ana.combat.TakeAction(t.Context(), connect.NewRequest(&playv1.TakeActionRequest{
		CampaignId: m.campaignID, EncounterId: e.GetId(), CombatantId: m.id(t, "Pensantus"), ActionKey: m.keys["Chamado"], IdempotencyKey: newKey(),
	}))
	wantCode(t, "TakeAction of a feature that declares a use", err, connect.CodeInvalidArgument)
	if pens := byLabel(t, m.get(t, m.master), "Pensantus"); pens.GetBonusActionUsed() {
		t.Error("the refused call spent the bonus action")
	}
}

// One table entry that has both a use (two in the combat) and an extra damage once per turn, under the same feature key: the
// two states are not one. A damage roll with the extra does not change the use count, and a use does not give the turn's
// extra back.
func TestUseFeatureAndOncePerTurnExtraDamageKeepTheirOwnState(t *testing.T) {
	t.Parallel()
	g := newGiantTableWith(t,
		&rulesv1.TableEffect{Type: "extra_damage", Applies: "any_target", Value: "3", Frequency: "once_per_turn", TextPt: "Dano extra."},
		&rulesv1.TableEffect{Type: "grant_action", Economy: "bonus_action", TextPt: "Brado.", Use: useOf(rulesv1.UseScope_USE_SCOPE_COMBAT, "2", nil, &rulesv1.FeatureApplies{TempHp: "2"})},
	)
	e := g.passTo(t, g.castersFight(t, 1), "Pensantus")
	key := giantFeatureKey(t, g)
	left := func() int32 {
		t.Helper()
		for _, f := range g.mustOptions(t, g.ana, e, "Pensantus").GetOptions().GetFeatureActions() {
			if f.GetAction().GetKey() == key {
				return f.GetUseLeft()
			}
		}
		t.Fatal("no Brado among the options")
		return 0
	}
	use := func(u *user) error {
		_, err := u.feature.UseFeature(t.Context(), connect.NewRequest(&playv1.UseFeatureRequest{
			CampaignId: g.campaignID, EncounterId: e.GetId(), CombatantId: g.id(t, "Pensantus"), ActionKey: key, IdempotencyKey: newKey(),
		}))
		return err
	}
	first := g.hit(t, e)
	g.h.roller.queue(5)
	g.mustDamage(t, g.ana, e, first.GetId(), marking(key)) // the extra damage marks the turn
	if got := left(); got != 2 {
		t.Fatalf("uses left after the extra damage = %d, want 2: a damage roll does not spend a use", got)
	}
	if err := use(g.ana); err != nil {
		t.Fatalf("the use after the extra damage: %v", err)
	}
	if got := left(); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
	second := g.hit(t, e)
	if q := partOf(second.GetParts(), key); q == nil || q.GetAvailable() {
		t.Errorf("the extra after a use = %v, want it still off: the use does not give the turn's extra back", q)
	}
	// The other way: the use first (a new turn), then the extra is not blocked by it.
	e = g.passTo(t, g.passTo(t, g.get(t, g.master), "Toren"), "Pensantus")
	if err := use(g.master); err != nil { // the master may use the bonus action again
		t.Fatalf("the use: %v", err)
	}
	if got := left(); got != 0 {
		t.Fatalf("uses left = %d, want 0", got)
	}
	third := g.hit(t, e)
	if q := partOf(third.GetParts(), key); q == nil || !q.GetAvailable() {
		t.Fatalf("the extra after a use in a new turn = %v, want it available", q)
	}
	g.h.roller.queue(5)
	if _, err := g.damage(t, g.ana, e, third.GetId(), marking(key)); err != nil {
		t.Errorf("the extra after a use: %v", err)
	}
	if got := left(); got != 0 {
		t.Errorf("uses left after the second extra = %d, want 0", got)
	}
}
