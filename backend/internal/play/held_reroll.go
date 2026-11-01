package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The d20 rolled again by a reroll rule of the character's table features (the engine's reroll effect, link.RerollRule:
// Applies the kind of roll, AtMost the highest face that may be rolled again, Resource and Cost the uses it spends, none
// for a free rule). SRD 5.1 puts it the way it puts the halfling's Lucky: the die is rolled again and the new roll must
// be used, even when it is worse. It is a link of the chain (held_questions.go) between Lucky's and the Bardic
// Inspiration die's, since the die is added to whatever the d20 is after it. The question comes as soon as the d20 is
// rolled, never later: the d20 that counts shows AtMost or lower, no 1 waits for Lucky's question first (the roll that
// Lucky asked about is not offered again, nor is a face it rolled again), and the character can pay a rule that covers
// the kind (the first of its rules, in the sheet's order). The new roll is final: it is never asked about again, even
// when it is another low face.
//
// The kinds that ask it are the ones the registry gives an applies word: the weapon attack and the spell attack
// ("attack"), the scene check and the group check ("check"), the effect's saving throw and the concentration save
// ("save"). The contests, Hide, the Counterspell check, the death save, the initiative, the trap search and the puzzle
// hint try never ask it, whatever the character has.
//
// Nothing is asked unless Service.d20Rerolls is on (Config.D20Rerolls): the web client does not answer the question yet,
// so with the switch off the roll settles as if the player declined, with nothing held, spent or written.
//
// The reroll is carried where Lucky's is (luckyReroll, with the rule's source key and uses), so every path that tells the
// d20 Lucky rolled again tells this one too, and the undo of a weapon attack or a cast gives the uses back (actionEvent.D20Reroll).

// rerollAsk is the rule a d20 is held for: the die that showed the low face and what answering yes spends.
type rerollAsk struct {
	Index    int32
	AtMost   int32
	Resource string
	Cost     int32
	Source   string
}

// rerollRulesFor are the reroll rules of the traits that cover the kind of roll (rules.RollApplies*), in the sheet's
// order. A rule with no resource is free; one with a resource must cost something.
func rerollRulesFor(t link.Traits, applies string) []link.RerollRule {
	var out []link.RerollRule
	for _, r := range t.RerollRules {
		if r.AtMost > 0 && (r.Resource == "" || r.Cost > 0) && slices.Contains(r.Applies, applies) {
			out = append(out, r)
		}
	}
	return out
}

// rerollsLeft are the reroll rules the roller could use on this roll, the ones it can pay: only a roll of a player's
// character that may ask, of a kind that has an applies word, with the switch on, is looked at, so that the sheet and the
// vitals of every other roll are never read.
func (s *Service) rerollsLeft(ctx context.Context, env gateEnv, spec rollSpec) ([]link.RerollRule, error) {
	applies := heldRegistry[spec.Kind].rerollApplies
	r := spec.Roller
	if !s.d20Rerolls || applies == "" || !spec.MayAsk || r.CharacterID == "" || (r.Combatant != nil && r.Combatant.Kind != kindPlayer) {
		return nil, nil
	}
	sheet, err := s.roster.CombatSheet(ctx, env.tx, r.CampaignID, r.CharacterID)
	if err != nil {
		return nil, fmt.Errorf("read the sheet: %w", err)
	}
	rules := rerollRulesFor(sheet.Traits, applies)
	if len(rules) == 0 {
		return nil, nil
	}
	v, err := s.vitals.GetVitalsTx(ctx, env.tx, r.CampaignID, r.CharacterID)
	if err != nil {
		return nil, fmt.Errorf("read the vitals: %w", err)
	}
	left := usesLeftOf(v)
	return slices.DeleteFunc(rules, func(rule link.RerollRule) bool { return rule.Resource != "" && int(left[rule.Resource]) < rule.Cost }), nil
}

// rerollAvailable says the roll may be held for a reroll: a failure to read counts as no, since the gate reads again and
// reports the error.
func (s *Service) rerollAvailable(ctx context.Context, env gateEnv, spec rollSpec) bool {
	rules, err := s.rerollsLeft(ctx, env, spec)
	return err == nil && len(rules) > 0
}

// rerollAt is the rule the d20 as rolled is held for, nil when it is not: no 1 waits for Lucky first, and the die that
// counts shows the limit of a rule the roller can pay, or lower.
func (ae answering) rerollAt(roll d20Roll, luckyAt *int32) *rerollAsk {
	if luckyAt != nil {
		return nil
	}
	face := roll.Face()
	i := slices.IndexFunc(ae.st.Rerolls, func(r link.RerollRule) bool { return face <= r.AtMost })
	if i < 0 {
		return nil
	}
	r := ae.st.Rerolls[i]
	return &rerollAsk{Index: clamp32(roll.Index, 0, 1), AtMost: clamp32(r.AtMost, 1, d20Sides), Resource: r.Resource, Cost: clamp32(r.Cost, 0, heldBonusLimit), Source: r.Source}
}

