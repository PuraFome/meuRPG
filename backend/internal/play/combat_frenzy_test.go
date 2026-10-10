package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The Berserker's Frenzy (SRD 5.1, Barbarian, Path of the Berserker): the barbarian may go
// into a frenzy for a rage; on each of its later turns one melee weapon attack is a bonus
// action, and when the rage ends it gains one level of exhaustion.

var blockedFrenzyUnavailable = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_FRENZY_UNAVAILABLE

// berserkerTable starts the fight with Brisa the barbarian in the order's first place, next to the
// goblin. level and subclass are the barbarian's.
func berserkerTable(t *testing.T, level int32, subclass string) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, abilities(16, 16, 14, 12, 8), []string{rapier}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.heroWith(t, a.campaignID, "Brisa", classLevel("class:barbarian", level, subclass), abilities(16, 14, 14, 10, 8), []string{battleaxe}, nil, nil)
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Brisa": 18, "Toren": 12, "Pensantus": 5},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Brisa": {3, 3}, "Goblin": {4, 3}, "Toren": {4, 4}, "Pensantus": {12, 3}},
	})
	return a, e
}

// mustEndTurnDiscarding ends the turn in course as the master, dropping any damage left.
func (a *armed) mustEndTurnDiscarding(t *testing.T, e *playv1.Encounter) *playv1.Encounter {
	t.Helper()
	out, err := a.endTurn(t, a.master, e, true)
	if err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	return out
}

// rage takes Fúria as Brisa's player, in a frenzy or not.
func (a *armed) rage(t *testing.T, e *playv1.Encounter, frenzy bool) (*playv1.Encounter, error) {
	t.Helper()
	res, err := a.bia.combat.TakeAction(t.Context(), connect.NewRequest(&playv1.TakeActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Brisa"), ActionKey: "feature:rage", IdempotencyKey: newKey(), Frenzy: frenzy,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetEncounter(), nil
}

func (a *armed) endRage(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	if _, err := a.bia.combat.EndRage(t.Context(), connect.NewRequest(&playv1.EndRageRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Brisa"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndRage() error = %v", err)
	}
}

// brisasExhaustion is the level of exhaustion the master reads on Brisa.
func (a *armed) brisasExhaustion(t *testing.T) int32 {
	t.Helper()
	return byLabel(t, a.get(t, a.master), "Brisa").GetExhaustionLevel()
}

// frenzyLines are the exhaustion lines of the log that say the level came from a frenzy.
func (a *armed) frenzyLines(t *testing.T, u *user, e *playv1.Encounter) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_EXHAUSTION && en.GetEffect().GetReason() == "frenzy" {
				out = append(out, en)
			}
		}
	}
	return out
}

// frenzyChip is Brisa's chip of the rage, or nil.
func frenzyChip(t *testing.T, e *playv1.Encounter) *playv1.CombatantEffect {
	t.Helper()
	for _, s := range byLabel(t, e, "Brisa").GetStates() {
		if s.GetKind() == playv1.CombatantStateKind_COMBATANT_STATE_KIND_RAGE {
			return s
		}
	}
	return nil
}

