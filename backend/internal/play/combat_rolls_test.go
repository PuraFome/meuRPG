package play

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// rollRPCs are the calls of the attack rolls' modes, the damage extras and the states of
// a combatant. They have their own matrix in this file.
var rollRPCs = []string{"RequestRollMode", "AnswerRollModeRequest", "CancelRollModeRequest", "RemoveDamagePart", "AnswerRageEnd", "EndRage"}

// The two d20 of a roll with advantage or disadvantage, for the tests that type them:
// the face that counts and a die that never does.

func advantage(face int32) func(*playv1.RollAttackRequest) {
	return func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{face, 1} }
}

func disadvantage(face int32) func(*playv1.RollAttackRequest) {
	return func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{face, 20} }
}

func castDisadvantage(face int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.D20Faces = []int32{face, 20} }
}

// The markers of the free text a player or the master writes with a roll: they may
// reach the master and the attacker's player, nobody else (RN-10, RN-20).
const (
	reasonMarker  = "LEAKCANARY-roll-reason-1"
	removalMarker = "LEAKCANARY-part-reason-1"
)

// readsOf is everything the combat gives a person, as text: the combat, its log and the
// options of a turn. A marker found there is a marker they read.
func (a *armed) readsOf(t *testing.T, u *user, e *playv1.Encounter, label string) string {
	t.Helper()
	text := jsonOf(a.get(t, u)) + jsonOf(a.log(t, u, e))
	if label != "" {
		if opts, err := a.options(t, u, e, label); err == nil {
			text += jsonOf(opts)
		}
	}
	return text
}

// rollsTable is a table with a rogue (Toren), a wizard (Pensantus) and a barbarian
// (Brisa) beside a goblin: Brisa stands next to it and plays first.
func rollsTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	scores := &rulesv1.AbilityScores{Strength: 16, Dexterity: 16, Constitution: 14, Intelligence: 12, Wisdom: 10, Charisma: 8}
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, scores, []string{rapier}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, scores, nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:barbarian", "race:human", 3, scores, []string{battleaxe}, nil)
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

func (a *armed) requestMode(t *testing.T, u *user, e *playv1.Encounter, attacker, key, target string, mode playv1.RollMode, reason string) (*playv1.RollModeRequest, error) {
	t.Helper()
	res, err := u.combat.RequestRollMode(t.Context(), connect.NewRequest(&playv1.RequestRollModeRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), AttackerId: a.id(t, attacker), AttackKey: key, TargetId: a.id(t, target),
		RollMode: mode, Reason: reason, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetRequest(), nil
}

func (a *armed) answerMode(t *testing.T, u *user, e *playv1.Encounter, id string, mode playv1.RollMode) error {
	t.Helper()
	_, err := u.combat.AnswerRollModeRequest(t.Context(), connect.NewRequest(&playv1.AnswerRollModeRequestRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), RequestId: id, DecidedMode: mode, IdempotencyKey: newKey(),
	}))
	return err
}

