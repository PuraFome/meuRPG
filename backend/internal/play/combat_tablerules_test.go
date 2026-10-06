package play

import (
	"context"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/tablerules"
)

// The table's rules inside the combat (MR-025, RN-24; Etapa 10, slice 10.4b): the
// critical hit ("dados dobrados" or "o máximo mais uma rolagem") and the death
// saves that only the owner and the master see. These tests need the database
// (MEURPG_TEST_DATABASE_URL). The fixtures are the combat tests' (newCasters,
// newSummoners); the target is an Ogro with plenty of hit points and a low armor
// class, so a natural 20 is the only thing that matters.

const (
	doubled = campaignsv1.CriticalRule_CRITICAL_RULE_DOUBLED_DICE
	maxRoll = campaignsv1.CriticalRule_CRITICAL_RULE_MAX_PLUS_ROLL
)

// setRules saves the table's rules as the master, changing what edit changes.
func (a *armed) setRules(t *testing.T, edit func(*campaignsv1.TableRules)) {
	t.Helper()
	got, err := a.master.campaigns.GetTableRules(t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("GetTableRules() error = %v", err)
	}
	rules := got.Msg.GetRules()
	edit(rules)
	if _, err := a.master.campaigns.SetTableRules(t.Context(), connect.NewRequest(&campaignsv1.SetTableRulesRequest{CampaignId: a.campaignID, Rules: rules})); err != nil {
		t.Fatalf("SetTableRules() error = %v", err)
	}
}

func criticalIs(r campaignsv1.CriticalRule) func(*campaignsv1.TableRules) {
	return func(t *campaignsv1.TableRules) { t.Critical = r }
}

func deathSavesAre(v campaignsv1.DeathSaveVisibility) func(*campaignsv1.TableRules) {
	return func(t *campaignsv1.TableRules) { t.DeathSaves = v }
}

var hiddenDeath = deathSavesAre(campaignsv1.DeathSaveVisibility_DEATH_SAVE_VISIBILITY_OWNER_AND_MASTER)

// ogre is the target of the critical hits: 400 hit points, armor class 5.
func (a *armed) ogre(t *testing.T) *playv1.Participant {
	t.Helper()
	return &playv1.Participant{CharacterId: a.master.npc(t, a.campaignID, "Ogro", 400, 5).GetId()}
}

// wantCritical checks a critical hit's pending damage: the rule it was made
// under, the dice that are rolled and the maximum that comes without rolling.
func wantCritical(t *testing.T, what string, p *playv1.PendingDamage, rule playv1.CriticalDamageRule, dice, maxAdded int32) {
	t.Helper()
	if !p.GetCritical() || p.GetCriticalRule() != rule || p.GetDiceCount() != dice || p.GetCriticalMax() != maxAdded {
		t.Errorf("%s: pending damage = critical %v, rule %v, %d dice, %d kept; want a critical hit, %v, %d dice, %d kept",
			what, p.GetCritical(), p.GetCriticalRule(), p.GetDiceCount(), p.GetCriticalMax(), rule, dice, maxAdded)
	}
}

// wantRolled checks the damage a roll made: the amount, the dice faces and the
// modifier (the sheet's bonus plus what the rule kept).
func wantRolled(t *testing.T, what string, p *playv1.PendingDamage, amount int32, faces []int32, modifier int32) {
	t.Helper()
	r := p.GetRoll()
	if p.GetAmount() != amount || r.GetModifier() != modifier || len(r.GetFaces()) != len(faces) {
		t.Errorf("%s: damage = %d, roll %v; want %d with faces %v and modifier %d", what, p.GetAmount(), r, amount, faces, modifier)
		return
	}
	for i, f := range faces {
		if r.GetFaces()[i] != f {
			t.Errorf("%s: faces = %v, want %v", what, r.GetFaces(), faces)
		}
	}
	if r.GetTotal() != amount {
		t.Errorf("%s: roll total = %d, want the damage %d", what, r.GetTotal(), amount)
	}
}

// casterCrits is the combat of the critical tests on the casters' fixture: the
// Ogro and the party, on the map, or without one. Pensantus plays first, then
// Toren, Brisa and the Ogro.
func (a *armed) casterCrits(t *testing.T, ogre *playv1.Participant, theatre bool) *playv1.Encounter {
	t.Helper()
	p := plan{
		npcs: []*playv1.Participant{ogre}, npcRolls: []int{1},
		players: map[string]int32{"Pensantus": 20, "Toren": 15, "Brisa": 10},
		reveal:  []string{"Ogro"},
		at:      map[string][2]int32{"Toren": {3, 3}, "Ogro": {4, 3}, "Pensantus": {10, 3}, "Brisa": {5, 3}},
	}
	if theatre {
		p.theatre, p.at = true, nil
	}
	return a.start(t, p)
}

