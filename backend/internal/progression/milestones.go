package progression

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/progression/progressiondb"
)

// The planned milestones (MR-016, RN-09, RN-12): in a MILESTONES
// campaign the master writes the milestones ahead of time (planned_milestones)
// and marks one reached when the party gets there.
//
//   - Reaching one is today's MarkMilestone with the milestone's text as its
//     reason, linked to the milestone (xp_awards.milestone_id): the same
//     award, the same "pode subir de nível" tag, the same undo.
//   - "Reached" is derived: at least one of the milestone's awards is not
//     undone. Undoing its only mark makes it planned again; a "Dar a mais
//     alguém" mark is one more award on the same milestone, so undoing it
//     (it is the last, as undo only takes the last) takes the milestone off
//     those characters and leaves it reached for the others.
//   - A reached milestone is never edited, moved or removed.
//   - A player reads only the reached ones (RN-20: the master's plans stay
//     the master's). Planned-list edits have no session event: the players
//     never see the list.

// The limits of the list, as progression.proto documents them. A milestone's
// text is as long as an award's reason, because it becomes one.
const (
	maxMilestones    = 100
	maxMilestoneText = maxReasonLength
)

// errMilestoneNotFound is the answer for an ID that is not a milestone of the
// campaign, valid or not.
func errMilestoneNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("milestone not found"))
}

// ListMilestones implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) ListMilestones(
	ctx context.Context,
	req *connect.Request[progressionv1.ListMilestonesRequest],
) (*connect.Response[progressionv1.ListMilestonesResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	views, err := s.milestoneViews(ctx, m)
	if err != nil {
		return nil, s.dbError(ctx, "list milestones", err)
	}
	return connect.NewResponse(&progressionv1.ListMilestonesResponse{Milestones: views}), nil
}

// AddMilestone implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) AddMilestone(
	ctx context.Context,
	req *connect.Request[progressionv1.AddMilestoneRequest],
) (*connect.Response[progressionv1.AddMilestoneResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	text, err := cleanMilestoneText(req.Msg.GetText())
	if err != nil {
		return nil, err
	}
	if err := s.requireMilestonesMode(ctx, nil, m.CampaignID); err != nil {
		return nil, err
	}
	key, err := idem.Clean(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	// The key is unique in the campaign, and kept with a hash of the whole request.
	scopedKey, requestHash := idem.Scope(m.CampaignID, key), addMilestoneHash(req.Msg, text)
	newMilestone := plannedInsert{campaignID: m.CampaignID, text: text, key: scopedKey, hash: requestHash}
	var added progressiondb.PlannedMilestone
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		added, err = s.addPlanned(ctx, tx, newMilestone)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "add a milestone", err)
	}
	views := s.listAfterWrite(ctx, m, "add a milestone")
	return connect.NewResponse(&progressionv1.AddMilestoneResponse{Milestone: writtenMilestone(views, added), Milestones: views}), nil
}

// addMilestoneHash is the hash AddMilestone keeps with its key: the request's,
// with the cleaned text the milestone is stored with, so a retry that differs
// only by the whitespace around the text is the same request.
func addMilestoneHash(req *progressionv1.AddMilestoneRequest, text string) *string {
	cleaned, ok := proto.Clone(req).(*progressionv1.AddMilestoneRequest)
	if !ok { // proto.Clone keeps the message's type; this never happens
		return idem.Hash(req)
	}
	cleaned.Text = text
	return idem.Hash(cleaned)
}

// plannedInsert is the milestone AddMilestone writes: its campaign, its text
// (already checked) and the request's scoped key and hash.
type plannedInsert struct {
	campaignID string
	text       string
	key, hash  *string
}