// A player may take disadvantage on their own; a better mode than the suggestion is
// the master's to give, on a request with a reason, and the reason stays with the
// master and the attacker (SRD 5.1, "Advantage and Disadvantage").
func TestPM06a_ABetterModeNeedsTheMastersApproval(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")

	opts := a.mustOptions(t, a.caio, e, "Toren")
	var target *playv1.TargetInReach
	for _, g := range opts.GetAttackTargets() {
		for _, tg := range g.GetTargets() {
			if tg.GetCombatantId() == a.id(t, "Goblin") {
				target = tg
			}
		}
	}
	if target == nil || target.GetRollMode() != playv1.RollMode_ROLL_MODE_NORMAL || len(target.GetSources()) != 0 {
		t.Fatalf("the goblin in reach = %v, want a normal roll with no source", target)
	}

	// Advantage the server did not suggest: refused until the master answers.
	_, err := a.attack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollMode, r.ModeReason, r.D20Faces = playv1.RollMode_ROLL_MODE_ADVANTAGE, reasonMarker, []int32{4, 17}
	})
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_NEEDS_APPROVAL)
	if _, err := a.requestMode(t, a.caio, e, "Toren", rapier, "Goblin", playv1.RollMode_ROLL_MODE_ADVANTAGE, ""); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a request with no reason: %v, want invalid_argument", err)
	}
	req, err := a.requestMode(t, a.caio, e, "Toren", rapier, "Goblin", playv1.RollMode_ROLL_MODE_ADVANTAGE, reasonMarker)
	if err != nil || req.GetStatus() != playv1.RollModeRequestStatus_ROLL_MODE_REQUEST_STATUS_PENDING {
		t.Fatalf("RequestRollMode() = %v, %v; want a pending request", req, err)
	}

	// The queue is the master's and the requester's.
	if got := a.get(t, a.master).GetRollModeRequests(); len(got) != 1 || got[0].GetReason() != reasonMarker {
		t.Errorf("the master's queue = %v, want the request with its reason", got)
	}
	if got := a.get(t, a.caio).GetRollModeRequests(); len(got) != 1 {
		t.Errorf("the requester's queue = %v, want the request", got)
	}
	if got := a.get(t, a.ana).GetRollModeRequests(); len(got) != 0 {
		t.Errorf("another player's queue = %v, want it empty", got)
	}
	if err := a.answerMode(t, a.caio, e, req.GetId(), playv1.RollMode_ROLL_MODE_ADVANTAGE); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player answering the request: %v, want permission_denied", err)
	}
	// Still waiting: the attack cannot use it.
	_, err = a.attack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollModeRequestId, r.D20Faces = req.GetId(), []int32{4, 17}
	})
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_PENDING)

	if err := a.answerMode(t, a.master, e, req.GetId(), playv1.RollMode_ROLL_MODE_ADVANTAGE); err != nil {
		t.Fatalf("AnswerRollModeRequest() error = %v", err)
	}
	// One d20 is not enough with advantage; two are.
	_, err = a.attack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollModeRequestId, r.D20Faces = req.GetId(), []int32{4}
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("one d20 with advantage: %v, want invalid_argument", err)
	}
	res := a.mustAttack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollModeRequestId, r.D20Faces = req.GetId(), []int32{4, 17}
	})
	roll := res.GetRoll()
	if roll.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || roll.GetSuggestedMode() != playv1.RollMode_ROLL_MODE_NORMAL ||
		roll.GetD20().GetCountedIndex() != 1 || len(roll.GetD20().GetFaces()) != 2 || roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the attack = %v, want advantage over a normal suggestion, the second d20 counted, a hit", roll)
	}

	// The log: the reason is the master's and Toren's player's alone.
	line := func(u *user) *playv1.CombatLogEntry {
		for _, en := range a.log(t, u, e).GetRounds()[0].GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetActorLabel() == "Toren" {
				return en
			}
		}
		return nil
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if en := line(u); en == nil || en.GetModeChange().GetReason() == "" && en.GetReason() != reasonMarker || en.GetModeChange().GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE {
			t.Errorf("%s reads the attack line %v, want the mode change and its reason", who, en)
		}
	}
	if en := line(a.ana); en == nil || en.GetModeChange() != nil || en.GetReason() != "" || en.GetAttackRoll() != nil {
		t.Errorf("another player reads %v, want neither the dice, the mode change nor the reason", en)
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if !strings.Contains(a.readsOf(t, u, e, ""), reasonMarker) {
			t.Errorf("%s does not read the reason (the control of the leak check)", who)
		}
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if got := a.readsOf(t, u, e, ""); strings.Contains(got, reasonMarker) {
			t.Errorf("%s reads the reason of a request that is not theirs", who)
		}
	}
}

