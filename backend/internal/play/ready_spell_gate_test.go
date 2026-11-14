package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// What holds a readied spell back (ready_spell.go, readiedGate): its readier's open concentration save, a hidden
// reveal the master owes; how its concentration save reads (the held spell for the owner and the master, the master's
// wait for everyone else); a spell attack cantrip held with its concentration; a release held for a die.

const blockedConcentrationPending = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CONCENTRATION_SAVE_PENDING

// heldSave has Pensantus ready Hold Person on the Capitão, the Capitão hit him on its turn, and the damage land: his concentration
// save on the held spell is open. It returns the save's window as Ana reads it.
func (a *armed) heldSave(t *testing.T) (*playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	e := a.castersFight(t, 1)
	a.mustReady(t, a.ana, a.readySpell(t, e, "Pensantus", holdPerson, 2, "Capitão Goblin"))
	e = a.passTo(t, e, "Capitão Goblin")
	hit := a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(19))
	if _, err := a.declineReaction(t, a.ana, e, hit.GetPendingDamage().GetId()); err != nil {
		t.Fatalf("DeclineReaction() error = %v", err)
	}
	a.h.roller.queue(1)
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), inAppDamage)
	if _, err := a.settle(t, a.master, e, hit.GetPendingDamage().GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatal("no concentration save for the readied spell")
	}
	return e, w
}

// A readier that owes a concentration save on its held spell cannot be fired until the save is made: the fire is
// CONCENTRATION_SAVE_PENDING and writes nothing. Kept, the spell is fired and released.
func TestAReadiedSpellWaitsForItsConcentrationSave(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e, w := a.heldSave(t)
	before := a.events(t)
	_, err := a.fire(t, a.master, e, byLabel(t, a.get(t, a.master), "Pensantus").GetReadied().GetId())
	wantBlockedBy(t, "the fire while the save waits", err, blockedConcentrationPending)
	if n := a.events(t); n != before || a.readiedOpen(t, "Pensantus") {
		t.Fatalf("the refused fire wrote: events %d (want %d), window open %v", n, before, a.readiedOpen(t, "Pensantus"))
	}
	if _, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(20)); err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	rw := a.mustFire(t, e, "Pensantus")
	a.h.roller.queue(1)
	if res := a.mustAnswer(t, a.ana, e, rw.GetId(), useSpellAnswer); !res.GetResult().GetUsed() {
		t.Errorf("the release after the save = %v", res.GetResult())
	}
	if got := a.readiedStatus(t, "Pensantus"); got != readiedFired {
		t.Errorf("rows after the release = %q, want fired", got)
	}
}

// The same holds for the release itself: a concentration save of the readier that opened while its READIED window is
// open refuses "Usar" and the CastSpell release.
func TestAReadiedReleaseWaitsForItsConcentrationSave(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e, w := a.heldSave(t)
	if _, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(20)); err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	rw := a.mustFire(t, e, "Pensantus")
	// A save that opened meanwhile (no damage lands while the window is open: the table opens it for the test).
	a.execSQL(t, `UPDATE reaction_windows SET status = 'open' WHERE id = '`+w.GetId()+`'`)
	_, err := a.answerReaction(t, a.ana, e, rw.GetId(), useSpellAnswer)
	wantBlockedBy(t, "Usar while the save waits", err, blockedConcentrationPending)
	_, err = a.cast(t, a.ana, e, "Pensantus", holdPerson, nil, a.at(t, "Capitão Goblin"), noCastRoll)
	wantBlockedBy(t, "CastSpell while the save waits", err, blockedConcentrationPending)
	if got := a.readiedStatus(t, "Pensantus"); got != readiedWaiting {
		t.Errorf("rows after the refusals = %q, want waiting", got)
	}
}

