package play

import (
	"connectrpc.com/connect"

	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The reactions of PM-04, one by one: a player's character that reacts to the goblin
// (the master plays it) or to a pit.

// reactorFight is a combat of Toren, the Reator (the character the test builds, Ana's),
// Brisa and a revealed Goblin next to the Reator, on the Goblin's turn.
func reactorFight(t *testing.T, build func(t *testing.T, a *armed) *charactersv1.Character) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		a.pens = build(t, a)
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3, abilities(10, 16, 14, 16, 8), []string{maceKey}, []string{sacredFlame}, nil, []string{cureWounds})
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Reator": {4, 4}, "Brisa": {8, 8}},
	})
	return a, a.passTo(t, e, "Goblin")
}

func rogueFive(t *testing.T, a *armed) *charactersv1.Character {
	t.Helper()
	return a.ana.hero(t, a.campaignID, "Reator", "class:rogue", "race:human", 5, abilities(10, 14, 12, 10, 8), []string{rapier}, nil)
}

// TestUncannyDodgeHalvesTheDamageOfAnAttackerTheRogueSees: the Goblin hits the rogue; its
// damage waits for the rogue, who reads its own damage (not the attacker's total or the
// goblin's numbers) and halves it (SRD, Rogue 5: rounded down).
func TestUncannyDodgeHalvesTheDamageOfAnAttackerTheRogueSees(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, rogueFive)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	if hit.GetPendingDamage().GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL {
		t.Fatalf("the hit waits for %v, want the damage roll: Uncanny Dodge comes after it", hit.GetPendingDamage().GetStatus())
	}
	held := a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	if held.GetPendingDamage().GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Fatalf("the damage landed before the rogue answered: %v", held.GetPendingDamage())
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	if w == nil || w.GetUncannyDodge().GetDamage() != 8 || w.GetUncannyDodge().GetHalved() != 4 {
		t.Fatalf("the rogue's window = %v, want 8 halved to 4", w)
	}
	if got := a.get(t, a.caio).GetReactionWait().GetTitlePt(); got != "Esperando a reação de Reator" {
		t.Errorf("Toren's player reads %q, want the rogue named", got)
	}
	res := a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(0))
	if r := res.GetResult().GetUncannyDodge(); r.GetDamageBefore() != 8 || r.GetDamageAfter() != 4 {
		t.Errorf("the result = %v, want 8 -> 4", r)
	}
	reator := byLabel(t, res.GetEncounter(), "Reator")
	if !reator.GetReactionUsed() {
		t.Error("the rogue's reaction is not used")
	}
	// A player's character takes damage when the master applies it: 4, not 8.
	if status, amount := a.pendingOf(t, hit.GetPendingDamage().GetId()); status != "rolled" || amount != 4 {
		t.Errorf("the pending damage = %s for %d, want rolled for 4", status, amount)
	}
}