// keep writes the rule the roll is held for in the hold.
func (r *rerollAsk) keep(h *heldRoll) {
	if r == nil {
		return
	}
	h.RerollAsk, h.RerollIndex, h.RerollAtMost = true, new(r.Index), new(r.AtMost)
	h.RerollResource, h.RerollCost, h.RerollSource = r.Resource, r.Cost, r.Source
}

// rerollAsks says the roll waits for the reroll answer.
func rerollAsks(h heldRoll) bool {
	return h.RerollAsk && h.RerollAnswerKey == nil && h.AnswerKey == nil
}

// rerolledOf is the d20 a hold's answer rolled again: Lucky's, or the reroll rule's (the two never meet in one roll); nil
// before the answer, when the player kept the roll, and for a roll that asked neither.
func rerolledOf(h heldRoll) *luckyReroll {
	if l := luckyOf(h); l != nil {
		return l
	}
	if h.RerollFrom == nil || h.RerollTo == nil {
		return nil
	}
	return &luckyReroll{
		Index: clamp32(int(derefInt32(h.RerollIndex)), 0, 1), From: *h.RerollFrom, To: *h.RerollTo,
		Source: h.RerollSource, Resource: h.RerollResource, Cost: h.RerollCost,
	}
}

// rerollUse is the uses a d20 rolled again spent, for the undo: nil for a roll Lucky rolled again, a free rule, or none.
func rerollUse(r *luckyReroll) *damageRerollUse {
	if r == nil || r.Resource == "" || r.Cost <= 0 {
		return nil
	}
	return &damageRerollUse{Resource: r.Resource, Cost: r.Cost}
}

// rollUsesBack gives back the uses the d20 of an attack spent: Stroke of Luck's and a reroll rule's.
func (u *undoer) rollUsesBack(ctx context.Context, who playdb.Combatant, ev actionEvent) error {
	if err := u.strokeBack(ctx, who, ev); err != nil {
		return err
	}
	return u.d20RerollBack(ctx, who, ev)
}

// d20RerollBack gives back the uses a weapon attack or a cast spent on a d20 rolled again.
func (u *undoer) d20RerollBack(ctx context.Context, who playdb.Combatant, ev actionEvent) error {
	if ev.D20Reroll == nil {
		return nil
	}
	return u.spendResourcesBack(ctx, who, ev.D20Reroll.Resource, ev.D20Reroll.Cost)
}

func (st inspirationHoldStore) answerReroll(ctx context.Context, h heldRoll) error {
	if err := st.q.AnswerInspirationHoldReroll(ctx, playdb.AnswerInspirationHoldRerollParams{
		ID: h.ID, Faces: faces32(h.Faces), Counted: clamp32(h.Counted, 0, 1), Total: clamp32(h.Total, -heldBonusLimit, heldBonusLimit),
		RerollAnswerKey: h.RerollAnswerKey, RerollFrom: h.RerollFrom, RerollTo: h.RerollTo,
	}); err != nil {
		return fmt.Errorf("answer the reroll: %w", err)
	}
	return nil
}

func (hs heldStores) answerReroll(ctx context.Context, h heldRoll) error {
	return hs.st.answerReroll(ctx, h)
}

// rerollQuestion is "Rolar de novo?".
type rerollQuestion struct{}

func (rerollQuestion) kind() questionKind { return askReroll }

// pending: the question was asked and neither it nor the roll was answered.
func (rerollQuestion) pending(h heldRoll, _ rollerState) bool { return rerollAsks(h) }

func (rerollQuestion) offer(h heldRoll, st rollerState) heldOffer {
	return heldOffer{Hold: h, State: st, Asks: askReroll}
}

// apply writes the answer. A player who rolls again gets the d20 at reroll_index rolled by the service's roller (the
// answer carries no die), replaced in the faces; the die that counts is picked again by the roll's mode and Reliable
// Talent counts the new face, as for Lucky. The rule's uses are spent through the vitals in the same transaction as the
// write. The new roll stands, even when it is worse, and is never asked about again. A player who keeps the roll spends
// nothing. The same answer again (a retry) finds the roll as it left it.
func (q rerollQuestion) apply(ctx context.Context, ae answering, h heldRoll, a heldReply) (heldRoll, error) {
	if h.RerollAnswerKey != nil && *h.RerollAnswerKey == a.key {
		return h, nil
	}
	if !q.pending(h, ae.st) {
		return h, connect.NewError(connect.CodeFailedPrecondition, errors.New("the reroll question was answered already"))
	}
	after := h
	after.RerollAnswerKey = &a.key
	if a.again {
		var err error
		if after, err = ae.rerolled(ctx, after); err != nil {
			return h, err
		}
	}
	if err := newHeldStores(ae.q).answerReroll(ctx, after); err != nil {
		return h, fmt.Errorf("answer the reroll question: %w", err)
	}
	return after, nil
}

