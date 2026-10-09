package play

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The combat highlights, "Destaques do combate" (MR-032, D8, RN-20; question
// 64 with its default). The canonical fight (scratch timeline, numbers in
// design/etapa8/README-A.md) is the main test; the rest of the rules are
// worked on fabricated events, with no database, and on a smaller fight with an
// undo, a heal and a critical hit.

// highlightRPCs are the CombatService methods of this slice; the authorization
// matrix of the encounter leaves them to this file's.
var highlightRPCs = []string{"GetCombatHighlights"}

// highlights calls GetCombatHighlights as u.
func (a *armed) highlights(t *testing.T, u *user, e *playv1.Encounter) (*playv1.GetCombatHighlightsResponse, error) {
	t.Helper()
	res, err := u.combat.GetCombatHighlights(t.Context(), connect.NewRequest(&playv1.GetCombatHighlightsRequest{CampaignId: a.campaignID, EncounterId: e.GetId()}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustHighlights(t *testing.T, u *user, e *playv1.Encounter) *playv1.GetCombatHighlightsResponse {
	t.Helper()
	res, err := a.highlights(t, u, e)
	if err != nil {
		t.Fatalf("GetCombatHighlights() error = %v", err)
	}
	return res
}

// endEncounter ends the combat as the master.
func (a *armed) endEncounter(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
}

// categories writes the categories as "kind value: winners", for comparing.
func categories(res *playv1.GetCombatHighlightsResponse) []string {
	out := []string{}
	for _, c := range res.GetCategories() {
		var who []string
		for _, w := range c.GetWinners() {
			who = append(who, w.GetName())
		}
		out = append(out, strings.TrimPrefix(c.GetKind().String(), "HIGHLIGHT_KIND_")+" "+itoa(c.GetValue())+": "+strings.Join(who, ", "))
	}
	return out
}

func itoa(n int32) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func wantCategories(t *testing.T, who string, res *playv1.GetCombatHighlightsResponse, want ...string) {
	t.Helper()
	if got := categories(res); !slices.Equal(got, append([]string{}, want...)) {
		t.Errorf("%s: categories = %q, want %q", who, got, want)
	}
}

// The canonical fight: Toren dealt 23 (7 + 9 + 7: a 9 and a 10 rolled on
// 7-hit-point goblins count 7), Pensantus 17, Brisa 8; Brisa took 24 (5 + 11 +
// 8: the arrow that hit her at 0 hit points is a death save failure, no hit
// points) and the others 10 and 0; Pensantus and Toren two final blows each;
// nobody healed and nobody rolled a critical (the goblin's is an NPC's). So
// Mais cura and Acertos críticos are left out, and Golpe final is a tie.
func TestMR032_HighlightsOfTheAmbush(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		scores := func(str, dex, con, intl int32) *rulesv1.AbilityScores {
			return &rulesv1.AbilityScores{Strength: str, Dexterity: dex, Constitution: con, Intelligence: intl, Wisdom: 10, Charisma: 8}
		}
		// Brisa's Constitution 18 makes her 24 hit points, as the timeline says.
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores(16, 13, 14, 10), []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, scores(10, 14, 12, 16), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores(10, 16, 18, 10), []string{rapier}, nil)
	})
	if hp := a.vitals(t, a.bri).GetHitPointsMax(); hp != 24 {
		t.Fatalf("Brisa has %d hit points, want 24", hp)
	}
	e := a.start(t, plan{
		npcs: []*playv1.Participant{
			{CharacterId: a.capitao.GetId(), Hidden: new(false)},
			{CharacterId: a.goblin.GetId(), Count: 3}, // hidden until the master shows them
		},
		npcRolls: []int{16, 12, 12, 9},
		players:  map[string]int32{"Brisa": 16, "Pensantus": 12, "Toren": 5},
		reveal:   []string{"Goblin 1", "Goblin 2"},
		at: map[string][2]int32{
			"Brisa": {9, 4}, "Capitão Goblin": {10, 5}, "Pensantus": {8, 2}, "Goblin 1": {6, 5}, "Goblin 2": {14, 2}, "Goblin 3": {12, 8}, "Toren": {5, 5},
		},
	})
	if got := strings.Join(labels(a.get(t, a.master)), ", "); got != "Brisa, Capitão Goblin, Pensantus, Goblin 1, Goblin 2, Goblin 3, Toren" {
		t.Fatalf("order = %s", got)
	}

	// An attack and, when it hits, its damage: the NPC takes it at once, a
	// player's character when the master applies it.
	attackAndApply := func(u *user, attacker, key, target string, d20, die int) {
		t.Helper()
		a.h.roller.queue(d20, die)
		hit := a.mustAttack(t, u, e, attacker, key, target, inAppRoll)
		if hit.GetPendingDamage() == nil {
			t.Fatalf("%s's attack on %s = %v, want a hit", attacker, target, hit.GetRoll())
		}
		dmg := a.mustDamage(t, u, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
		if dmg.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
			if _, err := a.settle(t, a.master, e, dmg.GetId(), true); err != nil {
				t.Fatalf("ApplyPendingDamage() error = %v", err)
			}
		}
	}
	miss := func(u *user, attacker, key, target string) {
		t.Helper()
		a.h.roller.queue(2)
		if hit := a.mustAttack(t, u, e, attacker, key, target, inAppRoll); hit.GetPendingDamage() != nil {
			t.Fatalf("%s's attack on %s hit, want a miss", attacker, target)
		}
	}
	end := func(u *user) { t.Helper(); a.mustEndTurn(t, u, e) }
	move := func(u *user, label string, col, row int32) {
		t.Helper()
		if _, err := u.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Col: col, Row: row,
		})); err != nil {
			t.Fatalf("MoveCombatant(%s) error = %v", label, err)
		}
	}

	// Rodada 1.
	if _, err := a.action(t, a.bia, e, "Brisa", "standard:hide"); err != nil {
		t.Fatalf("TakeAction(hide) error = %v", err)
	}
	end(a.bia)
	attackAndApply(a.master, "Capitão Goblin", sword, "Toren", 15, 3) // 5 to Toren
	end(a.master)
	end(a.ana)
	end(a.master)
	attackAndApply(a.master, "Goblin 2", shortBow, "Brisa", 15, 3) // 5 to Brisa
	end(a.master)
	if _, err := a.action(t, a.master, e, "Goblin 3", "standard:hide"); err != nil {
		t.Fatalf("TakeAction(hide, Goblin 3) error = %v", err)
	}
	end(a.master)
	attackAndApply(a.caio, "Toren", battleaxe, "Goblin 1", 15, 6) // 9 on a goblin of 7: 7, a final blow
	end(a.caio)

	// Rodada 2.
	attackAndApply(a.bia, "Brisa", rapier, "Capitão Goblin", 15, 5) // 8 on the Capitão
	end(a.bia)
	a.h.roller.queue(15) // the Capitão shoots with an enemy next to him: two d20
	attackAndApply(a.master, "Capitão Goblin", shortBow, "Toren", 15, 3) // 5 more to Toren
	end(a.master)
	attackAndApply(a.ana, "Pensantus", fireBolt, "Goblin 2", 13, 7) // 7 on a goblin of 7: a final blow
	end(a.ana)
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin 3"), IdempotencyKey: newKey(), Hidden: false,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	a.h.roller.queue(20, 5, 4)
	crit := a.mustAttack(t, a.master, e, "Goblin 3", sword, "Brisa", inAppRoll) // an NPC's critical: 11 to Brisa
	dmg := a.mustDamage(t, a.master, e, crit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	if _, err := a.settle(t, a.master, e, dmg.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	end(a.master)
	a.setPhysical(t, a.caio)
	move(a.caio, "Toren", 9, 5)
	physical := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(16))
	a.mustDamage(t, a.caio, e, physical.GetPendingDamage().GetId(), typedDamage(6)) // 9 on the Capitão
	end(a.caio)

	// Rodada 3.
	miss(a.bia, "Brisa", rapier, "Capitão Goblin")
	end(a.bia)
	attackAndApply(a.master, "Capitão Goblin", sword, "Brisa", 15, 6) // 8 to Brisa: she has 8 left, so 0
	end(a.master)
	attackAndApply(a.ana, "Pensantus", fireBolt, "Capitão Goblin", 15, 10) // 10 on the Capitão's last 10: a final blow
	end(a.ana)
	attackAndApply(a.master, "Goblin 3", shortBow, "Brisa", 15, 3) // 5 at 0 hit points: a death save failure
	end(a.master)
	move(a.caio, "Toren", 11, 7)                                       // next to Goblin 3
	a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin 3", d20(2)) // a miss
	end(a.caio)

	// Rodada 4: the combat is not over until the master ends it, and the
	// highlights wait for that.
	end(a.master) // Brisa, down, owes a death save: the master passes the turn
	miss(a.ana, "Pensantus", fireBolt, "Goblin 3")
	end(a.ana)
	end(a.master) // Goblin 3
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin 3", d20(15))
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(7)) // 10 on a goblin of 7: 7, a final blow

	_, err := a.highlights(t, a.master, e)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ENDED)
	_, err = a.highlights(t, a.ana, e)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ENDED)
	a.endEncounter(t, e)

	want := []string{
		"MOST_DAMAGE 23: Toren",
		"TANK 24: Brisa",
		"FINAL_BLOW 2: Pensantus, Toren",
	}
	// The master: the categories, and the table of every number, in the order of
	// the combat (Brisa, Pensantus, Toren), zeros included.
	res := a.mustHighlights(t, a.master, e)
	wantCategories(t, "the master", res, want...)
	type row struct {
		name                               string
		dealt, healed, taken, blows, crits int32
	}
	var table []row
	for _, c := range res.GetCharacters() {
		table = append(table, row{c.GetName(), c.GetDamageDealt(), c.GetHealingDone(), c.GetDamageTaken(), c.GetFinalBlows(), c.GetCriticalHits()})
	}
	wantTable := []row{{"Brisa", 8, 0, 24, 0, 0}, {"Pensantus", 17, 0, 0, 2, 0}, {"Toren", 23, 0, 10, 2, 0}}
	if !slices.Equal(table, wantTable) {
		t.Errorf("the master's table = %+v, want %+v", table, wantTable)
	}

	// A player: the same categories and numbers, and of the table exactly one
	// row, their own character's (zeros included): never anyone else's (RN-20).
	// Nothing of an NPC is named: not a name, not an ID.
	for who, c := range map[string]struct {
		u   *user
		own row
	}{"Pensantus's player": {a.ana, row{"Pensantus", 17, 0, 0, 2, 0}}, "Toren's player": {a.caio, row{"Toren", 23, 0, 10, 2, 0}}} {
		u := c.u
		res := a.mustHighlights(t, u, e)
		wantCategories(t, who, res, want...)
		var mine []row
		for _, ch := range res.GetCharacters() {
			mine = append(mine, row{ch.GetName(), ch.GetDamageDealt(), ch.GetHealingDone(), ch.GetDamageTaken(), ch.GetFinalBlows(), ch.GetCriticalHits()})
		}
		if !slices.Equal(mine, []row{c.own}) {
			t.Errorf("%s got the rows %+v, want only their own %+v", who, mine, c.own)
		}
		text := asJSON(t, res)
		for _, banned := range []string{"Goblin", "Capitão", a.capitao.GetId(), a.goblin.GetId()} {
			if strings.Contains(text, banned) {
				t.Errorf("%s was sent %q:\n%s", who, banned, text)
			}
		}
	}
}