// pendingOf is the status and the amount of a pending damage, read from the database.
func (a *armed) pendingOf(t *testing.T, id string) (status string, amount int32) {
	t.Helper()
	var amt *int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT status, amount FROM pending_damages WHERE id = $1`, id).Scan(&status, &amt); err != nil {
		t.Fatalf("read pending damage %s: %v", id, err)
	}
	if amt != nil {
		amount = *amt
	}
	return status, amount
}

func tieflingFighter(t *testing.T, a *armed) *charactersv1.Character {
	t.Helper()
	return a.ana.hero(t, a.campaignID, "Reator", "class:fighter", "race:tiefling", 3, abilities(10, 14, 14, 10, 14), []string{rapier}, nil)
}

// TestHellishRebukeThroughTheInfernalLegacyAsksTheMastersSave: a tiefling of the 3rd
// level hurt by the Goblin answers with the Infernal Legacy (a 2nd-level Hellish
// Rebuke, once a long rest): the Goblin's saving throw is the master's, and the fire
// lands on it. The legacy is spent and is not offered again.
func TestHellishRebukeThroughTheInfernalLegacyAsksTheMastersSave(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, tieflingFighter)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(3))
	if w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE); w != nil {
		t.Fatalf("the window opened before the damage landed: %v", w)
	}
	if _, err := a.settle(t, a.master, e, hit.GetPendingDamage().GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
	if w == nil {
		t.Fatal("the tiefling has no Hellish Rebuke window after being hurt")
	}
	var racial *playv1.HellishRebukeOption
	for _, o := range w.GetHellishRebuke().GetOptions() {
		if o.GetRacial() {
			racial = o
		}
	}
	if racial == nil || racial.GetLevel() != 2 || racial.GetUsesLeft() != 1 {
		t.Fatalf("the options = %v, want the Infernal Legacy at the 2nd level with one use", w.GetHellishRebuke().GetOptions())
	}
	res := a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.UseRacial = playv1.ReactionChoice_REACTION_CHOICE_USE, true
	})
	if res.GetResult().GetNextWindowId() != w.GetId() {
		t.Fatalf("the result = %v, want the window to go on to the Goblin's save", res.GetResult())
	}
	// The Goblin is an NPC: its save is the master's, never the player's.
	if _, err := a.answerReaction(t, a.ana, res.GetEncounter(), w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 3}
	}); err == nil {
		t.Error("the tiefling's player rolled the Goblin's saving throw")
	}
	a.h.roller.queue(8, 9, 7) // the fire: 3d10
	save := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
	if save == nil || !save.GetSecondStep() {
		t.Fatalf("the master's window = %v, want the second step", save)
	}
	done := a.mustAnswer(t, a.master, e, save.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 3}
	})
	r := done.GetResult().GetHellishRebuke()
	if r.GetSaved() || r.GetDamage() != 24 || !r.GetRacial() {
		t.Errorf("the result = %v, want a failed save and 24 fire", r)
	}
	if goblin := byLabel(t, done.GetEncounter(), "Goblin"); !goblin.GetDefeated() && goblin.GetHitPointsCurrent() != 0 {
		t.Errorf("the Goblin = %v, want it down", goblin)
	}
	if left := a.legacyLeft(t, a.pens.GetId()); left != 0 {
		t.Errorf("the Infernal Legacy has %d uses left, want none", left)
	}
}

// legacyLeft is how many uses of the Infernal Legacy the character has left.
func (a *armed) legacyLeft(t *testing.T, characterID string) int32 {
	t.Helper()
	var used *string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT resources_used->>'infernal_legacy' FROM character_vitals WHERE character_id = $1`, characterID).Scan(&used); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("read the legacy: %v", err)
	}
	n := int32(0)
	if used != nil {
		_, _ = fmt.Sscan(*used, &n)
	}
	return 1 - n
}

func loreBard(t *testing.T, a *armed) *charactersv1.Character {
	t.Helper()
	return a.ana.heroWith(t, a.campaignID, "Reator", classLevel("class:bard", 3, "subclass:lore"), abilities(10, 14, 12, 10, 16), []string{rapier}, nil, nil)
}

