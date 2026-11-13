package play

import (
	"context"
	"slices"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// featureRowsOf copies the feature actions taken with UseFeature, as the rules worked them out.
func featureRowsOf(o *rulesv1.TurnOptions) []*rulesv1.ActionOption {
	var out []*rulesv1.ActionOption
	for _, a := range o.GetFeatureActions() {
		if a.GetViaUseFeature() {
			out = append(out, proto.Clone(a).(*rulesv1.ActionOption)) //nolint:forcetypeassert // Clone of an ActionOption
		}
	}
	return out
}

// annotateFeatures finishes the rows of the features taken with UseFeature, with the checks UseFeature makes
// (featureStopNow and featureScene.reason), in its order: the turn (a feature that spends the reaction is used on any turn, so the row the gate
// disabled for another creature's turn is taken back from the rules' own), the economy and the resource, the uses of the
// scope, the targets. It also gives each row the uses of its scope (use_total, use_left) and the total of its rest resource,
// and lists who each row that picks creatures can pick. The feature state is never sent (RN-10: it is the combatant's).
func (s *Service) annotateFeatures(ctx context.Context, m authz.Membership, d *encounterData, v combatViewer, who playdb.Combatant, reads *turnReads, opts *rulesv1.TurnOptions, gateCode rulesv1.DisabledReasonCode) ([]*playv1.ResourceTargets, error) {
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
	stops := map[bool]*featureStop{}
	stopOf := func(reaction bool) (*featureStop, error) {
		if st, ok := stops[reaction]; ok {
			return st, nil
		}
		st, err := s.featureStopNow(ctx, read, who, reaction)
		stops[reaction] = st
		return st, err
	}
	var lists []*playv1.ResourceTargets
	for _, row := range opts.GetFeatureActions() {
		if !row.GetViaUseFeature() {
			continue
		}
		fa, ok := featureOfKey(sheet, row.GetAction().GetKey())
		if !ok {
			continue
		}
		pristine := reads.features[slices.IndexFunc(reads.features, func(r *rulesv1.ActionOption) bool { return r.GetAction().GetKey() == fa.Key })]
		reaction := isReaction(pristine)
		if reaction {
			// The rules' own row, taken back: the gate disabled it for another creature's turn.
			row.Enabled, row.Reason = pristine.GetEnabled(), pristine.GetReason()
		}
		stop, err := stopOf(reaction)
		if err != nil {
			return nil, s.dbError(ctx, "check what holds the feature", err)
		}
		if stop != nil {
			row.Enabled, row.Reason = false, stop.reason()
		}
		u := *fa.Use
		if u.Scope != "" {
			row.UseTotal = clamp32(u.Uses, 0, 1<<30)
			row.UseLeft = clamp32(max(scene.left(fa.Key, u), 0), 0, 1<<30)
		}
		if fa.Resource != "" {
			for _, r := range vit.GetResources() {
				if r.GetKey() == fa.Resource {
					row.ResourceTotal = r.GetTotal()
				}
			}
		}
		if row.GetEnabled() {
			if r := scene.reason(fa, row, false); r != nil {
				row.Enabled, row.Reason = false, r
			}
		}
		if t := targetRules(u); t.Aimed() {
			lists = append(lists, &playv1.ResourceTargets{ActionKey: fa.Key, Targets: scene.candidates(u)})
		}
	}
	return lists, nil
}