// addPlanned is AddMilestone's transaction: the mode asked again, the list
// locked, then the milestone of the key or a new one after the last.
func (s *Service) addPlanned(ctx context.Context, tx pgx.Tx, p plannedInsert) (progressiondb.PlannedMilestone, error) {
	q := s.queries.WithTx(tx)
	if err := s.requireMilestonesMode(ctx, tx, p.campaignID); err != nil {
		return progressiondb.PlannedMilestone{}, err
	}
	// The list is locked first, so two calls with the same key take turns.
	current, err := q.ListPlannedMilestonesForUpdate(ctx, p.campaignID)
	if err != nil {
		return progressiondb.PlannedMilestone{}, fmt.Errorf("list the milestones: %w", err)
	}
	added, _, err := idem.Create(ctx, p.key, p.hash, q.GetPlannedMilestoneByCreateKey,
		func(row progressiondb.PlannedMilestone) *string { return row.CreateHash },
		func() (progressiondb.PlannedMilestone, error) { return s.insertPlanned(ctx, q, p, current) })
	return added, err
}

// insertPlanned writes the milestone after the last of current, unless the
// list is full. It passes on pgx.ErrNoRows, the key another call took first,
// for idem.Create.
func (s *Service) insertPlanned(ctx context.Context, q *progressiondb.Queries, p plannedInsert, current []progressiondb.PlannedMilestone) (progressiondb.PlannedMilestone, error) {
	if len(current) >= maxMilestones {
		return progressiondb.PlannedMilestone{}, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("the campaign already has %d milestones", maxMilestones))
	}
	position := int32(0)
	if n := len(current); n > 0 {
		position = current[n-1].Position + 1
	}
	row, err := q.InsertPlannedMilestone(ctx, progressiondb.InsertPlannedMilestoneParams{
		CampaignID: p.campaignID, Position: position, Text: p.text, CreateKey: p.key, CreateHash: p.hash, Now: s.now(),
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return row, fmt.Errorf("insert a milestone: %w", err)
	}
	return row, err
}

// UpdateMilestone implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) UpdateMilestone(
	ctx context.Context,
	req *connect.Request[progressionv1.UpdateMilestoneRequest],
) (*connect.Response[progressionv1.UpdateMilestoneResponse], error) {
	m, id, err := s.milestoneCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMilestoneId())
	if err != nil {
		return nil, err
	}
	text, err := cleanMilestoneText(req.Msg.GetText())
	if err != nil {
		return nil, err
	}
	err = s.changePlanned(ctx, m, id, func(q *progressiondb.Queries) error {
		return q.UpdatePlannedMilestoneText(ctx, progressiondb.UpdatePlannedMilestoneTextParams{CampaignID: m.CampaignID, ID: id, Text: text, Now: s.now()})
	})
	if err != nil {
		return nil, s.dbError(ctx, "update a milestone", err)
	}
	views := s.listAfterWrite(ctx, m, "update a milestone")
	written := progressiondb.PlannedMilestone{ID: id, Text: text}
	return connect.NewResponse(&progressionv1.UpdateMilestoneResponse{Milestone: writtenMilestone(views, written), Milestones: views}), nil
}

// MoveMilestone implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) MoveMilestone(
	ctx context.Context,
	req *connect.Request[progressionv1.MoveMilestoneRequest],
) (*connect.Response[progressionv1.MoveMilestoneResponse], error) {
	m, id, err := s.milestoneCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMilestoneId())
	if err != nil {
		return nil, err
	}
	step, ok := directionSteps[req.Msg.GetDirection()]
	if !ok {
		return nil, invalidArgument("direction", errors.New("is required"))
	}
	err = s.changePlanned(ctx, m, id, func(q *progressiondb.Queries) error {
		return movePlanned(ctx, q, m.CampaignID, id, step)
	})
	if err != nil {
		return nil, s.dbError(ctx, "move a milestone", err)
	}
	return connect.NewResponse(&progressionv1.MoveMilestoneResponse{Milestones: s.listAfterWrite(ctx, m, "move a milestone")}), nil
}

// directionSteps is how far a move goes in the list, by direction: up is
// towards the first.
var directionSteps = map[progressionv1.MilestoneDirection]int{
	progressionv1.MilestoneDirection_MILESTONE_DIRECTION_UP:   -1,
	progressionv1.MilestoneDirection_MILESTONE_DIRECTION_DOWN: 1,
}