// critRound is one round of criticals under the rule the table has now: Pensantus's
// Raio de Fogo (a spell attack of the Attack action, 1d10), Toren's machado (1d8+3,
// typed with real dice), Brisa's Raio Guia (a spell that attacks, 4d6) and the
// Ogro's scimitar on Toren (1d6+2, the master's). dice and kept say what the rule
// makes of each: how many dice are rolled and the maximum added. Every roll is
// scripted.
func (a *armed) critRound(t *testing.T, e *playv1.Encounter, rule playv1.CriticalDamageRule) {
	t.Helper()
	doubledRule := rule == playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE
	pick := func(two, one int32) int32 { // the dice rolled: doubled, or once
		if doubledRule {
			return two
		}
		return one
	}
	kept := func(n int32) int32 { // what comes without rolling
		if doubledRule {
			return 0
		}
		return n
	}

	// The hint comes before the roll, for everyone who plays.
	e = a.passTo(t, e, "Pensantus")
	for who, u := range map[string]*user{"master": a.master, "Pensantus's player": a.ana} {
		if got := a.mustOptions(t, u, e, "Pensantus").GetCriticalRule(); got != rule {
			t.Errorf("the turn options' critical_rule for %s = %v, want %v", who, got, rule)
		}
	}

	// A spell attack (the cantrip, 1d10; Pensantus is at the 3rd level), the app's dice.
	hit := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Ogro", d20(20))
	if hit.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_CRITICAL_HIT || hit.GetRoll().GetCriticalRule() != rule {
		t.Errorf("Raio de Fogo's roll = %v, want a critical hit with the rule %v", hit.GetRoll(), rule)
	}
	wantCritical(t, "Raio de Fogo", hit.GetPendingDamage(), rule, pick(2, 1), kept(10))
	if doubledRule {
		a.h.roller.queue(3, 4)
		wantRolled(t, "Raio de Fogo", a.mustDamage(t, a.ana, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage(), 7, []int32{3, 4}, 0)
	} else {
		a.h.roller.queue(3)
		wantRolled(t, "Raio de Fogo", a.mustDamage(t, a.ana, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage(), 13, []int32{3}, 10)
	}
	a.mustEndTurn(t, a.ana, e)

	// A weapon attack (1d8+3), the player's own dice: the typed sum is of the dice
	// that are rolled, never of what the rule keeps.
	hit = a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Ogro", d20(20))
	wantCritical(t, "the machado", hit.GetPendingDamage(), rule, pick(2, 1), kept(8))
	id := hit.GetPendingDamage().GetId()
	top := pick(16, 8)
	for _, sum := range []int32{pick(1, 0), top + 1} {
		if _, err := a.damage(t, a.caio, e, id, typedDamage(sum)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("typed_sum %d for the machado's %d dice = %v, want invalid_argument", sum, pick(2, 1), err)
		}
	}
	if doubledRule {
		wantRolled(t, "the machado", a.mustDamage(t, a.caio, e, id, typedDamage(11)).GetPendingDamage(), 14, nil, 3) // 11 + 3
	} else {
		wantRolled(t, "the machado", a.mustDamage(t, a.caio, e, id, typedDamage(5)).GetPendingDamage(), 16, nil, 11) // 5 + 8 + 3
	}
	a.mustEndTurn(t, a.caio, e)

	// A spell that attacks (4d6, a 1st-circle slot).
	cast := a.mustCast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Ogro"), func(r *playv1.CastSpellRequest) {
		r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 20}
	})
	if len(cast.GetCast().GetPendingDamages()) != 1 {
		t.Fatalf("Raio Guia = %v, want one pending damage", cast.GetCast())
	}
	bolt := cast.GetCast().GetPendingDamages()[0]
	wantCritical(t, "Raio Guia", bolt, rule, pick(8, 4), kept(24))
	if doubledRule {
		a.h.roller.queue(1, 1, 1, 1, 2, 2, 2, 2)
		wantRolled(t, "Raio Guia", a.mustDamage(t, a.bia, e, bolt.GetId(), inAppDamage).GetPendingDamage(), 12, []int32{1, 1, 1, 1, 2, 2, 2, 2}, 0)
	} else {
		a.h.roller.queue(1, 2, 3, 4)
		wantRolled(t, "Raio Guia", a.mustDamage(t, a.bia, e, bolt.GetId(), inAppDamage).GetPendingDamage(), 34, []int32{1, 2, 3, 4}, 24)
	}
	a.mustEndTurn(t, a.bia, e)

	// An NPC's attack, the master's own roll: it waits for the master to apply it
	// to a player's character, and it follows the same rule.
	npc := a.mustAttack(t, a.master, e, "Ogro", sword, "Toren", d20(20))
	wantCritical(t, "the Ogro's scimitar", npc.GetPendingDamage(), rule, pick(2, 1), kept(6))
	if doubledRule {
		a.h.roller.queue(4, 5)
		wantRolled(t, "the Ogro's scimitar", a.mustDamage(t, a.master, e, npc.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage(), 11, []int32{4, 5}, 2)
	} else {
		a.h.roller.queue(4)
		wantRolled(t, "the Ogro's scimitar", a.mustDamage(t, a.master, e, npc.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage(), 12, []int32{4}, 8)
	}
	if applied, err := a.settle(t, a.master, e, npc.GetPendingDamage().GetId(), true); err != nil || applied.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Fatalf("ApplyPendingDamage(the Ogro's critical) = %v, %v; want it applied", applied, err)
	}

	// The log says the same, as each viewer reads it: the rule, what was kept, and
	// the dice only for the master and the one who rolled them.
	for _, line := range a.log(t, a.master, e).GetRounds()[0].GetEntries() {
		if d := line.GetDamage(); d != nil && d.GetRoll() != nil && d.GetCriticalRule() != rule {
			t.Errorf("a log line's critical rule = %v, want %v: %v", d.GetCriticalRule(), rule, line)
		}
	}
	var scimitar *playv1.CombatLogDamage
	for _, line := range a.log(t, a.master, e).GetRounds()[0].GetEntries() {
		if line.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && line.GetActorLabel() == "Ogro" {
			scimitar = line.GetDamage()
		}
	}
	if scimitar == nil || scimitar.GetCriticalRule() != rule || scimitar.GetCriticalMax() != kept(6) || scimitar.GetRoll().GetModifier() != 2+kept(6) {
		t.Errorf("the log's line of the Ogro's scimitar = %v, want the rule %v, %d kept and a modifier of %d", scimitar, rule, kept(6), 2+kept(6))
	}
}