// The chip says "Em frenesi" and the turn of the rage offers no bonus attack: the frenzy attack is for the
// turns after the one the rage began in. From the next turn the axe is also a bonus action attack once the
// action is spent, and only once; taking it back gives the bonus action back.
func TestFrenzyOffersTheBonusAttackFromTheNextTurnOnce(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	e, err := a.rage(t, e, true)
	if err != nil {
		t.Fatalf("TakeAction(Rage, frenzy) error = %v", err)
	}
	chip := frenzyChip(t, e)
	if chip == nil || !chip.GetFrenzy() || chip.GetLabelPt() != "Em frenesi" {
		t.Fatalf("Brisa's rage chip = %v, want Em frenesi", chip)
	}
	if chip.GetSourceId() != "" {
		t.Errorf("the chip's source = %q, want none (a frenzy is not a state from another combatant)", chip.GetSourceId())
	}

	// The turn of the rage: the action's attack is the only one; the axe is no bonus attack.
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	if rule := attackOption(a.mustOptions(t, a.bia, e, "Brisa"), battleaxe).GetBonusRule(); rule == rulesv1.BonusAttackRule_BONUS_ATTACK_RULE_FRENZY {
		t.Errorf("the axe in the turn of the rage = rule %v, want no frenzy attack yet", rule)
	}
	_, err = a.attack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	wantBlockedBy(t, "a second attack in the turn of the rage", err, blockedActionUsed)

	// The next turn: the action's attack first, then the frenzy attack with the bonus action.
	e = a.passTo(t, a.mustEndTurnDiscarding(t, e), "Brisa")
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	opt := attackOption(a.mustOptions(t, a.bia, e, "Brisa"), battleaxe)
	if !opt.GetEnabled() || opt.GetBonusRule() != rulesv1.BonusAttackRule_BONUS_ATTACK_RULE_FRENZY {
		t.Fatalf("the axe after the action's attack = enabled %v, rule %v, want the enabled frenzy attack", opt.GetEnabled(), opt.GetBonusRule())
	}
	if opt.GetAttack().GetDamageDice().GetBonus() != 3 {
		t.Errorf("the frenzy attack's damage bonus = %d, want the Strength modifier kept (+3)", opt.GetAttack().GetDamageDice().GetBonus())
	}
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	_, err = a.attack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	wantBlockedBy(t, "a second frenzy attack in one turn", err, blockedBonusUsed)
	spent := attackOption(a.mustOptions(t, a.bia, e, "Brisa"), battleaxe)
	if spent.GetEnabled() || spent.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_BONUS_ACTION_USED {
		t.Errorf("the axe with the bonus action spent = enabled %v, reason %v, want BONUS_ACTION_USED", spent.GetEnabled(), spent.GetReason().GetCode())
	}

	// Taking the frenzy attack back gives the bonus action back.
	l := a.log(t, a.master, e)
	if err := a.undo(t, a.master, e, l.GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
}

// The frenzy attack needs no Attack action before it: the barbarian that spent its action on something
// else (Dash) still has it, and it is still the bonus action, not the action.
func TestFrenzyAttackFollowsNoAttackAction(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	if _, err := a.rage(t, e, true); err != nil {
		t.Fatalf("TakeAction(Rage, frenzy) error = %v", err)
	}
	e = a.passTo(t, a.mustEndTurnDiscarding(t, e), "Brisa")
	if e, err := a.action(t, a.bia, e, "Brisa", "standard:dash"); err != nil {
		t.Fatalf("TakeAction(Dash) error = %v", err)
	} else if rule := attackOption(a.mustOptions(t, a.bia, e, "Brisa"), battleaxe).GetBonusRule(); rule != rulesv1.BonusAttackRule_BONUS_ATTACK_RULE_FRENZY {
		t.Fatalf("the axe after the Dash = rule %v, want the frenzy attack", rule)
	}
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	_, err := a.attack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	wantBlockedBy(t, "a second attack", err, blockedBonusUsed)
}

// The rage that ends, by the bonus action, leaves exactly one level of exhaustion and says so in the log,
// for the master and for the barbarian's player alone. The next turns add nothing.
func TestFrenzyEndingTheRageAddsOneLevelOfExhaustion(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	if _, err := a.rage(t, e, true); err != nil {
		t.Fatalf("TakeAction(Rage, frenzy) error = %v", err)
	}
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	if got := a.brisasExhaustion(t); got != 0 {
		t.Fatalf("exhaustion while the frenzy lasts = %d, want 0", got)
	}
	e = a.passTo(t, a.mustEndTurnDiscarding(t, e), "Brisa")
	a.endRage(t, e)
	if got := a.brisasExhaustion(t); got != 1 {
		t.Fatalf("exhaustion after the frenzied rage = %d, want 1", got)
	}
	if got := frenzyChip(t, a.get(t, a.master)); got != nil {
		t.Errorf("the rage chip after the end = %v, want none", got)
	}
	lines := a.frenzyLines(t, a.master, e)
	if len(lines) != 1 || lines[0].GetEffect().GetExhaustionLevel() != 1 || lines[0].GetEffect().GetExhaustionBefore() != 0 {
		t.Fatalf("the master's frenzy lines = %v, want one, from level 0 to 1", lines)
	}
	if got := a.frenzyLines(t, a.bia, e); len(got) != 1 {
		t.Errorf("the barbarian's player reads %d frenzy lines, want 1", len(got))
	}
	if got := a.frenzyLines(t, a.caio, e); len(got) != 0 {
		t.Errorf("another player reads %v, want no frenzy line (the exhaustion is the owner's and the master's)", got)
	}
	// Nothing more comes: the rage is over.
	e = a.passTo(t, a.mustEndTurnDiscarding(t, e), "Brisa")
	a.passTo(t, a.mustEndTurnDiscarding(t, e), "Brisa")
	if got := a.brisasExhaustion(t); got != 1 {
		t.Errorf("exhaustion two turns later = %d, want still 1", got)
	}
}

// A turn that ends with no attack on a hostile creature and no damage asks the player whether the rage ends
// (SRD 5.1, Rage); letting it end is the end of the frenzy too.
func TestFrenzyTheTurnEndAnswerAddsExhaustion(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	if _, err := a.rage(t, e, true); err != nil {
		t.Fatalf("TakeAction(Rage, frenzy) error = %v", err)
	}
	e, err := a.endTurn(t, a.bia, a.get(t, a.bia), false)
	if err != nil || e.GetRagePendingCombatantId() != a.id(t, "Brisa") {
		t.Fatalf("EndTurn() = %v, %v; want the question about the rage", e.GetRagePendingCombatantId(), err)
	}
	if got := a.brisasExhaustion(t); got != 0 {
		t.Fatalf("exhaustion while the question waits = %d, want 0", got)
	}
	if _, err := a.bia.combat.AnswerRageEnd(t.Context(), connect.NewRequest(&playv1.AnswerRageEndRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Brisa"), EndRage: true, IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("AnswerRageEnd(end) error = %v", err)
	}
	if got := a.brisasExhaustion(t); got != 1 {
		t.Errorf("exhaustion after the rage ended at the turn's end = %d, want 1", got)
	}
}

// A frenzied rage ends with the combat, and leaves its level.
func TestFrenzyTheEndOfTheCombatAddsExhaustion(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	if _, err := a.rage(t, e, true); err != nil {
		t.Fatalf("TakeAction(Rage, frenzy) error = %v", err)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if got := a.brisasExhaustion(t); got != 1 {
		t.Errorf("exhaustion after the combat ended in a frenzied rage = %d, want 1", got)
	}
	if got := a.frenzyLines(t, a.master, e); len(got) != 1 {
		t.Errorf("the master's frenzy lines = %d, want 1", len(got))
	}
}

// A rage without a frenzy never adds exhaustion, and a Berserker may rage without it.
func TestARageWithoutFrenzyAddsNoExhaustion(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	e, err := a.rage(t, e, false)
	if err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	if chip := frenzyChip(t, e); chip == nil || chip.GetFrenzy() || chip.GetLabelPt() != "Em fúria" {
		t.Fatalf("Brisa's rage chip = %v, want Em fúria without the frenzy", chip)
	}
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	e = a.passTo(t, a.mustEndTurnDiscarding(t, e), "Brisa")
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	if rule := attackOption(a.mustOptions(t, a.bia, e, "Brisa"), battleaxe).GetBonusRule(); rule == rulesv1.BonusAttackRule_BONUS_ATTACK_RULE_FRENZY {
		t.Errorf("the axe in a plain rage = rule %v, want no frenzy attack", rule)
	}
	a.endRage(t, e)
	if got := a.brisasExhaustion(t); got != 0 {
		t.Errorf("exhaustion after a plain rage = %d, want 0", got)
	}
	if got := a.frenzyLines(t, a.master, e); len(got) != 0 {
		t.Errorf("frenzy lines of a plain rage = %v, want none", got)
	}
}

// Only a Berserker has Frenzy: a barbarian of another path or of level 2, with no subclass yet, is refused the
// choice with a typed reason, its Fúria is not offered the question, and nothing is spent.
func TestFrenzyIsRefusedWithoutTheFeature(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		level    int32
		subclass string
	}{
		"a barbarian of level 2":                {2, ""},
		"a barbarian of level 3 without a path": {3, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, e := berserkerTable(t, c.level, c.subclass)
			_, err := a.rage(t, e, true)
			wantBlockedBy(t, "Fúria with a frenzy", err, blockedFrenzyUnavailable)
			if got := byLabel(t, a.get(t, a.master), "Brisa").GetStates(); len(got) != 0 {
				t.Errorf("Brisa's states after the refusal = %v, want none (the refusal left nothing behind)", got)
			}
			for _, f := range a.mustOptions(t, a.bia, e, "Brisa").GetOptions().GetFeatureActions() {
				if f.GetOffersFrenzy() {
					t.Errorf("%s offers the frenzy question", f.GetAction().GetKey())
				}
			}
			if _, err := a.rage(t, e, false); err != nil {
				t.Errorf("TakeAction(Rage) after the refusal error = %v, want the plain rage to work", err)
			}
		})
	}
}

// The Berserker's Fúria asks the question; the undo of the rage takes the frenzy back with no exhaustion.
func TestFrenzyIsOfferedToTheBerserkerAndUndone(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	offered := false
	for _, f := range a.mustOptions(t, a.bia, e, "Brisa").GetOptions().GetFeatureActions() {
		if f.GetAction().GetKey() == "feature:rage" && f.GetOffersFrenzy() {
			offered = true
		}
	}
	if !offered {
		t.Fatal("the Berserker's Fúria does not offer the frenzy")
	}
	if _, err := a.rage(t, e, true); err != nil {
		t.Fatalf("TakeAction(Rage, frenzy) error = %v", err)
	}
	a.undoLast(t, a.get(t, a.master))
	if got := byLabel(t, a.get(t, a.master), "Brisa").GetStates(); len(got) != 0 {
		t.Errorf("Brisa's states after the undo = %v, want none", got)
	}
	if got := a.brisasExhaustion(t); got != 0 {
		t.Errorf("exhaustion after the undo = %d, want 0", got)
	}
}
