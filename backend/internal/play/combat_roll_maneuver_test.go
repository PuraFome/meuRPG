package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The table dice a player adds to a roll of its own character (superiority_die of applies check, save and
// initiative). The names are invented: a Fighter 5 of the subclass "Mestre do Olhar" has a d6 for Perception
// checks, a d6 for Constitution saves and a d4 for the initiative, from a pool of two uses.

const rollDiceUses = "dados_de_olhar"

// ---- pure ----

func TestAManeuverGoesOnTheRollsItsKindAndTargetsNameAndOnlyThose(t *testing.T) {
	t.Parallel()
	check := link.Maneuver{Applies: rules.ManeuverCheck, Targets: []string{"skill.perception", "check.int"}}
	save := link.Maneuver{Applies: rules.ManeuverSave, Ability: "con"}
	for _, tc := range []struct {
		name               string
		m                  link.Maneuver
		applies, skill, ab string
		want               bool
	}{
		{"a listed skill", check, rules.ManeuverCheck, "skill:perception", "wis", true},
		{"a listed ability", check, rules.ManeuverCheck, "ability:int", "int", true},
		{"a skill of a listed ability", check, rules.ManeuverCheck, "skill:investigation", "int", true},
		{"a skill that is not listed", check, rules.ManeuverCheck, "skill:athletics", "str", false},
		{"every check when no target is listed", link.Maneuver{Applies: rules.ManeuverCheck}, rules.ManeuverCheck, "skill:athletics", "str", true},
		{"a check die on a save", check, rules.ManeuverSave, "", "int", false},
		{"the save of its ability", save, rules.ManeuverSave, "", "con", true},
		{"the save of another ability", save, rules.ManeuverSave, "", "dex", false},
		{"any save when it names no ability", link.Maneuver{Applies: rules.ManeuverSave}, rules.ManeuverSave, "", "dex", true},
		{"the initiative", link.Maneuver{Applies: rules.ManeuverInitiative}, rules.ManeuverInitiative, "", "", true},
		{"a damage maneuver on a check", link.Maneuver{Applies: rules.ManeuverDamage}, rules.ManeuverCheck, "skill:perception", "wis", false},
	} {
		if got := maneuverFits(tc.m, tc.applies, tc.skill, tc.ab); got != tc.want {
			t.Errorf("%s: fits = %v, want %v", tc.name, got, tc.want)
		}
	}
	sheet := link.Sheet{Traits: link.Traits{Maneuvers: []link.Maneuver{{Key: "a", Applies: rules.ManeuverInitiative}}}}
	for _, key := range []string{"b", "a"} {
		_, err := maneuverForRoll(sheet, key, rules.ManeuverCheck, "skill:perception", "wis")
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("maneuver %q on a check: %v, want invalid_argument", key, err)
		}
	}
}

// ---- end to end ----

type rollDiceTable struct {
	*armed
	look, brace, dash, strikeKey string
}

func newRollDiceTable(t *testing.T) *rollDiceTable {
	t.Helper()
	g := &rollDiceTable{}
	g.armed = newArmedWith(t, func(a *armed) {
		die := func(name, applies string, sides string, edit func(*rulesv1.TableEffect)) *rulesv1.TableFeature {
			e := &rulesv1.TableEffect{Type: "superiority_die", Applies: applies, Resource: rollDiceUses, Value: sides}
			if edit != nil {
				edit(e)
			}
			return &rulesv1.TableFeature{NamePt: name, DescPt: []string{"Um dado."}, Effects: []*rulesv1.TableEffect{e}}
		}
		pool := &rulesv1.TableFeature{NamePt: "Reserva de Olhar", DescPt: []string{"Dados."}, Effects: []*rulesv1.TableEffect{
			{Type: "resource", Resource: rollDiceUses, Max: "2", Recharge: "short_rest"},
		}}
		sub := &rulesv1.TableSubclass{
			NamePt: "Mestre do Olhar", ClassKey: "class:fighter", DescPt: []string{"Dados."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{
				pool,
				die("Olhar Atento", rules.ManeuverCheck, "6", func(e *rulesv1.TableEffect) { e.Targets = []string{"skill.perception"} }),
				die("Firmeza", rules.ManeuverSave, "6", func(e *rulesv1.TableEffect) { e.Ability = "con" }),
				die("Saída Rápida", rules.ManeuverInitiative, "4", nil),
				die("Golpe Preciso", rules.ManeuverAttack, "8", nil),
			}}},
		}
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: sub},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 12, Charisma: 8}, RaceKey: "race:human",
			Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
			Classes: []*charactersv1.ClassLevel{{
				ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: res.Msg.GetEntry().GetKey()},
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
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	for _, d := range g.pens.GetDerived().GetRollDice() {
		switch d.GetApplies() {
		case rules.ManeuverCheck:
			g.look = d.GetKey()
		case rules.ManeuverSave:
			g.brace = d.GetKey()
		case rules.ManeuverInitiative:
			g.dash = d.GetKey()
		case rules.ManeuverAttack:
			g.strikeKey = d.GetKey()
		}
	}
	if g.look == "" || g.brace == "" || g.dash == "" || g.strikeKey == "" {
		t.Fatalf("the derived sheet lists the dice %v, want a check, a save, an initiative and an attack die", g.pens.GetDerived().GetRollDice())
	}
	return g
}