// Disadvantage a player picks on their own needs a reason and rolls two dice.
func TestPM06a_APlayerMayTakeDisadvantageWithAReason(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	_, err := a.attack(t, a.bia, e, "Brisa", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollMode, r.D20Faces = playv1.RollMode_ROLL_MODE_DISADVANTAGE, []int32{15, 3}
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("disadvantage with no reason: %v, want invalid_argument", err)
	}
	res := a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollMode, r.ModeReason, r.D20Faces = playv1.RollMode_ROLL_MODE_DISADVANTAGE, "Escorregou na lama", []int32{15, 3}
	})
	roll := res.GetRoll()
	if roll.GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || roll.GetD20().GetCountedIndex() != 1 || roll.GetD20().GetTotal() != 3+roll.GetD20().GetModifier() {
		t.Errorf("the attack = %v, want the lower d20 to count", roll)
	}
}

// An attack with advantage and Sneak Attack, with real dice: the extra is offered,
// rolled in its own group, and counted once in the turn.
func TestPM06b_SneakAttackAddsItsDiceToAnAttackWithAdvantage(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")
	a.setConditions(t, e, "Goblin", "condition:restrained")

	opts := a.mustOptions(t, a.caio, a.get(t, a.caio), "Toren")
	found := false
	for _, g := range opts.GetAttackTargets() {
		for _, tg := range g.GetTargets() {
			if tg.GetCombatantId() == a.id(t, "Goblin") {
				found = tg.GetRollMode() == playv1.RollMode_ROLL_MODE_ADVANTAGE && len(tg.GetSources()) == 1 &&
					tg.GetSources()[0].GetKind() == playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_RESTRAINED_TARGET
			}
		}
	}
	if !found {
		t.Fatalf("the restrained goblin in reach = %v, want advantage from its condition", opts.GetAttackTargets())
	}
	res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	pending := res.GetPendingDamage()
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || pending == nil {
		t.Fatalf("the attack = %v, want a hit with advantage", res.GetRoll())
	}
	var sneak *playv1.DamagePart
	for _, p := range pending.GetParts() {
		if p.GetKey() == "sneak-attack" {
			sneak = p
		}
	}
	if sneak == nil || !sneak.GetChoosable() || !sneak.GetAvailable() || sneak.GetDiceCount() != 2 || sneak.GetDiceSides() != 6 {
		t.Fatalf("the offered parts = %v, want Sneak Attack, 2d6, available", pending.GetParts())
	}

	dmg := a.mustDamage(t, a.caio, a.get(t, a.caio), pending.GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		r.SelectedExtras = []*playv1.SelectedExtra{{Key: "sneak-attack"}}
		r.TypedParts = []*playv1.TypedPart{{PartKey: "weapon", Sum: 5}, {PartKey: "sneak-attack", Sum: 8}}
	}).GetPendingDamage()
	var weaponSum, sneakSum int32
	for _, r := range dmg.GetPartRolls() {
		switch r.GetPartKey() {
		case "weapon":
			weaponSum = r.GetSum()
		case "sneak-attack":
			sneakSum = r.GetSum()
			if !r.GetCounted() {
				t.Error("Sneak Attack rolled and did not count")
			}
		}
	}
	if weaponSum != 5 || sneakSum != 8 {
		t.Errorf("the part rolls = %v, want the weapon at 5 and Sneak Attack at 8", dmg.GetPartRolls())
	}
	if dmg.GetAmount() < 13 {
		t.Errorf("the damage = %d, want at least 13 (5 + 8 and the modifier)", dmg.GetAmount())
	}

}

