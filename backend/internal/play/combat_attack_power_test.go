package play

import (
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The power_attack effect of a table feature on an attack (RollAttackRequest.power_attack_key): the penalty comes off
// the roll, the request is checked against the sheet, the use of the resource is spent with the attack and the options
// list the trades. Pipo (a halfling fighter 5, Caio's) has two of them: "Golpe Pesado", 5 off the roll for 10 damage with a
// heavy melee weapon, from a pool of two uses, and "Golpe Livre", 10 off the roll for 2 damage with any melee weapon and
// no resource. He hits at +6 (Strength 16, proficiency 3); the Goblin has armor class 12 and the Capitão 18.

const (
	powerUses      = "golpes_pesados"
	greatsword     = "equipment:greatsword"
	longbow        = "equipment:longbow"
	heavyPenalty   = 5
	heavyDamage    = 10
	freePenalty    = 10
	freeDamage     = 2
	pipoToHit      = 6
	powerHeavyName = "Golpe Pesado"
	powerFreeName  = "Golpe Livre"
)

type powerTable struct {
	*armed
	heavyKey, freeKey string
}

func newPowerTable(t *testing.T) *powerTable {
	t.Helper()
	g := &powerTable{}
	g.armed = newArmedWith(t, func(a *armed) {
		pool := &rulesv1.TableFeature{NamePt: "Reserva de Golpes", DescPt: []string{"Usos."}, Effects: []*rulesv1.TableEffect{
			{Type: "resource", Resource: powerUses, Max: "2", Recharge: "short_rest"},
		}}
		heavy := &rulesv1.TableFeature{NamePt: powerHeavyName, DescPt: []string{"Troca precisão por dano."}, Effects: []*rulesv1.TableEffect{{
			Type: "power_attack", AttackPenalty: heavyPenalty, DamageBonus: heavyDamage, Weapon: "melee", OnlyProficient: true,
			WeaponProperties: []string{"weapon-property:heavy"}, Resource: powerUses, TextPt: "Só com armas pesadas.",
		}}}
		free := &rulesv1.TableFeature{NamePt: powerFreeName, DescPt: []string{"Troca precisão por dano."}, Effects: []*rulesv1.TableEffect{{
			Type: "power_attack", AttackPenalty: freePenalty, DamageBonus: freeDamage, Weapon: "melee",
		}}}
		sub := &rulesv1.TableSubclass{
			NamePt: "Mestre do Golpe", ClassKey: "class:fighter", DescPt: []string{"Golpes."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{pool, heavy, free}}},
		}
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: sub},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10}, RaceKey: "race:halfling",
			Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
			Classes: []*charactersv1.ClassLevel{{
				ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: res.Msg.GetEntry().GetKey()},
			}},
			Languages:            []string{"Anão", "Élfico", "Gnômico"},
			SkillProficiencyKeys: []string{"skill:athletics", "skill:perception"},
			FeatureChoiceKeys:    []string{"feature:fighter-fighting-style-defense"},
			WeaponKeys:           []string{greatsword, longsword, longbow},
			HitPoints:            &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}, ExperiencePoints: 6500,
		}}}
		made, err := a.caio.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pipo", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Pipo) error = %v", err)
		}
		a.toren = made.Msg.GetCharacter()
		a.pens = a.ana.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
	})
	return g
}

// fight starts a combat where Pipo is first and next to the Goblin, with Toren beside them.
func (g *powerTable) fight(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := g.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: g.goblin.GetId()}, {CharacterId: g.capitao.GetId()}},
		npcRolls: []int{3, 2},
		players:  map[string]int32{"Pipo": 20, "Toren": 9},
		reveal:   []string{"Goblin", "Capitão Goblin"},
		at:       map[string][2]int32{"Pipo": {4, 3}, "Goblin": {4, 4}, "Capitão Goblin": {5, 3}, "Toren": {3, 4}},
	})
	// The keys of the trades are the ones the options list.
	for _, o := range g.mustOptions(t, g.caio, e, "Pipo").GetOptions().GetAttacks() {
		if o.GetAttack().GetKey() != greatsword {
			continue
		}
		for _, p := range o.GetPowerAttacks() {
			switch p.GetName() {
			case powerHeavyName:
				g.heavyKey = p.GetKey()
			case powerFreeName:
				g.freeKey = p.GetKey()
			}
		}
	}
	if g.heavyKey == "" || g.freeKey == "" {
		t.Fatalf("the greatsword lists the trades %q and %q, want both", g.heavyKey, g.freeKey)
	}
	return e
}