// TestRN24_TheCriticalFollowsTheTablesRule: every critical hit of the combat
// (weapon attacks, spell attacks, spells that attack, an NPC's attacks) makes its
// damage by the table's rule, with the app's dice and with real ones, and the
// rule is read at each roll: a change applies from the next roll on, and a damage
// already opened keeps the rule it was opened under.
func TestRN24_TheCriticalFollowsTheTablesRule(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.casterCrits(t, a.ogre(t), false)

	// The SRD's rule is the default: a table that never saved any gets doubled dice.
	a.critRound(t, e, playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE)

	a.setRules(t, criticalIs(maxRoll))
	a.critRound(t, e, playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_MAX_PLUS_ROLL)

	a.setRules(t, criticalIs(doubled))
	a.critRound(t, e, playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE)

	// A change in the middle of a hit: the damage that was opened under doubled dice
	// is rolled with doubled dice; the next attack follows the new rule.
	e = a.passTo(t, e, "Pensantus")
	opened := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Ogro", d20(20)).GetPendingDamage()
	a.setRules(t, criticalIs(maxRoll))
	a.h.roller.queue(5, 6)
	wantRolled(t, "the damage opened under doubled dice, rolled under the new rule", a.mustDamage(t, a.ana, e, opened.GetId(), inAppDamage).GetPendingDamage(), 11, []int32{5, 6}, 0)
	a.mustEndTurn(t, a.ana, e)
	next := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Ogro", d20(20)).GetPendingDamage()
	wantCritical(t, "the next attack", next, playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_MAX_PLUS_ROLL, 1, 8)

	// A hit that is not a critical one never follows the rule: its dice are rolled as they are.
	a.mustDamage(t, a.caio, e, next.GetId(), typedDamage(1))
	a.mustEndTurn(t, a.caio, e)
	plain := a.mustAttack(t, a.bia, e, "Brisa", maceKey, "Ogro", d20(19))
	if p := plain.GetPendingDamage(); p.GetCritical() || p.GetCriticalMax() != 0 || p.GetCriticalRule() != playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_UNSPECIFIED || p.GetDiceCount() != 1 {
		t.Errorf("a hit that is not a critical one = %v, want one die, no critical rule and nothing kept", p)
	}
}