// The undo, a heal, a critical hit and the hit points actually removed, on a
// real fight: an action the master took back counts for nothing, a critical hit
// counts for who rolled it, Retomar o fôlego counts what it really healed, a 19
// on a 7-hit-point goblin is 7, the master's own hand on an NPC counts for
// nobody, and damage to an NPC the master still hides is a number like any
// other (RN-20), named nowhere.
func TestMR032_HighlightsSkipTheUndoneAndCountWhatHappened(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{
			{CharacterId: a.goblin.GetId(), Hidden: new(false)},
			{CharacterId: a.capitao.GetId()}, // hidden: a new NPC starts hidden
		},
		npcRolls: []int{1, 2},
		players:  map[string]int32{"Toren": 20, "Pensantus": 15, "Brisa": 10},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Capitão Goblin": {12, 3}, "Brisa": {11, 3}},
	})
	master := a.master
	// hit is an attack that hits, and its damage rolled; a player's character's
	// damage is applied by the master.
	hit := func(attacker, key, target string, d20 int, dice ...int) {
		t.Helper()
		a.h.roller.queue(append([]int{d20}, dice...)...)
		res := a.mustAttack(t, master, e, attacker, key, target, inAppRoll)
		if res.GetPendingDamage() == nil {
			t.Fatalf("%s's attack on %s = %v, want a hit", attacker, target, res.GetRoll())
		}
		dmg := a.mustDamage(t, master, e, res.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
		if dmg.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
			if _, err := a.settle(t, master, e, dmg.GetId(), true); err != nil {
				t.Fatalf("ApplyPendingDamage() error = %v", err)
			}
		}
	}
	undoLast := func() {
		t.Helper()
		if err := a.undo(t, master, e, a.log(t, master, e).GetUndoableEventId()); err != nil {
			t.Fatalf("UndoLastAction() error = %v", err)
		}
	}
	next := func() { t.Helper(); a.mustEndTurn(t, master, e) }

	hit("Toren", battleaxe, "Goblin", 20, 8, 8) // a critical: 2d8 (8, 8) + 3 = 19, of which 7 land
	next()
	hit("Pensantus", fireBolt, "Capitão Goblin", 15, 7) // 7 on the hidden Capitão
	next()
	hit("Brisa", rapier, "Capitão Goblin", 15, 5) // 8
	next()
	hit("Capitão Goblin", sword, "Toren", 15, 3) // 5 to Toren, applied by the master
	next()
	if cur := a.get(t, master); cur.GetRound() != 2 || cur.GetCurrentCombatantId() != a.id(t, "Toren") {
		t.Fatalf("round %d, on turn %s; want round 2 and Toren", cur.GetRound(), cur.GetCurrentCombatantId())
	}
	// Retomar o fôlego: 1d10 (8) + 2 = 10, of which the 5 Toren is missing heal.
	a.h.roller.queue(8)
	res, err := a.feature(t, master, e, "Toren", secondWindKey, takeInApp)
	if err != nil || res.GetHealed() != 5 {
		t.Fatalf("Retomar o fôlego = %v, %v; want 5 healed", res.GetHealed(), err)
	}
	// An attack that hits, its damage, and the master takes both back.
	hit("Toren", battleaxe, "Capitão Goblin", 15, 6)
	undoLast() // the damage
	undoLast() // the attack
	if cur, _, _ := a.hp(t, "Capitão Goblin"); cur != 12 {
		t.Fatalf("the Capitão has %d PV after the undo, want 12 (27 - 7 - 8)", cur)
	}
	if _, err := a.adjustHP(t, e, "Capitão Goblin", damageHP(3)); err != nil { // the master's own hand
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	a.endEncounter(t, e)

	want := []string{
		"MOST_DAMAGE 8: Brisa",
		"MOST_HEALING 5: Toren",
		"TANK 5: Toren",
		"FINAL_BLOW 1: Toren",
		"CRITICAL_HITS 1: Toren",
	}
	wantCategories(t, "the master", a.mustHighlights(t, master, e), want...)
	players := a.mustHighlights(t, a.ana, e)
	wantCategories(t, "a player", players, want...)
	if text := asJSON(t, players); strings.Contains(text, "Capitão") || strings.Contains(text, a.capitao.GetId()) {
		t.Errorf("a player was sent the hidden Capitão: %s", text)
	}
	// The master's table shows the numbers behind the categories: the undone hit
	// (9) and the master's 3 are in nobody's.
	var dealt []int32
	for _, c := range a.mustHighlights(t, master, e).GetCharacters() {
		dealt = append(dealt, c.GetDamageDealt())
	}
	if want := []int32{7, 7, 8}; !slices.Equal(dealt, want) { // Toren, Pensantus, Brisa: the order of the combat
		t.Errorf("damage dealt = %v, want %v: the undone 9 and the master's 3 count for nobody", dealt, want)
	}
}