// movePlanned is MoveMilestone's change, inside changePlanned's transaction:
// the milestone swaps places with the nearest planned one in that direction,
// and the list is numbered again from 0.
func movePlanned(ctx context.Context, q *progressiondb.Queries, campaignID, id string, step int) error {
	list, err := q.ListPlannedMilestonesForUpdate(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the milestones: %w", err)
	}
	live, err := q.ListLiveMilestoneAwards(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the marks: %w", err)
	}
	// changePlanned locked this milestone in the same transaction, so the locked
	// list of its campaign holds it.
	i := slices.IndexFunc(list, func(p progressiondb.PlannedMilestone) bool { return p.ID == id })
	j := nearestPlanned(list, live, i, step)
	if j < 0 || j >= len(list) {
		return nil // already first or last: nothing changes
	}
	list[i], list[j] = list[j], list[i]
	return renumberPlanned(ctx, q, campaignID, list)
}

// nearestPlanned is the index of the nearest planned milestone from i in the
// direction of step: the reached ones (with a live mark) are skipped, and keep
// their places. It is out of the list when there is none.
func nearestPlanned(list []progressiondb.PlannedMilestone, live []progressiondb.XpAward, i, step int) int {
	reached := func(j int) bool {
		return slices.ContainsFunc(live, func(a progressiondb.XpAward) bool { return conv.OrZero(a.MilestoneID) == list[j].ID })
	}
	j := i + step
	for j >= 0 && j < len(list) && reached(j) {
		j += step
	}
	return j
}

// renumberPlanned writes each milestone's index in list as its position, where
// it is not that already.
func renumberPlanned(ctx context.Context, q *progressiondb.Queries, campaignID string, list []progressiondb.PlannedMilestone) error {
	for p := range list {
		want := int32(p)
		if list[p].Position == want {
			continue
		}
		if err := q.SetPlannedMilestonePosition(ctx, progressiondb.SetPlannedMilestonePositionParams{CampaignID: campaignID, ID: list[p].ID, Position: want}); err != nil {
			return fmt.Errorf("set a milestone's position: %w", err)
		}
	}
	return nil
}

// RemoveMilestone implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) RemoveMilestone(
	ctx context.Context,
	req *connect.Request[progressionv1.RemoveMilestoneRequest],
) (*connect.Response[progressionv1.RemoveMilestoneResponse], error) {
	m, id, err := s.milestoneCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMilestoneId())
	if err != nil {
		return nil, err
	}
	err = s.changePlanned(ctx, m, id, func(q *progressiondb.Queries) error {
		// Awards are never rewritten (ADR-0007) and deleting the milestone would
		// clear their milestone_id: one that was reached before, and undone, stays.
		if had, err := q.HasMilestoneAwards(ctx, progressiondb.HasMilestoneAwardsParams{CampaignID: m.CampaignID, MilestoneID: id}); err != nil {
			return fmt.Errorf("check the milestone's history: %w", err)
		} else if had {
			return errBlocked(progressionv1.XPBlockedReason_XP_BLOCKED_REASON_MILESTONE_HAS_HISTORY, "", 0)
		}
		return q.DeletePlannedMilestone(ctx, progressiondb.DeletePlannedMilestoneParams{CampaignID: m.CampaignID, ID: id})
	})
	if err != nil {
		return nil, s.dbError(ctx, "remove a milestone", err)
	}
	return connect.NewResponse(&progressionv1.RemoveMilestoneResponse{Milestones: s.listAfterWrite(ctx, m, "remove a milestone")}), nil
}

// MarkMilestoneReached implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) MarkMilestoneReached(
	ctx context.Context,
	req *connect.Request[progressionv1.MarkMilestoneReachedRequest],
) (*connect.Response[progressionv1.MarkMilestoneReachedResponse], error) {
	award, view, err := s.giveMilestone(ctx, req.Msg.GetCampaignId(), req.Msg.GetMilestoneId(), req.Msg.GetIdempotencyKey(), req.Msg.GetCharacterIds(), false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&progressionv1.MarkMilestoneReachedResponse{Milestone: view, Award: award}), nil
}