// TestRN24_ACreaturesCriticalFollowsTheRule: a summoned creature's attack (the
// owner's player rolls it) makes its critical by the table's rule too.
func TestRN24_ACreaturesCriticalFollowsTheRule(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	a.mustCastSummon(t, a.ana, a.pens, animateDead, slotOfLevel(3), 0, []string{"monster:skeleton"})
	ogre := a.ogre(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{ogre}, npcRolls: []int{1},
		players: map[string]int32{"Pensantus": 20, "Sálvia": 15, "Toren": 12, "Esqueleto": 8},
		reveal:  []string{"Ogro"},
		at:      map[string][2]int32{"Toren": {3, 3}, "Ogro": {4, 3}, "Pensantus": {10, 3}, "Sálvia": {6, 5}, "Esqueleto": {5, 3}},
	})
	for _, c := range []struct {
		rule        campaignsv1.CriticalRule
		want        playv1.CriticalDamageRule
		dice, kept  int32
		faces       []int32
		amount, mod int32
	}{
		{doubled, playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE, 2, 0, []int32{1, 2}, 5, 2},
		{maxRoll, playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_MAX_PLUS_ROLL, 1, 6, []int32{4}, 12, 8},
	} {
		a.setRules(t, criticalIs(c.rule))
		e = a.passTo(t, e, "Esqueleto")
		hit := a.mustAttack(t, a.ana, e, "Esqueleto", "monster:skeleton#shortsword", "Ogro", d20(20))
		wantCritical(t, "the skeleton's shortsword", hit.GetPendingDamage(), c.want, c.dice, c.kept)
		a.h.roller.queue(int(c.faces[0]))
		if len(c.faces) > 1 {
			a.h.roller.queue(int(c.faces[1]))
		}
		wantRolled(t, "the skeleton's shortsword", a.mustDamage(t, a.ana, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage(), c.amount, c.faces, c.mod)
		a.mustEndTurn(t, a.ana, e)
	}
}

// TestRN24_TheRulesAreReadInTheCallersTransaction: a change reads the table's rules
// in its own transaction (never through the pool, PR #121), and a read outside one
// reads them from the pool; nothing keeps them longer.
func TestRN24_TheRulesAreReadInTheCallersTransaction(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	probe := &rulesProbe{inner: a.h.svc.defaults}
	a.h.svc.defaults = probe
	e := a.casterCrits(t, a.ogre(t), false)
	a.setRules(t, criticalIs(maxRoll))

	before := probe.counts()
	e = a.passTo(t, e, "Pensantus")
	a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Ogro", d20(20))
	after := probe.counts()
	if after.inTx <= before.inTx {
		t.Errorf("rules read in a transaction: %d before the attack, %d after; want the attack to read them in its own", before.inTx, after.inTx)
	}

	// Each call reads them again: a change between two calls is seen by the second.
	a.setRules(t, criticalIs(doubled))
	mid := probe.counts()
	if _, err := a.endTurn(t, a.master, e, true); err != nil { // Pensantus's open damage is dropped
		t.Fatalf("EndTurn() error = %v", err)
	}
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Ogro", d20(20))
	wantCritical(t, "the attack after the change", hit.GetPendingDamage(), playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE, 2, 0)
	if probe.counts().inTx <= mid.inTx {
		t.Error("the attack after the change did not read the rules again")
	}
	// A player's read of the combat reads them outside a transaction.
	mid = probe.counts()
	a.get(t, a.ana)
	if probe.counts().outside <= mid.outside {
		t.Error("a player's GetEncounter did not read the table's rules")
	}
	// The master's read does not need them.
	mid = probe.counts()
	a.get(t, a.master)
	if probe.counts().outside != mid.outside {
		t.Error("the master's GetEncounter read the table's rules, which it never hides anything for")
	}
}

// rulesProbe counts the reads of the table's rules, in and out of a transaction.
type rulesProbe struct {
	inner CombatDefaults
	mu    sync.Mutex
	n     struct{ inTx, outside int }
}

type probeCounts struct{ inTx, outside int }

func (p *rulesProbe) counts() probeCounts {
	p.mu.Lock()
	defer p.mu.Unlock()
	return probeCounts{p.n.inTx, p.n.outside}
}

func (p *rulesProbe) StoredTableRules(ctx context.Context, tx pgx.Tx, campaignID string) (tablerules.Rules, error) {
	p.mu.Lock()
	if tx != nil {
		p.n.inTx++
	} else {
		p.n.outside++
	}
	p.mu.Unlock()
	return p.inner.StoredTableRules(ctx, tx, campaignID)
}