// highlights are worked out from the events alone, so the rules that need
// odd inputs are tested on fabricated ones, with no database.

// fabric builds the events of a combat for tallyHighlights.
type fabric struct {
	rows []playdb.ListEncounterCombatEventsRow
}

func (f *fabric) add(kind string, ev actionEvent) string {
	id := newKey()
	payload, _ := json.Marshal(ev)
	f.rows = append(f.rows, playdb.ListEncounterCombatEventsRow{ID: id, Kind: kind, Payload: payload})
	return id
}

// party is two players and an NPC, as combatants.
func party() []playdb.Combatant {
	return []playdb.Combatant{
		{ID: "c-toren", CharacterID: "ch-toren", Kind: kindPlayer, Label: "Toren"},
		{ID: "c-bri", CharacterID: "ch-bri", Kind: kindPlayer, Label: "Brisa"},
		{ID: "c-gob", CharacterID: "ch-gob", Kind: kindNPC, Label: "Goblin"},
	}
}

// land is a damage roll on an NPC that took `lost` hit points of `before`.
func land(actor string, before, lost int32) actionEvent {
	after := hpState{HP: before - lost, Defeated: before-lost == 0}
	return actionEvent{Actor: actor, Target: "c-gob", Applied: true, Amount: lost + 20, Before: &hpState{HP: before}, After: &after}
}

