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
	// subKey is the subclass that holds both trades: archiving it hides them from the players.
	subKey string
	// entry is that subclass as the table stored it, for editing the trades.
	entry *rulesv1.TableEntry
}

func newPowerTable(t *testing.T) *powerTable {
	t.Helper()
	return newPowerTableWith(t, false)
}

// newPowerTableWith is newPowerTable, and with withDie also a maneuver die of the attack that spends the trades' pool.
func newPowerTableWith(t *testing.T, withDie bool) *powerTable {
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
		features := []*rulesv1.TableFeature{pool, heavy, free}
		if withDie {
			features = append(features, &rulesv1.TableFeature{NamePt: "Golpe Preciso", DescPt: []string{"Um dado."}, Effects: []*rulesv1.TableEffect{
				{Type: "superiority_die", Applies: rules.ManeuverAttack, Resource: powerUses, Value: "8"},
			}})
		}
		sub := &rulesv1.TableSubclass{
			NamePt: "Mestre do Golpe", ClassKey: "class:fighter", DescPt: []string{"Golpes."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: features}},
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
			WeaponKeys:           []string{greatsword, longsword, longbow, dagger, shortsword},
			HitPoints:            &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}, ExperiencePoints: 6500,
		}}}
		made, err := a.caio.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pipo", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Pipo) error = %v", err)
		}
		g.subKey, g.entry = res.Msg.GetEntry().GetKey(), res.Msg.GetEntry()
		a.toren = made.Msg.GetCharacter()
		a.pens = a.ana.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Orla", "class:bard", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, nil, nil)
	})
	return g
}

// fightInspired starts the combat with Orla first, gives Pipo her Bardic Inspiration die and passes to Pipo's turn.
func (g *powerTable) fightInspired(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := g.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: g.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Orla": 20, "Pipo": 19, "Toren": 9},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Pipo": {4, 3}, "Goblin": {4, 4}, "Toren": {3, 4}, "Orla": {3, 3}},
	})
	if _, err := g.giveInspiration(t, g.bia, e, "Orla", "Pipo"); err != nil {
		t.Fatalf("GiveBardicInspiration() error = %v", err)
	}
	e = g.turnOf(t, "Pipo")
	for _, o := range g.mustOptions(t, g.caio, e, "Pipo").GetOptions().GetAttacks() {
		if o.GetAttack().GetKey() == greatsword {
			for _, p := range o.GetPowerAttacks() {
				if p.GetName() == powerHeavyName {
					g.heavyKey = p.GetKey()
				}
			}
		}
	}
	if g.heavyKey == "" {
		t.Fatal("the greatsword lists no heavy trade")
	}
	return e
}