func (p *rulesProbe) CombatWithoutMap(ctx context.Context, tx pgx.Tx, campaignID string) (bool, error) {
	return p.inner.CombatWithoutMap(ctx, tx, campaignID)
}

// ---- the death saves ----

// drain returns what a stream has now, without waiting: the change that made
// it is published before its call returns.
func drain(w *watcher) []*playv1.WatchGameSessionResponse {
	var out []*playv1.WatchGameSessionResponse
	for {
		select {
		case ev, ok := <-w.events:
			if !ok {
				return out
			}
			if ev.GetHeartbeat() == nil {
				out = append(out, ev)
			}
		default:
			return out
		}
	}
}

// deathLines are a viewer's log lines of death saves.
func deathLines(l *playv1.ListCombatLogResponse) []*playv1.CombatLogEntry {
	var out []*playv1.CombatLogEntry
	for _, r := range l.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_DEATH_SAVE {
				out = append(out, en)
			}
		}
	}
	return out
}

// seesNoDeathSaves checks everything a player gets about Toren's fall when the
// table hides the death saves: the word "Caído" and nothing more.
func (a *armed) seesNoDeathSaves(t *testing.T, who string, u *user, e *playv1.Encounter) {
	t.Helper()
	enc := a.get(t, u)
	c := byLabel(t, enc, "Toren")
	if c.GetState() != playv1.CombatantState_COMBATANT_STATE_DOWN || c.GetDeathSuccesses() != 0 || c.GetDeathFailures() != 0 || c.GetDeathSaveDue() {
		t.Errorf("%s reads Toren as state %v, %d successes, %d failures, due %v; want DOWN (\"Caído\") and nothing more",
			who, c.GetState(), c.GetDeathSuccesses(), c.GetDeathFailures(), c.GetDeathSaveDue())
	}
	if lines := deathLines(a.log(t, u, e)); len(lines) != 0 {
		t.Errorf("%s reads %d death save lines in the log, want none: %v", who, len(lines), lines)
	}
}