// rerolled is the roll with the die that showed the low face rolled again, and the rule's uses spent.
//
// The server rolls the new die itself with its roller, whatever the campaign's dice mode: a player who rolls real dice
// (RN-18) is not asked to type the new face, and the hold keeps its Physical flag from the first roll. This reroll is
// therefore not RN-18 faithful for a physical-dice table; the answer carries no way of rolling to change that.
func (ae answering) rerolled(ctx context.Context, h heldRoll) (heldRoll, error) {
	if len(h.Faces) == 0 {
		return h, connect.NewError(connect.CodeInternal, errors.New("a reroll of a roll with no faces"))
	}
	index := min(max(int(derefInt32(h.RerollIndex)), 0), len(h.Faces)-1)
	face, err := ae.s.appD20()
	if err != nil {
		return h, err
	}
	if h.RerollResource != "" {
		if err := ae.s.spendUses(ctx, ae.gateEnv, ae.spec.Roller, h.RerollResource, h.RerollCost); err != nil {
			return h, err
		}
	}
	faces := slices.Clone(h.Faces)
	from := clamp32(faces[index], 1, d20Sides)
	faces[index] = face
	roll := d20Roll{Faces: faces, Index: countedAfter(h.Faces, h.Counted, faces, ae.spec.Mode), Physical: h.Physical}
	roll.Total = roll.Face() + ae.spec.Modifier
	roll = reliableD20(roll, ae.spec.Modifier, ae.spec.Reliable)
	h.Faces, h.Counted, h.Total = faces, roll.Index, roll.Total
	h.RerollFrom, h.RerollTo = new(from), new(clamp32(face, 1, d20Sides))
	return h, nil
}

// rerollReplyTo is replyTo for the reroll question: it must have been asked, and an answer already given is answered
// again, never twice with another key.
func (h heldRoll) rerollReplyTo(a heldReply) (heldReply, answerState, error) {
	switch {
	case !h.RerollAsk:
		return a, answerOpen, connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll asks no reroll question"))
	case h.AnswerKey != nil:
		// Resolved already: the same answer again gives the roll as it was resolved, under the key it was resolved with.
		if *h.AnswerKey != a.key && deref(h.RerollAnswerKey) != a.key {
			return a, answerOpen, errAnsweredAlready()
		}
		return heldReply{holdID: h.ID, key: *h.AnswerKey, question: askDie}, answerSettled, nil
	case h.RerollAnswerKey != nil:
		// Answered, and held for a question that follows: the same answer again reads the roll as it is.
		if *h.RerollAnswerKey != a.key {
			return a, answerOpen, connect.NewError(connect.CodeFailedPrecondition, errors.New("the reroll question was answered already"))
		}
		return a, answerRepeated, nil
	}
	return a, answerOpen, nil
}

// gateRerollAnswer applies the reroll answer: the new d20 is written in the roll first; a roller that holds a die is asked
// about it next, on the new roll, else the roll settles (Indomitable's and Stroke of Luck's questions are looked at on
// the new roll).
func (s *Service) gateRerollAnswer(ctx context.Context, ae answering, h heldRoll, a heldReply) (gateResult, error) {
	h, err := (rerollQuestion{}).apply(ctx, ae, h, a)
	if err != nil {
		return gateResult{}, err
	}
	if !(dieQuestion{}).pending(h, ae.st) {
		return s.gateSettle(ctx, ae, h, a)
	}
	if err := newHeldStores(ae.q).asked(ctx, ae.gateEnv, h); err != nil {
		return gateResult{}, err
	}
	return gateResult{Held: new(askedBy(h, ae.st))}, nil
}

// rerollOffer is the reroll question on the wire.
func (o heldOffer) rerollOffer() *playv1.RerollQuestion {
	h := o.Hold
	return &playv1.RerollQuestion{
		RerollIndex: derefInt32(h.RerollIndex), AtMost: derefInt32(h.RerollAtMost), Resource: h.RerollResource, Cost: h.RerollCost, SourceKey: h.RerollSource,
	}
}
