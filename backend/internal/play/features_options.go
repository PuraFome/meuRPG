package play

import (
	"context"
	"slices"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// featureRowsOf copies the feature actions taken with UseFeature, as the rules worked them out.
func featureRowsOf(o *rulesv1.TurnOptions) []*rulesv1.ActionOption {
	var out []*rulesv1.ActionOption
	for _, a := range o.GetFeatureActions() {
		if a.GetViaUseFeature() {
			out = append(out, proto.CloneOf(a))
		}
	}
	return out
}

// annotateFeatures finishes the rows of the features taken with UseFeature, with the checks UseFeature makes
// (featureStopNow and featureScene.reason), in its order: the turn (a feature that spends the reaction is used on any turn, so the row the gate
// disabled for another creature's turn is taken back from the rules' own), the economy and the resource, the uses of the
// scope, the targets. It also gives each row the uses of its scope (use_total, use_left) and the total of its rest resource,
// and lists who each row that picks creatures can pick. The feature state is never sent (RN-10: it is the combatant's).
func (s *Service) annotateFeatures(ctx context.Context, m authz.Membership, d *encounterData, v combatViewer, who playdb.Combatant, reads *turnReads, opts *rulesv1.TurnOptions, _ rulesv1.DisabledReasonCode) ([]*playv1.ResourceTargets, error) {
	if len(reads.features) == 0 {
		return nil, nil
	}
	sheet, err := reads.sheetOf(ctx)
	if err != nil {
		return nil, s.dbError(ctx, "read the character's features", err)
	}
	scene, err := s.featureSceneOf(ctx, nil, m.CampaignID, d.enc, d.cs, who, v)
	if err != nil {
		return nil, s.dbError(ctx, "work out the features' targets", err)
	}
	vit, err := reads.vitalsOf(ctx)
	if err != nil {
		return nil, s.dbError(ctx, "read the character's resources", err)
	}
	// The stop (featureStopNow) is the one UseFeature asks, read here with the viewer's own standing; one for each kind of row.
	read, err := s.openTx(ctx, combatTx{q: s.queries, session: playdb.GameSession{CampaignID: m.CampaignID}, enc: d.enc, master: v.master, svc: s})
	if err != nil {
		return nil, s.dbError(ctx, "read the table's rules", err)
	}
	a := &featureAnnotator{s: s, who: who, reads: reads, sheet: sheet, scene: scene, vit: vit, read: read, stops: map[bool]*featureStop{}}
	var lists []*playv1.ResourceTargets
	for _, row := range opts.GetFeatureActions() {
		if !row.GetViaUseFeature() {
			continue
		}
		list, err := a.annotate(ctx, row)
		if err != nil {
			return nil, err
		}
		if list != nil {
			lists = append(lists, list)
		}
	}
	return lists, nil
}

// featureAnnotator is what annotateFeatures reads once and every row of a feature taken with UseFeature is finished with.
type featureAnnotator struct {
	s     *Service
	who   playdb.Combatant
	reads *turnReads
	sheet link.Sheet
	scene featureScene
	vit   *playv1.CharacterVitals
	read  *combatTx
	stops map[bool]*featureStop
}

// stopOf is the stop (featureStopNow) of a kind of row, asked once for each kind.
func (a *featureAnnotator) stopOf(ctx context.Context, reaction bool) (*featureStop, error) {
	if st, ok := a.stops[reaction]; ok {
		return st, nil
	}
	st, err := a.s.featureStopNow(ctx, a.read, a.who, reaction)
	a.stops[reaction] = st
	return st, err
}

// annotate finishes one row: the turn, the wait, the uses of the scope, the resource's total and the targets. It returns the
// creatures the row can pick, nil for a row that picks none (or that the sheet does not declare).
func (a *featureAnnotator) annotate(ctx context.Context, row *rulesv1.ActionOption) (*playv1.ResourceTargets, error) {
	fa, ok := featureOfKey(a.sheet, row.GetAction().GetKey())
	if !ok {
		return nil, nil
	}
	pristine := a.reads.features[slices.IndexFunc(a.reads.features, func(r *rulesv1.ActionOption) bool { return r.GetAction().GetKey() == fa.Key })]
	reaction := isReaction(pristine)
	if reaction {
		// The rules' own row, taken back: the gate disabled it for another creature's turn.
		row.Enabled, row.Reason = pristine.GetEnabled(), pristine.GetReason()
	}
	stop, err := a.stopOf(ctx, reaction)
	if err != nil {
		return nil, a.s.dbError(ctx, "check what holds the feature", err)
	}
	if stop != nil {
		row.Enabled, row.Reason = false, stop.reason()
	}
	u := *fa.Use
	a.fillTotals(row, fa, u)
	if row.GetEnabled() {
		if r := a.scene.reason(fa, row, false); r != nil {
			row.Enabled, row.Reason = false, r
		}
	}
	if targetRules(u).Aimed() {
		return &playv1.ResourceTargets{ActionKey: fa.Key, Targets: a.scene.candidates(u)}, nil
	}
	return nil, nil
}

// fillTotals gives the row the uses of its scope and the total of its rest resource.
func (a *featureAnnotator) fillTotals(row *rulesv1.ActionOption, fa link.FeatureAction, u link.FeatureUse) {
	if u.Scope != "" {
		row.UseTotal = clamp32(u.Uses, 0, 1<<30)
		row.UseLeft = clamp32(max(a.scene.left(fa.Key, u), 0), 0, 1<<30)
	}
	if fa.Resource == "" {
		return
	}
	for _, r := range a.vit.GetResources() {
		if r.GetKey() == fa.Resource {
			row.ResourceTotal = r.GetTotal()
		}
	}
}
