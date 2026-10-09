package play

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Bringing the dead back (RN-03; SRD 5.1, "Dropping to 0 Hit Points" and Revivify). The table:
// Toren (a fighter) and Brisa, the cleric Ilaria (Life domain, level 5: Revivify is always
// prepared) and Pensantus; a goblin. Toren falls in round 1 and the master confirms his death.

const revivify = "spell:revivify"

// ilaria is a cleric of the Life domain at level 5, with Revivify prepared.
func (u *user) ilaria(t *testing.T, campaignID string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 10},
		RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{classLevel("class:cleric", 5, "subclass:life")},
		WeaponKeys: []string{maceKey}, CantripKeys: []string{sacredFlame}, PreparedSpellKeys: []string{cureWounds, revivify},
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Ilaria", Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(Ilaria) error = %v", err)
	}
	return res.Msg.GetCharacter()
}

// newMortuary is the table, with the combat started: Toren (18), Ilaria (12), Brisa (1) and a
// goblin (3 + its bonus). Toren and Ilaria stand next to each other, Brisa far.
func newMortuary(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 4,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.ilaria(t, a.campaignID)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Ilaria": 12, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Ilaria": {3, 4}, "Goblin": {4, 3}, "Brisa": {8, 8}},
	})
	return a, e
}

// dies is Toren falling to 0, failing three death saves and the master confirming it, in the
// combat's current round.
func (a *armed) dies(t *testing.T, e *playv1.Encounter, label string, c *charactersv1.Character) {
	t.Helper()
	a.correct(t, c, hpIs(0))
	a.execSQL(t, `UPDATE combatants SET death_failures = 3 WHERE id = $1`, a.id(t, label))
	if _, err := a.master.combat.ConfirmDeath(t.Context(), connect.NewRequest(&playv1.ConfirmDeathRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("ConfirmDeath(%s) error = %v", label, err)
	}
}

func withDiamonds(r *playv1.CastSpellRequest) { r.MaterialConfirmed = true }

func (a *armed) deadTarget(t *testing.T, label string) []*playv1.SpellTarget {
	t.Helper()
	return []*playv1.SpellTarget{{DeadTargetId: a.id(t, label)}}
}

func (a *armed) preview(t *testing.T, u *user, e *playv1.Encounter, caster string) (*playv1.PreviewRevivifyResponse, error) {
	t.Helper()
	res, err := u.revivify.PreviewRevivify(t.Context(), connect.NewRequest(&playv1.PreviewRevivifyRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CasterId: a.id(t, caster),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func deathEntry(e *playv1.Encounter, _ string, id string) *playv1.CombatDeath {
	for _, d := range e.GetDeaths() {
		if d.GetCombatantId() == id {
			return d
		}
	}
	return nil
}

// RN-03: the master's Reviver puts the combatant back in the order where it was, with no death
// save counted, and it acts on its next turn.
func TestRN03_ReviveKeepsThePlaceInTheOrderAndTheCombatantActsOnItsNextTurn(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	before := labels(e)
	a.dies(t, e, "Toren", a.toren)
	toren := a.id(t, "Toren")

	// The master's list of the dead has the round and the window; the owner's only the round;
	// nobody else gets the list (RN-10).
	master := a.get(t, a.master)
	d := deathEntry(master, "Toren", toren)
	if d == nil || d.GetDeathRound() != 1 || !d.GetFitsRevivify() || d.GetRevivifyBlocked() || !d.GetIsPlayerCharacter() || d.GetCharacterId() != a.toren.GetId() {
		t.Fatalf("the master's deaths = %v, want Toren in round 1 within the minute", master.GetDeaths())
	}
	if own := deathEntry(a.get(t, a.caio), "Toren", toren); own == nil || own.FitsRevivify != nil || own.RevivifyBlocked != nil {
		t.Errorf("the owner's deaths = %v, want Toren with the round only", own)
	}
	for who, u := range map[string]*user{"Ilaria's player": a.ana, "Brisa's player": a.bia} {
		if got := a.get(t, u).GetDeaths(); len(got) != 0 {
			t.Errorf("%s reads the deaths %v, want none", who, got)
		}
	}
	var diedRound *int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT death_round FROM characters WHERE id = $1`, a.toren.GetId()).Scan(&diedRound); err != nil || diedRound == nil || *diedRound != 1 {
		t.Errorf("the character's death_round = %v, %v; want 1", diedRound, err)
	}

	// Brisa's player cannot; the master can.
	_, err := a.bia.characters.ReviveCharacter(t.Context(), connect.NewRequest(&charactersv1.ReviveCharacterRequest{CampaignId: a.campaignID, CharacterId: a.toren.GetId(), IdempotencyKey: newKey()}))
	wantCode(t, "a player's ReviveCharacter", err, connect.CodePermissionDenied)
	res, err := a.master.characters.ReviveCharacter(t.Context(), connect.NewRequest(&charactersv1.ReviveCharacterRequest{CampaignId: a.campaignID, CharacterId: a.toren.GetId(), IdempotencyKey: newKey()}))
	if err != nil {
		t.Fatalf("ReviveCharacter() error = %v", err)
	}
	if res.Msg.GetCharacter().GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED {
		t.Errorf("the revived character's state = %v, want LOCKED as before", res.Msg.GetCharacter().GetState())
	}
	if v := a.vitals(t, a.toren); v.GetHitPointsCurrent() != 1 {
		t.Errorf("Toren's hit points = %d, want 1", v.GetHitPointsCurrent())
	}
	now := a.get(t, a.master)
	c := byLabel(t, now, "Toren")
	if c.GetDefeated() || c.GetState() == playv1.CombatantState_COMBATANT_STATE_DEAD || c.GetDeathFailures() != 0 || c.GetDeathSuccesses() != 0 {
		t.Errorf("Toren after the revival = defeated %v, state %v, failures %d; want alive, with no death save counted", c.GetDefeated(), c.GetState(), c.GetDeathFailures())
	}
	if got := labels(now); strings.Join(got, ",") != strings.Join(before, ",") {
		t.Errorf("the order after the revival = %v, want %v: he keeps his place", got, before)
	}
	if now.GetCurrentCombatantId() == toren {
		t.Errorf("it is Toren's turn right after the revival: he acts on his next turn")
	}
	if len(now.GetDeaths()) != 0 {
		t.Errorf("deaths after the revival = %v, want none", now.GetDeaths())
	}
	var logged bool
	for _, r := range a.log(t, a.ana, now).GetRounds() {
		for _, en := range r.GetEntries() {
			logged = logged || en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CHARACTER_REVIVED && en.GetTargetLabel() == "Toren"
		}
	}
	if !logged {
		t.Errorf("the combat log has no \"O mestre reviveu Toren\" line")
	}
	// It is his turn again when the order comes round: round 2.
	e = a.turnOf(t, "Toren")
	if e.GetRound() != 2 {
		t.Errorf("Toren acts again in round %d, want 2", e.GetRound())
	}
	// Revived mid-fight, the revival is not undone by the master's Desfazer.
	if err := a.undo(t, a.master, e, newKey()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("UndoLastAction after a revival = %v, want nothing to undo", err)
	}
}

// castRevivify casts Revivify with the diamonds ticked.
func (a *armed) castRevivify(t *testing.T, u *user, e *playv1.Encounter, caster, target string) (*playv1.CastSpellResponse, error) {
	t.Helper()
	return a.cast(t, u, e, caster, revivify, slotOfLevel(3), a.deadTarget(t, target), withDiamonds)
}

// SRD 5.1, Revivify: a creature that died in the last minute lives again with 1 hit point, at the
// cost of the 3rd level slot, the action and the diamonds, which the caster confirms.
func TestRevivify_BringsTheDeadBackWithOneHitPointAndSpendsSlotActionAndDiamonds(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)

	// The player's list: Toren, near, in the minute, with no reason attached to anyone.
	pv, err := a.preview(t, a.ana, e, "Ilaria")
	if err != nil {
		t.Fatalf("PreviewRevivify() error = %v", err)
	}
	if len(pv.GetTargets()) != 1 || pv.GetTargets()[0].GetName() != "Toren" || pv.GetTargets()[0].GetTargetId() != a.id(t, "Toren") ||
		pv.GetTargets()[0].GetDeathRound() != 1 || pv.GetTargets()[0].GetRoundsSinceDeath() != 0 || pv.GetTargets()[0].GetNeedsMasterConfirmation() ||
		len(pv.GetUnavailable()) != 0 || pv.GetSlot().GetLevel() != 3 || pv.GetSlotsFree() != 2 || !pv.GetCountsTime() {
		t.Fatalf("PreviewRevivify() = %v, want Toren alone, a 3rd level slot (2 free), time counted", pv)
	}

	// The diamonds are required, and a smaller slot is not a Revivify.
	_, err = a.cast(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.deadTarget(t, "Toren"), noCastRoll)
	wantCode(t, "Revivify without the diamonds", err, connect.CodeInvalidArgument)
	_, err = a.cast(t, a.ana, e, "Ilaria", revivify, slotOfLevel(2), a.deadTarget(t, "Toren"), withDiamonds)
	wantCode(t, "Revivify with a 2nd level slot", err, connect.CodeInvalidArgument)
	_, err = a.cast(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.at(t, "Toren"), withDiamonds)
	wantCode(t, "Revivify aimed at a combatant, not a dead_target_id", err, connect.CodeInvalidArgument)
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 {
		t.Fatalf("slots used after the refusals = %d, want 0", usedSlots(v, 3))
	}

	key := newKey()
	res, err := a.castKey(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.deadTarget(t, "Toren"), withDiamonds, key)
	if err != nil {
		t.Fatalf("CastSpell(Revivify) error = %v", err)
	}
	r := res.GetCast().GetTargets()[0]
	if res.GetCast().GetEffectKind() != playv1.SpellEffectKind_SPELL_EFFECT_KIND_REVIVE || r.GetCombatantId() != a.id(t, "Toren") ||
		r.GetEffect().GetOutcome() != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_AFFECTED || r.GetEffect().GetRevivedCharacterId() != a.toren.GetId() ||
		r.GetEffect().HitPointsAfter != nil {
		t.Errorf("the cast = %v, want REVIVE affecting Toren; the caster's player is not told his hit points", res.GetCast())
	}
	if v := a.vitals(t, a.toren); v.GetHitPointsCurrent() != 1 {
		t.Errorf("Toren's hit points = %d, want 1", v.GetHitPointsCurrent())
	}
	if got := a.caio.character(t, a.toren).GetState(); got != charactersv1.CharacterState_CHARACTER_STATE_LOCKED {
		t.Errorf("Toren's state = %v, want LOCKED: alive as before", got)
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 1 {
		t.Errorf("3rd level slots used = %d, want 1", usedSlots(v, 3))
	}
	now := a.get(t, a.master)
	if c := byLabel(t, now, "Toren"); c.GetDefeated() || c.GetDeathFailures() != 0 {
		t.Errorf("Toren after the spell = defeated %v, failures %d; want alive and clean", c.GetDefeated(), c.GetDeathFailures())
	}
	if !byLabel(t, now, "Ilaria").GetActionUsed() {
		t.Errorf("Ilaria's action is free after casting Revivify (1 action)")
	}
	if len(now.GetDeaths()) != 0 {
		t.Errorf("deaths after the spell = %v, want none", now.GetDeaths())
	}
	// The master reads the diamonds and when he died; the caster, the diamonds; the others, neither.
	logOf := func(u *user) *playv1.CombatLogSpell {
		for _, r := range a.log(t, u, now).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST && en.GetKey() == revivify {
					return en.GetSpell()
				}
			}
		}
		return nil
	}
	if s := logOf(a.master); s == nil || !s.GetMaterialSpent() || s.GetRevivedDeathRound() != 1 || s.GetTargets()[0].GetEffect().HitPointsAfter == nil {
		t.Errorf("the master's log = %v, want the diamonds, round 1 and the hit points", s)
	}
	if s := logOf(a.ana); s == nil || !s.GetMaterialSpent() || s.RevivedDeathRound != nil {
		t.Errorf("the caster's log = %v, want the diamonds and no round", s)
	}
	if s := logOf(a.bia); s == nil || s.GetMaterialSpent() || s.RevivedDeathRound != nil || s.GetTargets()[0].GetEffect().GetRevivedCharacterId() == "" {
		t.Errorf("another player's log = %v, want only who lived again", s)
	}
	// A retry with the key spends nothing more; the master cannot undo it.
	if _, err := a.castKey(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.deadTarget(t, "Toren"), withDiamonds, key); err != nil {
		t.Errorf("the retry of Revivify error = %v", err)
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 1 {
		t.Errorf("3rd level slots used after the retry = %d, want 1", usedSlots(v, 3))
	}
	if err := a.undo(t, a.master, now, newKey()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("UndoLastAction after Revivify = %v, want nothing to undo", err)
	}
}

// SRD 5.1, "The Order of Combat": a minute is 10 rounds. 9 and 10 rounds after the death fit
// (in the tenth, up to the place in the order where it happened), 11 do not.
func TestRevivify_TheMinuteIsTenRounds(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren) // round 1, Toren's place in the order: first (0). Ilaria is second (1).
	setRound := func(r int) { a.execSQL(t, `UPDATE encounters SET round = $1 WHERE id = $2`, r, e.GetId()) }
	listed := func(u *user) bool {
		pv, err := a.preview(t, u, e, "Ilaria")
		if err != nil {
			t.Fatalf("PreviewRevivify() error = %v", err)
		}
		return len(pv.GetTargets()) == 1
	}
	reasonOf := func() playv1.RevivifyUnavailableReason {
		pv, err := a.preview(t, a.master, e, "Ilaria")
		if err != nil || len(pv.GetUnavailable()) != 1 {
			t.Fatalf("the master's PreviewRevivify() = %v, %v; want one creature that does not fit", pv, err)
		}
		return pv.GetUnavailable()[0].GetReason()
	}
	setRound(10) // 9 rounds after
	if !listed(a.ana) {
		t.Errorf("9 rounds after the death: Toren is not listed")
	}
	setRound(11) // 10 rounds after: only up to the place in the order where he died; Ilaria is after it
	if listed(a.ana) || listed(a.master) {
		t.Errorf("10 rounds after the death, past his place in the order: Toren is listed")
	}
	if got := reasonOf(); got != playv1.RevivifyUnavailableReason_REVIVIFY_UNAVAILABLE_REASON_TOO_LONG_AGO {
		t.Errorf("the master's reason = %v, want TOO_LONG_AGO", got)
	}
	a.execSQL(t, `UPDATE combatants SET death_order_index = 1 WHERE id = $1`, a.id(t, "Toren")) // he died at Ilaria's place
	if !listed(a.ana) {
		t.Errorf("10 rounds after the death, up to his place in the order: Toren is not listed")
	}
	setRound(12) // 11 rounds after
	if listed(a.ana) {
		t.Errorf("11 rounds after the death: Toren is listed")
	}
	_, err := a.castRevivify(t, a.ana, e, "Ilaria", "Toren")
	wantCode(t, "Revivify 11 rounds after", err, connect.CodeFailedPrecondition)
	setRound(11)
	if _, err := a.castRevivify(t, a.ana, e, "Ilaria", "Toren"); err != nil {
		t.Errorf("Revivify 10 rounds after, in time, error = %v", err)
	}
}

// RN-10: a player's list holds only whom the spell reaches now, and nothing says why another is
// missing. The same answer comes for a far creature, a hidden one, one the master marked, one that
// is not dead and one that does not exist; the master reads the reasons.
func TestRN10_APlayerNeverLearnsWhyACreatureCannotBeRevived(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(99)); err != nil { // a goblin dead too, at (4,3)
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	move := func(label string, col, row int32) {
		t.Helper()
		if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Col: col, Row: row,
		})); err != nil {
			t.Fatalf("MoveCombatant(%s) error = %v", label, err)
		}
	}
	reasons := func() map[string]playv1.RevivifyUnavailableReason {
		pv, err := a.preview(t, a.master, e, "Ilaria")
		if err != nil {
			t.Fatalf("PreviewRevivify() error = %v", err)
		}
		out := map[string]playv1.RevivifyUnavailableReason{}
		for _, u := range pv.GetUnavailable() {
			out[u.GetName()] = u.GetReason()
		}
		return out
	}
	names := func(u *user) []string {
		pv, err := a.preview(t, u, e, "Ilaria")
		if err != nil {
			t.Fatalf("PreviewRevivify() error = %v", err)
		}
		if len(pv.GetUnavailable()) != 0 {
			t.Errorf("a player's PreviewRevivify carries reasons: %v", pv.GetUnavailable())
		}
		var out []string
		for _, tg := range pv.GetTargets() {
			out = append(out, tg.GetName())
		}
		return out
	}
	const marked = playv1.RevivifyUnavailableReason_REVIVIFY_UNAVAILABLE_REASON_MASTER_BLOCKED

	// Both are within reach: the player lists both, the master has no reason to read.
	if got := names(a.ana); len(got) != 2 {
		t.Fatalf("the player's list = %v, want Toren and the Goblin", got)
	}
	if got := reasons(); len(got) != 0 {
		t.Fatalf("the master's reasons = %v, want none", got)
	}

	// The goblin is hidden by the master: gone from the player's list, with the reason for him.
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	if got := names(a.ana); len(got) != 1 || got[0] != "Toren" {
		t.Errorf("the player's list with a hidden goblin = %v, want Toren", got)
	}
	if got := reasons()["Goblin"]; got != playv1.RevivifyUnavailableReason_REVIVIFY_UNAVAILABLE_REASON_HIDDEN {
		t.Errorf("the master's reason for the hidden goblin = %v, want HIDDEN", got)
	}
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: false,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}

	// Toren is marked "Revivificar não funciona": gone from the list, and nothing tells why.
	setBlocked := func(label string, blocked bool) {
		t.Helper()
		if _, err := a.master.revivify.SetRevivifyBlocked(t.Context(), connect.NewRequest(&playv1.SetRevivifyBlockedRequest{
			CampaignId: a.campaignID, Target: &playv1.SetRevivifyBlockedRequest_CombatantId{CombatantId: a.id(t, label)}, Blocked: blocked,
		})); err != nil {
			t.Fatalf("SetRevivifyBlocked(%s) error = %v", label, err)
		}
	}
	_, err := a.ana.revivify.SetRevivifyBlocked(t.Context(), connect.NewRequest(&playv1.SetRevivifyBlockedRequest{
		CampaignId: a.campaignID, Target: &playv1.SetRevivifyBlockedRequest_CombatantId{CombatantId: a.id(t, "Toren")}, Blocked: true,
	}))
	wantCode(t, "a player's SetRevivifyBlocked", err, connect.CodePermissionDenied)
	setBlocked("Toren", true)
	setBlocked("Toren", true) // the same value again changes nothing
	if got := names(a.ana); len(got) != 1 || got[0] != "Goblin" {
		t.Errorf("the player's list with Toren marked = %v, want the Goblin", got)
	}
	if got := reasons()["Toren"]; got != marked {
		t.Errorf("the master's reason for the marked Toren = %v, want MASTER_BLOCKED", got)
	}
	if d := deathEntry(a.get(t, a.master), "Toren", a.id(t, "Toren")); d == nil || !d.GetRevivifyBlocked() {
		t.Errorf("the master's deaths entry = %v, want the switch on", d)
	}
	if own := deathEntry(a.get(t, a.caio), "Toren", a.id(t, "Toren")); own == nil || own.RevivifyBlocked != nil {
		t.Errorf("Toren's player reads %v: the switch is the master's", own)
	}
	// The Reviver of the master is not stopped by the switch.
	if got := a.master.character(t, a.toren); !got.GetCanRevive() {
		t.Errorf("the master cannot revive a creature marked against Revivify")
	}
	setBlocked("Toren", false)

	// The same answer, word for word, for everything the spell cannot touch.
	move("Ilaria", 9, 9) // far from both
	if got := names(a.ana); len(got) != 0 {
		t.Errorf("the player's list far from the dead = %v, want none", got)
	}
	if got := reasons(); got["Toren"] != playv1.RevivifyUnavailableReason_REVIVIFY_UNAVAILABLE_REASON_TOO_FAR || got["Goblin"] != playv1.RevivifyUnavailableReason_REVIVIFY_UNAVAILABLE_REASON_TOO_FAR {
		t.Errorf("the master's reasons far from the dead = %v, want TOO_FAR for both", got)
	}
	e = a.passTo(t, e, "Ilaria")
	answer := func(target []*playv1.SpellTarget) string {
		t.Helper()
		_, err := a.cast(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), target, withDiamonds)
		wantCode(t, "Revivify on a target it cannot touch", err, connect.CodeFailedPrecondition)
		return err.Error()
	}
	far := answer(a.deadTarget(t, "Toren"))
	living := answer(a.deadTarget(t, "Brisa"))
	nobody := answer([]*playv1.SpellTarget{{DeadTargetId: newKey()}})
	notAnID := answer([]*playv1.SpellTarget{{DeadTargetId: "toren"}})
	move("Ilaria", 3, 4)
	setBlocked("Toren", true)
	blockedAnswer := answer(a.deadTarget(t, "Toren"))
	for name, got := range map[string]string{"a living creature": living, "nobody": nobody, "not an id": notAnID, "a marked death": blockedAnswer} {
		if got != far {
			t.Errorf("the answer for %s = %q, but for a far creature %q: a refusal must not tell them apart", name, got, far)
		}
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 || byLabel(t, a.get(t, a.master), "Ilaria").GetActionUsed() {
		t.Errorf("a refused Revivify spent something: slots %d", usedSlots(v, 3))
	}
}

// Dead is what the master confirmed (RN-03): a character at 0 hit points is "Caído" and no target.
func TestRevivify_ACharacterAtZeroIsNotDeadUntilTheMasterConfirms(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.correct(t, a.toren, hpIs(0))
	pv, err := a.preview(t, a.ana, e, "Ilaria")
	if err != nil || len(pv.GetTargets()) != 0 {
		t.Fatalf("PreviewRevivify() with Toren down = %v, %v; want no target", pv, err)
	}
	_, err = a.castRevivify(t, a.ana, e, "Ilaria", "Toren")
	wantCode(t, "Revivify on a fallen, not dead, character", err, connect.CodeFailedPrecondition)
	_, err = a.castRevivify(t, a.ana, e, "Ilaria", "Ilaria")
	wantCode(t, "Revivify on the caster", err, connect.CodeFailedPrecondition)
	_, err = a.castRevivify(t, a.caio, e, "Ilaria", "Toren")
	wantCode(t, "Revivify by another player", err, connect.CodePermissionDenied)
}

// An NPC taken to 0 is dead for the spell: it lives again with 1 hit point and acts on its turn.
func TestRevivify_AnNPCLivesAgainWithOneHitPoint(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(99)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	pv, err := a.preview(t, a.ana, e, "Ilaria")
	if err != nil || len(pv.GetTargets()) != 1 || pv.GetTargets()[0].GetName() != "Goblin" {
		t.Fatalf("PreviewRevivify() = %v, %v; want the goblin", pv, err)
	}
	e = a.passTo(t, e, "Ilaria")
	res, err := a.castRevivify(t, a.ana, e, "Ilaria", "Goblin")
	if err != nil {
		t.Fatalf("CastSpell(Revivify) on the goblin error = %v", err)
	}
	if r := res.GetCast().GetTargets()[0].GetEffect(); r.GetRevivedCharacterId() != "" || r.HitPointsAfter != nil || r.GetOutcome() != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_AFFECTED {
		t.Errorf("the effect on an NPC = %v, want affected, and neither a character nor its hit points for the caster", r)
	}
	cur, _, defeated := a.hp(t, "Goblin")
	if cur != 1 || defeated {
		t.Errorf("the goblin = %d hit points, defeated %v; want 1 and alive", cur, defeated)
	}
	if a.get(t, a.master).GetCurrentCombatantId() == a.id(t, "Goblin") {
		t.Errorf("the goblin acts in the turn it was revived in")
	}
}

// RN-03: the spell cannot bring a character back while its player has another living one. The
// caster's player is told nothing of why; the master is; nothing is spent.
func TestRN03_RevivifyIsRefusedWhileThePlayerHasAnotherLivingCharacter(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)
	a.caio.hero(t, a.campaignID, "Nuvem", "class:fighter", "race:human", 1,
		&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)

	_, err := a.castRevivify(t, a.ana, e, "Ilaria", "Toren")
	wantCode(t, "Revivify with another living character", err, connect.CodeFailedPrecondition)
	if strings.Contains(err.Error(), "Nuvem") || strings.Contains(err.Error(), "living") {
		t.Errorf("the player's refusal says why: %v", err)
	}
	_, err = a.castRevivify(t, a.master, e, "Ilaria", "Toren")
	wantCode(t, "the master's Revivify with another living character", err, connect.CodeFailedPrecondition)
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 {
		t.Errorf("3rd level slots used after the refusals = %d, want 0", usedSlots(v, 3))
	}
	if got := a.master.character(t, a.toren).GetState(); got != charactersv1.CharacterState_CHARACTER_STATE_DEAD {
		t.Errorf("Toren's state = %v, want still DEAD", got)
	}
}

// newVigil is the table without a combat: the session is open, Toren is dead and Ilaria can
// cast Revivify.
func newVigil(t *testing.T) *armed {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 4,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.ilaria(t, a.campaignID)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	if _, err := a.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: a.campaignID, CharacterId: a.toren.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	return a
}

func (a *armed) ask(u *user, caster, target *charactersv1.Character, key string, edit ...func(*playv1.RequestRevivifyRequest)) (*playv1.RevivifyRequest, error) {
	req := &playv1.RequestRevivifyRequest{
		CampaignId: a.campaignID, CasterCharacterId: caster.GetId(), TargetCharacterId: target.GetId(), Slot: slotOfLevel(3), MaterialConfirmed: true, IdempotencyKey: key,
	}
	for _, e := range edit {
		e(req)
	}
	res, err := u.revivify.RequestRevivify(context.Background(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetRequest(), nil
}

func (a *armed) answerRevivify(t *testing.T, u *user, id string, within bool, key string) (*playv1.RevivifyRequest, error) {
	t.Helper()
	res, err := u.revivify.ConfirmRevivifyTime(t.Context(), connect.NewRequest(&playv1.ConfirmRevivifyTimeRequest{CampaignId: a.campaignID, PendingId: id, WithinMinute: within, IdempotencyKey: key}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetRequest(), nil
}

func (a *armed) requests(t *testing.T, u *user) []*playv1.RevivifyRequest {
	t.Helper()
	res, err := u.revivify.ListRevivifyRequests(t.Context(), connect.NewRequest(&playv1.ListRevivifyRequestsRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("ListRevivifyRequests() error = %v", err)
	}
	return res.Msg.GetRequests()
}

// SRD 5.1, Revivify: outside a combat the app does not count the minute; the master says whether
// the creature died less than a minute ago, and nothing is spent until he does.
func TestRevivify_OutsideACombatTheMasterConfirmsTheMinute(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	pv, err := a.revivify2(t, a.ana, a.pens)
	if err != nil {
		t.Fatalf("PreviewRevivify() error = %v", err)
	}
	if pv.GetCountsTime() || len(pv.GetTargets()) != 1 || pv.GetTargets()[0].GetTargetId() != a.toren.GetId() || !pv.GetTargets()[0].GetNeedsMasterConfirmation() ||
		pv.GetTargets()[0].DeathRound != nil || pv.GetSlot().GetLevel() != 3 {
		t.Fatalf("PreviewRevivify() outside a combat = %v, want Toren, waiting for the master, no time counted", pv)
	}
	_, err = a.revivify2(t, a.caio, a.pens)
	wantCode(t, "another player's PreviewRevivify", err, connect.CodePermissionDenied)

	_, err = a.ask(a.ana, a.pens, a.toren, newKey(), func(r *playv1.RequestRevivifyRequest) { r.MaterialConfirmed = false })
	wantCode(t, "RequestRevivify without the diamonds", err, connect.CodeInvalidArgument)
	_, err = a.ask(a.caio, a.pens, a.toren, newKey())
	wantCode(t, "RequestRevivify by another player", err, connect.CodePermissionDenied)
	_, err = a.ask(a.ana, a.pens, a.bri, newKey())
	wantCode(t, "RequestRevivify on a living character", err, connect.CodeNotFound)

	key := newKey()
	req, err := a.ask(a.ana, a.pens, a.toren, key)
	if err != nil {
		t.Fatalf("RequestRevivify() error = %v", err)
	}
	if req.GetStatus() != playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_PENDING || req.GetCasterName() != "Ilaria" || req.GetTargetName() != "Toren" {
		t.Fatalf("RequestRevivify() = %v, want pending, Ilaria on Toren", req)
	}
	if again, err := a.ask(a.ana, a.pens, a.toren, key); err != nil || again.GetId() != req.GetId() {
		t.Errorf("the retry of RequestRevivify = %v, %v; want the first request", again, err)
	}
	_, err = a.ask(a.ana, a.pens, a.bri, key)
	wantCode(t, "a key reused for another request", err, connect.CodeInvalidArgument)
	if got := len(a.requests(t, a.master)); got != 1 {
		t.Errorf("the master reads %d requests, want 1", got)
	}
	if got := len(a.requests(t, a.ana)); got != 1 {
		t.Errorf("the caster's player reads %d requests, want 1", got)
	}
	for who, u := range map[string]*user{"Toren's player": a.caio, "Brisa's player": a.bia} {
		if got := a.requests(t, u); len(got) != 0 {
			t.Errorf("%s reads the casts %v, want none", who, got)
		}
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 {
		t.Errorf("slots used while waiting = %d, want 0: nothing is spent until the master answers", usedSlots(v, 3))
	}

	// Only the master answers; a player is told the request is not there, for any id.
	for _, id := range []string{req.GetId(), newKey()} {
		_, err = a.answerRevivify(t, a.ana, id, true, newKey())
		wantCode(t, "a player's ConfirmRevivifyTime", err, connect.CodeNotFound)
	}
	// "Já passou": nothing is spent and Toren stays dead.
	k := newKey()
	denied, err := a.answerRevivify(t, a.master, req.GetId(), false, k)
	if err != nil || denied.GetStatus() != playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_DENIED || denied.GetAnsweredAt() == nil {
		t.Fatalf("ConfirmRevivifyTime(false) = %v, %v; want denied", denied, err)
	}
	if again, err := a.answerRevivify(t, a.master, req.GetId(), false, k); err != nil || again.GetStatus() != denied.GetStatus() {
		t.Errorf("the retry of the answer = %v, %v; want the same", again, err)
	}
	_, err = a.answerRevivify(t, a.master, req.GetId(), true, newKey())
	wantCode(t, "answering twice", err, connect.CodeFailedPrecondition)
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 {
		t.Errorf("slots used after \"Já passou\" = %d, want 0", usedSlots(v, 3))
	}
	if got := a.master.character(t, a.toren).GetState(); got != charactersv1.CharacterState_CHARACTER_STATE_DEAD {
		t.Errorf("Toren after \"Já passou\" = %v, want DEAD", got)
	}

	// "Faz menos de 1 minuto": the slot is spent and Toren lives with 1 hit point.
	second, err := a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("second RequestRevivify() error = %v", err)
	}
	done, err := a.answerRevivify(t, a.master, second.GetId(), true, newKey())
	if err != nil || done.GetStatus() != playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_CONFIRMED || done.GetSlotsLeft() != 1 {
		t.Fatalf("ConfirmRevivifyTime(true) = %v, %v; want confirmed, 1 slot left", done, err)
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 1 {
		t.Errorf("3rd level slots used = %d, want 1", usedSlots(v, 3))
	}
	if v := a.vitals(t, a.toren); v.GetHitPointsCurrent() != 1 {
		t.Errorf("Toren's hit points = %d, want 1", v.GetHitPointsCurrent())
	}
	if got := a.caio.character(t, a.toren).GetState(); got != charactersv1.CharacterState_CHARACTER_STATE_LOCKED {
		t.Errorf("Toren's state = %v, want LOCKED as before", got)
	}
	if got := a.requests(t, a.ana); len(got) != 2 || got[0].GetStatus() != playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_CONFIRMED {
		t.Errorf("the caster's requests = %v, want the confirmed one first", got)
	}
}

func (a *armed) revivify2(t *testing.T, u *user, caster *charactersv1.Character) (*playv1.PreviewRevivifyResponse, error) {
	t.Helper()
	res, err := u.revivify.PreviewRevivify(t.Context(), connect.NewRequest(&playv1.PreviewRevivifyRequest{CampaignId: a.campaignID, CasterCharacterId: caster.GetId()}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// RN-10: outside a combat the master's switch keeps the character off the player's list and out of
// reach of the cast, with the same answer as a character that is not there; RN-03 stops the
// master's "less than a minute" too, and the request keeps waiting.
func TestRevivify_OutsideACombatTheSwitchAndRN03(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	if _, err := a.master.revivify.SetRevivifyBlocked(t.Context(), connect.NewRequest(&playv1.SetRevivifyBlockedRequest{
		CampaignId: a.campaignID, Target: &playv1.SetRevivifyBlockedRequest_CharacterId{CharacterId: a.toren.GetId()}, Blocked: true,
	})); err != nil {
		t.Fatalf("SetRevivifyBlocked() error = %v", err)
	}
	pv, err := a.revivify2(t, a.ana, a.pens)
	if err != nil || len(pv.GetTargets()) != 0 || len(pv.GetUnavailable()) != 0 {
		t.Fatalf("the player's PreviewRevivify() = %v, %v; want no target and no reason", pv, err)
	}
	mv, err := a.revivify2(t, a.master, a.pens)
	if err != nil || len(mv.GetUnavailable()) != 1 || mv.GetUnavailable()[0].GetReason() != playv1.RevivifyUnavailableReason_REVIVIFY_UNAVAILABLE_REASON_MASTER_BLOCKED {
		t.Fatalf("the master's PreviewRevivify() = %v, %v; want MASTER_BLOCKED", mv, err)
	}
	_, errBlocked := a.ask(a.ana, a.pens, a.toren, newKey())
	_, errLiving := a.ask(a.ana, a.pens, a.bri, newKey())
	if connect.CodeOf(errBlocked) != connect.CodeNotFound || errBlocked.Error() != errLiving.Error() {
		t.Errorf("a marked death answers %v, a living character %v; want the same not_found", errBlocked, errLiving)
	}
	if _, err := a.master.revivify.SetRevivifyBlocked(t.Context(), connect.NewRequest(&playv1.SetRevivifyBlockedRequest{
		CampaignId: a.campaignID, Target: &playv1.SetRevivifyBlockedRequest_CharacterId{CharacterId: a.bri.GetId()}, Blocked: true,
	})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("SetRevivifyBlocked on a living character error = %v, want failed_precondition", err)
	}
	if _, err := a.master.revivify.SetRevivifyBlocked(t.Context(), connect.NewRequest(&playv1.SetRevivifyBlockedRequest{
		CampaignId: a.campaignID, Target: &playv1.SetRevivifyBlockedRequest_CharacterId{CharacterId: a.toren.GetId()}, Blocked: false,
	})); err != nil {
		t.Fatalf("SetRevivifyBlocked(false) error = %v", err)
	}

	// RN-03: Toren's player made another character; the master's "less than a minute" is refused
	// and the request still waits, with nothing spent.
	req, err := a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("RequestRevivify() error = %v", err)
	}
	a.caio.hero(t, a.campaignID, "Nuvem", "class:fighter", "race:human", 1,
		&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
	_, err = a.answerRevivify(t, a.master, req.GetId(), true, newKey())
	wantCode(t, "ConfirmRevivifyTime with another living character", err, connect.CodeFailedPrecondition)
	if got := a.requests(t, a.master); len(got) != 1 || got[0].GetStatus() != playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_PENDING {
		t.Errorf("the request after the refusal = %v, want it still pending", got)
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 {
		t.Errorf("slots used after the refusal = %d, want 0", usedSlots(v, 3))
	}
}

// PM-04: a Counterspell answers a Revivify like any cast. The cast waits for the window and
// nothing happens meanwhile: Toren is still dead and no slot is spent. A pass lets it through.
func TestRevivify_AnEnemyWizardsCounterspellCountersItOrLetsItThrough(t *testing.T) {
	t.Parallel()
	for _, counter := range []bool{true, false} {
		name := "passed"
		if counter {
			name = "countered"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newArmedWith(t, func(a *armed) {
				a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 4,
					&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
				a.pens = a.ana.ilaria(t, a.campaignID)
				a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
					&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
			})
			wiz := a.master.bossCaster(t, a.campaignID, "Mago Sombrio", 7, []string{fireBolt}, []string{counterspellSpell}, []string{counterspellSpell})
			e := a.start(t, plan{
				npcs:     []*playv1.Participant{{CharacterId: wiz}},
				npcRolls: []int{3},
				players:  map[string]int32{"Toren": 18, "Ilaria": 12, "Brisa": 1},
				reveal:   []string{"Mago Sombrio"},
				at:       map[string][2]int32{"Toren": {3, 3}, "Ilaria": {3, 4}, "Mago Sombrio": {5, 4}, "Brisa": {8, 8}},
			})
			a.dies(t, e, "Toren", a.toren)

			held, err := a.castRevivify(t, a.ana, e, "Ilaria", "Toren")
			if err != nil {
				t.Fatalf("CastSpell(Revivify) error = %v", err)
			}
			if held.GetCast() != nil {
				t.Fatalf("Revivify was not held for the wizard's Counterspell: %v", held.GetCast())
			}
			if !byLabel(t, a.get(t, a.master), "Toren").GetDefeated() {
				t.Error("Toren is alive while the cast waits")
			}
			if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 {
				t.Errorf("slots spent while the cast waits = %d, want 0", usedSlots(v, 3))
			}
			// Nothing of the turn moves, and nothing can be undone, while it waits.
			if _, err := a.castRevivify(t, a.ana, e, "Ilaria", "Toren"); err == nil {
				t.Error("a second Revivify while the window waits was accepted")
			}
			if err := a.undo(t, a.master, a.get(t, a.master), newKey()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Errorf("UndoLastAction while the cast waits = %v, want nothing to undo", err)
			}
			w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			if w == nil {
				t.Fatalf("the master's windows = %v, want the wizard's Counterspell", a.windows(t, a.master))
			}
			answer := passAnswer
			if counter {
				answer = useAnswer(3)
			}
			res := a.mustAnswer(t, a.master, e, w.GetId(), answer)

			if v := a.vitals(t, a.pens); usedSlots(v, 3) != 1 {
				t.Errorf("slots spent after the answer = %d, want 1 (a countered spell still expends it)", usedSlots(v, 3))
			}
			now := a.get(t, a.master)
			if len(now.GetReactionWindows()) != 0 {
				t.Errorf("windows open after the answer = %v, want none", now.GetReactionWindows())
			}
			toren := byLabel(t, now, "Toren")
			if counter {
				if !toren.GetDefeated() {
					t.Errorf("Toren after a countered Revivify = defeated %v, want still dead", toren.GetDefeated())
				}
				if !res.GetResult().GetCounterspell().GetCountered() {
					t.Errorf("the answer's result = %v, want countered", res.GetResult())
				}
				if len(now.GetDeaths()) != 1 {
					t.Errorf("deaths after a countered Revivify = %v, want Toren's", now.GetDeaths())
				}
				return
			}
			if toren.GetDefeated() || a.vitals(t, a.toren).GetHitPointsCurrent() != 1 {
				t.Errorf("Toren after the cast went through = defeated %v, want alive with 1 hit point", toren.GetDefeated())
			}
			if len(now.GetDeaths()) != 0 {
				t.Errorf("deaths after the cast went through = %v, want none", now.GetDeaths())
			}
			// The cast that went through closes the undo chain, as an unheld one does.
			if err := a.undo(t, a.master, now, newKey()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Errorf("UndoLastAction after the Revivify = %v, want nothing to undo", err)
			}
		})
	}
}

// The master's "faz menos de 1 minuto" while a combat is open: Ilaria asked before the combat, with
// Toren dead; the master brought him back, the combat began and he fell and was confirmed dead
// in it. The answer comes now. He lives again in the order too, not only on his sheet, and the
// combat's log says so.
func TestRevivify_OutsideACombatTheCombatantLivesAgainInAnOpenCombat(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	req, err := a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("RequestRevivify() error = %v", err)
	}
	if _, err := a.master.characters.ReviveCharacter(t.Context(), connect.NewRequest(&charactersv1.ReviveCharacterRequest{
		CampaignId: a.campaignID, CharacterId: a.toren.GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("ReviveCharacter() error = %v", err)
	}
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Ilaria": 12, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Ilaria": {3, 4}, "Goblin": {4, 3}, "Brisa": {8, 8}},
	})
	a.dies(t, e, "Toren", a.toren)
	if c := byLabel(t, a.get(t, a.master), "Toren"); !c.GetDefeated() {
		t.Fatalf("Toren after his death = defeated %v, want true", c.GetDefeated())
	}

	done, err := a.answerRevivify(t, a.master, req.GetId(), true, newKey())
	if err != nil || done.GetStatus() != playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_CONFIRMED {
		t.Fatalf("ConfirmRevivifyTime(true) = %v, %v; want confirmed", done, err)
	}

	now := a.get(t, a.master)
	if c := byLabel(t, now, "Toren"); c.GetDefeated() || c.GetDeathFailures() != 0 {
		t.Errorf("Toren in the order after the spell = defeated %v, failures %d; want alive and clean", c.GetDefeated(), c.GetDeathFailures())
	}
	if len(now.GetDeaths()) != 0 {
		t.Errorf("deaths after the spell = %v, want none", now.GetDeaths())
	}
	if v := a.vitals(t, a.toren); v.GetHitPointsCurrent() != 1 {
		t.Errorf("Toren's hit points = %d, want 1", v.GetHitPointsCurrent())
	}
	var line bool
	for _, r := range a.log(t, a.master, now).GetRounds() {
		for _, en := range r.GetEntries() {
			line = line || en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CHARACTER_REVIVED
		}
	}
	if !line {
		t.Error("the combat's log has no \"reviveu\" line")
	}
	if err := a.undo(t, a.master, now, newKey()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("UndoLastAction after the revival = %v, want nothing to undo", err)
	}
}

// A revived character's vitals are what they were: the hit dice spent stay spent (they are
// not given back by the revival), the resources too, and it has 1 hit point. A rest does not
// touch the dead; after the revival the character takes part in it like any other.
func TestRN03_ARevivedCharactersVitalsLineUpWithTheHitDiceAndTheRests(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 4,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
	})
	spent := a.correct(t, a.toren, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitDiceUsedByDie = map[int32]int32{10: 3} })
	if got := hitDiceUsed(spent)[10]; got != 3 {
		t.Fatalf("hit dice spent before the death = %d, want 3", got)
	}
	if _, err := a.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: a.campaignID, CharacterId: a.toren.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	rest := func(kind playv1.RestKind) *playv1.TakeRestResponse {
		t.Helper()
		res, err := a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: a.campaignID, Kind: kind, IdempotencyKey: newKey()}))
		if err != nil {
			t.Fatalf("TakeRest(%v) error = %v", kind, err)
		}
		return res.Msg
	}
	for _, v := range rest(playv1.RestKind_REST_KIND_LONG).GetVitals() {
		if v.GetCharacterId() == a.toren.GetId() {
			t.Errorf("a long rest touched the dead Toren: %v", v)
		}
	}
	if _, err := a.master.characters.ReviveCharacter(t.Context(), connect.NewRequest(&charactersv1.ReviveCharacterRequest{CampaignId: a.campaignID, CharacterId: a.toren.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("ReviveCharacter() error = %v", err)
	}
	v := a.vitals(t, a.toren)
	if v.GetHitPointsCurrent() != 1 || hitDiceUsed(v)[10] != 3 {
		t.Fatalf("Toren revived = %d hit points, hit dice spent %v; want 1 and {10: 3}", v.GetHitPointsCurrent(), hitDiceUsed(v))
	}
	// The long rest after it: hit points full and half the dice (2 of 4) back, like any other.
	var after *playv1.CharacterVitals
	for _, got := range rest(playv1.RestKind_REST_KIND_LONG).GetVitals() {
		if got.GetCharacterId() == a.toren.GetId() {
			after = got
		}
	}
	if after == nil {
		t.Fatal("a long rest does not touch the revived Toren")
	}
	if after.GetHitPointsCurrent() != after.GetHitPointsMax() || hitDiceUsed(after)[10] != 1 {
		t.Errorf("Toren after the long rest = %d/%d hit points, hit dice spent %v; want full and {10: 1}", after.GetHitPointsCurrent(), after.GetHitPointsMax(), hitDiceUsed(after))
	}
}