func tallyOf(f *fabric) map[string]characterTally {
	out := map[string]characterTally{}
	for _, t := range tallyHighlights(f.rows, party()) {
		out[t.name] = *t
	}
	return out
}

// MR-032: damage past 0 hit points does not count, temporary hit points that
// absorbed some do, and the hit that brings an NPC to 0 is the final blow.
func TestMR032_OverkillDoesNotCount(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	f.add(eventDamageRolled, land("c-toren", 7, 7)) // 27 rolled, 7 land: defeated
	e := land("c-bri", 10, 4)
	e.Before.Temp, e.After.Temp = 3, 0 // 3 temporary points and 10: 7 of 13 gone
	e.After.HP = 6
	f.add(eventDamageRolled, e)
	got := tallyOf(f)
	if toren := got["Toren"]; toren.damage != 7 || toren.finalBlows != 1 {
		t.Errorf("Toren = %+v, want 7 damage and a final blow", toren)
	}
	if bri := got["Brisa"]; bri.damage != 7 || bri.finalBlows != 0 {
		t.Errorf("Brisa = %+v, want 7 damage (3 temporary + 4) and no final blow", bri)
	}
}

// MR-032: what an undo took back counts for nothing, even when other events
// came after it.
func TestMR032_UndoneActionsAreSkipped(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	crit := f.add(eventAttackRolled, actionEvent{Actor: "c-toren", Outcome: outcomeCrit})
	dmg := f.add(eventDamageRolled, land("c-toren", 7, 7))
	f.add(eventActionUndone, actionEvent{Undone: dmg, UndoneKind: eventDamageRolled})
	f.add(eventActionUndone, actionEvent{Undone: crit, UndoneKind: eventAttackRolled})
	f.add(eventAttackRolled, actionEvent{Actor: "c-bri", Outcome: outcomeCrit})
	got := tallyOf(f)
	if toren := got["Toren"]; toren != (characterTally{characterID: "ch-toren", name: "Toren"}) {
		t.Errorf("Toren = %+v, want nothing: his hit was taken back", toren)
	}
	if bri := got["Brisa"]; bri.crits != 1 {
		t.Errorf("Brisa = %+v, want her critical hit", bri)
	}
}