// TestRN24_HiddenDeathSavesAreTheOwnersAndTheMasters: with the table's rule on, a
// character's death saves (the counts, every roll, the failures a hit at 0 adds)
// reach only its owner and the master, in the combatant, the log, the answers and
// the stream; everybody else gets "Caído". The result that makes the character
// stable is a result the table sees. With the rule off, nothing changes.
func TestRN24_HiddenDeathSavesAreTheOwnersAndTheMasters(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	a.setRules(t, hiddenDeath)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	streams := map[string]*watcher{
		"master": a.master.watch(t, a.campaignID), "Toren's player": a.caio.watch(t, a.campaignID),
		"Pensantus's player": a.ana.watch(t, a.campaignID), "Brisa's player": a.bia.watch(t, a.campaignID),
	}
	for _, w := range streams {
		w.ready(t)
	}
	a.mustEndTurn(t, a.ana, e) // Toren's turn: the save is due

	// A success (14): the owner and the master read it, the others nothing.
	a.h.roller.queue(14)
	res, err := a.deathSave(t, a.caio, e, "Toren", rollApp)
	if err != nil || res.GetDeathSave().GetSuccesses() != 1 || res.GetDeathSave().GetRoll().GetFaces()[0] != 14 {
		t.Fatalf("Toren's death save = %v, %v; want a success (14) for its player", res.GetDeathSave(), err)
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		c := byLabel(t, a.get(t, u), "Toren")
		lines := deathLines(a.log(t, u, e))
		if c.GetDeathSuccesses() != 1 || c.GetDeathFailures() != 0 || len(lines) != 1 || lines[0].GetDeathSave().GetRoll().GetFaces()[0] != 14 ||
			lines[0].GetDeathSave().GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS {
			t.Errorf("%s reads %d and %d with the log %v, want 1 and 0 with the roll (14) and the outcome", who, c.GetDeathSuccesses(), c.GetDeathFailures(), lines)
		}
	}
	a.seesNoDeathSaves(t, "Pensantus's player", a.ana, e)
	a.seesNoDeathSaves(t, "Brisa's player", a.bia, e)
	for who, w := range streams {
		evs := drain(w)
		text := asJSONAll(t, evs)
		for _, k := range kinds(evs) {
			if (who == "Pensantus's player" || who == "Brisa's player") && k == "other" {
				t.Errorf("%s's stream = %v, want hints only", who, text)
			}
		}
		if (who == "Pensantus's player" || who == "Brisa's player") && (strings.Contains(text, "death") || strings.Contains(text, "roll") || strings.Contains(text, "ailure")) {
			t.Errorf("%s's stream carries death save data: %s", who, text)
		}
	}
	a.mustEndTurn(t, a.caio, e) // Brisa
	a.mustEndTurn(t, a.bia, e)  // the goblin

	// A hit at 0 is a failure: the master reads it when he applies it, Toren's
	// player in the log, everybody else not at all.
	a.h.roller.queue(15, 3)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", inAppRoll)
	id := hit.GetPendingDamage().GetId()
	a.mustDamage(t, a.master, e, id, inAppDamage)
	applied, err := a.settle(t, a.master, e, id, true)
	if err != nil || applied.GetDeathFailuresAdded() != 1 {
		t.Fatalf("the hit at 0 = %v, %v; want one failure added", applied, err)
	}
	for who, want := range map[string]int32{"master": 1, "Toren's player": 1, "Pensantus's player": 0, "Brisa's player": 0} {
		u := map[string]*user{"master": a.master, "Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia}[who]
		var got int32
		for _, r := range a.log(t, u, e).GetRounds() {
			for _, en := range r.GetEntries() {
				got += en.GetDamage().GetDeathFailuresAdded()
			}
		}
		if got != want {
			t.Errorf("the failures the log tells %s = %d, want %d", who, got, want)
		}
	}
	a.mustEndTurn(t, a.master, e) // the Capitão

	// A failure (5), then two successes: stable. The others read the result, not the road.
	for _, face := range []int{5, 12, 12} {
		e = a.passTo(t, e, "Toren")
		a.h.roller.queue(face)
		if _, err := a.deathSave(t, a.caio, e, "Toren", rollApp); err != nil {
			t.Fatalf("RollDeathSave(%d) error = %v", face, err)
		}
		a.mustEndTurn(t, a.caio, e)
		a.seesNoDeathSavesBut(t, "Pensantus's player", a.ana, e, face == 12 && a.successesOf(t, "Toren") >= 3)
	}
	for who, u := range map[string]*user{"master": a.master, "Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if got := combatantState(a.get(t, u), "Toren"); got != playv1.CombatantState_COMBATANT_STATE_STABLE {
			t.Errorf("Toren's state for %s = %v, want STABLE (\"Estável\", a result the table sees)", who, got)
		}
	}
	lines := deathLines(a.log(t, a.ana, e))
	if len(lines) != 1 || !lines[0].GetDeathSave().GetStable() || lines[0].GetDeathSave().GetRoll() != nil ||
		lines[0].GetDeathSave().GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_UNSPECIFIED || lines[0].GetDeathSave().GetSuccesses() != 0 || lines[0].GetDeathSave().GetFailures() != 0 {
		t.Errorf("Pensantus's player reads the death save lines %v, want one that says stable and nothing else", lines)
	}
	if n := len(deathLines(a.log(t, a.caio, e))); n != 4 {
		t.Errorf("Toren's player reads %d death save lines, want his 4", n)
	}

	// The end of the combat and of the session: the highlights and the summary carry no death save to anyone.
	a.endEncounter(t, e)
	highlights := map[string]string{}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia, "Toren's player": a.caio} {
		highlights[who] = asJSON(t, a.mustHighlights(t, u, e))
	}
	open := a.master.liveSession(t, a.campaignID).GetGameSession()
	ended := a.master.end(t, open)
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia, "Toren's player": a.caio} {
		text := highlights[who]
		sum, err := a.summary(t, u, ended.GetId())
		if err != nil {
			t.Fatalf("GetSessionSummary() as %s error = %v", who, err)
		}
		text += asJSON(t, sum)
		if low := strings.ToLower(text); strings.Contains(low, "death") || strings.Contains(low, "stable") || strings.Contains(low, "dying") {
			t.Errorf("the highlights and the summary %s reads mention the death saves: %s", who, text)
		}
	}
}

// successesOf is Toren's death save successes as the master reads them.
func (a *armed) successesOf(t *testing.T, label string) int32 {
	t.Helper()
	return byLabel(t, a.get(t, a.master), label).GetDeathSuccesses()
}

// seesNoDeathSavesBut is seesNoDeathSaves, except that when stable is set the
// character is stable for the viewer too, and the log has its one line.
func (a *armed) seesNoDeathSavesBut(t *testing.T, who string, u *user, e *playv1.Encounter, stable bool) {
	t.Helper()
	if !stable {
		a.seesNoDeathSaves(t, who, u, e)
		return
	}
	c := byLabel(t, a.get(t, u), "Toren")
	if c.GetState() != playv1.CombatantState_COMBATANT_STATE_STABLE || c.GetDeathSuccesses() != 0 || c.GetDeathFailures() != 0 {
		t.Errorf("%s reads Toren as %v with %d and %d; want STABLE and no counts", who, c.GetState(), c.GetDeathSuccesses(), c.GetDeathFailures())
	}
}

