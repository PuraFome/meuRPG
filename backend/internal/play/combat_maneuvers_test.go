package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A table option that adds a die from a resource to the damage of a weapon attack that
// hit (the superiority_die effect, applies "damage"), end to end. The names are
// invented: a Fighter 5 of the table's subclass "Mestre das Garças" picks two of its
// "golpes". These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	maneuverDiceKey = "dados_de_golpe"
	longsword       = "equipment:longsword"
)

type maneuverTable struct {
	*armed
	heron, bull, grip, guard string
}

// newManeuverTable makes the table's subclass (two damage golpes and one grapple golpe,
// a pool of `uses` dice) and Pensantus, Fighter 5, who picked the two damage ones.
func newManeuverTable(t *testing.T, uses string) *maneuverTable {
	t.Helper()
	m := &maneuverTable{}
	m.armed = newArmedWith(t, func(a *armed) {
		die := func(applies string, extra func(*rulesv1.TableEffect)) []*rulesv1.TableEffect {
			e := &rulesv1.TableEffect{Type: "superiority_die", Applies: applies, Resource: maneuverDiceKey, Value: "8", TextPt: "O alvo faz um teste de Sabedoria."}
			if extra != nil {
				extra(e)
			}
			return []*rulesv1.TableEffect{e}
		}
		golpes := &rulesv1.TableFeature{
			NamePt: "Golpes", DescPt: []string{"Escolha golpes."},
			Effects: []*rulesv1.TableEffect{
				{Type: "resource", Resource: maneuverDiceKey, Max: uses, Recharge: "short_rest"},
				{Type: "choice", Choice: "feature", Count: 4},
			},
			Options: []*rulesv1.TableFeature{
				{NamePt: "Golpe da Garça", DescPt: []string{"Um golpe."}, Effects: die("damage", nil)},
				{NamePt: "Golpe do Touro", DescPt: []string{"Outro golpe."}, Effects: die("damage", nil)},
				{NamePt: "Golpe de Pegada", DescPt: []string{"Uma agarrada."}, Effects: die("grapple", func(e *rulesv1.TableEffect) { e.Economy = "bonus_action" })},
				{NamePt: "Golpe de Guarda", DescPt: []string{"Uma defesa."}, Effects: die("reduce_melee_damage", func(e *rulesv1.TableEffect) { e.Economy = "reaction"; e.Ability = "dex" })},
			},
		}
		sub := &rulesv1.TableSubclass{
			NamePt: "Mestre das Garças", ClassKey: "class:fighter", DescPt: []string{"Golpes."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{golpes}}},
		}
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: sub},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		entry := res.Msg.GetEntry()
		opts := entry.GetTableSubclass().GetLevels()[0].GetFeatures()[0].GetOptions()
		m.heron, m.bull, m.grip, m.guard = opts[0].GetKey(), opts[1].GetKey(), opts[2].GetKey(), opts[3].GetKey()
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, RaceKey: "race:human",
			Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
			Classes: []*charactersv1.ClassLevel{{
				ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: entry.GetKey()},
			}},
			SkillProficiencyKeys: []string{"skill:athletics", "skill:perception"},
			FeatureChoiceKeys:    []string{"feature:fighter-fighting-style-defense", m.heron, m.bull, m.grip, m.guard},
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
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	return m
}

// hit makes Pensantus hit the Capitão and returns the pending damage.
func (m *maneuverTable) hit(t *testing.T, e *playv1.Encounter) *playv1.PendingDamage {
	t.Helper()
	res := m.mustAttack(t, m.ana, e, "Pensantus", longsword, "Capitão Goblin", d20(19))
	if res.GetPendingDamage() == nil {
		t.Fatalf("the attack = %v, want a hit", res.GetRoll())
	}
	return res.GetPendingDamage()
}

func marking(keys ...string) func(*playv1.RollDamageRequest) {
	return func(r *playv1.RollDamageRequest) {
		inAppDamage(r)
		r.ExtrasChosen = true
		for _, k := range keys {
			r.SelectedExtras = append(r.SelectedExtras, &playv1.SelectedExtra{Key: k})
		}
	}
}

func partOf(parts []*playv1.DamagePart, key string) *playv1.DamagePart {
	for _, p := range parts {
		if p.GetKey() == key {
			return p
		}
	}
	return nil
}

func diceLeft(t *testing.T, m *maneuverTable) int32 {
	t.Helper()
	left, _ := poolOf(m.vitals(t, m.pens), maneuverDiceKey)
	return left
}