// MR-032: damage on a player's character counts when the master applies it, to
// the character that took it; a hit at 0 hit points is no hit points; an NPC's
// own hit and a combatant that left the combat are nobody's.
func TestMR032_DamageTakenIsWhatTheMasterApplied(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	// Rolled and still waiting, discarded or never applied: no damage_applied.
	f.add(eventDamageRolled, actionEvent{Actor: "c-gob", Target: "c-toren", Amount: 9})
	f.add(eventDamageApplied, actionEvent{Actor: "c-gob", Target: "c-toren", Amount: 5, Before: &hpState{HP: 20}, After: &hpState{HP: 15}})
	f.add(eventDamageApplied, actionEvent{Actor: "c-gob", Target: "c-toren", Amount: 4, Before: &hpState{HP: 15, Temp: 2}, After: &hpState{HP: 13}}) // 2 temporary and 2 more
	f.add(eventDamageApplied, actionEvent{Actor: "c-gob", Target: "c-bri", Amount: 6, Before: &hpState{}, After: &hpState{}})                        // at 0
	f.add(eventDamageRolled, land("c-gone", 7, 7))                                                                                                   // its combatant left
	f.add(eventDamageRolled, land("c-gob", 7, 7))                                                                                                    // an NPC acting on an NPC
	got := tallyOf(f)
	if got["Toren"].taken != 9 || got["Brisa"].taken != 0 {
		t.Errorf("taken: Toren %d, Brisa %d; want 9 (5, then 2 temporary and 2) and 0", got["Toren"].taken, got["Brisa"].taken)
	}
	if got["Toren"].damage+got["Brisa"].damage != 0 {
		t.Errorf("damage dealt = %+v, want nobody's", got)
	}
}

