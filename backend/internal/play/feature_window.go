package play

import (
	"context"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The feature window (docs/architecture/play/reactions.md "The feature window"): a reaction feature whose use declares a
// trigger (rules.FeatureTrigger) opens a window of kind FEATURE for its user's player when an enemy starts its turn or moves
// within the trigger's range. The window is opened where the readied and the opportunity windows open (the start of a turn,
// the end of a move) and answered with AnswerReaction (feature_window_answer.go). A hidden creature's trigger opens the window
// for the master alone (RN-10).

// featureMoment is one happening a feature may be waiting for: the creature that did it and, for a move, where it stood before.
type featureMoment struct {
	kind  string
	mover playdb.Combatant  // as it stands now
	from  *playdb.Combatant // a move: as it stood before
}

// within is how far the creature is from the reactor, and whether the trigger fires for the range: the creature is within it
// and, for a move, was not before (a creature already near that moves near does not trigger it again).
func (m featureMoment) within(reactor playdb.Combatant, rangeFt int) (int32, bool) {
	dist, ok := distanceFt(reactor, m.mover)
	if !ok || int(dist) > rangeFt {
		return dist, false
	}
	if m.kind == rules.FeatureTriggerMoves && m.from != nil {
		if before, ok := distanceFt(reactor, *m.from); ok && int(before) <= rangeFt {
			return dist, false
		}
	}
	return dist, true
}

// standing is the combatants with the creature where it stands now (a move's list was read before it).
func (m featureMoment) standing(cs []playdb.Combatant) []playdb.Combatant {
	out := slices.Clone(cs)
	if i := slices.IndexFunc(out, func(c playdb.Combatant) bool { return c.ID == m.mover.ID }); i >= 0 {
		out[i] = m.mover
	}
	return out
}

// mayWatch says the combatant may be waiting for the creature: a player's character of the other side, in the fight and on the map.
func (m featureMoment) mayWatch(r playdb.Combatant) bool {
	return r.Kind == kindPlayer && r.ID != m.mover.ID && r.Side != m.mover.Side && !r.Defeated && !r.Dismissed && !r.ReactionUsed && placed(r) && placed(m.mover)
}

// openFeatureWindows opens the windows of the features the moment triggers, one for each feature of each reactor, in the
// initiative order and in one group. It answers the ids of the windows it opened.
func (s *Service) openFeatureWindows(ctx context.Context, c *combatTx, cs []playdb.Combatant, m featureMoment) ([]string, error) {
	if m.mover.Defeated || m.mover.Dismissed {
		return nil, nil
	}
	cs = m.standing(cs)
	var specs []windowSpec
	for _, r := range cs {
		if !m.mayWatch(r) {
			continue
		}
		more, err := s.featureSpecs(ctx, c, cs, r, m)
		if err != nil {
			return nil, err
		}
		specs = append(specs, more...)
	}
	if len(specs) == 0 {
		return nil, nil
	}
	ws, err := s.openWindows(ctx, c, newGroup(), nil, specs)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(ws))
	for i, w := range ws {
		ids[i] = w.ID
	}
	return ids, nil
}

// triggeredBy are the features of the sheet that wait for the moment.
func triggeredBy(sheet link.Sheet, kind string) []link.FeatureAction {
	return slices.DeleteFunc(slices.Clone(sheet.FeatureActions), func(fa link.FeatureAction) bool {
		return fa.Use == nil || fa.Use.Trigger == nil || !slices.Contains(fa.Use.Trigger.Kinds, kind)
	})
}

// featureSpecs are the windows the moment opens for one reactor.
func (s *Service) featureSpecs(ctx context.Context, c *combatTx, cs []playdb.Combatant, r playdb.Combatant, m featureMoment) ([]windowSpec, error) {
	sheet, err := s.sheetOf(ctx, c.tx, c.session.CampaignID, r)
	if err != nil {
		return nil, err
	}
	waiting := triggeredBy(sheet, m.kind)
	if len(waiting) == 0 {
		return nil, nil
	}
	k, ok, err := s.kitOf(ctx, c.tx, c.session.CampaignID, r)
	if err != nil || !ok || !k.canReact() {
		return nil, err
	}
	sees, err := s.watches(ctx, c, cs, r, m.mover)
	if err != nil || !sees {
		return nil, err
	}
	var specs []windowSpec
	for _, fa := range waiting {
		dist, near := m.within(r, fa.Use.Trigger.RangeFt)
		if !near || !s.featureReady(ctx, c, cs, k, fa, m.mover, dist) {
			continue
		}
		specs = append(specs, windowSpec{
			kind: reaction.Feature, reactor: &r,
			trigger: windowTrigger{Actor: m.mover.ID, Key: fa.Key, Distance: dist, Moment: m.kind, Hidden: m.mover.Hidden},
		})
	}
	return specs, nil
}

// watches says the reactor notices the creature: a blind reactor notices nothing, an invisible creature is noticed by no
// one, and on a map with the fog each reactor sees with its own eyes. A hidden creature is noticed by the master alone: the
// window opens for him whatever the reactor sees (RN-10).
func (s *Service) watches(ctx context.Context, c *combatTx, cs []playdb.Combatant, r, mover playdb.Combatant) (bool, error) {
	if slices.Contains(r.Conditions, "condition:blinded") {
		return false, nil
	}
	if mover.Hidden {
		return true, nil
	}
	if eyesCannotSee(r, mover) {
		return false, nil
	}
	return s.fogSees(ctx, c, cs, r, mover)
}