// fight starts a combat where Pipo is first and next to the Goblin, with Toren beside them.
func (g *powerTable) fight(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := g.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: g.goblin.GetId()}, {CharacterId: g.capitao.GetId()}},
		npcRolls: []int{3, 2},
		players:  map[string]int32{"Pipo": 20, "Toren": 9, "Orla": 5},
		reveal:   []string{"Goblin", "Capitão Goblin"},
		at:       map[string][2]int32{"Pipo": {4, 3}, "Goblin": {4, 4}, "Capitão Goblin": {5, 3}, "Toren": {3, 4}, "Orla": {3, 3}},
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
	if want := int32(pipoToHit - heavyPenalty); first.GetRoll().GetD20().GetModifier() != want || again.GetRoll().GetD20().GetModifier() != want {
		t.Errorf("the modifiers = %d and %d, want %d on both: the penalty once", first.GetRoll().GetD20().GetModifier(), again.GetRoll().GetD20().GetModifier(), want)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d after the resend, want 1: one use for two sends", got)
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
		{"a 1 that would reach with no penalty", func(*powerTable) string { return "" }, 1, playv1.AttackOutcome_ATTACK_OUTCOME_MISS, 1 + pipoToHit},
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
			if offer := res.GetHeldRollOffer(); offer.GetLucky() != nil { // Pipo is a halfling: a 1 asks first, and he keeps it
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
		{"a key nobody has", greatsword, "feature:inventado@mesa", "is not a power attack of the character"},
		{"a key that is not a power attack", greatsword, "feature:fighter-fighting-style-defense", "is not a power attack of the character"},
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
	}); connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "is not a power attack of the character") {
		t.Errorf("another character's key: error = %v, want invalid_argument saying the key is not the character's", err)
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
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "weapon attack of a player's character") {
		t.Errorf("an NPC's attack with a trade: error = %v, want invalid_argument naming the player's character", err)
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

// Each check of the attacker refuses on its own: the sheet below holds the key and the line is a weapon in every case
// but the one the check is about, so the refusal's text is the check's (a key lookup or the filter would say another).
func TestAPowerAttackNeverGoesOnASpellAttackOrAnNPCsAttack(t *testing.T) {
	t.Parallel()
	sheet := link.Sheet{Traits: link.Traits{PowerAttacks: []link.PowerAttack{{Key: "x", NamePT: "X", AttackPenalty: 1, DamageBonus: 1, Weapon: "melee"}}}}
	player := playdb.Combatant{Kind: kindPlayer, CharacterID: "c1"}
	for _, tc := range []struct {
		name     string
		attacker playdb.Combatant
		attack   link.Attack
	}{
		{"a spell attack (a weapon line otherwise)", player, link.Attack{Spell: true, Weapon: true, Melee: true}},
		{"a line that is no weapon", player, link.Attack{Melee: true}},
		{"an NPC", playdb.Combatant{Kind: kindNPC, CharacterID: "c2"}, link.Attack{Weapon: true, Melee: true}},
		{"a player's combatant with no character", playdb.Combatant{Kind: kindPlayer}, link.Attack{Weapon: true, Melee: true}},
	} {
		f := &attackFlow{req: &playv1.RollAttackRequest{PowerAttackKey: "x"}, attacker: tc.attacker, attack: tc.attack, sheet: sheet}
		if err := f.checkPowerAttack(t.Context()); connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "weapon attack of a player's character") {
			t.Errorf("%s: %v, want invalid_argument naming the weapon attack of a player's character", tc.name, err)
		}
	}
	// The positive control: the same sheet and a weapon line of a player pass the attacker checks (and the filter).
	ok := &attackFlow{req: &playv1.RollAttackRequest{PowerAttackKey: "x"}, attacker: player, attack: link.Attack{Weapon: true, Melee: true}, sheet: sheet}
	if err := ok.checkPowerAttack(t.Context()); err != nil || ok.power == nil {
		t.Errorf("a weapon attack of a player with the key: %v, power %v; want it accepted", err, ok.power)
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
	offer := held.GetHeldRollOffer()
	if offer.GetLucky() == nil || offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK {
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

// A Bardic Inspiration die holds the roll like Lucky does: the question's d20 already counts the penalty, and the
// answer settles on that bonus (6 - 5 = 1), once. The penalty twice would total 8 + 3 and miss; none would total 17.
func TestAHeldRollForBardicInspirationAppliesThePowerAttackPenaltyOnce(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fightInspired(t)
	held, err := g.swing(t, e, g.heavyKey, 9)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	offer := held.GetHeldRollOffer()
	if offer.GetDie() == nil || offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK {
		t.Fatalf("the roll = %v, want the die's question", held)
	}
	const bonus = pipoToHit - heavyPenalty
	if d := offer.GetD20(); d.GetModifier() != bonus || d.GetTotal() != 9+bonus {
		t.Errorf("the question = modifier %d, total %d; want %d and %d", d.GetModifier(), d.GetTotal(), bonus, 9+bonus)
	}
	if got := g.usesLeft(t); got != 2 {
		t.Fatalf("uses left = %d while the roll is held, want 2", got)
	}
	res, err := g.answer(t, g.caio, e, offer.GetHoldId(), func(r *playv1.AnswerBardicInspirationRequest) {
		r.Use, r.Roll = true, &playv1.AnswerBardicInspirationRequest_TypedFace{TypedFace: 3}
	})
	if err != nil {
		t.Fatalf("AnswerBardicInspiration() error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	// 9 + 6 - 5 + 3 = 13 against 12.
	if d := roll.GetD20(); d.GetTotal() != 9+bonus+3 || roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the answer = modifier %d, total %d, outcome %v; want total %d and a hit", d.GetModifier(), d.GetTotal(), roll.GetOutcome(), 9+bonus+3)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d after the answer, want 1", got)
	}
}

// A reaction window (the master's check of every hit) holds the roll and the replay runs the attack again from the
// request: the penalty is taken off the sheet's bonus once, not on top of what the first pass did.
func TestAReplayedRollAfterAReactionWindowAppliesThePowerAttackPenaltyOnce(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	g.setRules(t, enemyReactionsAlways)
	e := g.fight(t)
	held, err := g.swing(t, e, g.heavyKey, 14)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("the attack = %v, want it held for the master's check", held.GetRoll())
	}
	w := g.windowOf(t, g.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil {
		t.Fatal("the master has no check")
	}
	if got := g.usesLeft(t); got != 2 {
		t.Fatalf("uses left = %d while the roll is held, want 2", got)
	}
	g.mustAnswer(t, g.master, e, w.GetId(), passAnswer)
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d after the replay, want 1 (spent once)", got)
	}
	if w2 := g.windowOf(t, g.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK); w2 != nil { // the hit is asked once more
		g.mustAnswer(t, g.master, e, w2.GetId(), passAnswer)
	}
	var payload string
	if err := g.h.pool.QueryRow(t.Context(), `SELECT payload::TEXT FROM session_events WHERE kind = 'attack_rolled' ORDER BY seq DESC LIMIT 1`).Scan(&payload); err != nil {
		t.Fatalf("read the attack's event: %v", err)
	}
	var ev struct {
		Modifier int32 `json:"modifier"`
		Total    int32 `json:"total"`
	}
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatalf("read the attack's event %s: %v", payload, err)
	}
	if want := int32(pipoToHit - heavyPenalty); ev.Modifier != want || ev.Total != 14+want {
		t.Errorf("the replayed attack's event = modifier %d, total %d; want %d and %d", ev.Modifier, ev.Total, want, 14+want)
	}
}

// The checks, the resource's included, run again on the replay: a pool spent while the roll waited refuses the replay
// and the attack is dropped, writing no attack and spending nothing more.
func TestAHeldPowerAttackWhoseUsesRanOutBeforeTheReplayIsDropped(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	g.setRules(t, enemyReactionsAlways)
	e := g.fight(t)
	held, err := g.swing(t, e, g.heavyKey, 14)
	if err != nil || !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("RollAttack() = %v, %v; want it held for the master's check", held, err)
	}
	w := g.windowOf(t, g.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil {
		t.Fatal("the master has no check")
	}
	tag, err := g.h.pool.Exec(t.Context(), `INSERT INTO character_vitals (character_id, resources_used, revision, updated_at) VALUES ($1, '{"`+powerUses+`": 2}'::JSONB, 1, now())
		ON CONFLICT (character_id) DO UPDATE SET resources_used = character_vitals.resources_used || '{"`+powerUses+`": 2}'::JSONB`, g.toren.GetId())
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("spend the pool: %v, rows %d", err, tag.RowsAffected())
	}
	if got := g.usesLeft(t); got != 0 {
		t.Fatalf("uses left = %d after the test spent the pool, want 0", got)
	}
	g.mustAnswer(t, g.master, e, w.GetId(), passAnswer)
	// What the attack wrote is the held request's marker alone: no roll, so no trade.
	var rolls int
	if err := g.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM session_events WHERE kind = 'attack_rolled' AND (payload ? 'total' OR payload ? 'power_attack')`).Scan(&rolls); err != nil {
		t.Fatalf("count the attacks: %v", err)
	}
	if rolls != 0 {
		t.Errorf("rolled attacks = %d, want 0: the replay was refused", rolls)
	}
	told := false
	for _, r := range g.log(t, g.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			told = told || strings.Contains(en.GetReactionTextPt(), "não pôde acontecer")
		}
	}
	if !told {
		t.Error("the master's log has no line for the dropped attack")
	}
	if g.pipoActionUsed(t) {
		t.Error("the dropped attack spent the action")
	}
	if got := g.usesLeft(t); got != 0 {
		t.Errorf("uses left = %d, want 0 (the pool the test spent)", got)
	}
}

// The trade goes on every way a character attacks: as the master rolling for the player's character, an
// opportunity attack, an attack as a reaction and a bonus action attack. Each lowers the roll and spends (or not) once.
func TestAPowerAttackGoesOnEveryWayOfAttacking(t *testing.T) {
	t.Parallel()
	const want = pipoToHit - heavyPenalty
	t.Run("the master rolling for the character", func(t *testing.T) {
		t.Parallel()
		g := newPowerTable(t)
		e := g.fight(t)
		res, err := g.attack(t, g.master, e, "Pipo", greatsword, "Goblin", func(r *playv1.RollAttackRequest) { d20(14)(r); r.PowerAttackKey = g.heavyKey })
		if err != nil {
			t.Fatalf("RollAttack() error = %v", err)
		}
		if d := res.GetRoll().GetD20(); d.GetModifier() != want || d.GetTotal() != 14+want || g.usesLeft(t) != 1 {
			t.Errorf("the master's roll = modifier %d, total %d, uses left %d; want %d, %d and 1", d.GetModifier(), d.GetTotal(), g.usesLeft(t), want, 14+want)
		}
	})
	t.Run("an opportunity attack", func(t *testing.T) {
		t.Parallel()
		g := newPowerTable(t)
		e := g.fight(t)
		g.passTo(t, e, "Goblin")
		offerID := g.mustOffer(t, e, "Goblin", "Pipo").GetOpportunityOfferId()
		res, err := g.attackAs(t, g.caio, e, "Pipo", greatsword, "Goblin", func(r *playv1.RollAttackRequest) {
			d20(14)(r)
			r.OpportunityOfferId, r.PowerAttackKey = offerID, g.heavyKey
		}, false)
		if err != nil {
			t.Fatalf("RollAttack(opportunity) error = %v", err)
		}
		if d := res.GetRoll().GetD20(); d.GetModifier() != want || d.GetTotal() != 14+want || g.usesLeft(t) != 1 {
			t.Errorf("the opportunity attack = modifier %d, total %d, uses left %d; want %d, %d and 1", d.GetModifier(), d.GetTotal(), g.usesLeft(t), want, 14+want)
		}
	})
	t.Run("an attack as a reaction", func(t *testing.T) {
		t.Parallel()
		g := newPowerTable(t)
		e := g.fight(t)
		g.mustEndTurn(t, g.caio, e)
		res, err := g.attackAs(t, g.caio, e, "Pipo", greatsword, "Goblin", func(r *playv1.RollAttackRequest) { d20(14)(r); r.PowerAttackKey = g.heavyKey }, true)
		if err != nil {
			t.Fatalf("RollAttack(as a reaction) error = %v", err)
		}
		if d := res.GetRoll().GetD20(); d.GetModifier() != want || d.GetTotal() != 14+want || g.usesLeft(t) != 1 {
			t.Errorf("the reaction attack = modifier %d, total %d, uses left %d; want %d, %d and 1", d.GetModifier(), d.GetTotal(), g.usesLeft(t), want, 14+want)
		}
	})
	t.Run("a bonus action attack", func(t *testing.T) {
		t.Parallel()
		g := newPowerTable(t)
		e := g.fight(t)
		for range 2 { // Extra Attack: both attacks of the Attack action, with a light weapon
			if _, err := g.swingAt(t, e, dagger, "Goblin", "", 14); err != nil {
				t.Fatalf("RollAttack(dagger) error = %v", err)
			}
		}
		res, err := g.swingAt(t, e, shortsword, "Goblin", g.freeKey, 14)
		if err != nil {
			t.Fatalf("RollAttack(the off hand) error = %v", err)
		}
		const free = pipoToHit - freePenalty
		if d := res.GetRoll().GetD20(); d.GetModifier() != free || d.GetTotal() != 14+free {
			t.Errorf("the off hand = modifier %d, total %d; want %d and %d", d.GetModifier(), d.GetTotal(), free, 14+free)
		}
		if !byLabel(t, g.get(t, g.master), "Pipo").GetBonusActionUsed() {
			t.Error("the attack was not the bonus action's")
		}
	})
}