// The damage step of a hit offers each damage golpe the character picked, with its die and
// its name, and not the grapple one; choosing it adds the die, spends a use and leaves the
// rider to the master alone.
func TestManeuverDamageIsOfferedAfterAHitAndSpendsAUse(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")

	pending := m.hit(t, e)
	heron := partOf(pending.GetParts(), m.heron)
	if heron == nil || !heron.GetChoosable() || !heron.GetAvailable() || heron.GetDiceCount() != 1 || heron.GetDiceSides() != 8 || heron.GetLabelPt() != "Golpe da Garça" {
		t.Fatalf("the offered parts = %v, want Golpe da Garça as 1d8, available", pending.GetParts())
	}
	if partOf(pending.GetParts(), m.bull) == nil {
		t.Errorf("the offered parts = %v, want the other damage golpe too", pending.GetParts())
	}
	if len(pending.GetParts()) != 3 { // the weapon and the two damage golpes: never the grapple one
		t.Errorf("the offered parts = %d, want 3", len(pending.GetParts()))
	}

	m.h.roller.queue(5, 4) // the weapon die, then the golpe's
	dmg := m.mustDamage(t, m.ana, e, pending.GetId(), marking(m.heron)).GetPendingDamage()
	var sum int32
	for _, r := range dmg.GetPartRolls() {
		if r.GetPartKey() == m.heron {
			sum = r.GetSum()
			if !r.GetCounted() || r.GetRiderPt() != "" {
				t.Errorf("the golpe's roll = %v, want it counted with no rider for the player", r)
			}
		}
	}
	if sum != 4 {
		t.Errorf("the part rolls = %v, want the golpe at 4", dmg.GetPartRolls())
	}
	if got := diceLeft(t, m); got != 3 {
		t.Errorf("dice left = %d, want 3", got)
	}
}

// The master reads the rider on the roll of the golpe.
func TestManeuverRiderIsTheMastersToRead(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	pending := m.hit(t, e)
	m.h.roller.queue(5, 4)
	dmg := m.mustDamage(t, m.master, e, pending.GetId(), marking(m.heron)).GetPendingDamage()
	for _, r := range dmg.GetPartRolls() {
		if r.GetPartKey() == m.heron {
			if r.GetRiderPt() != "O alvo faz um teste de Sabedoria." {
				t.Errorf("the master's rider = %q", r.GetRiderPt())
			}
			return
		}
	}
	t.Fatalf("no roll of the golpe in %v", dmg.GetPartRolls())
}

// A golpe not marked costs nothing; a miss opens no damage and so offers none.
func TestManeuverIsNotOfferedAfterAMissAndNotMarkedCostsNothing(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	if miss := m.mustAttack(t, m.ana, e, "Pensantus", longsword, "Capitão Goblin", d20(2)); miss.GetPendingDamage() != nil {
		t.Fatalf("the miss opened a damage: %v", miss.GetPendingDamage())
	}
	if got := diceLeft(t, m); got != 4 {
		t.Errorf("dice left after a miss = %d, want 4", got)
	}
	pending := m.hit(t, e)
	m.mustDamage(t, m.ana, e, pending.GetId(), marking())
	if got := diceLeft(t, m); got != 4 {
		t.Errorf("dice left after a hit with no golpe = %d, want 4", got)
	}
}

// Only one golpe for each attack.
func TestManeuverOnlyOnePerAttack(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	pending := m.hit(t, e)
	if _, err := m.damage(t, m.ana, e, pending.GetId(), marking(m.heron, m.bull)); err == nil {
		t.Fatal("two golpes on one attack are accepted")
	}
	if got := diceLeft(t, m); got != 4 {
		t.Errorf("dice left after the refusal = %d, want 4", got)
	}
}

// With no use left the golpe is shown as unavailable and refused.
func TestManeuverIsRefusedWithNoUseLeft(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "1")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	first := m.hit(t, e)
	m.h.roller.queue(5, 4)
	m.mustDamage(t, m.ana, e, first.GetId(), marking(m.heron))
	if got := diceLeft(t, m); got != 0 {
		t.Fatalf("dice left = %d, want 0", got)
	}
	second := m.hit(t, e) // the second attack of Extra Attack
	if p := partOf(second.GetParts(), m.heron); p == nil || p.GetAvailable() {
		t.Fatalf("the golpe with no use left = %v, want it listed as unavailable", p)
	}
	if _, err := m.damage(t, m.ana, e, second.GetId(), marking(m.bull)); err == nil {
		t.Error("a golpe with no use left is accepted")
	}
}