// GiveMilestoneTo implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) GiveMilestoneTo(
	ctx context.Context,
	req *connect.Request[progressionv1.GiveMilestoneToRequest],
) (*connect.Response[progressionv1.GiveMilestoneToResponse], error) {
	award, view, err := s.giveMilestone(ctx, req.Msg.GetCampaignId(), req.Msg.GetMilestoneId(), req.Msg.GetIdempotencyKey(), req.Msg.GetCharacterIds(), true)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&progressionv1.GiveMilestoneToResponse{Milestone: view, Award: award}), nil
}

// giveMilestone is MarkMilestoneReached and GiveMilestoneTo: the same award as
// MarkMilestone, linked to the planned milestone, with its text as the reason
// (checkMilestone, inside give's transaction, finds it and checks the
// milestone is in the state the call needs).
func (s *Service) giveMilestone(ctx context.Context, campaignID, milestoneID, key string, characterIDs []string, again bool) (*progressionv1.XPAward, *progressionv1.Milestone, error) {
	m, id, err := s.milestoneCall(ctx, campaignID, milestoneID)
	if err != nil {
		return nil, nil, err
	}
	g := grant{mode: modeMilestone, milestoneID: id, again: again}
	if g.key, g.characters, err = checkTargets(key, characterIDs); err != nil {
		return nil, nil, err
	}
	award, err := s.give(ctx, m, g)
	if err != nil {
		return nil, nil, err
	}
	views, err := s.milestoneViews(ctx, m)
	if err != nil {
		return nil, nil, s.dbError(ctx, "read the milestones", err)
	}
	view := milestoneByID(views, id)
	for _, mark := range view.GetMarks() {
		if mark.GetId() == award.ID {
			return mark, view, nil
		}
	}
	// The mark was undone since (a retry of a request whose award is undone):
	// still answer with it, as the history shows it.
	marks, err := s.awardViews(ctx, m, award)
	if err != nil {
		return nil, nil, s.dbError(ctx, "read a milestone's mark", err)
	}
	return marks[0], view, nil
}

// checkMilestone runs inside the transaction of a milestone award: it locks
// the planned milestone (so two marks of it take turns) and checks it is in
// the state the call needs. It returns the milestone's text, the award's
// reason.
func (s *Service) checkMilestone(ctx context.Context, q *progressiondb.Queries, campaignID string, g grant) (string, error) {
	ms, err := q.GetPlannedMilestoneForUpdate(ctx, progressiondb.GetPlannedMilestoneForUpdateParams{CampaignID: campaignID, ID: g.milestoneID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errMilestoneNotFound()
	}
	if err != nil {
		return "", fmt.Errorf("find the milestone: %w", err)
	}
	live, err := q.ListLiveMilestoneAwardsOf(ctx, progressiondb.ListLiveMilestoneAwardsOfParams{CampaignID: campaignID, MilestoneID: g.milestoneID})
	if err != nil {
		return "", fmt.Errorf("list the milestone's marks: %w", err)
	}
	switch {
	case !g.again && len(live) > 0:
		return "", errBlocked(progressionv1.XPBlockedReason_XP_BLOCKED_REASON_MILESTONE_ALREADY_REACHED, "", 0)
	case g.again && len(live) == 0:
		return "", errBlocked(progressionv1.XPBlockedReason_XP_BLOCKED_REASON_MILESTONE_NOT_REACHED, "", 0)
	case g.again:
		marked, err := q.ListMilestoneMarkedCharacters(ctx, progressiondb.ListMilestoneMarkedCharactersParams{CampaignID: campaignID, MilestoneID: g.milestoneID})
		if err != nil {
			return "", fmt.Errorf("list who has the milestone: %w", err)
		}
		for _, id := range g.characters {
			if slices.Contains(marked, id) {
				return "", errBlocked(progressionv1.XPBlockedReason_XP_BLOCKED_REASON_CHARACTER_ALREADY_MARKED, id, 0)
			}
		}
	}
	return ms.Text, nil
}

// milestoneCall is what the calls on one milestone start with: the caller is
// the campaign's master, the campaign counts milestones (MODE_NOT_ALLOWED
// otherwise), and the milestone ID is a UUID (any other is a
// milestone that does not exist).
func (s *Service) milestoneCall(ctx context.Context, campaignID, milestoneID string) (authz.Membership, string, error) {
	m, err := authz.RequireCampaignRole(ctx, campaignID, authz.RoleMaster)
	if err != nil {
		return m, "", err
	}
	if err := s.requireMilestonesMode(ctx, nil, m.CampaignID); err != nil {
		return m, "", err
	}
	id, err := uuid.Parse(milestoneID)
	if err != nil {
		return m, "", errMilestoneNotFound()
	}
	return m, id.String(), nil
}

// changePlanned runs change inside a transaction that has locked the
// milestone, once it is known to exist and to be still planned.
func (s *Service) changePlanned(ctx context.Context, m authz.Membership, id string, change func(q *progressiondb.Queries) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if err := s.requireMilestonesMode(ctx, tx, m.CampaignID); err != nil {
			return err
		}
		if _, err := q.GetPlannedMilestoneForUpdate(ctx, progressiondb.GetPlannedMilestoneForUpdateParams{CampaignID: m.CampaignID, ID: id}); errors.Is(err, pgx.ErrNoRows) {
			return errMilestoneNotFound()
		} else if err != nil {
			return fmt.Errorf("find the milestone: %w", err)
		}
		live, err := q.ListLiveMilestoneAwardsOf(ctx, progressiondb.ListLiveMilestoneAwardsOfParams{CampaignID: m.CampaignID, MilestoneID: id})
		if err != nil {
			return fmt.Errorf("list the milestone's marks: %w", err)
		}
		if len(live) > 0 {
			return errBlocked(progressionv1.XPBlockedReason_XP_BLOCKED_REASON_MILESTONE_ALREADY_REACHED, "", 0)
		}
		return change(q)
	})
}