// A raging barbarian takes half of the bludgeoning, piercing and slashing damage,
// and the master sees the step (SRD 5.1, Barbarian, Rage).
func TestPM07b_RageHalvesPhysicalDamageAndShowsTheStep(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e, err := a.action(t, a.bia, e, "Brisa", "feature:rage")
	if err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	var chip *playv1.CombatantEffect
	for _, s := range byLabel(t, e, "Brisa").GetStates() {
		if s.GetKind() == playv1.CombatantStateKind_COMBATANT_STATE_KIND_RAGE {
			chip = s
		}
	}
	if chip == nil || chip.GetLabelPt() != "Em fúria" {
		t.Fatalf("Brisa's states = %v, want Em fúria", byLabel(t, e, "Brisa").GetStates())
	}
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(2)) // attacked a hostile: the rage holds
	e = a.passTo(t, a.mustEndTurn(t, a.bia, e), "Goblin")

	a.h.roller.queue(4) // the goblin's d20 is typed; the damage die is the app's
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Brisa", d20(18))
	if hit.GetPendingDamage() == nil {
		t.Fatalf("the goblin's attack = %v, want a hit", hit.GetRoll())
	}
	dmg := a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	if dmg.GetAmount() != 6 || dmg.GetAmountAfterSteps() != 3 || len(dmg.GetSteps()) != 1 ||
		dmg.GetSteps()[0].GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_RESISTANCE || dmg.GetSteps()[0].GetBefore() != 6 || dmg.GetSteps()[0].GetAfter() != 3 {
		t.Fatalf("the damage = %v, want 6 halved to 3 by one resistance step", dmg)
	}
	before := byLabel(t, a.get(t, a.master), "Brisa").GetHitPointsCurrent()
	if _, err := a.settle(t, a.master, e, dmg.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	if got := byLabel(t, a.get(t, a.master), "Brisa").GetHitPointsCurrent(); got != before-3 {
		t.Errorf("Brisa's hit points = %d (was %d); want 3 lost", got, before)
	}
}

// A raging barbarian who neither attacked nor took damage is asked before the rage ends.
func TestPM07a_TheRageEndIsAQuestionToThePlayer(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e, err := a.action(t, a.bia, e, "Brisa", "feature:rage")
	if err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	e, err = a.endTurn(t, a.bia, e, false)
	if err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	brisa := a.id(t, "Brisa")
	if e.GetRagePendingCombatantId() != brisa || e.GetCurrentCombatantId() != brisa {
		t.Fatalf("after ending the turn: rage pending for %q, on turn %q; want Brisa for both", e.GetRagePendingCombatantId(), e.GetCurrentCombatantId())
	}
	answer := func(u *user, end bool) (*playv1.Encounter, error) {
		res, err := u.combat.AnswerRageEnd(t.Context(), connect.NewRequest(&playv1.AnswerRageEndRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: brisa, EndRage: end, IdempotencyKey: newKey(),
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetEncounter(), nil
	}
	if _, err := answer(a.caio, true); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player answering: %v, want permission_denied", err)
	}
	back, err := answer(a.bia, false)
	if err != nil || back.GetRagePendingCombatantId() != "" || back.GetCurrentCombatantId() != brisa {
		t.Fatalf("\"Voltar e atacar\" = %v, %v; want the turn kept and no question", back.GetCurrentCombatantId(), err)
	}
	a.mustAttack(t, a.bia, back, "Brisa", battleaxe, "Goblin", d20(2))
	next, err := a.endTurn(t, a.bia, a.get(t, a.bia), false)
	if err != nil || next.GetCurrentCombatantId() == brisa {
		t.Fatalf("EndTurn() after an attack = %v, %v; want the turn to pass", next.GetCurrentCombatantId(), err)
	}
}

// The master may leave a resistance out when he applies the damage.
func TestPM07b_TheMasterMayIgnoreAResistance(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e, err := a.action(t, a.bia, e, "Brisa", "feature:rage")
	if err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(2))
	e = a.passTo(t, a.mustEndTurn(t, a.bia, e), "Goblin")
	a.h.roller.queue(4)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Brisa", d20(18))
	dmg := a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	if len(dmg.GetSteps()) != 1 || len(dmg.GetSteps()[0].GetSourceKeys()) != 1 {
		t.Fatalf("the steps = %v, want one resistance with one source", dmg.GetSteps())
	}
	// Players never read the steps of a damage they do not take.
	if got := a.get(t, a.caio); len(got.GetCombatants()) == 0 {
		t.Fatal("Toren's player reads no combat")
	}
	before := byLabel(t, a.get(t, a.master), "Brisa").GetHitPointsCurrent()
	_, err = a.master.combat.ApplyPendingDamage(t.Context(), connect.NewRequest(&playv1.ApplyPendingDamageRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: dmg.GetId(), IdempotencyKey: newKey(),
		IgnoreModifiers: dmg.GetSteps()[0].GetSourceKeys(),
	}))
	if err != nil {
		t.Fatalf("ApplyPendingDamage(ignoring the resistance) error = %v", err)
	}
	if got := byLabel(t, a.get(t, a.master), "Brisa").GetHitPointsCurrent(); got != before-6 {
		t.Errorf("Brisa's hit points = %d (was %d); want the whole 6 lost", got, before)
	}
}

