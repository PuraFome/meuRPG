package play

import (
	"context"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What a feature that declares a use asks of the combat (docs/architecture/play/feature-state.md, "Using a feature"). The
// checks live here, once: UseFeature runs them inside its transaction and GetTurnOptions runs them for the options it
// answers, so that what the options call enabled is what the call accepts.

// useRulesOf is the use of a feature as the rules read it, for the seam ActionUse.Left.
func useRulesOf(u link.FeatureUse) *rules.ActionUse {
	return &rules.ActionUse{Scope: u.Scope, Uses: u.Uses}
}

// featureOfKey is the feature action of the sheet with the key that declares a use.
func featureOfKey(sheet link.Sheet, key string) (link.FeatureAction, bool) {
	i := slices.IndexFunc(sheet.FeatureActions, func(a link.FeatureAction) bool { return a.Key == key && a.Use != nil })
	if i < 0 {
		return link.FeatureAction{}, false
	}
	return sheet.FeatureActions[i], true
}

// featureEconomy is the economy the option of a feature action spends.
func featureEconomy(row *rulesv1.ActionOption) rulesv1.ActionEconomy {
	return row.GetAction().GetEconomy()
}

// isReaction says the feature spends the reaction: it may be used on another creature's turn too.
func isReaction(row *rulesv1.ActionOption) bool {
	return featureEconomy(row) == rulesv1.ActionEconomy_ACTION_ECONOMY_REACTION
}

// featureGateCode says why the combatant cannot use a feature now, UNSPECIFIED when it can: the combat is not running, the
// combatant is out of the fight or down, it cannot act (an effect, a condition) or is surprised, and, for a feature that
// does not spend the reaction, that it is not its turn. A feature that spends the reaction is used on any turn, by a
// combatant that may react: the same rules the engine's other reactions follow (a reaction that an effect takes away
// answers REACTION_USED).
func (s *Service) featureGateCode(ctx context.Context, tx pgx.Tx, campaignID string, enc playdb.Encounter, who playdb.Combatant, reaction bool) (rulesv1.DisabledReasonCode, error) {
	code, err := s.gateFor(ctx, tx, campaignID, enc, who, reaction)
	if err != nil || code != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		return code, err
	}
	if reaction && who.EffectNoReaction {
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_USED, nil
	}
	return code, nil
}

// featureScene is the combat as a feature's checks read it: the combatants, who uses the feature and what it has spent.
type featureScene struct {
	enc  playdb.Encounter
	cs   []playdb.Combatant
	who  playdb.Combatant
	v    combatViewer
	down map[string]bool
	now  stateNow
	st   featureState
}

// featureSceneOf reads the scene; tx is the open transaction, or nil for a read.
func (s *Service) featureSceneOf(ctx context.Context, tx pgx.Tx, campaignID string, enc playdb.Encounter, cs []playdb.Combatant, who playdb.Combatant, v combatViewer) (featureScene, error) {
	sc := featureScene{enc: enc, cs: cs, who: who, v: v, down: map[string]bool{}, now: stateNowOf(enc), st: readFeatureState(who.FeatureState)}
	ids, err := s.downCharacters(ctx, tx, campaignID, cs)
	if err != nil {
		return sc, err
	}
	for _, id := range ids {
		sc.down[id] = true
	}
	return sc, nil
}

// spent is how many uses of the feature the combatant spent in the scope that holds now.
func (sc featureScene) spent(key string, u link.FeatureUse) int {
	e, ok := sc.st.live(useStateKey(key), sc.now)
	if !ok || e.S != u.Scope {
		return 0
	}
	return int(e.N)
}

// left is the uses the scope still allows, -1 for a feature with no limit in a scope.
func (sc featureScene) left(key string, u link.FeatureUse) int {
	return useRulesOf(u).Left(sc.spent(key, u))
}

// mark is the mark of the scope the use is written under now: the turn's key, the round, or none for the combat.
func (sc featureScene) mark(scope string) string {
	switch scope {
	case scopeTurn:
		return sc.now.Turn
	case scopeRound:
		return strconv.Itoa(int(sc.now.Round))
	}
	return ""
}

