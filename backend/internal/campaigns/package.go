package campaigns

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns/campaignsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

// The campaign's share of the campaign package (MR-050): the campaign itself,
// the table's rules and the document. Everything here goes through the same
// checks the calls that make these things by hand use (names.Clean, the rules
// of SetTableRules, cleanDocument), so a package cannot put in a campaign what
// the master could not type.

// PackagePart returns the campaign's part of the package.
func (s *Service) PackagePart() campaignpackage.Part { return &packagePart{s: s} }

type packagePart struct{ s *Service }

// stagedCampaign is what Stage prepares for Apply.
type stagedCampaign struct {
	name     string
	xpMode   string
	rules    campaignsdb.UpsertTableRulesParams
	dice     string
	document string
}

const stagedKey = "campaigns.campaign"

// Export implements campaignpackage.Part.
func (p *packagePart) Export(ctx context.Context, tx pgx.Tx, campaignID string, snap *campaignpackage.Snapshot) error {
	q := p.s.queries.WithTx(tx)
	c, err := q.GetCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("read the campaign: %w", err)
	}
	row, err := p.s.rowOf(ctx, tx, campaignID)
	if err != nil {
		return err
	}
	doc, err := q.GetCampaignDocument(ctx, campaignID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read the campaign document: %w", err)
	}
	snap.CampaignName = c.Name
	return snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CAMPAIGN, campaignpackage.CampaignEntry, &pkgv1.PackageCampaign{
		Name:       c.Name,
		XpMode:     xpModeFromDB[c.XpMode],
		TableRules: tableRulesToProto(row, c.DiceMode),
		Document:   doc.Body,
	})
}

// Stage implements campaignpackage.Part.
func (p *packagePart) Stage(_ context.Context, in *campaignpackage.Import) error {
	entries := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CAMPAIGN)
	if len(entries) != 1 || entries[0].GetPath() != campaignpackage.CampaignEntry {
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CAMPAIGN, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY, 0)
		return nil
	}
	var msg pkgv1.PackageCampaign
	if !in.Read(campaignpackage.CampaignEntry, &msg) {
		return nil
	}
	bad := func(kind pkgv1.PackageProblemKind) {
		in.Problem(kind, msg.GetName(), pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
	}
	staged := stagedCampaign{}
	name, err := names.Clean(msg.GetName(), MaxNameLength)
	if err != nil {
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CAMPAIGN, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil //nolint:nilerr // a bad package is a problem for the preview, not a failure
	}
	staged.name = name
	in.CampaignName = name
	if staged.xpMode = xpModeToDB[msg.GetXpMode()]; staged.xpMode == "" {
		bad(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CAMPAIGN)
		return nil
	}
	if staged.rules, staged.dice, err = tableRulesParams(msg.GetTableRules()); err != nil {
		bad(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CAMPAIGN)
		return nil //nolint:nilerr // a bad package is a problem for the preview, not a failure
	}
	if staged.document, err = cleanDocument(msg.GetDocument()); err != nil {
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_DOCUMENT, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, MaxDocumentBytes)
		return nil //nolint:nilerr // a bad package is a problem for the preview, not a failure
	}
	in.Set(stagedKey, &staged)
	return nil
}

// errCampaignReplayed ends an Apply whose idempotency key already made a
// campaign (another call with the same key won the race).
var errCampaignReplayed = campaignpackage.ErrReplayed