// TestCuttingWordsTakesTheDieOffAnAttackAgainstAnAlly: the Goblin's attack on Toren
// waits for the bard (the default setting asks on attacks); the bard's die comes off the
// roll before it is compared with the armor class, one Bardic Inspiration is spent, and
// the bard's player reads only "errou" of the result, never a total (RN-20).
func TestCuttingWordsTakesTheDieOffAnAttackAgainstAnAlly(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, loreBard)
	held := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(8)) // 8 + 4 = 12 against Toren's 11
	if !held.GetRoll().GetHeldForReaction() || held.GetPendingDamage() != nil {
		t.Fatalf("the attack = %v, want it held for the bard", held)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil || w.GetCuttingWords().GetRollKind() != playv1.ReactionRollKind_REACTION_ROLL_KIND_ATTACK || w.GetCuttingWords().GetDieSides() != 6 || w.GetCuttingWords().GetUsesLeft() != 3 {
		t.Fatalf("the bard's window = %v, want a d6 on an attack with 3 uses", w)
	}
	if w.GetTrigger() != nil {
		t.Errorf("the bard's player reads the master's trigger numbers: %v", w.GetTrigger())
	}
	res := a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 5}
	})
	cw := res.GetResult().GetCuttingWords()
	if !cw.GetEffective() || cw.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || cw.GetDie().GetTotal() != 5 {
		t.Errorf("the result = %v, want the die 5 to make it a miss", res.GetResult())
	}
	if n := len(res.GetEncounter().GetReactionWindows()); n != 0 {
		t.Errorf("windows left: %d", n)
	}
	if byLabel(t, res.GetEncounter(), "Reator").GetReactionUsed() != true {
		t.Error("the bard's reaction is not used")
	}
}

func monkThree(t *testing.T, a *armed) *charactersv1.Character {
	t.Helper()
	return a.ana.heroWith(t, a.campaignID, "Reator", classLevel("class:monk", 3, "subclass:open-hand"), abilities(10, 16, 12, 10, 14), nil, nil, nil)
}

// TestDeflectMissilesCatchesTheArrowAndThrowsItBack: the Goblin's arrow hits the monk; the
// reduction 1d10 + Dexterity + monk level (8 + 3 + 3) takes all of its 8 damage, so the
// monk catches it, and for 1 ki throws it back with its own proficiency, as part of the
// same reaction (SRD, Monk 3).
func TestDeflectMissilesCatchesTheArrowAndThrowsItBack(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, monkThree)
	hit := a.mustAttack(t, a.master, e, "Goblin", shortBow, "Reator", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES)
	if w == nil || w.GetDeflectMissiles().GetDamage() != 8 || w.GetDeflectMissiles().GetFlatBonus() != 6 {
		t.Fatalf("the monk's window = %v, want 8 damage and a +6 reduction", w)
	}
	res := a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 8}
	})
	d := res.GetResult().GetDeflectMissiles()
	if !d.GetCaught() || d.GetDamageAfter() != 0 || !d.GetThrowBackAvailable() || res.GetResult().GetNextWindowId() != w.GetId() {
		t.Fatalf("the result = %v, want the arrow caught and the throw back offered", res.GetResult())
	}
	if throw := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES); throw == nil || throw.GetDeflectThrow().GetKiLeft() != 3 || !throw.GetSecondStep() {
		t.Fatalf("the monk's second window = %v, want the throw back with 3 ki", throw)
	}
	thrown, err := a.throwBack(t, a.ana, e, w.GetId(), "Reator", "Goblin", d20(15))
	if err != nil {
		t.Fatalf("RollAttack(throw back) error = %v", err)
	}
	if thrown.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || thrown.GetPendingDamage() == nil {
		t.Errorf("the thrown arrow = %v, want a hit", thrown.GetRoll())
	}
	if w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES); w != nil {
		t.Errorf("the window stayed open after the throw: %v", w)
	}
	// Another throw with the same window is refused: the missile is gone.
	if _, err := a.throwBack(t, a.ana, e, w.GetId(), "Reator", "Goblin", d20(15)); err == nil {
		t.Error("the same arrow was thrown twice")
	}
}

// throwBack is the RollAttack of a Deflect Missiles throw back.
func (a *armed) throwBack(t *testing.T, u *user, e *playv1.Encounter, windowID, attacker, target string, roll func(*playv1.RollAttackRequest)) (*playv1.RollAttackResponse, error) {
	t.Helper()
	req := &playv1.RollAttackRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), AttackerId: a.id(t, attacker), AttackKey: shortBow, TargetId: a.id(t, target),
		IdempotencyKey: newKey(), CatchWindowId: windowID,
	}
	roll(req)
	res, err := u.combat.RollAttack(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}