// targetRules is what a feature asks of its targets, as the rules read it.
func targetRules(u link.FeatureUse) rules.FeatureTargets {
	return rules.FeatureTargets{Min: u.TargetMin, Max: u.TargetMax, Who: u.TargetWho, RangeFt: u.TargetRangeFt}
}

// candidates lists the creatures the caller sees as targets of the feature, in turn order, each with the reason it cannot
// be picked ("" when it can). A creature the caller cannot see is not listed (RN-10), and neither is one that is out of the
// fight. The distance is the squares between the centres, 5 ft each (RN-21); the master and a combat with no grid are not
// held to the reach.
func (sc featureScene) candidates(u link.FeatureUse) []*playv1.ResourceTarget {
	t := targetRules(u)
	heals := u.Heal != nil
	ignoreRange := sc.v.master || isTheatre(sc.enc)
	onlyAlly := !slices.ContainsFunc(sc.cs, func(c playdb.Combatant) bool {
		return c.ID != sc.who.ID && c.Side == sc.who.Side && !c.Defeated && !c.Dismissed && sc.v.sees(c)
	})
	var out []*playv1.ResourceTarget
	for _, c := range sc.cs {
		self := c.ID == sc.who.ID
		if c.Dismissed || c.Defeated || c.DeathFailures >= deathSaveCount || (!self && !sc.v.sees(c)) {
			continue
		}
		cand := rules.FeatureCandidate{Self: self, SameSide: c.Side == sc.who.Side, OnlyAlly: onlyAlly, Down: sc.down[c.ID], IgnoreRange: ignoreRange}
		target := &playv1.TargetInReach{CombatantId: c.ID, Label: c.Label, State: stateOf(c)}
		switch dist, ok := distanceFt(sc.who, c); {
		case self:
			cand.OnMap = true
			target.DistanceFt = new(int32)
		case ok:
			cand.OnMap, cand.Distance = true, int(dist)
			target.DistanceFt = &dist
			target.TooFar = t.RangeFt > 0 && !ignoreRange && int(dist) > t.RangeFt
		}
		out = append(out, &playv1.ResourceTarget{Target: target, DisabledReasonPt: t.Refusal(cand, heals)})
	}
	return out
}

// pickable is the candidates the feature can be aimed at now.
func pickable(list []*playv1.ResourceTarget) []*playv1.ResourceTarget {
	return slices.DeleteFunc(slices.Clone(list), func(t *playv1.ResourceTarget) bool { return t.GetDisabledReasonPt() != "" })
}

// noValidTargetText is the reason of a feature that picks creatures when none can be picked now.
const noValidTargetText = "Sem alvo válido."

// reason is why the feature cannot be used now, in the order the use checks it after the gate: the economy and the
// feature's own resource (the row the rules worked out), the uses of its scope, the targets. nil when it can.
//
// The master has the last word on the economy (TakeAction lets him use an action the turn spent): with master set, a row
// disabled only by the economy goes on to the uses and the targets, which stop him too.
func (sc featureScene) reason(fa link.FeatureAction, row *rulesv1.ActionOption, master bool) *rulesv1.DisabledReason {
	if !row.GetEnabled() && !(master && masterMayOverride(row.GetReason().GetCode())) {
		return row.GetReason()
	}
	u := *fa.Use
	if u.Scope != "" && sc.left(fa.Key, u) == 0 {
		return &rulesv1.DisabledReason{Code: rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_USES}
	}
	if t := targetRules(u); t.Aimed() && t.Who != rules.FeatureWhoSelf && len(pickable(sc.candidates(u))) == 0 {
		return &rulesv1.DisabledReason{Code: rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_VALID_TARGET, TextPt: noValidTargetText}
	}
	return nil
}

// masterMayOverride says the reason is one of the economy: the master uses the feature anyway.
func masterMayOverride(code rulesv1.DisabledReasonCode) bool {
	switch code {
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED, rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_BONUS_ACTION_USED,
		rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_USED, rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ALREADY_USED_THIS_TURN,
		rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ATTACK_ACTION_FIRST:
		return true
	}
	return false
}