// The master takes an extra out of a damage with a reason; the weapon stays.
func TestPM06b_TheMasterTakesAnExtraOutWithAReason(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")
	a.setConditions(t, e, "Goblin", "condition:restrained")
	res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	pending := res.GetPendingDamage()
	a.mustDamage(t, a.caio, a.get(t, a.caio), pending.GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		r.SelectedExtras = []*playv1.SelectedExtra{{Key: "sneak-attack"}}
		r.TypedParts = []*playv1.TypedPart{{PartKey: "weapon", Sum: 3}, {PartKey: "sneak-attack", Sum: 5}}
	})
	remove := func(u *user, part, reason string) error {
		_, err := u.combat.RemoveDamagePart(t.Context(), connect.NewRequest(&playv1.RemoveDamagePartRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: pending.GetId(), PartKey: part, Reason: reason, IdempotencyKey: newKey(),
		}))
		return err
	}
	if err := remove(a.caio, "sneak-attack", "não vale"); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player removing a part: %v, want permission_denied", err)
	}
	if err := remove(a.master, "sneak-attack", ""); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("removing with no reason: %v, want invalid_argument", err)
	}
	if err := remove(a.master, "weapon", "a arma fica"); err == nil {
		t.Error("the weapon was taken out of its own damage")
	}
	hpBefore := byLabel(t, a.get(t, a.master), "Goblin").GetHitPointsCurrent()
	if err := remove(a.master, "sneak-attack", removalMarker); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if got := byLabel(t, a.get(t, a.master), "Goblin").GetHitPointsCurrent(); got <= hpBefore {
		t.Errorf("the goblin's hit points = %d (was %d), want what the extra took given back", got, hpBefore)
	}
	line := func(u *user) *playv1.CombatLogEntry {
		for _, en := range a.log(t, u, e).GetRounds()[0].GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_DAMAGE_PART_REMOVED {
				return en
			}
		}
		return nil
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if en := line(u); en == nil || en.GetReason() != removalMarker {
			t.Errorf("%s reads %v, want the removal with its reason", who, en)
		}
	}
	if en := line(a.ana); en != nil {
		t.Errorf("another player reads %v, want no removal line", en)
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if !strings.Contains(a.readsOf(t, u, e, ""), removalMarker) {
			t.Errorf("%s does not read the removal reason (the control of the leak check)", who)
		}
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if strings.Contains(a.readsOf(t, u, e, ""), removalMarker) {
			t.Errorf("%s reads the master's reason for a removal that is not theirs", who)
		}
	}
}

// The master's undo takes the rage back with the action that began it.
func TestPM07a_UndoTakesTheRageBack(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	if _, err := a.action(t, a.bia, e, "Brisa", "feature:rage"); err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	if got := byLabel(t, a.get(t, a.master), "Brisa").GetStates(); len(got) != 1 {
		t.Fatalf("Brisa's states = %v, want the rage", got)
	}
	a.undoLast(t, a.get(t, a.master))
	if got := byLabel(t, a.get(t, a.master), "Brisa").GetStates(); len(got) != 0 {
		t.Errorf("Brisa's states after the undo = %v, want none", got)
	}
}