// featureReady says the feature can be used now, as the options of its user say, and can be aimed at the creature.
func (s *Service) featureReady(ctx context.Context, c *combatTx, cs []playdb.Combatant, k kit, fa link.FeatureAction, mover playdb.Combatant, dist int32) bool {
	i := slices.IndexFunc(k.opts.GetFeatureActions(), func(o *rulesv1.ActionOption) bool { return o.GetAction().GetKey() == fa.Key })
	if i < 0 || !k.opts.GetFeatureActions()[i].GetViaUseFeature() {
		return false
	}
	sc, err := s.featureSceneOf(ctx, c.tx, c.session.CampaignID, c.enc, cs, k.who, c.featureViewer(k.who, mover, cs))
	if err != nil || sc.reason(fa, k.opts.GetFeatureActions()[i], false) != nil {
		return false
	}
	return sc.canAimAt(*fa.Use, mover, dist)
}

// featureGone says the feature of a FEATURE window cannot be used on the creature that triggered it any more: the creature left
// the fight or joined the reactor's side, or the feature is not ready now (no use or resource left, the reaction spent, nothing to aim at).
func (s *Service) featureGone(ctx context.Context, c *combatTx, w playdb.ReactionWindow, t windowTrigger, byID map[string]playdb.Combatant) (bool, error) {
	mover, ok := byID[t.Actor]
	reactor, ok2 := byID[conv.OrZero(w.ReactorID)]
	if !ok || !ok2 || mover.Defeated || mover.Dismissed || mover.Side == reactor.Side {
		return true, nil
	}
	sheet, err := s.sheetOf(ctx, c.tx, c.session.CampaignID, reactor)
	if err != nil {
		return false, err
	}
	fa, ok := featureOfKey(sheet, t.Key)
	if !ok {
		return true, nil
	}
	k, ok, err := s.kitOf(ctx, c.tx, c.session.CampaignID, reactor)
	if err != nil || !ok {
		return !ok && err == nil, err
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return false, err
	}
	dist, _ := distanceFt(reactor, mover)
	return !s.featureReady(ctx, c, cs, k, fa, mover, dist), nil
}

// featureViewer is who reads the targets of the feature: its user's player, and the master when the creature is hidden from them.
func (c *combatTx) featureViewer(user, mover playdb.Combatant, cs []playdb.Combatant) combatViewer {
	if mover.Hidden || user.UserID == nil {
		return c.viewer(authz.Membership{CampaignID: c.session.CampaignID, Role: authz.RoleMaster}, cs)
	}
	return c.viewer(authz.Membership{CampaignID: c.session.CampaignID, UserID: *user.UserID, Role: authz.RolePlayer}, cs)
}

// canAimAt says the feature can be aimed at the creature, within the reach it asks of its targets. A feature that is not
// aimed at a creature can always be used.
func (sc featureScene) canAimAt(u link.FeatureUse, who playdb.Combatant, dist int32) bool {
	t := targetRules(u)
	if !t.Aimed() || t.Who == rules.FeatureWhoSelf {
		return true
	}
	if t.RangeFt > 0 && int(dist) > t.RangeFt {
		return false
	}
	return slices.ContainsFunc(sc.candidates(u), func(c *playv1.ResourceTarget) bool {
		return c.GetTarget().GetCombatantId() == who.ID && c.GetDisabledReasonPt() == ""
	})
}

// featureTriggerKinds are the moments of a FEATURE window as the proto names them.
var featureTriggerKinds = map[string]rulesv1.FeatureTriggerKind{
	rules.FeatureTriggerStartsTurn: rulesv1.FeatureTriggerKind_FEATURE_TRIGGER_KIND_ENEMY_STARTS_TURN_WITHIN,
	rules.FeatureTriggerMoves:      rulesv1.FeatureTriggerKind_FEATURE_TRIGGER_KIND_ENEMY_MOVES_WITHIN,
}

// featureTrigger is what a FEATURE window tells whoever answers it: the creature, the feature, how far the creature stands
// and the moment. Only the master and, for a creature that is not hidden, the reactor's player read it.
func (wv *windowView) featureTrigger(w playdb.ReactionWindow, reactor *playdb.Combatant) *playv1.ReactionTrigger {
	t := windowTriggerOf(w)
	out := &playv1.ReactionTrigger{ActorId: t.Actor, ActionKey: t.Key, FeatureTrigger: featureTriggerKinds[t.Moment]}
	if a, ok := wv.byID[t.Actor]; ok {
		out.ActorLabel = a.Label
	}
	if t.Distance > 0 {
		out.DistanceFt = &t.Distance
	}
	if reactor != nil {
		if sheet, err := wv.s.sheetOf(wv.ctx, nil, wv.m.CampaignID, *reactor); err == nil {
			if fa, ok := featureOfKey(sheet, t.Key); ok {
				out.ActionNamePt = fa.Name
			}
		}
	}
	return out
}