// The concentration save on a held spell is the readier's secret (RN-10): Ana and the master read the held spell in
// its prompt and in its line; Caio reads the master's wait, not Pensantus's save, and no line.
func TestTheConcentrationSaveOnAHeldSpellNamesItForTheOwnerAlone(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e, w := a.heldSave(t)
	if got := w.GetConcentrationSave(); got.GetSpellKey() != holdPerson || got.GetSpellNamePt() != "Imobilizar Pessoa" {
		t.Errorf("Ana's prompt names %q (%q), want Hold Person", got.GetSpellKey(), got.GetSpellNamePt())
	}
	mw := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if mw.GetTrigger().GetConcentrationSpellKey() != holdPerson || mw.GetConcentrationSave().GetSpellKey() != holdPerson {
		t.Errorf("the master's window names %q / %q, want Hold Person", mw.GetTrigger().GetConcentrationSpellKey(), mw.GetConcentrationSave().GetSpellKey())
	}
	if got := a.get(t, a.master).GetReactionWait().GetTitlePt(); !strings.Contains(got, "Pensantus") {
		t.Errorf("the master's wait = %q, want Pensantus's save named", got)
	}
	caio := a.get(t, a.caio)
	if got := caio.GetReactionWait().GetTitlePt(); got != "Esperando o mestre" || len(caio.GetReactionWindows()) != 0 {
		t.Errorf("Caio's wait = %q (windows %v), want \"Esperando o mestre\" and no window", got, caio.GetReactionWindows())
	}
	if _, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(20)); err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	for _, u := range []*user{a.ana, a.master} {
		if text := asJSON(t, a.log(t, u, e)); !strings.Contains(text, "manteve a concentração em Imobilizar Pessoa") {
			t.Errorf("%s's log does not name the held spell in the save's line", u.id)
		}
	}
	if text := asJSON(t, a.log(t, a.caio, e)); strings.Contains(text, "concentração") || strings.Contains(text, "Imobilizar") {
		t.Errorf("Caio's log tells of the save on the held spell: %s", text)
	}
}

// Fire Bolt, a spell attack cantrip, is readied as a spell: it holds the concentration (Hold Person ends), and its
// release is its spell attack as a reaction. As an attack it is refused: it would hold no concentration.
func TestASpellAttackCantripIsReadiedAsASpell(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	a.h.roller.queue(1)
	a.mustCast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Capitão Goblin"), noCastRoll)
	e = a.nextTurnOf(t, e, "Pensantus")
	asAttack := a.readySpell(t, e, "Pensantus", "", 0, "Goblin")
	asAttack.Kind, asAttack.ActionKey = playv1.ReadiedKind_READIED_KIND_ATTACK, fireBolt
	_, err := a.ready(t, a.ana, asAttack)
	wantBlockedBy(t, "Fire Bolt as an attack", err, blockedReadyAction)
	if got := a.concentrationAs(t, a.ana, "Pensantus"); got != holdPerson {
		t.Fatalf("after the refusal the concentration is %q, want Hold Person", got)
	}

	a.mustReady(t, a.ana, a.readySpell(t, e, "Pensantus", fireBolt, 0, "Goblin"))
	if got := a.concentrationAs(t, a.ana, "Pensantus"); got != fireBolt || a.heldEffects(t, holdPerson) != 0 {
		t.Fatalf("after the Ready: concentration %q, Hold Person's effects %d; want Fire Bolt held and Hold Person ended", got, a.heldEffects(t, holdPerson))
	}
	if got := a.concentrationAs(t, a.caio, "Pensantus"); got != "" {
		t.Errorf("Caio reads the concentration %q", got)
	}
	e = a.mustEndTurn(t, a.master, e)
	w := a.mustFire(t, e, "Pensantus")
	a.h.roller.queue(19, 5, 5)
	if res := a.mustAnswer(t, a.ana, e, w.GetId(), useSpellAnswer); !res.GetResult().GetUsed() {
		t.Fatalf("the release = %v", res.GetResult())
	}
	pens := byLabel(t, a.get(t, a.ana), "Pensantus")
	if !pens.GetReactionUsed() || pens.GetConcentrationSpell() != "" || a.readiedStatus(t, "Pensantus") != readiedFired {
		t.Errorf("after the release: reaction %v, concentration %q, rows %q", pens.GetReactionUsed(), pens.GetConcentrationSpell(), a.readiedStatus(t, "Pensantus"))
	}
	a.mustUndo(t, e)
	if got := a.concentrationAs(t, a.ana, "Pensantus"); got != fireBolt || a.readiedStatus(t, "Pensantus") != readiedWaiting || !a.readiedOpen(t, "Pensantus") {
		t.Errorf("after the undo of the release: concentration %q, rows %q", got, a.readiedStatus(t, "Pensantus"))
	}
}

