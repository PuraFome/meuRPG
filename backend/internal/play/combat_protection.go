package play

import (
	"context"
	"slices"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The Protection fighting style (SRD 5.1, Fighter): when a creature the shield-bearer sees
// attacks a target other than the bearer that is within 5 ft of it, the bearer may spend its
// reaction to give that attack roll disadvantage. The attack waits for the answer before its
// d20 is rolled, like one Cutting Words can cut (holdAttackRoll), and the replay rolls it with
// the disadvantage.

// protectionWindows lists the Protection windows an attack opens: each shield-bearer that has the
// style, can react, sees the attacker and stands within 5 ft of the target, which is not the bearer
// itself. The target must be on the bearer's side: the style shields a neighbor, and a bearer is never
// asked to put disadvantage on the attack of its own side against an enemy. A map is needed to know
// the 5 ft, so a combat without one opens none.
func (s *Service) protectionWindows(ctx context.Context, c *combatTx, cs []playdb.Combatant, attacker, target playdb.Combatant, key string) ([]windowSpec, error) {
	var specs []windowSpec
	for _, r := range cs {
		if r.ID == attacker.ID || r.ID == target.ID || r.Defeated || r.Side != target.Side || r.Side == attacker.Side {
			continue
		}
		if d, placed := distanceFt(r, target); !placed || d > reaction.ProtectionReachFt {
			continue
		}
		ok, err := s.canProtect(ctx, c, cs, r, attacker)
		if err != nil {
			return nil, err
		}
		if ok {
			specs = append(specs, windowSpec{kind: reaction.ProtectionKind, reactor: &r, trigger: windowTrigger{Actor: attacker.ID, Target: target.ID, Key: key, Roll: "attack"}})
		}
	}
	return specs, nil
}

// canProtect says the combatant has the style with a shield in hand, can react now and sees the attacker
// (any distance: the style has no range of its own). A hidden attacker is seen by no player (RN-10), so it
// opens no window that would tell a player there is one.
func (s *Service) canProtect(ctx context.Context, c *combatTx, cs []playdb.Combatant, r, attacker playdb.Combatant) (bool, error) {
	k, ok, err := s.kitOf(ctx, c.tx, c.session.CampaignID, r)
	if err != nil || !ok || !k.st.Protection || !k.canReact() {
		return false, err
	}
	if eyesCannotSee(r, attacker) {
		return false, nil
	}
	return s.fogSees(ctx, c, cs, r, attacker)
}

// answerProtection: the held attack roll gets disadvantage for the reaction (SRD, Fighter). The answer
// spends the reaction and the replay rolls the attack; the other Protection windows of the same attack
// close, since one disadvantage is all the roll takes.
func (s *Service) answerProtection(ctx context.Context, c *combatTx, w playdb.ReactionWindow, reactor playdb.Combatant, use bool, got *answered) error {
	if !use {
		return s.passWindow(ctx, c, w, reactor, got)
	}
	k, err := s.reactorKit(ctx, c, reactor)
	if err != nil {
		return err
	}
	if !k.st.Protection {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES, "the Protection style needs a shield in hand")
	}
	if err := s.spendReaction(ctx, c, reactor, reaction.ProtectionKind); err != nil {
		return err
	}
	if _, err := s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, windowOutcome{ByMaster: c.master, Used: true}, reaction.ProtectionKind); err != nil {
		return err
	}
	if err := s.closeOtherProtections(ctx, c, w); err != nil {
		return err
	}
	c.lastAnswered = reaction.ProtectionKind
	t := windowTriggerOf(w)
	got.ev = actionEvent{
		Round: c.enc.Round, Secret: reactor.Hidden, Actor: reactor.ID, Target: t.Actor, ReactionBefore: reactor.ReactionUsed,
		Reaction: &reactionEvent{Kind: w.Kind, Window: w.ID, Used: true, ByMaster: c.master},
	}
	got.result = &playv1.ReactionResult{
		Kind: playv1.ReactionKind_REACTION_KIND_PROTECTION, ByMaster: c.master, Used: true,
		Result: &playv1.ReactionResult_Protection{Protection: &playv1.ProtectionResult{}},
	}
	c.characterID = &reactor.CharacterID
	return nil
}