func (g *powerTable) usesLeft(t *testing.T) int32 {
	t.Helper()
	left, _ := poolOf(g.vitals(t, g.toren), powerUses)
	return left
}

// swing attacks as Pipo's player with the greatsword on the Goblin, a typed d20 and the trade named.
func (g *powerTable) swing(t *testing.T, e *playv1.Encounter, key string, face int32, edit ...func(*playv1.RollAttackRequest)) (*playv1.RollAttackResponse, error) {
	t.Helper()
	return g.swingAt(t, e, greatsword, "Goblin", key, face, edit...)
}

func (g *powerTable) swingAt(t *testing.T, e *playv1.Encounter, weapon, target, key string, face int32, edit ...func(*playv1.RollAttackRequest)) (*playv1.RollAttackResponse, error) {
	t.Helper()
	return g.attack(t, g.caio, e, "Pipo", weapon, target, func(r *playv1.RollAttackRequest) {
		d20(face)(r)
		r.PowerAttackKey = key
		for _, f := range edit {
			f(r)
		}
	})
}

func (g *powerTable) eventCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := g.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM session_events`).Scan(&n); err != nil {
		t.Fatalf("count the events: %v", err)
	}
	return n
}

func (g *powerTable) pipoActionUsed(t *testing.T) bool {
	t.Helper()
	return byLabel(t, g.get(t, g.master), "Pipo").GetActionUsed()
}

func TestAPowerAttackLowersTheRollAndSpendsAUse(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)

	plain, err := g.swing(t, e, "", 14)
	if err != nil {
		t.Fatalf("RollAttack(no trade) error = %v", err)
	}
	if d := plain.GetRoll().GetD20(); d.GetModifier() != pipoToHit || d.GetTotal() != 14+pipoToHit {
		t.Fatalf("the plain attack = modifier %d, total %d; want %d and %d", d.GetModifier(), d.GetTotal(), pipoToHit, 14+pipoToHit)
	}
	if got := g.usesLeft(t); got != 2 {
		t.Fatalf("uses left = %d, want 2 (an attack that names no trade spends none)", got)
	}

	traded, err := g.swing(t, e, g.heavyKey, 14)
	if err != nil {
		t.Fatalf("RollAttack(the heavy trade) error = %v", err)
	}
	d := traded.GetRoll().GetD20()
	if want := int32(pipoToHit - heavyPenalty); d.GetModifier() != want || d.GetTotal() != 14+want {
		t.Errorf("the trade = modifier %d, total %d; want %d and %d", d.GetModifier(), d.GetTotal(), want, 14+want)
	}
	if traded.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the trade's outcome = %v, want a hit (15 against 12)", traded.GetRoll().GetOutcome())
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

// What the trade was is on the event the attack wrote, so a resend finds the same and the undo needs nothing more.
func TestThePowerAttackIsOnTheEventAndAResendIsNotAppliedTwice(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	key := newKey()
	send := func() *playv1.RollAttackResponse {
		res, err := g.swing(t, e, g.heavyKey, 14, func(r *playv1.RollAttackRequest) { r.IdempotencyKey = key })
		if err != nil {
			t.Fatalf("RollAttack() error = %v", err)
		}
		return res
	}
	first := send()
	events := g.eventCount(t)
	again := send()
	if again.GetRoll().GetD20().GetModifier() != first.GetRoll().GetD20().GetModifier() || again.GetRoll().GetD20().GetTotal() != first.GetRoll().GetD20().GetTotal() {
		t.Errorf("the resend = %v, want the roll of the first", again.GetRoll().GetD20())
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d after the resend, want 1", got)
	}
	if got := g.eventCount(t); got != events {
		t.Errorf("events = %d after the resend, want %d", got, events)
	}
	var payload string
	if err := g.h.pool.QueryRow(t.Context(), `SELECT payload::TEXT FROM session_events WHERE kind = 'attack_rolled' ORDER BY seq DESC LIMIT 1`).Scan(&payload); err != nil {
		t.Fatalf("read the attack's event: %v", err)
	}
	var stored struct {
		PowerAttack *struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Penalty     int    `json:"penalty"`
			DamageBonus int    `json:"damage_bonus"`
		} `json:"power_attack"`
	}
	if err := json.Unmarshal([]byte(payload), &stored); err != nil {
		t.Fatalf("read the attack's event %s: %v", payload, err)
	}
	if p := stored.PowerAttack; p == nil || p.Key != g.heavyKey || p.Name != powerHeavyName || p.Penalty != heavyPenalty || p.DamageBonus != heavyDamage {
		t.Errorf("the attack's event = %s, want the trade's key, name, penalty and damage bonus", payload)
	}
}

func TestTheUndoGivesBackTheUseOfAPowerAttack(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	if _, err := g.swing(t, e, g.heavyKey, 14); err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Fatalf("uses left = %d, want 1", got)
	}
	g.undoLast(t, e)
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d after the undo, want 2", got)
	}
	if g.pipoActionUsed(t) {
		t.Error("the action stays spent after the undo")
	}
}

func TestAPowerAttackWithNoResourceSpendsNothing(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	res, err := g.swing(t, e, g.freeKey, 20)
	if err != nil {
		t.Fatalf("RollAttack(the free trade) error = %v", err)
	}
	if d := res.GetRoll().GetD20(); d.GetModifier() != int32(pipoToHit-freePenalty) {
		t.Errorf("modifier = %d, want %d", d.GetModifier(), pipoToHit-freePenalty)
	}
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// A natural 20 hits and a natural 1 misses with the penalty, whatever the total says: against the Capitão's armor class
// of 18 a 20 with the free trade totals 16.
func TestTheNaturalTwentyHitsAndTheNaturalOneMissesWithThePenalty(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		trade   func(g *powerTable) string
		face    int32
		outcome playv1.AttackOutcome
		total   int32
	}{
		{"a 20 that falls short of 18", func(g *powerTable) string { return g.freeKey }, 20, playv1.AttackOutcome_ATTACK_OUTCOME_CRITICAL_HIT, 20 + pipoToHit - freePenalty},
		{"a 19 that falls short of 18", func(g *powerTable) string { return g.freeKey }, 19, playv1.AttackOutcome_ATTACK_OUTCOME_MISS, 19 + pipoToHit - freePenalty},
		{"a 1 that would reach with no penalty", func(g *powerTable) string { return "" }, 1, playv1.AttackOutcome_ATTACK_OUTCOME_MISS, 1 + pipoToHit},
		{"a 1 with the penalty", func(g *powerTable) string { return g.freeKey }, 1, playv1.AttackOutcome_ATTACK_OUTCOME_MISS, 1 + pipoToHit - freePenalty},
		{"a 18 with the heavy trade that still reaches 18", func(g *powerTable) string { return g.heavyKey }, 18, playv1.AttackOutcome_ATTACK_OUTCOME_HIT, 18 + pipoToHit - heavyPenalty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := newPowerTable(t)
			e := g.fight(t)
			res, err := g.swingAt(t, e, greatsword, "Capitão Goblin", tc.trade(g), tc.face)
			if err != nil {
				t.Fatalf("RollAttack() error = %v", err)
			}
			if offer := res.GetLuckyOffer(); offer != nil { // Pipo is a halfling: a 1 asks first, and he keeps it
				answer, err := g.answerLucky(t, g.caio, e, offer.GetHoldId(), keepTheOne)
				if err != nil {
					t.Fatalf("AnswerLucky(keep) error = %v", err)
				}
				res = answer.GetAttack()
			}
			if res.GetRoll().GetOutcome() != tc.outcome || res.GetRoll().GetD20().GetTotal() != tc.total {
				t.Errorf("the attack = outcome %v, total %d; want %v and %d", res.GetRoll().GetOutcome(), res.GetRoll().GetD20().GetTotal(), tc.outcome, tc.total)
			}
		})
	}
}

// Nothing is written for a trade the sheet or the weapon refuses: no event, no action, no use.
func TestAPowerAttackTheSheetOrTheWeaponRefusesWritesNothing(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	events := g.eventCount(t)
	for _, tc := range []struct {
		name   string
		weapon string
		key    string
		want   string
	}{
		{"a weapon without the property", longsword, g.heavyKey, "sem a propriedade Pesada"},
		{"a ranged weapon for a melee trade", longbow, g.freeKey, "arma de outro tipo"},
		{"a key nobody has", greatsword, "feature:inventado@mesa", ""},
		{"a key that is not a power attack", greatsword, "feature:fighter-fighting-style-defense", ""},
	} {
		_, err := g.swingAt(t, e, tc.weapon, "Goblin", tc.key, 14)
		if connect.CodeOf(err) != connect.CodeInvalidArgument || (tc.want != "" && !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s: error = %v, want invalid_argument %q", tc.name, err, tc.want)
		}
	}
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
	if g.pipoActionUsed(t) {
		t.Error("a refused attack spent the action")
	}
	if got := g.eventCount(t); got != events {
		t.Errorf("events = %d, want %d: a refused attack wrote one", got, events)
	}
	// Toren has no power attack: Pipo's key is not his.
	g.passTo(t, e, "Toren")
	events = g.eventCount(t)
	if _, err := g.attack(t, g.ana, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) {
		d20(14)(r)
		r.PowerAttackKey = g.heavyKey
	}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("another character's key: error = %v, want invalid_argument", err)
	}
	if got := g.eventCount(t); got != events {
		t.Errorf("events = %d, want %d: Toren's refused attack wrote one", got, events)
	}
}

func TestAPowerAttackOfAnNPCAttackerIsRefused(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	g.passTo(t, e, "Goblin")
	_, err := g.attack(t, g.master, g.get(t, g.master), "Goblin", "basic:0", "Pipo", func(r *playv1.RollAttackRequest) {
		d20(14)(r)
		r.PowerAttackKey = g.heavyKey
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("an NPC's attack with a trade: error = %v, want invalid_argument", err)
	}
}

func TestAPowerAttackWithNoUseLeftIsBlocked(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	for range 2 {
		if _, err := g.swing(t, e, g.heavyKey, 14); err != nil {
			t.Fatalf("RollAttack() error = %v", err)
		}
	}
	g.passTo(t, e, "Toren")
	g.passTo(t, e, "Pipo") // the next round: the action is whole again
	events := g.eventCount(t)
	_, err := g.swing(t, e, g.heavyKey, 14)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT)
	if got := g.eventCount(t); got != events {
		t.Errorf("events = %d, want %d: the refused attack wrote one", got, events)
	}
	if _, err := g.swing(t, e, g.freeKey, 14); err != nil {
		t.Errorf("the free trade after the pool is spent: error = %v, want none", err)
	}
}

// A spell attack and an NPC attacker never take a trade, whatever the key says (as the maneuver die).
func TestAPowerAttackNeverGoesOnASpellAttackOrAnNPCsAttack(t *testing.T) {
	t.Parallel()
	player := playdb.Combatant{Kind: kindPlayer, CharacterID: "c1"}
	for _, tc := range []struct {
		name     string
		attacker playdb.Combatant
		attack   link.Attack
	}{
		{"a spell attack", player, link.Attack{Spell: true}},
		{"an NPC", playdb.Combatant{Kind: kindNPC, CharacterID: "c2"}, link.Attack{Weapon: true}},
	} {
		f := &attackFlow{req: &playv1.RollAttackRequest{PowerAttackKey: "x"}, attacker: tc.attacker, attack: tc.attack}
		if err := f.checkPowerAttack(t.Context()); connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "weapon attack of a player's character") {
			t.Errorf("%s: %v, want invalid_argument naming the weapon attack of a player's character", tc.name, err)
		}
	}
	if err := (&attackFlow{req: &playv1.RollAttackRequest{}, attacker: playdb.Combatant{Kind: kindNPC}}).checkPowerAttack(t.Context()); err != nil {
		t.Errorf("an attack that names no trade: %v, want none", err)
	}
}

func TestTheTurnOptionsListThePowerAttacksTheWeaponGoesWith(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	names := func(o *rulesv1.AttackOption) []string {
		var out []string
		for _, p := range o.GetPowerAttacks() {
			out = append(out, p.GetName())
		}
		return out
	}
	for _, viewer := range []*user{g.caio, g.master} {
		got := map[string][]string{}
		var heavy *rulesv1.PowerAttackOption
		for _, o := range g.mustOptions(t, viewer, e, "Pipo").GetOptions().GetAttacks() {
			got[o.GetAttack().GetKey()] = names(o)
			if o.GetAttack().GetKey() == greatsword {
				for _, p := range o.GetPowerAttacks() {
					if p.GetName() == powerHeavyName {
						heavy = p
					}
				}
			}
		}
		want := map[string][]string{
			greatsword:             {powerHeavyName, powerFreeName},
			longsword:              {powerFreeName},
			longbow:                nil, // a melee trade does not go with a ranged weapon
			rules.UnarmedStrikeKey: {powerFreeName},
		}
		for key, w := range want {
			if g := got[key]; strings.Join(g, ",") != strings.Join(w, ",") {
				t.Errorf("the options of %s list %v for %s, want %v", key, g, key, w)
			}
		}
		if heavy.GetKey() != g.heavyKey || heavy.GetPenalty() != heavyPenalty || heavy.GetDamageBonus() != heavyDamage {
			t.Errorf("the heavy trade = %v, want key %s, penalty %d and damage %d", heavy, g.heavyKey, heavyPenalty, heavyDamage)
		}
	}
	// A character with no such feature and an NPC list none.
	for label, u := range map[string]*user{"Toren": g.ana, "Goblin": g.master} {
		for _, o := range g.mustOptions(t, u, e, label).GetOptions().GetAttacks() {
			if len(o.GetPowerAttacks()) != 0 {
				t.Errorf("%s's %s lists %v, want no power attack", label, o.GetAttack().GetKey(), o.GetPowerAttacks())
			}
		}
	}
}

// The held roll of a halfling's 1 (Lucky) keeps the bonus it was made with, penalty included: the answer's roll takes the
// penalty off once, not twice, and the use is spent on the replay.
func TestAHeldRollAppliesThePowerAttackPenaltyOnce(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	held, err := g.swing(t, e, g.heavyKey, 1)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	offer := held.GetLuckyOffer()
	if offer == nil {
		t.Fatalf("the roll = %v, want Lucky's question", held)
	}
	if d := offer.GetD20(); d.GetModifier() != int32(pipoToHit-heavyPenalty) || d.GetTotal() != 1+pipoToHit-heavyPenalty {
		t.Errorf("the offer = modifier %d, total %d; want %d and %d", d.GetModifier(), d.GetTotal(), pipoToHit-heavyPenalty, 1+pipoToHit-heavyPenalty)
	}
	if got := g.usesLeft(t); got != 2 {
		t.Fatalf("uses left = %d while the roll is held, want 2", got)
	}
	res, err := g.answerLucky(t, g.caio, e, offer.GetHoldId(), rerollTyped(15))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	// 15 + 6 - 5 = 16 against 12: a hit. The penalty twice would make it 11, a miss.
	if d := roll.GetD20(); d.GetModifier() != int32(pipoToHit-heavyPenalty) || d.GetTotal() != 15+pipoToHit-heavyPenalty || roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the answer = modifier %d, total %d, outcome %v; want %d, %d and a hit", d.GetModifier(), d.GetTotal(), roll.GetOutcome(), pipoToHit-heavyPenalty, 15+pipoToHit-heavyPenalty)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d after the answer, want 1: spent once, on the replay", got)
	}
	g.undoLast(t, e)
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d after the undo, want 2", got)
	}
}