// MR-032: a heal counts what it gave back, from a spell or from Retomar o
// fôlego.
func TestMR032_HealingIsWhatWasGivenBack(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	f.add(eventActionTaken, actionEvent{Actor: "c-toren", Heal: true, Amount: 4})
	f.add(eventDamageRolled, actionEvent{Actor: "c-bri", Heal: true, Applied: true, Amount: 6, Settled: []damageHit{
		{Target: "c-toren", Amount: 6, Applied: true}, {Target: "c-bri", Amount: 3, Applied: true},
	}})
	got := tallyOf(f)
	if got["Toren"].healing != 4 || got["Brisa"].healing != 9 {
		t.Errorf("healing: Toren %d, Brisa %d; want 4 and 9", got["Toren"].healing, got["Brisa"].healing)
	}
}

// MR-032: a tie names everyone tied, in the order of the combat, and a
// category where everybody has 0 is left out.
func TestMR032_TiesNameEveryoneAndZerosAreLeftOut(t *testing.T) {
	t.Parallel()
	tallies := []*characterTally{
		{characterID: "a", name: "Toren", damage: 7, taken: 3, crits: 0},
		{characterID: "b", name: "Brisa", damage: 7, taken: 0},
		{characterID: "c", name: "Pensantus", damage: 2, taken: 0},
	}
	var got []string
	for _, c := range highlightCategories(tallies) {
		var who []string
		for _, w := range c.GetWinners() {
			who = append(who, w.GetName())
		}
		got = append(got, c.GetKind().String()+" "+itoa(c.GetValue())+": "+strings.Join(who, ", "))
	}
	want := []string{"HIGHLIGHT_KIND_MOST_DAMAGE 7: Toren, Brisa", "HIGHLIGHT_KIND_TANK 3: Toren"}
	if !slices.Equal(got, want) {
		t.Errorf("categories = %q, want %q (healing, final blows and critical hits are all 0)", got, want)
	}
	if n := len(highlightCategories(nil)); n != 0 {
		t.Errorf("a combat with nobody has %d categories, want none", n)
	}
}