// requireMilestonesMode refuses, with MODE_NOT_ALLOWED, a campaign that
// counts XP: it has no milestones. A write asks it again inside its transaction
// (tx), so that a change of the XP mode (RN-09) and the write take turns.
func (s *Service) requireMilestonesMode(ctx context.Context, tx pgx.Tx, campaignID string) error {
	mode, err := s.campaigns.CampaignXPMode(ctx, tx, campaignID)
	if err != nil {
		return s.dbError(ctx, "read the campaign's XP mode", err)
	}
	if mode != campaignsv1.XpMode_XP_MODE_MILESTONES {
		return errBlocked(progressionv1.XPBlockedReason_XP_BLOCKED_REASON_MODE_NOT_ALLOWED, "", mode)
	}
	return nil
}

// cleanMilestoneText checks a milestone's text: one line, 1 to 120
// characters.
func cleanMilestoneText(text string) (string, error) {
	text, err := names.Clean(text, maxMilestoneText)
	if err != nil {
		return "", invalidArgument("text", err)
	}
	return text, nil
}

// milestoneViews builds the campaign's milestones as the caller may read
// them, in the master's order: all of them for the master, the reached ones
// for a player; then the ones marked off the list, oldest first. The marks are
// the awards that are not undone, as the history shows them.
func (s *Service) milestoneViews(ctx context.Context, m authz.Membership) ([]*progressionv1.Milestone, error) {
	snap, err := s.readMilestones(ctx, m)
	if err != nil {
		return nil, err
	}
	var marks []*progressionv1.XPAward
	if len(snap.live) > 0 {
		if marks, err = s.viewsOf(ctx, m, snap.data, snap.live); err != nil {
			return nil, err
		}
	}
	g := groupMarks(snap.live, marks)
	out := make([]*progressionv1.Milestone, 0, len(snap.rows))
	for _, r := range snap.rows {
		v := &progressionv1.Milestone{Id: r.ID, Text: r.Text, Reached: len(g.byMilestone[r.ID]) > 0, Marks: g.byMilestone[r.ID], HasHistory: slices.Contains(snap.withHistory, r.ID)}
		if v.Reached {
			v.ReachedAt = timestamppb.New(g.reachedAt[r.ID])
		} else if m.Role != authz.RoleMaster {
			continue // a player never learns what is still planned
		}
		out = append(out, v)
	}
	return append(out, g.offList...), nil
}