// TestRN24_ARevivalAndADeathStayWithTheOwnerAndTheMaster: a natural 20 brings the
// character back with 1 hit point and the others read no line of it (only that
// Toren stands again); a natural 1 and a third failure leave no trace for them
// either, and they still read "Caído", never "Morrendo". The master's confirmation
// of the death is the table's.
func TestRN24_ARevivalAndADeathStayWithTheOwnerAndTheMaster(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	a.setRules(t, hiddenDeath)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e)

	// Two failures with a natural 1, nobody else reads it.
	if _, err := a.deathSave(t, a.caio, e, "Toren", deathFace(1)); err != nil {
		t.Fatalf("RollDeathSave(1) error = %v", err)
	}
	a.seesNoDeathSaves(t, "Pensantus's player", a.ana, e)
	a.mustEndTurn(t, a.caio, e)

	// A natural 20: revived. Toren's player and the master get the vitals; the others
	// read no line and no vitals, and Toren is up for them.
	watch := a.ana.watch(t, a.campaignID)
	watch.ready(t)
	caioStream := a.caio.watch(t, a.campaignID)
	caioStream.ready(t)
	e = a.passTo(t, e, "Toren")
	if _, err := a.deathSave(t, a.caio, e, "Toren", deathFace(20)); err != nil {
		t.Fatalf("RollDeathSave(20) error = %v", err)
	}
	if n := len(deathLines(a.log(t, a.ana, e))); n != 0 {
		t.Errorf("Pensantus's player reads %d death save lines after a revival, want none", n)
	}
	if got := combatantState(a.get(t, a.ana), "Toren"); got == playv1.CombatantState_COMBATANT_STATE_DOWN || got == playv1.CombatantState_COMBATANT_STATE_STABLE {
		t.Errorf("Toren's state for Pensantus's player after the revival = %v, want him standing", got)
	}
	if n := len(deathLines(a.log(t, a.caio, e))); n != 2 {
		t.Errorf("Toren's player reads %d death save lines, want his 2", n)
	}
	var vitalsFor, vitalsAna bool
	for _, ev := range drain(watch) {
		vitalsAna = vitalsAna || ev.GetVitalsChanged() != nil
	}
	for _, ev := range drain(caioStream) {
		vitalsFor = vitalsFor || ev.GetVitalsChanged() != nil
	}
	if vitalsAna || !vitalsFor {
		t.Errorf("vitals_changed for Pensantus's player = %v and for Toren's player = %v after the revival, want false and true", vitalsAna, vitalsFor)
	}
}

// TestRN24_ADeathTheMasterConfirmsIsTheTables: dying is the master's alone and the
// confirmation of a death is announced to everyone, as the result it is.
func TestRN24_ADeathTheMasterConfirmsIsTheTables(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	a.setRules(t, hiddenDeath)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e)
	if _, err := a.deathSave(t, a.caio, e, "Toren", deathFace(1)); err != nil { // two failures
		t.Fatalf("RollDeathSave(1) error = %v", err)
	}
	a.mustEndTurn(t, a.caio, e)
	e = a.passTo(t, e, "Toren")
	if _, err := a.deathSave(t, a.caio, e, "Toren", deathFace(2)); err != nil { // the third
		t.Fatalf("RollDeathSave(2) error = %v", err)
	}
	for who, want := range map[string]playv1.CombatantState{
		"master": playv1.CombatantState_COMBATANT_STATE_DYING, "Toren's player": playv1.CombatantState_COMBATANT_STATE_DOWN,
		"Pensantus's player": playv1.CombatantState_COMBATANT_STATE_DOWN,
	} {
		u := map[string]*user{"master": a.master, "Toren's player": a.caio, "Pensantus's player": a.ana}[who]
		if got := combatantState(a.get(t, u), "Toren"); got != want {
			t.Errorf("Toren's state for %s = %v, want %v", who, got, want)
		}
	}
	a.seesNoDeathSaves(t, "Pensantus's player", a.ana, e)
	a.mustEndTurn(t, a.caio, e)
	e = a.passTo(t, e, "Toren")
	if _, err := a.master.combat.ConfirmDeath(t.Context(), connect.NewRequest(&playv1.ConfirmDeathRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("ConfirmDeath() error = %v", err)
	}
	if got := combatantState(a.get(t, a.ana), "Toren"); got != playv1.CombatantState_COMBATANT_STATE_DEAD {
		t.Errorf("Toren's state for Pensantus's player after the confirmation = %v, want DEAD", got)
	}
	var announced bool
	for _, r := range a.log(t, a.ana, e).GetRounds() {
		for _, en := range r.GetEntries() {
			announced = announced || en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_DEATH_CONFIRMED
		}
	}
	if !announced {
		t.Error("Pensantus's player reads no line for the confirmed death")
	}
}