func (g *rollDiceTable) usesLeft(t *testing.T) int32 {
	t.Helper()
	left, _ := poolOf(g.vitals(t, g.pens), rollDiceUses)
	return left
}

func TestTheDerivedSheetListsTheDiceOnTheCharactersOwnRolls(t *testing.T) {
	t.Parallel()
	g := newRollDiceTable(t)
	by := map[string]*rulesv1.RollDie{}
	for _, d := range g.pens.GetDerived().GetRollDice() {
		by[d.GetApplies()] = d
	}
	if d := by[rules.ManeuverCheck]; d.GetSides() != 6 || d.GetResource() != rollDiceUses || len(d.GetTargets()) != 1 || d.GetTargets()[0] != "skill.perception" || d.GetNamePt() != "Olhar Atento" {
		t.Errorf("check die = %v", d)
	}
	if d := by[rules.ManeuverSave]; d.GetAbility() != "con" || d.GetSides() != 6 {
		t.Errorf("save die = %v", d)
	}
	if d := by[rules.ManeuverInitiative]; d.GetSides() != 4 {
		t.Errorf("initiative die = %v", d)
	}
}

// scene rolls an action of the open scene as Pensantus in the app, with the faces the roller gives (the dice an effect adds are rolled before the d20), naming the maneuver.
func (g *rollDiceTable) scene(t *testing.T, action, maneuver string, faces ...int) (*playv1.RollSceneCheckResponse, error) {
	t.Helper()
	g.h.roller.queue(faces...)
	res, err := g.ana.play.RollSceneCheck(t.Context(), connect.NewRequest(&playv1.RollSceneCheckRequest{
		CampaignId: g.campaignID, ActionId: action, IdempotencyKey: newKey(), ManeuverKey: maneuver,
		Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestADieOnASceneCheckIsRolledShownAndSpendsAUse(t *testing.T) {
	t.Parallel()
	g := newRollDiceTable(t)
	point, actions := g.h.newScene(g.mapID, "O corredor", true, 5,
		sceneSpec{key: "skill:perception", dc: new(int32(15))}, sceneSpec{key: "skill:athletics", dc: new(int32(15))}, sceneSpec{key: "save:con", dc: new(int32(15))})
	g.openScene(t, point)

	plain, err := g.scene(t, actions[0], "", 10)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	with, err := g.scene(t, actions[0], g.look, 4, 10)
	if err != nil {
		t.Fatalf("RollSceneCheck(with the die) error = %v", err)
	}
	if got, want := with.GetRoll().GetRoll().GetTotal(), plain.GetRoll().GetRoll().GetTotal()+4; got != want {
		t.Errorf("total = %d, want %d (the die's 4 added)", got, want)
	}
	shown := false
	for _, s := range with.GetRoll().GetSources() {
		shown = shown || strings.Contains(s.GetTextPt(), "+1d6: 4")
	}
	if !shown {
		t.Errorf("sources = %v, want the die named with its face", with.GetRoll().GetSources())
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
	// A die that does not go on the check, or that the character has not, is refused and spends nothing.
	for name, tc := range map[string]struct{ action, key string }{
		"another skill":         {actions[1], g.look},
		"a save die on a check": {actions[0], g.brace},
		"a key nobody has":      {actions[0], "feature:inventado@mesa"},
	} {
		if _, err := g.scene(t, tc.action, tc.key, 3, 10); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want invalid_argument", name, err)
		}
	}
	// The save of the scene takes the save die of its ability.
	if _, err := g.scene(t, actions[2], g.brace, 2, 10); err != nil {
		t.Errorf("RollSceneCheck(a Constitution save with the save die) error = %v", err)
	}
	if got := g.usesLeft(t); got != 0 {
		t.Errorf("uses left = %d, want 0", got)
	}
	_, err = g.scene(t, actions[0], g.look, 3, 10)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT)
}

func TestADieOnTheInitiativeIsAddedWhenTheD20IsSettledAndSpendsAUse(t *testing.T) {
	t.Parallel()
	g := newRollDiceTable(t)
	e := g.start(t, plan{npcs: []*playv1.Participant{{CharacterId: g.goblin.GetId()}}, npcRolls: []int{3}, setup: true})
	submit := func(label string, edit func(*playv1.SubmitInitiativeRequest)) (*playv1.SubmitInitiativeResponse, error) {
		req := &playv1.SubmitInitiativeRequest{CampaignId: g.campaignID, EncounterId: e.GetId(), CombatantId: g.id(t, label), IdempotencyKey: newKey()}
		edit(req)
		res, err := g.ana.combat.SubmitInitiative(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	// A key that is not an initiative die, and a physical die with no face, are refused before anything is set.
	if _, err := submit("Pensantus", func(r *playv1.SubmitInitiativeRequest) { inApp(r); r.ManeuverKey = g.look }); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a check die on the initiative: %v, want invalid_argument", err)
	}
	if _, err := submit("Pensantus", func(r *playv1.SubmitInitiativeRequest) { typed(12)(r); r.ManeuverKey = g.dash }); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a physical die with no face: %v, want invalid_argument", err)
	}
	if got := g.usesLeft(t); got != 2 {
		t.Fatalf("uses left after the refusals = %d, want 2", got)
	}
	g.h.roller.queue(12, 3) // the d20, then the die
	res, err := submit("Pensantus", func(r *playv1.SubmitInitiativeRequest) { inApp(r); r.ManeuverKey = g.dash })
	if err != nil {
		t.Fatalf("SubmitInitiative(with the die) error = %v", err)
	}
	bonus := g.pens.GetDerived().GetInitiative()
	if got := byLabel(t, res.GetEncounter(), "Pensantus").GetInitiative(); got != 12+bonus+3 {
		t.Errorf("initiative = %d, want %d (12 + %d + the die's 3)", got, 12+bonus+3, bonus)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

func TestADieOnAPhysicalInitiativeTakesTheTypedFace(t *testing.T) {
	t.Parallel()
	g := newRollDiceTable(t)
	e := g.start(t, plan{npcs: []*playv1.Participant{{CharacterId: g.goblin.GetId()}}, npcRolls: []int{3}, setup: true})
	g.setPhysical(t, g.ana)
	res, err := g.ana.combat.SubmitInitiative(t.Context(), connect.NewRequest(&playv1.SubmitInitiativeRequest{
		CampaignId: g.campaignID, EncounterId: e.GetId(), CombatantId: g.id(t, "Pensantus"), IdempotencyKey: newKey(),
		Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 9}, ManeuverKey: g.dash, ManeuverFace: 4,
	}))
	if err != nil {
		t.Fatalf("SubmitInitiative(typed) error = %v", err)
	}
	if got, want := byLabel(t, res.Msg.GetEncounter(), "Pensantus").GetInitiative(), 9+g.pens.GetDerived().GetInitiative()+4; got != want {
		t.Errorf("initiative = %d, want %d", got, want)
	}
	// A face over the die is refused.
	if _, err := g.ana.combat.SubmitInitiative(t.Context(), connect.NewRequest(&playv1.SubmitInitiativeRequest{
		CampaignId: g.campaignID, EncounterId: e.GetId(), CombatantId: g.id(t, "Pensantus"), IdempotencyKey: newKey(),
		Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 9}, ManeuverKey: g.dash, ManeuverFace: 5,
	})); err == nil {
		t.Error("a face of 5 on a d4 is accepted")
	}
}

func TestADieOnTheConcentrationSaveIsRolledWithItAndSpendsAUse(t *testing.T) {
	t.Parallel()
	g := newRollDiceTable(t)
	e := g.castersFightNPCFirst(t)
	g.concentrate(t, "Pensantus", webSpell)
	hit := g.mustAttack(t, g.master, e, "Capitão Goblin", sword, "Pensantus", d20(9))
	g.h.roller.queue(6)
	g.mustDamage(t, g.master, e, hit.GetPendingDamage().GetId(), inAppDamage)
	if _, err := g.settle(t, g.master, e, hit.GetPendingDamage().GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	w := g.windowOf(t, g.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("no concentration window among %v", g.windows(t, g.ana))
	}
	// A die of another ability is refused; the Constitution one is rolled with the save.
	if _, err := g.resolveConcentration(t, g.ana, e, w.GetId(), func(r *playv1.ResolveConcentrationSaveRequest) {
		r.Roll, r.ManeuverKey = &playv1.ResolveConcentrationSaveRequest_RollInApp{RollInApp: true}, g.look
	}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a check die on the save: %v, want invalid_argument", err)
	}
	g.h.roller.queue(10, 5)
	res, err := g.resolveConcentration(t, g.ana, e, w.GetId(), func(r *playv1.ResolveConcentrationSaveRequest) {
		r.Roll, r.ManeuverKey = &playv1.ResolveConcentrationSaveRequest_RollInApp{RollInApp: true}, g.brace
	})
	if err != nil {
		t.Fatalf("ResolveConcentrationSave(with the die) error = %v", err)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
	if save := res.GetResult().GetSave(); save.GetTotal() != save.GetFaces()[0]+save.GetModifier() || save.GetModifier() < 5 {
		t.Errorf("save = %v, want the d20 and a modifier with the die's 5 in it", save)
	}
}