// milestoneSnapshot is what milestoneViews reads in one read transaction.
type milestoneSnapshot struct {
	rows        []progressiondb.PlannedMilestone
	live        []progressiondb.XpAward // the marks not undone, oldest first
	withHistory []string                // the milestones that were ever marked
	data        awardData               // what the marks show, when there are any
}

// readMilestones reads the planned milestones, the marks and what the marks
// show as one moment: an undo between them would show a mark that is live
// next to a "Desfazer" on another one.
func (s *Service) readMilestones(ctx context.Context, m authz.Membership) (milestoneSnapshot, error) {
	var snap milestoneSnapshot
	err := db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if snap.rows, err = q.ListPlannedMilestones(ctx, m.CampaignID); err != nil {
			return fmt.Errorf("list the milestones: %w", err)
		}
		if snap.live, err = q.ListLiveMilestoneAwards(ctx, m.CampaignID); err != nil {
			return fmt.Errorf("list the marks: %w", err)
		}
		if snap.withHistory, err = q.ListMilestoneIDsWithAwards(ctx, m.CampaignID); err != nil {
			return fmt.Errorf("list the milestones with history: %w", err)
		}
		if len(snap.live) > 0 {
			snap.data, err = s.loadAwardData(ctx, q, m, snap.live)
		}
		return err
	})
	return snap, err
}

// markGroups is the live marks sorted out: by planned milestone with the
// time of its oldest, and the ones marked off the list as milestones of their
// own.
type markGroups struct {
	byMilestone map[string][]*progressionv1.XPAward
	reachedAt   map[string]time.Time
	offList     []*progressionv1.Milestone // one milestone per award
}

// groupMarks sorts out the live marks; marks holds their views, in the same
// order.
func groupMarks(live []progressiondb.XpAward, marks []*progressionv1.XPAward) markGroups {
	g := markGroups{byMilestone: map[string][]*progressionv1.XPAward{}, reachedAt: map[string]time.Time{}}
	for i, a := range live {
		id := conv.OrZero(a.MilestoneID)
		if id == "" {
			g.offList = append(g.offList, &progressionv1.Milestone{
				Id: a.ID, Text: a.Reason, Reached: true, ReachedAt: timestamppb.New(a.CreatedAt), Marks: []*progressionv1.XPAward{marks[i]}, OffList: true,
			})
			continue
		}
		g.byMilestone[id] = append(g.byMilestone[id], marks[i])
		if _, ok := g.reachedAt[id]; !ok {
			g.reachedAt[id] = a.CreatedAt // oldest first
		}
	}
	return g
}

// milestoneByID finds a milestone in a list of views.
func milestoneByID(views []*progressionv1.Milestone, id string) *progressionv1.Milestone {
	for _, v := range views {
		if v.GetId() == id {
			return v
		}
	}
	return nil
}

// listAfterWrite reads the campaign's milestones for the answer of a write
// that is committed (see committedViews).
func (s *Service) listAfterWrite(ctx context.Context, m authz.Membership, what string) []*progressionv1.Milestone {
	return s.committedViews(ctx, what, func() ([]*progressionv1.Milestone, error) { return s.milestoneViews(ctx, m) })
}

// committedViews reads the milestones for the answer of a write that is
// committed. When the read fails it logs the failure and gives no list: the
// write is there, and an error would make the app send it again. The answer
// then carries an empty list, which the app takes as "read it again".
func (s *Service) committedViews(ctx context.Context, what string, read func() ([]*progressionv1.Milestone, error)) []*progressionv1.Milestone {
	views, err := read()
	if err != nil {
		s.logger.ErrorContext(ctx, "read the milestones after a committed write", "action", what, "error", err)
		return nil
	}
	return views
}

// writtenMilestone is the milestone a write made or changed, as the answer
// shows it: the one in views, or, when the list could not be read, the row the
// write left, as a planned milestone.
func writtenMilestone(views []*progressionv1.Milestone, row progressiondb.PlannedMilestone) *progressionv1.Milestone {
	if v := milestoneByID(views, row.ID); v != nil {
		return v
	}
	return &progressionv1.Milestone{Id: row.ID, Text: row.Text}
}