// TestRN24_VisibleDeathSavesAreUnchanged: with the table's default (everyone sees
// them) the counts and the lines are as they always were, and so is the rule when
// it is changed back.
func TestRN24_VisibleDeathSavesAreUnchanged(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e)
	a.h.roller.queue(14)
	if _, err := a.deathSave(t, a.caio, e, "Toren", rollApp); err != nil {
		t.Fatalf("RollDeathSave() error = %v", err)
	}
	check := func(label string) {
		t.Helper()
		for who, u := range map[string]*user{"master": a.master, "Toren's player": a.caio, "Pensantus's player": a.ana} {
			c := byLabel(t, a.get(t, u), "Toren")
			lines := deathLines(a.log(t, u, e))
			if c.GetDeathSuccesses() != 1 || len(lines) != 1 || lines[0].GetDeathSave().GetSuccesses() != 1 ||
				lines[0].GetDeathSave().GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS {
				t.Errorf("%s: %s reads %d successes and the lines %v, want 1 and one success line", label, who, c.GetDeathSuccesses(), lines)
			}
			// The d20 stays the owner's and the master's, as it always was.
			if len(lines) == 1 {
				if got := lines[0].GetDeathSave().GetRoll() != nil; got != (who != "Pensantus's player") {
					t.Errorf("%s: %s reads the d20 = %v", label, who, got)
				}
			}
		}
	}
	check("the default")
	a.setRules(t, hiddenDeath)
	a.seesNoDeathSaves(t, "Pensantus's player", a.ana, e)
	a.setRules(t, deathSavesAre(campaignsv1.DeathSaveVisibility_DEATH_SAVE_VISIBILITY_VISIBLE_TO_ALL))
	check("the rule set back")
}

// ---- the combat without a grid: no difference by mode ----

// TestRN24_TheTheatreFollowsTheSameRules: a combat without a map applies the
// critical rule and hides the death saves exactly as the one on a map.
func TestRN24_TheTheatreFollowsTheSameRules(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.casterCrits(t, a.ogre(t), true)
	if e.GetMode() != playv1.EncounterMode_ENCOUNTER_MODE_THEATRE {
		t.Fatalf("mode = %v, want THEATRE", e.GetMode())
	}
	a.setRules(t, func(r *campaignsv1.TableRules) {
		r.Critical = maxRoll
		r.DeathSaves = campaignsv1.DeathSaveVisibility_DEATH_SAVE_VISIBILITY_OWNER_AND_MASTER
	})

	e = a.passTo(t, e, "Toren")
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Ogro", d20(20)) // no reach in the theatre: the master judges
	wantCritical(t, "the machado without a map", hit.GetPendingDamage(), playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_MAX_PLUS_ROLL, 1, 8)
	a.h.roller.queue(2)
	wantRolled(t, "the machado without a map", a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage(), 13, []int32{2}, 11)

	a.setRules(t, criticalIs(doubled))
	e = a.passTo(t, e, "Pensantus")
	hit = a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Ogro", d20(20))
	wantCritical(t, "Raio de Fogo without a map", hit.GetPendingDamage(), playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE, 2, 0)

	a.setRules(t, func(r *campaignsv1.TableRules) {
		r.DeathSaves = campaignsv1.DeathSaveVisibility_DEATH_SAVE_VISIBILITY_OWNER_AND_MASTER
	})
	a.correct(t, a.toren, hpIs(0))
	e = a.passTo(t, e, "Toren")
	a.h.roller.queue(14)
	if _, err := a.deathSave(t, a.caio, e, "Toren", rollApp); err != nil {
		t.Fatalf("RollDeathSave() error = %v", err)
	}
	a.seesNoDeathSaves(t, "Pensantus's player", a.ana, e)
	if c := byLabel(t, a.get(t, a.caio), "Toren"); c.GetDeathSuccesses() != 1 {
		t.Errorf("Toren's player reads %d successes without a map, want 1", c.GetDeathSuccesses())
	}
}