// GetCombatHighlights: any member, once the combat ended; a combat of another
// campaign is not found; the rest as everywhere.
func TestMR032_HighlightsAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.endEncounter(t, e)
	outsider := a.h.newUser("Intruso")
	pending := a.h.newUser("Pendente")
	a.h.joinPending(a.master, a.campaignID, pending)
	call := func(u *user, ctx context.Context, id string) error {
		_, err := u.combat.GetCombatHighlights(ctx, connect.NewRequest(&playv1.GetCombatHighlightsRequest{CampaignId: a.campaignID, EncounterId: id}))
		return err
	}
	methods := playv1.File_meurpg_play_v1_combat_proto.Services().ByName("CombatService").Methods()
	for _, name := range highlightRPCs {
		if methods.ByName(protoreflect.Name(name)) == nil {
			t.Errorf("the service has no %s", name)
		}
	}
	for who, c := range map[string]struct {
		u    *user
		want connect.Code
	}{
		"master": {a.master, allowed}, "player": {a.caio, allowed}, "another player": {a.ana, allowed},
		"outsider": {outsider, connect.CodeNotFound}, "pending member": {pending, connect.CodeNotFound}, "anonymous": {a.h.anonymous(), connect.CodeUnauthenticated},
	} {
		err := call(c.u, t.Context(), e.GetId())
		switch {
		case c.want == allowed && err != nil:
			t.Errorf("%s: %v, want allowed", who, err)
		case c.want != allowed && connect.CodeOf(err) != c.want:
			t.Errorf("%s: %v, want %v", who, err, c.want)
		}
	}
	// An ID that names no combat of this campaign is not found, as for the log.
	for _, id := range []string{newKey(), "não é um ID"} {
		if err := call(a.master, t.Context(), id); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("GetCombatHighlights(%q) = %v, want not_found", id, err)
		}
	}
	// And a combat that is running is refused to everyone with the reason.
	n := a.start(t, plan{npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{5}, players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1}})
	for who, u := range map[string]*user{"master": a.master, "player": a.caio} {
		_, err := a.highlights(t, u, n)
		wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ENDED)
		_ = who
	}
}

// MR-031: the master's NPC card in the combat carries the portrait's URL
// ("/images/<id>"); a player's copy of the same combat never does, nor the
// image's ID (a player may see a portrait only on the stage).
func TestMR031_TheMastersNPCCardHasThePortrait(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	imageID := newKey()
	if _, err := a.h.pool.Exec(t.Context(),
		`INSERT INTO gallery_images (id, campaign_id, name, content_type, width, height, byte_size, created_at) VALUES ($1, $2, 'aldo.png', 'image/png', 40, 30, 100, now())`,
		imageID, a.campaignID); err != nil {
		t.Fatalf("insert the gallery image: %v", err)
	}
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{
		HitPointsMax: 9, ArmorClass: 11, SpeedFt: 30, PortraitImageId: imageID,
	}}}
	res, err := a.master.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_MINION, Name: "Aldo", Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(Aldo with a portrait) error = %v", err)
	}
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: res.Msg.GetCharacter().GetId(), Hidden: new(false)}, {CharacterId: a.goblin.GetId(), Hidden: new(false)}},
		npcRolls: []int{5, 4},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
	})
	if got := byLabel(t, e, "Aldo").GetPortraitUrl(); got != "/images/"+imageID {
		t.Errorf("the master's card for Aldo has the portrait %q, want /images/%s", got, imageID)
	}
	if got := byLabel(t, e, "Goblin").GetPortraitUrl(); got != "" {
		t.Errorf("the card of an NPC with no portrait has %q, want none", got)
	}
	player := a.get(t, a.ana)
	if got := byLabel(t, player, "Aldo").GetPortraitUrl(); got != "" {
		t.Errorf("a player's copy has the portrait %q, want none", got)
	}
	if strings.Contains(asJSON(t, player), imageID) {
		t.Errorf("a player's combat names the portrait's image: %s", asJSON(t, player))
	}
}