// The calls of the roll modes, the removal of an extra and the rage end are authorized
// like the rest of the combat.
func TestPM06_RollCallsAreAuthorized(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	id := newKey()
	campaign, enc := a.campaignID, e.GetId()
	calls := map[string]func(u *user, ctx context.Context) error{
		"RequestRollMode": func(u *user, ctx context.Context) error {
			_, err := u.combat.RequestRollMode(ctx, connect.NewRequest(&playv1.RequestRollModeRequest{CampaignId: campaign, EncounterId: enc, AttackerId: a.id(t, "Brisa"), AttackKey: battleaxe, TargetId: a.id(t, "Goblin"), RollMode: playv1.RollMode_ROLL_MODE_ADVANTAGE, Reason: "x", IdempotencyKey: newKey()}))
			return err
		},
		"AnswerRollModeRequest": func(u *user, ctx context.Context) error {
			_, err := u.combat.AnswerRollModeRequest(ctx, connect.NewRequest(&playv1.AnswerRollModeRequestRequest{CampaignId: campaign, EncounterId: enc, RequestId: id, DecidedMode: playv1.RollMode_ROLL_MODE_ADVANTAGE, IdempotencyKey: newKey()}))
			return err
		},
		"CancelRollModeRequest": func(u *user, ctx context.Context) error {
			_, err := u.combat.CancelRollModeRequest(ctx, connect.NewRequest(&playv1.CancelRollModeRequestRequest{CampaignId: campaign, EncounterId: enc, RequestId: id, IdempotencyKey: newKey()}))
			return err
		},
		"RemoveDamagePart": func(u *user, ctx context.Context) error {
			_, err := u.combat.RemoveDamagePart(ctx, connect.NewRequest(&playv1.RemoveDamagePartRequest{CampaignId: campaign, EncounterId: enc, PendingDamageId: id, PartKey: "sneak-attack", Reason: "x", IdempotencyKey: newKey()}))
			return err
		},
		"AnswerRageEnd": func(u *user, ctx context.Context) error {
			_, err := u.combat.AnswerRageEnd(ctx, connect.NewRequest(&playv1.AnswerRageEndRequest{CampaignId: campaign, EncounterId: enc, CombatantId: a.id(t, "Brisa"), IdempotencyKey: newKey()}))
			return err
		},
		"EndRage": func(u *user, ctx context.Context) error {
			_, err := u.combat.EndRage(ctx, connect.NewRequest(&playv1.EndRageRequest{CampaignId: campaign, EncounterId: enc, CombatantId: a.id(t, "Brisa"), IdempotencyKey: newKey()}))
			return err
		},
	}
	if len(calls) != len(rollRPCs) {
		t.Fatalf("the matrix covers %d calls, the roll calls are %d", len(calls), len(rollRPCs))
	}
	outsider := a.h.newUser("Intruso")
	for name, call := range calls {
		if err := call(a.h.anonymous(), t.Context()); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("%s signed out: %v, want unauthenticated", name, err)
		}
		if err := call(outsider, t.Context()); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("%s as an outsider: %v, want not_found", name, err)
		}
		// The master sets a mode on the roll itself: asking himself is refused.
		if err := call(a.master, t.Context()); name != "RequestRollMode" && (connect.CodeOf(err) == connect.CodePermissionDenied || connect.CodeOf(err) == connect.CodeUnauthenticated) {
			t.Errorf("%s as the master: %v, want it past authorization", name, err)
		}
	}
	// Another player's character is not theirs to ask for, answer or end.
	for _, name := range []string{"RequestRollMode", "AnswerRageEnd", "EndRage"} {
		if err := calls[name](a.caio, t.Context()); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s on another player's character: %v, want permission_denied", name, err)
		}
	}
	for _, name := range []string{"AnswerRollModeRequest", "RemoveDamagePart"} {
		if err := calls[name](a.bia, t.Context()); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s as a player: %v, want permission_denied", name, err)
		}
	}
}