// Apply implements campaignpackage.Part. It creates the campaign, its master,
// its rules and its document; the other parts fill the campaign after it.
func (p *packagePart) Apply(ctx context.Context, tx pgx.Tx, in *campaignpackage.Import) error {
	staged, _ := in.Get(stagedKey).(*stagedCampaign)
	if staged == nil {
		return errors.New("campaigns: the package part was not staged")
	}
	q := p.s.queries.WithTx(tx)
	// The cap is counted in the transaction that inserts, as CreateCampaign does:
	// two creations at the same time cannot both slip under it.
	if p.s.maxCampaigns > 0 {
		mastered, err := q.CountMasteredCampaigns(ctx, in.UserID)
		if err != nil {
			return fmt.Errorf("count the campaigns the caller is master of: %w", err)
		}
		if int(mastered) >= p.s.maxCampaigns {
			return errCreationRefused(campaignsv1.CampaignCreationRefusedReason_CAMPAIGN_CREATION_REFUSED_REASON_LIMIT_REACHED, p.s.maxCampaigns)
		}
	}
	_, err := q.InsertImportedCampaign(ctx, campaignsdb.InsertImportedCampaignParams{
		ID: in.CampaignID, Name: staged.name, XpMode: staged.xpMode, DiceMode: staged.dice, CreatedBy: in.UserID,
		CreateKey: in.CreateKey, CreateHash: in.CreateHash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errCampaignReplayed
	}
	if err != nil {
		return fmt.Errorf("insert the campaign: %w", err)
	}
	if _, err := q.InsertMember(ctx, campaignsdb.InsertMemberParams{
		CampaignID: in.CampaignID, UserID: in.UserID, Role: string(authz.RoleMaster), Status: string(authz.StatusActive),
	}); err != nil {
		return fmt.Errorf("insert the master: %w", err)
	}
	now := p.s.now()
	rules := staged.rules
	rules.CampaignID, rules.Now = in.CampaignID, now
	if _, err := q.UpsertTableRules(ctx, rules); err != nil {
		return fmt.Errorf("save the table rules: %w", err)
	}
	// The document's links name maps, characters and images of the package; they
	// get the new ids, decided once (the fresh ids of links to nothing differ per call).
	body, _ := in.Get(rewrittenKey).(string)
	if body == "" && staged.document != "" {
		body = in.IDs.RewriteDocument(staged.document)
		in.Set(rewrittenKey, body)
	}
	if body != "" {
		if _, err := q.InsertCampaignDocument(ctx, campaignsdb.InsertCampaignDocumentParams{
			CampaignID: in.CampaignID, Body: body, UpdatedAt: now, UpdatedBy: &in.UserID,
		}); err != nil {
			return fmt.Errorf("save the campaign document: %w", err)
		}
	}
	return nil
}

const rewrittenKey = "campaigns.document"

// CheckCreation implements campaignpackage.Creation: the errors CreateCampaign
// gives a person who may not create a campaign, before any byte is sent.
func (s *Service) CheckCreation(ctx context.Context, userID string) error {
	allowed, err := s.canCreate(ctx, userID)
	if err != nil {
		return s.dbError(ctx, "check who may create campaigns", err)
	}
	if !allowed {
		return errCreationRefused(campaignsv1.CampaignCreationRefusedReason_CAMPAIGN_CREATION_REFUSED_REASON_NOT_ALLOWED, 0)
	}
	if s.maxCampaigns > 0 {
		mastered, err := s.queries.CountMasteredCampaigns(ctx, userID)
		if err != nil {
			return s.dbError(ctx, "count the campaigns the caller is master of", err)
		}
		if int(mastered) >= s.maxCampaigns {
			return errCreationRefused(campaignsv1.CampaignCreationRefusedReason_CAMPAIGN_CREATION_REFUSED_REASON_LIMIT_REACHED, s.maxCampaigns)
		}
	}
	return nil
}

// FindCreated implements campaignpackage.Creation.
func (s *Service) FindCreated(ctx context.Context, userID, key string, hash *string) (*campaignsv1.Campaign, bool, error) {
	c, err := s.queries.GetCampaignByCreateKey(ctx, idem.Scope(userID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, s.dbError(ctx, "find the campaign of a key", err)
	}
	if err := idem.SameRequest(c.CreateHash, hash); err != nil {
		return nil, false, err
	}
	out := campaignToProto(c, authz.RoleMaster, false)
	out.CreatedAt = timestamppb.New(c.CreatedAt)
	return out, true, nil
}