// closeOtherProtections closes the Protection windows still open on the attack the window w holds.
func (s *Service) closeOtherProtections(ctx context.Context, c *combatTx, w playdb.ReactionWindow) error {
	group, err := c.q.ListReactionWindowsOfGroup(ctx, playdb.ListReactionWindowsOfGroupParams{EncounterID: c.enc.ID, GroupID: w.GroupID})
	if err != nil {
		return err
	}
	for _, o := range group {
		if o.ID == w.ID || o.Kind != w.Kind || o.Status != windowOpen {
			continue
		}
		if _, err := s.closeWindow(ctx, c, o, windowClosed, reaction.ReasonTriggerGone, windowOutcome{}, ""); err != nil {
			return err
		}
	}
	return nil
}

// protectionMode is the mode of a replayed attack after the Protection that was used: the source joins
// the others and the mode is worked out again. The sentence names the bearer to whoever sees it, and is
// "Outra fonte" to a player who does not (RN-10).
func (r modeResult) protectionMode(bearer playdb.Combatant, v combatViewer) modeResult {
	r.Sources = append(slices.Clone(r.Sources), combat.Source{Kind: combat.SourceProtection, Effect: combat.ModeDisadvantage})
	r.Mode = combat.Resolve(r.Sources)
	shown := &playv1.AdvantageSource{
		Kind: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PROTECTION, Effect: playv1.RollMode_ROLL_MODE_DISADVANTAGE,
		TextPt: "Proteção de " + bearer.Label + ": desvantagem",
	}
	if !v.master && !v.sees(bearer) {
		shown = &playv1.AdvantageSource{
			Kind: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_OTHER_SOURCE, Effect: playv1.RollMode_ROLL_MODE_DISADVANTAGE, TextPt: "Outra fonte: desvantagem",
		}
	}
	r.Shown = append(slices.Clone(r.Shown), shown)
	return r
}

// applyProtection gives the replayed attack the disadvantage of a Protection that was used. The request
// was made under the mode the screen showed before the reaction, so a requested mode equal to it is not a
// choice and is dropped; the dice the caller typed are made as many as the new mode takes (the extra die
// is rolled in the app).
func (f *attackFlow) applyProtection() error {
	if f.c.replay == nil || len(f.c.replay.protectors) == 0 {
		return nil
	}
	i := slices.IndexFunc(f.cs, func(x playdb.Combatant) bool { return x.ID == f.c.replay.protectors[0] })
	if i < 0 {
		return nil
	}
	before := f.truth.Mode
	f.truth = f.truth.protectionMode(f.cs[i], f.v)
	if asked, ok := modeFromProto(f.req.GetRollMode()); ok && asked == before && f.req.GetRollModeRequestId() == "" {
		f.req = withoutRollMode(f.req)
	}
	if f.req.GetRollMode() != playv1.RollMode_ROLL_MODE_UNSPECIFIED || f.req.GetRollModeRequestId() != "" {
		return nil // the caller or the master chose the mode: its dice are the caller's
	}
	var err error
	f.call.in, err = f.s.diceForMode(f.call.in, f.truth.Mode)
	return err
}

// withoutRollMode is the request with no requested mode.
func withoutRollMode(req *playv1.RollAttackRequest) *playv1.RollAttackRequest {
	out, _ := proto.Clone(req).(*playv1.RollAttackRequest)
	out.RollMode, out.ModeReason = playv1.RollMode_ROLL_MODE_UNSPECIFIED, ""
	return out
}

// diceForMode makes the typed d20 of a roll as many as the mode takes: the caller's first dice are kept
// and a missing one is rolled in the app. A roll in the app needs nothing.
func (s *Service) diceForMode(in rollInput, mode combat.RollMode) (rollInput, error) {
	if in.inApp {
		return in, nil
	}
	faces := in.typedFaces
	if len(faces) == 0 {
		faces = []int{in.typed}
	}
	n := mode.Dice()
	if len(faces) > n {
		faces = faces[:n]
	}
	if missing := n - len(faces); missing > 0 {
		res, err := dice.Roll(s.roller, dice.Expr{Count: missing, Sides: d20Sides})
		if err != nil {
			return in, err
		}
		faces = append(slices.Clone(faces), res.Faces...)
	}
	in.typed, in.typedFaces = 0, faces
	return in, nil
}

// protectedBy is who protected the attack being replayed, for its event.
func (f *attackFlow) protectedBy() []string {
	if f.c.replay == nil {
		return nil
	}
	return f.c.replay.protectors
}

// protectionBack gives back the reaction the Protection of an undone attack spent.
func (u *undoer) protectionBack(ctx context.Context, ev actionEvent) error {
	for _, id := range ev.ProtectedBy {
		if who, ok := u.find(id); ok {
			if err := u.setEconomy(ctx, who, who.ActionUsed, who.BonusActionUsed, false, who.Dashed); err != nil {
				return err
			}
		}
	}
	return nil
}