// A release whose spell attack waits for Brisa's answer about her Bardic Inspiration die keeps the READIED window from
// being answered or passing by itself (answer_now false for the master); the answer casts it, as the reaction.
func TestAReadiedReleaseHeldForADieKeepsItsWindow(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	e = a.passTo(t, e, "Brisa")
	a.mustReady(t, a.bia, a.readySpell(t, e, "Brisa", guidingBolt, 1, "Capitão Goblin"))
	e = a.mustEndTurn(t, a.master, e)
	a.giveCombatantDie(t, "Brisa", e)
	a.mustFire(t, e, "Brisa")
	typed := func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 4} }
	res, err := a.cast(t, a.bia, e, "Brisa", guidingBolt, nil, a.at(t, "Capitão Goblin"), typed)
	if err != nil || res.GetHeldRollOffer() == nil || res.GetCast() != nil {
		t.Fatalf("the release = %v, %v; want it held for the die", res, err)
	}
	if w := a.windowOf(t, a.master, kindReadied); w == nil || w.GetAnswerNow() {
		t.Errorf("the master's READIED window = %v, want it open and not to be answered now", w)
	}
	if _, err := a.answerReaction(t, a.bia, e, a.windowOf(t, a.master, kindReadied).GetId(), passAnswer); err == nil {
		t.Error("the READIED window was let pass while its release waits for the die")
	}
	out, err := a.answerOutside(t, a.bia, res.GetHeldRollOffer().GetHoldId(), true, false, 8)
	if err != nil || out.GetSpellCast().GetCast() == nil {
		t.Fatalf("the die's answer = %v, %v; want the cast", out, err)
	}
	brisa := byLabel(t, a.get(t, a.bia), "Brisa")
	if !brisa.GetReactionUsed() || a.readiedStatus(t, "Brisa") != readiedFired || a.readiedOpen(t, "Brisa") {
		t.Errorf("after the answer: reaction %v, rows %q", brisa.GetReactionUsed(), a.readiedStatus(t, "Brisa"))
	}
}

// A readied attack whose roll waits for Toren's answer about his Bardic Inspiration die keeps its READIED window too:
// the master's window is not to be answered now, "Deixar passar" is refused, and the answer makes the attack.
func TestAReadiedAttackHeldForADieKeepsItsWindow(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	a.giveCombatantDie(t, "Toren", e)
	w := a.mustFire(t, e, "Toren")
	res, err := a.attack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { d20(10)(r); r.CatchWindowId = w.GetId() })
	if err != nil || res.GetHeldRollOffer() == nil {
		t.Fatalf("the readied attack = %v, %v; want it held for the die", res, err)
	}
	if mw := a.windowOf(t, a.master, kindReadied); mw == nil || mw.GetAnswerNow() {
		t.Errorf("the master's READIED window = %v, want it open and not to be answered now", mw)
	}
	if _, err := a.answerReaction(t, a.caio, e, w.GetId(), passAnswer); err == nil {
		t.Error("the READIED window was let pass while its attack waits for the die")
	}
	if _, err := a.answer(t, a.caio, e, res.GetHeldRollOffer().GetHoldId(), keepTheDie); err != nil {
		t.Fatalf("the die's answer error = %v", err)
	}
	if toren := byLabel(t, a.get(t, a.caio), "Toren"); !toren.GetReactionUsed() || a.readiedStatus(t, "Toren") != readiedFired {
		t.Errorf("after the answer: reaction %v, rows %q", toren.GetReactionUsed(), a.readiedStatus(t, "Toren"))
	}
}

// A question about a hidden creature the master owes holds every action: the release of a readied spell too.
func TestAReadiedReleaseWaitsForAHiddenReveal(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.readyHoldPerson(t)
	w := a.mustFire(t, e, "Pensantus")
	a.execSQL(t, `INSERT INTO hidden_reveals (encounter_id, caster_id, spell_key, combatant_ids, origin_col, origin_row, seq, created_at)
		VALUES ('`+e.GetId()+`', '`+a.id(t, "Toren")+`', 'spell:burning-hands', ARRAY['`+a.id(t, "Goblin")+`'], 1, 1, 1, now())`)
	_, err := a.answerReaction(t, a.ana, e, w.GetId(), useSpellAnswer)
	wantBlockedBy(t, "Usar while the master's question waits", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_HIDDEN_REVEAL_PENDING)
	_, err = a.cast(t, a.ana, e, "Pensantus", holdPerson, nil, a.at(t, "Capitão Goblin"), noCastRoll)
	wantBlockedBy(t, "CastSpell while the master's question waits", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_HIDDEN_REVEAL_PENDING)
	if got := a.readiedStatus(t, "Pensantus"); got != readiedWaiting {
		t.Errorf("rows after the refusals = %q, want waiting", got)
	}
}
